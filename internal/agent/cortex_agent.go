package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cortexai/cortexai/internal/tools"
	"github.com/rs/zerolog/log"
)

// ToolCall represents a tool invocation request from the LLM
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]interface{}
}

// CortexAgent wraps Anthropic SDK for multi-turn tool-calling agent loop
type CortexAgent struct {
	client    *anthropic.Client
	model     string
	maxTokens int
}

// NewCortexAgent creates an agent backed by Anthropic Claude or compatible provider (e.g. Z.ai)
func NewCortexAgent(apiKey, model, baseURL string) *CortexAgent {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	return &CortexAgent{
		client:    client,
		model:     model,
		maxTokens: 4096,
	}
}

// Run executes the agent loop: LLM calls tools until stop_reason = "end_turn".
// Returns (finalText, toolsUsed, lastExecutedSQL, error).
// lastExecutedSQL is the last SQL passed to execute_bigquery_sql tool — used as
// fallback when the model doesn't include a ```sql block in its final reply.
func (a *CortexAgent) Run(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool) (string, []string, string, error) {
	// Build Anthropic tool definitions as ToolUnionUnionParam slice
	anthToolParams := make([]anthropic.ToolUnionUnionParam, len(agentTools))
	for i, t := range agentTools {
		var propsRaw interface{}
		if props, ok := t.InputSchema["properties"]; ok {
			propsRaw = props
		}

		schema := map[string]interface{}{
			"type":       "object",
			"properties": propsRaw,
		}
		if required, ok := t.InputSchema["required"]; ok {
			schema["required"] = required
		}
		anthToolParams[i] = anthropic.ToolParam{
			Name:        anthropic.String(t.Name),
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.F[interface{}](schema),
		}
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
	}

	var toolsUsed []string
	var lastExecutedSQL string // track last SQL sent to execute_bigquery_sql tool
	maxIter := 10

	for iter := 0; iter < maxIter; iter++ {
		params := anthropic.MessageNewParams{
			Model:     anthropic.F(anthropic.Model(a.model)),
			MaxTokens: anthropic.F(int64(a.maxTokens)),
			Messages:  anthropic.F(messages),
			Tools:     anthropic.F(anthToolParams),
		}
		if systemPrompt != "" {
			params.System = anthropic.F([]anthropic.TextBlockParam{
				anthropic.NewTextBlock(systemPrompt),
			})
		}

		resp, err := a.client.Messages.New(ctx, params)
		if err != nil {
			return "", toolsUsed, lastExecutedSQL, fmt.Errorf("LLM call failed: %w", err)
		}

		// Collect text and tool calls from response
		var textContent string
		var pendingToolCalls []ToolCall

		for _, block := range resp.Content {
			switch b := block.AsUnion().(type) {
			case anthropic.TextBlock:
				textContent += b.Text
			case anthropic.ToolUseBlock:
				var input map[string]interface{}
				// FIX #18: handle json.Unmarshal error
				if err := json.Unmarshal(b.Input, &input); err != nil {
					log.Warn().Err(err).Str("tool", b.Name).Msg("failed to parse tool input")
					input = map[string]interface{}{}
				}
				pendingToolCalls = append(pendingToolCalls, ToolCall{
					ID:    b.ID,
					Name:  b.Name,
					Input: input,
				})
			}
		}

		log.Debug().
			Int("iter", iter).
			Str("stop_reason", string(resp.StopReason)).
			Str("text_preview", func() string {
				if len(textContent) > 80 {
					return textContent[:80]
				}
				return textContent
			}()).
			Int("tool_calls", len(pendingToolCalls)).
			Msg("agent iteration")

		// Exit if done: end_turn / stop / no pending tools / not tool_use
		isDone := resp.StopReason == "end_turn" ||
			resp.StopReason == "stop" ||
			resp.StopReason == "stop_sequence" ||
			resp.StopReason == "max_tokens" ||
			len(pendingToolCalls) == 0
		if isDone {
			return textContent, toolsUsed, lastExecutedSQL, nil
		}

		// Force final answer after 7 iterations to avoid runaway loops
		if iter >= 7 {
			messages = append(messages, resp.ToParam())
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock("You have enough data. Please provide your final answer now without calling any more tools."),
			))
			params := anthropic.MessageNewParams{
				Model:     anthropic.F(anthropic.Model(a.model)),
				MaxTokens: anthropic.F(int64(a.maxTokens)),
				Messages:  anthropic.F(messages),
			}
			if systemPrompt != "" {
				params.System = anthropic.F([]anthropic.TextBlockParam{anthropic.NewTextBlock(systemPrompt)})
			}
			finalResp, err := a.client.Messages.New(ctx, params)
			if err != nil {
				return textContent, toolsUsed, lastExecutedSQL, fmt.Errorf("final answer call failed: %w", err)
			}
			for _, block := range finalResp.Content {
				if b, ok := block.AsUnion().(anthropic.TextBlock); ok {
					textContent += b.Text
				}
			}
			return textContent, toolsUsed, lastExecutedSQL, nil
		}

		// Add assistant message using ToParam() helper
		messages = append(messages, resp.ToParam())

		// Execute tools and build tool results
		var toolResults []anthropic.ContentBlockParamUnion
		for _, tc := range pendingToolCalls {
			toolsUsed = append(toolsUsed, tc.Name)
			// Track the last SQL executed so bigquery_handler can use it as fallback
			if tc.Name == "execute_bigquery_sql" {
				if sql, ok := tc.Input["sql"].(string); ok && sql != "" {
					lastExecutedSQL = sql
				}
			}
			result, execErr := executeTool(ctx, tc, agentTools)
			if execErr != nil {
				log.Warn().Err(execErr).Str("tool", tc.Name).Msg("tool execution error")
				result = fmt.Sprintf("error: %v", execErr)
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tc.ID, result, execErr != nil))
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", toolsUsed, lastExecutedSQL, fmt.Errorf("agent loop exceeded max iterations (%d)", maxIter)
}

func executeTool(ctx context.Context, tc ToolCall, agentTools []tools.Tool) (string, error) {
	for _, t := range agentTools {
		if t.Name == tc.Name {
			return t.Execute(ctx, tc.Input)
		}
	}
	return "", fmt.Errorf("unknown tool: %s", tc.Name)
}

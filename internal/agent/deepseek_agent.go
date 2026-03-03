package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cortexai/cortexai/internal/tools"
	"github.com/rs/zerolog/log"
)

// DeepSeekAgent implements LLMRunner for DeepSeek (OpenAI-compatible endpoint).
// It uses net/http + encoding/json directly — no external SDK required.
type DeepSeekAgent struct {
	apiKey     string
	model      string
	baseURL    string
	maxTokens  int
	httpClient *http.Client
}

// NewDeepSeekAgent creates a DeepSeekAgent.
// model defaults to "deepseek-chat"; baseURL defaults to "https://api.deepseek.com/v1".
func NewDeepSeekAgent(apiKey, model, baseURL string) *DeepSeekAgent {
	if model == "" {
		model = "deepseek-chat"
	}
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	return &DeepSeekAgent{
		apiKey:     apiKey,
		model:      model,
		baseURL:    strings.TrimRight(baseURL, "/"),
		maxTokens:  4096,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Model returns the configured model identifier.
func (a *DeepSeekAgent) Model() string { return a.model }

// Run executes the agent loop (no streaming events).
func (a *DeepSeekAgent) Run(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool) (string, []string, string, error) {
	return a.run(ctx, systemPrompt, userPrompt, agentTools, nil)
}

// RunWithEmit is like Run but calls emitFn at each LLM iteration and tool call.
func (a *DeepSeekAgent) RunWithEmit(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool, emitFn EmitFn) (string, []string, string, error) {
	return a.run(ctx, systemPrompt, userPrompt, agentTools, emitFn)
}

// ── OpenAI wire types ────────────────────────────────────────────────────────

type dsMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	ToolCalls  []dsToolCall `json:"tool_calls,omitempty"`
}

type dsToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function dsFunctionCall `json:"function"`
}

type dsFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type dsTool struct {
	Type     string       `json:"type"` // "function"
	Function dsFunctionDef `json:"function"`
}

type dsFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type dsChatRequest struct {
	Model     string      `json:"model"`
	Messages  []dsMessage `json:"messages"`
	Tools     []dsTool    `json:"tools,omitempty"`
	MaxTokens int         `json:"max_tokens"`
}

type dsChatResponse struct {
	Choices []dsChoice `json:"choices"`
	Error   *dsError   `json:"error,omitempty"`
}

type dsChoice struct {
	Message      dsMessage `json:"message"`
	FinishReason string    `json:"finish_reason"`
}

type dsError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ── Helper functions (also used by tests) ───────────────────────────────────

// buildInitialMessages creates the initial message list from system/user prompts.
func buildInitialMessages(systemPrompt, userPrompt string) []dsMessage {
	var messages []dsMessage
	if systemPrompt != "" {
		messages = append(messages, dsMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, dsMessage{Role: "user", Content: userPrompt})
	return messages
}

// convertToOpenAITools converts tools.Tool slice to DeepSeek/OpenAI tool format.
func convertToOpenAITools(agentTools []tools.Tool) []dsTool {
	dsTools := make([]dsTool, len(agentTools))
	for i, t := range agentTools {
		schema := map[string]interface{}{
			"type": "object",
		}
		if props, ok := t.InputSchema["properties"]; ok {
			schema["properties"] = props
		}
		if required, ok := t.InputSchema["required"]; ok {
			schema["required"] = required
		}
		dsTools[i] = dsTool{
			Type: "function",
			Function: dsFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schema,
			},
		}
	}
	return dsTools
}

// ── Core agent loop ──────────────────────────────────────────────────────────

func (a *DeepSeekAgent) run(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool, emitFn EmitFn) (string, []string, string, error) {
	dsTools := convertToOpenAITools(agentTools)
	messages := buildInitialMessages(systemPrompt, userPrompt)

	var toolsUsed []string
	var lastExecutedSQL string
	maxIter := 10

	for iter := 0; iter < maxIter; iter++ {
		if emitFn != nil {
			emitFn("llm_call", map[string]interface{}{"iteration": iter})
		}

		resp, err := a.callAPI(ctx, messages, dsTools)
		if err != nil {
			return "", toolsUsed, lastExecutedSQL, fmt.Errorf("LLM call failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", toolsUsed, lastExecutedSQL, fmt.Errorf("LLM returned no choices")
		}

		choice := resp.Choices[0]
		msg := choice.Message
		textContent := msg.Content

		log.Debug().
			Int("iter", iter).
			Str("finish_reason", choice.FinishReason).
			Str("text_preview", func() string {
				if len(textContent) > 80 {
					return textContent[:80]
				}
				return textContent
			}()).
			Int("tool_calls", len(msg.ToolCalls)).
			Msg("deepseek iteration")

		// "length" finish_reason: model was truncated — return immediately
		if choice.FinishReason == "length" {
			return textContent, toolsUsed, lastExecutedSQL, nil
		}

		// No tool calls → done (handles "stop" and any other finish_reason)
		if len(msg.ToolCalls) == 0 {
			return textContent, toolsUsed, lastExecutedSQL, nil
		}

		// Force final answer after 7 iterations to avoid runaway loops
		if iter >= 7 {
			messages = append(messages, msg)
			messages = append(messages, dsMessage{
				Role:    "user",
				Content: "You have enough data. Please provide your final answer now without calling any more tools.",
			})
			finalResp, finalErr := a.callAPI(ctx, messages, nil)
			if finalErr != nil {
				return textContent, toolsUsed, lastExecutedSQL, fmt.Errorf("final answer call failed: %w", finalErr)
			}
			if len(finalResp.Choices) > 0 {
				textContent += finalResp.Choices[0].Message.Content
			}
			return textContent, toolsUsed, lastExecutedSQL, nil
		}

		// Append assistant message and execute tools
		messages = append(messages, msg)

		var toolResultMessages []dsMessage
		for _, tc := range msg.ToolCalls {
			name := tc.Function.Name
			toolsUsed = append(toolsUsed, name)

			var input map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				log.Warn().Err(err).Str("tool", name).Msg("failed to parse tool arguments")
				input = map[string]interface{}{}
			}

			// Track last executed SQL for fallback
			if name == "execute_bigquery_sql" {
				if sql, ok := input["sql"].(string); ok && sql != "" {
					lastExecutedSQL = sql
				}
			}

			if emitFn != nil {
				evData := map[string]interface{}{"tool": name, "iteration": iter}
				if name == "execute_bigquery_sql" {
					if sql, ok := input["sql"].(string); ok && len(sql) > 0 {
						preview := sql
						if len(preview) > 120 {
							preview = preview[:120] + "..."
						}
						evData["sql_preview"] = preview
					}
				}
				emitFn("tool_call", evData)
			}

			result, execErr := executeTool(ctx, ToolCall{ID: tc.ID, Name: name, Input: input}, agentTools)
			if execErr != nil {
				log.Warn().Err(execErr).Str("tool", name).Msg("tool execution error")
				result = fmt.Sprintf("error: %v", execErr)
			}
			toolResultMessages = append(toolResultMessages, dsMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
		messages = append(messages, toolResultMessages...)
	}

	return "", toolsUsed, lastExecutedSQL, fmt.Errorf("agent loop exceeded max iterations (%d)", maxIter)
}

func (a *DeepSeekAgent) callAPI(ctx context.Context, messages []dsMessage, dsTools []dsTool) (*dsChatResponse, error) {
	reqBody := dsChatRequest{
		Model:     a.model,
		Messages:  messages,
		MaxTokens: a.maxTokens,
	}
	if len(dsTools) > 0 {
		reqBody.Tools = dsTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := a.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	httpResp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", httpResp.StatusCode, string(respBytes))
	}

	var resp dsChatResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	return &resp, nil
}

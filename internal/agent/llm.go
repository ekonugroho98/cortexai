package agent

import (
	"context"

	"github.com/cortexai/cortexai/internal/tools"
)

// LLMRunner is the interface implemented by CortexAgent (Anthropic) and DeepSeekAgent
// (OpenAI-compatible), enabling dependency injection in BigQueryHandler and
// ElasticsearchHandler without coupling them to a specific provider SDK.
type LLMRunner interface {
	// Run executes the agent loop.
	// Returns (finalText, toolsUsed, lastExecutedSQL, error).
	Run(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool) (string, []string, string, error)

	// RunWithEmit is like Run but calls emitFn at each LLM iteration and tool call.
	RunWithEmit(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool, emitFn EmitFn) (string, []string, string, error)

	// Model returns the model identifier used by this runner.
	Model() string
}

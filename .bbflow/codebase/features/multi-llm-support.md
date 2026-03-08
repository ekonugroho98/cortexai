# Feature: Multi-LLM Support

## Overview
Pluggable LLM provider system supporting Anthropic Claude and DeepSeek via a common interface.

## Key Files
- `internal/agent/llm.go` — `LLMRunner` interface
- `internal/agent/cortex_agent.go` — Anthropic implementation
- `internal/agent/deepseek_agent.go` — DeepSeek implementation (pure net/http)
- `internal/config/config.go` — `LLMProvider` field ("anthropic" | "deepseek")
- `internal/server/routes.go` — provider factory switch

## Interface
```go
type LLMRunner interface {
    Run(ctx context.Context, system, user string, tools []tools.Tool) (string, error)
    RunWithEmit(ctx context.Context, system, user string, tools []tools.Tool, emit EmitFn) (string, error)
    Model() string
}
```

## Agent Loop
- Max 10 iterations, force final answer at iteration 7
- Tool calling with execute feedback
- Handles GLM quirk: "stop" reason WITH tool_use blocks
- LastExecutedSQL tracking for fallback SQL extraction

## Config
```json
{
  "llm_provider": "anthropic",
  "anthropic_api_key": "...",
  "anthropic_base_url": "https://open.bigmodel.cn/api/anthropic/",
  "deepseek_api_key": "...",
  "deepseek_base_url": "https://api.deepseek.com/v1"
}
```

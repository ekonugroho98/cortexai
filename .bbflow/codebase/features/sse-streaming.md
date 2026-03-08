# Feature: SSE Streaming

## Overview
Server-Sent Events for real-time feedback during long-running LLM agent operations.

## Key Files
- `internal/handler/agent.go` — `QueryAgentStream()` endpoint
- `internal/agent/bigquery_handler.go` — `HandleStream()` with emitFn
- `internal/agent/elasticsearch_handler.go` — `HandleStream()` with emitFn
- `internal/agent/llm.go` — `EmitFn` type, `RunWithEmit()` method

## Endpoint
`POST /api/v1/query-agent/stream` (analyst+ role)

## Event Types
| Event | Data |
|-------|------|
| `start` | Initial metadata |
| `progress` | Status updates |
| `llm_call` | LLM iteration info |
| `tool_call` | Tool execution details |
| `result` | Final AgentResponse |
| `error` | Error details |

## Implementation
- `Content-Type: text/event-stream`
- Flush after each SSE event
- EmitFn callback passed through handler → agent → LLMRunner

# Feature: Persona System (Tool Filtering + Data Source Restriction)

**Status:** Active
**Mode:** BROWNFIELD — extends core config + agent handler
**Last updated:** 2026-02-27

---

## Overview

The persona system allows per-user AI behavior customization. Each user can be assigned a named persona (via `UserConfig.Persona`), which controls the LLM provider, model, system prompt style, and now — tool availability and data source access.

---

## PersonaConfig Fields

```go
type PersonaConfig struct {
    Provider           string   // "anthropic" | "deepseek"
    Model              string   // e.g. "claude-sonnet-4-6"
    BaseURL            string   // optional: override base URL
    SystemPromptStyle  string   // "executive" | "technical" | "support"
    MaxTokens          int      // 0 = use agent default (4096)
    ExcludedTools      []string // tool names hidden from LLM; nil = all tools
    AllowedDataSources []string // allowed data sources; nil = all sources
}
```

All fields except `Provider` and `Model` are optional. Nil/empty `ExcludedTools` and `AllowedDataSources` means unrestricted — backward compatible.

---

## Tool Filtering

**Location:** `internal/agent/bigquery_handler.go` — `filterTools()`

```go
func filterTools(ts []tools.Tool, excluded []string) []tools.Tool
```

- Called in both `Handle()` and `HandleStream()` before `runner.Run()` / `runner.RunWithEmit()`
- Uses `map[string]struct{}` for O(1) lookup
- Returns `ts` unchanged if `excluded` is nil/empty (zero allocation)
- Does not mutate input slice

**Example:** `executive` persona with `excluded_tools: ["get_bigquery_sample_data"]` → LLM sees only 4 BQ tools, never offered sample data fetching.

---

## Data Source Restriction

**Location:** `internal/handler/agent.go` — `checkDataSourceAllowed()`

```go
func (h *AgentHandler) checkDataSourceAllowed(pc config.PersonaConfig, dataSource string) error
```

- Returns `nil` if `pc.AllowedDataSources` is nil/empty (allow all — default)
- Returns descriptive error if `dataSource` not in allowed list
- HTTP 403 response in both `QueryAgent` and `QueryAgentStream`
- In streaming path: check occurs **before** SSE headers are written (so 403 status can still be set)

**Error format:** `"data source '%s' is not available for your persona. Available: %v"`

---

## resolvePersona()

**Location:** `internal/handler/agent.go`

```go
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string, config.PersonaConfig)
```

Returns the LLMRunner, prompt style, and full `PersonaConfig` for the user's persona.
Zero-value `PersonaConfig{}` is the safe default — nil slices = no restrictions.

---

## Config Example

```json
"personas": {
  "executive": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "system_prompt_style": "executive",
    "max_tokens": 2048,
    "excluded_tools": ["get_bigquery_sample_data"],
    "allowed_data_sources": ["bigquery"]
  },
  "developer": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "system_prompt_style": "technical"
  }
}
```

---

## Files

| File | Role |
|------|------|
| `internal/config/config.go` | `PersonaConfig` struct definition |
| `internal/agent/bigquery_handler.go` | `filterTools()`, `Handle()`, `HandleStream()` |
| `internal/handler/agent.go` | `resolvePersona()`, `checkDataSourceAllowed()`, `QueryAgent()`, `QueryAgentStream()` |
| `internal/agent/bigquery_handler_test.go` | `TestFilterTools_*` (6 tests) |
| `internal/handler/agent_test.go` | `TestCheckDataSourceAllowed_*` (6 tests) |
| `config/cortexai.example.json` | Reference config with executive persona |

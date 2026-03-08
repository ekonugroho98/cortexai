# DESIGN — Persona Tool Filtering + Data Source Restriction

**Spec ID:** persona-tool-filtering
**Mode:** BROWNFIELD
**Complexity:** SIMPLE
**Created:** 2026-02-27

---

## File Structure

### MODIFY (4 files, 0 new files)

```
internal/config/config.go
  PersonaConfig struct:
    + ExcludedTools      []string `json:"excluded_tools,omitempty"`
    + AllowedDataSources []string `json:"allowed_data_sources,omitempty"`

internal/agent/bigquery_handler.go
  NEW (private helper):
    filterTools(tools []tools.Tool, excluded []string) []tools.Tool
  MODIFY Handle():
    func (..., promptStyle string, excludedTools []string) (*models.AgentResponse, error)
    — apply filterTools(bqTools, excludedTools) before runner.Run()
  MODIFY HandleStream():
    func (..., emitFn EmitFn, excludedTools []string) error
    — apply filterTools(bqTools, excludedTools) before runner.RunWithEmit()

internal/handler/agent.go
  MODIFY resolvePersona():
    func (...) (agent.LLMRunner, string, config.PersonaConfig)
  NEW (private method):
    func (h *AgentHandler) checkDataSourceAllowed(pc config.PersonaConfig, dataSource string) error
  MODIFY QueryAgent():
    — after resolvePersona(), call checkDataSourceAllowed(); if error → HTTP 403
    — pass pc.ExcludedTools to bqHandler.Handle()
  MODIFY QueryAgentStream():
    — after resolvePersona(), call checkDataSourceAllowed(); if error → HTTP 403
    — pass pc.ExcludedTools to bqHandler.HandleStream()

config/cortexai.example.json
  UPDATE personas.executive:
    + "excluded_tools": ["get_bigquery_sample_data"]
    + "allowed_data_sources": ["bigquery"]
```

---

## Data Models

### PersonaConfig (MODIFY — config.go)

```go
type PersonaConfig struct {
    Provider           string   `json:"provider"`
    Model              string   `json:"model"`
    SystemPromptStyle  string   `json:"system_prompt_style"`
    BaseURL            string   `json:"base_url,omitempty"`
    MaxTokens          int      `json:"max_tokens,omitempty"`
    ExcludedTools      []string `json:"excluded_tools,omitempty"`      // NEW
    AllowedDataSources []string `json:"allowed_data_sources,omitempty"` // NEW
}
```

Nil/empty `ExcludedTools` → all tools passed (backward compat, REQ-005).
Nil/empty `AllowedDataSources` → all data sources allowed (backward compat, REQ-006).

---

## Component Responsibilities

| Component | Change | Responsibility |
|-----------|--------|----------------|
| `config.PersonaConfig` | MODIFY (+2 fields) | Holds per-persona tool and data source restrictions |
| `bigquery_handler.filterTools()` | NEW (private) | Filters tool slice by removing excluded names using `t.Name` field |
| `BigQueryHandler.Handle()` | MODIFY (+1 param) | Applies tool exclusions before LLM call |
| `BigQueryHandler.HandleStream()` | MODIFY (+1 param) | Applies tool exclusions before LLM stream call |
| `AgentHandler.resolvePersona()` | MODIFY (extend return) | Returns full `PersonaConfig` for downstream restriction enforcement |
| `AgentHandler.checkDataSourceAllowed()` | NEW (private method) | Validates data source against `AllowedDataSources`; returns nil if unrestricted |
| `AgentHandler.QueryAgent()` | MODIFY | Enforces data source restriction (403) + passes `ExcludedTools` to Handle() |
| `AgentHandler.QueryAgentStream()` | MODIFY | Enforces data source restriction (403) + passes `ExcludedTools` to HandleStream() |

---

## Integration Points

- `filterTools()` is package-private to `internal/agent` — not exported.
- `checkDataSourceAllowed()` is a method on `AgentHandler` — not exported.
- No changes to `ElasticsearchHandler`, `LLMPool`, `system_prompts.go`, or `LLMRunner` interface.
- `tools.Tool.Name` is a direct struct field (not a method) — used for string comparison in `filterTools`.
- Data source identifiers match `service.DataSourceBigQuery` (`"bigquery"`) and `service.DataSourceElasticsearch` (`"elasticsearch"`).

### filterTools() Implementation Sketch

```go
func filterTools(ts []tools.Tool, excluded []string) []tools.Tool {
    if len(excluded) == 0 {
        return ts
    }
    excludedSet := make(map[string]struct{}, len(excluded))
    for _, name := range excluded {
        excludedSet[name] = struct{}{}
    }
    result := make([]tools.Tool, 0, len(ts))
    for _, t := range ts {
        if _, skip := excludedSet[t.Name]; !skip {
            result = append(result, t)
        }
    }
    return result
}
```

### checkDataSourceAllowed() Implementation Sketch

```go
func (h *AgentHandler) checkDataSourceAllowed(pc config.PersonaConfig, dataSource string) error {
    if len(pc.AllowedDataSources) == 0 {
        return nil // unrestricted
    }
    for _, allowed := range pc.AllowedDataSources {
        if allowed == dataSource {
            return nil
        }
    }
    return fmt.Errorf("data source '%s' is not available for your persona. Available: %v",
        dataSource, pc.AllowedDataSources)
}
```

---

## Error Handling

| Scenario | HTTP Status | Response Body |
|----------|-------------|---------------|
| Data source not in `AllowedDataSources` | 403 | `{"error": "data source 'elasticsearch' is not available for your persona. Available: [bigquery]"}` |
| `AllowedDataSources` is nil/empty | — | No error (allow all) |
| `ExcludedTools` is nil/empty | — | No error (all tools passed) |

Use existing `models.WriteError(w, http.StatusForbidden, msg)` helper for consistency.

---

## Testing Strategy

| Test | File | Coverage |
|------|------|----------|
| `TestFilterTools_NilExclusion` | `internal/agent/bigquery_handler_test.go` | nil excluded → all tools returned |
| `TestFilterTools_SingleExclusion` | `internal/agent/bigquery_handler_test.go` | one name excluded → correct tool removed |
| `TestFilterTools_AllExcluded` | `internal/agent/bigquery_handler_test.go` | all names excluded → empty slice |
| `TestCheckDataSourceAllowed_NilAllowed` | `internal/handler/agent_test.go` (new) | nil AllowedDataSources → nil error |
| `TestCheckDataSourceAllowed_Allowed` | `internal/handler/agent_test.go` (new) | source in list → nil error |
| `TestCheckDataSourceAllowed_Blocked` | `internal/handler/agent_test.go` (new) | source not in list → error with message |

All 44 existing agent tests must continue passing (no regressions).

---

## Implementation Notes

1. **excludedTools param position**: Append at end of `Handle()` and `HandleStream()` — minimizes diff at call sites.
2. **resolvePersona() return**: The zero-value `config.PersonaConfig{}` is returned for unknown/nil personas — `checkDataSourceAllowed` on a zero-value PC has nil `AllowedDataSources`, which means "allow all" (correct default).
3. **QueryAgentStream ES ordering**: The data source check must happen BEFORE the existing `"streaming is not yet supported for Elasticsearch"` 501 return — so the 403 (persona restriction) takes precedence when applicable.
4. **No changes to routes.go**: All new params are internal to handler/agent.go call chains.

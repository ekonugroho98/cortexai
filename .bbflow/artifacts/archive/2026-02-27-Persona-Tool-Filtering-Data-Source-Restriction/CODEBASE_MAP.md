# CODEBASE MAP — persona-tool-filtering

**Generated:** 2026-02-27
**Mode:** BROWNFIELD
**Based on:** persona-system implementation (just completed)

---

## Affected Files

### MODIFY

#### `internal/config/config.go`
- **Struct:** `PersonaConfig` — tambah dua field opsional
- **Change:**
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

---

#### `internal/agent/bigquery_handler.go`
- **Function:** `Handle(ctx, req, apiKey, allowedDatasets, runner, promptStyle)` → tambah `excludedTools []string`
- **Function:** `HandleStream(ctx, req, apiKey, allowedDatasets, runner, promptStyle, emitFn)` → tambah `excludedTools []string`
- **Logic:** Build filtered tool list:
  ```go
  tools := buildTools(h.bq, req.DatasetID, ...)
  tools = filterTools(tools, excludedTools)
  runner.Run(ctx, systemPrompt, userPrompt, tools)
  ```
- **New helper:** `filterTools(tools []tools.Tool, excluded []string) []tools.Tool`

---

#### `internal/handler/agent.go`
- **Function:** `resolvePersona(user)` → already returns `(LLMRunner, string)`, extend to return persona config or expose ExcludedTools + AllowedDataSources
- **Function:** `QueryAgent()` — tambah logika:
  1. Check `AllowedDataSources` sebelum routing ke BQ/ES
  2. Pass `ExcludedTools` ke `bqHandler.Handle()` / `esHandler.Handle()`
- **Function:** `QueryAgentStream()` — sama dengan QueryAgent()
- **New helper (opsional):** `checkDataSourceAllowed(persona, dataSource string) error`

**Current resolvePersona signature:**
```go
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string)
```
**Option:** Extend return to include persona config:
```go
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string, config.PersonaConfig)
```
Atau lookup personas map langsung di QueryAgent.

---

#### `config/cortexai.example.json`
- **Section:** `personas.executive`
- **Change:**
  ```json
  "executive": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "system_prompt_style": "executive",
    "max_tokens": 2048,
    "excluded_tools": ["get_bigquery_sample_data"],
    "allowed_data_sources": ["bigquery"]
  }
  ```

---

## NOT Affected

| File | Alasan |
|------|--------|
| `internal/agent/elasticsearch_handler.go` | ES diblokir di routing level — handler tidak pernah dipanggil untuk persona yang restrict ES |
| `internal/agent/llm_pool.go` | Pool management tidak berubah |
| `internal/agent/system_prompts.go` | Prompt styles tidak berubah |
| `internal/agent/llm.go` | LLMRunner interface tidak berubah |
| `internal/models/user.go` | User model tidak berubah |
| `internal/service/user_store.go` | UserStore tidak berubah |
| `internal/middleware/` | Auth, RBAC, rate limiting tidak berubah |
| `internal/server/routes.go` | Route registration tidak berubah |

---

## Tool Names Reference

Tools yang saat ini tersedia di BigQueryHandler:

| Tool Name | Function |
|-----------|----------|
| `execute_bigquery_sql` | Execute SQL query |
| `get_bigquery_schema` | Get table schema |
| `get_bigquery_sample_data` | Get sample rows from table |
| `list_bigquery_datasets` | List available datasets |

Tool yang bisa di-exclude: semua nama di atas (string match).

---

## Data Source Identifiers

| Identifier | Handler |
|------------|---------|
| `"bigquery"` | BigQueryHandler |
| `"elasticsearch"` | ElasticsearchHandler |

Identifier ini digunakan di `allowed_data_sources` config field.

---

## Routing Logic (Current)

```
QueryAgent()
  → IntentRouter.Route(prompt) → "bigquery" | "elasticsearch"
  → if "bigquery" → bqHandler.Handle(...)
  → if "elasticsearch" → esHandler.Handle(...)
```

**After this change:**
```
QueryAgent()
  → resolvePersona(user) → (runner, promptStyle, personaConfig)
  → IntentRouter.Route(prompt) → "bigquery" | "elasticsearch"
  → checkDataSourceAllowed(personaConfig, routedSource) → error if not allowed
  → if "bigquery" → bqHandler.Handle(..., personaConfig.ExcludedTools)
  → if "elasticsearch" → esHandler.Handle(...)
```

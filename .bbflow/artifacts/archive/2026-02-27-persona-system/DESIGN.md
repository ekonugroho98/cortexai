# DESIGN — Persona System

**Spec ID:** persona-system

---

## File Structure

```
internal/
├── config/
│   └── config.go                    MODIFY — +PersonaConfig, +Config.Personas, +UserConfig.Persona
├── models/
│   └── user.go                      MODIFY — +User.Persona, +UserResponse.Persona, update ToResponse()
├── service/
│   └── user_store.go               MODIFY — +UserEntry.Persona, pass to User in NewUserStore()
├── agent/
│   ├── llm_pool.go                 CREATE — LLMPool struct
│   ├── system_prompts.go           CREATE — per-persona prompt constants + dispatcher functions
│   ├── bigquery_handler.go         MODIFY — export BaseSystemPrompt, refactor cache, Handle() params
│   └── elasticsearch_handler.go    MODIFY — export ESSystemPrompt, Handle() params
├── handler/
│   └── agent.go                    MODIFY — +llmPool, +personas, resolvePersona(), pass to handlers
└── server/
    └── routes.go                   MODIFY — build LLMPool, wire AgentHandler with pool+personas

config/
└── cortexai.example.json           MODIFY — add personas section, update users
```

---

## Data Models

### PersonaConfig (new, internal/config/config.go)
```go
type PersonaConfig struct {
    Provider          string `json:"provider"`
    Model             string `json:"model"`
    BaseURL           string `json:"base_url,omitempty"`
    SystemPromptStyle string `json:"system_prompt_style"`
    MaxTokens         int    `json:"max_tokens,omitempty"`
}
```

### Config changes
```go
type Config struct {
    // ... existing fields unchanged ...
    Squads   []SquadConfig              `json:"squads"`
    Personas map[string]PersonaConfig   `json:"personas"`   // NEW
    Users    []UserConfig               `json:"users"`
}

type UserConfig struct {
    // ... existing fields unchanged ...
    Persona string `json:"persona"`  // NEW: "" = "default"
}
```

### User changes (internal/models/user.go)
```go
type User struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Role    Role   `json:"role"`
    APIKey  string `json:"-"`
    SquadID string `json:"squad_id,omitempty"`
    Squad   *Squad `json:"-"`
    Persona string `json:"persona,omitempty"`  // NEW
}

type UserResponse struct {
    // ... existing ...
    Persona string `json:"persona,omitempty"`  // NEW
}
```

### UserEntry changes (internal/service/user_store.go)
```go
type UserEntry struct {
    ID      string
    Name    string
    Role    string
    APIKey  string
    SquadID string
    Persona string  // NEW
}
```

---

## Component Responsibilities

| Component | File | Responsibility | Change Type |
|-----------|------|---------------|-------------|
| `PersonaConfig` | config/config.go | Hold provider/model/style per persona | NEW struct |
| `LLMPool` | agent/llm_pool.go | Map provider:model → LLMRunner, fallback | NEW file |
| `SystemPromptStyle()` | agent/system_prompts.go | Dispatch to correct BQ prompt by style | NEW file |
| `ESSystemPromptStyle()` | agent/system_prompts.go | Dispatch to correct ES prompt by style | NEW file |
| `BaseSystemPrompt` | agent/bigquery_handler.go | Exported default BQ prompt constant | MODIFY (export) |
| `ESSystemPrompt` | agent/elasticsearch_handler.go | Exported default ES prompt constant | MODIFY (export) |
| `getSchemaSection()` | agent/bigquery_handler.go | Cache/fetch schema-only (no base prompt) | MODIFY (refactor) |
| `BigQueryHandler.Handle()` | agent/bigquery_handler.go | Accept runner + promptStyle params | MODIFY signature |
| `ElasticsearchHandler.Handle()` | agent/elasticsearch_handler.go | Accept runner + promptStyle params | MODIFY signature |
| `resolvePersona()` | handler/agent.go | Resolve user.Persona → (runner, style) | NEW method |
| `AgentHandler` | handler/agent.go | Add llmPool + personas fields | MODIFY struct |
| `setupRoutes()` | server/routes.go | Build LLMPool, wire all dependencies | MODIFY |

---

## Interface / Contract Changes

### BigQueryHandler.Handle() — signature change
```go
// BEFORE
func (h *BigQueryHandler) Handle(
    ctx context.Context,
    req *models.AgentRequest,
    apiKey string,
    allowedDatasets []string,
) (*models.AgentResponse, error)

// AFTER
func (h *BigQueryHandler) Handle(
    ctx context.Context,
    req *models.AgentRequest,
    apiKey string,
    allowedDatasets []string,
    runner LLMRunner,      // per-request runner from persona
    promptStyle string,    // "executive" | "technical" | "support" | ""
) (*models.AgentResponse, error)
```

Same change for `HandleStream()`.

### ElasticsearchHandler.Handle() — signature change
```go
// BEFORE
func (h *ElasticsearchHandler) Handle(
    ctx context.Context,
    req *models.AgentRequest,
    apiKey string,
    allowedPatterns []string,
) (*models.AgentResponse, error)

// AFTER
func (h *ElasticsearchHandler) Handle(
    ctx context.Context,
    req *models.AgentRequest,
    apiKey string,
    allowedPatterns []string,
    runner LLMRunner,
    promptStyle string,
) (*models.AgentResponse, error)
```

---

## Implementation Details

### TASK-05: bigquery_handler.go cache refactor

**Before (problematic — caches full prompt including base):**
```go
func (h *BigQueryHandler) buildSystemPrompt(ctx, datasetID) string {
    // cache stores: baseSystemPrompt + schema portion
    // → different personas get wrong cached base prompt
}
```

**After (cache schema-only, compose at runtime):**
```go
// getSchemaSection returns ONLY the schema portion (no base prompt)
// Cache key: datasetID — shared across all personas
func (h *BigQueryHandler) getSchemaSection(ctx context.Context, datasetID string) string {
    if datasetID == "" || h.bq == nil {
        return ""
    }
    if section, ok := h.schemaCache.get(datasetID); ok {
        return section  // returns schema section only
    }
    v, err, _ := h.schemaCache.sf.Do(datasetID, func() (interface{}, error) {
        if section, ok := h.schemaCache.get(datasetID); ok {
            return section, nil
        }
        tables, err := h.bq.ListTables(ctx, datasetID)
        if err != nil {
            return "", nil  // soft fail
        }
        var sb strings.Builder
        sb.WriteString("\n\n## Available Dataset: " + datasetID + "\n")
        // ... build schema section ...
        sb.WriteString("\nSince schemas are already provided above, ...")
        section := sb.String()
        h.schemaCache.set(datasetID, section)
        return section, nil
    })
    // ...
}

// In Handle():
basePrompt := SystemPromptStyle(promptStyle)   // from system_prompts.go
schemaSection := h.getSchemaSection(ctx, datasetID)
systemPrompt := basePrompt + schemaSection
resp, err = runner.Run(ctx, systemPrompt, req.Prompt, agentTools)
```

### TASK-08: resolvePersona() in handler/agent.go

```go
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string) {
    personaName := "default"
    if user != nil && user.Persona != "" {
        personaName = user.Persona
    }

    pc, ok := h.personas[personaName]
    if !ok {
        pc, ok = h.personas["default"]
        if !ok {
            return h.llmPool.Get(""), ""  // absolute fallback
        }
    }

    key := agent.PoolKey(pc.Provider, pc.Model)
    runner := h.llmPool.Get(key)
    if runner == nil {
        runner = h.llmPool.Get("")  // pool fallback
    }
    return runner, pc.SystemPromptStyle
}
```

---

## Error Handling Strategy

| Scenario | Behavior |
|----------|----------|
| User.Persona not in personas map | Fallback to "default" persona silently |
| "default" persona not in config | Fallback to pool's fallback runner |
| Pool has no runners at all | `llmPool.HasRunners()` = false → agentH not created → 503 |
| Persona provider has no API key | Skip registration, log warning at startup |
| runner nil after resolve | 503 "no AI model available" |

---

## Testing Strategy

| Test | File | Type |
|------|------|------|
| `TestLLMPool_Get` | llm_pool_test.go | unit |
| `TestLLMPool_Fallback` | llm_pool_test.go | unit |
| `TestLLMPool_Dedup` | llm_pool_test.go | unit |
| `TestLLMPool_HasRunners` | llm_pool_test.go | unit |
| `TestSystemPromptStyle` | system_prompts_test.go | unit |
| `TestESSystemPromptStyle` | system_prompts_test.go | unit |
| `TestGetSchemaSectionNoBasePrompt` | bigquery_handler_test.go | unit |
| `TestResolvePersona` | agent_test.go (or handler test) | unit |
| Regression: `go test ./...` | all packages | regression |

---

## Implementation Notes

1. Implement in dependency order: config → models → user_store → llm_pool → bq_handler+es_handler → system_prompts → handler/agent → routes → example config
2. Export constants BEFORE creating system_prompts.go (system_prompts.go references them)
3. `h.agent` field stays in BigQueryHandler/ElasticsearchHandler — used as constructor default, not in Handle()
4. Run `go build ./...` after each task to catch compile errors early
5. After TASK-09, run full `go test ./...` to verify no regressions

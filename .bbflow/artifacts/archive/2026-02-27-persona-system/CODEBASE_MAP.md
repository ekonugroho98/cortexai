# CODEBASE_MAP — Persona System

**Spec ID:** persona-system

---

## Affected Files

### CREATE — internal/agent/llm_pool.go
**Purpose:** LLMPool struct yang mengelola multiple LLMRunner instances, keyed by "provider:model"
**New types:** `LLMPool`
**New functions:** `NewLLMPool()`, `Register()`, `SetFallback()`, `Get()`, `HasRunners()`, `PoolKey()`
**Dependencies added:** `fmt`

### CREATE — internal/agent/system_prompts.go
**Purpose:** Per-persona system prompt constants dan dispatcher functions
**New functions:** `SystemPromptStyle(style string) string`, `ESSystemPromptStyle(style string) string`
**New constants:** `executiveSystemPrompt`, `technicalSystemPrompt`, `supportSystemPrompt`, `esExecutiveSystemPrompt`, `esSupportSystemPrompt`
**Depends on:** `BaseSystemPrompt` (exported from bigquery_handler.go), `ESSystemPrompt` (exported from elasticsearch_handler.go)

---

### MODIFY — internal/config/config.go
**Location:** After existing `SquadConfig` struct definition
**Changes:**
- Add `PersonaConfig` struct (new, before `Config`)
- Add `Personas map[string]PersonaConfig` to `Config` struct (after `Squads` field)
- Add `Persona string` to `UserConfig` struct
**No env override** for Personas — JSON config only

**Current `Config` relevant fields:**
```go
type Config struct {
    // ... existing fields ...
    Squads  []SquadConfig  `json:"squads"`
    Users   []UserConfig   `json:"users"`
    // ADD: Personas map[string]PersonaConfig `json:"personas"`
}

type UserConfig struct {
    // ... existing fields ...
    // ADD: Persona string `json:"persona"`
}
```

---

### MODIFY — internal/models/user.go
**Lines to change:**
- `User` struct: add `Persona string` field
- `UserResponse` struct: add `Persona string` field
- `ToResponse()` method: add `Persona: u.Persona` in return statement

**Current struct (from memory):**
```go
type User struct {
    ID      string
    Name    string
    Role    Role
    APIKey  string  // json:"-"
    SquadID string
    Squad   *Squad  // json:"-"
    // ADD: Persona string `json:"persona,omitempty"`
}
```

---

### MODIFY — internal/service/user_store.go
**Changes:**
- `UserEntry` struct: add `Persona string`
- `NewUserStore()`: set `Persona: ue.Persona` when building `models.User`

**Relevant section (NewUserStore):**
```go
u := &models.User{
    ID:      ue.ID,
    Name:    ue.Name,
    Role:    role,
    APIKey:  ue.APIKey,
    SquadID: ue.SquadID,
    Squad:   squad,
    // ADD: Persona: ue.Persona,
}
```

---

### MODIFY — internal/agent/bigquery_handler.go
**Key changes:**
1. Export constant: `baseSystemPrompt` → `BaseSystemPrompt`
2. Rename/refactor: `buildSystemPrompt()` → extract `getSchemaSection()` (cache schema-only, not full prompt)
3. `Handle()` signature: add `runner LLMRunner, promptStyle string` params
4. `Handle()` body: use `runner.Run()` instead of `h.agent.Run()`, compose `SystemPromptStyle(promptStyle) + getSchemaSection()`
5. `HandleStream()` signature: same additions

**Current Handle signature:**
```go
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string) (*models.AgentResponse, error)
```
**New Handle signature:**
```go
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string, runner LLMRunner, promptStyle string) (*models.AgentResponse, error)
```

---

### MODIFY — internal/agent/elasticsearch_handler.go
**Key changes:**
1. Export constant: `esSystemPrompt` → `ESSystemPrompt`
2. `Handle()` signature: add `runner LLMRunner, promptStyle string` params
3. `Handle()` body: use `runner.Run()` instead of `h.agent.Run()`, use `ESSystemPromptStyle(promptStyle)` instead of `esSystemPrompt`
4. `HandleStream()` signature: same additions

---

### MODIFY — internal/handler/agent.go
**Key changes:**
1. `AgentHandler` struct: add `llmPool *agent.LLMPool` and `personas map[string]config.PersonaConfig`
2. `NewAgentHandler()`: add `llmPool` and `personas` params
3. Add `resolvePersona(user *models.User) (agent.LLMRunner, string)` method
4. `QueryAgent()`: call `resolvePersona(user)` before routing, pass `runner, promptStyle` to handlers, add `resp.AgentMetadata["persona"] = personaName`
5. `QueryAgentStream()`: same persona resolution + pass to HandleStream

---

### MODIFY — internal/server/routes.go
**Key changes:**
1. Remove existing `var llmRunner agent.LLMRunner; switch cfg.LLMProvider {...}` block
2. Replace with: `llmPool := agent.NewLLMPool()` + loop over `cfg.Personas` to register runners
3. Backward compat: `if len(cfg.Personas) == 0` → build single fallback runner from legacy config
4. `handler.NewAgentHandler(...)` call: add `llmPool, cfg.Personas` params
5. `agentH = handler.NewAgentHandler(bqAgentH, esAgentH, router, llmPool, cfg.Personas)`
6. `if llmPool.HasRunners()` replaces `if llmRunner != nil`
7. Pass `llmPool.Get("")` as default runner to `BigQueryHandler` and `ElasticsearchHandler` constructors

---

### MODIFY — config/cortexai.example.json
**Add** `personas` object with 4 examples: default, executive, app_support, developer
**Update** `users` array: add `"persona"` field to each user entry

---

## Dependency Graph (Change Order)

```
config.go (PersonaConfig)
    ↓
models/user.go (User.Persona)
    ↓
service/user_store.go (UserEntry.Persona)
    ↓
agent/llm_pool.go [NEW]
    ↓
agent/system_prompts.go [NEW] ← depends on exported constants below
    ↓
agent/bigquery_handler.go (export BaseSystemPrompt, refactor cache, Handle params)
agent/elasticsearch_handler.go (export ESSystemPrompt, Handle params)
    ↓
handler/agent.go (resolvePersona, pass to handlers)
    ↓
server/routes.go (build LLMPool, wire everything)
    ↓
config/cortexai.example.json (example update)
```

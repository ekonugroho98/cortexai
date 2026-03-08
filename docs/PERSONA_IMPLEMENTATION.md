# Implementation Document: Persona System + Per-Persona AI Model Selection

## Overview

Tambahkan sistem **Persona** ke CortexAI. Persona menentukan:
1. **AI model** mana yang dipakai (Claude Opus, Claude Sonnet, GLM, DeepSeek)
2. **System prompt style** — executive (ringkas bisnis) vs technical (detail + SQL/logs)
3. **Available tools** — executive tidak perlu sample_data, developer dapat semuanya
4. **Response behavior** — max tokens, language preference

Persona berbeda dari Role. **Role** mengontrol akses (RBAC: admin/analyst/viewer), sedangkan **Persona** mengontrol perilaku AI. Seorang analyst bisa punya persona `executive` atau `developer`.

---

## Current Architecture (WAJIB dipahami sebelum implementasi)

### LLMRunner Interface (`internal/agent/llm.go`)
```go
type LLMRunner interface {
    Run(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool) (string, []string, string, error)
    RunWithEmit(ctx context.Context, systemPrompt, userPrompt string, agentTools []tools.Tool, emitFn EmitFn) (string, []string, string, error)
    Model() string
}
```

Dua implementasi sudah ada:
- `CortexAgent` — Anthropic SDK (Claude, GLM via Z.ai)
- `DeepSeekAgent` — OpenAI-compatible HTTP (DeepSeek)

### Sekarang: Satu LLMRunner global
Di `routes.go`, **satu** `LLMRunner` dibuat saat startup dan di-inject ke semua handler:
```go
var llmRunner agent.LLMRunner
switch cfg.LLMProvider {
case "deepseek":
    llmRunner = agent.NewDeepSeekAgent(...)
default:
    llmRunner = agent.NewCortexAgent(...)
}
// llmRunner dipakai oleh semua request
bqAgentH = agent.NewBigQueryHandler(llmRunner, bqSvc, ...)
esAgentH = agent.NewElasticsearchHandler(llmRunner, esSvc, ...)
```

### Yang harus berubah: Per-request LLMRunner selection
Setelah implementasi, flow menjadi:
```
Request masuk
  → Auth middleware inject User (sudah ada)
  → User.Persona field digunakan untuk lookup PersonaConfig
  → PersonaConfig menentukan: LLMRunner mana, system prompt style apa
  → Handler pakai LLMRunner + system prompt sesuai persona
```

---

## File Changes (Ordered)

### 1. `internal/config/config.go` — Tambah PersonaConfig

**Tambahkan struct baru** sebelum `Config`:
```go
// PersonaConfig defines AI behavior for a persona type.
type PersonaConfig struct {
    Provider         string `json:"provider"`           // "anthropic" | "deepseek" — key ke LLMRunner pool
    Model            string `json:"model"`              // model ID, e.g. "claude-sonnet-4-6", "glm-4.5-air"
    BaseURL          string `json:"base_url,omitempty"` // optional: override base URL untuk provider ini
    SystemPromptStyle string `json:"system_prompt_style"` // "executive" | "technical" | "support"
    MaxTokens        int    `json:"max_tokens,omitempty"` // optional: 0 = use default (4096)
}
```

**Tambahkan field di `Config` struct** (setelah `Squads`):
```go
Personas map[string]PersonaConfig `json:"personas"` // persona_name → config
```

**Tambahkan field di `UserConfig` struct**:
```go
Persona string `json:"persona"` // references key di Personas map; empty = "default"
```

**JANGAN tambah env override untuk personas** — terlalu complex untuk env var, config JSON saja.

---

### 2. `internal/models/user.go` — Tambah Persona field

**Tambah field di `User` struct**:
```go
type User struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Role    Role   `json:"role"`
    APIKey  string `json:"-"`
    SquadID string `json:"squad_id,omitempty"`
    Squad   *Squad `json:"-"`
    Persona string `json:"persona,omitempty"` // ← TAMBAH INI: "executive", "developer", dll. Empty = "default"
}
```

**Tambah field di `UserResponse` struct**:
```go
Persona string `json:"persona,omitempty"` // ← TAMBAH INI
```

**Update `ToResponse()`** — tambahkan `Persona: u.Persona` di return.

---

### 3. `internal/service/user_store.go` — Pass persona dari config ke User

**Tambah `Persona` field di `UserEntry`**:
```go
type UserEntry struct {
    ID      string
    Name    string
    Role    string
    APIKey  string
    SquadID string
    Persona string // ← TAMBAH
}
```

**Di `NewUserStore()`**, saat building User:
```go
u := &models.User{
    ID:      ue.ID,
    Name:    ue.Name,
    Role:    role,
    APIKey:  ue.APIKey,
    SquadID: ue.SquadID,
    Persona: ue.Persona, // ← TAMBAH
}
```

---

### 4. `internal/agent/llm_pool.go` — **FILE BARU**: LLM Runner Pool

Buat file baru `internal/agent/llm_pool.go`:

```go
package agent

import "fmt"

// LLMPool manages multiple LLMRunner instances keyed by "provider:model".
// Multiple personas can share the same LLMRunner if they use identical provider+model+baseURL.
type LLMPool struct {
    runners  map[string]LLMRunner // key = "provider:model" or "provider:model:baseURL"
    fallback LLMRunner            // default runner when persona not found
}

// NewLLMPool creates an empty pool.
func NewLLMPool() *LLMPool {
    return &LLMPool{
        runners: make(map[string]LLMRunner),
    }
}

// Register adds a runner to the pool.
func (p *LLMPool) Register(key string, runner LLMRunner) {
    p.runners[key] = runner
}

// SetFallback sets the default runner used when a key is not found.
func (p *LLMPool) SetFallback(runner LLMRunner) {
    p.fallback = runner
}

// Get returns the runner for the given key. Falls back to the default runner.
// Returns nil only if the key is not found AND no fallback is set.
func (p *LLMPool) Get(key string) LLMRunner {
    if r, ok := p.runners[key]; ok {
        return r
    }
    return p.fallback
}

// PoolKey generates the lookup key for a provider+model combination.
func PoolKey(provider, model string) string {
    return fmt.Sprintf("%s:%s", provider, model)
}
```

---

### 5. `internal/agent/system_prompts.go` — **FILE BARU**: Per-persona system prompts

Buat file baru `internal/agent/system_prompts.go`:

```go
package agent

// SystemPromptStyle returns the BigQuery system prompt for a given persona style.
// The style parameter comes from PersonaConfig.SystemPromptStyle.
// If style is empty or unknown, returns the default base prompt.
func SystemPromptStyle(style string) string {
    switch style {
    case "executive":
        return executiveSystemPrompt
    case "support":
        return supportSystemPrompt
    case "technical":
        return technicalSystemPrompt
    default:
        return baseSystemPrompt // from bigquery_handler.go
    }
}

// ESSystemPromptStyle returns the Elasticsearch system prompt for a given persona style.
func ESSystemPromptStyle(style string) string {
    switch style {
    case "executive":
        return esExecutiveSystemPrompt
    case "support":
        return esSupportSystemPrompt
    case "technical":
        return esSystemPrompt // from elasticsearch_handler.go
    default:
        return esSystemPrompt
    }
}

const executiveSystemPrompt = `You are CortexAI, a senior business intelligence analyst.

Your audience is C-level executives and senior management. Your task is to help them understand business metrics from BigQuery data.

RULES:
1. Generate only SELECT queries - never INSERT, UPDATE, DELETE, DROP, or DDL
2. Always add LIMIT clause (max 1000 rows) unless user specifies otherwise
3. Use fully qualified table names: ` + "`dataset.table`" + `
4. ALWAYS wrap your final SQL in a code block exactly like this:
` + "```sql" + `
SELECT ...
` + "```" + `
5. Execute the SQL exactly once after writing it
6. For JOIN queries: use get_bigquery_sample_data to verify join key values match before executing

RESPONSE STYLE:
- Provide concise, actionable business insights
- Lead with the KEY FINDING — what does the data mean for the business?
- Use percentages, trends, and comparisons (MoM, YoY, vs target)
- Avoid technical jargon — no mention of SQL syntax, JOINs, or query internals
- Format numbers with thousands separators for readability
- If results are tabular, summarize the top insights rather than listing raw rows
- Respond in the same language as the user's prompt`

const supportSystemPrompt = `You are CortexAI, a technical support specialist with expertise in application troubleshooting.

Your audience is app support engineers investigating production issues. Your task is to help them find relevant data in BigQuery for incident analysis.

RULES:
1. Generate only SELECT queries - never INSERT, UPDATE, DELETE, DROP, or DDL
2. Always add LIMIT clause (max 1000 rows) unless user specifies otherwise
3. Use fully qualified table names: ` + "`dataset.table`" + `
4. ALWAYS wrap your final SQL in a code block exactly like this:
` + "```sql" + `
SELECT ...
` + "```" + `
5. Execute the SQL exactly once after writing it
6. For JOIN queries: use get_bigquery_sample_data to verify join key values match before executing

RESPONSE STYLE:
- Focus on finding the root cause of issues
- Include relevant timestamps, error codes, and transaction IDs
- Show step-by-step investigation path
- Highlight anomalies in the data
- Suggest next investigation steps if the initial query doesn't reveal the issue
- Respond in the same language as the user's prompt`

const technicalSystemPrompt = `You are CortexAI, an expert data analyst and developer assistant with deep knowledge of BigQuery SQL.

Your audience is developers and data engineers. Your task is to help them query BigQuery data with precision and efficiency.

RULES:
1. Generate only SELECT queries - never INSERT, UPDATE, DELETE, DROP, or DDL
2. Always add LIMIT clause (max 1000 rows) unless user specifies otherwise
3. Use fully qualified table names: ` + "`dataset.table`" + `
4. ALWAYS wrap your final SQL in a code block exactly like this:
` + "```sql" + `
SELECT ...
` + "```" + `
5. Execute the SQL exactly once after writing it
6. Explain results in plain language
7. For JOIN queries: use get_bigquery_sample_data to verify join key values match before executing

RESPONSE STYLE:
- Be thorough and technical — show your work
- Include the full SQL query with inline comments for complex logic
- Explain query performance implications (scanned bytes, partitioning, clustering)
- Mention potential optimizations
- If data looks suspicious, flag it and explain why
- Respond in the same language as the user's prompt`

const esExecutiveSystemPrompt = `You are CortexAI, a senior business intelligence analyst with Elasticsearch expertise.

Your audience is C-level executives. Help them understand operational metrics from log and event data.

RULES:
1. Always use list_elasticsearch_indices first to discover available indices
2. Build precise, focused queries - never search all documents without filters
3. Use the elasticsearch_search tool to execute searches
4. Maximum 100 results per search

RESPONSE STYLE:
- Lead with business impact — what does this data mean?
- Summarize patterns and trends rather than showing raw log entries
- Use percentages and comparisons for context
- Avoid technical ES jargon (shards, mappings, etc.)
- Respond in the same language as the user's prompt`

const esSupportSystemPrompt = `You are CortexAI, a technical support specialist with Elasticsearch and log analysis expertise.

Your audience is app support engineers investigating production issues. Help them find relevant logs and events.

RULES:
1. Always use list_elasticsearch_indices first to discover available indices
2. Build precise, focused queries - never search all documents without filters
3. Use the elasticsearch_search tool to execute searches
4. Focus on the specific identifier/time range provided by the user
5. Maximum 100 results per search

RESPONSE STYLE:
- Focus on finding root cause in logs
- Include timestamps, error codes, and stack traces when found
- Show investigation steps clearly
- Highlight anomalous patterns in log data
- Suggest next steps if initial search doesn't reveal the issue
- Respond in the same language as the user's prompt`
```

**PENTING**: `baseSystemPrompt` dan `esSystemPrompt` sudah ada masing-masing di `bigquery_handler.go` dan `elasticsearch_handler.go`. Jangan duplikasi — `SystemPromptStyle` function harus reference constant yang sudah ada. Untuk melakukan ini, EXPORT constant yang sudah ada:
- Di `bigquery_handler.go`: ubah `baseSystemPrompt` → `BaseSystemPrompt` (exported)
- Di `elasticsearch_handler.go`: ubah `esSystemPrompt` → `ESSystemPrompt` (exported)
- Di `system_prompts.go`: gunakan `BaseSystemPrompt` dan `ESSystemPrompt` untuk case default

---

### 6. `internal/agent/bigquery_handler.go` — Terima persona-aware LLMRunner

**Perubahan di `Handle()` method**:

Saat ini signature:
```go
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string) (*models.AgentResponse, error)
```

Ubah menjadi:
```go
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string, runner LLMRunner, promptStyle string) (*models.AgentResponse, error)
```

**Perubahan di dalam `Handle()`**:
1. Ganti `h.agent.Model()` → `runner.Model()`
2. Ganti `h.agent.Run(...)` → `runner.Run(...)`
3. Ganti `systemPrompt := h.buildSystemPrompt(ctx, datasetID)` menjadi:
   ```go
   basePrompt := SystemPromptStyle(promptStyle) // dari system_prompts.go
   systemPrompt := h.buildSystemPromptFrom(ctx, datasetID, basePrompt)
   ```

**Buat method baru `buildSystemPromptFrom()`** — copy dari `buildSystemPrompt()` tapi terima base prompt sebagai parameter:
```go
func (h *BigQueryHandler) buildSystemPromptFrom(ctx context.Context, datasetID, base string) string {
    if datasetID == "" || h.bq == nil {
        return base
    }
    // cache key harus include style agar beda persona tidak share cache yg salah
    cacheKey := datasetID + ":" + base[:min(50, len(base))] // atau hash
    // ... sisanya sama dengan buildSystemPrompt, tapi pakai `base` bukan `baseSystemPrompt`
}
```

**ALTERNATIF LEBIH SEDERHANA** (DIREKOMENDASIKAN): Karena schema portion yang di-cache itu sama untuk semua persona (hanya table list), pisahkan cache jadi 2 bagian:
- Cache hanya menyimpan **schema portion** (table definitions) — BUKAN base prompt
- Saat build final prompt: gabungkan `SystemPromptStyle(style)` + cached schema portion

Ini berarti refactor `buildSystemPrompt()`:
```go
// getSchemaSection returns the cached schema portion for a dataset (tanpa base prompt).
func (h *BigQueryHandler) getSchemaSection(ctx context.Context, datasetID string) string {
    // sama seperti buildSystemPrompt tapi TIDAK prepend baseSystemPrompt
    // hanya return "\n\n## Available Dataset: ...\n..." portion
}

// Di Handle():
basePrompt := SystemPromptStyle(promptStyle)
schemaSection := h.getSchemaSection(ctx, datasetID)
systemPrompt := basePrompt + schemaSection
```

**Perubahan yang sama untuk `HandleStream()`**:
```go
func (h *BigQueryHandler) HandleStream(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string, runner LLMRunner, promptStyle string, emitFn func(event string, data interface{}))
```
Ganti semua `h.agent.*` → `runner.*` dan pakai `promptStyle` untuk system prompt.

---

### 7. `internal/agent/elasticsearch_handler.go` — Terima persona-aware LLMRunner

**Perubahan di `Handle()` signature**:
```go
func (h *ElasticsearchHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedPatterns []string, runner LLMRunner, promptStyle string) (*models.AgentResponse, error)
```

**Perubahan di dalam `Handle()`**:
1. Ganti `h.agent.Model()` → `runner.Model()`
2. Ganti `h.agent.Run(...)` → `runner.Run(...)`
3. Ganti `esSystemPrompt` → `ESSystemPromptStyle(promptStyle)`

---

### 8. `internal/handler/agent.go` — Resolve persona per request

**Tambah field di `AgentHandler` struct**:
```go
type AgentHandler struct {
    bqHandler *agent.BigQueryHandler
    esHandler *agent.ElasticsearchHandler
    router    *service.IntentRouter
    llmPool   *agent.LLMPool                    // ← TAMBAH
    personas  map[string]config.PersonaConfig    // ← TAMBAH
}
```

**Update constructor**:
```go
func NewAgentHandler(
    bqHandler *agent.BigQueryHandler,
    esHandler *agent.ElasticsearchHandler,
    router *service.IntentRouter,
    llmPool *agent.LLMPool,
    personas map[string]config.PersonaConfig,
) *AgentHandler
```

**Tambah helper function untuk resolve persona**:
```go
// resolvePersona returns the LLMRunner and system prompt style for the given user.
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string) {
    personaName := "default"
    if user != nil && user.Persona != "" {
        personaName = user.Persona
    }

    pc, ok := h.personas[personaName]
    if !ok {
        // Fallback ke default persona jika ada
        pc, ok = h.personas["default"]
        if !ok {
            // Absolute fallback: pool's fallback runner, no special style
            return h.llmPool.Get(""), ""
        }
    }

    key := agent.PoolKey(pc.Provider, pc.Model)
    runner := h.llmPool.Get(key)
    if runner == nil {
        // Provider+model combo tidak ter-register — use fallback
        runner = h.llmPool.Get("")
    }
    return runner, pc.SystemPromptStyle
}
```

**Update `QueryAgent()` method**:
```go
func (h *AgentHandler) QueryAgent(w http.ResponseWriter, r *http.Request) {
    // ... (existing validation code stays the same) ...

    // Resolve persona for this user
    user, _ := middleware.GetCurrentUser(r.Context())
    runner, promptStyle := h.resolvePersona(user)
    if runner == nil {
        models.WriteError(w, http.StatusServiceUnavailable, "no AI model available")
        return
    }

    // ... (existing routing code stays the same) ...

    switch source {
    case service.DataSourceElasticsearch:
        if h.esHandler == nil {
            models.WriteError(w, http.StatusServiceUnavailable, "Elasticsearch is not configured")
            return
        }
        resp, err = h.esHandler.Handle(r.Context(), &req, apiKey, allowedESPatterns, runner, promptStyle)
    default:
        if h.bqHandler == nil {
            models.WriteError(w, http.StatusServiceUnavailable, "BigQuery is not configured")
            return
        }
        resp, err = h.bqHandler.Handle(r.Context(), &req, apiKey, allowedDatasets, runner, promptStyle)
    }

    // ... (rest stays the same) ...
}
```

**Update `QueryAgentStream()` method** — tambah `runner, promptStyle` juga:
```go
// ... setelah resolve persona:
h.bqHandler.HandleStream(r.Context(), &req, apiKey, allowedDatasets, runner, promptStyle, emitSSE)
```

**Tambah persona info di response metadata**:
```go
// Setelah resolve persona, sebelum switch:
metadata["persona"] = personaName   // e.g. "executive"
metadata["model"] = runner.Model()  // e.g. "claude-sonnet-4-6"
```
Catatan: metadata di-set di BigQueryHandler/ElasticsearchHandler, bukan di AgentHandler. Jadi tambahkan `persona` field di `AgentMetadata` yang sudah di-set oleh handler, atau pass persona name ke handler juga. **Pendekatan paling bersih**: tambah ke `resp.AgentMetadata` setelah handler return:
```go
resp.AgentMetadata["persona"] = personaName
```

---

### 9. `internal/server/routes.go` — Build LLM Pool dari config

**Ganti factory logic yang ada**:

SEBELUM (hapus kode ini):
```go
var llmRunner agent.LLMRunner
switch cfg.LLMProvider {
case "deepseek":
    if cfg.DeepSeekAPIKey != "" {
        llmRunner = agent.NewDeepSeekAgent(...)
    }
default:
    if cfg.AnthropicAPIKey != "" {
        llmRunner = agent.NewCortexAgent(...)
    }
}
```

SESUDAH (ganti dengan):
```go
// ─── LLM Pool ─────────────────────────────────────────────────────────────
llmPool := agent.NewLLMPool()

// Register runners for all unique provider+model combinations from personas
registered := make(map[string]bool) // track registered keys to avoid duplicates
for name, pc := range cfg.Personas {
    key := agent.PoolKey(pc.Provider, pc.Model)
    if registered[key] {
        continue
    }

    var runner agent.LLMRunner
    switch pc.Provider {
    case "deepseek":
        apiKey := cfg.DeepSeekAPIKey
        baseURL := pc.BaseURL
        if baseURL == "" {
            baseURL = cfg.DeepSeekBaseURL
        }
        if apiKey != "" {
            runner = agent.NewDeepSeekAgent(apiKey, pc.Model, baseURL)
        }
    default: // "anthropic"
        apiKey := cfg.AnthropicAPIKey
        baseURL := pc.BaseURL
        if baseURL == "" {
            baseURL = cfg.AnthropicBaseURL
        }
        if apiKey != "" {
            runner = agent.NewCortexAgent(apiKey, pc.Model, baseURL)
        }
    }

    if runner != nil {
        llmPool.Register(key, runner)
        registered[key] = true
        log.Info().Str("persona", name).Str("provider", pc.Provider).Str("model", pc.Model).Msg("LLM runner registered")
    }
}

// Fallback: jika tidak ada personas di config, buat runner dari legacy config
if len(cfg.Personas) == 0 {
    var fallbackRunner agent.LLMRunner
    switch cfg.LLMProvider {
    case "deepseek":
        if cfg.DeepSeekAPIKey != "" {
            model := cfg.ModelList["deepseek"]
            fallbackRunner = agent.NewDeepSeekAgent(cfg.DeepSeekAPIKey, model, cfg.DeepSeekBaseURL)
            log.Info().Str("provider", "deepseek").Str("model", model).Msg("fallback LLM runner")
        }
    default:
        if cfg.AnthropicAPIKey != "" {
            model := cfg.ModelList["anthropic"]
            fallbackRunner = agent.NewCortexAgent(cfg.AnthropicAPIKey, model, cfg.AnthropicBaseURL)
            log.Info().Str("provider", "anthropic").Str("model", model).Msg("fallback LLM runner")
        }
    }
    if fallbackRunner != nil {
        llmPool.SetFallback(fallbackRunner)
    }
} else {
    // Set default persona's runner as fallback
    if defaultPC, ok := cfg.Personas["default"]; ok {
        key := agent.PoolKey(defaultPC.Provider, defaultPC.Model)
        if r := llmPool.Get(key); r != nil {
            llmPool.SetFallback(r)
        }
    }
}

hasLLM := llmPool.Get("") != nil || len(llmPool.runners) > 0
// Catatan: llmPool.runners is unexported, jadi cek via llmPool.Get("") != nil
// Alternatif: tambah method `func (p *LLMPool) HasRunners() bool` di llm_pool.go
```

**PENTING**: Tambah method `HasRunners()` di `llm_pool.go`:
```go
func (p *LLMPool) HasRunners() bool {
    return len(p.runners) > 0 || p.fallback != nil
}
```

**Update handler creation**:
```go
if llmPool.HasRunners() {
    var bqAgentH *agent.BigQueryHandler
    var esAgentH *agent.ElasticsearchHandler

    if bqSvc != nil {
        // BigQueryHandler TETAP menerima satu LLMRunner di struct untuk schema caching
        // (tidak berubah, karena schema cache tidak tergantung model).
        // Per-request runner di-pass via Handle() parameter.
        // Gunakan fallback runner sebagai default di constructor.
        defaultRunner := llmPool.Get("")
        if defaultRunner == nil {
            // Ambil runner pertama yang ada
            for _, r := range ... // atau tambah method FirstRunner() di pool
        }
        bqAgentH = agent.NewBigQueryHandler(defaultRunner, bqSvc, piiDetector, promptVal, sqlVal, costTracker, dataMasker, auditLogger)
        cacheH = handler.NewCacheHandler(bqAgentH)
    }
    if esSvc != nil {
        esAgentH = agent.NewElasticsearchHandler(llmPool.Get(""), esSvc, piiDetector, promptVal, esPromptVal, auditLogger)
    }

    agentH = handler.NewAgentHandler(bqAgentH, esAgentH, router, llmPool, cfg.Personas)
}
```

**CATATAN ARSITEKTUR PENTING**:
`BigQueryHandler` dan `ElasticsearchHandler` struct masih menyimpan `agent LLMRunner` field. Ini dipakai sebagai **default** runner. Tapi per-request, `Handle()` sekarang menerima `runner LLMRunner` parameter yang override default. Ada 2 opsi:

**Opsi A (DIREKOMENDASIKAN — minimal change)**: Jangan hapus `agent` field dari struct. Biarkan sebagai default. Tapi `Handle()` dan `HandleStream()` sekarang menerima `runner LLMRunner` parameter dan pakai itu, bukan `h.agent`.

**Opsi B (cleaner tapi lebih banyak perubahan)**: Hapus `agent` field dari struct. Semua LLMRunner akses hanya via parameter.

Pilih **Opsi A** — lebih aman, backwards-compatible, dan constructor tidak berubah signature.

---

### 10. `internal/server/routes.go` — Pass persona data ke UserEntry

Di bagian yang convert config → service entries, tambah Persona:
```go
userEntries := make([]service.UserEntry, len(cfg.Users))
for i, u := range cfg.Users {
    userEntries[i] = service.UserEntry{
        ID:      u.ID,
        Name:    u.Name,
        Role:    u.Role,
        APIKey:  u.APIKey,
        SquadID: u.SquadID,
        Persona: u.Persona, // ← TAMBAH
    }
}
```

---

### 11. `config/cortexai.example.json` — Update example config

Tambah `personas` section dan update `users` dengan persona field:

```json
{
  "personas": {
    "default": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-6",
      "system_prompt_style": "technical",
      "max_tokens": 4096
    },
    "executive": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-6",
      "system_prompt_style": "executive",
      "max_tokens": 4096
    },
    "app_support": {
      "provider": "anthropic",
      "model": "glm-4.5-air",
      "base_url": "https://open.bigmodel.cn/api/anthropic/",
      "system_prompt_style": "support",
      "max_tokens": 8192
    },
    "developer": {
      "provider": "deepseek",
      "model": "deepseek-chat",
      "system_prompt_style": "technical",
      "max_tokens": 8192
    }
  },
  "users": [
    { "id": "u1", "name": "Alice", "role": "admin",   "squad_id": "",             "persona": "executive",   "api_key": "sk-alice-replace-me" },
    { "id": "u2", "name": "Bob",   "role": "analyst",  "squad_id": "payment",      "persona": "developer",   "api_key": "sk-bob-replace-me" },
    { "id": "u3", "name": "Carol", "role": "analyst",  "squad_id": "user-platform","persona": "app_support", "api_key": "sk-carol-replace-me" },
    { "id": "u4", "name": "Dave",  "role": "viewer",   "squad_id": "payment",      "persona": "default",     "api_key": "sk-dave-replace-me" }
  ]
}
```

---

## Dependency Order (Implementasi berurutan)

```
1. config.go           — PersonaConfig struct, Config.Personas, UserConfig.Persona
2. models/user.go      — User.Persona, UserResponse.Persona, ToResponse update
3. service/user_store.go — UserEntry.Persona, pass ke User
4. agent/llm_pool.go   — FILE BARU: LLMPool
5. agent/system_prompts.go — FILE BARU: per-persona prompts
6. agent/bigquery_handler.go — export BaseSystemPrompt, refactor buildSystemPrompt,
                               Handle/HandleStream accept runner+promptStyle params
7. agent/elasticsearch_handler.go — export ESSystemPrompt, Handle accept runner+promptStyle
8. handler/agent.go    — resolvePersona(), pass runner+promptStyle to handlers
9. server/routes.go    — build LLMPool, pass personas to AgentHandler
10. cortexai.example.json — example config update
```

---

## Verification

```bash
# Must compile clean
go build ./...

# All existing tests pass (no regressions)
go test ./...

# Specific test areas
go test ./internal/agent/ -v       # BigQuery handler + DeepSeek tests
go test ./internal/middleware/ -v   # Auth + RBAC tests still pass
```

---

## Backward Compatibility

1. **No personas in config** → fallback ke legacy `llm_provider` + `model_list` behavior. Semua request pakai satu LLMRunner global, base system prompt default.

2. **User tanpa persona field** → persona = "default". Jika "default" persona tidak ada di config → pakai pool fallback runner + default system prompt.

3. **Legacy `api_keys` (tanpa user profile)** → viewer role, no squad, no persona → fallback runner + default prompt. Tidak ada breaking change.

4. **Existing handler tests** — `BigQueryHandler` dan `ElasticsearchHandler` constructor TIDAK berubah. Yang berubah hanya signature `Handle()` dan `HandleStream()`. Test yang memanggil `Handle()` perlu update parameter, tapi test yang ada sekarang (extractSQL tests) tidak terpengaruh.

---

## Edge Cases yang Harus di-Handle

1. **Persona references provider tanpa API key**: Skip registration, log warning. Request dari user dengan persona ini akan fallback ke default runner.

2. **Dua persona pakai model yang sama**: Hanya buat 1 `LLMRunner` instance, share via pool key. Hemat memory + connection pooling.

3. **User.Persona references persona yang tidak ada di config**: Fallback ke "default" persona, lalu ke pool fallback runner.

4. **Empty personas map + empty legacy config**: `llmPool.HasRunners()` returns false → `agentH` = nil → `/query-agent` returns 503. Ini sudah di-handle oleh existing route nil check.

---

## Response Example (setelah implementasi)

Request dari user Alice (persona: executive):
```json
POST /api/v1/query-agent
{
  "prompt": "berapa total revenue bulan ini?",
  "dataset_id": "payment_datalake_01"
}

// Response
{
  "status": "success",
  "prompt": "berapa total revenue bulan ini?",
  "generated_sql": "SELECT SUM(amount) as total_revenue ...",
  "execution_result": { ... },
  "agent_metadata": {
    "data_source": "bigquery",
    "model": "claude-sonnet-4-6",
    "persona": "executive",
    "routing_confidence": 1.0,
    "tools_used": ["execute_bigquery_sql"]
  },
  "reasoning": "Total revenue bulan ini adalah Rp 12.5 miliar, naik 8.3% dari bulan lalu..."
}
```

Request dari user Bob (persona: developer):
```json
POST /api/v1/query-agent
{
  "prompt": "cek transaksi gagal dengan error code 5001",
  "dataset_id": "payment_datalake_01"
}

// Response
{
  "status": "success",
  "agent_metadata": {
    "model": "deepseek-chat",
    "persona": "developer",
    ...
  },
  "reasoning": "Found 23 failed transactions with error_code=5001 in the last 24h...\n\n```sql\nSELECT ... -- detailed query with comments\n```\n\nThe error pattern suggests..."
}
```

---

## Files Summary

| Action | File | Perubahan |
|--------|------|-----------|
| MODIFY | `internal/config/config.go` | +PersonaConfig, +Config.Personas, +UserConfig.Persona |
| MODIFY | `internal/models/user.go` | +User.Persona, +UserResponse.Persona |
| MODIFY | `internal/service/user_store.go` | +UserEntry.Persona, pass to User |
| CREATE | `internal/agent/llm_pool.go` | LLMPool struct + methods |
| CREATE | `internal/agent/system_prompts.go` | Per-persona system prompts |
| MODIFY | `internal/agent/bigquery_handler.go` | Export BaseSystemPrompt, refactor cache, Handle/HandleStream accept runner+style |
| MODIFY | `internal/agent/elasticsearch_handler.go` | Export ESSystemPrompt, Handle accept runner+style |
| MODIFY | `internal/handler/agent.go` | +llmPool, +personas, resolvePersona(), pass to handlers |
| MODIFY | `internal/server/routes.go` | Build LLMPool, pass to AgentHandler |
| MODIFY | `config/cortexai.example.json` | Add personas section, update users |

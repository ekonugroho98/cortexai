# ARCHITECTURE — Persona System

**Spec ID:** persona-system
**Mode:** BROWNFIELD
**Complexity:** COMPLEX

---

## 1. System Context

Persona System menambahkan lapisan resolusi antara Auth middleware (yang meng-inject User) dan Agent handlers (BigQueryHandler/ElasticsearchHandler). Layer baru ini — `resolvePersona()` di AgentHandler — memilih LLMRunner dan system prompt style yang tepat berdasarkan `user.Persona`.

---

## 2. BEFORE vs AFTER

### BEFORE (Single Global Runner)
```
startup
  └─ routes.go creates ONE LLMRunner (CortexAgent or DeepSeekAgent)
       └─ injected into BigQueryHandler + ElasticsearchHandler constructor

request
  ├─ Auth middleware → injects User (with Role, Squad)
  └─ AgentHandler
       ├─ resolves source (BQ or ES)
       ├─ BigQueryHandler.Handle(ctx, req, apiKey, datasets)
       │    └─ uses h.agent (the one global runner)
       │    └─ uses baseSystemPrompt (hardcoded)
       └─ ElasticsearchHandler.Handle(ctx, req, apiKey, patterns)
             └─ uses h.agent
             └─ uses esSystemPrompt (hardcoded)
```

### AFTER (Per-Persona Runner Selection)
```
startup
  └─ routes.go builds LLMPool
       ├─ iterates cfg.Personas → Register(provider:model → runner)
       ├─ deduplicates: 2 personas with same provider+model = 1 runner
       └─ sets fallback runner (from "default" persona or legacy config)

request
  ├─ Auth middleware → injects User (with Role, Squad, Persona)
  └─ AgentHandler
       ├─ resolvePersona(user) → (runner LLMRunner, promptStyle string)
       │    ├─ user.Persona → lookup in h.personas map
       │    ├─ → PoolKey(provider, model) → llmPool.Get(key)
       │    └─ fallback chain: persona → "default" → pool.fallback
       ├─ BigQueryHandler.Handle(ctx, req, apiKey, datasets, runner, promptStyle)
       │    ├─ uses runner (per-request, from persona)
       │    ├─ basePrompt = SystemPromptStyle(promptStyle)
       │    ├─ schemaSection = getSchemaSection(ctx, datasetID) [cached, shared]
       │    └─ systemPrompt = basePrompt + schemaSection
       └─ ElasticsearchHandler.Handle(ctx, req, apiKey, patterns, runner, promptStyle)
             ├─ uses runner
             └─ systemPrompt = ESSystemPromptStyle(promptStyle)
```

---

## 3. Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│ Config (cortexai.json)                                              │
│   personas:                                                         │
│     "executive"   → {provider: anthropic, model: claude-sonnet-4-6}│
│     "developer"   → {provider: deepseek,  model: deepseek-chat}    │
│     "app_support" → {provider: anthropic, model: glm-4.5-air}      │
│   users:                                                            │
│     Alice → persona: "executive"                                    │
│     Bob   → persona: "developer"                                    │
└─────────────────────┬───────────────────────────────────────────────┘
                      │ startup
                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│ LLMPool (agent/llm_pool.go)                                         │
│   runners map:                                                      │
│     "anthropic:claude-sonnet-4-6" → CortexAgent (GLM/Claude)       │
│     "deepseek:deepseek-chat"      → DeepSeekAgent                  │
│   fallback → CortexAgent (from "default" persona)                  │
└─────────────────────┬───────────────────────────────────────────────┘
                      │ Get(key)
                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│ AgentHandler (handler/agent.go)                                     │
│   resolvePersona(user) → (runner, promptStyle)                     │
│     user.Persona="executive"                                        │
│     → personas["executive"].Provider="anthropic", Model="claude-…" │
│     → PoolKey="anthropic:claude-sonnet-4-6"                         │
│     → llmPool.Get("anthropic:claude-sonnet-4-6")                   │
│     → runner=CortexAgent, promptStyle="executive"                  │
└──────────┬──────────────────────┬──────────────────────────────────┘
           │                      │
           ▼                      ▼
┌──────────────────┐   ┌──────────────────────────────────────────┐
│ BigQueryHandler  │   │ ElasticsearchHandler                     │
│ Handle(runner,   │   │ Handle(runner, promptStyle, ...)         │
│  promptStyle)    │   │                                          │
│                  │   │ systemPrompt =                           │
│ basePrompt =     │   │   ESSystemPromptStyle(promptStyle)       │
│  SystemPromptStyle│   │ runner.Run(systemPrompt, ...)           │
│  (promptStyle)   │   └──────────────────────────────────────────┘
│ schema = getSchema│
│  Section(dataset) │   ← cache shared across all personas
│ systemPrompt =   │
│  basePrompt+schema│
│ runner.Run(...)  │
└──────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ SystemPrompts (agent/system_prompts.go)                             │
│   SystemPromptStyle("executive") → executiveSystemPrompt           │
│   SystemPromptStyle("technical") → technicalSystemPrompt           │
│   SystemPromptStyle("support")   → supportSystemPrompt             │
│   SystemPromptStyle("")          → BaseSystemPrompt (exported)     │
│   ESSystemPromptStyle(...)       → ES variants                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. Schema Cache — Refactored

```
BEFORE:
  cache[datasetID] = "You are CortexAI..." + "\n\n## Dataset..." + schemas
  Problem: Different personas need different base prompt — cache would be wrong!

AFTER:
  cache[datasetID] = "\n\n## Available Dataset: ..." + schemas  ← schema section ONLY
  At request time: systemPrompt = SystemPromptStyle(promptStyle) + getSchemaSection(datasetID)
  Benefit: Cache shared across ALL personas for the same dataset
```

---

## 5. Key Architecture Decisions

### Decision 1: Opsi A — Keep `h.agent` field in handler structs
Handler structs (`BigQueryHandler`, `ElasticsearchHandler`) tetap menyimpan `agent LLMRunner` sebagai default. Constructor tidak berubah. `Handle()` dan `HandleStream()` menerima `runner` sebagai parameter yang override `h.agent`. Ini adalah perubahan minimal dengan zero risk ke constructor callers.

**Trade-off:** Ada `h.agent` field yang tidak lagi dipakai di `Handle()`. Bisa di-remove di refactor mendatang (Opsi B).

### Decision 2: LLMPool keyed by `"provider:model"`
Dua persona dengan provider+model yang sama berbagi satu runner instance. Ini hemat memory dan connection pool HTTP. Key tidak menyertakan `base_url` — jika base_url berbeda untuk model yang sama, user harus gunakan nama model yang berbeda sebagai workaround (acceptable trade-off).

### Decision 3: Fallback chain
```
user.Persona → personas["user_persona"] → personas["default"] → pool.fallback → nil (503)
```
Ini robust terhadap misconfiguration tanpa panic.

### Decision 4: Schema cache stores schema-only
Memisahkan base prompt dari schema portion memungkinkan cache sharing across personas. Overhead composing at runtime minimal (string concatenation per request).

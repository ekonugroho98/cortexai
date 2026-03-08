# REQUIREMENTS — Persona System

**Spec ID:** persona-system
**Created:** 2026-02-27

---

## Data Model

### PersonaConfig (config.go)
```go
type PersonaConfig struct {
    Provider          string `json:"provider"`            // "anthropic" | "deepseek"
    Model             string `json:"model"`               // e.g. "claude-sonnet-4-6", "glm-4.5-air"
    BaseURL           string `json:"base_url,omitempty"`  // optional override
    SystemPromptStyle string `json:"system_prompt_style"` // "executive" | "technical" | "support"
    MaxTokens         int    `json:"max_tokens,omitempty"` // 0 = use default (4096)
}
```

### Config.Personas (config.go)
```go
Personas map[string]PersonaConfig `json:"personas"` // key = persona name
```

### UserConfig.Persona (config.go)
```go
Persona string `json:"persona"` // references key in Personas map; empty = "default"
```

### User.Persona (models/user.go)
```go
Persona string `json:"persona,omitempty"` // "executive", "developer", etc.
```

---

## LLMPool Interface Contract

```go
// Pool key format
PoolKey(provider, model string) string // returns "provider:model"

// LLMPool methods
NewLLMPool() *LLMPool
Register(key string, runner LLMRunner)
SetFallback(runner LLMRunner)
Get(key string) LLMRunner  // returns fallback if key not found, nil if no fallback
HasRunners() bool           // true if len(runners) > 0 OR fallback != nil
```

---

## System Prompt Styles

### executive
- Audience: C-level executives, senior management
- Tone: Concise, business-focused, no technical jargon
- Format: Lead with KEY FINDING, use percentages/trends/comparisons, summarize rather than listing raw rows
- Language: Match user's prompt language

### technical
- Audience: Developers and data engineers
- Tone: Thorough, detailed, SQL-forward
- Format: Show full SQL with inline comments, explain performance implications, mention optimizations, flag data anomalies
- Language: Match user's prompt language

### support
- Audience: App support engineers investigating production issues
- Tone: Investigative, step-by-step
- Format: Focus on root cause, include timestamps/error codes/transaction IDs, suggest next steps
- Language: Match user's prompt language

---

## resolvePersona() Logic

```
Input: user *models.User
Output: (runner LLMRunner, promptStyle string)

1. personaName = "default"
2. IF user != nil AND user.Persona != "" → personaName = user.Persona
3. pc = h.personas[personaName]
4. IF not found → pc = h.personas["default"]
5. IF still not found → return h.llmPool.Get(""), ""
6. key = PoolKey(pc.Provider, pc.Model)
7. runner = h.llmPool.Get(key)
8. IF runner == nil → runner = h.llmPool.Get("")  // absolute fallback
9. return runner, pc.SystemPromptStyle
```

---

## Schema Cache Refactor

**Before (problematic):**
```
cache[datasetID] = baseSystemPrompt + "\n\n## Available Dataset..." + schema
// Different personas would read the same cached string with WRONG base prompt
```

**After (correct):**
```
cache[datasetID] = "\n\n## Available Dataset..." + schema  // schema portion ONLY

// At request time:
basePrompt := SystemPromptStyle(promptStyle)
schemaSection := h.getSchemaSection(ctx, datasetID)  // from cache
systemPrompt := basePrompt + schemaSection            // compose at runtime
```

Cache key remains `datasetID`. All personas share the same schema cache. Base prompt is composed at request time.

---

## routes.go Build Logic

```
1. Create llmPool = NewLLMPool()
2. For each persona in cfg.Personas:
   a. key = PoolKey(persona.Provider, persona.Model)
   b. IF key already registered → skip (dedup)
   c. Determine apiKey and baseURL:
      - anthropic: cfg.AnthropicAPIKey + (persona.BaseURL OR cfg.AnthropicBaseURL)
      - deepseek:  cfg.DeepSeekAPIKey  + (persona.BaseURL OR cfg.DeepSeekBaseURL)
   d. IF apiKey != "" → create runner, register to pool
   e. Log: "LLM runner registered" with persona name, provider, model
3. IF cfg.Personas is empty (legacy mode):
   a. Create single runner from cfg.LLMProvider
   b. llmPool.SetFallback(runner)
4. ELSE:
   a. IF cfg.Personas["default"] exists → SetFallback(pool.Get(defaultKey))
5. hasLLM = llmPool.HasRunners()
```

---

## Response Metadata Fields (New)

`agent_metadata` object in `AgentResponse` should include:
```json
{
  "persona": "executive",
  "model": "claude-sonnet-4-6"
}
```

These are added to `resp.AgentMetadata` in `handler/agent.go` after handler returns:
```go
resp.AgentMetadata["persona"] = personaName
```
(model is already set by the handler via `runner.Model()`)

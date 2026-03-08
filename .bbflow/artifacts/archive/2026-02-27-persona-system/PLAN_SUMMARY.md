# Plan Summary — Persona System

**Spec ID:** persona-system | **Mode:** BROWNFIELD | **Complexity:** COMPLEX
**Confidence:** ✅ HIGH | **Risk:** 🟡 MEDIUM | **Effort:** ~4.5 hours

---

## Approach

Add `LLMPool` at startup (keyed by `provider:model`, deduplicated), export system prompt constants, refactor schema cache to store schema-only section, add `resolvePersona()` in `AgentHandler` that does O(1) map lookup, pass `(runner, promptStyle)` per-request to handlers.

See ARCHITECTURE.md for full before/after design.

---

## Risks

| ID | Risk | Level | Mitigation |
|----|------|-------|-----------|
| RISK-01 | Handle() signature change breaks callers | 🟢 LOW | Go compiler enforces all callers |
| RISK-02 | Schema cache refactor returns wrong prompt | 🟡 MEDIUM | Unit test getSchemaSection() returns no base prompt |
| RISK-03 | Legacy config without personas breaks startup | 🟡 MEDIUM | Explicit len(cfg.Personas)==0 fallback branch |
| RISK-04 | LLMPool concurrent read/write race | 🟢 LOW | Pool written once at startup, read-only during requests |

---

## Tasks

| ID | Name | Layer | Effort | Status |
|----|------|-------|--------|--------|
| TASK-01 | Add PersonaConfig to config.go | config | 15m | ⬜ pending |
| TASK-02 | Add Persona field to User and UserResponse | models | 10m | ⬜ pending |
| TASK-03 | Pass Persona through UserEntry → User | service | 15m | ⬜ pending |
| TASK-04 | Create LLMPool (agent/llm_pool.go) | agent | 20m | ⬜ pending |
| TASK-05 | Refactor bigquery_handler.go — export + cache + Handle() | agent | 45m | ⬜ pending |
| TASK-06 | Refactor elasticsearch_handler.go — export + Handle() | agent | 20m | ⬜ pending |
| TASK-07 | Create system_prompts.go — per-persona prompts | agent | 25m | ⬜ pending |
| TASK-08 | Update handler/agent.go — resolvePersona | handler | 45m | ⬜ pending |
| TASK-09 | Update server/routes.go — build LLMPool + wire | server | 60m | ⬜ pending |
| TASK-10 | Update cortexai.example.json — add personas | config | 10m | ⬜ pending |
| TASK-11 | Write unit tests — LLMPool + SystemPromptStyle | test | 30m | ⬜ pending |

---

## Execution Order

```
Phase 1 (parallel):   TASK-01, TASK-02, TASK-04
Phase 2 (parallel):   TASK-03, TASK-05, TASK-06
Phase 3:              TASK-07
Phase 4 (sequential): TASK-08 → TASK-09
Phase 5 (parallel):   TASK-10, TASK-11
```

---

## Verification Gates

- After each task: `go build ./...`
- After TASK-09: `go test ./...` (full regression)
- Final: start server with legacy config (no personas) → must start normally

---

## Notes

All changes backward compatible. `BigQueryHandler` and `ElasticsearchHandler` constructors unchanged. Biggest implementation effort is TASK-09 (routes.go wiring) and TASK-05 (cache refactor). Tests are embedded in tasks — no separate test sprint needed.

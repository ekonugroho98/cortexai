# Plan Summary — Persona Tool Filtering + Data Source Restriction

**Spec ID:** `persona-tool-filtering` | **Mode:** BROWNFIELD | **Complexity:** SIMPLE | **Platform:** backend (Go)
**Risk:** 🟢 LOW | **Confidence:** ✅ HIGH

---

## Approach

Extend `PersonaConfig` with two optional fields (`excluded_tools`, `allowed_data_sources`); filter the BQ tool list in `BigQueryHandler` before the LLM call; check data source restrictions in `AgentHandler` before routing. Nil/empty fields preserve existing behavior (backward compatible).

---

## Tasks

| ID | Task | Layer | Status |
|----|------|-------|--------|
| TASK-01 | Extend `PersonaConfig` (+`ExcludedTools`, +`AllowedDataSources`) | config | ⬜ pending |
| TASK-02 | `filterTools()` helper + update `Handle()` / `HandleStream()` | agent | ⬜ pending |
| TASK-03 | `resolvePersona()` extend + `checkDataSourceAllowed()` + `QueryAgent`/`Stream` | handler | ⬜ pending |
| TASK-04 | Unit tests (`filterTools` + `checkDataSourceAllowed`) + `cortexai.example.json` | test+config | ⬜ pending |

---

## Execution Order

**Phase 1 — Config:** TASK-01
→ **Phase 2 — Agent Layer:** TASK-02
→ **Phase 3 — Handler Layer:** TASK-03
→ **Phase 4 — Tests + Config:** TASK-04

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| `filterTools()` is private to `internal/agent` | No other package needs to call it |
| `checkDataSourceAllowed()` is a method on `AgentHandler` | Co-located with routing logic |
| `excludedTools` appended at end of `Handle()`/`HandleStream()` | Minimizes call-site diff |
| `resolvePersona()` returns `config.PersonaConfig` (3rd value) | Single lookup, no map re-access |
| Zero-value `PersonaConfig{}` = unrestricted | Safe default for personas without new fields |
| HTTP 403 for blocked data source | Correct semantics — resource forbidden, not missing |

---

## Risks

| Risk | Level | Mitigation |
|------|-------|------------|
| `Handle()` / `HandleStream()` signature change — callers must update | 🟢 LOW | Only called from `handler/agent.go`; updated in TASK-03 |
| `resolvePersona()` return type 2→3 values | 🟢 LOW | Only 2 call sites — both in `QueryAgent()` and `QueryAgentStream()` |

---

## Files Changed

| File | Type | Change |
|------|------|--------|
| `internal/config/config.go` | MODIFY | +2 fields on `PersonaConfig` |
| `internal/agent/bigquery_handler.go` | MODIFY | +`filterTools()`, +`excludedTools` param on `Handle()`/`HandleStream()` |
| `internal/handler/agent.go` | MODIFY | Extend `resolvePersona()`, add `checkDataSourceAllowed()`, update `QueryAgent()`/`QueryAgentStream()` |
| `config/cortexai.example.json` | MODIFY | Add `excluded_tools` + `allowed_data_sources` to `executive` persona |
| `internal/handler/agent_test.go` | CREATE | Tests for `checkDataSourceAllowed()` |

---

## Notes

4 files modified, 1 new test file. Nil/empty `excluded_tools` → all tools passed (backward compat). Nil/empty `allowed_data_sources` → all data sources allowed (backward compat). Existing 44+ agent tests must pass without regressions. No changes to routes, ES handler, `LLMPool`, `system_prompts`, or `LLMRunner` interface.

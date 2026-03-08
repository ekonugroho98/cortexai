# Archive: Persona Tool Filtering + Data Source Restriction

**Spec ID:** persona-tool-filtering
**Archived:** 2026-02-27
**Mode:** BROWNFIELD | **Complexity:** SIMPLE | **Platform:** backend (Go)
**Result:** ✅ PASS — 80/80 tests, 0 regressions

---

## Artifacts

| File | Purpose |
|------|---------|
| `DELTA_SPEC.md` | Change specification (requirements REQ-001–REQ-011) |
| `CODEBASE_MAP.md` | Affected files analysis |
| `DESIGN.md` | Technical design |
| `PLAN.yaml` | Implementation plan (4 tasks) |
| `PLAN_SUMMARY.md` | Human-readable plan summary |
| `PHASE_STATUS.yaml` | Full workflow timeline and state |
| `VERIFICATION_REPORT.md` | Quality gate results |
| `COMPLETION_SUMMARY.md` | Final delivery summary |

---

## Summary

Added two optional fields to `PersonaConfig` — `excluded_tools` and `allowed_data_sources` — enabling per-persona tool filtering and data source restriction in a fully backward-compatible way.

**Files changed:** 4 modified, 1 created
- `internal/config/config.go` — +ExcludedTools, +AllowedDataSources on PersonaConfig
- `internal/agent/bigquery_handler.go` — +filterTools(), Handle/HandleStream +excludedTools param
- `internal/handler/agent.go` — resolvePersona() 3 return values, +checkDataSourceAllowed(), QueryAgent/Stream 403 enforcement
- `config/cortexai.example.json` — executive persona with excluded_tools + allowed_data_sources
- `internal/handler/agent_test.go` (new) — 6 checkDataSourceAllowed tests
- `internal/agent/bigquery_handler_test.go` (extended) — +6 filterTools tests

**Workflow duration:** ~1 hour 12 minutes (10:35 → 11:47)

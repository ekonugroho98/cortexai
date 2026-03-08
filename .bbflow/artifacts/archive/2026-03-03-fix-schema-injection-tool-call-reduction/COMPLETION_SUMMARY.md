# Completion Summary

**Spec ID:** fix-schema-injection-tool-call-reduction
**Mode:** BUGFIX | **Complexity:** SIMPLE
**Completed:** 2026-03-03

---

## What Was Delivered

Replaced soft "you can skip" hint with an explicit IMPORTANT directive in both BQ and PG schema section builders, eliminating redundant tool calls when schema is pre-injected.

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/bigquery_handler.go` | Added `BQSchemaClosingInstruction` const; replaced inline string |
| `internal/agent/postgres_handler.go` | Added `PGSchemaClosingInstruction` const; replaced inline string |
| `internal/agent/bigquery_handler_test.go` | Added `TestBQSchemaSectionClosingInstruction_IsDirective` |
| `internal/agent/postgres_handler_test.go` | Added `TestPGSchemaSectionClosingInstruction_IsDirective` |

## Key Decision

Closing instruction extracted as a named constant (not inline string literal) to enable unit testing without a live BQ/PG connection. Test asserts presence of `IMPORTANT:`, `DO NOT call`, and `at most 1 execute call`, and absence of `you can skip`.

## Before / After

**Before:** `"Since schemas are already provided above, you can skip list_tables and get_bigquery_schema tool calls."`

**After:** `"IMPORTANT: All table schemas are already provided above. DO NOT call list_tables or get_bigquery_schema — go directly to writing and executing SQL. You should need at most 1 execute call."`

## Test Results

- 138/138 tests pass
- 2 new tests added (BQ + PG closing instruction assertions)
- 0 regressions

## Commits

- `fix(agent): strengthen BQ schema section closing instruction to directive language`
- `fix(agent): strengthen PG schema section closing instruction to directive language`

## Archive Location

`.bbflow/artifacts/archive/2026-03-03-fix-schema-injection-tool-call-reduction/`

## Post-Deploy Note

Schema cache retains old wording until 5min TTL expires. After deploying, call `DELETE /api/v1/cache/schema/{dataset}` or restart the server to force cache refresh.

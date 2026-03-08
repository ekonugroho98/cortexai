# Completion Summary

**Spec ID:** fix-dry-run-schema-tool-exclusion
**Mode:** BUGFIX | **Complexity:** SIMPLE
**Completed:** 2026-03-03

---

## What Was Delivered

Extended the `dry_run` block in all 4 handler call sites to exclude schema inspection tools when schema is already pre-injected, eliminating ~2-3s redundant latency per dry_run request.

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/bigquery_handler.go` | Handle() + HandleStream(): added schema tool exclusions when `dry_run=true && datasetID != ""` |
| `internal/agent/postgres_handler.go` | Handle() + HandleStream(): added schema tool exclusions when `dry_run=true && dbName != ""` |
| `internal/agent/bigquery_handler_test.go` | Added `TestFilterTools_DryRunWithSchemaPattern` |

## Tool Availability After Fix

| Tool | dry_run=false | dry_run=true, no dataset | dry_run=true, dataset set |
|------|--------------|--------------------------|--------------------------|
| `list_bigquery_datasets` | ✅ | ✅ | ✅ |
| `list_bigquery_tables` | ✅ | ✅ | ❌ |
| `get_bigquery_schema` | ✅ | ✅ | ❌ |
| `get_bigquery_sample_data` | ✅ | ✅ | ❌ |
| `execute_bigquery_sql` | ✅ | ❌ | ❌ |

## Key Decision

`req.DatasetID` checked directly in dry_run block (before `filterTools`) since `datasetID` variable is resolved after. For PG, `dbName` is already a parameter — simpler `dbName != ""` condition.

## Test Results

- 139/139 tests pass
- 1 new test (`TestFilterTools_DryRunWithSchemaPattern`)
- 0 regressions

## Commits

- `test(agent): add TestFilterTools_DryRunWithSchemaPattern for dry_run+schema exclusion`
- `fix(agent): exclude schema inspection tools on dry_run when schema is injected (BQ)`
- `fix(agent): exclude schema inspection tools on dry_run when schema is injected (PG)`

## Archive Location

`.bbflow/artifacts/archive/2026-03-03-fix-dry-run-schema-tool-exclusion/`

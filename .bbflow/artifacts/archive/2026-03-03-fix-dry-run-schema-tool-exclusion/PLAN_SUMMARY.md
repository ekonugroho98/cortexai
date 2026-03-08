# Plan Summary — fix-dry-run-schema-tool-exclusion

**Mode:** BUGFIX | **Complexity:** SIMPLE | **Confidence:** ✅ HIGH

## Root Cause
`dry_run` block in all 4 handler call sites only excludes the execute tool. Schema inspection tools remain, so LLM calls `get_bigquery_schema` redundantly (~2-3s wasted latency).

## Fix Strategy
Extend the `dry_run` block to also exclude `list_*_tables`, `get_*_schema`, `get_*_sample_data` when dataset/database is specified (schema already injected). Retain `list_*_datasets`.

## Tasks

| # | Name | Files | Layer |
|---|------|-------|-------|
| 1 | Add TestFilterTools_DryRunWithSchemaPattern | `bigquery_handler_test.go` | test |
| 2 | Fix BQ Handle() + HandleStream() dry_run block | `bigquery_handler.go` | domain |
| 3 | Fix PG Handle() + HandleStream() dry_run block | `postgres_handler.go` | domain |

## Execution Order

Task 1 → Task 2, Task 3 (BQ and PG independent)

## Risks

🟢 **LOW** — `get_bigquery_sample_data` excluded on dry_run; JOIN preview requires dry_run=false (expected behaviour).

## Notes
`TestFilterTools_DryRunWithSchemaPattern` tests `filterTools()` directly with the expected exclusion set. Handler methods cannot be unit-tested without live BQ/PG connections.

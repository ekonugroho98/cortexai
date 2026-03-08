# Bug Report: dry_run Still Calls get_bigquery_schema Despite Schema Pre-Injection

**Spec ID:** fix-dry-run-schema-tool-exclusion
**Mode:** BUGFIX
**Complexity:** SIMPLE
**Date:** 2026-03-03

---

## Summary

When `dry_run=true` and a dataset/database is specified (meaning schema is pre-injected into the system prompt), the LLM still has access to `get_bigquery_schema`, `list_bigquery_tables`, and `get_bigquery_sample_data` tools. It calls `get_bigquery_schema` redundantly, wasting ~2-3s latency per request. Only `execute_bigquery_sql` is excluded — but not the schema inspection tools.

---

## Steps to Reproduce

1. Send `POST /api/v1/query-agent` with `dry_run: true` and a valid `dataset_id`.
2. Observe `tools_used` in the response metadata.
3. **Result:** `get_bigquery_schema` appears in `tools_used`.

---

## Expected Behavior

When `dry_run=true` AND `dataset_id != ""` (schema already injected into system prompt):
- `execute_bigquery_sql` — excluded (already done)
- `get_bigquery_schema` — excluded (schema already in prompt)
- `list_bigquery_tables` — excluded (schema already in prompt)
- `get_bigquery_sample_data` — excluded (dry_run means no data access)
- `list_bigquery_datasets` — **retained** (LLM may need to identify dataset)

Same for PostgreSQL when `dbName != ""`:
- `execute_postgres_sql` — excluded
- `get_postgres_schema` — excluded
- `list_postgres_tables` — excluded
- `get_postgres_sample_data` — excluded
- `list_postgres_databases` — **retained**

---

## Actual Behavior

Only `execute_bigquery_sql`/`execute_postgres_sql` is excluded on `dry_run`. All schema inspection tools remain available, so the LLM calls `get_bigquery_schema` despite the schema already being present in the system prompt.

---

## Root Cause

In `BigQueryHandler.Handle()` (line 248–250) and `HandleStream()` (line 419–421), and `PostgresHandler.Handle()` (line 193–195) and `HandleStream()` (line 378–380), the `dry_run` block only appends the execute tool:

```go
// BQ Handle() — current (incomplete)
if req.DryRun {
    excludedTools = append(excludedTools, "execute_bigquery_sql")
}
```

The schema inspection tools are not excluded. The schema section condition (`datasetID != ""`) is checked after `filterTools()` in BQ handlers (datasetID is resolved at line 260 / 431), so the dry_run block must check `req.DatasetID != nil && *req.DatasetID != ""` directly.

For PG handlers, `dbName` is already a parameter available at the dry_run block.

---

## Fix

Extend the `dry_run` block in all 4 call sites to also exclude schema inspection tools when schema is pre-injected:

**BQ Handle() and HandleStream():**
```go
if req.DryRun {
    excludedTools = append(excludedTools, "execute_bigquery_sql")
    if req.DatasetID != nil && *req.DatasetID != "" {
        excludedTools = append(excludedTools, "list_bigquery_tables", "get_bigquery_schema", "get_bigquery_sample_data")
    }
}
```

**PG Handle() and HandleStream():**
```go
if req.DryRun {
    excludedTools = append(excludedTools, "execute_postgres_sql")
    if dbName != "" {
        excludedTools = append(excludedTools, "list_postgres_tables", "get_postgres_schema", "get_postgres_sample_data")
    }
}
```

---

## Affected Files

| File | Lines | Change |
|------|-------|--------|
| `internal/agent/bigquery_handler.go` | 248–250 (Handle), 419–421 (HandleStream) | Extend dry_run block |
| `internal/agent/postgres_handler.go` | 193–195 (Handle), 378–380 (HandleStream) | Extend dry_run block |
| `internal/agent/bigquery_handler_test.go` | new test | `TestFilterTools_DryRunWithSchemaPattern` |

---

## Workaround

None.

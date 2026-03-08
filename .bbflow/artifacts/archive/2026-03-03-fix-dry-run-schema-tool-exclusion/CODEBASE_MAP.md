# Codebase Map: fix-dry-run-schema-tool-exclusion

## Affected Files

### `internal/agent/bigquery_handler.go`
- **Handle()** line 248–250 — `if req.DryRun` block (MODIFY: add schema tool exclusions)
- **HandleStream()** line 419–421 — `if req.DryRun` block (MODIFY: same)
- Note: `datasetID` is resolved AFTER filterTools in both methods — check `req.DatasetID != nil && *req.DatasetID != ""` directly

### `internal/agent/postgres_handler.go`
- **Handle()** line 193–195 — `if req.DryRun` block (MODIFY: add schema tool exclusions)
- **HandleStream()** line 378–380 — `if req.DryRun` block (MODIFY: same)
- Note: `dbName` is already a function parameter — condition is simply `dbName != ""`

### `internal/agent/bigquery_handler_test.go`
- ADD `TestFilterTools_DryRunWithSchemaPattern` — verifies only `list_bigquery_datasets` remains when all 5 tools are present and dry_run + schema exclusions are applied

## Tool Name Reference

| BQ Tool | Name string |
|---------|-------------|
| BQListDatasetsTool | `"list_bigquery_datasets"` |
| BQListTablesTool | `"list_bigquery_tables"` |
| BQGetSchemaTool | `"get_bigquery_schema"` |
| BQSampleDataTool | `"get_bigquery_sample_data"` |
| BQExecuteQueryTool | `"execute_bigquery_sql"` |

| PG Tool | Name string |
|---------|-------------|
| PGListDatabasesTool | `"list_postgres_databases"` |
| PGListTablesTool | `"list_postgres_tables"` |
| PGGetSchemaTool | `"get_postgres_schema"` |
| PGSampleDataTool | `"get_postgres_sample_data"` |
| PGExecuteQueryTool | `"execute_postgres_sql"` |

## Code Flow

```
Handle() / HandleStream()
  ├── security checks (PII, prompt validation)
  ├── if req.DryRun {
  │     excludedTools = append(..., "execute_bigquery_sql")
  │     [FIX] if req.DatasetID != nil && *req.DatasetID != "" {
  │     [FIX]   excludedTools = append(..., "list_bigquery_tables",
  │     [FIX]                              "get_bigquery_schema",
  │     [FIX]                              "get_bigquery_sample_data")
  │     [FIX] }
  │   }
  ├── filterTools(allTools, excludedTools)  ← tools built here
  └── runner.Run(ctx, systemPrompt, prompt, filteredTools)
```

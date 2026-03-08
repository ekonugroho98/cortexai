# Codebase Map: fix-schema-injection-tool-call-reduction

## Affected Files

### `internal/agent/bigquery_handler.go`
- **Function:** `getSchemaSection(ctx, datasetID) string`
- **Line:** 130 — `sb.WriteString(...)` — closing instruction appended to the schema block
- **Change type:** MODIFY — replace string literal only, no logic change

### `internal/agent/postgres_handler.go`
- **Function:** `getPGSchemaSection(ctx, squadID, dbName, pgSvc) string`
- **Line:** 104 — `sb.WriteString(...)` — closing instruction appended to the schema block
- **Change type:** MODIFY — replace string literal only, no logic change

## How Schema Injection Works

```
Request arrives → Handle() / HandleStream()
  → getSchemaSection(ctx, datasetID)         [BQ]
  → getPGSchemaSection(ctx, squad, db, svc)  [PG]
      ├── Check cache (5min TTL + singleflight)
      ├── On miss: fetch all table schemas from BQ/PG
      ├── Build string: header + per-table schema rows
      └── Append closing instruction ← THIS IS THE FIX TARGET
  → systemPrompt = SystemPromptStyle(style) + schemaSection
  → runner.Run(ctx, systemPrompt, userPrompt, tools)
```

## Test Location

- `internal/agent/bigquery_handler_test.go` — `TestGetSchemaSection*` tests check schema output content; add assertion for new wording
- `internal/agent/postgres_handler_test.go` — analogous tests for PG schema section

# Codebase Map: fix-dry-run-sql-execution

**Generated:** 2026-03-03
**Mode:** BUGFIX

---

## Primary Files (Modified)

### `internal/agent/bigquery_handler.go`

**Role:** Orchestrates NL→SQL pipeline untuk BigQuery. Manages schema cache, tool filtering, agent loop, SQL extraction, dan security checks.

**Key sections relevant to bug:**

| Line Range | What it does |
|------------|--------------|
| ~199       | `Handle()` signature — receives `excludedTools []string` |
| ~242–248   | `filterTools(allBQTools, excludedTools)` — **BUG LOCATION** (Handle) |
| ~282       | `if generatedSQL != "" && !req.DryRun` — post-loop guard |
| ~363       | `HandleStream()` signature |
| ~410–416   | `filterTools(allBQTools, excludedTools)` — **BUG LOCATION** (HandleStream) |
| ~456       | `if generatedSQL != "" && !req.DryRun` — post-loop guard (stream) |
| ~650       | `filterTools()` implementation — nil-safe, O(1) set lookup |

**Existing `filterTools` signature:**
```go
func filterTools(ts []tools.Tool, excluded []string) []tools.Tool
```

### `internal/agent/postgres_handler.go`

**Role:** Mirrors BigQuery handler — orchestrates NL→SQL pipeline untuk PostgreSQL.

**Key sections relevant to bug:**

| Line Range | What it does |
|------------|--------------|
| ~126       | `Handle()` signature — receives `excludedTools []string` |
| ~188–194   | `filterTools(pgTools, excludedTools)` — **BUG LOCATION** (Handle) |
| ~224       | `if generatedSQL != "" && !req.DryRun` — post-loop guard |
| ~304       | `HandleStream()` signature |
| ~370–376   | `filterTools(pgTools, excludedTools)` — **BUG LOCATION** (HandleStream) |
| ~412       | `if generatedSQL != "" && !req.DryRun` — post-loop guard (stream) |

---

## Supporting Files (Read-Only Reference)

### `internal/models/response.go` (or request types)
- `AgentRequest.DryRun bool` — field yang sudah ada, tidak perlu diubah

### `internal/tools/bq_list_datasets.go` + tools package
- Tool names: `execute_bigquery_sql`, `list_bigquery_datasets`, `get_bigquery_table_schema`, `get_bigquery_sample_data`, `list_bigquery_tables`
- PG tools: `execute_postgres_sql`, `list_postgres_databases`, `list_postgres_tables`, `get_postgres_schema`, `get_postgres_sample_data`

### `internal/agent/cortex_agent.go`
- `lastExecutedSQL` tracking — diupdate dari tool call params saat LLM memanggil execute tool
- Dengan fix ini, LLM tidak akan memanggil execute tool → `lastExecutedSQL` akan kosong
- `generated_sql` di response perlu diverifikasi: apakah fallback ke SQL dalam text response tetap berfungsi saat execute tool tidak tersedia?

### `internal/handler/agent.go`
- `resolvePersona()` — menyiapkan `excludedTools` dari `PersonaConfig.ExcludedTools`
- Pass `excludedTools` ke `Handle()` / `HandleStream()` — tidak perlu diubah

---

## Fix Pattern

```go
// Pattern yang sama diterapkan di 4 lokasi:
// Sebelum filterTools(), tambahkan:

if req.DryRun {
    excludedTools = append(excludedTools, "<execute_tool_name>")
}
```

**Lokasi aplikasi:**
1. `bigquery_handler.go` Handle() — sebelum line ~242
2. `bigquery_handler.go` HandleStream() — sebelum line ~410
3. `postgres_handler.go` Handle() — sebelum line ~188
4. `postgres_handler.go` HandleStream() — sebelum line ~370

---

## Data Flow (Current vs Fixed)

```
CURRENT (buggy):
handler/agent.go
  └─ resolvePersona() → excludedTools (dari PersonaConfig)
  └─ Handle(req, excludedTools)
       └─ filterTools(allTools, excludedTools)  ← execute tool masih ada
       └─ agent.Run(tools_with_execute)
            └─ LLM calls execute_bigquery_sql  ← SQL dieksekusi!
       └─ if !req.DryRun { re-execute }  ← terlambat

FIXED:
handler/agent.go
  └─ resolvePersona() → excludedTools (dari PersonaConfig)
  └─ Handle(req, excludedTools)
       └─ if req.DryRun: excludedTools += "execute_bigquery_sql"
       └─ filterTools(allTools, excludedTools)  ← execute tool DIFILTER
       └─ agent.Run(tools_without_execute)
            └─ LLM hanya bisa inspect schema, TIDAK bisa execute
       └─ if !req.DryRun { re-execute }  ← defense-in-depth, retained
```

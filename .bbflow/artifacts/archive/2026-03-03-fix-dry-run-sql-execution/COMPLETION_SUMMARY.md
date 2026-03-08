# Completion Summary: Fix dry_run SQL Execution During Agent Loop

**Spec ID:** fix-dry-run-sql-execution
**Mode:** BUGFIX | **Complexity:** SIMPLE
**Completed:** 2026-03-03
**Archive:** `.bbflow/artifacts/archive/2026-03-03-fix-dry-run-sql-execution/`

---

## What Was Delivered

`dry_run: true` sekarang mencegah LLM memanggil execute tool selama agent loop,
bukan hanya memblok re-eksekusi setelah loop selesai.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/bigquery_handler.go` | +3 lines di `Handle()` + `HandleStream()` sebelum `filterTools()` |
| `internal/agent/postgres_handler.go` | +3 lines di `Handle()` + `HandleStream()` sebelum `filterTools()` |
| `internal/agent/bigquery_handler_test.go` | +`TestFilterTools_DryRunPattern` (40 lines) |
| `CHANGELOG.md` | Fixed entry ditambahkan |

---

## Root Cause & Fix

**Root Cause:** `filterTools()` dipanggil sebelum `req.DryRun` dicek. Post-loop guard
`if !req.DryRun { ... }` hanya mencegah re-eksekusi SQL, bukan tool call LLM di dalam loop.

**Fix:** Di 4 lokasi (Handle + HandleStream × 2 handler), sebelum `filterTools()`:

```go
// bigquery_handler.go
if req.DryRun {
    excludedTools = append(excludedTools, "execute_bigquery_sql")
}

// postgres_handler.go
if req.DryRun {
    excludedTools = append(excludedTools, "execute_postgres_sql")
}
```

---

## Test Results

```
go test ./...
ok  internal/agent    (incl. TestFilterTools_DryRunPattern — NEW)
ok  internal/handler
ok  internal/middleware
ok  internal/security
ok  internal/service
ok  internal/tools
```

All 137 tests pass (136 existing + 1 new).

---

## Acceptance Criteria

- [x] `dry_run: true` → execute_bigquery_sql tidak ada di tools list LLM
- [x] `dry_run: true` → execute_postgres_sql tidak ada di tools list LLM
- [x] Schema/inspection tools tetap tersedia saat dry_run=true
- [x] `dry_run: false` behavior tidak berubah
- [x] Fix berlaku untuk Handle() dan HandleStream() pada kedua handler
- [x] All existing tests pass

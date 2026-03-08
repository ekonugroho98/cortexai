# Plan Summary: Fix dry_run SQL Execution During Agent Loop

**Spec ID:** fix-dry-run-sql-execution | **Mode:** BUGFIX | **Complexity:** SIMPLE | **Confidence:** ✅ HIGH

---

## Approach

**Root Cause:** `filterTools()` dipanggil sebelum ada kesempatan untuk mengecualikan execute tool ketika `dry_run=true`. Post-loop guard `if !req.DryRun` terlambat — SQL sudah dieksekusi di dalam agent loop.

**Fix:** Di setiap `Handle()` dan `HandleStream()` pada kedua handler, tambahkan `if req.DryRun { excludedTools = append(excludedTools, "<execute_tool>") }` sebelum `filterTools()` dipanggil.

---

## Tasks

| ID | Task | File | Effort | Status |
|----|------|------|--------|--------|
| TASK-01 | Fix BigQuery handler (Handle + HandleStream) | `internal/agent/bigquery_handler.go` | 15m | ⬜ |
| TASK-02 | Fix PostgreSQL handler (Handle + HandleStream) | `internal/agent/postgres_handler.go` | 15m | ⬜ |
| TASK-03 | Add unit tests + run all tests | `internal/agent/bigquery_handler_test.go` | 15m | ⬜ |

**Total: 3 tasks · ~45 min**

---

## Execution Order

```
Phase 1 (parallel): TASK-01 (BQ fix) , TASK-02 (PG fix)
    ↓
Phase 2: TASK-03 (tests + go test ./...)
```

---

## Risks

| Risk | Level | Mitigation |
|------|-------|------------|
| `lastExecutedSQL` kosong saat dry_run=true jika LLM tidak embed SQL di tool call params | 🟢 LOW | lastExecutedSQL diisi dari tool call params (bukan execution result); fallback extractSQL tetap aktif |
| Duplicate entry di excludedTools (persona sudah exclude + dry_run=true) | 🟢 LOW | filterTools() menggunakan map lookup — duplicate entry aman, tidak ada error |

---

## What Changes

```go
// TASK-01: bigquery_handler.go — dua lokasi (Handle + HandleStream)
// Tambahkan sebelum filterTools():
if req.DryRun {
    excludedTools = append(excludedTools, "execute_bigquery_sql")
}

// TASK-02: postgres_handler.go — dua lokasi (Handle + HandleStream)
// Tambahkan sebelum filterTools():
if req.DryRun {
    excludedTools = append(excludedTools, "execute_postgres_sql")
}
```

4 insertions total. No signature changes. No new dependencies.

---

## Acceptance Criteria (Overall)

- [ ] `dry_run: true` → LLM tidak bisa memanggil execute_bigquery_sql / execute_postgres_sql
- [ ] Schema & inspection tools tetap tersedia saat dry_run=true
- [ ] `dry_run: false` behavior identik dengan sebelum fix
- [ ] `go test ./...` returns exit code 0 (136+ tests pass)

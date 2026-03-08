# Bug Report: dry_run Flag Does Not Prevent SQL Execution During Agent Loop

**Spec ID:** fix-dry-run-sql-execution
**Mode:** BUGFIX
**Complexity:** SIMPLE
**Platform:** backend (Go/chi)
**Date:** 2026-03-03
**Status:** Open

---

## Summary

`dry_run: true` pada `AgentRequest` tidak mencegah LLM memanggil tool `execute_bigquery_sql` atau `execute_postgres_sql` selama agent loop. Akibatnya, SQL tetap dieksekusi ke database—bertentangan dengan semantik dry_run.

---

## Bug Details

### Steps to Reproduce

1. Kirim request ke `POST /api/v1/query-agent` dengan body:
   ```json
   {
     "prompt": "tampilkan top 5 user",
     "dataset_id": "wlt_datalake_01",
     "data_source": "bigquery",
     "dry_run": true
   }
   ```
2. Observe agent loop logs — LLM masih memanggil `execute_bigquery_sql` hingga 6x.
3. SQL tetap dieksekusi ke BigQuery/PostgreSQL meskipun `dry_run: true`.

### Expected Behavior

Saat `dry_run: true`, LLM **tidak boleh memanggil** tool `execute_bigquery_sql` atau `execute_postgres_sql` sama sekali. Agent boleh melakukan schema inspection (list datasets, list tables, get schema, sample data), tapi eksekusi SQL harus sepenuhnya dicegah.

Response harus berisi `generated_sql` (SQL yang akan dieksekusi) tanpa hasil query aktual.

### Actual Behavior

LLM memanggil `execute_bigquery_sql` / `execute_postgres_sql` berulang kali selama agent loop. Flag `dry_run` hanya di-check **setelah** agent loop selesai (post-loop guard) di:

- `bigquery_handler.go:282` — `if generatedSQL != "" && !req.DryRun { ... }`
- `bigquery_handler.go:456` — same in HandleStream
- `postgres_handler.go:224` — `if generatedSQL != "" && !req.DryRun { ... }`
- `postgres_handler.go:412` — same in HandleStream

Post-loop guard ini hanya mencegah **re-eksekusi** SQL setelah loop selesai, bukan mencegah tool call di dalam loop.

### Workaround

None.

---

## Root Cause Analysis

### Location

| File | Lines | Method |
|------|-------|--------|
| `internal/agent/bigquery_handler.go` | 242–248 | `Handle()` — `filterTools()` call |
| `internal/agent/bigquery_handler.go` | 410–416 | `HandleStream()` — `filterTools()` call |
| `internal/agent/postgres_handler.go` | 188–194 | `Handle()` — `filterTools()` call |
| `internal/agent/postgres_handler.go` | 370–376 | `HandleStream()` — `filterTools()` call |

### Root Cause

`filterTools()` menerima `excludedTools []string` dan memfilter tool dari daftar yang diberikan ke LLM. Namun, ketika `req.DryRun == true`, tidak ada mekanisme yang menambahkan execute tool ke `excludedTools` sebelum `filterTools()` dipanggil.

**Current flow (buggy):**
```
req.DryRun = true
  ↓
filterTools(allBQTools, excludedTools)  ← execute tool MASIH included
  ↓
agent.Run(tools_with_execute)  ← LLM dapat memanggil execute_bigquery_sql
  ↓
if !req.DryRun { re-execute SQL }  ← guard ini tidak berguna karena SQL sudah dieksekusi di dalam loop
```

**Expected flow (fixed):**
```
req.DryRun = true
  ↓
if req.DryRun: excludedTools = append(excludedTools, "execute_bigquery_sql")
  ↓
filterTools(allBQTools, excludedTools)  ← execute tool DIFILTER
  ↓
agent.Run(tools_without_execute)  ← LLM tidak bisa memanggil execute tool
  ↓
generated_sql diambil dari lastExecutedSQL fallback (tool call terakhir)
```

---

## Fix Description

Di setiap `Handle()` dan `HandleStream()` pada kedua handler, **sebelum** memanggil `filterTools()`, check apakah `req.DryRun == true`. Jika ya, append nama execute tool ke `excludedTools`:

**BigQuery handler (`bigquery_handler.go`):**
```go
// Sebelum filterTools():
if req.DryRun {
    excludedTools = append(excludedTools, "execute_bigquery_sql")
}
bqTools := filterTools([]tools.Tool{...}, excludedTools)
```

**PostgreSQL handler (`postgres_handler.go`):**
```go
// Sebelum filterTools():
if req.DryRun {
    excludedTools = append(excludedTools, "execute_postgres_sql")
}
pgTools := filterTools([]tools.Tool{...}, excludedTools)
```

4 lokasi total (Handle + HandleStream × 2 handler).

### Why This Works

- `filterTools()` sudah menggunakan O(1) map lookup untuk exclusion — tidak ada perubahan logika yang diperlukan.
- `lastExecutedSQL` fallback di `bigquery_handler.go` akan menangkap SQL dari tool call terakhir LLM walaupun tool execution diblok. *(Note: perlu verifikasi bahwa lastExecutedSQL tracked dari tool call params, bukan dari execution result.)*
- Post-loop guard `if !req.DryRun` di baris 282/456/224/412 tetap dipertahankan sebagai defense-in-depth.

---

## Affected Files

```
internal/agent/bigquery_handler.go    ← Handle() line ~242, HandleStream() line ~410
internal/agent/postgres_handler.go    ← Handle() line ~188, HandleStream() line ~370
```

**No changes required to:**
- `models/` — `AgentRequest.DryRun` field sudah ada
- `handler/agent.go` — tidak perlu modifikasi
- `tools/` — tool definitions tidak berubah
- Tests

---

## Acceptance Criteria

- [ ] Saat `dry_run: true`, tool `execute_bigquery_sql` tidak muncul dalam tools yang diberikan ke LLM
- [ ] Saat `dry_run: true`, tool `execute_postgres_sql` tidak muncul dalam tools yang diberikan ke LLM
- [ ] Agent tetap bisa memanggil schema/inspection tools (`list_bigquery_datasets`, `get_bigquery_table_schema`, dll)
- [ ] `generated_sql` field pada response tetap terisi (dari `lastExecutedSQL` fallback atau SQL dalam response text)
- [ ] `dry_run: false` (default) tidak terpengaruh — behavior identik dengan sebelum fix
- [ ] Fix berlaku untuk `Handle()` dan `HandleStream()` pada kedua handler
- [ ] Semua existing tests masih pass

---

## Risk Assessment

**Risk Level: LOW**

- Perubahan sangat terlokalisasi (4 titik, 1 baris each)
- Menggunakan mekanisme `excludedTools` yang sudah ada dan teruji
- Tidak ada perubahan API contract
- `filterTools()` sudah idempotent dan nil-safe

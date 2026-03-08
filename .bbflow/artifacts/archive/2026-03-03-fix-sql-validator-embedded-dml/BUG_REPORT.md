# Bug Report: SQL Validator Tidak Deteksi DML Embedded dalam SELECT/WITH Query

**Spec ID:** fix-sql-validator-embedded-dml
**Mode:** BUGFIX
**Complexity:** SIMPLE
**Platform:** backend (Go/chi)
**Date:** 2026-03-03
**Status:** Open

---

## Summary

`SQLValidator.Validate()` tidak mendeteksi DML statements (`DELETE`, `INSERT`, `UPDATE`) yang di-embed dalam CTE (`WITH ... AS (...)`) atau subquery dari query yang dimulai dengan `SELECT` atau `WITH`. Query seperti `WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x` lolos validasi.

---

## Bug Details

### Steps to Reproduce

```go
v := security.NewSQLValidator()
result := v.Validate("WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x")
// result == "" → lolos, padahal seharusnya diblock
```

Contoh lain yang lolos:
```
WITH del AS (DELETE FROM users WHERE id = 1 RETURNING id) SELECT * FROM del
SELECT * FROM t WHERE id IN (INSERT INTO evil SELECT 1 RETURNING id)
SELECT a, (UPDATE orders SET status='x' WHERE id=1 RETURNING id) FROM t
```

### Expected Behavior

`Validate()` mengembalikan error string non-empty untuk setiap SQL yang mengandung DML statement (`DELETE FROM`, `INSERT INTO`, `UPDATE...SET`) di mana pun posisinya dalam query — termasuk di dalam CTE, subquery, atau expression.

### Actual Behavior

`Validate()` mengembalikan `""` (valid) karena:
1. `HasPrefix(upperSQL, "WITH")` → pass (query dimulai dengan WITH)
2. Semua `;`-prefixed patterns tidak cocok (tidak ada `;` sebelum `DELETE`)
3. Tidak ada pattern `\bDELETE\s+FROM\b` tanpa anchor `;`

### Workaround

None.

---

## Root Cause Analysis

### Current Protection Layers

| Layer | Protects Against | Gap |
|-------|-----------------|-----|
| `HasPrefix(SELECT\|WITH)` | Standalone `DELETE FROM orders` | ✅ No gap |
| `;`-prefixed patterns | Chained injection `SELECT 1; DELETE...` | ✅ No gap |
| **Missing** | DML embedded in CTE/subquery | ❌ **Gap** |

### Root Cause

`sqlDangerousPatterns` menggunakan `;`-prefix untuk semua DML patterns:
```go
regexp.MustCompile(`(?i);\s*DELETE\s+`),  // only matches ; DELETE
regexp.MustCompile(`(?i);\s*INSERT\s+`),  // only matches ; INSERT
regexp.MustCompile(`(?i);\s*UPDATE\s+`),  // only matches ; UPDATE
```

Tidak ada pattern yang mendeteksi `DELETE FROM` / `INSERT INTO` / `UPDATE...SET` di tengah query tanpa didahului `;`.

### Attack Vector

PostgreSQL dan BigQuery mendukung DML dalam CTE (`WITH ... AS (DML)`). Contoh valid BigQuery:
```sql
-- Query yang lolos validator tapi berbahaya:
WITH deleted AS (
  DELETE FROM `project.dataset.orders` WHERE id = 1
  RETURNING *
)
SELECT * FROM deleted
```

---

## Fix Description

Tambahkan 3 patterns baru ke `sqlDangerousPatterns` menggunakan `\b` word boundary (bukan `^` atau `;`), sehingga cocok di posisi mana pun dalam SQL:

```go
// DML embedded in subquery/CTE (no semicolon prefix needed)
regexp.MustCompile(`(?i)\bDELETE\s+FROM\b`),
regexp.MustCompile(`(?i)\bINSERT\s+INTO\b`),
regexp.MustCompile(`(?i)\bUPDATE\s+\w+\s+SET\b`),
```

**Mengapa `\b` bukan `^` atau `;`:**
- `^` (anchor awal): sudah ditangani oleh `HasPrefix` check, tidak berguna di sini
- `;` (semicolon): tidak ada untuk embedded DML dalam CTE/subquery
- `\b` (word boundary): cocok di posisi mana pun — awal, tengah, atau setelah `(`

**Mengapa hanya 3 patterns (bukan 7):**
- `DROP`, `ALTER`, `TRUNCATE`, `CREATE` sudah ditangani oleh `HasPrefix(SELECT|WITH)` check jika berdiri sendiri, dan sangat jarang digunakan dalam subquery/CTE. Namun dapat ditambahkan sebagai defense-in-depth jika diperlukan.
- Fokus pada `DELETE FROM`, `INSERT INTO`, `UPDATE...SET` karena ketiganya punya syntax yang jelas dan bisa muncul dalam CTE (terutama di PostgreSQL dengan `RETURNING`).

**Patterns yang TIDAK dihapus:**
- Semua `;`-prefixed patterns tetap — masih diperlukan untuk chained injection detection.

---

## Affected Files

```
internal/security/sql_validator.go    ← sqlDangerousPatterns slice
internal/security/security_test.go    ← TestSQLValidator atau ValidateSQL tests
```

---

## Acceptance Criteria

- [ ] `WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x` → blocked
- [ ] `SELECT * FROM t WHERE id IN (INSERT INTO evil SELECT 1 RETURNING id)` → blocked
- [ ] `SELECT a, (UPDATE orders SET x=1 WHERE id=1 RETURNING a) FROM t` → blocked
- [ ] Normal `SELECT ... FROM ... WHERE ...` tetap valid
- [ ] `WITH cte AS (SELECT ...) SELECT * FROM cte` tetap valid (CTE SELECT tetap boleh)
- [ ] `UNION ALL SELECT` tetap valid (sudah ada allowlist khusus)
- [ ] `;`-prefixed patterns tetap ada (chained injection masih terdeteksi)
- [ ] Test cases baru ditambahkan untuk ke-3 embedded DML scenarios
- [ ] All existing tests pass

---

## Risk Assessment

**Risk Level: LOW**

- Perubahan additive — hanya menambah patterns, tidak mengubah/menghapus yang ada
- `\b` word boundary tidak menghasilkan false positive untuk query SELECT biasa
- `UPDATE\s+\w+\s+SET\b` cukup spesifik — tidak akan cocok dengan `UPDATE` dalam konteks lain

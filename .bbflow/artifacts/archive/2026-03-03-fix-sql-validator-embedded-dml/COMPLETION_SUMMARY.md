# Completion Summary: Fix SQL Validator Embedded DML Detection

**Spec ID:** fix-sql-validator-embedded-dml
**Mode:** BUGFIX | **Complexity:** SIMPLE
**Completed:** 2026-03-03
**Archive:** `.bbflow/artifacts/archive/2026-03-03-fix-sql-validator-embedded-dml/`

---

## What Was Delivered

`SQLValidator` now blocks DML statements embedded inside CTE (`WITH ... AS (DML)`) or subquery contexts, closing a gap where queries like `WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x` passed validation.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/security/sql_validator.go` | +3 word-boundary patterns (`\bDELETE\s+FROM\b`, `\bINSERT\s+INTO\b`, `\bUPDATE\s+\w+\s+SET\b`) to `sqlDangerousPatterns` |
| `internal/security/security_test.go` | +3 invalid embedded DML cases + 2 valid CTE-SELECT guard cases to `TestSQLValidator` |
| `CHANGELOG.md` | Fixed entry added |

---

## Root Cause & Fix

**Root Cause:** `sqlDangerousPatterns` used `;`-prefix for all DML patterns. No `;` precedes DELETE in `WITH x AS (DELETE FROM ...)`, so these queries passed.

**Fix — 3 patterns with `\b` word boundary:**
```go
// DML embedded in subquery/CTE (no semicolon prefix needed)
regexp.MustCompile(`(?i)\bDELETE\s+FROM\b`),
regexp.MustCompile(`(?i)\bINSERT\s+INTO\b`),
regexp.MustCompile(`(?i)\bUPDATE\s+\w+\s+SET\b`),
```

**Key decisions:**
- `\b` (word boundary) — matches DML at any position (start, middle, after `(`), not just after `;`
- Existing `;`-prefixed patterns preserved — still needed for chained injection detection
- No false positives: `\b` prevents matching `delete_from_date` column names

---

## Acceptance Criteria

- [x] `WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x` → blocked
- [x] `SELECT * FROM t WHERE id IN (INSERT INTO evil SELECT 1 RETURNING id)` → blocked
- [x] `SELECT a, (UPDATE orders SET status='x' WHERE id=1 RETURNING a) FROM t` → blocked
- [x] Normal `SELECT ... FROM ... WHERE ...` → valid
- [x] `WITH cte AS (SELECT ...) SELECT * FROM cte` → valid (CTE SELECT allowed)
- [x] `SELECT * FROM a UNION ALL SELECT * FROM b` → valid (UNION ALL allowed)
- [x] `;`-prefixed patterns preserved
- [x] All existing tests pass

---

## Test Results

```
go test ./...
ok  internal/security   (all tests pass, +5 new cases)
ok  internal/agent      (cached)
ok  internal/handler    (cached)
ok  internal/middleware (cached)
ok  internal/service    (cached)
ok  internal/tools      (cached)
```

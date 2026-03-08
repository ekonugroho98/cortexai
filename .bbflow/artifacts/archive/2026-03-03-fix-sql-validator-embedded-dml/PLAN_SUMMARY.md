# Plan Summary: Fix SQL Validator Embedded DML Detection

**Spec ID:** fix-sql-validator-embedded-dml | **Mode:** BUGFIX | **Complexity:** SIMPLE

---

## Approach

Root Cause: `sqlDangerousPatterns` only matches DML after `;`. DML embedded in `WITH...AS(DML)` passes because there is no `;` before DELETE/INSERT/UPDATE.
Fix: Add 3 `\b` word-boundary patterns to `sqlDangerousPatterns` — no semicolon anchor — matching DML anywhere in query string.

---

## Tasks

| ID | Name | Files | Effort |
|----|------|-------|--------|
| 1 | Add failing tests for embedded DML detection | `internal/security/security_test.go` | ~5 min |
| 2 | Add word-boundary DML patterns to sqlDangerousPatterns | `internal/security/sql_validator.go` | ~5 min |

---

## Execution Order

Task 1 → Task 2 (sequential — write tests first, then implement)

---

## Risks

| Risk | Level | Mitigation |
|------|-------|------------|
| `\bDELETE\s+FROM\b` false-positive on column names like `delete_from_date` | 🟢 LOW | `\b` requires word boundary — `delete_from_date` won't match (`_` is a word char) |
| `\bUPDATE\s+\w+\s+SET\b` misses schema-qualified `UPDATE schema.table SET` | 🟢 LOW | Extremely rare in CTE context; `.` breaks `\w+` match |

---

## Confidence: ✅ HIGH

2 files, 2 tasks, additive-only change.

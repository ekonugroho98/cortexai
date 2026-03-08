# Codebase Map: fix-sql-validator-embedded-dml

**Generated:** 2026-03-03
**Mode:** BUGFIX

---

## Primary Files (Modified)

### `internal/security/sql_validator.go`

**Role:** Validates SQL queries before execution. Two-layer protection:
1. `HasPrefix(SELECT|WITH)` — blocks standalone DML
2. `sqlDangerousPatterns` — regex patterns for injection vectors

**Current `sqlDangerousPatterns` structure:**

| Pattern | Detects |
|---------|---------|
| `(?i);\s*DROP\s+` | Chained DROP after `;` |
| `(?i);\s*DELETE\s+` | Chained DELETE after `;` |
| `(?i);\s*INSERT\s+` | Chained INSERT after `;` |
| `(?i);\s*UPDATE\s+` | Chained UPDATE after `;` |
| `(?i);\s*ALTER\s+` | Chained ALTER after `;` |
| `(?i);\s*CREATE\s+` | Chained CREATE after `;` |
| `(?i);\s*TRUNCATE\s+` | Chained TRUNCATE after `;` |
| `(?i);\s*EXEC\s*\(?` | Chained EXEC |
| `(?i);\s*EXECUTE\s+` | Chained EXECUTE |
| `(?i)\bUNION\s+SELECT\b` | UNION injection (not UNION ALL) |
| `(?i)\bINTO\s+OUTFILE\b` | File write |
| ... | ... |

**Fix location:** Line 9–34, add 3 new entries to `sqlDangerousPatterns` after existing `;`-prefixed group.

### `internal/security/security_test.go`

**Role:** Tests for all security components.

**Relevant sections:**

| Location | What it tests |
|----------|--------------|
| `TestValidateSQL` or `TestSQLValidator` | SQL validation cases |

**Fix location:** Add 3 new test cases for embedded DML scenarios + 2 CTE-SELECT valid cases.

---

## Protection Layer Diagram

```
SQL Input
    │
    ▼
HasPrefix(SELECT|WITH)?
    ├── NO  → "only SELECT queries are allowed" ✅ blocks standalone DELETE/DROP/etc
    └── YES ↓
            │
    sqlDangerousPatterns check
            ├── ;\s*DELETE\s+     → blocks "SELECT 1; DELETE FROM t"
            ├── ;\s*INSERT\s+     → blocks "SELECT 1; INSERT INTO t"
            ├── \bUNION\s+SELECT\b → blocks UNION injection
            ├── [NEW] \bDELETE\s+FROM\b → blocks "WITH x AS (DELETE FROM t...)"
            ├── [NEW] \bINSERT\s+INTO\b → blocks "SELECT (INSERT INTO t...)"
            ├── [NEW] \bUPDATE\s+\w+\s+SET\b → blocks "WITH x AS (UPDATE t SET...)"
            └── ... other patterns
                    │
                    ▼
                "" (valid) ✅
```

---

## Key Distinction: Pattern Scope

| Pattern type | Example | Use case |
|---|---|---|
| `^\s*DELETE\s+FROM\b` | Prompt validator | Block raw DML as user prompt (different layer) |
| `;\s*DELETE\s+` | SQL validator | Block chained statement injection |
| `\bDELETE\s+FROM\b` | SQL validator (NEW) | Block DML embedded in CTE/subquery |

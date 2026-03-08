# Plan Summary: Fix Prompt Validator SQL DML Detection

**Spec ID:** fix-prompt-validator-sql-dml | **Mode:** BUGFIX | **Complexity:** SIMPLE | **Confidence:** ✅ HIGH

---

## Approach

**Root Cause:** `dangerousPatterns` tidak memiliki SQL DML patterns → `DELETE FROM orders` lolos validasi.

**Fix:** Append 7 regex patterns ke `dangerousPatterns` dengan `^` anchor untuk menghindari false positive pada natural language prompts.

---

## Tasks

| ID | Task | File | Effort | Status |
|----|------|------|--------|--------|
| TASK-01 | Add 7 SQL DML patterns to dangerousPatterns | `internal/security/prompt_validator.go` | 10m | ⬜ |
| TASK-02 | Add 7 test cases + NL false-positive guard | `internal/security/security_test.go` | 10m | ⬜ |

**Total: 2 tasks · ~20 min**

---

## Execution Order

```
Phase 1: TASK-01 (add patterns)
    ↓
Phase 2: TASK-02 (add tests + go test ./...)
```

---

## Risks

| Risk | Level | Mitigation |
|------|-------|------------|
| `CREATE` pattern bisa over-block `"create a report"` NL prompt | 🟡 MEDIUM | Verifikasi setelah implementasi; persempit ke `CREATE\s+(TABLE\|DATABASE\|...)` jika needed |
| `UPDATE \w+ SET` bisa lolos untuk alias kompleks | 🟢 LOW | Format standar SQL sudah tercakup; edge case tidak prioritas |

---

## What Changes

```go
// internal/security/prompt_validator.go — tambahkan sebelum } penutup dangerousPatterns:

// SQL DML statements (raw mutation commands in prompt)
regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\b`),
regexp.MustCompile(`(?i)^\s*DROP\s+`),
regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\b`),
regexp.MustCompile(`(?i)^\s*UPDATE\s+\w+\s+SET\b`),
regexp.MustCompile(`(?i)^\s*ALTER\s+`),
regexp.MustCompile(`(?i)^\s*TRUNCATE\s+`),
regexp.MustCompile(`(?i)^\s*CREATE\s+`),
```

---

## Acceptance Criteria (Overall)

- [ ] 7 DML statement types diblock oleh PromptValidator
- [ ] NL prompts dengan kata "delete/update/create" dalam kalimat tetap valid
- [ ] `go test ./...` returns exit code 0

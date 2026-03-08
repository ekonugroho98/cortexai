# Bug Report: Prompt Validator Tidak Block Raw SQL DML di Prompt

**Spec ID:** fix-prompt-validator-sql-dml
**Mode:** BUGFIX
**Complexity:** SIMPLE
**Platform:** backend (Go/chi)
**Date:** 2026-03-03
**Status:** Open

---

## Summary

`PromptValidator.Validate()` di `internal/security/prompt_validator.go` tidak mendeteksi SQL DML statements yang ditulis langsung di prompt user (DELETE, DROP, INSERT, UPDATE, ALTER, TRUNCATE, CREATE). Prompt seperti `"DELETE FROM orders WHERE id = 1"` lolos validasi dan sampai ke agent loop.

---

## Bug Details

### Steps to Reproduce

1. Kirim request ke `POST /api/v1/query-agent`:
   ```json
   {
     "prompt": "DELETE FROM orders WHERE id = 1",
     "dataset_id": "wlt_datalake_01"
   }
   ```
2. Lihat response `agent_metadata.prompt_validation` → `"passed"`
3. Agent loop menerima prompt berbahaya

### Expected Behavior

Prompt yang mengandung SQL DML statement di awal (sebagai raw command, bukan dalam konteks natural language) harus diblock oleh `PromptValidator` dengan `prompt_validation: "blocked: ..."`.

### Actual Behavior

`prompt_validation: "passed"` — DML statement lolos ke agent loop.

### Workaround

None. SQL Validator (`sql_validator.go`) baru aktif setelah LLM menghasilkan SQL — tidak memproteksi dari injection melalui prompt awal.

---

## Root Cause Analysis

### Location

`internal/security/prompt_validator.go` — `dangerousPatterns` slice (line 12–59)

### Root Cause

`dangerousPatterns` mencakup command execution, file ops, code execution, dan prompt injection — namun **tidak ada pattern untuk SQL DML statements**. User bisa mengirim raw SQL mutation command sebagai prompt dan lolos validasi:

```
"DELETE FROM orders WHERE id = 1"  → Tidak ada pattern yang cocok → PASSED
"DROP TABLE users"                  → Tidak ada pattern yang cocok → PASSED
"INSERT INTO admin VALUES ('hacker', 'pwd')" → PASSED
```

---

## Fix Description

Tambahkan 7 patterns baru ke `dangerousPatterns` slice di `prompt_validator.go`, dalam grup baru `// SQL DML statements`:

```go
// SQL DML statements (raw mutation commands in prompt)
regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\b`),
regexp.MustCompile(`(?i)^\s*DROP\s+`),
regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\b`),
regexp.MustCompile(`(?i)^\s*UPDATE\s+\w+\s+SET\b`),
regexp.MustCompile(`(?i)^\s*ALTER\s+`),
regexp.MustCompile(`(?i)^\s*TRUNCATE\s+`),
regexp.MustCompile(`(?i)^\s*CREATE\s+`),
```

**Catatan penting pada anchoring:**
- `^` anchor digunakan agar tidak memblok prompt natural language yang *menyebut* kata "delete" atau "update" dalam konteks query (e.g., "show me deleted orders", "update the chart").
- `\s*` setelah `^` mengizinkan whitespace di awal.
- `\b` word boundary dan keyword tambahan (e.g., `\s+FROM\b`, `\s+INTO\b`) memastikan hanya statement format SQL yang terdeteksi.

---

## Affected Files

```
internal/security/prompt_validator.go   ← dangerousPatterns slice
internal/security/security_test.go      ← TestPromptValidator invalid cases
```

---

## Acceptance Criteria

- [ ] `DELETE FROM orders WHERE id = 1` → `prompt_validation: blocked`
- [ ] `DROP TABLE users` → blocked
- [ ] `INSERT INTO admin VALUES (...)` → blocked
- [ ] `UPDATE users SET password = '...' WHERE ...` → blocked
- [ ] `ALTER TABLE orders ADD COLUMN ...` → blocked
- [ ] `TRUNCATE TABLE sessions` → blocked
- [ ] `CREATE TABLE evil (...)` → blocked
- [ ] Natural language prompts yang mengandung kata "delete/update/create" dalam konteks query **tidak diblock**:
  - "show me users who deleted their accounts" → still valid
  - "how many updates were made last week" → still valid
  - "create a report of top users" → still valid (contains "create" but not `^\s*CREATE\s+`)
- [ ] Semua existing tests masih pass
- [ ] 7 test cases baru ditambahkan ke `TestPromptValidator` invalid list

---

## Risk Assessment

**Risk Level: LOW**

- Perubahan terlokalisasi di satu slice + satu test file
- Anchor `^` mencegah false positive untuk prompt natural language
- Pattern spesifik (e.g., `DELETE\s+FROM\b`, tidak hanya `DELETE`) meminimalkan over-blocking
- `CREATE` adalah yang paling broad — hanya terblock jika di awal prompt (`^\s*CREATE\s+`)

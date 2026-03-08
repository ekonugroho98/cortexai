# Codebase Map: fix-prompt-validator-sql-dml

**Generated:** 2026-03-03
**Mode:** BUGFIX

---

## Primary Files (Modified)

### `internal/security/prompt_validator.go`

**Role:** Validates user prompts before reaching the agent loop. Checks against `dangerousPatterns` (30+ regex), suspicious indicators, and length limits.

**Key section:**

| Lines | What it does |
|-------|-------------|
| 11–59 | `dangerousPatterns` — grouped: command exec, file ops, code exec, prompt injection |
| 99–125 | `Validate()` — iterates patterns, returns first match as blocked |

**Fix location:** Append new group after line 58 (end of `dangerousPatterns` slice), before closing `}`.

### `internal/security/security_test.go`

**Role:** Tests for all security components (PIIDetector, PromptValidator, SQLValidator, etc.)

**Key section:**

| Lines | What it does |
|-------|-------------|
| 204–235 | `TestPromptValidator` — `valid []string` + `invalid []struct{prompt,reason}` |

**Fix location:** Add 7 new entries to `invalid` struct slice in `TestPromptValidator`.

---

## Pattern Design Reference

Current pattern groups in `dangerousPatterns`:
```
// Command execution   — rm, cp, mv, curl, wget, nc, bash, sh, python, node, git, sudo, su
// File operations     — ../, /etc/passwd, /etc/shadow, /proc/, /sys/, .env, id_rsa, .ssh/, >/
// Code execution      — eval(), exec(), system(), __import__(), subprocess(), os.system, popen
// Prompt injection    — ignore/disregard/forget/override previous instructions, new/change context
```

New group to add:
```
// SQL DML statements  — DELETE FROM, DROP, INSERT INTO, UPDATE...SET, ALTER, TRUNCATE, CREATE
```

---

## Anchor Rationale

`^` anchor is critical to avoid false positives:

| Prompt | With `^` | Without `^` |
|--------|----------|-------------|
| `"DELETE FROM orders"` | BLOCKED ✅ | BLOCKED |
| `"show deleted orders"` | VALID ✅ | BLOCKED ❌ (false positive) |
| `"how many updates this week"` | VALID ✅ | BLOCKED ❌ (false positive) |
| `"DROP TABLE users"` | BLOCKED ✅ | BLOCKED |
| `"don't drop the table"` | VALID ✅ | BLOCKED ❌ (false positive) |

The `(?i)` flag handles case-insensitivity; `\s*` after `^` handles leading whitespace.

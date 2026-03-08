# Codebase Map: fix-answer-language-mismatch

**Generated:** 2026-03-03
**Mode:** BROWNFIELD

---

## Affected Files

### `internal/agent/bigquery_handler.go`

**Role:** BQ handler, contains `BaseSystemPrompt` (fallback for unknown/empty persona styles).

**Change:** Add rule #8 to `BaseSystemPrompt`:
```
8. Always respond in the same language as the user's prompt. If the user writes in Indonesian, respond in Indonesian. If in English, respond in English.
```

**Location:** Line 66–80

---

### `internal/agent/elasticsearch_handler.go`

**Role:** ES handler, contains `ESSystemPrompt` (fallback for unknown/empty persona styles).

**Change:** Split rule #4 into two rules — keep "Interpret results" in #4, add language rule as #5, renumber #5→#6:

Before:
```
4. Interpret results and explain findings clearly in Indonesian or English (match user's language)
5. Focus on the specific identifier/time range provided by the user
6. Maximum 100 results per search
```

After:
```
4. Interpret results and explain findings clearly
5. Always respond in the same language as the user's prompt. If the user writes in Indonesian, respond in Indonesian. If in English, respond in English.
6. Focus on the specific identifier/time range provided by the user
7. Maximum 100 results per search
```

**Location:** Line 16–32

---

### `internal/agent/system_prompts.go`

**Role:** All persona-variant prompts for BQ, PG, and ES.

**Changes:**

| Const | Lines | Change |
|-------|-------|--------|
| `executiveSystemPrompt` | ~6–26 | Add rule #8 (language) |
| `technicalSystemPrompt` | ~28–48 | Add rule #8 (language) |
| `supportSystemPrompt` | ~50–70 | Add rule #8 (language) |
| `esExecutiveSystemPrompt` | ~74–95 | Replace rule #4; add rule #5; renumber #5→#6 |
| `esSupportSystemPrompt` | ~97–119 | Replace rule #4; add rule #5; renumber #5→#6 |
| `pgExecutiveSystemPrompt` | ~123–145 | Add rule #10 (language) |
| `pgTechnicalSystemPrompt` | ~147–169 | Add rule #10 (language) |
| `pgSupportSystemPrompt` | ~171–193 | Add rule #10 (language) |
| `PGBaseSystemPrompt` | ~197–213 | Add rule #10 (language) |

---

## No New Files

No new files are created. All changes are string modifications to existing const declarations.

## Tests

`internal/agent/system_prompts_test.go` and `internal/agent/postgres_handler_test.go` test style routing and fallback — these do NOT assert specific rule text, so no test changes are needed. Existing tests will continue to pass.

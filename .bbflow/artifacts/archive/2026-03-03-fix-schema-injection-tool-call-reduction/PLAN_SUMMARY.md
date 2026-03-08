# Plan Summary — fix-schema-injection-tool-call-reduction

**Mode:** BUGFIX | **Complexity:** SIMPLE | **Confidence:** ✅ HIGH

## Root Cause
Soft `"you can skip"` language in `getSchemaSection()` and `getPGSchemaSection()` closing instruction — LLM treats it as optional, resulting in redundant schema fetches and repeated execute calls.

## Fix Strategy
Replace the string literal in both schema section builders with an explicit IMPORTANT directive that hard-caps execute calls at 1.

## Tasks

| # | Name | Files | Layer |
|---|------|-------|-------|
| 1 | Fix BQ schema section closing instruction | `bigquery_handler.go` + `_test.go` | domain |
| 2 | Fix PG schema section closing instruction | `postgres_handler.go` + `_test.go` | domain |

## Execution Order

Task 1 → Task 2 (independent, either order)

## Risks

🟢 **LOW** — Schema cache stores old wording until 5min TTL expires. Call `DELETE /api/v1/cache/schema/{dataset}` or restart after deploy.

## Notes
String-only change. No logic, no API, no interface changes. Closing instruction extracted as named constant to enable unit testing without a live BQ/PG connection.

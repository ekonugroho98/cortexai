# Plan Summary — response-caching

**Feature:** Response Cache untuk Query Identik
**Mode:** GREENFIELD | **Complexity:** SIMPLE | **Confidence:** ✅ HIGH

---

## Approach

Add `responseCache` struct (mirroring existing `schemaCache` pattern) to the agent package, wire one instance each into `BigQueryHandler` and `PostgresHandler`, check/set the cache in `Handle()` after security passes but before schema fetch, and expose `DELETE /api/v1/cache/responses` for manual invalidation.

---

## Risks

| ID | Risk | Level |
|----|------|-------|
| R-01 | sha256 collision | 🟢 LOW |
| R-02 | Stale data within TTL | 🟢 LOW |
| R-03 | Memory growth | 🟢 LOW |

---

## Tasks

| ID | Name | Layer | Est. |
|----|------|-------|------|
| 1 | Add responseCache struct + responseCacheKey helper | data | ⬜ |
| 2 | Add TTL expiry, dry_run, and error non-caching tests | test | ⬜ |
| 3 | Wire respCache into BigQueryHandler + Handle() | data | ⬜ |
| 4 | Wire respCache into PostgresHandler + Handle() | data | ⬜ |
| 5 | Add DELETE /api/v1/cache/responses endpoint + route | di | ⬜ |

---

## Execution Order

```
Phase 1 (TDD): Task 1 → Task 2
Phase 2 (Wiring): Task 3 + Task 4  (parallel)
Phase 3 (HTTP): Task 5
```

---

## Key Files

| File | Change |
|------|--------|
| `internal/agent/bigquery_handler.go` | +`responseCache` struct, +`responseCacheKey`, +`FlushResponseCache()`, Handle() cache check |
| `internal/agent/bigquery_handler_test.go` | +4 tests: HitMiss, TTLExpiry, DryRunNotCached, ErrorNotCached |
| `internal/agent/postgres_handler.go` | +`respCache` field, +`FlushResponseCache()`, Handle() cache check |
| `internal/handler/cache.go` | +`FlushResponseCache()` handler |
| `internal/server/routes.go` | +`DELETE /cache/responses` route |

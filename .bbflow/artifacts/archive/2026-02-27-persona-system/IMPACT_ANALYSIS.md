# IMPACT_ANALYSIS — Persona System

**Spec ID:** persona-system

---

## Impact Summary

| Dimension | Impact | Level |
|-----------|--------|-------|
| API Contract | No change to endpoint paths/methods | None |
| Request schema | No change | None |
| Response schema | `agent_metadata` gains 2 new fields (`persona`, `model`) | Additive |
| Config schema | New `personas` section, new `persona` field in users | Additive |
| Existing tests | `extractSQL` tests → unaffected. `Handle()` tests → need new params | Low |
| Deployment | No infra changes needed | None |
| Performance | LLMPool adds O(1) map lookup per request | Negligible |
| Memory | Multiple LLMRunner instances (one per unique provider:model) | Low |

---

## Breaking Changes

**NONE.** All changes are backward compatible:

1. `BigQueryHandler` constructor: unchanged
2. `ElasticsearchHandler` constructor: unchanged
3. `LLMRunner` interface: unchanged
4. API endpoint signatures: unchanged
5. Config without `personas` key: falls back to legacy `llm_provider` behavior

---

## Risk Areas

### Risk 1: Handle() signature change
**Files affected:** `bigquery_handler.go`, `elasticsearch_handler.go`
**Risk:** Any future code that calls `Handle()` directly (e.g., integration tests) must be updated to pass `runner` and `promptStyle`.
**Current callers:** Only `handler/agent.go` — updated in same PR.
**Mitigation:** Compile-time enforcement — go build will catch all callers.

### Risk 2: Schema cache refactor
**File:** `bigquery_handler.go`
**Risk:** If `getSchemaSection()` is not properly refactored from `buildSystemPrompt()`, cache invalidation may fail or schema may be double-prepended.
**Mitigation:** Unit test `getSchemaSection()` returns schema without base prompt. `buildSystemPrompt()` can be deleted or kept as compatibility shim.

### Risk 3: LLMPool fallback chain
**File:** `server/routes.go`
**Risk:** If no runners register (all API keys missing) and no fallback is set, `llmPool.HasRunners()` returns false and agent handler is nil. `/query-agent` returns 503.
**Mitigation:** This is existing behavior — already handled by nil checks in route registration. The fallback chain (persona → default → pool fallback) is defensive.

### Risk 4: Concurrent access to LLMPool
**File:** `agent/llm_pool.go`
**Risk:** LLMPool is read-only after startup (only Register/SetFallback called at init, Get called per-request concurrently).
**Mitigation:** No mutex needed — map is written once at startup, read-only during request handling. Go maps are safe for concurrent reads.

---

## Test Impact

| Test File | Impact |
|-----------|--------|
| `internal/agent/bigquery_handler_test.go` | `extractSQL` tests: unaffected. Any `Handle()` call tests: need `runner, promptStyle` params added. |
| `internal/agent/deepseek_agent_test.go` | Unaffected — tests agent internals, not Handle() |
| `internal/middleware/middleware_test.go` | Unaffected |
| `internal/security/security_test.go` | Unaffected |
| `internal/service/router_test.go` | Unaffected |

**New tests recommended:**
- `agent/llm_pool_test.go`: TestGet, TestFallback, TestDedup, TestHasRunners
- `agent/system_prompts_test.go`: TestStyleDispatch (verify each style returns correct prompt)

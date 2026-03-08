# Feature Specification: Response Cache untuk Query Identik

**Spec ID:** response-caching
**Mode:** GREENFIELD
**Complexity:** SIMPLE
**Platform:** backend (Go)
**Created:** 2026-03-03

---

## 1. Background

CortexAI melakukan LLM call pada setiap request ke `Handle()`, termasuk query yang identik secara persis (prompt + dataset_id + persona yang sama). Setiap LLM call memakan 8-28 detik. Jika user mengulang pertanyaan yang sama, atau sistem memanggil endpoint yang sama secara berulang (polling, retry UI), tidak ada mekanisme deduplication — setiap request membebani LLM dan menambah latency.

Pattern ini sudah diterapkan untuk schema caching (`schemaCache` struct, di `bigquery_handler.go`) dengan TTL 5 menit dan `sync.RWMutex`. Response caching akan mengikuti pattern yang sama.

---

## 2. Problem Statement

Query identik (prompt + dataset_id + persona) selalu memanggil LLM ulang, menghabiskan 8-28s per request. User yang mengulang pertanyaan yang sama tidak mendapat jawaban instan.

---

## 3. Goals (Acceptance Criteria)

- [ ] **REQ-001** Cache hit pada query identik mengembalikan response tanpa LLM call (latency < 5ms)
- [ ] **REQ-002** Cache key = `sha256(prompt + datasetID + promptStyle)` — exact match only
- [ ] **REQ-003** TTL menggunakan nilai `schema_cache_ttl` yang sudah ada di config (default 5 menit)
- [ ] **REQ-004** Response dengan status bukan `"success"` tidak di-cache
- [ ] **REQ-005** `dry_run=true` response tidak di-cache
- [ ] **REQ-006** `agent_metadata` menyertakan `"response_cache": "hit"` atau `"response_cache": "miss"`
- [ ] **REQ-007** `DELETE /api/v1/cache/responses` endpoint untuk invalidasi manual seluruh response cache
- [ ] **REQ-008** Caching hanya di `Handle()` (non-streaming), tidak di `HandleStream()`
- [ ] **REQ-009** Thread-safe dengan `sync.RWMutex`
- [ ] **REQ-010** BQ dan PG handler masing-masing memiliki `responseCache` instance sendiri

---

## 4. Non-Goals (Out of Scope)

- Fuzzy matching / semantic similarity cache (hanya exact match)
- Streaming response caching (`HandleStream()`)
- Per-user cache segmentation (cache key tidak include user ID)
- Distributed/external cache (in-memory only, per-process)
- Cache size limit / eviction (TTL-based expiry cukup)

---

## 5. Technical Design

### 5.1 responseCache Struct

Didefinisikan di `internal/agent/bigquery_handler.go` (same package sebagai PG handler, sehingga reusable).

```
type responseCacheEntry struct {
    response  *models.AgentResponse
    expiresAt time.Time
}

type responseCache struct {
    mu      sync.RWMutex
    entries map[string]responseCacheEntry
    ttl     time.Duration
}

func newResponseCache(ttl time.Duration) *responseCache
func (c *responseCache) get(key string) (*models.AgentResponse, bool)
func (c *responseCache) set(key string, resp *models.AgentResponse)
func (c *responseCache) flush()
```

### 5.2 Cache Key

```go
import "crypto/sha256"
import "fmt"

key := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt + "|" + datasetID + "|" + promptStyle)))
```

Separator `|` mencegah ambiguity antara field. DatasetID bisa kosong string jika tidak diset.

### 5.3 Wiring

**`NewBigQueryHandler`**: tambah field `respCache *responseCache`, inisialisasi dengan `newResponseCache(h.schemaCacheTTL)`.

**`NewPostgresHandler`**: tambah field `respCache *responseCache`, inisialisasi dengan `newResponseCache(h.schemaCacheTTL)`.

### 5.4 Cache Check Point di Handle()

Setelah security checks (PII + prompt validation) pass, sebelum schema fetch dan LLM call:

```
1. Hitung cache key (sha256)
2. Jika dry_run → skip cache (langsung ke pipeline)
3. Cek cache.get(key)
4. Cache HIT → set agent_metadata["response_cache"] = "hit", return response clone
5. Cache MISS → set agent_metadata["response_cache"] = "miss", lanjut pipeline
6. Setelah pipeline selesai, jika status == "success" → cache.set(key, response)
```

### 5.5 Invalidation Endpoint

Di `internal/handler/cache.go`, tambah handler untuk `DELETE /api/v1/cache/responses`.

Handler memerlukan akses ke BQ handler dan PG handler dari AgentHandler. Response cache di-flush semua (tidak per-dataset — ini global cache clear).

Interface baru: `ResponseCacheFlushable` dengan method `FlushResponseCache()`.

### 5.6 agent_metadata Fields

```json
{
  "agent_metadata": {
    "response_cache": "hit",
    "persona": "...",
    "model": "..."
  }
}
```

---

## 6. Files Changed

| File | Action | Keterangan |
|------|--------|------------|
| `internal/agent/bigquery_handler.go` | MODIFY | Add `responseCache` struct + wiring + Handle() cache check |
| `internal/agent/postgres_handler.go` | MODIFY | Add `respCache` field + wiring + Handle() cache check |
| `internal/handler/cache.go` | MODIFY | Add `DELETE /api/v1/cache/responses` handler |
| `internal/server/routes.go` | MODIFY | Register new route |
| `internal/agent/bigquery_handler_test.go` | MODIFY | Add 4 response cache tests |

---

## 7. Tests

| Test | Verifikasi |
|------|------------|
| `TestResponseCache_HitMiss` | Second call dengan key identik → cache hit, tidak memanggil LLM |
| `TestResponseCache_TTLExpiry` | Entry kadaluarsa → cache miss, bukan stale hit |
| `TestResponseCache_DryRunNotCached` | dry_run=true response tidak masuk cache |
| `TestResponseCache_ErrorNotCached` | Response status != "success" tidak di-cache |

---

## 8. Risks

| Risk | Level | Mitigation |
|------|-------|------------|
| Cache key collision (sha256) | LOW | sha256 collision probability negligible |
| Stale data jika underlying data berubah | LOW | TTL terbatas (5 menit), sama dengan schema cache behavior |
| Memory usage | LOW | Responses kecil (~KB), TTL eviction, per-process only |

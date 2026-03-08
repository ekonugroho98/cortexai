# DELTA_SPEC — Persona Tool Filtering + Data Source Restriction

**Spec ID:** persona-tool-filtering
**Mode:** BROWNFIELD
**Complexity:** SIMPLE
**Platform:** backend (Go)
**Status:** Pending Approval
**Created:** 2026-02-27

---

## 1. Background & Motivation

Persona system yang baru diimplementasikan memungkinkan per-user AI behavior yang berbeda (model, system prompt style). Namun saat ini semua persona mendapat tool list yang sama dan bisa query ke semua data source (BigQuery dan Elasticsearch).

Kebutuhan bisnis yang belum terpenuhi:

- **Executive persona** tidak memerlukan tool `get_bigquery_sample_data` — executive tidak peduli dengan sample data, hanya ingin insight bisnis. Memberikan tool ini justru memungkinkan LLM "berkeliaran" fetching samples yang tidak perlu, memperlambat respons.
- **Executive persona tidak boleh query Elasticsearch** — log dan raw operational data (ES) bukan konsumsi executive. Mereka hanya butuh BigQuery analytics.

Solusi: tambah dua field opsional ke `PersonaConfig` — `excluded_tools` dan `allowed_data_sources` — sehingga behavior ini **configurable** tanpa hardcode, dan bisa diperluas ke persona lain di masa depan.

---

## 2. Scope of Change

### Yang Berubah
| File | Tipe | Perubahan |
|------|------|-----------|
| `internal/config/config.go` | MODIFY | +`ExcludedTools []string`, +`AllowedDataSources []string` di `PersonaConfig` |
| `internal/agent/bigquery_handler.go` | MODIFY | `Handle()` dan `HandleStream()` terima `excludedTools []string`, filter tool list sebelum di-pass ke runner |
| `internal/handler/agent.go` | MODIFY | Check `AllowedDataSources` sebelum routing; pass `ExcludedTools` ke `Handle()`/`HandleStream()` |
| `config/cortexai.example.json` | MODIFY | Update `executive` persona: tambah `excluded_tools` dan `allowed_data_sources` |

### Yang TIDAK Berubah
- `ElasticsearchHandler` — diblokir di routing level, tidak perlu terima parameter baru
- `LLMPool`, `system_prompts.go`, `LLMRunner` interface — tidak berubah
- Seluruh middleware chain, RBAC, squad isolation — tidak berubah
- API endpoint paths — tidak berubah
- Persona lain (developer, app_support) — tidak terpengaruh jika tidak define field baru

---

## 3. Functional Requirements

### Must Have (Critical)

- [ ] **REQ-001** `PersonaConfig` memiliki field opsional `excluded_tools []string` — daftar nama tool yang tidak akan di-pass ke LLM untuk persona ini
- [ ] **REQ-002** `PersonaConfig` memiliki field opsional `allowed_data_sources []string` — daftar data source yang diizinkan (`"bigquery"`, `"elasticsearch"`); kosong = semua diizinkan
- [ ] **REQ-003** `BigQueryHandler.Handle()` dan `HandleStream()` menerima `excludedTools []string`, mem-filter tool list sebelum memanggil `runner.Run()` / `runner.RunWithEmit()`
- [ ] **REQ-004** Jika persona query ke data source yang tidak ada di `allowed_data_sources` → kembalikan error dengan pesan yang jelas ke user (bukan panic/500)
- [ ] **REQ-005** Persona tanpa `excluded_tools` (nil/kosong) → semua tools diberikan (backward compat)
- [ ] **REQ-006** Persona tanpa `allowed_data_sources` (nil/kosong) → semua data source diizinkan (backward compat)
- [ ] **REQ-007** `config/cortexai.example.json` diupdate: persona `executive` memiliki `"excluded_tools": ["get_bigquery_sample_data"]` dan `"allowed_data_sources": ["bigquery"]`

### Should Have (Important)

- [ ] **REQ-008** Error message jika data source tidak diizinkan mengandung info yang actionable — contoh: `"Data source 'elasticsearch' is not available for your persona. Available: [bigquery]"`
- [ ] **REQ-009** Unit test untuk tool filtering logic di BigQueryHandler
- [ ] **REQ-010** Unit test untuk data source restriction logic di AgentHandler

### Could Have (Nice to have)

- [ ] **REQ-011** Log warning saat startup jika persona memiliki `allowed_data_sources` yang tidak valid (misal `"mysql"` — tidak dikenal sistem)

---

## 4. Backward Compatibility Requirements

**WAJIB dipertahankan:**

1. **Personas tanpa `excluded_tools`** → field nil/kosong → semua tools diberikan persis seperti sebelumnya
2. **Personas tanpa `allowed_data_sources`** → field nil/kosong → routing ke BQ dan ES tetap berjalan seperti biasa
3. **Config JSON lama** (tanpa field baru di PersonaConfig) → tidak perlu update, JSON unmarshal akan biarkan field nil
4. **BigQueryHandler constructor** → tidak berubah signature
5. **ElasticsearchHandler** → tidak berubah sama sekali
6. **Semua existing tests** → tidak ada regresi

---

## 5. Non-Functional Requirements

- **Performance**: Filtering tool list adalah O(n) dimana n = jumlah tools (~5 tools), overhead negligible
- **Data source check**: O(k) dimana k = jumlah allowed_data_sources, sebelum routing — overhead negligible
- **Config validation**: Field invalid (data source tidak dikenal) → log warning, tidak block startup
- **Error response**: Data source restriction violation → HTTP 403 dengan JSON error body yang jelas

---

## 6. Acceptance Criteria

- [ ] `go build ./...` clean (zero errors)
- [ ] `go test ./...` pass (semua existing test tidak ada regresi)
- [ ] User dengan persona `executive` query ke BigQuery → tool `get_bigquery_sample_data` tidak ada dalam tool call loop LLM
- [ ] User dengan persona `executive` query ke Elasticsearch → dapat error message yang jelas (bukan 500)
- [ ] User dengan persona `developer` (tidak define field baru) → semua tools tersedia, bisa query BQ dan ES
- [ ] Config tanpa `excluded_tools`/`allowed_data_sources` → server start normal, behavior tidak berubah

---

## 7. Out of Scope

- Whitelisting tools (hanya blacklist yang di-support via `excluded_tools`)
- Per-request tool override
- UI untuk manage tool restrictions
- Filtering tools di ElasticsearchHandler (ES sepenuhnya diblokir via `allowed_data_sources`, tidak perlu per-tool ES filtering)
- Rate limiting per persona

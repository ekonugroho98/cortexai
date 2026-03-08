# DELTA_SPEC — Persona System + Per-Persona AI Model Selection

**Spec ID:** persona-system
**Mode:** BROWNFIELD
**Complexity:** COMPLEX
**Platform:** backend (Go)
**Status:** Pending Approval
**Created:** 2026-02-27

---

## 1. Background & Motivation

CortexAI saat ini menggunakan **satu LLMRunner global** yang dibuat saat startup dan dipakai oleh semua request. Ini berarti semua user — CEO, developer, dan support engineer — mendapatkan respons AI dengan style dan model yang sama.

Kebutuhan bisnis berbeda per role/persona:
- **Executive**: Ingin insight bisnis ringkas dalam bahasa non-teknis, tanpa melihat SQL
- **Developer**: Ingin SQL detail dengan komentar, penjelasan performa query, optimisasi
- **App Support**: Ingin fokus troubleshooting — timestamp, error codes, stack trace, investigation steps

Persona **berbeda dari Role**. Role mengontrol akses (RBAC: admin/analyst/viewer). Persona mengontrol perilaku AI — model mana yang digunakan, style respons apa, dan max tokens berapa.

---

## 2. Scope of Change

### Yang Berubah
| File | Tipe | Perubahan |
|------|------|-----------|
| `internal/config/config.go` | MODIFY | +`PersonaConfig` struct, +`Config.Personas`, +`UserConfig.Persona` |
| `internal/models/user.go` | MODIFY | +`User.Persona`, +`UserResponse.Persona`, update `ToResponse()` |
| `internal/service/user_store.go` | MODIFY | +`UserEntry.Persona`, pass ke `User` saat build |
| `internal/agent/llm_pool.go` | CREATE | `LLMPool` struct — manages multiple LLMRunner instances |
| `internal/agent/system_prompts.go` | CREATE | Per-persona system prompts (executive, technical, support) |
| `internal/agent/bigquery_handler.go` | MODIFY | Export `BaseSystemPrompt`, refactor cache ke `getSchemaSection()`, `Handle()`/`HandleStream()` terima `runner LLMRunner, promptStyle string` |
| `internal/agent/elasticsearch_handler.go` | MODIFY | Export `ESSystemPrompt`, `Handle()`/`HandleStream()` terima `runner LLMRunner, promptStyle string` |
| `internal/handler/agent.go` | MODIFY | +`llmPool`, +`personas` field, `resolvePersona()` helper, pass ke handlers |
| `internal/server/routes.go` | MODIFY | Ganti factory logic → build `LLMPool` dari `cfg.Personas`, pass ke `AgentHandler` |
| `config/cortexai.example.json` | MODIFY | Tambah `personas` section, update `users` dengan `persona` field |

### Yang TIDAK Berubah
- `BigQueryHandler` dan `ElasticsearchHandler` **constructor** tidak berubah signature
- `LLMRunner` interface tidak berubah
- `CortexAgent` dan `DeepSeekAgent` tidak berubah
- Seluruh middleware chain tidak berubah
- API endpoint paths tidak berubah
- RBAC logic tidak berubah

---

## 3. Functional Requirements

### Must Have (Critical)

- [ ] **REQ-001** Sistem mendukung konfigurasi multiple persona via `personas` map di `cortexai.json`
- [ ] **REQ-002** Setiap persona mendefinisikan: `provider`, `model`, `system_prompt_style`, opsional `base_url` dan `max_tokens`
- [ ] **REQ-003** Setiap user dapat di-assign ke satu persona via `persona` field di config
- [ ] **REQ-004** Per-request, sistem me-resolve LLMRunner yang tepat berdasarkan `user.Persona`
- [ ] **REQ-005** `LLMPool` mengelola multiple LLMRunner instances, keyed by `provider:model`
- [ ] **REQ-006** Dua persona dengan `provider+model` yang sama berbagi satu LLMRunner instance (memory efficient)
- [ ] **REQ-007** System prompt style `executive` menghasilkan respons bisnis ringkas, tanpa SQL jargon
- [ ] **REQ-008** System prompt style `technical` menghasilkan respons detail teknis dengan SQL inline comments dan performance notes
- [ ] **REQ-009** System prompt style `support` menghasilkan respons troubleshooting dengan timestamps, error codes, dan investigation steps
- [ ] **REQ-010** `BigQueryHandler.Handle()` dan `HandleStream()` menerima `runner LLMRunner, promptStyle string` sebagai parameter
- [ ] **REQ-011** `ElasticsearchHandler.Handle()` dan `HandleStream()` menerima `runner LLMRunner, promptStyle string` sebagai parameter
- [ ] **REQ-012** Schema cache di BigQueryHandler hanya menyimpan **schema portion** (bukan base prompt) — persona berbeda berbagi schema cache yang sama
- [ ] **REQ-013** Response `agent_metadata` menyertakan `persona` dan `model` yang digunakan
- [ ] **REQ-014** `GET /api/v1/me` menampilkan `persona` field di UserResponse

### Should Have (Important)

- [ ] **REQ-015** Persona `executive` tidak perlu alat `get_bigquery_sample_data` (optional, bisa diinclude atau exclude dari tool list)
- [ ] **REQ-016** Startup log menampilkan daftar persona yang berhasil ter-register beserta provider+model

### Could Have (Nice to have)

- [ ] **REQ-017** Persona bisa di-override sementara via request field `persona_override` (tidak di scope MVP)

---

## 4. Backward Compatibility Requirements

**WAJIB dipertahankan — tidak boleh ada breaking change:**

1. **No personas in config** → fallback ke legacy `llm_provider` + existing runner behavior. Tidak perlu update config lama.
2. **User tanpa `persona` field** → persona = `"default"`. Jika persona `"default"` tidak ada di config → pakai pool fallback runner + default system prompt.
3. **Legacy `api_keys`** (bukan `users`) → viewer role, no squad, no persona → pool fallback runner + base system prompt.
4. **`BigQueryHandler` constructor** → tetap menerima satu `LLMRunner` (dipakai sebagai default). Constructor signature tidak berubah.
5. **Existing tests** → `extractSQL` tests dan `DeepSeekAgent` tests tidak terpengaruh.

---

## 5. Non-Functional Requirements

- **Performance**: LLMRunner instance di-share via pool (tidak dibuat baru per-request). Overhead resolve persona = O(1) map lookup.
- **Memory**: Minimal — hanya runner instances yang dibutuhkan (deduplicated by provider:model key).
- **Config validation**: Invalid persona reference di user config → log warning, fallback ke default (bukan panic/error startup).
- **Observability**: Setiap request log mencatat persona dan model yang digunakan.

---

## 6. Acceptance Criteria

- [ ] `go build ./...` clean (zero errors)
- [ ] `go test ./...` pass (semua existing test tidak ada regresi)
- [ ] Request dari user dengan persona `executive` → respons tidak mengandung SQL jargon, ringkas
- [ ] Request dari user dengan persona `developer` → respons mengandung SQL dengan komentar
- [ ] Config tanpa `personas` section → server start normal, semua endpoint berfungsi (backward compat)
- [ ] User tanpa `persona` field → mendapat default behavior
- [ ] `GET /api/v1/me` → response mengandung `"persona": "executive"` (atau persona user tersebut)
- [ ] Response `agent_metadata` → mengandung `"persona"` dan `"model"` fields

---

## 7. Out of Scope

- UI/dashboard untuk manage personas
- Runtime persona switching via API (tanpa restart)
- Per-request persona override (`persona_override` field)
- Persona-based tool filtering (e.g., exclude `sample_data` untuk executive)
- A/B testing antar persona

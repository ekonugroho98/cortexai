# Delta Spec: Fix Answer Language Mismatch

**Spec ID:** fix-answer-language-mismatch
**Mode:** BROWNFIELD
**Complexity:** SIMPLE
**Platform:** backend (Go/chi)
**Date:** 2026-03-03
**Status:** Open

---

## Summary

Agent menjawab dalam English meskipun prompt user dalam Bahasa Indonesia (terlihat di scenario 3 dan 5). Root cause: BQ dan PG system prompts tidak memiliki language-matching rule. ES prompts memiliki rule parsial tapi wording tidak konsisten.

---

## Problem Statement

Ketika user mengirim prompt dalam Bahasa Indonesia, beberapa persona (terutama BQ dan PG) menjawab dalam English. Ini terjadi karena LLM default ke English jika tidak ada instruksi bahasa eksplisit.

---

## Changes

### Added

**Language-matching rule** ditambahkan ke semua system prompts yang belum memilikinya:

```
Always respond in the same language as the user's prompt.
If the user writes in Indonesian, respond in Indonesian.
If in English, respond in English.
```

### Modified

**ES system prompts** — rule #4 yang sudah ada diganti dengan wording di atas (konsisten dengan BQ + PG):

**Before:**
```
4. Interpret results and explain findings clearly in Indonesian or English (match user's language)
```

**After:**
```
4. Interpret results and explain findings clearly
5. Always respond in the same language as the user's prompt. If the user writes in Indonesian, respond in Indonesian. If in English, respond in English.
```

---

## Affected Prompts

| Prompt Const | File | Data Source | Change |
|---|---|---|---|
| `BaseSystemPrompt` | `bigquery_handler.go` | BQ | Add rule (new rule #8) |
| `executiveSystemPrompt` | `system_prompts.go` | BQ | Add rule (new rule #8) |
| `technicalSystemPrompt` | `system_prompts.go` | BQ | Add rule (new rule #8) |
| `supportSystemPrompt` | `system_prompts.go` | BQ | Add rule (new rule #8) |
| `PGBaseSystemPrompt` | `system_prompts.go` | PG | Add rule (new rule #10) |
| `pgExecutiveSystemPrompt` | `system_prompts.go` | PG | Add rule (new rule #10) |
| `pgTechnicalSystemPrompt` | `system_prompts.go` | PG | Add rule (new rule #10) |
| `pgSupportSystemPrompt` | `system_prompts.go` | PG | Add rule (new rule #10) |
| `ESSystemPrompt` | `elasticsearch_handler.go` | ES | Replace rule #4, add rule #5 |
| `esExecutiveSystemPrompt` | `system_prompts.go` | ES | Replace rule #4, add rule #5 |
| `esSupportSystemPrompt` | `system_prompts.go` | ES | Replace rule #4, add rule #5 |

**Total:** 11 prompts across 3 files.

---

## Acceptance Criteria

- [ ] Prompt dalam Indonesian → jawaban dalam Indonesian untuk semua persona BQ (executive, technical, support, default)
- [ ] Prompt dalam Indonesian → jawaban dalam Indonesian untuk semua persona PG (executive, technical, support, default)
- [ ] Prompt dalam Indonesian → jawaban dalam Indonesian untuk semua persona ES (executive, support, default)
- [ ] Prompt dalam English → jawaban dalam English (tidak ada regresi)
- [ ] Semua existing tests tetap pass
- [ ] Wording rule language-matching konsisten di semua 11 prompts

---

## Affected Files

```
internal/agent/bigquery_handler.go     ← BaseSystemPrompt
internal/agent/elasticsearch_handler.go ← ESSystemPrompt
internal/agent/system_prompts.go       ← 9 prompts (3 BQ variants, 4 PG, 2 ES variants)
```

---

## Risk Assessment

**Risk Level: LOW**

- Perubahan additive untuk BQ dan PG — hanya menambah rule, tidak mengubah yang lain
- ES: mengganti wording rule yang sudah ada dengan versi lebih eksplisit — tidak menghilangkan intent, hanya memperjelas
- Tidak ada perubahan pada test routing atau fallback behavior
- Existing tests (style routing, fallback, non-empty) tidak akan terpengaruh karena mereka tidak assert konten rule

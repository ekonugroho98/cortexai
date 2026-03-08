# Plan Summary: Fix Answer Language Mismatch

**Spec ID:** fix-answer-language-mismatch | **Mode:** BROWNFIELD | **Complexity:** SIMPLE

---

## Approach

Root Cause: BQ + PG prompts have no language rule; ES prompts have inconsistent partial rule.
Fix: Add `"Always respond in the same language as the user's prompt..."` to all 11 prompts across 3 files.
TDD: Write one table-driven failing test first, then implement file by file.

---

## Tasks

| ID | Name | Files | Type |
|----|------|-------|------|
| 1 | Write failing test for language rule in all prompts | `system_prompts_test.go` | TDD: write test |
| 2 | Add language rule to `BaseSystemPrompt` | `bigquery_handler.go` | Implement |
| 3 | Update `ESSystemPrompt` | `elasticsearch_handler.go` | Implement |
| 4 | Add/update language rule in all 9 variant prompts | `system_prompts.go` | Implement |

---

## Execution Order

Task 1 → Task 2 → Task 3 → Task 4 (sequential — each task makes more prompts pass the test from Task 1)

---

## Risks

| Risk | Level |
|------|-------|
| ES restructuring (renumber rules) could drop existing rule text | 🟢 LOW |

---

## Confidence: ✅ HIGH

Pure string additions, 3 files, no logic changes.

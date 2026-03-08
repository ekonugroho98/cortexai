# Bug Report: LLM Excessive Tool Calls Despite Schema Pre-Injection

**Spec ID:** fix-schema-injection-tool-call-reduction
**Mode:** BUGFIX
**Complexity:** SIMPLE
**Date:** 2026-03-03

---

## Summary

The closing instruction at the end of the schema injection block uses soft, optional language ("you can skip…"). The LLM treats this as a suggestion rather than a directive, and continues calling `get_bigquery_schema` / `get_postgres_schema` (schema re-fetch) and `execute_bigquery_sql` / `execute_postgres_sql` multiple times per request.

---

## Steps to Reproduce

1. Configure a BigQuery or PostgreSQL data source with at least one dataset/database containing tables.
2. Send `POST /api/v1/query-agent` with a natural-language prompt referencing those tables.
3. Observe the agent traces (tools_used in response metadata).
4. **Result:** `get_bigquery_schema` appears (1 extra call) + `execute_bigquery_sql` appears 3–6 times.

---

## Expected Behavior

- LLM reads the pre-injected schema block in the system prompt.
- Goes directly to writing SQL (optionally calling `get_bigquery_sample_data` for JOIN verification).
- Calls `execute_bigquery_sql` / `execute_postgres_sql` **at most 1 time**.
- Never calls `get_bigquery_schema`, `list_tables`, `get_postgres_schema`, or `list_postgres_tables`.

---

## Actual Behavior

Scenario 13 observed:
- 1× `get_bigquery_schema` call (schema re-fetch even though schema is already injected)
- 6× `execute_bigquery_sql` calls (repeated execution with minor variations)

---

## Root Cause

### BQ — `internal/agent/bigquery_handler.go` line 130

Current closing instruction (soft):
```
Since schemas are already provided above, you can skip list_tables and get_bigquery_schema tool calls. Go directly to get_bigquery_sample_data for JOIN queries, then write and execute the SQL.
```

The phrase **"you can skip"** is permissive. LLMs interpret this as optional guidance, not a hard constraint. The instruction also does not address repeated execution (`execute_bigquery_sql` called multiple times).

### PG — `internal/agent/postgres_handler.go` line 104

Same pattern with equivalent soft wording:
```
Since schemas are already provided above, you can skip list_postgres_tables and get_postgres_schema tool calls. Go directly to get_postgres_sample_data for JOIN queries, then write and execute the SQL.
```

---

## Fix

Strengthen the closing instruction in both functions to use explicit directive language with a hard cap on execute calls.

**New wording (BQ):**
```
IMPORTANT: All table schemas are already provided above. DO NOT call list_tables or get_bigquery_schema — go directly to writing and executing SQL. You should need at most 1 execute call.
```

**New wording (PG):**
```
IMPORTANT: All table schemas are already provided above. DO NOT call list_postgres_tables or get_postgres_schema — go directly to writing and executing SQL. You should need at most 1 execute call.
```

---

## Affected Files

| File | Line | Change |
|------|------|--------|
| `internal/agent/bigquery_handler.go` | 130 | Replace `sb.WriteString(...)` closing instruction |
| `internal/agent/postgres_handler.go` | 104 | Replace `sb.WriteString(...)` closing instruction |

---

## Workaround

None. The only mitigation was manual post-processing of repeated results, which is not scalable.

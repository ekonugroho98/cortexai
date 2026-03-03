# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Fixed
- `dry_run=true` with `dataset_id`/`dbName` set now also excludes `list_*_tables`, `get_*_schema`, and `get_*_sample_data` from the LLM tool list, in addition to the execute tool. Previously these schema inspection tools remained available despite the schema already being injected into the system prompt, causing the LLM to call `get_bigquery_schema` redundantly (~2-3s wasted latency per request). Only `list_*_datasets`/`list_*_databases` is retained. Applied to `BigQueryHandler.Handle()`, `HandleStream()`, `PostgresHandler.Handle()`, `HandleStream()`.
- `getSchemaSection()` and `getPGSchemaSection()` closing instruction now uses explicit directive language (`IMPORTANT: … DO NOT call … at most 1 execute call`) instead of the previous soft hint (`you can skip`). The old wording was treated as optional by the LLM, causing redundant `get_bigquery_schema`/`get_postgres_schema` calls and up to 6× repeated `execute_bigquery_sql`/`execute_postgres_sql` calls per request. Constants `BQSchemaClosingInstruction` and `PGSchemaClosingInstruction` are exported for testability.
- All system prompts (BQ, PG, ES — all 11 variants) now instruct the LLM to respond in the same language as the user's prompt. Previously BQ and PG prompts had no language instruction, causing the agent to default to English even when the user wrote in Indonesian. ES prompts had an inconsistent partial rule that has been standardized.
- `SQLValidator` now blocks DML statements (`DELETE FROM`, `INSERT INTO`, `UPDATE...SET`) embedded inside CTE (`WITH ... AS (DML)`) or subquery contexts. Previously, queries like `WITH x AS (DELETE FROM orders RETURNING id) SELECT * FROM x` passed validation because the existing patterns only matched DML preceded by `;`. Three new word-boundary patterns (`\bDELETE\s+FROM\b`, `\bINSERT\s+INTO\b`, `\bUPDATE\s+\w+\s+SET\b`) are added to `sqlDangerousPatterns` — no semicolon anchor — matching DML at any position in the query.
- `dry_run: true` now correctly prevents the LLM from calling `execute_bigquery_sql` and `execute_postgres_sql` during the agent loop.
- `PromptValidator` now blocks raw SQL DML statements (`DELETE FROM`, `DROP`, `INSERT INTO`, `UPDATE...SET`, `ALTER`, `TRUNCATE`, `CREATE`) sent directly as user prompts. Previously these passed validation and reached the agent loop. Patterns use `^` anchor to avoid false positives on natural language queries that mention these words mid-sentence. Previously, the flag only blocked re-execution after the loop completed (post-loop guard), allowing the LLM to execute SQL 6+ times regardless. Fix appends the execute tool to `excludedTools` before `filterTools()` in `Handle()` and `HandleStream()` of both `BigQueryHandler` and `PostgresHandler`.

### Added
- Response cache for exact-match agent queries in `BigQueryHandler.Handle()` and `PostgresHandler.Handle()`. Cache key = `sha256(prompt|datasetID|promptStyle)`, TTL = `schema_cache_ttl` (default 5 min). Cache hit returns response without LLM call; `agent_metadata["response_cache"]` reports `"hit"` or `"miss"`. Errors and `dry_run=true` responses are never cached. `DELETE /api/v1/cache/responses` (admin) flushes all cached responses. `HandleStream()` is excluded from caching (streaming responses are not cacheable).
- Per-persona tool filtering: `PersonaConfig.ExcludedTools []string` — tool names hidden from LLM agent per persona (nil = all tools)
- Per-persona data source restriction: `PersonaConfig.AllowedDataSources []string` — HTTP 403 if persona queries a blocked data source (nil = all sources)
- `filterTools()` in BigQueryHandler — O(1) set-based exclusion applied before LLM agent loop
- `checkDataSourceAllowed()` in AgentHandler — descriptive 403 error before routing (and before SSE headers in streaming path)
- `cortexai.example.json` executive persona configured with `excluded_tools` + `allowed_data_sources` as reference
- Persona system: per-user AI behavior (provider, model, system prompt style, max tokens) via `personas` config map
- `LLMPool` for managing multiple LLMRunner instances, keyed by `provider:model` (memory-efficient deduplication)
- Three BigQuery system prompt styles: `executive` (concise business), `technical` (detailed SQL), `support` (troubleshooting)
- Two Elasticsearch system prompt styles: `executive`, `support`
- `resolvePersona()` in AgentHandler — O(1) per-request persona resolution
- `agent_metadata` now includes `persona` and `model` fields in every agent response
- `GET /api/v1/me` now returns `persona` field in UserResponse
- Startup log lists all registered personas with provider+model
- Multi-squad data isolation (per-squad BigQuery datasets + ES index patterns)
- Role-based access control (admin/analyst/viewer)
- User profile endpoint (`GET /api/v1/me`)
- RBAC middleware (`RequireRole`)
- DeepSeek LLM provider (OpenAI-compatible, pure net/http)
- `LLMRunner` interface for pluggable LLM providers
- SSE streaming for agent queries (`POST /api/v1/query-agent/stream`)
- Schema cache invalidation endpoint (`DELETE /api/v1/cache/schema/{dataset}`)
- BB-Flow framework integration

### Fixed
- Agent loop premature exit when GLM returns "stop" with tool calls
- Empty `generated_sql` when SQL pointer initialized to empty string
- `extractSQL` strategy 1 not stripping trailing semicolons
- `extractSQL` strategy 3b failing on multiline SQL (`\nFROM`)

## [1.0.0] - 2026-02-25

### Added
- Initial Go rewrite of CortexAI (previously Python/FastAPI)
- Anthropic SDK integration for LLM agent loop
- BigQuery NL-to-SQL pipeline with schema pre-loading
- Elasticsearch NL-to-query pipeline
- Intent router (BigQuery vs Elasticsearch auto-detection)
- Security suite: SQL injection, prompt injection, PII detection, data masking
- API key authentication with rate limiting
- Cost tracking and byte limit enforcement
- Audit logging with SHA256 hashing
- Docker support (~15MB image)
- Kubernetes deployment manifests

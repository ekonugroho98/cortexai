# Feature: BigQuery NL→SQL Pipeline

## Overview
Converts natural language prompts to BigQuery SQL via LLM agent with comprehensive security checks.

## Key Files
- `internal/agent/bigquery_handler.go` — Main pipeline orchestrator
- `internal/tools/bq_*.go` — BQ tool definitions
- `internal/service/bigquery.go` — BigQuery SDK wrapper

## Pipeline Steps
1. Squad dataset access check
2. PII detection
3. Prompt validation (30+ patterns)
4. Build tools (list_datasets, list_tables, get_schema, sample_data, execute_query)
5. Build system prompt with pre-loaded schema (cached 5min, singleflight)
6. Run agent loop with timeout
7. Extract SQL (4 strategies) or fallback to lastExecutedSQL
8. SQL validation (24+ patterns)
9. Execute query
10. Cost check, data masking, audit logging

## Schema Cache
- TTL: 5 minutes
- Dedup: singleflight per dataset_id
- Invalidation: `DELETE /api/v1/cache/schema/{dataset}` (admin only)

## SQL Extraction Strategies
1. ` ```sql...``` ` code block (strip trailing `;`)
2. ` ```...``` ` generic code block
3. After `###` markdown heading (sanity check for `FROM` keyword)
4. Fallback: lastExecutedSQL from execute_bigquery_sql tool call

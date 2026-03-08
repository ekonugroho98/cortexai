# Feature: Elasticsearch Integration

## Overview
NL→ES Query DSL pipeline with identifier validation and index pattern filtering.

## Key Files
- `internal/agent/elasticsearch_handler.go` — ES NL→query pipeline
- `internal/service/elasticsearch.go` — ES SDK wrapper + WithPatterns()
- `internal/security/es_prompt_validator.go` — Identifier validation
- `internal/handler/elasticsearch.go` — ES pass-through endpoints
- `internal/tools/es_*.go` — ES tool definitions

## Pipeline Steps
1. PII detection
2. General prompt validation
3. ES identifier validation (requires order_id, user_id, time_range, etc.)
4. Build tools (list_indices, search) with squad-scoped patterns
5. Run agent loop
6. Truncate output to 500 chars

## Squad Isolation
`WithPatterns(allowedPatterns)` — cheap shallow copy of ElasticsearchService with overridden allowedPatterns. Shared underlying ES client.

## Endpoints
- `GET /elasticsearch/health`, `/cluster/info`, `/cluster/health`
- `GET /elasticsearch/indices`, `/indices/{name}`
- `POST /elasticsearch/search`, `/count`, `/aggregate`

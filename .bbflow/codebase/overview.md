# CortexAI — Codebase Overview

## Summary

CortexAI is a high-performance Go rewrite of a Python/FastAPI data intelligence platform. It provides an HTTP API for natural language querying of BigQuery and Elasticsearch, powered by LLM agents (Anthropic Claude or DeepSeek).

## Key Characteristics

- **Language:** Go 1.22+
- **Router:** chi/v5
- **LLM Providers:** Anthropic SDK (Claude/GLM via Z.ai) + DeepSeek (OpenAI-compatible, pure net/http)
- **Data Sources:** Google BigQuery, Elasticsearch v8
- **Architecture:** Layered (cmd → internal → handler/agent/service/security)
- **Binary Size:** ~15MB Docker image
- **Startup:** <1s

## Codebase Statistics

| Metric | Value |
|--------|-------|
| Go source files | 54 |
| Internal packages | 11 |
| API endpoints | 18+ |
| Security patterns | 55+ (SQL + prompt injection) |
| Test files | 5 |

## Core Capabilities

1. **Multi-LLM Agent Loop** — Max 10 iterations, tool calling, force answer at iter 7
2. **BigQuery NL→SQL Pipeline** — Schema pre-loading, 4-strategy SQL extraction, cost tracking
3. **Elasticsearch NL→Query Pipeline** — Identifier validation, index pattern filtering
4. **Security Suite** — SQL/prompt injection prevention, PII detection, data masking, audit logging
5. **RBAC + Squad Isolation** — Role-based access (admin/analyst/viewer), per-squad dataset restrictions
6. **SSE Streaming** — Real-time feedback during agent operations
7. **Schema Caching** — 5min TTL + singleflight deduplication

## Entry Point

`cmd/cortexai/main.go` → loads config → initializes server → graceful shutdown on SIGINT/SIGTERM

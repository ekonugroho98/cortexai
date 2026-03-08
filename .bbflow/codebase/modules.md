# CortexAI — Module Reference

## Package Map

```
internal/
├── agent/          LLM agent loop + data source handlers
├── config/         Configuration loading + defaults
├── handler/        HTTP endpoint handlers
├── middleware/      HTTP middleware chain
├── models/         Request/response/user types
├── security/       Validators, masking, audit
├── server/         Server lifecycle + route registration
├── service/        BigQuery, Elasticsearch, Router, UserStore
└── tools/          LLM tool definitions (BQ + ES)
```

---

## internal/agent

**Purpose:** LLM interaction layer — agent loops and data source orchestration.

| File | Key Types | Responsibility |
|------|-----------|---------------|
| `llm.go` | `LLMRunner` interface, `EmitFn` | Provider abstraction (Run, RunWithEmit, Model) |
| `cortex_agent.go` | `CortexAgent`, `ToolCall` | Anthropic SDK agent loop (10 iter max) |
| `deepseek_agent.go` | `DeepSeekAgent` | OpenAI-compatible agent (pure net/http) |
| `bigquery_handler.go` | `BigQueryHandler`, `schemaCache` | BQ NL→SQL pipeline with security checks |
| `elasticsearch_handler.go` | `ElasticsearchHandler` | ES NL→query pipeline |

**Dependencies:** config, models, security, service, tools

---

## internal/config

**Purpose:** Centralized configuration with JSON + env override.

| File | Key Types | Responsibility |
|------|-----------|---------------|
| `config.go` | `Config`, `PersonaConfig`, `SquadConfig`, `UserConfig` | Load(), JSON + env override; PersonaConfig: ExcludedTools, AllowedDataSources |
| `defaults.go` | constants | Default values, PII keywords, CORS origins |

**Dependencies:** none (leaf package)

---

## internal/handler

**Purpose:** HTTP request/response handling.

| File | Key Types | Responsibility |
|------|-----------|---------------|
| `agent.go` | `AgentHandler` | POST /query-agent, /query-agent/stream |
| `query.go` | `QueryHandler` | POST /query (direct SQL) |
| `datasets.go` | `DatasetHandler` | GET /datasets, /datasets/{id} |
| `tables.go` | `TableHandler` | GET /tables with squad access check |
| `cache.go` | `CacheHandler` | DELETE /cache/schema/{dataset} |
| `health.go` | `HealthHandler` | GET /health |
| `user.go` | `UserHandler` | GET /me |
| `elasticsearch.go` | `ElasticsearchHandler` | ES pass-through endpoints |

**Dependencies:** agent, models, middleware, service, security

---

## internal/middleware

**Purpose:** HTTP middleware chain.

| File | Responsibility |
|------|---------------|
| `auth.go` | API key validation, User context injection |
| `rbac.go` | Role-based access control (RequireRole) |
| `logging.go` | Request/response logging |
| `requestid.go` | UUID generation, X-Request-ID propagation |
| `ratelimit.go` | Sliding window rate limiting |
| `recovery.go` | Panic recovery |
| `cors.go` | CORS headers |
| `security_headers.go` | HSTS, CSP, X-Frame-Options |

**Dependencies:** models

---

## internal/models

**Purpose:** Shared data types.

| File | Key Types |
|------|-----------|
| `request.go` | `QueryRequest`, `AgentRequest` |
| `response.go` | `QueryResponse`, `AgentResponse`, `QueryMetadata` |
| `user.go` | `User`, `Role`, `Squad`, `UserResponse` |
| `elasticsearch.go` | `SearchRequest`, `CountRequest`, `AggregateRequest` |
| `errors.go` | `WriteError()`, `WriteJSON()` |

**Dependencies:** none (leaf package)

---

## internal/security

**Purpose:** Input validation, output protection, audit trail.

| File | Key Functions |
|------|--------------|
| `sql_validator.go` | 24 regex patterns, whitelist keywords, SELECT/WITH required |
| `prompt_validator.go` | 30+ patterns (command exec, path traversal, prompt injection) |
| `es_prompt_validator.go` | Identifier requirement (order_id, user_id, time_range, etc.) |
| `pii_detector.go` | Keyword-based PII detection |
| `data_masker.go` | Email/phone/SSN/credit card masking |
| `cost_tracker.go` | Byte limit enforcement, cost estimation |
| `audit_logger.go` | SHA256-hashed audit trail |

**Dependencies:** none (leaf package)

---

## internal/server

**Purpose:** Server lifecycle and route wiring.

| File | Responsibility |
|------|---------------|
| `server.go` | HTTP server init, graceful shutdown |
| `routes.go` | Route registration, dependency injection, startup checks |

**Dependencies:** all internal packages (wiring point)

---

## internal/service

**Purpose:** External service integrations.

| File | Key Types | Responsibility |
|------|-----------|---------------|
| `bigquery.go` | `BigQueryService`, `QueryResult` | BQ SDK wrapper (list, schema, execute) |
| `elasticsearch.go` | `ElasticsearchService` | ES SDK wrapper + WithPatterns() for isolation |
| `router.go` | `IntentRouter`, `RoutingResult` | Keyword-based BQ vs ES classification |
| `user_store.go` | `UserStore` | API key → User lookup, squad resolution |

**Dependencies:** models, config

---

## internal/tools

**Purpose:** LLM tool definitions for agent function calling.

| File | Tool Name | Responsibility |
|------|-----------|---------------|
| `types.go` | `Tool` struct | Tool interface definition |
| `bq_list_datasets.go` | `list_bigquery_datasets` | Filtered by squad allowlist |
| `bq_get_schema.go` | `get_schema`, `list_tables` | Schema introspection |
| `bq_sample_data.go` | `get_sample_data` | 3-row sample for JOIN verification |
| `bq_execute_query.go` | `execute_bigquery_sql` | SQL execution with result |
| `es_list_indices.go` | `list_elasticsearch_indices` | Index enumeration |
| `es_search.go` | `elasticsearch_search` | Query DSL execution |

**Dependencies:** service

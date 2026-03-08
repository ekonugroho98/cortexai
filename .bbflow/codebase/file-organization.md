# CortexAI — File Organization

```
cortexai/
├── cmd/cortexai/
│   └── main.go                         # Entry point
├── internal/
│   ├── agent/
│   │   ├── llm.go                      # LLMRunner interface
│   │   ├── cortex_agent.go             # Anthropic agent loop
│   │   ├── deepseek_agent.go           # DeepSeek agent (OpenAI-compatible)
│   │   ├── bigquery_handler.go         # BQ NL→SQL pipeline
│   │   ├── bigquery_handler_test.go    # 19 tests (extractSQL, cache)
│   │   ├── deepseek_agent_test.go      # 7 tests
│   │   └── elasticsearch_handler.go    # ES NL→query pipeline
│   ├── config/
│   │   ├── config.go                   # Config struct + Load()
│   │   └── defaults.go                 # Constants
│   ├── handler/
│   │   ├── agent.go                    # /query-agent, /query-agent/stream
│   │   ├── cache.go                    # /cache/schema/{dataset}
│   │   ├── datasets.go                 # /datasets endpoints
│   │   ├── elasticsearch.go            # ES pass-through
│   │   ├── health.go                   # /health
│   │   ├── query.go                    # /query (direct SQL)
│   │   ├── tables.go                   # /tables endpoints
│   │   └── user.go                     # /me endpoint
│   ├── middleware/
│   │   ├── auth.go                     # API key auth + User context
│   │   ├── rbac.go                     # Role-based access control
│   │   ├── logging.go                  # Request logging
│   │   ├── requestid.go               # Request ID + contextKey type
│   │   ├── ratelimit.go               # Sliding window rate limit
│   │   ├── recovery.go                # Panic recovery
│   │   ├── cors.go                    # CORS headers
│   │   ├── security_headers.go        # HSTS, CSP, etc.
│   │   └── middleware_test.go         # Tests
│   ├── models/
│   │   ├── request.go                 # QueryRequest, AgentRequest
│   │   ├── response.go                # QueryResponse, AgentResponse
│   │   ├── user.go                    # User, Role, Squad
│   │   ├── elasticsearch.go           # ES request/response types
│   │   └── errors.go                  # WriteError, WriteJSON
│   ├── security/
│   │   ├── sql_validator.go           # 24 SQL injection patterns
│   │   ├── prompt_validator.go        # 30+ prompt injection patterns
│   │   ├── es_prompt_validator.go     # ES identifier validation
│   │   ├── pii_detector.go            # PII keyword detection
│   │   ├── data_masker.go             # Email/phone/SSN/CC masking
│   │   ├── cost_tracker.go            # Byte limit enforcement
│   │   ├── audit_logger.go            # SHA256-hashed audit trail
│   │   └── security_test.go           # Tests
│   ├── server/
│   │   ├── server.go                  # Server lifecycle
│   │   └── routes.go                  # Route registration + DI
│   ├── service/
│   │   ├── bigquery.go                # BigQuery SDK wrapper
│   │   ├── elasticsearch.go           # Elasticsearch SDK wrapper
│   │   ├── router.go                  # Intent router (BQ vs ES)
│   │   ├── router_test.go            # Router tests
│   │   └── user_store.go             # API key → User store
│   └── tools/
│       ├── types.go                   # Tool interface
│       ├── bq_list_datasets.go        # list_bigquery_datasets
│       ├── bq_get_schema.go           # get_schema + list_tables
│       ├── bq_sample_data.go          # get_sample_data
│       ├── bq_execute_query.go        # execute_bigquery_sql
│       ├── es_list_indices.go         # list_elasticsearch_indices
│       └── es_search.go              # elasticsearch_search
├── config/
│   ├── cortexai.example.json          # Config template (committed)
│   └── cortexai.json                  # Active config (gitignored)
├── deploy/
│   ├── docker/Dockerfile              # Multi-stage build
│   └── k8s/                           # K8s manifests
├── go.mod                             # Dependencies
├── go.sum                             # Checksums
├── Makefile                           # Build automation
├── README.md                          # Project README
└── CLAUDE.md                          # AI session context
```

## Conventions

- **One handler per resource** — `datasets.go`, `tables.go`, `user.go`
- **Tests alongside source** — `*_test.go` in same package
- **Security as leaf package** — no internal dependencies, pure validators
- **Models as leaf package** — shared types, no business logic
- **Server as wiring point** — `routes.go` connects all packages

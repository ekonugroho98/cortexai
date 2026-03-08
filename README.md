# CortexAI — Go Enterprise Intelligence Platform

High-performance Go rewrite of CortexAI (previously Python/FastAPI). HTTP API for querying **BigQuery**, **PostgreSQL**, and **Elasticsearch** using natural language via LLM (Anthropic Claude / GLM / DeepSeek).

## Architecture

```
HTTP Request
    │
    ├─ Middleware: Recovery → RequestID → Logging → SecurityHeaders → CORS → RateLimit → Auth
    │
    └─ /api/v1/
        ├─ GET  /me                               # Current user profile
        ├─ GET  /datasets                         # List BigQuery datasets (viewer+)
        ├─ GET  /datasets/{id}                    # Get dataset info (viewer+)
        ├─ GET  /datasets/{id}/tables             # List tables (viewer+)
        ├─ GET  /datasets/{id}/tables/{table_id}  # Get table schema (viewer+)
        ├─ POST /query                            # Direct SQL execution (analyst+)
        ├─ POST /query-agent                      # NL → SQL/ES/PG via LLM agent (analyst+)
        ├─ POST /query-agent/stream               # NL → SQL/ES/PG, SSE streaming (analyst+)
        ├─ DELETE /cache/schema/{dataset}         # Invalidate BQ schema cache (admin)
        ├─ DELETE /cache/pg-schema/{squad}/{db}   # Invalidate PG schema cache (admin)
        ├─ DELETE /cache/responses                # Flush response cache (admin)
        └─ /elasticsearch/                        # ES endpoints (if enabled)
            ├─ GET  /health
            ├─ GET  /cluster/info
            ├─ GET  /cluster/health
            ├─ GET  /indices
            ├─ GET  /indices/{name}
            ├─ POST /search
            ├─ POST /count
            └─ POST /aggregate
```

### Agent Pipeline

```
POST /query-agent
    │
    ├─ Security checks (PII, prompt injection)
    ├─ Response cache lookup (sha256 key)
    ├─ IntentRouter → scores BQ / PG / ES keywords; tie-break: BQ > PG > ES
    │
    ├─ BigQueryHandler    → schema inject → CortexAgent/DeepSeekAgent loop (max 10 iter)
    ├─ PostgresHandler    → schema inject → LLM loop + EXPLAIN cost check
    └─ ElasticsearchHandler → NL → ES query → LLM loop
           │
           └─ security (SQL/ES validation, cost limit, data masking, audit log)
                    └─ AgentResponse
```

## Quick Start

```bash
# 1. Copy config
cp config/cortexai.example.json config/cortexai.json

# 2. Edit config (API keys, GCP project, PostgreSQL, etc.)
vim config/cortexai.json

# 3. Build & run
make dev

# 4. Test
curl localhost:8000/health
curl -H "X-API-Key: your-key" localhost:8000/api/v1/datasets
```

## Configuration

Config file: `config/cortexai.json` (gitignored). Template: `config/cortexai.example.json`.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CORTEXAI_CONFIG` | Path to JSON config file | — |
| `CORTEXAI_PORT` | HTTP port | `8000` |
| `CORTEXAI_ENV` | Environment (`development`/`production`) | `development` |
| `CORTEXAI_API_KEYS` | Comma-separated legacy API keys | — |
| `GCP_PROJECT_ID` | GCP project ID | — |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to GCP service account JSON | — |
| `ANTHROPIC_API_KEY` | Anthropic / Z.ai compatible API key | — |
| `ANTHROPIC_BASE_URL` | Override Anthropic endpoint (e.g. Z.ai) | — |
| `LLM_PROVIDER` | `anthropic` or `deepseek` | `anthropic` |
| `DEEPSEEK_API_KEY` | DeepSeek API key | — |
| `DEEPSEEK_BASE_URL` | DeepSeek base URL | — |
| `ELASTICSEARCH_ENABLED` | Enable ES integration | `false` |
| `ELASTICSEARCH_HOST` | ES host | `localhost` |
| `POSTGRES_ENABLED` | Enable PostgreSQL integration | `false` |
| `ENABLE_AUTH` | Enable API key auth | `true` |
| `RATE_LIMIT_PER_MINUTE` | Rate limit per client | `60` |

### Multi-LLM Provider

Active model is `glm-4.5-air` via Z.ai (Anthropic-compatible endpoint). Switch providers via `llm_provider`:

```json
{
  "llm_provider": "anthropic",
  "anthropic_api_key": "...",
  "anthropic_base_url": "https://open.bigmodel.cn/api/anthropic/",
  "model_list": { "anthropic": "glm-4.5-air" }
}
```

```json
{
  "llm_provider": "deepseek",
  "deepseek_api_key": "...",
  "deepseek_base_url": "https://api.deepseek.com/v1",
  "model_list": { "deepseek": "deepseek-chat" }
}
```

### Persona System

Per-user AI behavior via `personas` map. Each persona maps to a provider+model+prompt style. Personas sharing the same `provider:model` reuse a single LLMRunner instance (deduplication).

```json
"personas": {
  "executive":   { "provider": "anthropic", "model": "glm-4.5-air", "system_prompt_style": "executive" },
  "developer":   { "provider": "anthropic", "model": "glm-4.5-air", "system_prompt_style": "technical" },
  "app_support": { "provider": "deepseek",  "model": "deepseek-chat", "system_prompt_style": "support" }
}
```

Users without a persona (or with an unknown one) fall back to the default runner and `BaseSystemPrompt`.

Prompt styles — BQ/PG: `executive`, `technical`, `support`; ES: `executive`, `support`.

### Multi-Squad Data Isolation

Each squad defines allowed datasets, ES index patterns, and PG databases. Users are assigned to a squad; admin users (no squad) bypass all restrictions.

```json
"squads": [
  {
    "id": "analytics",
    "name": "Analytics Team",
    "datasets": ["wlt_datalake_01"],
    "es_index_patterns": ["logs-*"],
    "postgres": {
      "host": "pg.internal", "port": 5432,
      "user": "ro_user", "password": "...",
      "databases": ["analytics_db"],
      "ssl_mode": "require", "max_conns": 10
    }
  }
]
```

### User & Role System

Roles: `admin` > `analyst` > `viewer`.

| Role | Access |
|------|--------|
| `viewer` | datasets/tables listing |
| `analyst` | `viewer` + query + query-agent |
| `admin` | all + cache invalidation |

```json
"users": [
  { "id": "alice", "name": "Alice", "role": "analyst", "api_key": "key-alice", "squad_id": "analytics", "persona": "executive" },
  { "id": "bob",   "name": "Bob",   "role": "admin",   "api_key": "key-bob" }
]
```

## API Reference

### `POST /api/v1/query-agent`

```json
{
  "prompt": "tampilkan top 5 user berdasarkan transaksi",
  "dataset_id": "wlt_datalake_01",
  "data_source": "bigquery",
  "timeout": 60,
  "dry_run": false
}
```

`data_source` is optional — auto-detected from keyword scoring if omitted. `dataset_id` is reused as the database name for PostgreSQL.

Response includes `agent_metadata` with `persona`, `model`, `response_cache` (`hit`/`miss`), and other diagnostics.

### `POST /api/v1/query-agent/stream`

Same request body as above. Returns Server-Sent Events:

```
data: {"type":"llm_call","iteration":1}
data: {"type":"tool_call","name":"get_bigquery_schema","iteration":1}
data: {"type":"result","data":{...AgentResponse...}}
```

## Security Features

- **Auth**: `X-API-Key` header validation with role-based access control
- **Rate limiting**: Sliding window per IP/API key
- **SQL injection prevention**: 30+ dangerous pattern detection (BQ + PG-specific)
- **Prompt injection prevention**: 30+ patterns, Indonesian + English keywords
- **DML blocking**: `DELETE/DROP/INSERT/UPDATE/ALTER/TRUNCATE/CREATE` from NL prompts
- **PII detection**: Keyword-based blocking
- **Data masking**: Email, phone, SSN, credit card masking in results
- **Cost tracking**: BigQuery byte limit + PostgreSQL EXPLAIN cost enforcement
- **Audit logging**: SHA256-hashed audit trail
- **Security headers**: HSTS, CSP, X-Frame-Options, etc.
- **Squad isolation**: Per-squad dataset/index/database allow-lists

## Caching

| Cache | TTL | Key | Scope |
|-------|-----|-----|-------|
| BQ schema | 5 min (singleflight) | `datasetID` | all personas |
| PG schema | 5 min | `squadID:dbName` | all personas |
| Response | 5 min | `sha256(prompt\|datasetID\|promptStyle)` | per handler |

- `dry_run=true` and error responses are never cached.
- `DELETE /api/v1/cache/responses` flushes the response cache (admin).
- `DELETE /api/v1/cache/schema/{dataset}` and `/cache/pg-schema/{squad}/{db}` invalidate schema caches (admin).

## Development

```bash
make dev            # build + run
make build          # go build -o bin/cortexai ./cmd/cortexai
go test ./...       # all tests (134 tests)
make lint           # linter
make docker-build   # ~15MB image
```

## Test Coverage

| Package | Tests |
|---------|-------|
| `internal/middleware` | 12 — auth, ratelimit, cors |
| `internal/security` | 14 — PII, prompt validator, SQL validator, PG cost tracker |
| `internal/service` | 12 — intent router (+PG), PG pool, user store |
| `internal/agent` | 67 — LLMPool, system prompts, extractSQL, schemaCache, DeepSeek, BQ/PG handlers, response cache |
| `internal/tools` | 10 — PG tools |
| `internal/handler` | 8 — checkDataSourceAllowed, RBAC |
| **Total** | **134** |

## Key Design Decisions

1. **UNION ALL SELECT allowed** — legitimate BigQuery multi-table combine, not blocked by SQL validator
2. **Schema pre-loading** — all table schemas injected into system prompt before agent run; only schema section is cached (not base prompt), all personas share the cache
3. **singleflight** — deduplicates concurrent BQ schema fetches; only 1 fetch per dataset at a time
4. **lastExecutedSQL fallback** — if LLM omits SQL code block, SQL is recovered from the last `execute_bigquery_sql` tool call
5. **Force final answer** — after iteration 7, injects "berikan jawaban final sekarang" to prevent loops
6. **dry_run tool exclusion** — when `dry_run=true` and a dataset/db is set, `list_tables`, `get_schema`, `get_sample_data`, and `execute_*` tools are all excluded; only `list_datasets` is retained
7. **3-way intent routing** — BQ / PG / ES keyword scoring; tie-break: BQ > PG > ES
8. **GLM stop reasons** — handles `stop`, `stop_sequence`, `max_tokens` in addition to `end_turn`
9. **LLMPool deduplication** — personas sharing `provider:model` reuse one LLMRunner instance

## Tech Stack

- **Go 1.22+**
- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/anthropics/anthropic-sdk-go` — LLM client (Anthropic / GLM)
- `cloud.google.com/go/bigquery` — BigQuery SDK
- `github.com/elastic/go-elasticsearch/v8` — ES client
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/rs/zerolog` — structured logging
- `golang.org/x/sync/singleflight` — schema cache dedup

## Docker

```bash
make docker-build
docker run -p 8000:8000 \
  -e ANTHROPIC_API_KEY=sk-... \
  -e GCP_PROJECT_ID=my-project \
  -e CORTEXAI_CONFIG=/etc/cortexai/config.json \
  cortexai:latest
```

## Kubernetes

```bash
# Edit deploy/k8s/configmap.yaml and secret.yaml first
make k8s-apply
kubectl get pods -l app=cortexai
```

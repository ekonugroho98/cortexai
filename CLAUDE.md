# CortexAI — Go Enterprise Intelligence Platform

## Project Overview
Go rewrite of CortexAI (previously Python/FastAPI). HTTP API untuk query BigQuery dan Elasticsearch menggunakan natural language via LLM (Anthropic Claude / GLM via Anthropic-compatible endpoint).

## Working Directory
`/Users/macbookpro/work/project/cortexai`

## Key Commands
```bash
make dev          # build + run dengan config/cortexai.json (fallback ke example)
make build        # go build -o bin/cortexai ./cmd/cortexai
go test ./...     # semua test
```

## LLM Configuration
- Provider: Anthropic SDK (`github.com/anthropics/anthropic-sdk-go`)
- Model aktif: `glm-4.5-air` via Z.ai (Anthropic-compatible endpoint)
- Base URL: `https://open.bigmodel.cn/api/anthropic/`
- Config: `config/cortexai.json` (gitignored, berisi API key asli)
- Template: `config/cortexai.example.json`

## Architecture
```
Request → middleware (auth, ratelimit, logging)
        → handler/agent.go
        → IntentRouter (BigQuery vs Elasticsearch)
        → BigQueryHandler / ElasticsearchHandler
        → CortexAgent.Run() — LLM tool-calling loop (max 10 iter, force answer at 7)
        → security checks (PII, prompt validation, SQL validation)
        → execute + cost check + data masking + audit log
        → AgentResponse
```

## Key Files
| File | Keterangan |
|------|-----------|
| `internal/agent/cortex_agent.go` | LLM loop, tool execution, lastExecutedSQL tracking |
| `internal/agent/bigquery_handler.go` | Schema cache (5min TTL + singleflight), SQL extraction (4 strategies) |
| `internal/agent/elasticsearch_handler.go` | ES NL→query pipeline |
| `internal/service/router.go` | Intent routing BQ vs ES berdasarkan keyword scoring |
| `internal/security/prompt_validator.go` | 30+ dangerous patterns, Indonesian + English keywords |
| `internal/security/sql_validator.go` | SQL injection prevention, UNION ALL SELECT diizinkan |
| `internal/config/config.go` | Config struct, env override, AnthropicBaseURL |

## API Endpoints
```
GET  /health
GET  /api/v1/datasets
GET  /api/v1/datasets/{id}
GET  /api/v1/datasets/{id}/tables
GET  /api/v1/datasets/{id}/tables/{table_id}
POST /api/v1/query              # direct SQL execution
POST /api/v1/query-agent        # NL → SQL/ES via LLM agent
GET  /api/v1/elasticsearch/*    # ES endpoints (jika enabled)
```

## AgentRequest Fields
```json
{
  "prompt": "tampilkan top 5 user",
  "dataset_id": "wlt_datalake_01",
  "data_source": "bigquery",   // optional, auto-detect jika tidak diisi
  "timeout": 60,               // detik
  "dry_run": false
}
```

## Important Design Decisions
1. **UNION ALL SELECT diizinkan** — legitimate BigQuery multi-table combine
2. **Schema pre-loading** — semua table schema diinjek ke system prompt sebelum agent run
3. **singleflight** — dedup concurrent BQ schema fetch, hanya 1 fetch per dataset sekaligus
4. **lastExecutedSQL fallback** — jika LLM tidak wrap SQL dalam code block, pakai SQL dari tool call terakhir
5. **Force final answer** — setelah iter ke-7, inject "berikan jawaban final sekarang"
6. **Router tie-breaking** — jika ES score == BQ score, default ke BigQuery
7. **GLM stop reasons** — handle `stop`, `stop_sequence`, `max_tokens` selain `end_turn`

## Credentials (gitignored)
- `config/cortexai.json` — config dengan API key asli
- `credentials/` — GCP service account JSON

## Tech Stack
- Go 1.22+
- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/anthropics/anthropic-sdk-go` — LLM client
- `cloud.google.com/go/bigquery` — BigQuery SDK
- `github.com/elastic/go-elasticsearch/v8` — ES client
- `github.com/rs/zerolog` — structured logging
- `golang.org/x/sync/singleflight` — schema cache dedup

## Test Status
```
ok  internal/middleware    (auth, ratelimit, cors)
ok  internal/security      (PII, prompt validator, SQL validator)
ok  internal/service       (intent router)
-   internal/agent         (no tests yet)
-   internal/handler       (no tests yet)
-   internal/tools         (no tests yet)
```

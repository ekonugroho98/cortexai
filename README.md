# CortexAI — Enterprise Intelligence Platform (Go Rewrite)

High-performance Go rewrite of CortexAI, replacing Python/FastAPI + `claude --print` subprocess with native Go + Anthropic SDK.

## Architecture

```
HTTP Request
    │
    ├─ Middleware chain: Recovery → Logging → SecurityHeaders → CORS → Auth → RateLimit
    │
    ├─ GET /health                        # Health check (no auth)
    │
    └─ /api/v1/
        ├─ GET  /datasets                 # List BigQuery datasets
        ├─ GET  /datasets/{id}            # Get dataset info
        ├─ GET  /datasets/{id}/tables     # List tables in dataset
        ├─ GET  /datasets/{id}/tables/{id} # Get table schema
        ├─ POST /query                    # Direct SQL execution (SELECT only)
        ├─ POST /query-agent              # AI agent (auto-routes BQ/ES)
        └─ /elasticsearch/
            ├─ GET  /health
            ├─ GET  /cluster/info
            ├─ GET  /cluster/health
            ├─ GET  /indices
            ├─ GET  /indices/{name}
            ├─ POST /search
            ├─ POST /count
            └─ POST /aggregate
```

## Key Improvements Over Python Version

| Aspect | Python (old) | Go (new) |
|--------|--------------|----------|
| AI Engine | `claude --print` subprocess | Anthropic SDK native |
| BigQuery | Python SDK | `cloud.google.com/go/bigquery` |
| Elasticsearch | curl subprocess | `go-elasticsearch/v8` native |
| Docker image | ~500MB | ~15MB |
| Startup time | 5-10s | <1s |
| Memory usage | 200MB+ | <64MB |

## Quick Start

```bash
# 1. Copy config
cp config/cortexai.example.json config/cortexai.json

# 2. Edit config (set your API keys, GCP project, etc.)
vim config/cortexai.json

# 3. Build & run
make build
CORTEXAI_CONFIG=config/cortexai.json ./bin/cortexai

# 4. Test
curl localhost:8000/health
curl -H "X-API-Key: your-key" localhost:8000/api/v1/datasets
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CORTEXAI_CONFIG` | Path to JSON config file | — |
| `CORTEXAI_PORT` | HTTP port | `8000` |
| `CORTEXAI_ENV` | Environment (development/production) | `development` |
| `CORTEXAI_API_KEYS` | Comma-separated API keys | — |
| `GCP_PROJECT_ID` | GCP project ID | — |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to GCP service account JSON | — |
| `ANTHROPIC_API_KEY` | Anthropic API key | — |
| `ELASTICSEARCH_ENABLED` | Enable ES integration | `false` |
| `ELASTICSEARCH_HOST` | ES host | `localhost` |
| `ENABLE_AUTH` | Enable API key auth | `true` |
| `RATE_LIMIT_PER_MINUTE` | Rate limit per client | `60` |

## Security Features

- **Auth**: `X-API-Key` header validation
- **Rate limiting**: Sliding window per IP/API key
- **SQL injection prevention**: 25+ dangerous pattern detection
- **Prompt injection prevention**: 30+ pattern detection
- **ES identifier requirement**: Requires specific identifiers
- **PII detection**: Keyword-based PII blocking
- **Data masking**: Email, phone, SSN, credit card masking
- **Cost tracking**: BigQuery byte limit enforcement
- **Audit logging**: SHA256-hashed audit trail
- **Security headers**: HSTS, CSP, X-Frame-Options, etc.

## Development

```bash
make test           # Run all tests
make test-security  # Run security tests only
make lint           # Run linter
make docker-build   # Build Docker image (~15MB)
```

## Docker

```bash
make docker-build
docker run -p 8000:8000 \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  -e GCP_PROJECT_ID=my-project \
  -e CORTEXAI_API_KEYS=my-secret-key \
  cortexai:latest
```

## Kubernetes

```bash
# Edit deploy/k8s/configmap.yaml and secret.yaml first
make k8s-apply
kubectl get pods -l app=cortexai
```

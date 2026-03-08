# CortexAI — Technology Stack

## Runtime

| Component | Version | Purpose |
|-----------|---------|---------|
| Go | 1.22+ | Language runtime |

## Core Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5` | HTTP router (lightweight, stdlib-compatible) |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic Claude LLM SDK |
| `cloud.google.com/go/bigquery` | Google BigQuery client |
| `github.com/elastic/go-elasticsearch/v8` | Elasticsearch v8 client |
| `github.com/rs/zerolog` | Structured JSON logging |
| `github.com/google/uuid` | UUID generation (request IDs) |
| `google.golang.org/api` | GCP API foundation |
| `golang.org/x/sync/singleflight` | Schema cache deduplication |

## Standard Library Usage

| Package | Purpose |
|---------|---------|
| `net/http` | HTTP server + DeepSeek client |
| `encoding/json` | JSON marshal/unmarshal |
| `context` | Request context, cancellation, timeouts |
| `regexp` | Security pattern matching |
| `crypto/sha256` | Audit log hashing |
| `sync` | Mutex for caches, rate limiter |
| `os/signal` | Graceful shutdown (SIGINT/SIGTERM) |

## Deployment

| Tool | Purpose |
|------|---------|
| Docker | Multi-stage build (~15MB Alpine image) |
| Kubernetes | Production deployment (3 replicas, HPA) |
| Make | Build automation (`make dev`, `make build`, `make test`) |

## Testing

- Go standard `testing` package
- No external test framework
- No mock library (manual mocks where needed)

## Not Used (Intentional)

- No ORM (direct BigQuery SDK)
- No external DeepSeek SDK (pure net/http)
- No database (config-driven user/squad management)
- No message queue (synchronous request/response + SSE)

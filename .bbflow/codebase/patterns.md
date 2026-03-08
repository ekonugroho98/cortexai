# CortexAI — Coding Patterns & Conventions

## Dependency Injection

All handlers and services receive dependencies via constructor:

```go
func NewBigQueryHandler(
    llm      agent.LLMRunner,      // interface, not concrete type
    bqSvc    *service.BigQueryService,
    pii      *security.PIIDetector,
    prompt   *security.PromptValidator,
    sql      *security.SQLValidator,
    cost     *security.CostTracker,
    masker   *security.DataMasker,
    audit    *security.AuditLogger,
) *BigQueryHandler
```

Wiring happens in `server/routes.go`.

## Interface-Based Abstraction

```go
// LLMRunner — swappable provider
type LLMRunner interface {
    Run(ctx, system, user string, tools []tools.Tool) (string, error)
    RunWithEmit(ctx, system, user string, tools []tools.Tool, emit EmitFn) (string, error)
    Model() string
}

// UserLookup — middleware ↔ service decoupling
type UserLookup interface {
    GetByKey(apiKey string) (*models.User, bool)
}
```

## Error Handling

- Functions return `(value, error)` tuples
- Soft failures: schema fetch returns base prompt on error (doesn't block request)
- JSON error responses via `models.WriteError(w, status, message)`

## Context Pattern

```go
// Inject in middleware
ctx = context.WithValue(ctx, userKey, user)

// Extract in handler
user := middleware.GetCurrentUser(r.Context())
```

## Middleware Chain Order

```
Recovery → RequestID → Logging → SecurityHeaders → CORS → Auth → RateLimit
```

RBAC applied per route group, not globally.

## Configuration Precedence

```
defaults.go (code) → cortexai.json (file) → env vars (override)
```

Environment variables always win.

## Naming Conventions

| Item | Convention | Example |
|------|-----------|---------|
| Files | snake_case | `bigquery_handler.go` |
| Packages | lowercase | `internal/agent` |
| Exported functions | PascalCase | `HandleStream()` |
| Unexported functions | camelCase | `buildSystemPrompt()` |
| Constants | PascalCase | `DefaultPort` |
| Interfaces | PascalCase, -er suffix | `LLMRunner`, `UserLookup` |

## Testing Pattern

- Standard `testing` package (no frameworks)
- Test files: `*_test.go` alongside source
- Table-driven tests for exhaustive cases (e.g., `extractSQL` 19 cases)
- No mocks for external services yet (integration tests needed)

## Logging

```go
zerolog.Info().
    Str("dataset_id", datasetID).
    Int("table_count", len(tables)).
    Msg("schema loaded")
```

Structured JSON, hashed PII in audit logs.

## Caching Strategy

```go
type schemaCache struct {
    mu      sync.Mutex
    entries map[string]*schemaCacheEntry
    sf      singleflight.Group
}
```

TTL-based with singleflight deduplication. Cleanup goroutine not needed (entries expire naturally).

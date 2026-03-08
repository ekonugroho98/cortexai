# CortexAI — Dependency Map

## Internal Package Dependencies

```
cmd/cortexai/main.go
  → config
  → server

server/routes.go (wiring point)
  → config, models, middleware, handler, agent, service, security, tools

handler/*
  → agent, models, middleware, service, security

agent/*
  → config, models, security, service, tools

service/*
  → models, config

middleware/*
  → models

security/*
  → (none — leaf package)

models/*
  → (none — leaf package)

tools/*
  → service
```

## Dependency Direction

```
cmd → server → handler → agent → service → (external SDKs)
                  ↓          ↓        ↓
              middleware   security   tools
                  ↓          ↓        ↓
               models     (leaf)   service
                  ↓
               (leaf)
```

No circular dependencies. `models` and `security` are leaf packages.

## External Dependency Risk

| Dependency | Risk | Mitigation |
|-----------|------|-----------|
| Anthropic SDK | API changes, alpha version | LLMRunner interface abstracts away |
| BigQuery SDK | Stable, Google-maintained | Low risk |
| Elasticsearch SDK | Major version changes | Pinned to v8 |
| chi/v5 | Stable, minimal API | Low risk |
| zerolog | Stable, no breaking changes | Low risk |

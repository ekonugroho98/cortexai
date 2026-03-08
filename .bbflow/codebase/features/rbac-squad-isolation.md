# Feature: RBAC + Squad-Based Data Isolation

## Overview
Role-based access control with per-squad dataset and ES index pattern restrictions.

## Key Files
- `internal/models/user.go` — User, Role, Squad types
- `internal/middleware/auth.go` — API key auth, User context injection
- `internal/middleware/rbac.go` — RequireRole() middleware
- `internal/service/user_store.go` — API key → User mapping
- `internal/config/config.go` — SquadConfig, UserConfig

## Roles
| Role | Permissions |
|------|-----------|
| admin | All endpoints + cache invalidation |
| analyst | Query + agent + datasets |
| viewer | Dataset/table listing only |

## Squad Isolation
- Each squad has: `Datasets[]` (BQ) + `ESIndexPatterns[]` (ES)
- BQ: `bq_list_datasets` tool pre-filtered; handler validates access
- ES: `WithPatterns()` creates shallow copy with scoped patterns
- Admin users (empty `squad_id`) bypass all restrictions

## Config
```json
{
  "squads": [
    {"id": "analytics", "name": "Analytics", "datasets": ["datalake_01"], "es_index_patterns": ["logs-*"]}
  ],
  "users": [
    {"id": "user1", "name": "Analyst", "role": "analyst", "api_key": "...", "squad_id": "analytics"}
  ]
}
```

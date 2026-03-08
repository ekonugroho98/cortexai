# Feature: Security Suite

## Overview
Comprehensive security layer with input validation, output protection, and audit trail.

## Key Files
- `internal/security/sql_validator.go` — SQL injection prevention (24 patterns)
- `internal/security/prompt_validator.go` — Prompt injection prevention (30+ patterns)
- `internal/security/es_prompt_validator.go` — ES identifier validation
- `internal/security/pii_detector.go` — PII keyword detection
- `internal/security/data_masker.go` — Column-based data masking
- `internal/security/cost_tracker.go` — Query cost enforcement
- `internal/security/audit_logger.go` — SHA256-hashed audit trail

## Security Layers
1. **Input:** SQL injection patterns, prompt injection patterns, PII keywords, ES identifier requirement
2. **Execution:** Cost tracking (byte limits), query timeout
3. **Output:** Data masking (email, phone, SSN, credit card), audit logging
4. **Transport:** HSTS, CSP, X-Frame-Options, rate limiting, CORS

## Notable Rules
- `UNION ALL SELECT` is **allowed** (legitimate BigQuery pattern)
- Queries must start with `SELECT` or `WITH` (CTEs)
- ES queries require a specific identifier (order_id, user_id, etc.)
- Audit logs use SHA256 hashing for PII protection

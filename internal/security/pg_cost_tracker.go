package security

import (
	"crypto/sha256"
	"fmt"

	"github.com/rs/zerolog/log"
)

// PGCostTracker enforces PostgreSQL EXPLAIN cost limits.
type PGCostTracker struct {
	maxCost float64
}

// NewPGCostTracker creates a tracker with the given maximum allowed EXPLAIN cost.
// A maxCost of 0 means no limit.
func NewPGCostTracker(maxCost float64) *PGCostTracker {
	return &PGCostTracker{maxCost: maxCost}
}

// CheckCost returns (true, "") if cost is within limits, or (false, reason) if exceeded.
func (t *PGCostTracker) CheckCost(cost float64) (bool, string) {
	if t.maxCost <= 0 {
		return true, ""
	}
	if cost <= t.maxCost {
		return true, ""
	}
	return false, fmt.Sprintf(
		"Query cost limit exceeded. EXPLAIN cost: %.2f, Limit: %.2f",
		cost, t.maxCost,
	)
}

// LogQueryCost logs PG query cost info with hashed identifiers.
func (t *PGCostTracker) LogQueryCost(sql string, cost float64, apiKey string, durationMs int64) {
	h := sha256.Sum256([]byte(sql))
	sqlHash := fmt.Sprintf("%x", h)[:16]
	kh := sha256.Sum256([]byte(apiKey))
	keyHash := fmt.Sprintf("%x", kh)[:16]

	log.Info().
		Str("event", "pg_query_cost").
		Str("sql_hash", sqlHash).
		Str("api_key_hash", keyHash).
		Float64("explain_cost", cost).
		Int64("duration_ms", durationMs).
		Msgf("PG query cost: %.2f | Duration: %dms | SQL: %s... | API: %s...",
			cost, durationMs, sqlHash, keyHash)
}

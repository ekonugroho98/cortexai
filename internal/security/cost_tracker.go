package security

import (
	"crypto/sha256"
	"fmt"

	"github.com/rs/zerolog/log"
)

const bytesPerGB = 1_000_000_000.0
const bigQueryCostPerTB = 5.0 // USD

// CostTracker enforces BigQuery query byte limits
type CostTracker struct {
	maxBytes int64
}

func NewCostTracker(maxBytes int64) *CostTracker {
	return &CostTracker{maxBytes: maxBytes}
}

// CheckLimits returns an error string if bytes exceed the limit
func (ct *CostTracker) CheckLimits(totalBytesProcessed int64, apiKey string) (bool, string) {
	if totalBytesProcessed <= ct.maxBytes {
		return true, ""
	}
	processedGB := float64(totalBytesProcessed) / bytesPerGB
	limitGB := float64(ct.maxBytes) / bytesPerGB
	return false, fmt.Sprintf(
		"Query cost limit exceeded. Processed: %.2fGB, Limit: %.2fGB",
		processedGB, limitGB,
	)
}

// LogQueryCost logs query cost info with hashed identifiers
func (ct *CostTracker) LogQueryCost(sql string, totalBytesProcessed int64, apiKey string, durationMs int64) {
	processedGB := float64(totalBytesProcessed) / bytesPerGB
	costUSD := processedGB / 1000.0 * bigQueryCostPerTB // GB → TB → cost

	sqlHash := hashStr(sql)[:16]
	keyHash := hashStr(apiKey)[:16]

	log.Info().
		Str("event", "query_cost").
		Str("sql_hash", sqlHash).
		Str("api_key_hash", keyHash).
		Float64("cost_gb", processedGB).
		Float64("cost_usd", costUSD).
		Int64("duration_ms", durationMs).
		Msgf("Query cost: %.4fGB ($%.4f) | Duration: %dms | SQL: %s... | API: %s...",
			processedGB, costUSD, durationMs, sqlHash, keyHash)
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

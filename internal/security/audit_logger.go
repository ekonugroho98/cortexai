package security

import (
	"github.com/rs/zerolog/log"
)

// AuditLogger logs security-relevant events with hashed identifiers
type AuditLogger struct {
	enabled bool
}

func NewAuditLogger(enabled bool) *AuditLogger {
	return &AuditLogger{enabled: enabled}
}

// LogQuery records a BigQuery execution event
func (a *AuditLogger) LogQuery(
	sql, apiKey, userContext string,
	executionTimeMs int64,
	rowCount int,
	bytesProcessed int64,
	success bool,
	errMsg string,
) {
	if !a.enabled {
		return
	}
	sqlHash := hashStr(sql)[:16]
	keyHash := hashStr(apiKey)[:16]

	evt := log.Info().
		Str("event", "query_audit").
		Str("sql_hash", sqlHash).
		Str("api_key_hash", keyHash).
		Str("user_context", userContext).
		Int64("execution_time_ms", executionTimeMs).
		Int("row_count", rowCount).
		Int64("bytes_processed", bytesProcessed).
		Bool("success", success)

	if errMsg != "" {
		evt = evt.Str("error", errMsg)
	}
	evt.Msg("audit")
}

// LogAIAgentRequest records an AI agent request event
func (a *AuditLogger) LogAIAgentRequest(
	prompt, apiKey, generatedSQL string,
	validationPassed bool,
	executionTimeMs int64,
) {
	if !a.enabled {
		return
	}
	promptHash := hashStr(prompt)[:16]
	keyHash := hashStr(apiKey)[:16]
	sqlHash := ""
	if generatedSQL != "" {
		sqlHash = hashStr(generatedSQL)[:16]
	}

	log.Info().
		Str("event", "agent_audit").
		Str("prompt_hash", promptHash).
		Str("api_key_hash", keyHash).
		Str("sql_hash", sqlHash).
		Bool("validation_passed", validationPassed).
		Int64("execution_time_ms", executionTimeMs).
		Msg("agent audit")
}

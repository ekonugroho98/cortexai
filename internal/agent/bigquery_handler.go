package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/cortexai/cortexai/internal/tools"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

const schemaCacheTTL = 5 * time.Minute

// schemaCache holds pre-built system prompts keyed by dataset ID.
type schemaCacheEntry struct {
	prompt    string
	expiresAt time.Time
}

type schemaCache struct {
	mu    sync.RWMutex
	store map[string]schemaCacheEntry
	sf    singleflight.Group // deduplicate concurrent fetches for the same dataset
}

func newSchemaCache() *schemaCache {
	return &schemaCache{store: make(map[string]schemaCacheEntry)}
}

func (c *schemaCache) get(datasetID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[datasetID]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.prompt, true
}

func (c *schemaCache) set(datasetID, prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[datasetID] = schemaCacheEntry{
		prompt:    prompt,
		expiresAt: time.Now().Add(schemaCacheTTL),
	}
}

func (c *schemaCache) invalidate(datasetID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, datasetID)
}

const baseSystemPrompt = `You are CortexAI, an expert data analyst with deep knowledge of BigQuery SQL.

Your task is to help users query their BigQuery data using natural language.

RULES:
1. Generate only SELECT queries - never INSERT, UPDATE, DELETE, DROP, or DDL
2. Always add LIMIT clause (max 1000 rows) unless user specifies otherwise
3. Use fully qualified table names: ` + "`dataset.table`" + `
4. ALWAYS wrap your final SQL in a code block exactly like this:
` + "```sql" + `
SELECT ...
` + "```" + `
5. Execute the SQL exactly once after writing it
6. Explain results in plain language
7. For JOIN queries: use get_bigquery_sample_data to verify join key values match before executing`

// buildSystemPrompt returns a cached system prompt pre-loaded with dataset schema.
// Cache TTL is 5 minutes. Concurrent requests for the same dataset share a single
// fetch via singleflight — only one BigQuery call is made regardless of how many
// goroutines call this simultaneously.
func (h *BigQueryHandler) buildSystemPrompt(ctx context.Context, datasetID string) string {
	if datasetID == "" || h.bq == nil {
		return baseSystemPrompt
	}

	// Cache hit — fast path, no lock contention beyond RLock
	if prompt, ok := h.schemaCache.get(datasetID); ok {
		log.Debug().Str("dataset", datasetID).Msg("schema cache hit")
		return prompt
	}

	// Cache miss — use singleflight so concurrent requests for the same dataset
	// share one BigQuery fetch instead of all hitting BQ at the same time.
	v, err, _ := h.schemaCache.sf.Do(datasetID, func() (interface{}, error) {
		// Double-check cache inside singleflight in case another goroutine
		// already populated it while we were waiting to enter.
		if prompt, ok := h.schemaCache.get(datasetID); ok {
			return prompt, nil
		}

		log.Debug().Str("dataset", datasetID).Msg("schema cache miss, fetching from BigQuery")
		fetchStart := time.Now()

		tables, err := h.bq.ListTables(ctx, datasetID)
		if err != nil {
			return baseSystemPrompt, nil // soft fail — return base prompt, don't cache
		}

		var sb strings.Builder
		sb.WriteString(baseSystemPrompt)
		sb.WriteString("\n\n## Available Dataset: " + datasetID + "\n")
		sb.WriteString("The following tables and schemas are already available to you:\n\n")

		for _, tbl := range tables {
			schema, meta, err := h.bq.GetTableSchema(ctx, datasetID, tbl.ID)
			if err != nil {
				log.Warn().Err(err).Str("table", tbl.ID).Msg("pre-load schema: get schema failed")
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s.%s (%d rows)\n", datasetID, tbl.ID, meta.NumRows))
			sb.WriteString(service.SchemaToString(schema))
			sb.WriteString("\n")
		}

		sb.WriteString("\nSince schemas are already provided above, you can skip list_tables and get_bigquery_schema tool calls. Go directly to get_bigquery_sample_data for JOIN queries, then write and execute the SQL.")

		prompt := sb.String()
		h.schemaCache.set(datasetID, prompt)

		log.Info().
			Str("dataset", datasetID).
			Int("tables", len(tables)).
			Dur("fetch_ms", time.Since(fetchStart)).
			Msg("schema cached")

		return prompt, nil
	})

	if err != nil || v == nil {
		return baseSystemPrompt
	}
	return v.(string)
}

// BigQueryHandler orchestrates the NL→SQL→execute pipeline
type BigQueryHandler struct {
	agent        *CortexAgent
	bq           *service.BigQueryService
	piiDetector  *security.PIIDetector
	promptVal    *security.PromptValidator
	sqlVal       *security.SQLValidator
	costTracker  *security.CostTracker
	dataMasker   *security.DataMasker
	auditLogger  *security.AuditLogger
	schemaCache  *schemaCache
}

// NewBigQueryHandler creates a handler with all security components wired in
func NewBigQueryHandler(
	agent *CortexAgent,
	bq *service.BigQueryService,
	piiDetector *security.PIIDetector,
	promptVal *security.PromptValidator,
	sqlVal *security.SQLValidator,
	costTracker *security.CostTracker,
	dataMasker *security.DataMasker,
	auditLogger *security.AuditLogger,
) *BigQueryHandler {
	return &BigQueryHandler{
		agent:       agent,
		bq:          bq,
		piiDetector: piiDetector,
		promptVal:   promptVal,
		sqlVal:      sqlVal,
		costTracker: costTracker,
		dataMasker:  dataMasker,
		schemaCache: newSchemaCache(),
		auditLogger: auditLogger,
	}
}

// Handle processes an agent request for BigQuery
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string) (*models.AgentResponse, error) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "bigquery",
		"model":       h.agent.model,
		"method":      "agent",
	}

	// 1. PII detection
	if found, kw := h.piiDetector.Detect(req.Prompt); found {
		metadata["pii_check"] = "blocked: " + kw
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("PII detected in prompt: %s", kw)
	}
	metadata["pii_check"] = "passed"

	// 2. Prompt validation
	vr := h.promptVal.Validate(req.Prompt)
	if !vr.Valid {
		metadata["prompt_validation"] = "blocked: " + vr.Message
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("prompt validation failed: %s", vr.Message)
	}
	metadata["prompt_validation"] = "passed"

	// 3. Build tools
	bqTools := []tools.Tool{
		tools.BQListDatasetsTool(h.bq),
		tools.BQListTablesTool(h.bq),
		tools.BQGetSchemaTool(h.bq),
		tools.BQSampleDataTool(h.bq),
		tools.BQExecuteQueryTool(h.bq),
	}

	// 4. Build system prompt with pre-loaded schema (cached)
	datasetID := ""
	if req.DatasetID != nil {
		datasetID = *req.DatasetID
	}
	systemPrompt := h.buildSystemPrompt(ctx, datasetID)

	// 5. Run agent loop
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	output, toolsUsed, lastSQL, err := h.agent.Run(agentCtx, systemPrompt, req.Prompt, bqTools)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	metadata["tools_used"] = toolsUsed

	// 6. Extract SQL from output — fallback to last tool-executed SQL if not in code block
	generatedSQL := extractSQL(output)
	if generatedSQL == "" && lastSQL != "" {
		generatedSQL = lastSQL
		log.Debug().Str("sql", generatedSQL[:min(60, len(generatedSQL))]).Msg("using lastExecutedSQL as fallback")
	}
	metadata["sql_validation"] = "n/a"
	metadata["cost_tracking"] = "n/a"
	metadata["data_masking"] = "n/a"

	var execResult *models.QueryResponse

	if generatedSQL != "" && !req.DryRun {
		// 6. SQL validation
		if errMsg := h.sqlVal.Validate(generatedSQL); errMsg != "" {
			metadata["sql_validation"] = "blocked: " + errMsg
			return &models.AgentResponse{
				Status:        "error",
				Prompt:        req.Prompt,
				AgentMetadata: metadata,
			}, fmt.Errorf("SQL validation failed: %s", errMsg)
		}
		metadata["sql_validation"] = "passed"

		// 7. FIX #8: Execute SQL and populate ExecutionResult with masking + cost checks
		projectID := ""
		if req.ProjectID != nil {
			projectID = *req.ProjectID
		}
		queryStart := time.Now()
		result, qErr := h.bq.ExecuteQuery(agentCtx, generatedSQL, projectID, false, 60000, true, false)
		if qErr == nil {
			queryMs := time.Since(queryStart).Milliseconds()

			// Cost check
			if ok, costErr := h.costTracker.CheckLimits(result.TotalBytesProcessed, apiKey); !ok {
				metadata["cost_tracking"] = "blocked: " + costErr
			} else {
				h.costTracker.LogQueryCost(generatedSQL, result.TotalBytesProcessed, apiKey, queryMs)
				metadata["cost_tracking"] = "ok"

				// Data masking
				data := h.dataMasker.MaskRows(result.Data)
				metadata["data_masking"] = "applied"

				execResult = &models.QueryResponse{
					Status:   "success",
					Data:     data,
					Columns:  result.Columns,
					RowCount: len(data),
					Metadata: models.QueryMetadata{
						JobID:               result.JobID,
						TotalBytesProcessed: result.TotalBytesProcessed,
						BytesBilled:         result.BytesBilled,
						CacheHit:            result.CacheHit,
						ExecutionTimeMs:     queryMs,
					},
				}
			}
		}
	}

	execTimeMs := time.Since(start).Milliseconds()
	h.auditLogger.LogAIAgentRequest(req.Prompt, apiKey, generatedSQL, true, execTimeMs)

	reasoning := truncate(output, 500)
	sqlPtr := &generatedSQL

	return &models.AgentResponse{
		Status:          "success",
		Prompt:          req.Prompt,
		GeneratedSQL:    sqlPtr,
		ExecutionResult: execResult,
		AgentMetadata:   metadata,
		Reasoning:       &reasoning,
	}, nil
}

// extractSQL pulls SQL from model output using 4 strategies in order:
// 1. ```sql ... ``` code block (preferred)
// 2. ``` ... ``` generic code block containing SELECT/WITH
// 3. SELECT/WITH statement spanning multiple lines (greedy until LIMIT or end)
// 4. Single-line SELECT statement as last resort
var (
	// CTE: WITH name AS ( ... ) SELECT ...
	reMultilineSQL = regexp.MustCompile(`(?is)(WITH\s+\w+\s+AS\s*\(.+?(?:LIMIT\s+\d+|;\s*$|\z))`)
	// Plain SELECT spanning multiple lines ending with LIMIT or semicolon
	reSelectBlock = regexp.MustCompile(`(?is)(SELECT\s+.+?FROM\s+.+?(?:LIMIT\s+\d+|;\s*$|\z))`)
	reSingleSQL   = regexp.MustCompile(`(?i)(SELECT\s+\S.+?\bFROM\b\s+\S+)`)
)

func extractSQL(text string) string {
	// Strategy 1: ```sql / ```SQL block
	lower := strings.ToLower(text)
	for _, tag := range []string{"```sql", "```SQL"} {
		idx := strings.Index(lower, strings.ToLower(tag))
		if idx == -1 {
			continue
		}
		// skip past the tag and optional newline
		body := text[idx+len(tag):]
		if len(body) > 0 && body[0] == '\n' {
			body = body[1:]
		}
		end := strings.Index(body, "```")
		if end != -1 {
			if sql := strings.TrimSpace(body[:end]); sql != "" {
				return sql
			}
		}
	}

	// Strategy 2: any ``` block whose content starts with SELECT or WITH
	parts := strings.Split(text, "```")
	for i := 1; i < len(parts); i += 2 {
		candidate := strings.TrimSpace(parts[i])
		// strip language tag line if present (e.g. "python\nSELECT")
		if nl := strings.Index(candidate, "\n"); nl != -1 {
			firstLine := strings.TrimSpace(candidate[:nl])
			if !strings.Contains(strings.ToUpper(firstLine), "SELECT") &&
				!strings.Contains(strings.ToUpper(firstLine), "WITH") {
				candidate = strings.TrimSpace(candidate[nl:])
			}
		}
		up := strings.ToUpper(candidate)
		if strings.HasPrefix(up, "SELECT") || strings.HasPrefix(up, "WITH") {
			return strings.TrimSuffix(strings.TrimSpace(candidate), ";")
		}
	}

	// Strategy 3a: proper CTE (WITH name AS ...)
	if m := reMultilineSQL.FindString(text); m != "" {
		return strings.TrimSuffix(strings.TrimSpace(m), ";")
	}

	// Strategy 3b: multi-line SELECT ... FROM ... LIMIT
	if m := reSelectBlock.FindString(text); m != "" {
		candidate := strings.TrimSuffix(strings.TrimSpace(m), ";")
		// sanity check: must contain FROM keyword
		if strings.Contains(strings.ToUpper(candidate), " FROM ") {
			return candidate
		}
	}

	// Strategy 4: single-line SELECT as last resort
	if m := reSingleSQL.FindString(text); m != "" {
		return strings.TrimSuffix(strings.TrimSpace(m), ";")
	}

	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

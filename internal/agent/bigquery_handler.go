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

// schemaCache holds pre-built system prompts keyed by dataset ID.
type schemaCacheEntry struct {
	prompt    string
	expiresAt time.Time
}

type schemaCache struct {
	mu    sync.RWMutex
	store map[string]schemaCacheEntry
	ttl   time.Duration
	sf    singleflight.Group // deduplicate concurrent fetches for the same dataset
}

func newSchemaCache(ttl time.Duration) *schemaCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &schemaCache{store: make(map[string]schemaCacheEntry), ttl: ttl}
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
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *schemaCache) invalidate(datasetID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, datasetID)
}

// BaseSystemPrompt is the default BigQuery agent system prompt.
// It is exported so system_prompts.go can use it as the fallback for unknown styles.
const BaseSystemPrompt = `You are CortexAI, an expert data analyst with deep knowledge of BigQuery SQL.

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
7. For JOIN queries: use get_bigquery_sample_data to verify join key values match before executing
8. Always respond in the same language as the user's prompt. If the user writes in Indonesian, respond in Indonesian. If in English, respond in English.`

// getSchemaSection returns a cached schema-only section pre-loaded from BigQuery.
// It returns only the schema portion (no base prompt) so different personas can
// prepend their own base prompt via SystemPromptStyle(). Cache TTL is 5 minutes.
// Concurrent requests for the same dataset share a single fetch via singleflight.
func (h *BigQueryHandler) getSchemaSection(ctx context.Context, datasetID string) string {
	if datasetID == "" || h.bq == nil {
		return ""
	}

	// Cache hit — fast path, no lock contention beyond RLock
	if schema, ok := h.schemaCache.get(datasetID); ok {
		log.Debug().Str("dataset", datasetID).Msg("schema cache hit")
		return schema
	}

	// Cache miss — use singleflight so concurrent requests for the same dataset
	// share one BigQuery fetch instead of all hitting BQ at the same time.
	v, err, _ := h.schemaCache.sf.Do(datasetID, func() (interface{}, error) {
		// Double-check cache inside singleflight in case another goroutine
		// already populated it while we were waiting to enter.
		if schema, ok := h.schemaCache.get(datasetID); ok {
			return schema, nil
		}

		log.Debug().Str("dataset", datasetID).Msg("schema cache miss, fetching from BigQuery")
		fetchStart := time.Now()

		tables, err := h.bq.ListTables(ctx, datasetID)
		if err != nil {
			return "", nil // soft fail — return empty, don't cache
		}

		var sb strings.Builder
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

		schema := sb.String()
		h.schemaCache.set(datasetID, schema)

		log.Info().
			Str("dataset", datasetID).
			Int("tables", len(tables)).
			Dur("fetch_ms", time.Since(fetchStart)).
			Msg("schema cached")

		return schema, nil
	})

	if err != nil || v == nil {
		return ""
	}
	return v.(string)
}

// BigQueryHandler orchestrates the NL→SQL→execute pipeline
type BigQueryHandler struct {
	agent        LLMRunner
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
	agent LLMRunner,
	bq *service.BigQueryService,
	piiDetector *security.PIIDetector,
	promptVal *security.PromptValidator,
	sqlVal *security.SQLValidator,
	costTracker *security.CostTracker,
	dataMasker *security.DataMasker,
	auditLogger *security.AuditLogger,
	schemaCacheTTL time.Duration,
) *BigQueryHandler {
	return &BigQueryHandler{
		agent:       agent,
		bq:          bq,
		piiDetector: piiDetector,
		promptVal:   promptVal,
		sqlVal:      sqlVal,
		costTracker: costTracker,
		dataMasker:  dataMasker,
		schemaCache: newSchemaCache(schemaCacheTTL),
		auditLogger: auditLogger,
	}
}

// InvalidateSchemaCache removes the cached schema prompt for the given dataset,
// forcing the next request to re-fetch from BigQuery.
func (h *BigQueryHandler) InvalidateSchemaCache(datasetID string) {
	h.schemaCache.invalidate(datasetID)
}

// Handle processes an agent request for BigQuery.
// allowedDatasets restricts which datasets this user can query (squad isolation);
// nil means no restriction (admin or no squad configured).
// runner is the LLMRunner resolved for the current user's persona; promptStyle
// controls the system prompt tone ("executive", "technical", "support", or "").
// excludedTools lists tool names to hide from the LLM; nil means all tools are available.
func (h *BigQueryHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string, runner LLMRunner, promptStyle string, excludedTools []string) (*models.AgentResponse, error) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "bigquery",
		"model":       runner.Model(),
		"method":      "agent",
	}

	// 0. Squad dataset access check
	if len(allowedDatasets) > 0 && req.DatasetID != nil && *req.DatasetID != "" {
		if !isDatasetAllowed(*req.DatasetID, allowedDatasets) {
			return &models.AgentResponse{
				Status:        "error",
				Prompt:        req.Prompt,
				AgentMetadata: metadata,
			}, fmt.Errorf("dataset '%s' is not accessible for your squad", *req.DatasetID)
		}
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

	// 3. Build tools (BQListDatasetsTool is filtered to squad's datasets)
	if req.DryRun {
		excludedTools = append(excludedTools, "execute_bigquery_sql")
	}
	bqTools := filterTools([]tools.Tool{
		tools.BQListDatasetsTool(h.bq, allowedDatasets),
		tools.BQListTablesTool(h.bq),
		tools.BQGetSchemaTool(h.bq),
		tools.BQSampleDataTool(h.bq),
		tools.BQExecuteQueryTool(h.bq),
	}, excludedTools)

	// 4. Build system prompt: persona base + cached schema section
	datasetID := ""
	if req.DatasetID != nil {
		datasetID = *req.DatasetID
	}
	systemPrompt := SystemPromptStyle(promptStyle) + h.getSchemaSection(ctx, datasetID)

	// 5. Run agent loop
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	llmStart := time.Now()
	output, toolsUsed, lastSQL, err := runner.Run(agentCtx, systemPrompt, req.Prompt, bqTools)
	llmMs := time.Since(llmStart).Milliseconds()
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

	answerText := cleanAnswer(output)
	var answerPtr *string
	if answerText != "" {
		answerPtr = &answerText
	}
	var sqlPtr *string
	if generatedSQL != "" {
		sqlPtr = &generatedSQL
	}

	metadata["total_time_ms"] = execTimeMs
	metadata["llm_time_ms"] = llmMs

	return &models.AgentResponse{
		Status:          "success",
		Prompt:          req.Prompt,
		GeneratedSQL:    sqlPtr,
		ExecutionResult: execResult,
		AgentMetadata:   metadata,
		Answer:          answerPtr,
	}, nil
}

// HandleStream processes an agent request for BigQuery with SSE event emission.
// allowedDatasets restricts which datasets this user can query (squad isolation).
// runner and promptStyle are resolved from the current user's persona (same as Handle).
// excludedTools lists tool names to hide from the LLM; nil means all tools are available.
// The final "result" or "error" event is always the last call to emitFn.
func (h *BigQueryHandler) HandleStream(ctx context.Context, req *models.AgentRequest, apiKey string, allowedDatasets []string, runner LLMRunner, promptStyle string, emitFn func(event string, data interface{}), excludedTools []string) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "bigquery",
		"model":       runner.Model(),
		"method":      "agent_stream",
	}

	emitFn("start", map[string]interface{}{"prompt": req.Prompt})

	// 0. Squad dataset access check
	if len(allowedDatasets) > 0 && req.DatasetID != nil && *req.DatasetID != "" {
		if !isDatasetAllowed(*req.DatasetID, allowedDatasets) {
			emitFn("error", map[string]interface{}{
				"message": fmt.Sprintf("dataset '%s' is not accessible for your squad", *req.DatasetID),
				"step":    "dataset_access_check",
			})
			return
		}
	}

	// 1. PII detection
	emitFn("progress", map[string]interface{}{"step": "pii_check"})
	if found, kw := h.piiDetector.Detect(req.Prompt); found {
		metadata["pii_check"] = "blocked: " + kw
		emitFn("error", map[string]interface{}{
			"message": "PII detected in prompt: " + kw,
			"step":    "pii_check",
		})
		return
	}
	metadata["pii_check"] = "passed"

	// 2. Prompt validation
	emitFn("progress", map[string]interface{}{"step": "prompt_validation"})
	vr := h.promptVal.Validate(req.Prompt)
	if !vr.Valid {
		metadata["prompt_validation"] = "blocked: " + vr.Message
		emitFn("error", map[string]interface{}{
			"message": "prompt validation failed: " + vr.Message,
			"step":    "prompt_validation",
		})
		return
	}
	metadata["prompt_validation"] = "passed"

	// 3. Build tools (BQListDatasetsTool is filtered to squad's datasets)
	if req.DryRun {
		excludedTools = append(excludedTools, "execute_bigquery_sql")
	}
	bqTools := filterTools([]tools.Tool{
		tools.BQListDatasetsTool(h.bq, allowedDatasets),
		tools.BQListTablesTool(h.bq),
		tools.BQGetSchemaTool(h.bq),
		tools.BQSampleDataTool(h.bq),
		tools.BQExecuteQueryTool(h.bq),
	}, excludedTools)

	// 4. Schema pre-loading
	datasetID := ""
	if req.DatasetID != nil {
		datasetID = *req.DatasetID
	}
	emitFn("progress", map[string]interface{}{"step": "schema_loading", "dataset": datasetID})
	systemPrompt := SystemPromptStyle(promptStyle) + h.getSchemaSection(ctx, datasetID)
	emitFn("progress", map[string]interface{}{"step": "schema_ready", "dataset": datasetID})

	// 5. Run agent loop with event emission
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	agentEmit := func(event string, data map[string]interface{}) {
		emitFn(event, data)
	}

	llmStart := time.Now()
	output, toolsUsed, lastSQL, err := runner.RunWithEmit(agentCtx, systemPrompt, req.Prompt, bqTools, agentEmit)
	llmMs := time.Since(llmStart).Milliseconds()
	if err != nil {
		emitFn("error", map[string]interface{}{"message": "agent run: " + err.Error()})
		return
	}
	metadata["tools_used"] = toolsUsed

	// 6. Extract SQL
	generatedSQL := extractSQL(output)
	if generatedSQL == "" && lastSQL != "" {
		generatedSQL = lastSQL
		log.Debug().Str("sql", generatedSQL[:min(60, len(generatedSQL))]).Msg("stream: using lastExecutedSQL as fallback")
	}
	metadata["sql_validation"] = "n/a"
	metadata["cost_tracking"] = "n/a"
	metadata["data_masking"] = "n/a"

	var execResult *models.QueryResponse

	if generatedSQL != "" && !req.DryRun {
		if errMsg := h.sqlVal.Validate(generatedSQL); errMsg != "" {
			metadata["sql_validation"] = "blocked: " + errMsg
			emitFn("error", map[string]interface{}{
				"message": "SQL validation failed: " + errMsg,
				"step":    "sql_validation",
			})
			return
		}
		metadata["sql_validation"] = "passed"

		projectID := ""
		if req.ProjectID != nil {
			projectID = *req.ProjectID
		}
		emitFn("progress", map[string]interface{}{"step": "executing_sql"})
		queryStart := time.Now()
		result, qErr := h.bq.ExecuteQuery(agentCtx, generatedSQL, projectID, false, 60000, true, false)
		if qErr == nil {
			queryMs := time.Since(queryStart).Milliseconds()
			if ok, costErr := h.costTracker.CheckLimits(result.TotalBytesProcessed, apiKey); !ok {
				metadata["cost_tracking"] = "blocked: " + costErr
			} else {
				h.costTracker.LogQueryCost(generatedSQL, result.TotalBytesProcessed, apiKey, queryMs)
				metadata["cost_tracking"] = "ok"
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

	answerText := cleanAnswer(output)
	var answerPtr *string
	if answerText != "" {
		answerPtr = &answerText
	}
	var sqlPtr *string
	if generatedSQL != "" {
		sqlPtr = &generatedSQL
	}

	metadata["total_time_ms"] = execTimeMs
	metadata["llm_time_ms"] = llmMs

	emitFn("result", &models.AgentResponse{
		Status:          "success",
		Prompt:          req.Prompt,
		GeneratedSQL:    sqlPtr,
		ExecutionResult: execResult,
		AgentMetadata:   metadata,
		Answer:          answerPtr,
	})
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
			if sql := strings.TrimSuffix(strings.TrimSpace(body[:end]), ";"); sql != "" {
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
		// sanity check: must contain FROM keyword (use "FROM " without leading space
		// to correctly handle newline-before-FROM in multi-line queries)
		if strings.Contains(strings.ToUpper(candidate), "FROM ") {
			return candidate
		}
	}

	// Strategy 4: single-line SELECT as last resort
	if m := reSingleSQL.FindString(text); m != "" {
		return strings.TrimSuffix(strings.TrimSpace(m), ";")
	}

	return ""
}

// cleanAnswer strips SQL code blocks from LLM output to produce a human-readable answer.
// It removes ```sql...``` and ```...``` blocks, collapses excess whitespace.
func cleanAnswer(output string) string {
	result := output

	// Remove ```sql ... ``` blocks
	for {
		idx := strings.Index(strings.ToLower(result), "```sql")
		if idx == -1 {
			break
		}
		end := strings.Index(result[idx+6:], "```")
		if end == -1 {
			break
		}
		result = result[:idx] + result[idx+6+end+3:]
	}

	// Remove remaining ``` ... ``` blocks
	for {
		idx := strings.Index(result, "```")
		if idx == -1 {
			break
		}
		end := strings.Index(result[idx+3:], "```")
		if end == -1 {
			break
		}
		result = result[:idx] + result[idx+3+end+3:]
	}

	// Collapse multiple newlines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(result)
}

// isDatasetAllowed returns true if the datasetID is in the allowed list.
func isDatasetAllowed(datasetID string, allowedDatasets []string) bool {
	for _, d := range allowedDatasets {
		if d == datasetID {
			return true
		}
	}
	return false
}

// filterTools returns a new slice of tools with any tool whose Name appears in
// excluded removed. If excluded is nil or empty, ts is returned unchanged.
func filterTools(ts []tools.Tool, excluded []string) []tools.Tool {
	if len(excluded) == 0 {
		return ts
	}
	excSet := make(map[string]struct{}, len(excluded))
	for _, name := range excluded {
		excSet[name] = struct{}{}
	}
	result := make([]tools.Tool, 0, len(ts))
	for _, t := range ts {
		if _, skip := excSet[t.Name]; !skip {
			result = append(result, t)
		}
	}
	return result
}

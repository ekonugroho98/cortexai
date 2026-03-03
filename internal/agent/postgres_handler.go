package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/cortexai/cortexai/internal/tools"
	"github.com/rs/zerolog/log"
)

// PGBaseSystemPrompt is re-exported from system_prompts.go as PGBaseSystemPrompt.
// (defined in system_prompts.go)

// PostgresHandler orchestrates the NL→SQL→execute pipeline for PostgreSQL.
type PostgresHandler struct {
	agent       LLMRunner
	pgRegistry  *service.PGPoolRegistry
	piiDetector *security.PIIDetector
	promptVal   *security.PromptValidator
	sqlVal      *security.SQLValidator
	costTracker *security.PGCostTracker
	dataMasker  *security.DataMasker
	auditLogger *security.AuditLogger
	schemaCache *schemaCache // reuse existing type from bigquery_handler.go (same package)
}

// NewPostgresHandler creates a handler with all security components wired in.
func NewPostgresHandler(
	agent LLMRunner,
	pgRegistry *service.PGPoolRegistry,
	piiDetector *security.PIIDetector,
	promptVal *security.PromptValidator,
	sqlVal *security.SQLValidator,
	costTracker *security.PGCostTracker,
	dataMasker *security.DataMasker,
	auditLogger *security.AuditLogger,
	schemaCacheTTL time.Duration,
) *PostgresHandler {
	return &PostgresHandler{
		agent:       agent,
		pgRegistry:  pgRegistry,
		piiDetector: piiDetector,
		promptVal:   promptVal,
		sqlVal:      sqlVal,
		costTracker: costTracker,
		dataMasker:  dataMasker,
		auditLogger: auditLogger,
		schemaCache: newSchemaCache(schemaCacheTTL),
	}
}

// InvalidateSchemaCache removes the cached schema for the given cache key (squadID:dbName).
func (h *PostgresHandler) InvalidateSchemaCache(cacheKey string) {
	h.schemaCache.invalidate(cacheKey)
}

// PGSchemaClosingInstruction is the directive appended to the pre-injected schema
// block, instructing the LLM to skip redundant schema/table tool calls and to
// execute SQL at most once.
const PGSchemaClosingInstruction = "\nIMPORTANT: All table schemas are already provided above. DO NOT call list_postgres_tables or get_postgres_schema — go directly to writing and executing SQL. You should need at most 1 execute call."

// getPGSchemaSection returns a cached schema section for the given squad+database.
// Cache key is "squadID:dbName".
func (h *PostgresHandler) getPGSchemaSection(ctx context.Context, squadID, dbName string, pgSvc *service.PostgresService) string {
	if dbName == "" || pgSvc == nil {
		return ""
	}

	cacheKey := squadID + ":" + dbName

	if schema, ok := h.schemaCache.get(cacheKey); ok {
		log.Debug().Str("cache_key", cacheKey).Msg("pg schema cache hit")
		return schema
	}

	v, err, _ := h.schemaCache.sf.Do(cacheKey, func() (interface{}, error) {
		if schema, ok := h.schemaCache.get(cacheKey); ok {
			return schema, nil
		}

		log.Debug().Str("database", dbName).Str("squad", squadID).Msg("pg schema cache miss, fetching")
		fetchStart := time.Now()

		tables, err := pgSvc.ListTables(ctx, dbName)
		if err != nil {
			return "", nil
		}

		var sb strings.Builder
		sb.WriteString("\n\n## Available Database: " + dbName + "\n")
		sb.WriteString("The following tables and schemas are already available to you:\n\n")

		for _, tbl := range tables {
			cols, err := pgSvc.GetTableSchema(ctx, dbName, tbl.Schema, tbl.Name)
			if err != nil {
				log.Warn().Err(err).Str("table", tbl.Schema+"."+tbl.Name).Msg("pg pre-load schema: get schema failed")
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s.%s (%s)\n", tbl.Schema, tbl.Name, tbl.Type))
			sb.WriteString(service.PGSchemaToString(cols))
			sb.WriteString("\n")
		}

		sb.WriteString(PGSchemaClosingInstruction)

		schema := sb.String()
		h.schemaCache.set(cacheKey, schema)

		log.Info().
			Str("database", dbName).
			Str("squad", squadID).
			Int("tables", len(tables)).
			Dur("fetch_ms", time.Since(fetchStart)).
			Msg("pg schema cached")

		return schema, nil
	})

	if err != nil || v == nil {
		return ""
	}
	return v.(string)
}

// Handle processes an agent request for PostgreSQL.
func (h *PostgresHandler) Handle(ctx context.Context, req *models.AgentRequest, apiKey string, squadID string, allowedDatabases []string, runner LLMRunner, promptStyle string, excludedTools []string) (*models.AgentResponse, error) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "postgres",
		"model":       runner.Model(),
		"method":      "agent",
	}

	// 0. Resolve pgSvc from registry
	pgSvc := h.pgRegistry.Get(squadID)
	if pgSvc == nil {
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("PostgreSQL is not configured for squad '%s'", squadID)
	}

	// 0b. Database access check
	dbName := ""
	if req.DatasetID != nil {
		dbName = *req.DatasetID
	}
	if dbName == "" {
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("database name is required (use dataset_id field)")
	}
	if len(allowedDatabases) > 0 && !isDatabaseAllowed(dbName, allowedDatabases) {
		return &models.AgentResponse{
			Status:        "error",
			Prompt:        req.Prompt,
			AgentMetadata: metadata,
		}, fmt.Errorf("database '%s' is not accessible for your squad", dbName)
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

	// 3. Build PG tools
	if req.DryRun {
		excludedTools = append(excludedTools, "execute_postgres_sql")
		if dbName != "" {
			// Schema already injected into system prompt — no need for inspection tools.
			excludedTools = append(excludedTools, "list_postgres_tables", "get_postgres_schema", "get_postgres_sample_data")
		}
	}
	pgTools := filterTools([]tools.Tool{
		tools.PGListDatabasesTool(allowedDatabases),
		tools.PGListTablesTool(pgSvc, dbName),
		tools.PGGetSchemaTool(pgSvc, dbName),
		tools.PGSampleDataTool(pgSvc, dbName),
		tools.PGExecuteQueryTool(pgSvc, dbName),
	}, excludedTools)

	// 4. Build system prompt: persona base + cached schema section
	systemPrompt := PGSystemPromptStyle(promptStyle) + h.getPGSchemaSection(ctx, squadID, dbName, pgSvc)

	// 5. Run agent loop
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	llmStart := time.Now()
	output, toolsUsed, lastSQL, err := runner.Run(agentCtx, systemPrompt, req.Prompt, pgTools)
	llmMs := time.Since(llmStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	metadata["tools_used"] = toolsUsed

	// 6. Extract SQL (dialect-agnostic)
	generatedSQL := extractSQL(output)
	if generatedSQL == "" && lastSQL != "" {
		generatedSQL = lastSQL
		log.Debug().Str("sql", generatedSQL[:min(60, len(generatedSQL))]).Msg("pg: using lastExecutedSQL as fallback")
	}
	metadata["sql_validation"] = "n/a"
	metadata["cost_tracking"] = "n/a"
	metadata["data_masking"] = "n/a"

	var execResult *models.QueryResponse

	if generatedSQL != "" && !req.DryRun {
		// 7. SQL validation (PG-specific)
		if errMsg := h.sqlVal.ValidatePG(generatedSQL); errMsg != "" {
			metadata["sql_validation"] = "blocked: " + errMsg
			return &models.AgentResponse{
				Status:        "error",
				Prompt:        req.Prompt,
				AgentMetadata: metadata,
			}, fmt.Errorf("SQL validation failed: %s", errMsg)
		}
		metadata["sql_validation"] = "passed"

		// 8. EXPLAIN cost check
		explainCost, explainErr := pgSvc.ExplainCost(agentCtx, dbName, generatedSQL)
		if explainErr == nil && explainCost != nil {
			if ok, costErr := h.costTracker.CheckCost(explainCost.TotalCost); !ok {
				metadata["cost_tracking"] = "blocked: " + costErr
				return &models.AgentResponse{
					Status:        "error",
					Prompt:        req.Prompt,
					AgentMetadata: metadata,
				}, fmt.Errorf("query cost check failed: %s", costErr)
			}
		}

		// 9. Execute query (read-only tx)
		queryStart := time.Now()
		result, qErr := pgSvc.ExecuteQuery(agentCtx, dbName, generatedSQL, 60000)
		if qErr == nil {
			queryMs := time.Since(queryStart).Milliseconds()

			if explainCost != nil {
				h.costTracker.LogQueryCost(generatedSQL, explainCost.TotalCost, apiKey, queryMs)
			}
			metadata["cost_tracking"] = "ok"

			// 10. Data masking
			data := h.dataMasker.MaskRows(result.Data)
			metadata["data_masking"] = "applied"

			execResult = &models.QueryResponse{
				Status:   "success",
				Data:     data,
				Columns:  result.Columns,
				RowCount: len(data),
				Metadata: models.QueryMetadata{
					ExecutionTimeMs: queryMs,
				},
			}
		}
	}

	// 11. Audit logging
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

// HandleStream processes an agent request for PostgreSQL with SSE event emission.
func (h *PostgresHandler) HandleStream(ctx context.Context, req *models.AgentRequest, apiKey string, squadID string, allowedDatabases []string, runner LLMRunner, promptStyle string, emitFn func(event string, data interface{}), excludedTools []string) {
	start := time.Now()
	metadata := map[string]interface{}{
		"data_source": "postgres",
		"model":       runner.Model(),
		"method":      "agent_stream",
	}

	emitFn("start", map[string]interface{}{"prompt": req.Prompt})

	// 0. Resolve pgSvc
	pgSvc := h.pgRegistry.Get(squadID)
	if pgSvc == nil {
		emitFn("error", map[string]interface{}{
			"message": fmt.Sprintf("PostgreSQL is not configured for squad '%s'", squadID),
			"step":    "pg_service_check",
		})
		return
	}

	// 0b. Database access check
	dbName := ""
	if req.DatasetID != nil {
		dbName = *req.DatasetID
	}
	if dbName == "" {
		emitFn("error", map[string]interface{}{
			"message": "database name is required (use dataset_id field)",
			"step":    "database_check",
		})
		return
	}
	if len(allowedDatabases) > 0 && !isDatabaseAllowed(dbName, allowedDatabases) {
		emitFn("error", map[string]interface{}{
			"message": fmt.Sprintf("database '%s' is not accessible for your squad", dbName),
			"step":    "database_access_check",
		})
		return
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

	// 3. Build PG tools
	if req.DryRun {
		excludedTools = append(excludedTools, "execute_postgres_sql")
		if dbName != "" {
			// Schema already injected into system prompt — no need for inspection tools.
			excludedTools = append(excludedTools, "list_postgres_tables", "get_postgres_schema", "get_postgres_sample_data")
		}
	}
	pgTools := filterTools([]tools.Tool{
		tools.PGListDatabasesTool(allowedDatabases),
		tools.PGListTablesTool(pgSvc, dbName),
		tools.PGGetSchemaTool(pgSvc, dbName),
		tools.PGSampleDataTool(pgSvc, dbName),
		tools.PGExecuteQueryTool(pgSvc, dbName),
	}, excludedTools)

	// 4. Schema pre-loading
	emitFn("progress", map[string]interface{}{"step": "schema_loading", "database": dbName})
	systemPrompt := PGSystemPromptStyle(promptStyle) + h.getPGSchemaSection(ctx, squadID, dbName, pgSvc)
	emitFn("progress", map[string]interface{}{"step": "schema_ready", "database": dbName})

	// 5. Run agent loop with event emission
	agentCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
	defer cancel()

	agentEmit := func(event string, data map[string]interface{}) {
		emitFn(event, data)
	}

	llmStart := time.Now()
	output, toolsUsed, lastSQL, err := runner.RunWithEmit(agentCtx, systemPrompt, req.Prompt, pgTools, agentEmit)
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
		log.Debug().Str("sql", generatedSQL[:min(60, len(generatedSQL))]).Msg("pg stream: using lastExecutedSQL as fallback")
	}
	metadata["sql_validation"] = "n/a"
	metadata["cost_tracking"] = "n/a"
	metadata["data_masking"] = "n/a"

	var execResult *models.QueryResponse

	if generatedSQL != "" && !req.DryRun {
		if errMsg := h.sqlVal.ValidatePG(generatedSQL); errMsg != "" {
			metadata["sql_validation"] = "blocked: " + errMsg
			emitFn("error", map[string]interface{}{
				"message": "SQL validation failed: " + errMsg,
				"step":    "sql_validation",
			})
			return
		}
		metadata["sql_validation"] = "passed"

		// EXPLAIN cost check
		explainCost, explainErr := pgSvc.ExplainCost(agentCtx, dbName, generatedSQL)
		if explainErr == nil && explainCost != nil {
			if ok, costErr := h.costTracker.CheckCost(explainCost.TotalCost); !ok {
				metadata["cost_tracking"] = "blocked: " + costErr
				emitFn("error", map[string]interface{}{
					"message": "Query cost check failed: " + costErr,
					"step":    "cost_check",
				})
				return
			}
		}

		emitFn("progress", map[string]interface{}{"step": "executing_sql"})
		queryStart := time.Now()
		result, qErr := pgSvc.ExecuteQuery(agentCtx, dbName, generatedSQL, 60000)
		if qErr == nil {
			queryMs := time.Since(queryStart).Milliseconds()

			if explainCost != nil {
				h.costTracker.LogQueryCost(generatedSQL, explainCost.TotalCost, apiKey, queryMs)
			}
			metadata["cost_tracking"] = "ok"

			data := h.dataMasker.MaskRows(result.Data)
			metadata["data_masking"] = "applied"
			execResult = &models.QueryResponse{
				Status:   "success",
				Data:     data,
				Columns:  result.Columns,
				RowCount: len(data),
				Metadata: models.QueryMetadata{
					ExecutionTimeMs: queryMs,
				},
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

// isDatabaseAllowed returns true if the dbName is in the allowed list.
func isDatabaseAllowed(dbName string, allowedDatabases []string) bool {
	for _, d := range allowedDatabases {
		if d == dbName {
			return true
		}
	}
	return false
}

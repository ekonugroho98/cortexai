package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cortexai/cortexai/internal/agent"
	"github.com/cortexai/cortexai/internal/handler"
	"github.com/cortexai/cortexai/internal/middleware"
	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// setupRoutes returns (router, bqSvc, pgRegistry, error) so services can be closed on shutdown
func (s *Server) setupRoutes() (http.Handler, *service.BigQueryService, *service.PGPoolRegistry, error) {
	cfg := s.cfg
	ctx := context.Background()

	// ─── Services ───────────────────────────────────────────────────────────────
	var bqSvc *service.BigQueryService
	if cfg.GCPProjectID != "" {
		var bqErr error
		bqSvc, bqErr = service.NewBigQueryService(ctx, cfg.GCPProjectID, cfg.GoogleApplicationCredentials, cfg.BigQueryLocation)
		if bqErr != nil {
			log.Warn().Err(bqErr).Msg("BigQuery service unavailable")
		}
	} else {
		log.Warn().Msg("GCP_PROJECT_ID not set - BigQuery disabled")
	}

	var esSvc *service.ElasticsearchService
	if cfg.ElasticsearchEnabled {
		var esErr error
		// FIX #4: pass allowedPatterns to ES service
		esSvc, esErr = service.NewElasticsearchService(
			cfg.ElasticsearchScheme,
			cfg.ElasticsearchHost,
			cfg.ElasticsearchPort,
			cfg.ElasticsearchUser,
			cfg.ElasticsearchPassword,
			cfg.ElasticsearchVerifyCerts,
			cfg.ElasticsearchMaxRetries,
			cfg.ElasticsearchTimeout,
			cfg.ESAllowedPatterns,
		)
		if esErr != nil {
			log.Warn().Err(esErr).Msg("Elasticsearch service unavailable")
		}
	}

	// ─── PostgreSQL Pool Registry ────────────────────────────────────────────────
	var pgRegistry *service.PGPoolRegistry
	if cfg.PostgresEnabled {
		pgRegistry = service.NewPGPoolRegistry()
		for _, sq := range cfg.Squads {
			if sq.Postgres != nil {
				pgSvc := service.NewPostgresService(
					sq.Postgres.Host,
					sq.Postgres.Port,
					sq.Postgres.User,
					sq.Postgres.Password,
					sq.Postgres.SSLMode,
					sq.Postgres.MaxConns,
				)
				pgRegistry.Register(sq.ID, pgSvc)
				log.Info().Str("squad", sq.ID).Str("host", sq.Postgres.Host).Int("databases", len(sq.Postgres.Databases)).Msg("PG pool registered")
			}
		}
	}

	// ─── User Store ───────────────────────────────────────────────────────────────
	// Convert config types → service entry types
	squadEntries := make([]service.SquadEntry, len(cfg.Squads))
	for i, s := range cfg.Squads {
		var pgDatabases []string
		if s.Postgres != nil {
			pgDatabases = s.Postgres.Databases
		}
		squadEntries[i] = service.SquadEntry{
			ID:              s.ID,
			Name:            s.Name,
			Datasets:        s.Datasets,
			ESIndexPatterns: s.ESIndexPatterns,
			PGDatabases:     pgDatabases,
		}
	}
	userEntries := make([]service.UserEntry, len(cfg.Users))
	for i, u := range cfg.Users {
		userEntries[i] = service.UserEntry{
			ID:      u.ID,
			Name:    u.Name,
			Role:    u.Role,
			APIKey:  u.APIKey,
			SquadID: u.SquadID,
			Persona: u.Persona,
		}
	}
	userStore := service.NewUserStore(userEntries, squadEntries, cfg.APIKeys)
	totalKeys := len(userStore.AllKeys())

	// FIX #13: startup summary — warn clearly about disabled features
	log.Info().
		Bool("bigquery_enabled", bqSvc != nil).
		Bool("elasticsearch_enabled", esSvc != nil).
		Bool("postgres_enabled", pgRegistry != nil).
		Bool("auth_enabled", cfg.EnableAuth && totalKeys > 0).
		Int("registered_users", len(cfg.Users)).
		Int("legacy_keys", len(cfg.APIKeys)).
		Bool("data_masking", cfg.EnableDataMasking).
		Bool("audit_logging", cfg.EnableAuditLogging).
		Bool("pii_detection", cfg.EnablePIIDetection).
		Msg("service configuration")

	if bqSvc == nil && !cfg.ElasticsearchEnabled && pgRegistry == nil {
		log.Warn().Msg("WARNING: no data sources configured - /api/v1/query and /api/v1/query-agent will return 503")
	}
	if cfg.EnableAuth && totalKeys == 0 {
		log.Warn().Msg("WARNING: auth enabled but no API keys configured - all API requests will be rejected")
	}

	// ─── Security ───────────────────────────────────────────────────────────────
	piiDetector := security.NewPIIDetector(cfg.PIIKeywords)
	promptVal := security.NewPromptValidator()
	sqlVal := security.NewSQLValidator()
	esPromptVal := security.NewESPromptValidator()
	costTracker := security.NewCostTracker(cfg.MaxQueryBytesProcessed)
	dataMasker := security.NewDataMasker(cfg.SensitiveColumns)
	auditLogger := security.NewAuditLogger(cfg.EnableAuditLogging)

	// ─── AI Agent / LLM Pool ─────────────────────────────────────────────────────
	// Build a fallback runner from the legacy LLMProvider config (backward compat).
	// This runner is used for users with no persona or an unknown persona.
	llmPool := agent.NewLLMPool()
	switch cfg.LLMProvider {
	case "deepseek":
		if cfg.DeepSeekAPIKey != "" {
			model := cfg.ModelList["deepseek"]
			llmPool.SetFallback(agent.NewDeepSeekAgent(cfg.DeepSeekAPIKey, model, cfg.DeepSeekBaseURL))
			log.Info().Str("provider", "deepseek").Str("model", model).Msg("AI fallback runner initialized")
		} else {
			log.Warn().Msg("LLM_PROVIDER=deepseek but DEEPSEEK_API_KEY not set - AI agent disabled")
		}
	default: // "anthropic" + GLM via Z.ai
		if cfg.AnthropicAPIKey != "" {
			model := cfg.ModelList["anthropic"]
			llmPool.SetFallback(agent.NewCortexAgent(cfg.AnthropicAPIKey, model, cfg.AnthropicBaseURL))
			log.Info().Str("provider", "anthropic").Str("model", model).Msg("AI fallback runner initialized")
		} else {
			log.Warn().Msg("ANTHROPIC_API_KEY not set - AI agent disabled")
		}
	}

	// Register per-persona runners. Personas sharing the same provider+model reuse
	// the same LLMRunner instance (LLMPool deduplicates by PoolKey).
	for name, pc := range cfg.Personas {
		var runner agent.LLMRunner
		apiKey := cfg.AnthropicAPIKey
		baseURL := cfg.AnthropicBaseURL
		switch pc.Provider {
		case "deepseek":
			apiKey = cfg.DeepSeekAPIKey
			baseURL = cfg.DeepSeekBaseURL
			if pc.BaseURL != "" {
				baseURL = pc.BaseURL
			}
			if apiKey != "" {
				runner = agent.NewDeepSeekAgent(apiKey, pc.Model, baseURL)
			}
		default: // "anthropic"
			if pc.BaseURL != "" {
				baseURL = pc.BaseURL
			}
			if apiKey != "" {
				runner = agent.NewCortexAgent(apiKey, pc.Model, baseURL)
			}
		}
		if runner != nil {
			key := agent.PoolKey(pc.Provider, pc.Model)
			llmPool.Register(key, runner)
			log.Info().Str("persona", name).Str("provider", pc.Provider).Str("model", pc.Model).Msg("persona LLM registered")
		} else {
			log.Warn().Str("persona", name).Str("provider", pc.Provider).Msg("persona skipped: missing API key")
		}
	}

	router := service.NewIntentRouter()

	// ─── Handlers ────────────────────────────────────────────────────────────────
	// FIX #6: pass bqSvc and esSvc to health handler for dependency checks
	healthH := handler.NewHealthHandler(bqSvc, esSvc)
	userH := handler.NewUserHandler()

	var datasetsH *handler.DatasetsHandler
	var tablesH *handler.TablesHandler
	var queryH *handler.QueryHandler
	var agentH *handler.AgentHandler
	var cacheH *handler.CacheHandler

	if bqSvc != nil {
		datasetsH = handler.NewDatasetsHandler(bqSvc)
		tablesH = handler.NewTablesHandler(bqSvc)
		queryH = handler.NewQueryHandler(bqSvc, sqlVal, costTracker, dataMasker, auditLogger, cfg.EnableDataMasking)
	}

	// PG cost tracker (created even if postgres is disabled — zero maxCost means no limit)
	pgCostTracker := security.NewPGCostTracker(cfg.MaxPGQueryCost)

	if llmPool.HasRunners() {
		var bqAgentH *agent.BigQueryHandler
		var esAgentH *agent.ElasticsearchHandler
		var pgAgentH *agent.PostgresHandler

		// Pass the pool fallback to handler constructors for the stored h.agent field.
		// Handle() and HandleStream() use the runner parameter passed per-request instead.
		fallbackRunner := llmPool.Get("")
		schemaTTL := time.Duration(cfg.SchemaCacheTTL) * time.Minute
		if bqSvc != nil {
			bqAgentH = agent.NewBigQueryHandler(fallbackRunner, bqSvc, piiDetector, promptVal, sqlVal, costTracker, dataMasker, auditLogger, schemaTTL)
		}
		if esSvc != nil {
			esAgentH = agent.NewElasticsearchHandler(fallbackRunner, esSvc, piiDetector, promptVal, esPromptVal, auditLogger)
		}
		if pgRegistry != nil {
			pgAgentH = agent.NewPostgresHandler(fallbackRunner, pgRegistry, piiDetector, promptVal, sqlVal, pgCostTracker, dataMasker, auditLogger, schemaTTL)
		}
		cacheH = handler.NewCacheHandler(bqAgentH, pgAgentH)
		// FIX #1: agentH is created even if bqAgentH is nil; nil check is inside QueryAgent
		agentH = handler.NewAgentHandler(bqAgentH, esAgentH, pgAgentH, router, llmPool, cfg.Personas)
	}

	// ─── Router ──────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Core middleware
	r.Use(middleware.Recovery)
	r.Use(middleware.RequestID) // FIX #11: request ID for tracing
	r.Use(middleware.Logging)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(middleware.DefaultCORSConfig(cfg.CORSOrigins)))
	r.Use(chiMiddleware.RealIP)

	// Public routes
	r.Get("/health", healthH.Health)
	r.Get("/", healthH.Health)

	// Auth + rate limiting for API routes
	apiMiddleware := []func(http.Handler) http.Handler{
		middleware.RateLimit(cfg.RateLimitPerMinute),
	}
	if cfg.EnableAuth && totalKeys > 0 {
		apiMiddleware = append(apiMiddleware, middleware.Auth(userStore, cfg.APIKeyHeader))
	}

	r.Group(func(r chi.Router) {
		for _, m := range apiMiddleware {
			r.Use(m)
		}

		r.Route(fmt.Sprintf("%s", cfg.APIPrefix), func(r chi.Router) {
			// User profile — available to all authenticated users
			r.Get("/me", userH.Me)

			// BigQuery — datasets/tables: viewer+; query/agent: analyst+
			if datasetsH != nil {
				r.Get("/datasets", datasetsH.ListDatasets)
				r.Get("/datasets/{dataset_id}", datasetsH.GetDataset)
			}
			if tablesH != nil {
				r.Get("/datasets/{dataset_id}/tables", tablesH.ListTables)
				r.Get("/datasets/{dataset_id}/tables/{table_id}", tablesH.GetTable)
			}
			if queryH != nil {
				r.With(middleware.RequireRole(models.RoleAnalyst, models.RoleAdmin)).
					Post("/query", queryH.Execute)
			}

			// AI Agent — analyst+
			if agentH != nil {
				r.With(middleware.RequireRole(models.RoleAnalyst, models.RoleAdmin)).
					Post("/query-agent", agentH.QueryAgent)
				r.With(middleware.RequireRole(models.RoleAnalyst, models.RoleAdmin)).
					Post("/query-agent/stream", agentH.QueryAgentStream)
			}

			// Cache management — admin only
			if cacheH != nil {
				r.With(middleware.RequireRole(models.RoleAdmin)).
					Delete("/cache/schema/{dataset}", cacheH.InvalidateSchemaCache)
				r.With(middleware.RequireRole(models.RoleAdmin)).
					Delete("/cache/pg-schema/{squad}/{database}", cacheH.InvalidatePGSchemaCache)
				r.With(middleware.RequireRole(models.RoleAdmin)).
					Delete("/cache/responses", cacheH.FlushResponseCache)
			}
		})
	})

	return r, bqSvc, pgRegistry, nil
}

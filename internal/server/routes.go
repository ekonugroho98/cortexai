package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cortexai/cortexai/internal/agent"
	"github.com/cortexai/cortexai/internal/handler"
	"github.com/cortexai/cortexai/internal/middleware"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// setupRoutes returns (router, bqSvc, error) so bqSvc can be closed on shutdown
func (s *Server) setupRoutes() (http.Handler, *service.BigQueryService, error) {
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

	// FIX #13: startup summary — warn clearly about disabled features
	log.Info().
		Bool("bigquery_enabled", bqSvc != nil).
		Bool("elasticsearch_enabled", esSvc != nil).
		Bool("auth_enabled", cfg.EnableAuth && len(cfg.APIKeys) > 0).
		Bool("data_masking", cfg.EnableDataMasking).
		Bool("audit_logging", cfg.EnableAuditLogging).
		Bool("pii_detection", cfg.EnablePIIDetection).
		Msg("service configuration")

	if bqSvc == nil && !cfg.ElasticsearchEnabled {
		log.Warn().Msg("WARNING: no data sources configured - /api/v1/query and /api/v1/query-agent will return 503")
	}
	if cfg.EnableAuth && len(cfg.APIKeys) == 0 {
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

	// ─── AI Agent ────────────────────────────────────────────────────────────────
	var cortexAgent *agent.CortexAgent
	if cfg.AnthropicAPIKey != "" {
		model := cfg.ModelList["anthropic"]
		cortexAgent = agent.NewCortexAgent(cfg.AnthropicAPIKey, model, cfg.AnthropicBaseURL)
	} else {
		log.Warn().Msg("ANTHROPIC_API_KEY not set - AI agent disabled")
	}

	router := service.NewIntentRouter()

	// ─── Handlers ────────────────────────────────────────────────────────────────
	// FIX #6: pass bqSvc and esSvc to health handler for dependency checks
	healthH := handler.NewHealthHandler(bqSvc, esSvc)

	var datasetsH *handler.DatasetsHandler
	var tablesH *handler.TablesHandler
	var queryH *handler.QueryHandler
	var agentH *handler.AgentHandler
	var esHandler *handler.ElasticsearchHandler

	if bqSvc != nil {
		datasetsH = handler.NewDatasetsHandler(bqSvc)
		tablesH = handler.NewTablesHandler(bqSvc)
		queryH = handler.NewQueryHandler(bqSvc, sqlVal, costTracker, dataMasker, auditLogger, cfg.EnableDataMasking)
	}

	if cortexAgent != nil {
		var bqAgentH *agent.BigQueryHandler
		var esAgentH *agent.ElasticsearchHandler

		if bqSvc != nil {
			bqAgentH = agent.NewBigQueryHandler(cortexAgent, bqSvc, piiDetector, promptVal, sqlVal, costTracker, dataMasker, auditLogger)
		}
		if esSvc != nil {
			esAgentH = agent.NewElasticsearchHandler(cortexAgent, esSvc, piiDetector, promptVal, esPromptVal, auditLogger)
		}
		// FIX #1: agentH is created even if bqAgentH is nil; nil check is inside QueryAgent
		agentH = handler.NewAgentHandler(bqAgentH, esAgentH, router)
	}

	if esSvc != nil {
		esHandler = handler.NewElasticsearchHandler(esSvc)
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
	if cfg.EnableAuth && len(cfg.APIKeys) > 0 {
		apiMiddleware = append(apiMiddleware, middleware.Auth(cfg.APIKeys, cfg.APIKeyHeader))
	}

	r.Group(func(r chi.Router) {
		for _, m := range apiMiddleware {
			r.Use(m)
		}

		r.Route(fmt.Sprintf("%s", cfg.APIPrefix), func(r chi.Router) {
			// BigQuery
			if datasetsH != nil {
				r.Get("/datasets", datasetsH.ListDatasets)
				r.Get("/datasets/{dataset_id}", datasetsH.GetDataset)
			}
			if tablesH != nil {
				r.Get("/datasets/{dataset_id}/tables", tablesH.ListTables)
				r.Get("/datasets/{dataset_id}/tables/{table_id}", tablesH.GetTable)
			}
			if queryH != nil {
				r.Post("/query", queryH.Execute)
			}

			// AI Agent
			if agentH != nil {
				r.Post("/query-agent", agentH.QueryAgent)
			}

			// Elasticsearch
			if esHandler != nil {
				r.Route("/elasticsearch", func(r chi.Router) {
					r.Get("/", esHandler.Info)
					r.Get("/health", esHandler.Health)
					r.Get("/cluster/info", esHandler.ClusterInfo)
					r.Get("/cluster/health", esHandler.ClusterHealth)
					r.Get("/indices", esHandler.ListIndices)
					r.Get("/indices/{index_name}", esHandler.GetIndex)
					r.Post("/search", esHandler.Search)
					r.Post("/count", esHandler.Count)
					r.Post("/aggregate", esHandler.Aggregate)
				})
			}
		})
	})

	return r, bqSvc, nil
}

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cortexai/cortexai/internal/agent"
	"github.com/cortexai/cortexai/internal/config"
	"github.com/cortexai/cortexai/internal/middleware"
	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
)

// AgentHandler handles POST /api/v1/query-agent
type AgentHandler struct {
	bqHandler *agent.BigQueryHandler
	esHandler *agent.ElasticsearchHandler
	pgHandler *agent.PostgresHandler
	router    *service.IntentRouter
	llmPool   *agent.LLMPool
	personas  map[string]config.PersonaConfig
}

func NewAgentHandler(
	bqHandler *agent.BigQueryHandler,
	esHandler *agent.ElasticsearchHandler,
	pgHandler *agent.PostgresHandler,
	router *service.IntentRouter,
	llmPool *agent.LLMPool,
	personas map[string]config.PersonaConfig,
) *AgentHandler {
	return &AgentHandler{
		bqHandler: bqHandler,
		esHandler: esHandler,
		pgHandler: pgHandler,
		router:    router,
		llmPool:   llmPool,
		personas:  personas,
	}
}

// resolvePersona maps a user's persona name to the correct LLMRunner, prompt style,
// and full PersonaConfig. Returns the pool fallback runner, empty style, and a
// zero-value PersonaConfig for unknown or empty personas (zero-value = no restrictions).
func (h *AgentHandler) resolvePersona(user *models.User) (agent.LLMRunner, string, config.PersonaConfig) {
	if user == nil || user.Persona == "" {
		return h.llmPool.Get(""), "", config.PersonaConfig{}
	}
	pc, ok := h.personas[user.Persona]
	if !ok {
		return h.llmPool.Get(""), "", config.PersonaConfig{}
	}
	return h.llmPool.Get(agent.PoolKey(pc.Provider, pc.Model)), pc.SystemPromptStyle, pc
}

// checkDataSourceAllowed returns nil if the given dataSource is permitted for the
// persona, or a descriptive error if it is not. An empty AllowedDataSources list
// means all data sources are allowed (backward compatible default).
func (h *AgentHandler) checkDataSourceAllowed(pc config.PersonaConfig, dataSource string) error {
	if len(pc.AllowedDataSources) == 0 {
		return nil
	}
	for _, allowed := range pc.AllowedDataSources {
		if allowed == dataSource {
			return nil
		}
	}
	return fmt.Errorf("data source '%s' is not available for your persona. Available: %v",
		dataSource, pc.AllowedDataSources)
}

// QueryAgent handles POST /api/v1/query-agent
func (h *AgentHandler) QueryAgent(w http.ResponseWriter, r *http.Request) {
	var req models.AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.SetDefaults()

	if req.Prompt == "" {
		models.WriteError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	apiKey := r.Header.Get("X-API-Key")

	// Extract squad restrictions and persona from authenticated user
	var allowedDatasets []string
	var allowedESPatterns []string
	var allowedPGDatabases []string
	var currentUser *models.User
	if user, ok := middleware.GetCurrentUser(r.Context()); ok {
		currentUser = user
		if user.Squad != nil {
			allowedDatasets = user.Squad.Datasets
			allowedESPatterns = user.Squad.ESIndexPatterns
			allowedPGDatabases = user.Squad.PGDatabases
		}
	}

	// Resolve persona → LLM runner + prompt style + persona config
	runner, promptStyle, pc := h.resolvePersona(currentUser)

	// Determine data source
	var source service.DataSource
	var routingConf float64
	var routingReason string

	if req.DataSource != nil && *req.DataSource != "" {
		source = service.DataSource(*req.DataSource)
		routingConf = 1.0
		routingReason = "explicitly specified by user"
	} else {
		routing := h.router.Route(req.Prompt)
		source = routing.Source
		routingConf = routing.Confidence
		routingReason = routing.Reasoning
	}

	// Persona-based data source restriction
	if err := h.checkDataSourceAllowed(pc, string(source)); err != nil {
		models.WriteError(w, http.StatusForbidden, err.Error())
		return
	}

	var resp *models.AgentResponse
	var err error

	switch source {
	case service.DataSourceElasticsearch:
		if h.esHandler == nil {
			models.WriteError(w, http.StatusServiceUnavailable, "Elasticsearch is not configured")
			return
		}
		resp, err = h.esHandler.Handle(r.Context(), &req, apiKey, allowedESPatterns, runner, promptStyle)
	case service.DataSourcePostgres:
		if h.pgHandler == nil {
			models.WriteError(w, http.StatusServiceUnavailable, "PostgreSQL is not configured")
			return
		}
		squadID := ""
		if currentUser != nil {
			squadID = currentUser.SquadID
		}
		resp, err = h.pgHandler.Handle(r.Context(), &req, apiKey, squadID, allowedPGDatabases, runner, promptStyle, pc.ExcludedTools)
	default:
		// FIX #1: nil check for bqHandler to prevent panic
		if h.bqHandler == nil {
			models.WriteError(w, http.StatusServiceUnavailable, "BigQuery is not configured")
			return
		}
		resp, err = h.bqHandler.Handle(r.Context(), &req, apiKey, allowedDatasets, runner, promptStyle, pc.ExcludedTools)
	}

	if err != nil {
		if resp != nil {
			resp.AgentMetadata["routing_confidence"] = routingConf
			resp.AgentMetadata["routing_reasoning"] = routingReason
			models.WriteJSON(w, http.StatusBadRequest, resp)
			return
		}
		models.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp.AgentMetadata["routing_confidence"] = routingConf
	resp.AgentMetadata["routing_reasoning"] = routingReason
	if currentUser != nil && currentUser.Persona != "" {
		resp.AgentMetadata["persona"] = currentUser.Persona
	}
	models.WriteJSON(w, http.StatusOK, resp)
}

// QueryAgentStream handles POST /api/v1/query-agent/stream.
// It runs the same pipeline as QueryAgent but streams progress via Server-Sent Events.
// Each SSE event is a JSON object: {"event":"<type>","data":<payload>}
//
// Event types:
//   - start          — request accepted, validation beginning
//   - progress       — pipeline step update (step, dataset fields)
//   - llm_call       — LLM API call starting (iteration field)
//   - tool_call      — tool invocation (tool, iteration, sql_preview fields)
//   - result         — AgentResponse payload on success
//   - error          — error payload with message field
func (h *AgentHandler) QueryAgentStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		models.WriteError(w, http.StatusInternalServerError, "streaming not supported by this server")
		return
	}

	var req models.AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.SetDefaults()

	if req.Prompt == "" {
		models.WriteError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Determine data source
	var source service.DataSource
	if req.DataSource != nil && *req.DataSource != "" {
		source = service.DataSource(*req.DataSource)
	} else {
		source = h.router.Route(req.Prompt).Source
	}

	// Resolve user context and persona before routing decisions so that
	// persona-based restrictions (data source, tool filtering) are enforced
	// before any SSE headers are written.
	apiKey := r.Header.Get("X-API-Key")
	var allowedDatasets []string
	var allowedPGDatabases []string
	var currentUser *models.User
	if user, ok := middleware.GetCurrentUser(r.Context()); ok {
		currentUser = user
		if user.Squad != nil {
			allowedDatasets = user.Squad.Datasets
			allowedPGDatabases = user.Squad.PGDatabases
		}
	}
	runner, promptStyle, pc := h.resolvePersona(currentUser)

	// Persona-based data source restriction (must be before SSE headers so HTTP
	// status 403 can still be written to the response).
	if err := h.checkDataSourceAllowed(pc, string(source)); err != nil {
		models.WriteError(w, http.StatusForbidden, err.Error())
		return
	}

	// Streaming is currently only supported for BigQuery and PostgreSQL
	if source == service.DataSourceElasticsearch {
		models.WriteError(w, http.StatusNotImplemented, "streaming is not yet supported for Elasticsearch")
		return
	}

	// Pre-flight handler check before writing SSE headers
	if source == service.DataSourcePostgres && h.pgHandler == nil {
		models.WriteError(w, http.StatusServiceUnavailable, "PostgreSQL is not configured")
		return
	}
	if source != service.DataSourcePostgres && h.bqHandler == nil {
		models.WriteError(w, http.StatusServiceUnavailable, "BigQuery is not configured")
		return
	}

	// Set SSE headers before writing any body
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	emitSSE := func(event string, data interface{}) {
		payload, err := json.Marshal(map[string]interface{}{"event": event, "data": data})
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	if source == service.DataSourcePostgres {
		squadID := ""
		if currentUser != nil {
			squadID = currentUser.SquadID
		}
		h.pgHandler.HandleStream(r.Context(), &req, apiKey, squadID, allowedPGDatabases, runner, promptStyle, emitSSE, pc.ExcludedTools)
	} else {
		h.bqHandler.HandleStream(r.Context(), &req, apiKey, allowedDatasets, runner, promptStyle, emitSSE, pc.ExcludedTools)
	}
}

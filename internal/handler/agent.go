package handler

import (
	"encoding/json"
	"net/http"

	"github.com/cortexai/cortexai/internal/agent"
	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
)

// AgentHandler handles POST /api/v1/query-agent
type AgentHandler struct {
	bqHandler *agent.BigQueryHandler
	esHandler *agent.ElasticsearchHandler
	router    *service.IntentRouter
}

func NewAgentHandler(
	bqHandler *agent.BigQueryHandler,
	esHandler *agent.ElasticsearchHandler,
	router *service.IntentRouter,
) *AgentHandler {
	return &AgentHandler{
		bqHandler: bqHandler,
		esHandler: esHandler,
		router:    router,
	}
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

	var resp *models.AgentResponse
	var err error

	switch source {
	case service.DataSourceElasticsearch:
		if h.esHandler == nil {
			models.WriteError(w, http.StatusServiceUnavailable, "Elasticsearch is not configured")
			return
		}
		resp, err = h.esHandler.Handle(r.Context(), &req, apiKey)
	default:
		// FIX #1: nil check for bqHandler to prevent panic
		if h.bqHandler == nil {
			models.WriteError(w, http.StatusServiceUnavailable, "BigQuery is not configured")
			return
		}
		resp, err = h.bqHandler.Handle(r.Context(), &req, apiKey)
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
	models.WriteJSON(w, http.StatusOK, resp)
}

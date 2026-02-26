package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/go-chi/chi/v5"
)

// ElasticsearchHandler handles ES REST endpoints
type ElasticsearchHandler struct {
	es *service.ElasticsearchService
}

func NewElasticsearchHandler(es *service.ElasticsearchService) *ElasticsearchHandler {
	return &ElasticsearchHandler{es: es}
}

// Info handles GET /api/v1/elasticsearch/
func (h *ElasticsearchHandler) Info(w http.ResponseWriter, r *http.Request) {
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"module":  "elasticsearch",
		"version": "1.0.0",
	})
}

// Health handles GET /api/v1/elasticsearch/health
func (h *ElasticsearchHandler) Health(w http.ResponseWriter, r *http.Request) {
	health, err := h.es.GetClusterHealth(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusServiceUnavailable, "ES health check failed: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"health": health,
	})
}

// ClusterInfo handles GET /api/v1/elasticsearch/cluster/info
func (h *ElasticsearchHandler) ClusterInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.es.GetClusterInfo(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"info":   info,
	})
}

// ClusterHealth handles GET /api/v1/elasticsearch/cluster/health
func (h *ElasticsearchHandler) ClusterHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.es.GetClusterHealth(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"health": health,
	})
}

// ListIndices handles GET /api/v1/elasticsearch/indices
func (h *ElasticsearchHandler) ListIndices(w http.ResponseWriter, r *http.Request) {
	indices, err := h.es.ListIndices(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"indices": indices,
		"count":   len(indices),
	})
}

// GetIndex handles GET /api/v1/elasticsearch/indices/{index_name}
func (h *ElasticsearchHandler) GetIndex(w http.ResponseWriter, r *http.Request) {
	indexName := chi.URLParam(r, "index_name")
	info, err := h.es.GetIndexInfo(r.Context(), indexName)
	if err != nil {
		models.WriteError(w, http.StatusNotFound, fmt.Sprintf("index %q not found: %v", indexName, err))
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"index":  indexName,
		"info":   info,
	})
}

// Search handles POST /api/v1/elasticsearch/search
func (h *ElasticsearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req models.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.SetDefaults()

	if req.Index == "" {
		models.WriteError(w, http.StatusBadRequest, "index is required")
		return
	}

	resp, err := h.es.Search(r.Context(), &req)
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, resp)
}

// Count handles POST /api/v1/elasticsearch/count
func (h *ElasticsearchHandler) Count(w http.ResponseWriter, r *http.Request) {
	var req models.CountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Index == "" {
		models.WriteError(w, http.StatusBadRequest, "index is required")
		return
	}

	count, err := h.es.Count(r.Context(), req.Index, req.Query)
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "count failed: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"index":  req.Index,
		"count":  count,
	})
}

// Aggregate handles POST /api/v1/elasticsearch/aggregate
func (h *ElasticsearchHandler) Aggregate(w http.ResponseWriter, r *http.Request) {
	var req models.AggregateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Index == "" {
		models.WriteError(w, http.StatusBadRequest, "index is required")
		return
	}

	result, err := h.es.Aggregate(r.Context(), req.Index, req.Aggregations, req.Query, req.Size)
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "aggregation failed: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"result": result,
	})
}

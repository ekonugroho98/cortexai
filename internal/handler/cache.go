package handler

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/agent"
	"github.com/cortexai/cortexai/internal/models"
	"github.com/go-chi/chi/v5"
)

// CacheHandler handles cache management endpoints.
type CacheHandler struct {
	bqHandler *agent.BigQueryHandler
	pgHandler *agent.PostgresHandler
}

func NewCacheHandler(bqHandler *agent.BigQueryHandler, pgHandler *agent.PostgresHandler) *CacheHandler {
	return &CacheHandler{bqHandler: bqHandler, pgHandler: pgHandler}
}

// InvalidateSchemaCache handles DELETE /api/v1/cache/schema/{dataset}.
// It removes the cached BigQuery schema for the given dataset so the next
// query-agent request re-fetches fresh schema from BigQuery.
func (h *CacheHandler) InvalidateSchemaCache(w http.ResponseWriter, r *http.Request) {
	dataset := chi.URLParam(r, "dataset")
	if dataset == "" {
		models.WriteError(w, http.StatusBadRequest, "dataset path parameter is required")
		return
	}
	h.bqHandler.InvalidateSchemaCache(dataset)
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "schema cache invalidated",
		"dataset": dataset,
	})
}

// FlushResponseCache handles DELETE /api/v1/cache/responses.
// It clears all in-memory agent response caches for BQ and PG handlers,
// forcing the next identical query to run the full LLM pipeline again.
func (h *CacheHandler) FlushResponseCache(w http.ResponseWriter, r *http.Request) {
	if h.bqHandler != nil {
		h.bqHandler.FlushResponseCache()
	}
	if h.pgHandler != nil {
		h.pgHandler.FlushResponseCache()
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "response cache flushed",
	})
}

// InvalidatePGSchemaCache handles DELETE /api/v1/cache/pg-schema/{squad}/{database}.
// It removes the cached PostgreSQL schema for the given squad+database.
func (h *CacheHandler) InvalidatePGSchemaCache(w http.ResponseWriter, r *http.Request) {
	if h.pgHandler == nil {
		models.WriteError(w, http.StatusServiceUnavailable, "PostgreSQL is not configured")
		return
	}
	squad := chi.URLParam(r, "squad")
	database := chi.URLParam(r, "database")
	if squad == "" || database == "" {
		models.WriteError(w, http.StatusBadRequest, "squad and database path parameters are required")
		return
	}
	cacheKey := squad + ":" + database
	h.pgHandler.InvalidateSchemaCache(cacheKey)
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"message":  "pg schema cache invalidated",
		"squad":    squad,
		"database": database,
	})
}

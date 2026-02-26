package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
)

const version = "1.0.0"

// HealthChecker is implemented by services that can report connectivity
type HealthChecker interface {
	TestConnection(ctx context.Context) error
}

// HealthHandler handles GET /health with optional dependency checks
type HealthHandler struct {
	bq *service.BigQueryService
	es *service.ElasticsearchService
}

func NewHealthHandler(bq *service.BigQueryService, es *service.ElasticsearchService) *HealthHandler {
	return &HealthHandler{bq: bq, es: es}
}

// Health handles GET /health
// FIX #6: check actual dependency connectivity instead of always returning 200
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{"server": "ok"}
	overallStatus := "healthy"

	// Use a short timeout for health checks so they don't block
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if h.bq != nil {
		if err := h.bq.TestConnection(ctx); err != nil {
			checks["bigquery"] = "unavailable: " + err.Error()
			overallStatus = "degraded"
		} else {
			checks["bigquery"] = "ok"
		}
	} else {
		checks["bigquery"] = "disabled"
	}

	if h.es != nil {
		if err := h.es.TestConnection(ctx); err != nil {
			checks["elasticsearch"] = "unavailable: " + err.Error()
			overallStatus = "degraded"
		} else {
			checks["elasticsearch"] = "ok"
		}
	} else {
		checks["elasticsearch"] = "disabled"
	}

	statusCode := http.StatusOK
	if overallStatus == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	models.WriteJSON(w, statusCode, models.HealthResponse{
		Status:  overallStatus,
		Version: version,
		Checks:  checks,
	})
}

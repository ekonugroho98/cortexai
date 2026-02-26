package handler

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/go-chi/chi/v5"
)

// DatasetsHandler handles BigQuery dataset endpoints
type DatasetsHandler struct {
	bq *service.BigQueryService
}

func NewDatasetsHandler(bq *service.BigQueryService) *DatasetsHandler {
	return &DatasetsHandler{bq: bq}
}

// ListDatasets handles GET /api/v1/datasets
func (h *DatasetsHandler) ListDatasets(w http.ResponseWriter, r *http.Request) {
	datasets, err := h.bq.ListDatasets(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "failed to list datasets: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"datasets": datasets,
		"count":    len(datasets),
	})
}

// GetDataset handles GET /api/v1/datasets/{dataset_id}
func (h *DatasetsHandler) GetDataset(w http.ResponseWriter, r *http.Request) {
	datasetID := chi.URLParam(r, "dataset_id")
	ds, err := h.bq.GetDataset(r.Context(), datasetID)
	if err != nil {
		models.WriteError(w, http.StatusNotFound, "dataset not found: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"dataset": ds,
	})
}

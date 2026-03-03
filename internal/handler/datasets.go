package handler

import (
	"fmt"
	"net/http"

	"github.com/cortexai/cortexai/internal/middleware"
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

// ListDatasets handles GET /api/v1/datasets.
// Response is filtered to the caller's squad datasets when a squad is configured.
func (h *DatasetsHandler) ListDatasets(w http.ResponseWriter, r *http.Request) {
	datasets, err := h.bq.ListDatasets(r.Context())
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "failed to list datasets: "+err.Error())
		return
	}

	// Filter to squad's datasets if the user belongs to a squad
	if user, ok := middleware.GetCurrentUser(r.Context()); ok && user.Squad != nil && len(user.Squad.Datasets) > 0 {
		allowed := make(map[string]bool, len(user.Squad.Datasets))
		for _, d := range user.Squad.Datasets {
			allowed[d] = true
		}
		filtered := datasets[:0]
		for _, ds := range datasets {
			if allowed[ds.ID] {
				filtered = append(filtered, ds)
			}
		}
		datasets = filtered
	}

	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"datasets": datasets,
		"count":    len(datasets),
	})
}

// GetDataset handles GET /api/v1/datasets/{dataset_id}.
// Returns 403 if the dataset is outside the caller's squad.
func (h *DatasetsHandler) GetDataset(w http.ResponseWriter, r *http.Request) {
	datasetID := chi.URLParam(r, "dataset_id")

	if user, ok := middleware.GetCurrentUser(r.Context()); ok && user.Squad != nil && len(user.Squad.Datasets) > 0 {
		if !user.Squad.AllowsDataset(datasetID) {
			models.WriteError(w, http.StatusForbidden,
				fmt.Sprintf("dataset '%s' is not accessible for your squad", datasetID))
			return
		}
	}

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

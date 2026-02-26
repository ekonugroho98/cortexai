package handler

import (
	"net/http"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
	"github.com/go-chi/chi/v5"
)

// TablesHandler handles BigQuery table endpoints
type TablesHandler struct {
	bq *service.BigQueryService
}

func NewTablesHandler(bq *service.BigQueryService) *TablesHandler {
	return &TablesHandler{bq: bq}
}

// ListTables handles GET /api/v1/datasets/{dataset_id}/tables
func (h *TablesHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	datasetID := chi.URLParam(r, "dataset_id")
	tables, err := h.bq.ListTables(r.Context(), datasetID)
	if err != nil {
		models.WriteError(w, http.StatusInternalServerError, "failed to list tables: "+err.Error())
		return
	}
	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"tables": tables,
		"count":  len(tables),
	})
}

// GetTable handles GET /api/v1/datasets/{dataset_id}/tables/{table_id}
func (h *TablesHandler) GetTable(w http.ResponseWriter, r *http.Request) {
	datasetID := chi.URLParam(r, "dataset_id")
	tableID := chi.URLParam(r, "table_id")

	schema, meta, err := h.bq.GetTableSchema(r.Context(), datasetID, tableID)
	if err != nil {
		models.WriteError(w, http.StatusNotFound, "table not found: "+err.Error())
		return
	}

	fields := make([]map[string]interface{}, len(schema))
	for i, f := range schema {
		fields[i] = map[string]interface{}{
			"name": f.Name,
			"type": f.Type,
		}
	}

	models.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"dataset":   datasetID,
		"table":     tableID,
		"num_rows":  meta.NumRows,
		"num_bytes": meta.NumBytes,
		"schema":    fields,
	})
}

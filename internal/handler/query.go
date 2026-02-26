package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/security"
	"github.com/cortexai/cortexai/internal/service"
)

// QueryHandler handles direct SQL query execution
type QueryHandler struct {
	bq          *service.BigQueryService
	sqlVal      *security.SQLValidator
	costTracker *security.CostTracker
	dataMasker  *security.DataMasker
	auditLogger *security.AuditLogger
	enableMask  bool
}

func NewQueryHandler(
	bq *service.BigQueryService,
	sqlVal *security.SQLValidator,
	costTracker *security.CostTracker,
	dataMasker *security.DataMasker,
	auditLogger *security.AuditLogger,
	enableMask bool,
) *QueryHandler {
	return &QueryHandler{
		bq:          bq,
		sqlVal:      sqlVal,
		costTracker: costTracker,
		dataMasker:  dataMasker,
		auditLogger: auditLogger,
		enableMask:  enableMask,
	}
}

// Execute handles POST /api/v1/query
func (h *QueryHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req models.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.SetDefaults()

	// SQL validation
	if errMsg := h.sqlVal.Validate(req.SQL); errMsg != "" {
		models.WriteError(w, http.StatusBadRequest, "SQL validation failed: "+errMsg)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	start := time.Now()

	projectID := ""
	if req.ProjectID != nil {
		projectID = *req.ProjectID
	}

	result, err := h.bq.ExecuteQuery(r.Context(), req.SQL, projectID, req.DryRun, req.TimeoutMs, req.UseQueryCache, req.UseLegacySQL)
	if err != nil {
		execMs := time.Since(start).Milliseconds()
		h.auditLogger.LogQuery(req.SQL, apiKey, "", execMs, 0, 0, false, err.Error())
		models.WriteError(w, http.StatusInternalServerError, "query execution failed: "+err.Error())
		return
	}

	execMs := time.Since(start).Milliseconds()

	// Cost check
	if ok, errMsg := h.costTracker.CheckLimits(result.TotalBytesProcessed, apiKey); !ok {
		h.auditLogger.LogQuery(req.SQL, apiKey, "", execMs, 0, result.TotalBytesProcessed, false, errMsg)
		models.WriteError(w, http.StatusTooManyRequests, errMsg)
		return
	}

	h.costTracker.LogQueryCost(req.SQL, result.TotalBytesProcessed, apiKey, execMs)

	// Data masking
	data := result.Data
	if h.enableMask {
		data = h.dataMasker.MaskRows(data)
	}

	h.auditLogger.LogQuery(req.SQL, apiKey, "", execMs, len(data), result.TotalBytesProcessed, true, "")

	models.WriteJSON(w, http.StatusOK, models.QueryResponse{
		Status:   "success",
		Data:     data,
		Columns:  result.Columns,
		RowCount: len(data),
		Metadata: models.QueryMetadata{
			JobID:               result.JobID,
			TotalBytesProcessed: result.TotalBytesProcessed,
			BytesBilled:         result.BytesBilled,
			CacheHit:            result.CacheHit,
			ExecutionTimeMs:     execMs,
		},
	})
}

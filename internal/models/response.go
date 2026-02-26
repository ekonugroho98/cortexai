package models

// HealthResponse is returned by GET /health
type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// QueryMetadata contains BigQuery job metadata
type QueryMetadata struct {
	JobID               string  `json:"job_id"`
	TotalRowsProcessed  int64   `json:"total_rows_processed"`
	TotalBytesProcessed int64   `json:"total_bytes_processed"`
	BytesBilled         int64   `json:"bytes_billed"`
	CacheHit            bool    `json:"cache_hit"`
	ExecutionTimeMs     int64   `json:"execution_time_ms"`
	SlotTimeMs          *int64  `json:"slot_time_ms,omitempty"`
}

// QueryResponse is returned by POST /api/v1/query
type QueryResponse struct {
	Status   string                   `json:"status"`
	Data     []map[string]interface{} `json:"data"`
	Metadata QueryMetadata            `json:"metadata"`
	RowCount int                      `json:"row_count"`
	Columns  []string                 `json:"columns"`
}

// DatasetInfo represents a BigQuery dataset
type DatasetInfo struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Location    string `json:"location"`
	Description string `json:"description,omitempty"`
}

// TableInfo represents a BigQuery table
type TableInfo struct {
	ID        string `json:"id"`
	DatasetID string `json:"dataset_id"`
	Type      string `json:"type"`
	NumRows   uint64 `json:"num_rows"`
	NumBytes  int64  `json:"num_bytes"`
}

// AgentResponse is returned by POST /api/v1/query-agent
type AgentResponse struct {
	Status          string                 `json:"status"`
	Prompt          string                 `json:"prompt"`
	GeneratedSQL    *string                `json:"generated_sql,omitempty"`
	ExecutionResult *QueryResponse         `json:"execution_result,omitempty"`
	AgentMetadata   map[string]interface{} `json:"agent_metadata"`
	Reasoning       *string                `json:"reasoning,omitempty"`
	Answer          *string                `json:"answer,omitempty"`
}

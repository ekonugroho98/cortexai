package models

// QueryRequest for POST /api/v1/query (direct SQL)
type QueryRequest struct {
	SQL             string  `json:"sql"`
	ProjectID       *string `json:"project_id,omitempty"`
	DryRun          bool    `json:"dry_run"`
	TimeoutMs       int     `json:"timeout_ms"`
	UseQueryCache   bool    `json:"use_query_cache"`
	UseLegacySQL    bool    `json:"use_legacy_sql"`
}

func (r *QueryRequest) SetDefaults() {
	if r.TimeoutMs == 0 {
		r.TimeoutMs = 60000
	}
	if r.TimeoutMs < 1000 {
		r.TimeoutMs = 1000
	}
	if r.TimeoutMs > 300000 {
		r.TimeoutMs = 300000
	}
	if !r.DryRun {
		r.UseQueryCache = true
	}
}

// AgentRequest for POST /api/v1/query-agent
type AgentRequest struct {
	Prompt     string  `json:"prompt"`
	ProjectID  *string `json:"project_id,omitempty"`
	DatasetID  *string `json:"dataset_id,omitempty"`
	DataSource *string `json:"data_source,omitempty"` // "bigquery" | "elasticsearch"
	DryRun     bool    `json:"dry_run"`
	Timeout    int     `json:"timeout"`
}

func (r *AgentRequest) SetDefaults() {
	if r.Timeout == 0 {
		r.Timeout = 300
	}
	if r.Timeout < 10 {
		r.Timeout = 10
	}
	if r.Timeout > 600 {
		r.Timeout = 600
	}
}

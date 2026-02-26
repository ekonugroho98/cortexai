package models

// SearchRequest for POST /api/v1/elasticsearch/search
type SearchRequest struct {
	Index        string                 `json:"index"`
	Query        map[string]interface{} `json:"query,omitempty"`
	Size         int                    `json:"size"`
	From         int                    `json:"from"`
	Sort         []string               `json:"sort,omitempty"`
	SourceFields []string               `json:"source_fields,omitempty"`
}

func (r *SearchRequest) SetDefaults() {
	if r.Size == 0 {
		r.Size = 10
	}
	if r.Size > 10000 {
		r.Size = 10000
	}
}

// SearchResponse for ES search results
type SearchResponse struct {
	Status       string                   `json:"status"`
	Index        string                   `json:"index"`
	Took         int                      `json:"took"`
	TimedOut     bool                     `json:"timed_out"`
	TotalHits    int64                    `json:"total_hits"`
	MaxScore     *float64                 `json:"max_score,omitempty"`
	Hits         []map[string]interface{} `json:"hits"`
	Aggregations map[string]interface{}   `json:"aggregations,omitempty"`
	Query        map[string]interface{}   `json:"query,omitempty"`
}

// CountRequest for POST /api/v1/elasticsearch/count
type CountRequest struct {
	Index string                 `json:"index"`
	Query map[string]interface{} `json:"query,omitempty"`
}

// AggregateRequest for POST /api/v1/elasticsearch/aggregate
type AggregateRequest struct {
	Index        string                 `json:"index"`
	Aggregations map[string]interface{} `json:"aggregations"`
	Query        map[string]interface{} `json:"query,omitempty"`
	Size         int                    `json:"size"`
}

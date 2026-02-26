package service

import "strings"

// DataSource represents which backend handles the query
type DataSource string

const (
	DataSourceBigQuery       DataSource = "bigquery"
	DataSourceElasticsearch  DataSource = "elasticsearch"
)

var elasticsearchKeywords = []string{
	// ES-specific: log investigation, real-time search
	"logs", "log", "exception", "stack trace", "stacktrace",
	"message", "timestamp", "warn", "debug",
	"elasticsearch", "index", "document", "kibana",
	"last hour", "last 24", "last minute",
	"investigation", "investigate", "what happened", "troubleshoot",
	"trace id", "request id", "correlation id",
}

var bigqueryKeywords = []string{
	// BQ-specific: analytics, reporting, aggregation
	"table", "dataset", "row", "column", "sql", "query",
	"analytics", "report", "aggregate", "sum", "count", "average",
	"bigquery", "warehouse", "data", "metrics", "kpi",
	"top", "bottom", "group by", "order by",
	"revenue", "sales", "transaction", "order", "payment",
	"user", "customer", "driver", "monthly", "daily", "weekly",
	"per bulan", "per hari", "per minggu", "total", "jumlah",
}

// RoutingResult contains data source routing info
type RoutingResult struct {
	Source     DataSource
	Confidence float64
	ESScore    int
	BQScore    int
	Reasoning  string
}

// IntentRouter routes natural language prompts to the appropriate data source
type IntentRouter struct{}

func NewIntentRouter() *IntentRouter {
	return &IntentRouter{}
}

// Route analyses the prompt and returns the best matching data source
func (r *IntentRouter) Route(prompt string) RoutingResult {
	lower := strings.ToLower(prompt)

	esScore := 0
	bqScore := 0

	for _, kw := range elasticsearchKeywords {
		if strings.Contains(lower, kw) {
			esScore++
		}
	}
	for _, kw := range bigqueryKeywords {
		if strings.Contains(lower, kw) {
			bqScore++
		}
	}

	total := esScore + bqScore
	if total == 0 {
		return RoutingResult{
			Source:     DataSourceBigQuery,
			Confidence: 0.5,
			ESScore:    0,
			BQScore:    0,
			Reasoning:  "no strong keywords, defaulting to BigQuery",
		}
	}

	if esScore > bqScore {
		confidence := float64(esScore) / float64(total)
		return RoutingResult{
			Source:     DataSourceElasticsearch,
			Confidence: confidence,
			ESScore:    esScore,
			BQScore:    bqScore,
			Reasoning:  "prompt contains Elasticsearch-related keywords",
		}
	}

	confidence := float64(bqScore) / float64(total)
	return RoutingResult{
		Source:     DataSourceBigQuery,
		Confidence: confidence,
		ESScore:    esScore,
		BQScore:    bqScore,
		Reasoning:  "prompt contains BigQuery/analytics-related keywords",
	}
}

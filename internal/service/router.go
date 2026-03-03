package service

import "strings"

// DataSource represents which backend handles the query
type DataSource string

const (
	DataSourceBigQuery      DataSource = "bigquery"
	DataSourceElasticsearch DataSource = "elasticsearch"
	DataSourcePostgres      DataSource = "postgres"
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

var postgresKeywords = []string{
	"postgres", "postgresql", "pg_", "pg ",
	"relational", "foreign key", "primary key",
	"sequence", "trigger", "stored procedure",
	"schema public", "information_schema",
	"serial", "uuid", "jsonb", "hstore",
	"lateral", "window function", "cte",
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
	PGScore    int
	Reasoning  string
}

// IntentRouter routes natural language prompts to the appropriate data source
type IntentRouter struct{}

func NewIntentRouter() *IntentRouter {
	return &IntentRouter{}
}

// Route analyses the prompt and returns the best matching data source.
// Three-way scoring: BQ vs PG vs ES. Tie-break order: BQ > PG > ES.
func (r *IntentRouter) Route(prompt string) RoutingResult {
	lower := strings.ToLower(prompt)

	esScore := 0
	bqScore := 0
	pgScore := 0

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
	for _, kw := range postgresKeywords {
		if strings.Contains(lower, kw) {
			pgScore++
		}
	}

	total := esScore + bqScore + pgScore
	if total == 0 {
		return RoutingResult{
			Source:     DataSourceBigQuery,
			Confidence: 0.5,
			Reasoning:  "no strong keywords, defaulting to BigQuery",
		}
	}

	// Determine winner: highest score wins; tie-break BQ > PG > ES
	winner := DataSourceBigQuery
	winScore := bqScore
	reason := "prompt contains BigQuery/analytics-related keywords"

	if pgScore > winScore {
		winner = DataSourcePostgres
		winScore = pgScore
		reason = "prompt contains PostgreSQL-related keywords"
	}
	if esScore > winScore {
		winner = DataSourceElasticsearch
		winScore = esScore
		reason = "prompt contains Elasticsearch-related keywords"
	}

	confidence := float64(winScore) / float64(total)
	return RoutingResult{
		Source:     winner,
		Confidence: confidence,
		ESScore:    esScore,
		BQScore:    bqScore,
		PGScore:    pgScore,
		Reasoning:  reason,
	}
}

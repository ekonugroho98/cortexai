package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// BQListDatasetsTool lists all BigQuery datasets
func BQListDatasetsTool(bq *service.BigQueryService) Tool {
	return Tool{
		Name:        "list_bigquery_datasets",
		Description: "List all available BigQuery datasets in the project. Use this to discover what data is available.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			datasets, err := bq.ListDatasets(ctx)
			if err != nil {
				return "", fmt.Errorf("list datasets: %w", err)
			}
			b, err := json.Marshal(datasets)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// BQListDatasetsTool lists BigQuery datasets accessible to the caller.
// allowedDatasets restricts the result to a squad's datasets; nil = no restriction.
func BQListDatasetsTool(bq *service.BigQueryService, allowedDatasets []string) Tool {
	// Pre-build lookup for O(1) checks at execute time
	var allowedSet map[string]bool
	if len(allowedDatasets) > 0 {
		allowedSet = make(map[string]bool, len(allowedDatasets))
		for _, d := range allowedDatasets {
			allowedSet[d] = true
		}
	}

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

			// Filter to squad's allowed datasets if restriction is set
			if allowedSet != nil {
				filtered := datasets[:0]
				for _, ds := range datasets {
					if allowedSet[ds.ID] {
						filtered = append(filtered, ds)
					}
				}
				datasets = filtered
			}

			b, err := json.Marshal(datasets)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}
}

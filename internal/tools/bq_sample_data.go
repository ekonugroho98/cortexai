package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// BQSampleDataTool fetches a few sample rows from a table so the agent
// can understand actual data values, types, and join key relationships.
func BQSampleDataTool(bq *service.BigQueryService) Tool {
	return Tool{
		Name:        "get_bigquery_sample_data",
		Description: "Get 3 sample rows from a BigQuery table to understand actual data values, formats, and relationships. Use this before writing JOIN queries to verify foreign key values match across tables.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dataset_id": map[string]interface{}{
					"type":        "string",
					"description": "The BigQuery dataset ID",
				},
				"table_id": map[string]interface{}{
					"type":        "string",
					"description": "The BigQuery table ID",
				},
			},
			"required": []string{"dataset_id", "table_id"},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			datasetID, _ := input["dataset_id"].(string)
			tableID, _ := input["table_id"].(string)
			if datasetID == "" || tableID == "" {
				return "", fmt.Errorf("dataset_id and table_id are required")
			}

			sql := fmt.Sprintf("SELECT * FROM `%s.%s` LIMIT 3", datasetID, tableID)
			result, err := bq.ExecuteQuery(ctx, sql, "", false, 10000, true, false)
			if err != nil {
				return "", fmt.Errorf("sample data: %w", err)
			}

			out := map[string]interface{}{
				"table":   fmt.Sprintf("%s.%s", datasetID, tableID),
				"columns": result.Columns,
				"sample":  result.Data,
				"note":    "These are sample rows only. Use these to understand data format and join key values.",
			}
			b, err := json.Marshal(out)
			if err != nil {
				return "", fmt.Errorf("marshal sample: %w", err)
			}
			return string(b), nil
		},
	}
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// BQExecuteQueryTool executes a SQL query and returns results
func BQExecuteQueryTool(bq *service.BigQueryService) Tool {
	return Tool{
		Name:        "execute_bigquery_sql",
		Description: "Execute a SQL SELECT query on BigQuery and return the results. Only SELECT queries are allowed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sql": map[string]interface{}{
					"type":        "string",
					"description": "The SQL SELECT query to execute",
				},
			},
			"required": []string{"sql"},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			sql, _ := input["sql"].(string)
			if sql == "" {
				return "", fmt.Errorf("sql is required")
			}

			result, err := bq.ExecuteQuery(ctx, sql, "", false, 60000, true, false)
			if err != nil {
				return "", fmt.Errorf("execute query: %w", err)
			}

			out := map[string]interface{}{
				"row_count":       len(result.Data),
				"columns":         result.Columns,
				"data":            result.Data,
				"bytes_processed": result.TotalBytesProcessed,
			}
			b, _ := json.Marshal(out)
			return string(b), nil
		},
	}
}

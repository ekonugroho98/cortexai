package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// PGSampleDataTool fetches 3 sample rows from a PostgreSQL table.
func PGSampleDataTool(pg *service.PostgresService, dbName string) Tool {
	return Tool{
		Name:        "get_postgres_sample_data",
		Description: "Fetch 3 sample rows from a PostgreSQL table. Useful for verifying join key values before writing complex queries.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"schema": map[string]interface{}{
					"type":        "string",
					"description": "The PostgreSQL schema name (e.g. 'public')",
				},
				"table": map[string]interface{}{
					"type":        "string",
					"description": "The table name",
				},
			},
			"required": []string{"schema", "table"},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			schema, _ := input["schema"].(string)
			table, _ := input["table"].(string)
			if schema == "" {
				return "", fmt.Errorf("schema is required")
			}
			if table == "" {
				return "", fmt.Errorf("table is required")
			}

			result, err := pg.GetSampleData(ctx, dbName, schema, table)
			if err != nil {
				return "", fmt.Errorf("sample data: %w", err)
			}

			out := map[string]interface{}{
				"table":   fmt.Sprintf("%s.%s", schema, table),
				"columns": result.Columns,
				"sample":  result.Data,
				"note":    "Showing up to 3 sample rows",
			}
			b, _ := json.Marshal(out)
			return string(b), nil
		},
	}
}

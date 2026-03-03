package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// PGExecuteQueryTool runs a read-only SQL query against a PostgreSQL database.
func PGExecuteQueryTool(pg *service.PostgresService, dbName string) Tool {
	return Tool{
		Name:        "execute_postgres_sql",
		Description: "Execute a read-only SELECT query against the PostgreSQL database. Only SELECT statements are allowed.",
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
			sqlQuery, _ := input["sql"].(string)
			if sqlQuery == "" {
				return "", fmt.Errorf("sql is required")
			}

			result, err := pg.ExecuteQuery(ctx, dbName, sqlQuery, 60000)
			if err != nil {
				return "", fmt.Errorf("execute query: %w", err)
			}

			out := map[string]interface{}{
				"row_count": result.RowCount,
				"columns":   result.Columns,
				"data":      result.Data,
			}
			b, _ := json.Marshal(out)
			return string(b), nil
		},
	}
}

package tools

import (
	"context"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// PGGetSchemaTool returns column details for a specific PostgreSQL table.
func PGGetSchemaTool(pg *service.PostgresService, dbName string) Tool {
	return Tool{
		Name:        "get_postgres_schema",
		Description: "Get column details (name, data type, nullable) for a specific PostgreSQL table. Provide the schema and table name.",
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

			cols, err := pg.GetTableSchema(ctx, dbName, schema, table)
			if err != nil {
				return "", fmt.Errorf("get schema: %w", err)
			}

			result := fmt.Sprintf("Table: %s.%s\nColumns:\n%s", schema, table, service.PGSchemaToString(cols))
			return result, nil
		},
	}
}

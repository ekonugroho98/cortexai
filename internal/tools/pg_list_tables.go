package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// PGListTablesTool lists tables in a PostgreSQL database.
func PGListTablesTool(pg *service.PostgresService, dbName string) Tool {
	return Tool{
		Name:        "list_postgres_tables",
		Description: "List all tables and views in the PostgreSQL database. Returns schema, name, and type for each table.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			tables, err := pg.ListTables(ctx, dbName)
			if err != nil {
				return "", fmt.Errorf("list tables: %w", err)
			}
			b, _ := json.Marshal(tables)
			return string(b), nil
		},
	}
}

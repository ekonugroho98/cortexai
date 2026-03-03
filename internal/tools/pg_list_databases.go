package tools

import (
	"context"
	"encoding/json"
)

// PGListDatabasesTool lists PostgreSQL databases accessible to the caller.
// allowedDatabases restricts the result to a squad's databases; nil = no restriction.
func PGListDatabasesTool(allowedDatabases []string) Tool {
	return Tool{
		Name:        "list_postgres_databases",
		Description: "List all available PostgreSQL databases. Use this to discover what data is available.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			type dbInfo struct {
				Name string `json:"name"`
			}
			var result []dbInfo
			for _, db := range allowedDatabases {
				result = append(result, dbInfo{Name: db})
			}
			b, _ := json.Marshal(result)
			return string(b), nil
		},
	}
}

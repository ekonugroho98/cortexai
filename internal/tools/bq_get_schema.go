package tools

import (
	"context"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// BQGetSchemaTool returns the schema for a BigQuery table
func BQGetSchemaTool(bq *service.BigQueryService) Tool {
	return Tool{
		Name:        "get_bigquery_schema",
		Description: "Get the schema (column names and types) for a specific BigQuery table. Use this before writing SQL to understand the table structure.",
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

			schema, meta, err := bq.GetTableSchema(ctx, datasetID, tableID)
			if err != nil {
				return "", fmt.Errorf("get schema: %w", err)
			}

			schemaStr := service.SchemaToString(schema)
			return fmt.Sprintf("Table: %s.%s\nRows: %d\nSchema:\n%s",
				datasetID, tableID, meta.NumRows, schemaStr), nil
		},
	}
}

// BQListTablesTool lists tables in a dataset
func BQListTablesTool(bq *service.BigQueryService) Tool {
	return Tool{
		Name:        "list_bigquery_tables",
		Description: "List all tables in a BigQuery dataset.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dataset_id": map[string]interface{}{
					"type":        "string",
					"description": "The BigQuery dataset ID",
				},
			},
			"required": []string{"dataset_id"},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			datasetID, _ := input["dataset_id"].(string)
			if datasetID == "" {
				return "", fmt.Errorf("dataset_id is required")
			}

			tables, err := bq.ListTables(ctx, datasetID)
			if err != nil {
				return "", fmt.Errorf("list tables: %w", err)
			}

			result := fmt.Sprintf("Tables in dataset %q:\n", datasetID)
			for _, t := range tables {
				result += fmt.Sprintf("  - %s (type: %s, rows: %d)\n", t.ID, t.Type, t.NumRows)
			}
			return result, nil
		},
	}
}

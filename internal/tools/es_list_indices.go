package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/service"
)

// ESListIndicesTool lists available Elasticsearch indices
func ESListIndicesTool(es *service.ElasticsearchService) Tool {
	return Tool{
		Name:        "list_elasticsearch_indices",
		Description: "List all available Elasticsearch indices. Use this to discover which indices are available before searching.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			indices, err := es.ListIndices(ctx)
			if err != nil {
				return "", fmt.Errorf("list indices: %w", err)
			}
			b, err := json.Marshal(indices)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}
}

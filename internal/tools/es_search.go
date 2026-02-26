package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/cortexai/cortexai/internal/service"
)

// ESSearchTool executes an Elasticsearch search
func ESSearchTool(es *service.ElasticsearchService) Tool {
	return Tool{
		Name:        "elasticsearch_search",
		Description: "Search documents in Elasticsearch using Query DSL. Returns matching documents.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"index": map[string]interface{}{
					"type":        "string",
					"description": "Index pattern to search (e.g., 'logs-*', 'hc-upg-k8s-prd-*')",
				},
				"query": map[string]interface{}{
					"type":        "object",
					"description": "Elasticsearch Query DSL object",
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results to return (default: 10, max: 100)",
				},
			},
			"required": []string{"index"},
		},
		Execute: func(ctx context.Context, input map[string]interface{}) (string, error) {
			index, _ := input["index"].(string)
			if index == "" {
				return "", fmt.Errorf("index is required")
			}

			size := 10
			if s, ok := input["size"].(float64); ok {
				size = int(s)
			}
			if size > 100 {
				size = 100
			}

			req := &models.SearchRequest{
				Index: index,
				Size:  size,
			}
			if q, ok := input["query"].(map[string]interface{}); ok {
				req.Query = q
			}

			resp, err := es.Search(ctx, req)
			if err != nil {
				return "", fmt.Errorf("es search: %w", err)
			}

			out := map[string]interface{}{
				"total_hits": resp.TotalHits,
				"took_ms":    resp.Took,
				"hits":       resp.Hits,
			}
			// FIX #18: handle json.Marshal error
			b, err := json.Marshal(out)
			if err != nil {
				return "", fmt.Errorf("marshal search results: %w", err)
			}
			return string(b), nil
		},
	}
}

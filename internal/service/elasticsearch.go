package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cortexai/cortexai/internal/models"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// ElasticsearchService wraps the go-elasticsearch client
type ElasticsearchService struct {
	client          *elasticsearch.Client
	allowedPatterns []string // index patterns that are permitted
}

// NewElasticsearchService creates an ES client using go-elasticsearch/v8
func NewElasticsearchService(scheme, host string, port int, user, password string, verifyCerts bool, maxRetries, timeout int, allowedPatterns []string) (*ElasticsearchService, error) {
	addr := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	cfg := elasticsearch.Config{
		Addresses:  []string{addr},
		MaxRetries: maxRetries,
	}
	if user != "" {
		cfg.Username = user
		cfg.Password = password
	}

	// FIX #2: properly disable TLS verification using crypto/tls
	if !verifyCerts {
		cfg.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402 - user explicitly disabled cert verification
			},
		}
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch.NewClient: %w", err)
	}
	return &ElasticsearchService{
		client:          client,
		allowedPatterns: allowedPatterns,
	}, nil
}

// IsIndexAllowed returns true if the index matches any of the allowed patterns.
// If no patterns are configured, all indices are allowed.
func (s *ElasticsearchService) IsIndexAllowed(index string) bool {
	if len(s.allowedPatterns) == 0 {
		return true
	}
	for _, pattern := range s.allowedPatterns {
		matched, err := filepath.Match(pattern, index)
		if err == nil && matched {
			return true
		}
		// Also allow if index starts with the pattern prefix (without wildcard)
		prefix := strings.TrimSuffix(pattern, "*")
		if prefix != pattern && strings.HasPrefix(index, prefix) {
			return true
		}
	}
	return false
}

// AllowedPatterns returns the configured index patterns
func (s *ElasticsearchService) AllowedPatterns() []string {
	return s.allowedPatterns
}

// TestConnection pings the cluster
func (s *ElasticsearchService) TestConnection(ctx context.Context) error {
	res, err := s.client.Ping(s.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("ping error: %s", res.Status())
	}
	return nil
}

// GetClusterInfo returns basic cluster info
func (s *ElasticsearchService) GetClusterInfo(ctx context.Context) (map[string]interface{}, error) {
	res, err := s.client.Info(s.client.Info.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return decodeBody(res.Body, res.Status())
}

// GetClusterHealth returns cluster health status
func (s *ElasticsearchService) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	res, err := s.client.Cluster.Health(
		s.client.Cluster.Health.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return decodeBody(res.Body, res.Status())
}

// ListIndices returns all indices, filtered by allowedPatterns if configured
func (s *ElasticsearchService) ListIndices(ctx context.Context) ([]map[string]interface{}, error) {
	res, err := s.client.Cat.Indices(
		s.client.Cat.Indices.WithContext(ctx),
		s.client.Cat.Indices.WithFormat("json"),
		s.client.Cat.Indices.WithH("index,docs.count,store.size,health,status"),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("list indices error: %s", res.Status())
	}

	var all []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decode indices: %w", err)
	}

	// FIX #4: filter by allowedPatterns
	if len(s.allowedPatterns) == 0 {
		return all, nil
	}
	var filtered []map[string]interface{}
	for _, idx := range all {
		name, _ := idx["index"].(string)
		if s.IsIndexAllowed(name) {
			filtered = append(filtered, idx)
		}
	}
	return filtered, nil
}

// GetIndexInfo returns index mapping and settings
func (s *ElasticsearchService) GetIndexInfo(ctx context.Context, indexName string) (map[string]interface{}, error) {
	// FIX #4: enforce allowedPatterns for direct index access
	if !s.IsIndexAllowed(indexName) {
		return nil, fmt.Errorf("access to index %q is not permitted", indexName)
	}
	res, err := s.client.Indices.Get(
		[]string{indexName},
		s.client.Indices.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return decodeBody(res.Body, res.Status())
}

// GetMapping returns index mapping
func (s *ElasticsearchService) GetMapping(ctx context.Context, index string) (map[string]interface{}, error) {
	if !s.IsIndexAllowed(index) {
		return nil, fmt.Errorf("access to index %q is not permitted", index)
	}
	res, err := s.client.Indices.GetMapping(
		s.client.Indices.GetMapping.WithContext(ctx),
		s.client.Indices.GetMapping.WithIndex(index),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return decodeBody(res.Body, res.Status())
}

// Search executes an ES search query, enforcing allowedPatterns
func (s *ElasticsearchService) Search(ctx context.Context, req *models.SearchRequest) (*models.SearchResponse, error) {
	// FIX #4: validate index against allowed patterns
	if !s.IsIndexAllowed(req.Index) {
		return nil, fmt.Errorf("access to index %q is not permitted", req.Index)
	}

	body := map[string]interface{}{
		"size": req.Size,
		"from": req.From,
	}
	if req.Query != nil {
		body["query"] = req.Query
	}
	if len(req.Sort) > 0 {
		body["sort"] = req.Sort
	}
	if len(req.SourceFields) > 0 {
		body["_source"] = req.SourceFields
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	opts := []func(*esapi.SearchRequest){
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(req.Index),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	}

	res, err := s.client.Search(opts...)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := decodeBody(res.Body, res.Status())
	if err != nil {
		return nil, err
	}

	return parseSearchResponse(req.Index, req.Query, raw), nil
}

// Count counts documents matching a query
func (s *ElasticsearchService) Count(ctx context.Context, index string, query map[string]interface{}) (int64, error) {
	if !s.IsIndexAllowed(index) {
		return 0, fmt.Errorf("access to index %q is not permitted", index)
	}

	var bodyBytes []byte
	var err error
	if query != nil {
		bodyBytes, err = json.Marshal(map[string]interface{}{"query": query})
		if err != nil {
			return 0, err
		}
	}

	opts := []func(*esapi.CountRequest){
		s.client.Count.WithContext(ctx),
		s.client.Count.WithIndex(index),
	}
	if len(bodyBytes) > 0 {
		opts = append(opts, s.client.Count.WithBody(bytes.NewReader(bodyBytes)))
	}

	res, err := s.client.Count(opts...)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	raw, err := decodeBody(res.Body, res.Status())
	if err != nil {
		return 0, err
	}

	if count, ok := raw["count"].(float64); ok {
		return int64(count), nil
	}
	return 0, nil
}

// Aggregate runs aggregation queries
func (s *ElasticsearchService) Aggregate(ctx context.Context, index string, aggs map[string]interface{}, query map[string]interface{}, size int) (map[string]interface{}, error) {
	if !s.IsIndexAllowed(index) {
		return nil, fmt.Errorf("access to index %q is not permitted", index)
	}

	body := map[string]interface{}{
		"size": size,
		"aggs": aggs,
	}
	if query != nil {
		body["query"] = query
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(index),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return decodeBody(res.Body, res.Status())
}

func decodeBody(r io.Reader, status string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5") {
		if errObj, ok := result["error"]; ok {
			return nil, fmt.Errorf("elasticsearch error [%s]: %v", status, errObj)
		}
		return nil, fmt.Errorf("elasticsearch error: %s", status)
	}
	return result, nil
}

func parseSearchResponse(index string, query map[string]interface{}, raw map[string]interface{}) *models.SearchResponse {
	resp := &models.SearchResponse{
		Status: "success",
		Index:  index,
		Query:  query,
	}

	if took, ok := raw["took"].(float64); ok {
		resp.Took = int(took)
	}
	if timedOut, ok := raw["timed_out"].(bool); ok {
		resp.TimedOut = timedOut
	}

	if hitsObj, ok := raw["hits"].(map[string]interface{}); ok {
		if totalObj, ok := hitsObj["total"].(map[string]interface{}); ok {
			if val, ok := totalObj["value"].(float64); ok {
				resp.TotalHits = int64(val)
			}
		}
		if maxScore, ok := hitsObj["max_score"].(float64); ok {
			resp.MaxScore = &maxScore
		}
		if hits, ok := hitsObj["hits"].([]interface{}); ok {
			for _, h := range hits {
				if hm, ok := h.(map[string]interface{}); ok {
					resp.Hits = append(resp.Hits, hm)
				}
			}
		}
	}

	if aggs, ok := raw["aggregations"].(map[string]interface{}); ok {
		resp.Aggregations = aggs
	}

	return resp
}

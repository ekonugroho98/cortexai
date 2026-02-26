package service

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cortexai/cortexai/internal/models"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// BigQueryService wraps the BigQuery SDK client
type BigQueryService struct {
	client    *bigquery.Client
	projectID string
	location  string
}

// NewBigQueryService creates a new BigQuery client
func NewBigQueryService(ctx context.Context, projectID, credentialsFile, location string) (*BigQueryService, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	client, err := bigquery.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("bigquery.NewClient: %w", err)
	}

	return &BigQueryService{
		client:    client,
		projectID: projectID,
		location:  location,
	}, nil
}

// Close releases the BigQuery client
func (s *BigQueryService) Close() error {
	return s.client.Close()
}

// TestConnection verifies BigQuery connectivity
func (s *BigQueryService) TestConnection(ctx context.Context) error {
	q := s.client.Query("SELECT 1")
	job, err := q.Run(ctx)
	if err != nil {
		return fmt.Errorf("query run: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("job wait: %w", err)
	}
	return status.Err()
}

// ListDatasets returns all datasets in the project
func (s *BigQueryService) ListDatasets(ctx context.Context) ([]models.DatasetInfo, error) {
	var datasets []models.DatasetInfo
	it := s.client.Datasets(ctx)
	for {
		ds, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list datasets: %w", err)
		}
		meta, err := ds.Metadata(ctx)
		if err != nil {
			log.Warn().Err(err).Str("dataset", ds.DatasetID).Msg("failed to get metadata")
			datasets = append(datasets, models.DatasetInfo{
				ID:        ds.DatasetID,
				ProjectID: ds.ProjectID,
			})
			continue
		}
		datasets = append(datasets, models.DatasetInfo{
			ID:          ds.DatasetID,
			ProjectID:   ds.ProjectID,
			Location:    meta.Location,
			Description: meta.Description,
		})
	}
	return datasets, nil
}

// GetDataset returns details for a specific dataset
func (s *BigQueryService) GetDataset(ctx context.Context, datasetID string) (*models.DatasetInfo, error) {
	ds := s.client.Dataset(datasetID)
	meta, err := ds.Metadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dataset %q: %w", datasetID, err)
	}
	return &models.DatasetInfo{
		ID:          datasetID,
		ProjectID:   s.projectID,
		Location:    meta.Location,
		Description: meta.Description,
	}, nil
}

// ListTables returns tables in a dataset
func (s *BigQueryService) ListTables(ctx context.Context, datasetID string) ([]models.TableInfo, error) {
	var tables []models.TableInfo
	it := s.client.Dataset(datasetID).Tables(ctx)
	for {
		tbl, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}
		meta, err := tbl.Metadata(ctx)
		if err != nil {
			log.Warn().Err(err).Str("table", tbl.TableID).Msg("failed to get table metadata")
			tables = append(tables, models.TableInfo{
				ID:        tbl.TableID,
				DatasetID: datasetID,
			})
			continue
		}
		tables = append(tables, models.TableInfo{
			ID:        tbl.TableID,
			DatasetID: datasetID,
			Type:      string(meta.Type),
			NumRows:   meta.NumRows,
			NumBytes:  meta.NumBytes,
		})
	}
	return tables, nil
}

// GetTableSchema returns schema for a specific table
func (s *BigQueryService) GetTableSchema(ctx context.Context, datasetID, tableID string) (bigquery.Schema, *bigquery.TableMetadata, error) {
	meta, err := s.client.Dataset(datasetID).Table(tableID).Metadata(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get table %q.%q: %w", datasetID, tableID, err)
	}
	return meta.Schema, meta, nil
}

// QueryResult holds the result of a BigQuery execution
type QueryResult struct {
	Data                []map[string]interface{}
	Columns             []string
	JobID               string
	TotalBytesProcessed int64
	BytesBilled         int64
	CacheHit            bool
	ExecutionTimeMs     int64
	TotalRows           int64
}

// ExecuteQuery runs a SQL query and returns results
func (s *BigQueryService) ExecuteQuery(ctx context.Context, sql, projectID string, dryRun bool, timeoutMs int, useCache, useLegacySQL bool) (*QueryResult, error) {
	q := s.client.Query(sql)
	q.DryRun = dryRun
	q.DisableQueryCache = !useCache
	q.UseLegacySQL = useLegacySQL

	if projectID != "" {
		q.DefaultProjectID = projectID
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	qCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	job, err := q.Run(qCtx)
	if err != nil {
		return nil, fmt.Errorf("query run: %w", err)
	}

	status, err := job.Wait(qCtx)
	if err != nil {
		return nil, fmt.Errorf("job wait: %w", err)
	}
	if err := status.Err(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	execMs := time.Since(start).Milliseconds()

	// FIX #17: removed unused qConfig variable
	stats := job.LastStatus().Statistics
	var bytesProcessed, bytesBilled int64
	var cacheHit bool
	if stats != nil {
		bytesProcessed = stats.TotalBytesProcessed
		if qStats, ok := stats.Details.(*bigquery.QueryStatistics); ok {
			bytesBilled = qStats.TotalBytesBilled
			cacheHit = qStats.CacheHit
		}
	}

	if dryRun {
		return &QueryResult{
			JobID:               job.ID(),
			TotalBytesProcessed: bytesProcessed,
			BytesBilled:         bytesBilled,
			ExecutionTimeMs:     execMs,
		}, nil
	}

	it, err := job.Read(qCtx)
	if err != nil {
		return nil, fmt.Errorf("job read: %w", err)
	}

	var rows []map[string]interface{}
	var columns []string
	first := true

	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}

		if first && it.Schema != nil {
			for _, f := range it.Schema {
				columns = append(columns, f.Name)
			}
			first = false
		}

		m := make(map[string]interface{}, len(row))
		for k, v := range row {
			m[k] = v
		}
		rows = append(rows, m)
	}

	return &QueryResult{
		Data:                rows,
		Columns:             columns,
		JobID:               job.ID(),
		TotalBytesProcessed: bytesProcessed,
		BytesBilled:         bytesBilled,
		CacheHit:            cacheHit,
		ExecutionTimeMs:     execMs,
		TotalRows:           int64(len(rows)),
	}, nil
}

// SchemaToString formats a BigQuery schema as a human-readable string for LLM context
func SchemaToString(schema bigquery.Schema) string {
	var sb string
	for _, f := range schema {
		sb += fmt.Sprintf("  %s %s\n", f.Name, f.Type)
	}
	return sb
}

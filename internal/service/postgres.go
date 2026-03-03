package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

// PGTableInfo holds metadata about a PostgreSQL table.
type PGTableInfo struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Type   string `json:"type"` // "BASE TABLE" or "VIEW"
}

// PGColumnInfo describes a single column in a PostgreSQL table.
type PGColumnInfo struct {
	Name       string  `json:"name"`
	DataType   string  `json:"data_type"`
	IsNullable string  `json:"is_nullable"`
	Default    *string `json:"column_default,omitempty"`
}

// PGQueryResult holds the result of a PostgreSQL query.
type PGQueryResult struct {
	Columns  []string                 `json:"columns"`
	Data     []map[string]interface{} `json:"data"`
	RowCount int                      `json:"row_count"`
}

// PGExplainCost holds EXPLAIN cost output.
type PGExplainCost struct {
	TotalCost float64 `json:"total_cost"`
	PlanRows  float64 `json:"plan_rows"`
	RawJSON   string  `json:"raw_json"`
}

// PostgresService manages connections to a single PostgreSQL host.
// Pools are lazily created per database name.
type PostgresService struct {
	host     string
	port     int
	user     string
	password string
	sslMode  string
	maxConns int

	mu    sync.RWMutex
	pools map[string]*sql.DB
}

// NewPostgresService creates a new PostgresService for the given host credentials.
func NewPostgresService(host string, port int, user, password, sslMode string, maxConns int) *PostgresService {
	if sslMode == "" {
		sslMode = "disable"
	}
	if maxConns <= 0 {
		maxConns = 10
	}
	return &PostgresService{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		sslMode:  sslMode,
		maxConns: maxConns,
		pools:    make(map[string]*sql.DB),
	}
}

// GetPool returns a connection pool for the given database, creating one lazily.
func (s *PostgresService) GetPool(dbName string) (*sql.DB, error) {
	s.mu.RLock()
	if db, ok := s.pools[dbName]; ok {
		s.mu.RUnlock()
		return db, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after write lock
	if db, ok := s.pools[dbName]; ok {
		return db, nil
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		s.user, s.password, s.host, s.port, dbName, s.sslMode)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open pg pool for %s: %w", dbName, err)
	}
	db.SetMaxOpenConns(s.maxConns)
	db.SetMaxIdleConns(s.maxConns / 2)

	s.pools[dbName] = db
	log.Info().Str("database", dbName).Str("host", s.host).Msg("pg pool created")
	return db, nil
}

// Close closes all connection pools.
func (s *PostgresService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []string
	for name, db := range s.pools {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	s.pools = make(map[string]*sql.DB)
	if len(errs) > 0 {
		return fmt.Errorf("close pg pools: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ListTables lists tables and views in the given database (public + user schemas).
func (s *PostgresService) ListTables(ctx context.Context, dbName string) ([]PGTableInfo, error) {
	db, err := s.GetPool(dbName)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name, table_type
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []PGTableInfo
	for rows.Next() {
		var t PGTableInfo
		if err := rows.Scan(&t.Schema, &t.Name, &t.Type); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// GetTableSchema returns column metadata for a specific table.
func (s *PostgresService) GetTableSchema(ctx context.Context, dbName, schema, table string) ([]PGColumnInfo, error) {
	db, err := s.GetPool(dbName)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}
	defer rows.Close()

	var cols []PGColumnInfo
	for rows.Next() {
		var c PGColumnInfo
		if err := rows.Scan(&c.Name, &c.DataType, &c.IsNullable, &c.Default); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// GetSampleData returns up to 3 sample rows from a table.
func (s *PostgresService) GetSampleData(ctx context.Context, dbName, schema, table string) (*PGQueryResult, error) {
	db, err := s.GetPool(dbName)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`SELECT * FROM %s.%s LIMIT 3`,
		quoteIdent(schema), quoteIdent(table))

	return s.executeRaw(ctx, db, query)
}

// ExecuteQuery runs a read-only SQL query.
func (s *PostgresService) ExecuteQuery(ctx context.Context, dbName, sqlQuery string, timeoutMs int) (*PGQueryResult, error) {
	db, err := s.GetPool(dbName)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read-only tx: %w", err)
	}
	defer tx.Rollback()

	if timeoutMs > 0 {
		_, err = tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeoutMs))
		if err != nil {
			return nil, fmt.Errorf("set timeout: %w", err)
		}
	}

	rows, err := tx.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// ExplainCost runs EXPLAIN (FORMAT JSON) and returns the cost estimate.
func (s *PostgresService) ExplainCost(ctx context.Context, dbName, sqlQuery string) (*PGExplainCost, error) {
	db, err := s.GetPool(dbName)
	if err != nil {
		return nil, err
	}

	var rawJSON string
	err = db.QueryRowContext(ctx, "EXPLAIN (FORMAT JSON) "+sqlQuery).Scan(&rawJSON)
	if err != nil {
		return nil, fmt.Errorf("explain: %w", err)
	}

	// Parse the JSON array to extract top-level plan cost
	var plans []struct {
		Plan struct {
			TotalCost float64 `json:"Total Cost"`
			PlanRows  float64 `json:"Plan Rows"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &plans); err != nil {
		return &PGExplainCost{RawJSON: rawJSON}, nil // return raw if parsing fails
	}

	cost := &PGExplainCost{RawJSON: rawJSON}
	if len(plans) > 0 {
		cost.TotalCost = plans[0].Plan.TotalCost
		cost.PlanRows = plans[0].Plan.PlanRows
	}
	return cost, nil
}

// PGSchemaToString formats column metadata for LLM context injection.
func PGSchemaToString(cols []PGColumnInfo) string {
	var sb strings.Builder
	for _, c := range cols {
		nullable := ""
		if c.IsNullable == "YES" {
			nullable = " (nullable)"
		}
		sb.WriteString(fmt.Sprintf("  %s %s%s\n", c.Name, c.DataType, nullable))
	}
	return sb.String()
}

// executeRaw runs a query and scans all result rows.
func (s *PostgresService) executeRaw(ctx context.Context, db *sql.DB, query string) (*PGQueryResult, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func scanRows(rows *sql.Rows) (*PGQueryResult, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	var data []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			v := vals[i]
			// Convert []byte to string for JSON compatibility
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[col] = v
		}
		data = append(data, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &PGQueryResult{
		Columns:  cols,
		Data:     data,
		RowCount: len(data),
	}, nil
}

// quoteIdent quotes a PostgreSQL identifier.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

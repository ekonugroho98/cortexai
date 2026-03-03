package tools

import (
	"context"
	"testing"
)

func TestPGListDatabasesTool_Name(t *testing.T) {
	tool := PGListDatabasesTool([]string{"db1", "db2"})
	if tool.Name != "list_postgres_databases" {
		t.Errorf("expected name 'list_postgres_databases', got %q", tool.Name)
	}
}

func TestPGListDatabasesTool_Execute(t *testing.T) {
	dbs := []string{"mydb", "analytics"}
	tool := PGListDatabasesTool(dbs)
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should be JSON array containing both databases
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	for _, db := range dbs {
		if !containsStr(result, db) {
			t.Errorf("result should contain %q: %s", db, result)
		}
	}
}

func TestPGListDatabasesTool_EmptyList(t *testing.T) {
	tool := PGListDatabasesTool(nil)
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "null" {
		t.Errorf("expected 'null' for nil databases, got %q", result)
	}
}

func TestPGGetSchemaTool_Name(t *testing.T) {
	tool := PGGetSchemaTool(nil, "testdb")
	if tool.Name != "get_postgres_schema" {
		t.Errorf("expected name 'get_postgres_schema', got %q", tool.Name)
	}
}

func TestPGGetSchemaTool_RequiresSchema(t *testing.T) {
	tool := PGGetSchemaTool(nil, "testdb")
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"table": "users",
	})
	if err == nil {
		t.Error("expected error for missing schema")
	}
}

func TestPGGetSchemaTool_RequiresTable(t *testing.T) {
	tool := PGGetSchemaTool(nil, "testdb")
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"schema": "public",
	})
	if err == nil {
		t.Error("expected error for missing table")
	}
}

func TestPGSampleDataTool_Name(t *testing.T) {
	tool := PGSampleDataTool(nil, "testdb")
	if tool.Name != "get_postgres_sample_data" {
		t.Errorf("expected name 'get_postgres_sample_data', got %q", tool.Name)
	}
}

func TestPGSampleDataTool_RequiresSchema(t *testing.T) {
	tool := PGSampleDataTool(nil, "testdb")
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"table": "users",
	})
	if err == nil {
		t.Error("expected error for missing schema")
	}
}

func TestPGExecuteQueryTool_Name(t *testing.T) {
	tool := PGExecuteQueryTool(nil, "testdb")
	if tool.Name != "execute_postgres_sql" {
		t.Errorf("expected name 'execute_postgres_sql', got %q", tool.Name)
	}
}

func TestPGExecuteQueryTool_RequiresSQL(t *testing.T) {
	tool := PGExecuteQueryTool(nil, "testdb")
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing sql")
	}
}

func TestPGListTablesTool_Name(t *testing.T) {
	tool := PGListTablesTool(nil, "testdb")
	if tool.Name != "list_postgres_tables" {
		t.Errorf("expected name 'list_postgres_tables', got %q", tool.Name)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

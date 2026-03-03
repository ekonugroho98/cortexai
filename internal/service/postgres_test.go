package service

import "testing"

func TestPGSchemaToString_Basic(t *testing.T) {
	cols := []PGColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: "NO"},
		{Name: "name", DataType: "character varying", IsNullable: "YES"},
		{Name: "created_at", DataType: "timestamp without time zone", IsNullable: "YES"},
	}
	result := PGSchemaToString(cols)
	if result == "" {
		t.Fatal("expected non-empty schema string")
	}
	if !contains(result, "id integer") {
		t.Error("expected 'id integer' in output")
	}
	if !contains(result, "name character varying (nullable)") {
		t.Error("expected 'name character varying (nullable)' in output")
	}
}

func TestPGSchemaToString_Empty(t *testing.T) {
	result := PGSchemaToString(nil)
	if result != "" {
		t.Errorf("expected empty string for nil cols, got %q", result)
	}
}

func TestPGPoolRegistry_RegisterAndGet(t *testing.T) {
	reg := NewPGPoolRegistry()
	svc := NewPostgresService("localhost", 5432, "user", "pass", "disable", 5)
	reg.Register("squad-a", svc)

	got := reg.Get("squad-a")
	if got != svc {
		t.Error("expected registered service, got different instance")
	}

	if reg.Get("squad-b") != nil {
		t.Error("expected nil for unregistered squad")
	}
}

func TestPGPoolRegistry_CloseAll(t *testing.T) {
	reg := NewPGPoolRegistry()
	// No services registered — should not error
	if err := reg.CloseAll(); err != nil {
		t.Errorf("CloseAll on empty registry: %v", err)
	}
}

func TestNewPostgresService_Defaults(t *testing.T) {
	svc := NewPostgresService("localhost", 5432, "user", "pass", "", 0)
	if svc.sslMode != "disable" {
		t.Errorf("expected default sslMode 'disable', got %q", svc.sslMode)
	}
	if svc.maxConns != 10 {
		t.Errorf("expected default maxConns 10, got %d", svc.maxConns)
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"public", `"public"`},
		{"user table", `"user table"`},
		{`my"table`, `"my""table"`},
	}
	for _, tt := range tests {
		got := quoteIdent(tt.input)
		if got != tt.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && containsStr(s, sub)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package agent

import (
	"strings"
	"testing"
)

func TestIsDatabaseAllowed_InList(t *testing.T) {
	if !isDatabaseAllowed("mydb", []string{"mydb", "other"}) {
		t.Error("expected true for mydb in [mydb, other]")
	}
}

func TestIsDatabaseAllowed_NotInList(t *testing.T) {
	if isDatabaseAllowed("secret", []string{"mydb", "other"}) {
		t.Error("expected false for secret not in [mydb, other]")
	}
}

func TestIsDatabaseAllowed_EmptyList(t *testing.T) {
	if isDatabaseAllowed("mydb", []string{}) {
		t.Error("expected false for empty allowed list")
	}
}

func TestIsDatabaseAllowed_SingleMatch(t *testing.T) {
	if !isDatabaseAllowed("only", []string{"only"}) {
		t.Error("expected true for single match")
	}
}

func TestPGSystemPromptStyle_Routing(t *testing.T) {
	tests := []struct {
		style    string
		contains string
	}{
		{"executive", "EXECUTIVE"},
		{"technical", "TECHNICAL"},
		{"support", "SUPPORT"},
	}
	for _, tt := range tests {
		p := PGSystemPromptStyle(tt.style)
		if p == "" {
			t.Errorf("PGSystemPromptStyle(%q) returned empty", tt.style)
		}
		found := false
		for i := 0; i <= len(p)-len(tt.contains); i++ {
			if p[i:i+len(tt.contains)] == tt.contains {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PGSystemPromptStyle(%q) should contain %q", tt.style, tt.contains)
		}
	}
}

func TestPGSystemPromptStyle_DefaultFallback(t *testing.T) {
	for _, style := range []string{"", "unknown", "EXECUTIVE"} {
		s := PGSystemPromptStyle(style)
		if s != PGBaseSystemPrompt {
			t.Errorf("PGSystemPromptStyle(%q): expected PGBaseSystemPrompt fallback", style)
		}
	}
}

func TestPGSystemPromptStyle_ContainsPostgreSQL(t *testing.T) {
	for _, style := range []string{"executive", "technical", "support", ""} {
		p := PGSystemPromptStyle(style)
		found := false
		target := "PostgreSQL"
		for i := 0; i <= len(p)-len(target); i++ {
			if p[i:i+len(target)] == target {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PGSystemPromptStyle(%q) should reference PostgreSQL", style)
		}
	}
}

// ── schema section closing instruction ───────────────────────────────────────

func TestPGSchemaSectionClosingInstruction_IsDirective(t *testing.T) {
	if !strings.Contains(PGSchemaClosingInstruction, "IMPORTANT: All table schemas are already provided above") {
		t.Errorf("closing instruction must start with IMPORTANT directive, got: %q", PGSchemaClosingInstruction)
	}
	if !strings.Contains(PGSchemaClosingInstruction, "DO NOT call list_postgres_tables or get_postgres_schema") {
		t.Errorf("closing instruction must forbid schema tool calls, got: %q", PGSchemaClosingInstruction)
	}
	if !strings.Contains(PGSchemaClosingInstruction, "at most 1 execute call") {
		t.Errorf("closing instruction must cap execute calls, got: %q", PGSchemaClosingInstruction)
	}
	if strings.Contains(PGSchemaClosingInstruction, "you can skip") {
		t.Errorf("closing instruction must not use soft 'you can skip' language, got: %q", PGSchemaClosingInstruction)
	}
}

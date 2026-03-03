package handler

import (
	"strings"
	"testing"

	"github.com/cortexai/cortexai/internal/config"
)

// checkDataSourceAllowed is tested via a zero-value AgentHandler receiver since
// the method only uses the PersonaConfig argument (no handler fields involved).

func TestCheckDataSourceAllowed_NilAllowedSources(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{} // AllowedDataSources is nil
	if err := h.checkDataSourceAllowed(pc, "bigquery"); err != nil {
		t.Errorf("nil AllowedDataSources: expected nil error, got %v", err)
	}
	if err := h.checkDataSourceAllowed(pc, "elasticsearch"); err != nil {
		t.Errorf("nil AllowedDataSources: expected nil error for ES, got %v", err)
	}
}

func TestCheckDataSourceAllowed_EmptyAllowedSources(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{}}
	if err := h.checkDataSourceAllowed(pc, "bigquery"); err != nil {
		t.Errorf("empty AllowedDataSources: expected nil error, got %v", err)
	}
}

func TestCheckDataSourceAllowed_SourceAllowed(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery"}}
	if err := h.checkDataSourceAllowed(pc, "bigquery"); err != nil {
		t.Errorf("bigquery is in AllowedDataSources: expected nil error, got %v", err)
	}
}

func TestCheckDataSourceAllowed_SourceBlocked(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery"}}
	err := h.checkDataSourceAllowed(pc, "elasticsearch")
	if err == nil {
		t.Fatal("elasticsearch not in AllowedDataSources: expected error, got nil")
	}
	errMsg := err.Error()
	if !contains(errMsg, "elasticsearch") {
		t.Errorf("error message should contain blocked source 'elasticsearch': %q", errMsg)
	}
	if !contains(errMsg, "bigquery") {
		t.Errorf("error message should contain available source 'bigquery': %q", errMsg)
	}
}

func TestCheckDataSourceAllowed_MultipleAllowed(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery", "elasticsearch"}}
	if err := h.checkDataSourceAllowed(pc, "bigquery"); err != nil {
		t.Errorf("bigquery allowed: expected nil, got %v", err)
	}
	if err := h.checkDataSourceAllowed(pc, "elasticsearch"); err != nil {
		t.Errorf("elasticsearch allowed: expected nil, got %v", err)
	}
}

func TestCheckDataSourceAllowed_PostgresAllowed(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery", "postgres"}}
	if err := h.checkDataSourceAllowed(pc, "postgres"); err != nil {
		t.Errorf("postgres allowed: expected nil, got %v", err)
	}
}

func TestCheckDataSourceAllowed_PostgresBlocked(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery"}}
	err := h.checkDataSourceAllowed(pc, "postgres")
	if err == nil {
		t.Fatal("postgres not in AllowedDataSources: expected error, got nil")
	}
	if !contains(err.Error(), "postgres") {
		t.Errorf("error should mention postgres: %q", err.Error())
	}
}

func TestCheckDataSourceAllowed_ErrorMessageFormat(t *testing.T) {
	h := &AgentHandler{}
	pc := config.PersonaConfig{AllowedDataSources: []string{"bigquery"}}
	err := h.checkDataSourceAllowed(pc, "elasticsearch")
	if err == nil {
		t.Fatal("expected error")
	}
	// REQ-008: message must be actionable — include blocked source AND available list
	msg := err.Error()
	if !contains(msg, "not available") && !contains(msg, "not permitted") && !contains(msg, "is not available") {
		t.Errorf("error message should indicate unavailability: %q", msg)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

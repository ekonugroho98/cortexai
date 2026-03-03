package agent

import (
	"context"
	"testing"

	"github.com/cortexai/cortexai/internal/tools"
)

func TestDeepSeekAgent_Model(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		expectedModel string
	}{
		{"explicit model", "deepseek-chat", "deepseek-chat"},
		{"default model when empty", "", "deepseek-chat"},
		{"custom model", "deepseek-reasoner", "deepseek-reasoner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewDeepSeekAgent("sk-test-key", tt.model, "")
			if got := a.Model(); got != tt.expectedModel {
				t.Errorf("Model() = %q, want %q", got, tt.expectedModel)
			}
		})
	}
}

func TestDeepSeekAgent_DefaultBaseURL(t *testing.T) {
	a := NewDeepSeekAgent("sk-test-key", "", "")
	if a.baseURL != "https://api.deepseek.com/v1" {
		t.Errorf("baseURL = %q, want %q", a.baseURL, "https://api.deepseek.com/v1")
	}
}

func TestDeepSeekAgent_BaseURLTrailingSlashStripped(t *testing.T) {
	a := NewDeepSeekAgent("sk-test-key", "", "https://custom.api.example.com/v1/")
	if a.baseURL != "https://custom.api.example.com/v1" {
		t.Errorf("baseURL = %q, want %q", a.baseURL, "https://custom.api.example.com/v1")
	}
}

func TestDeepSeekAgent_ImplementsLLMRunner(t *testing.T) {
	// Compile-time check that DeepSeekAgent satisfies LLMRunner
	var _ LLMRunner = (*DeepSeekAgent)(nil)
}

func TestDeepSeekAgent_BuildMessages_SystemAndUser(t *testing.T) {
	t.Run("with system prompt", func(t *testing.T) {
		msgs := buildInitialMessages("you are an assistant", "hello")
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		if msgs[0].Role != "system" {
			t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "system")
		}
		if msgs[0].Content != "you are an assistant" {
			t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "you are an assistant")
		}
		if msgs[1].Role != "user" {
			t.Errorf("msgs[1].Role = %q, want %q", msgs[1].Role, "user")
		}
		if msgs[1].Content != "hello" {
			t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "hello")
		}
	})

	t.Run("without system prompt", func(t *testing.T) {
		msgs := buildInitialMessages("", "query data")
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Role != "user" {
			t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "user")
		}
		if msgs[0].Content != "query data" {
			t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "query data")
		}
	})
}

func TestDeepSeekAgent_BuildTools(t *testing.T) {
	agentTools := []tools.Tool{
		{
			Name:        "execute_bigquery_sql",
			Description: "Execute a BigQuery SQL query",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to execute",
					},
				},
				"required": []string{"sql"},
			},
			Execute: func(_ context.Context, _ map[string]interface{}) (string, error) {
				return "", nil
			},
		},
		{
			Name:        "list_tables",
			Description: "List available BigQuery tables",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{},
			},
			Execute: func(_ context.Context, _ map[string]interface{}) (string, error) {
				return "", nil
			},
		},
	}

	dsTools := convertToOpenAITools(agentTools)

	if len(dsTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(dsTools))
	}

	// Verify first tool
	if dsTools[0].Type != "function" {
		t.Errorf("dsTools[0].Type = %q, want %q", dsTools[0].Type, "function")
	}
	if dsTools[0].Function.Name != "execute_bigquery_sql" {
		t.Errorf("dsTools[0].Function.Name = %q, want %q", dsTools[0].Function.Name, "execute_bigquery_sql")
	}
	if dsTools[0].Function.Description != "Execute a BigQuery SQL query" {
		t.Errorf("dsTools[0].Function.Description = %q", dsTools[0].Function.Description)
	}

	// Verify parameters schema contains "type": "object"
	params, ok := dsTools[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Parameters is not map[string]interface{}")
	}
	if params["type"] != "object" {
		t.Errorf("params[type] = %v, want %q", params["type"], "object")
	}

	// Verify properties were passed through
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("params[properties] is not map[string]interface{}")
	}
	if _, hasSql := props["sql"]; !hasSql {
		t.Error("expected 'sql' in properties")
	}

	// Verify required was passed through
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("params[required] is not []string")
	}
	if len(required) != 1 || required[0] != "sql" {
		t.Errorf("required = %v, want [sql]", required)
	}

	// Verify second tool
	if dsTools[1].Function.Name != "list_tables" {
		t.Errorf("dsTools[1].Function.Name = %q, want %q", dsTools[1].Function.Name, "list_tables")
	}
}

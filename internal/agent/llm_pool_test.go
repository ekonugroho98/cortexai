package agent

import (
	"context"
	"testing"

	"github.com/cortexai/cortexai/internal/tools"
)

// mockRunner is a minimal LLMRunner for testing the pool.
type mockRunner struct{ model string }

func (m *mockRunner) Run(_ context.Context, _, _ string, _ []tools.Tool) (string, []string, string, error) {
	return "", nil, "", nil
}
func (m *mockRunner) RunWithEmit(_ context.Context, _, _ string, _ []tools.Tool, _ EmitFn) (string, []string, string, error) {
	return "", nil, "", nil
}
func (m *mockRunner) Model() string { return m.model }

// Ensure mockRunner satisfies LLMRunner at compile time.
var _ LLMRunner = (*mockRunner)(nil)

func TestLLMPool_GetRegisteredRunner(t *testing.T) {
	pool := NewLLMPool()
	var r LLMRunner = &mockRunner{model: "gpt-4"}
	pool.Register(PoolKey("openai", "gpt-4"), r)

	got := pool.Get(PoolKey("openai", "gpt-4"))
	if got != r {
		t.Fatalf("expected registered runner, got %v", got)
	}
}

func TestLLMPool_GetFallbackWhenKeyMissing(t *testing.T) {
	pool := NewLLMPool()
	var fallback LLMRunner = &mockRunner{model: "fallback"}
	pool.SetFallback(fallback)

	got := pool.Get("nonexistent:key")
	if got != fallback {
		t.Fatalf("expected fallback runner, got %v", got)
	}
}

func TestLLMPool_GetNilWhenNoFallback(t *testing.T) {
	pool := NewLLMPool()
	got := pool.Get("nonexistent:key")
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestLLMPool_RegisterOverwritesExistingKey(t *testing.T) {
	pool := NewLLMPool()
	var r1 LLMRunner = &mockRunner{model: "v1"}
	var r2 LLMRunner = &mockRunner{model: "v2"}
	key := PoolKey("anthropic", "claude")

	pool.Register(key, r1)
	pool.Register(key, r2)

	got := pool.Get(key)
	if got != r2 {
		t.Fatalf("expected r2 after overwrite, got %v", got)
	}
}

func TestLLMPool_HasRunners_Empty(t *testing.T) {
	pool := NewLLMPool()
	if pool.HasRunners() {
		t.Fatal("expected HasRunners()=false for empty pool")
	}
}

func TestLLMPool_HasRunners_WithRegistered(t *testing.T) {
	pool := NewLLMPool()
	pool.Register("k", &mockRunner{})
	if !pool.HasRunners() {
		t.Fatal("expected HasRunners()=true after Register")
	}
}

func TestLLMPool_HasRunners_WithFallbackOnly(t *testing.T) {
	pool := NewLLMPool()
	pool.SetFallback(&mockRunner{})
	if !pool.HasRunners() {
		t.Fatal("expected HasRunners()=true with fallback set")
	}
}

func TestPoolKey_Format(t *testing.T) {
	key := PoolKey("anthropic", "claude-sonnet-4-6")
	if key != "anthropic:claude-sonnet-4-6" {
		t.Fatalf("unexpected PoolKey format: %q", key)
	}
}

func TestLLMPool_FallbackReturnsNilWithNoRegisteredAndNoFallback(t *testing.T) {
	pool := NewLLMPool()
	if pool.Get(PoolKey("any", "model")) != nil {
		t.Fatal("empty pool with no fallback should return nil")
	}
}

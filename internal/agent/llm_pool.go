package agent

import "fmt"

// LLMPool manages multiple LLMRunner instances keyed by "provider:model".
// Multiple personas can share the same LLMRunner if they use identical provider+model.
// The pool is written once at startup and read-only during request handling,
// so no mutex is required.
type LLMPool struct {
	runners  map[string]LLMRunner
	fallback LLMRunner // used when a key is not found
}

// NewLLMPool creates an empty pool.
func NewLLMPool() *LLMPool {
	return &LLMPool{
		runners: make(map[string]LLMRunner),
	}
}

// Register adds a runner to the pool under the given key.
// If the key is already registered, the existing runner is replaced.
func (p *LLMPool) Register(key string, runner LLMRunner) {
	p.runners[key] = runner
}

// SetFallback sets the default runner returned when Get() cannot find a key.
func (p *LLMPool) SetFallback(runner LLMRunner) {
	p.fallback = runner
}

// Get returns the runner for the given key.
// If the key is not registered, it returns the fallback runner.
// Returns nil only when the key is not found AND no fallback is set.
func (p *LLMPool) Get(key string) LLMRunner {
	if r, ok := p.runners[key]; ok {
		return r
	}
	return p.fallback
}

// HasRunners returns true if at least one runner is registered or a fallback is set.
func (p *LLMPool) HasRunners() bool {
	return len(p.runners) > 0 || p.fallback != nil
}

// PoolKey generates the canonical lookup key for a provider+model combination.
func PoolKey(provider, model string) string {
	return fmt.Sprintf("%s:%s", provider, model)
}

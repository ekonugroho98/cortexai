// Package tools defines the Tool interface and shared types used by both
// the agent and individual tool implementations.
package tools

import "context"

// Tool represents a callable function the LLM can invoke
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Execute     func(ctx context.Context, input map[string]interface{}) (string, error)
}

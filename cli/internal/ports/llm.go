package ports

import "context"

// LLMClient is the driven port for model calls.
// Implementations may target Claude, Codex, or any provider.
type LLMClient interface {
	Complete(ctx context.Context, prompt string, opts CompletionOptions) (string, error)
}

type CompletionOptions struct {
	Model       string
	MaxTokens   int
	Temperature float64
}

package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

// AgentWorkerGeneratorOptions configures a Generator backed by an AgentWorker.
type AgentWorkerGeneratorOptions struct {
	Worker        agentworker.AgentWorker
	WorkerKind    agentworker.WorkerKind
	Provider      agentworker.Provider
	Model         string
	CWD           string
	JobID         string
	AttemptID     string
	RequestID     string
	Timeout       time.Duration
	ContextBudget int
	Digest        string
	Metadata      map[string]string
}

// AgentWorkerGenerator adapts the AgentWorker runtime contract to the narrow
// Generator interface consumed by forge summarization/review code.
type AgentWorkerGenerator struct {
	opts AgentWorkerGeneratorOptions
}

// NewAgentWorkerGenerator returns a forge-compatible Generator backed by the
// shared AgentWorker runtime.
func NewAgentWorkerGenerator(opts AgentWorkerGeneratorOptions) (*AgentWorkerGenerator, error) {
	if opts.Worker == nil {
		return nil, fmt.Errorf("agent worker is required")
	}
	if opts.WorkerKind == "" {
		return nil, fmt.Errorf("worker kind is required")
	}
	if opts.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if opts.Model == "" {
		opts.Model = string(opts.WorkerKind)
	}
	if opts.ContextBudget <= 0 {
		opts.ContextBudget = DefaultContextBudget
	}
	return &AgentWorkerGenerator{opts: opts}, nil
}

// Generate starts an AgentWorker session for one prompt and returns its durable
// transcript text after the session reaches a successful terminal state.
func (g *AgentWorkerGenerator) Generate(prompt string) (string, error) {
	ctx := context.Background()
	if g.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.opts.Timeout)
		defer cancel()
	}

	session, err := g.opts.Worker.Start(ctx, agentworker.StartRequest{
		WorkerKind: g.opts.WorkerKind,
		Provider:   g.opts.Provider,
		JobID:      g.opts.JobID,
		AttemptID:  g.opts.AttemptID,
		RequestID:  g.opts.RequestID,
		Model:      g.opts.Model,
		CWD:        g.opts.CWD,
		Prompt:     prompt,
		Metadata:   g.opts.Metadata,
	})
	if err != nil {
		return "", fmt.Errorf("agent worker start: %w", err)
	}

	state, err := waitForAgentWorkerTerminal(ctx, session)
	if err != nil {
		return "", err
	}
	if !state.Successful() {
		return "", fmt.Errorf("agent worker terminal state %s: %s", state.Status, state.Reason)
	}

	transcript, err := session.Transcript(ctx)
	if err != nil {
		return "", fmt.Errorf("agent worker transcript: %w", err)
	}
	text := strings.TrimSpace(transcript.Text)
	if text == "" {
		return "", fmt.Errorf("agent worker transcript is empty")
	}
	return text, nil
}

func waitForAgentWorkerTerminal(ctx context.Context, session agentworker.AgentSession) (agentworker.TerminalState, error) {
	events, err := session.Stream(ctx, agentworker.StreamOptions{})
	if err != nil {
		return agentworker.TerminalState{}, fmt.Errorf("agent worker stream: %w", err)
	}

	var last agentworker.TerminalState
	for events != nil {
		select {
		case <-ctx.Done():
			return agentworker.TerminalState{}, ctx.Err()
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if event.State.Status != "" {
				last = event.State
			}
			if event.Type == agentworker.EventTerminal || event.State.Terminal() {
				return event.State, nil
			}
		}
	}

	if last.Terminal() {
		return last, nil
	}
	state, err := session.TerminalState(ctx)
	if err != nil {
		return agentworker.TerminalState{}, fmt.Errorf("agent worker terminal state: %w", err)
	}
	return state, nil
}

// Digest returns the provider/model digest when known.
func (g *AgentWorkerGenerator) Digest() string {
	if g.opts.Digest != "" {
		return g.opts.Digest
	}
	return string(g.opts.Provider) + ":" + g.opts.Model
}

// ContextBudget returns the prompt context budget advertised by this worker.
func (g *AgentWorkerGenerator) ContextBudget() int {
	return g.opts.ContextBudget
}

// ModelName returns the model label used for forge metadata.
func (g *AgentWorkerGenerator) ModelName() string {
	return g.opts.Model
}

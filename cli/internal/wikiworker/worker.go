package wikiworker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

// ExtractionRequest describes one worker-backed wiki extraction.
type ExtractionRequest struct {
	Prompt    string
	JobID     string
	AttemptID string
	RequestID string
	Worker    agentworker.WorkerKind
	Provider  agentworker.Provider
	Model     string
	CWD       string
	Metadata  map[string]string
}

// ExtractionResult is a validated wiki extraction plus runtime evidence.
type ExtractionResult struct {
	Session    agentworker.SessionRef
	Terminal   agentworker.TerminalState
	Envelope   agentworker.OutputEnvelope
	Extraction Extraction
	Artifacts  []agentworker.Artifact
}

// Worker runs wiki extraction through the AgentWorker runtime.
type Worker struct {
	agent agentworker.AgentWorker
}

// RetryOptions configures retry and quarantine behavior for invalid worker
// output.
type RetryOptions struct {
	MaxAttempts   int
	QuarantineDir string
	Writer        agentworker.QuarantineWriter
}

// ExtractionValidationError carries invalid worker output evidence.
type ExtractionValidationError struct {
	Err       error
	Session   agentworker.SessionRef
	Terminal  agentworker.TerminalState
	RawOutput string
}

func (e *ExtractionValidationError) Error() string {
	return e.Err.Error()
}

func (e *ExtractionValidationError) Unwrap() error {
	return e.Err
}

// QuarantineError reports that invalid worker output was quarantined after
// retries were exhausted.
type QuarantineError struct {
	Path string
	Err  error
}

func (e *QuarantineError) Error() string {
	return fmt.Sprintf("worker output quarantined at %s: %v", e.Path, e.Err)
}

func (e *QuarantineError) Unwrap() error {
	return e.Err
}

// NewWorker constructs a wiki extraction worker.
func NewWorker(agent agentworker.AgentWorker) (*Worker, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent worker is required")
	}
	return &Worker{agent: agent}, nil
}

// RunExtraction starts a worker session, waits for terminal success, and
// validates both the worker envelope and wiki extraction payload.
func (w *Worker) RunExtraction(ctx context.Context, req ExtractionRequest) (ExtractionResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ExtractionResult{}, fmt.Errorf("prompt is required")
	}
	if req.Worker == "" {
		req.Worker = agentworker.WorkerKindCodex
	}
	if req.Provider == "" {
		req.Provider = agentworker.ProviderGasCity
	}
	session, err := w.agent.Start(ctx, agentworker.StartRequest{
		WorkerKind: req.Worker,
		Provider:   req.Provider,
		JobID:      req.JobID,
		AttemptID:  req.AttemptID,
		RequestID:  req.RequestID,
		Model:      req.Model,
		CWD:        req.CWD,
		Prompt:     req.Prompt,
		Metadata:   req.Metadata,
	})
	if err != nil {
		return ExtractionResult{}, err
	}

	terminal, err := waitForSessionTerminal(ctx, session)
	if err != nil {
		return ExtractionResult{}, err
	}
	if !terminal.Successful() {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, fmt.Errorf("worker terminal state %s: %s", terminal.Status, terminal.Reason)
	}

	transcript, err := session.Transcript(ctx)
	if err != nil {
		return ExtractionResult{}, err
	}
	envelope, err := agentworker.ParseOutputEnvelope([]byte(transcript.Text))
	if err != nil {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, &ExtractionValidationError{
			Err:       err,
			Session:   session.Ref(),
			Terminal:  terminal,
			RawOutput: transcript.Text,
		}
	}
	payload := envelope.Payload
	if len(payload) == 0 {
		payload = []byte(envelope.Text)
	}
	extraction, err := ParseExtraction(payload)
	if err != nil {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal, Envelope: envelope}, &ExtractionValidationError{
			Err:       err,
			Session:   session.Ref(),
			Terminal:  terminal,
			RawOutput: transcript.Text,
		}
	}
	artifacts, err := session.Artifacts(ctx)
	if err != nil {
		return ExtractionResult{}, err
	}
	return ExtractionResult{
		Session:    session.Ref(),
		Terminal:   terminal,
		Envelope:   envelope,
		Extraction: extraction,
		Artifacts:  append(envelope.Artifacts, artifacts...),
	}, nil
}

// RunExtractionWithRetry retries invalid worker output to a cap, then writes a
// quarantine record with the raw output and durable job/session refs.
func (w *Worker) RunExtractionWithRetry(ctx context.Context, req ExtractionRequest, opts RetryOptions) (ExtractionResult, error) {
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastResult ExtractionResult
	var lastValidation *ExtractionValidationError
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := w.RunExtraction(ctx, req)
		if err == nil {
			return result, nil
		}
		lastResult = result
		var validationErr *ExtractionValidationError
		if !errors.As(err, &validationErr) {
			return result, err
		}
		lastValidation = validationErr
	}
	if lastValidation == nil {
		return lastResult, fmt.Errorf("worker extraction failed without validation error")
	}
	writer := opts.Writer
	if writer.Dir == "" {
		writer.Dir = opts.QuarantineDir
	}
	if writer.Dir == "" {
		writer.Dir = filepath.Join(".agents", "quarantine", "agentworker")
	}
	path, err := writer.Write(agentworker.QuarantineRecord{
		Kind:      "wiki_extraction",
		Reason:    "invalid_worker_output",
		Error:     lastValidation.Err.Error(),
		JobID:     req.JobID,
		AttemptID: req.AttemptID,
		RequestID: req.RequestID,
		Session:   lastValidation.Session,
		Terminal:  lastValidation.Terminal,
		Attempts:  maxAttempts,
		RawOutput: lastValidation.RawOutput,
	})
	if err != nil {
		return lastResult, err
	}
	return lastResult, &QuarantineError{Path: path, Err: lastValidation.Err}
}

func waitForSessionTerminal(ctx context.Context, session agentworker.AgentSession) (agentworker.TerminalState, error) {
	events, err := session.Stream(ctx, agentworker.StreamOptions{})
	if err != nil {
		return agentworker.TerminalState{}, err
	}
	for events != nil {
		select {
		case <-ctx.Done():
			return agentworker.TerminalState{}, ctx.Err()
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if event.Type == agentworker.EventTerminal || event.State.Terminal() {
				return event.State, nil
			}
		}
	}
	return session.TerminalState(ctx)
}

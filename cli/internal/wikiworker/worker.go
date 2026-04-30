package wikiworker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

var (
	activeTranscriptPollInterval  = 250 * time.Millisecond
	activeTranscriptCloseIdle     = 3 * time.Minute
	activeTranscriptValidationLag = 2 * time.Minute
)

type closeableSession interface {
	Close(context.Context) error
}

type activeTranscriptState struct {
	terminal             agentworker.TerminalState
	unstructuredSince    time.Time
	lastActivity         time.Time
	sawTranscriptOutput  bool
	lastTranscriptOutput string
}

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

// RunExtraction starts a worker session and validates both the worker envelope
// and wiki extraction payload once either terminal success or usable transcript
// output is available.
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

	return waitForSessionExtraction(ctx, session)
}

type transcriptValidationResult struct {
	result           ExtractionResult
	err              error
	ok               bool
	hasOutput        bool
	transcriptOutput bool
	assistantOutput  bool
	output           string
}

func validateSessionTranscript(ctx context.Context, session agentworker.AgentSession, terminal agentworker.TerminalState, requireStructured bool) transcriptValidationResult {
	transcript, err := session.Transcript(ctx)
	if err != nil {
		return transcriptValidationResult{err: err, ok: true}
	}
	output, assistantOutput := transcriptWorkerOutput(transcript)
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return transcriptValidationResult{}
	}
	if requireStructured && !assistantOutput && !looksLikeStructuredWorkerOutput(trimmedOutput) {
		return transcriptValidationResult{transcriptOutput: true, output: trimmedOutput}
	}
	if requireStructured && !looksLikeStructuredWorkerOutput(trimmedOutput) {
		return transcriptValidationResult{hasOutput: true, transcriptOutput: true, assistantOutput: assistantOutput, output: trimmedOutput}
	}
	rawOutput := transcript.Text
	if strings.TrimSpace(rawOutput) == "" {
		rawOutput = output
	}
	envelope, err := agentworker.ParseOutputEnvelope([]byte(output))
	if err != nil {
		return transcriptValidationResult{
			result: ExtractionResult{Session: session.Ref(), Terminal: terminal},
			err: &ExtractionValidationError{
				Err:       err,
				Session:   session.Ref(),
				Terminal:  terminal,
				RawOutput: rawOutput,
			},
			ok:               true,
			hasOutput:        true,
			transcriptOutput: true,
			assistantOutput:  assistantOutput,
			output:           trimmedOutput,
		}
	}
	payload := envelope.Payload
	if len(payload) == 0 {
		payload = []byte(envelope.Text)
	}
	extraction, err := ParseExtraction(payload)
	if err != nil {
		return transcriptValidationResult{
			result: ExtractionResult{Session: session.Ref(), Terminal: terminal, Envelope: envelope},
			err: &ExtractionValidationError{
				Err:       err,
				Session:   session.Ref(),
				Terminal:  terminal,
				RawOutput: rawOutput,
			},
			ok:               true,
			hasOutput:        true,
			transcriptOutput: true,
			assistantOutput:  assistantOutput,
			output:           trimmedOutput,
		}
	}
	artifacts, err := session.Artifacts(ctx)
	if err != nil {
		return transcriptValidationResult{err: err, ok: true, hasOutput: true, transcriptOutput: true, assistantOutput: assistantOutput, output: trimmedOutput}
	}
	return transcriptValidationResult{result: ExtractionResult{
		Session:    session.Ref(),
		Terminal:   terminal,
		Envelope:   envelope,
		Extraction: extraction,
		Artifacts:  append(envelope.Artifacts, artifacts...),
	}, ok: true, hasOutput: true, transcriptOutput: true, assistantOutput: assistantOutput, output: trimmedOutput}
}

func transcriptWorkerOutput(transcript agentworker.Transcript) (string, bool) {
	sawAssistant := false
	for i := len(transcript.Messages) - 1; i >= 0; i-- {
		message := transcript.Messages[i]
		if !strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
			continue
		}
		sawAssistant = true
		content := strings.TrimSpace(message.Content)
		if content == "" || isTranscriptPlaceholderOutput(content) {
			continue
		}
		return message.Content, true
	}
	if sawAssistant {
		return "", false
	}
	return transcript.Text, false
}

func isTranscriptPlaceholderOutput(output string) bool {
	switch strings.TrimSpace(output) {
	case "[thinking]", "[exec_command]":
		return true
	default:
		return false
	}
}

func looksLikeStructuredWorkerOutput(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "{")
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

func waitForSessionExtraction(ctx context.Context, session agentworker.AgentSession) (ExtractionResult, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, err := session.Stream(streamCtx, agentworker.StreamOptions{})
	if err != nil {
		return ExtractionResult{}, err
	}
	ticker := time.NewTicker(activeTranscriptPollInterval)
	defer ticker.Stop()

	state := activeTranscriptState{
		terminal:     agentworker.TerminalState{Status: session.Ref().Status},
		lastActivity: time.Now(),
	}

	for events != nil {
		select {
		case <-ctx.Done():
			return ExtractionResult{}, ctx.Err()
		case <-ticker.C:
			if result, err, ok := pollActiveSession(ctx, session, &state); ok {
				return result, err
			}
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if result, err, ok := handleSessionEvent(ctx, session, &state, event); ok {
				return result, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return ExtractionResult{}, err
	}
	return finishStreamedSession(ctx, session, state.terminal)
}

func pollActiveSession(ctx context.Context, session agentworker.AgentSession, state *activeTranscriptState) (ExtractionResult, error, bool) {
	if result, err, ok := validateActiveSessionTranscript(ctx, session, state); ok {
		return result, err, true
	}
	if result, err, ok := closeIdleSession(ctx, session, state); ok {
		return result, err, true
	}
	return ExtractionResult{}, nil, false
}

func handleSessionEvent(ctx context.Context, session agentworker.AgentSession, state *activeTranscriptState, event agentworker.Event) (ExtractionResult, error, bool) {
	state.lastActivity = time.Now()
	if event.State.Status != "" {
		state.terminal = event.State
	}
	if event.Type == agentworker.EventTerminal || event.State.Terminal() {
		result, err := validateTerminalEvent(ctx, session, event.State)
		return result, err, true
	}
	state.unstructuredSince = time.Time{}
	return validateActiveSessionTranscript(ctx, session, state)
}

func validateActiveSessionTranscript(ctx context.Context, session agentworker.AgentSession, state *activeTranscriptState) (ExtractionResult, error, bool) {
	validation := validateSessionTranscript(ctx, session, state.terminal, true)
	if validation.transcriptOutput {
		state.sawTranscriptOutput = true
		if validation.output != "" && validation.output != state.lastTranscriptOutput {
			state.lastTranscriptOutput = validation.output
			state.lastActivity = time.Now()
		}
	}
	if validation.ok {
		result, err := finalizeActiveExtraction(ctx, session, validation.result, validation.err)
		return result, err, true
	}
	if !validation.hasOutput {
		state.unstructuredSince = time.Time{}
		return ExtractionResult{}, nil, false
	}
	if state.unstructuredSince.IsZero() {
		state.unstructuredSince = time.Now()
		return ExtractionResult{}, nil, false
	}
	if time.Since(state.unstructuredSince) < activeTranscriptValidationLag {
		return ExtractionResult{}, nil, false
	}
	validation = validateSessionTranscript(ctx, session, state.terminal, false)
	if validation.ok {
		result, err := finalizeActiveExtraction(ctx, session, validation.result, validation.err)
		return result, err, true
	}
	return ExtractionResult{}, nil, false
}

func closeIdleSession(ctx context.Context, session agentworker.AgentSession, state *activeTranscriptState) (ExtractionResult, error, bool) {
	if !state.sawTranscriptOutput || time.Since(state.lastActivity) < activeTranscriptCloseIdle {
		return ExtractionResult{}, nil, false
	}
	closer, ok := session.(closeableSession)
	if !ok {
		return ExtractionResult{}, nil, false
	}
	if err := closer.Close(ctx); err != nil {
		return ExtractionResult{}, err, true
	}
	terminal, err := session.TerminalState(ctx)
	if err != nil {
		return ExtractionResult{}, err, true
	}
	if terminal.Terminal() && !terminal.Successful() {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, fmt.Errorf("worker terminal state %s: %s", terminal.Status, terminal.Reason), true
	}
	result, err := validateTerminalSessionTranscript(ctx, session, terminal)
	return result, err, true
}

func validateTerminalEvent(ctx context.Context, session agentworker.AgentSession, terminal agentworker.TerminalState) (ExtractionResult, error) {
	if !terminal.Successful() {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, fmt.Errorf("worker terminal state %s: %s", terminal.Status, terminal.Reason)
	}
	return validateTerminalSessionTranscript(ctx, session, terminal)
}

func finishStreamedSession(ctx context.Context, session agentworker.AgentSession, streamTerminal agentworker.TerminalState) (ExtractionResult, error) {
	terminal, err := session.TerminalState(ctx)
	if err != nil {
		return ExtractionResult{}, err
	}
	if terminal.Status == "" {
		terminal = streamTerminal
	}
	if terminal.Terminal() && !terminal.Successful() {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, fmt.Errorf("worker terminal state %s: %s", terminal.Status, terminal.Reason)
	}
	validation := validateSessionTranscript(ctx, session, terminal, false)
	if validation.ok {
		return validation.result, validation.err
	}
	if !terminal.Successful() {
		return ExtractionResult{Session: session.Ref(), Terminal: terminal}, fmt.Errorf("worker terminal state %s: %s", terminal.Status, terminal.Reason)
	}
	return validateTerminalSessionTranscript(ctx, session, terminal)
}

func finalizeActiveExtraction(ctx context.Context, session agentworker.AgentSession, result ExtractionResult, validationErr error) (ExtractionResult, error) {
	if validationErr != nil || result.Terminal.Terminal() {
		return result, validationErr
	}
	closer, ok := session.(closeableSession)
	if !ok {
		return result, nil
	}
	if err := closer.Close(ctx); err != nil {
		return ExtractionResult{}, err
	}
	terminal, err := session.TerminalState(ctx)
	if err != nil {
		return ExtractionResult{}, err
	}
	if !terminal.Terminal() {
		terminal = agentworker.TerminalState{Status: agentworker.StatusCompleted, Reason: "structured output accepted and provider close requested"}
	}
	result.Session = session.Ref()
	result.Session.Status = agentworker.StatusCompleted
	result.Terminal = terminal
	return result, nil
}

func validateTerminalSessionTranscript(ctx context.Context, session agentworker.AgentSession, terminal agentworker.TerminalState) (ExtractionResult, error) {
	validation := validateSessionTranscript(ctx, session, terminal, false)
	if validation.ok {
		return validation.result, validation.err
	}
	rawOutput, err := sessionRawTranscript(ctx, session)
	if err != nil {
		return ExtractionResult{}, err
	}
	return ExtractionResult{Session: session.Ref(), Terminal: terminal}, &ExtractionValidationError{
		Err:       agentworker.ErrEmptyOutput,
		Session:   session.Ref(),
		Terminal:  terminal,
		RawOutput: rawOutput,
	}
}

func sessionRawTranscript(ctx context.Context, session agentworker.AgentSession) (string, error) {
	transcript, err := session.Transcript(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(transcript.Text) != "" {
		return transcript.Text, nil
	}
	var b strings.Builder
	for _, message := range transcript.Messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" && content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if role != "" {
			b.WriteString(strings.ToUpper(role))
			b.WriteString(":\n")
		}
		b.WriteString(content)
	}
	if b.Len() == 0 {
		return "empty worker transcript", nil
	}
	return b.String(), nil
}

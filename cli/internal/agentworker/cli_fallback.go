package agentworker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultCLIFallbackWaitDelay = 2 * time.Second

type CLIFallbackWorkerOptions struct {
	Command           string
	Args              []string
	Env               []string
	WallClockTimeout  time.Duration
	MemoryMaxBytes    int64
	CgroupRoot        string
	CgroupNamePrefix  string
	DisableCgroupCaps bool
	Now               func() time.Time
}

type CLIFallbackWorker struct {
	command           string
	args              []string
	env               []string
	wallClockTimeout  time.Duration
	memoryMaxBytes    int64
	cgroupRoot        string
	cgroupNamePrefix  string
	disableCgroupCaps bool
	now               func() time.Time
	mu                sync.Mutex
	sessions          map[string]*CLIFallbackSession
}

type CLIFallbackSession struct {
	ref          SessionRef
	events       []Event
	transcript   Transcript
	terminal     TerminalState
	cgroupStatus CgroupStatus
}

func NewCLIFallbackWorker(opts CLIFallbackWorkerOptions) (*CLIFallbackWorker, error) {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	prefix := strings.TrimSpace(opts.CgroupNamePrefix)
	if prefix == "" {
		prefix = "agentops-worker"
	}
	return &CLIFallbackWorker{
		command:           strings.TrimSpace(opts.Command),
		args:              append([]string{}, opts.Args...),
		env:               append([]string{}, opts.Env...),
		wallClockTimeout:  opts.WallClockTimeout,
		memoryMaxBytes:    opts.MemoryMaxBytes,
		cgroupRoot:        opts.CgroupRoot,
		cgroupNamePrefix:  prefix,
		disableCgroupCaps: opts.DisableCgroupCaps,
		now:               now,
		sessions:          map[string]*CLIFallbackSession{},
	}, nil
}

func (w *CLIFallbackWorker) Start(ctx context.Context, req StartRequest) (AgentSession, error) {
	if req.Provider == "" {
		req.Provider = ProviderCLIFallback
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.Provider != ProviderCLIFallback {
		return nil, fmt.Errorf("cli fallback worker requires provider %q", ProviderCLIFallback)
	}
	command, args := w.commandFor(req)
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("cli fallback command is required")
	}
	runCtx := ctx
	cancel := func() {}
	if runCtx == nil {
		runCtx = context.Background()
	}
	if w.wallClockTimeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, w.wallClockTimeout)
	}
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, command, args...)
	cmd.Dir = strings.TrimSpace(req.CWD)
	cmd.Env = append(os.Environ(), w.env...)
	cmd.Stdin = strings.NewReader(req.Prompt)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = defaultCLIFallbackWaitDelay
	configureIsolatedProcess(cmd)
	cmd.Cancel = func() error {
		return killIsolatedProcess(cmd)
	}

	startedAt := w.now().UTC()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cli fallback worker: %w", err)
	}
	sessionID := fmt.Sprintf("cli-%d", cmd.Process.Pid)
	ref := SessionRef{
		WorkerKind: req.WorkerKind,
		Provider:   ProviderCLIFallback,
		JobID:      req.JobID,
		AttemptID:  req.AttemptID,
		RequestID:  req.RequestID,
		SessionID:  sessionID,
		Status:     StatusRunning,
	}
	cgroupStatus := CgroupStatus{Reason: "cgroup caps disabled"}
	if !w.disableCgroupCaps {
		cgroupStatus = applyCgroupV2Limits(cmd.Process.Pid, CgroupLimits{
			Root:           w.cgroupRoot,
			Name:           w.cgroupName(req, cmd.Process.Pid),
			MemoryMaxBytes: w.memoryMaxBytes,
		})
	}
	waitErr := cmd.Wait()
	completedAt := w.now().UTC()
	terminal := classifyCLIFallbackTerminal(runCtx, waitErr)
	ref.Status = terminal.Status
	transcript := Transcript{
		Text: stdout.String(),
		Messages: []TranscriptMessage{
			{Role: "user", Content: req.Prompt, At: startedAt},
			{Role: "assistant", Content: stdout.String(), At: completedAt},
		},
	}
	if stderr.Len() > 0 {
		transcript.Messages = append(transcript.Messages, TranscriptMessage{Role: "stderr", Content: stderr.String(), At: completedAt})
	}
	events := []Event{
		{Cursor: "1", At: startedAt, Type: EventStarted, Message: filepath.Base(command), State: TerminalState{Status: StatusRunning}},
		{Cursor: "2", At: completedAt, Type: EventTerminal, Message: terminal.Reason, State: terminal},
	}
	session := &CLIFallbackSession{
		ref:          ref,
		events:       events,
		transcript:   transcript,
		terminal:     terminal,
		cgroupStatus: cgroupStatus,
	}
	w.mu.Lock()
	w.sessions[sessionID] = session
	w.mu.Unlock()
	return session, nil
}

func (w *CLIFallbackWorker) Attach(_ context.Context, ref SessionRef) (AgentSession, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	session, ok := w.sessions[ref.SessionID]
	if !ok {
		return nil, fmt.Errorf("cli fallback session not found: %s", ref.SessionID)
	}
	return session, nil
}

func (w *CLIFallbackWorker) commandFor(req StartRequest) (string, []string) {
	if w.command != "" {
		return w.command, append([]string{}, w.args...)
	}
	return string(req.WorkerKind), nil
}

func (w *CLIFallbackWorker) cgroupName(req StartRequest, pid int) string {
	seed := firstNonEmpty(req.JobID, req.RequestID, fmt.Sprintf("%d", pid))
	return w.cgroupNamePrefix + "-" + sanitizeCgroupName(seed)
}

func classifyCLIFallbackTerminal(ctx context.Context, err error) TerminalState {
	if err == nil {
		return TerminalState{Status: StatusCompleted}
	}
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return TerminalState{Status: StatusFailed, FailureCode: string(StatusFailed), Reason: "worker exceeded wall-clock limit"}
	}
	if ctx != nil && ctx.Err() == context.Canceled {
		return TerminalState{Status: StatusCancelled, FailureCode: string(StatusCancelled), Reason: "worker context cancelled"}
	}
	return TerminalState{Status: StatusFailed, FailureCode: string(StatusFailed), Reason: err.Error()}
}

func (s *CLIFallbackSession) Ref() SessionRef {
	return s.ref
}

func (s *CLIFallbackSession) Nudge(_ context.Context, _ NudgeRequest) error {
	return fmt.Errorf("cli fallback sessions are one-shot and cannot be nudged")
}

func (s *CLIFallbackSession) Cancel(_ context.Context, req CancelRequest) error {
	s.terminal = TerminalState{Status: StatusCancelled, FailureCode: string(StatusCancelled), Reason: req.Reason}
	s.ref.Status = StatusCancelled
	return nil
}

func (s *CLIFallbackSession) Stream(_ context.Context, _ StreamOptions) (<-chan Event, error) {
	ch := make(chan Event, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s *CLIFallbackSession) Transcript(_ context.Context) (Transcript, error) {
	return s.transcript, nil
}

func (s *CLIFallbackSession) Artifacts(_ context.Context) ([]Artifact, error) {
	return nil, nil
}

func (s *CLIFallbackSession) TerminalState(_ context.Context) (TerminalState, error) {
	return s.terminal, nil
}

func (s *CLIFallbackSession) CgroupStatus() CgroupStatus {
	return s.cgroupStatus
}

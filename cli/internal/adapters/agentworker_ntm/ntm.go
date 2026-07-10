package agentworker_ntm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Worker struct {
	runner Runner
}

func New(runner Runner) *Worker { return &Worker{runner: runner} }

func (w *Worker) Start(ctx context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	if req.Provider == "" {
		req.Provider = agentworker.ProviderNTM
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.Provider != agentworker.ProviderNTM {
		return nil, fmt.Errorf("ntm worker requires provider %q", agentworker.ProviderNTM)
	}
	artifactPaths, err := requestedArtifactPaths(req)
	if err != nil {
		return nil, err
	}
	if _, err := w.runner.Run(ctx, "ntm", "--robot-help"); err != nil {
		return nil, fmt.Errorf("discover ntm robot capabilities: %w", err)
	}
	sessionID := sessionName(req)
	spawnArgs := []string{"--robot-spawn=" + sessionID, workerFlag(req.WorkerKind), "--spawn-no-user", "--spawn-wait"}
	if req.CWD != "" {
		spawnArgs = append(spawnArgs, "--spawn-dir="+req.CWD)
	}
	if _, err := w.runJSON(ctx, spawnArgs...); err != nil {
		return nil, fmt.Errorf("spawn ntm session: %w", err)
	}
	if _, err := w.runJSON(ctx, "--robot-send="+sessionID, "--msg="+req.Prompt, "--track"); err != nil {
		return nil, fmt.Errorf("deliver ntm work: %w", err)
	}
	return &session{worker: w, ref: agentworker.SessionRef{
		WorkerKind: req.WorkerKind, Provider: agentworker.ProviderNTM,
		JobID: req.JobID, AttemptID: req.AttemptID, RequestID: req.RequestID,
		SessionID: sessionID, Status: agentworker.StatusRunning,
	}, artifactPaths: artifactPaths}, nil
}

func requestedArtifactPaths(req agentworker.StartRequest) ([]string, error) {
	requested := strings.TrimSpace(req.Metadata["review_evidence_path"])
	if requested == "" {
		return nil, nil
	}
	if !filepath.IsAbs(requested) {
		if strings.TrimSpace(req.CWD) == "" {
			return nil, fmt.Errorf("relative review_evidence_path requires cwd")
		}
		requested = filepath.Join(req.CWD, requested)
	}
	requested = filepath.Clean(requested)
	if strings.TrimSpace(req.CWD) != "" {
		root, err := filepath.Abs(req.CWD)
		if err != nil {
			return nil, fmt.Errorf("resolve artifact cwd: %w", err)
		}
		path, err := filepath.Abs(requested)
		if err != nil {
			return nil, fmt.Errorf("resolve review evidence path: %w", err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("review_evidence_path escapes cwd")
		}
		requested = path
	}
	return []string{requested}, nil
}

func (w *Worker) Attach(ctx context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	if ref.Provider != agentworker.ProviderNTM {
		return nil, fmt.Errorf("cannot attach non-NTM provider %q", ref.Provider)
	}
	if _, err := w.runner.Run(ctx, "ntm", "--robot-help"); err != nil {
		return nil, fmt.Errorf("discover ntm robot capabilities: %w", err)
	}
	return &session{worker: w, ref: ref}, nil
}

func (w *Worker) runJSON(ctx context.Context, args ...string) ([]byte, error) {
	raw, err := w.runner.Run(ctx, "ntm", args...)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode ntm robot response: %w", err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("ntm robot command failed: %s", envelope.Error)
	}
	return raw, nil
}

func workerFlag(kind agentworker.WorkerKind) string {
	switch kind {
	case agentworker.WorkerKindClaude:
		return "--spawn-cc=1"
	default:
		return "--spawn-cod=1"
	}
}

func sessionName(req agentworker.StartRequest) string {
	parts := []string{req.JobID, req.AttemptID}
	if req.JobID == "" {
		parts[0] = req.RequestID
	}
	if parts[0] == "" {
		parts[0] = "agentops"
	}
	if parts[1] == "" {
		parts = parts[:1]
	}
	name := strings.Join(parts, "-")
	replacer := strings.NewReplacer("/", "-", " ", "-", ":", "-")
	return replacer.Replace(name)
}

type session struct {
	worker        *Worker
	ref           agentworker.SessionRef
	artifactPaths []string
}

func (s *session) Ref() agentworker.SessionRef { return s.ref }

func (s *session) Nudge(ctx context.Context, req agentworker.NudgeRequest) error {
	if strings.TrimSpace(req.Message) == "" {
		return fmt.Errorf("nudge message is required")
	}
	_, err := s.worker.runJSON(ctx, "--robot-send="+s.ref.SessionID, "--msg="+req.Message, "--track")
	return err
}

func (s *session) Cancel(ctx context.Context, _ agentworker.CancelRequest) error {
	_, err := s.worker.runJSON(ctx, "--robot-interrupt="+s.ref.SessionID)
	return err
}

func (s *session) Stream(ctx context.Context, _ agentworker.StreamOptions) (<-chan agentworker.Event, error) {
	ch := make(chan agentworker.Event, 2)
	state, err := s.TerminalState(ctx)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- agentworker.Event{Cursor: "1", At: time.Now().UTC(), Type: agentworker.EventOutput, State: state}
	if state.Terminal() {
		ch <- agentworker.Event{Cursor: "2", At: time.Now().UTC(), Type: agentworker.EventTerminal, State: state}
	}
	close(ch)
	return ch, nil
}

func (s *session) Transcript(ctx context.Context) (agentworker.Transcript, error) {
	raw, err := s.worker.runJSON(ctx, "--robot-tail="+s.ref.SessionID)
	if err != nil {
		return agentworker.Transcript{}, err
	}
	var response struct {
		Panes []struct {
			Index   int    `json:"index"`
			Content string `json:"content"`
		} `json:"panes"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return agentworker.Transcript{}, err
	}
	parts := make([]string, 0, len(response.Panes))
	for _, pane := range response.Panes {
		parts = append(parts, pane.Content)
	}
	return agentworker.Transcript{Text: strings.Join(parts, "\n"), SourcePath: "ntm://" + s.ref.SessionID}, nil
}

func (s *session) Artifacts(ctx context.Context) ([]agentworker.Artifact, error) {
	artifacts := make([]agentworker.Artifact, 0, len(s.artifactPaths))
	for _, path := range s.artifactPaths {
		if err := s.ensureReviewEvidence(ctx, path); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, agentworker.Artifact{
			Kind: "review-evidence", Path: path, MIME: "text/plain",
			JobID: s.ref.JobID, AttemptID: s.ref.AttemptID, SessionID: s.ref.SessionID,
			ValidationStatus: "valid",
		})
	}
	return artifacts, nil
}

func (s *session) ensureReviewEvidence(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("review evidence must be a nonempty regular file: %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect review evidence: %w", err)
	}

	transcript, err := s.Transcript(ctx)
	if err != nil {
		return fmt.Errorf("capture review evidence transcript: %w", err)
	}
	if strings.TrimSpace(transcript.Text) == "" {
		return fmt.Errorf("review evidence transcript is empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create review evidence: %w", err)
	}
	if _, err := file.WriteString(transcript.Text + "\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("write review evidence: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close review evidence: %w", err)
	}
	return nil
}

func (s *session) TerminalState(ctx context.Context) (agentworker.TerminalState, error) {
	raw, err := s.worker.runJSON(ctx, "--robot-snapshot")
	if err != nil {
		return agentworker.TerminalState{Status: agentworker.StatusProviderUnreachable, Reason: err.Error()}, nil
	}
	var response struct {
		LatestCursor any `json:"latest_cursor"`
		Sessions     []struct {
			Name  string `json:"name"`
			Panes []struct {
				State string `json:"state"`
			} `json:"panes"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return agentworker.TerminalState{}, err
	}
	for _, candidate := range response.Sessions {
		if candidate.Name != s.ref.SessionID {
			continue
		}
		if len(candidate.Panes) == 0 {
			return agentworker.TerminalState{Status: agentworker.StatusStarting}, nil
		}
		state := agentworker.ClassifyTerminalState(candidate.Panes[0].State)
		s.ref.Status = state.Status
		s.ref.EventCursor = fmt.Sprint(response.LatestCursor)
		return state, nil
	}
	return agentworker.TerminalState{Status: agentworker.StatusLost, FailureCode: string(agentworker.StatusLost), Reason: "NTM session not found after acceptance"}, nil
}

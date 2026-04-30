package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

type daemonFakeOpenClawSnapshotExecutor struct {
	Delay     time.Duration
	Err       error
	Artifacts map[string]string
}

func (e daemonFakeOpenClawSnapshotExecutor) JobTypes() []daemonpkg.JobType {
	return []daemonpkg.JobType{daemonpkg.JobTypeOpenClawSnapshot}
}

func (e daemonFakeOpenClawSnapshotExecutor) RunJob(ctx context.Context, claim daemonpkg.QueueClaim) (daemonpkg.JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != daemonpkg.JobTypeOpenClawSnapshot {
		return daemonpkg.JobExecutionResult{}, fmt.Errorf("fake executor does not support job type %s", claim.Job.JobType)
	}
	if e.Delay > 0 {
		timer := time.NewTimer(e.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return daemonpkg.JobExecutionResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	artifacts := map[string]string{
		"executor_policy": "fake",
		"snapshot_status": "validated",
	}
	for key, value := range e.Artifacts {
		artifacts[key] = value
	}
	return daemonpkg.JobExecutionResult{Artifacts: artifacts}, e.Err
}

func newDaemonFakeWikiAgentWorker() agentworker.AgentWorker {
	return daemonFakeWikiAgentWorker{}
}

type daemonFakeWikiAgentWorker struct{}

func (daemonFakeWikiAgentWorker) Start(_ context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &daemonFakeWikiAgentSession{req: req}, nil
}

func (daemonFakeWikiAgentWorker) Attach(_ context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return &daemonFakeWikiAgentSession{ref: ref}, nil
}

type daemonFakeWikiAgentSession struct {
	req agentworker.StartRequest
	ref agentworker.SessionRef
}

func (s *daemonFakeWikiAgentSession) Ref() agentworker.SessionRef {
	if s.ref.SessionID != "" {
		return s.ref
	}
	return agentworker.SessionRef{
		WorkerKind:        s.req.WorkerKind,
		Provider:          agentworker.Provider("fake"),
		JobID:             s.req.JobID,
		AttemptID:         s.req.AttemptID,
		RequestID:         s.req.RequestID,
		ProviderRequestID: "fake-wiki-request-" + sanitizeDaemonFakeArtifactName(s.req.JobID),
		SessionID:         "fake-wiki-session-" + sanitizeDaemonFakeArtifactName(s.req.JobID),
		EventCursor:       "fake-cursor-terminal",
		Status:            agentworker.StatusCompleted,
	}
}

func (s *daemonFakeWikiAgentSession) Nudge(context.Context, agentworker.NudgeRequest) error {
	return nil
}

func (s *daemonFakeWikiAgentSession) Cancel(context.Context, agentworker.CancelRequest) error {
	return nil
}

func (s *daemonFakeWikiAgentSession) Stream(context.Context, agentworker.StreamOptions) (<-chan agentworker.Event, error) {
	ch := make(chan agentworker.Event, 1)
	ch <- agentworker.Event{
		Cursor: "fake-cursor-terminal",
		At:     time.Now().UTC(),
		Type:   agentworker.EventTerminal,
		State:  agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	close(ch)
	return ch, nil
}

func (s *daemonFakeWikiAgentSession) Transcript(context.Context) (agentworker.Transcript, error) {
	payload := map[string]any{
		"schema_version": 1,
		"title":          "Fake daemon wiki extraction",
		"summary":        "The fake AgentWorker produced deterministic wiki extraction output.",
		"entities":       []string{"AgentOps", "AgentWorker"},
		"concepts":       []string{"daemon wiki executor"},
		"decisions":      []string{"Use AgentWorker for daemon wiki jobs"},
		"open_questions": []string{},
		"work_phase":     "implement",
	}
	envelope := map[string]any{
		"schema_version": 1,
		"session":        s.Ref(),
		"status":         string(agentworker.StatusCompleted),
		"payload":        payload,
		"artifacts": []map[string]string{{
			"kind":              "wiki-note",
			"path":              ".agents/wiki/sources/fake-daemon-wiki.md",
			"validation_status": "valid",
		}},
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return agentworker.Transcript{}, err
	}
	return agentworker.Transcript{Text: string(data)}, nil
}

func (s *daemonFakeWikiAgentSession) Artifacts(context.Context) ([]agentworker.Artifact, error) {
	return []agentworker.Artifact{{
		Kind:             "wiki-note",
		Path:             ".agents/wiki/sources/fake-daemon-wiki.md",
		SessionID:        s.Ref().SessionID,
		ValidationStatus: "valid",
	}}, nil
}

func (s *daemonFakeWikiAgentSession) TerminalState(context.Context) (agentworker.TerminalState, error) {
	return agentworker.TerminalState{Status: agentworker.StatusCompleted}, nil
}

func sanitizeDaemonFakeArtifactName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "job"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", ".", "-")
	return replacer.Replace(value)
}

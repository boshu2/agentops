package reviewlane_worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func writeBoundInput(t *testing.T, dir, name, contents string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(contents))
	return path, fmt.Sprintf("%x", sum)
}

func TestNTMReviewLaneFreshReadOnlyNonce(t *testing.T) {
	evidenceDir := t.TempDir()
	contract, contractSHA := writeBoundInput(t, evidenceDir, "contract.txt", "Given x\n")
	diff, diffSHA := writeBoundInput(t, evidenceDir, "diff.patch", "+ change\n")
	evidencePath := filepath.Join(evidenceDir, "reviewer-1.txt")
	if err := os.WriteFile(evidencePath, []byte("reviewed files: 3\nx.go:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	worker := newFakeWorker(agentworker.ProviderNTM, agentworker.StatusCompleted,
		"VERDICT: CONFIRMED\nNONCE: n1\nCONTEXT: reviewer-1\nREAD_ONLY: true",
		[]agentworker.Artifact{{Kind: "review-evidence", Path: evidencePath, ValidationStatus: "valid"}})
	worker.statuses = []agentworker.SessionStatus{agentworker.StatusRunning, agentworker.StatusCompleted}
	lane := New(worker, agentworker.WorkerKindCodex, agentworker.ProviderNTM)
	var _ ports.ReviewLanePort = lane
	result, err := lane.Run(context.Background(), ports.ReviewRequestV1{
		SchemaVersion: "review-request.v1", SubjectID: "age-1", HeadSHA: "deadbeef",
		AcceptanceContract: contract, AcceptanceContractSHA256: contractSHA, DiffPath: diff, DiffSHA256: diffSHA, AuthorContextID: "author-1",
		AuthorFamily: "claude", DiversityMode: "fresh-context", Nonce: "n1",
		EvidenceDir: evidenceDir, ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Disposition != ports.ReviewConfirmed || result.ContextID != "reviewer-1" || result.Nonce != "n1" || !result.ReadOnly {
		t.Fatalf("unexpected result: %#v", result)
	}
	if worker.lastSession == nil || worker.lastSession.terminalCalls != 2 {
		t.Fatalf("review lane must poll through running state, calls=%v", worker.lastSession)
	}
}

func TestReviewLaneTransportFailureIsNotRefutation(t *testing.T) {
	dir := t.TempDir()
	contract, contractSHA := writeBoundInput(t, dir, "contract.txt", "Given x\n")
	diff, diffSHA := writeBoundInput(t, dir, "diff.patch", "+ change\n")
	worker := newFakeWorker(agentworker.ProviderNTM, agentworker.StatusProviderUnreachable, "", nil)
	lane := New(worker, agentworker.WorkerKindCodex, agentworker.ProviderNTM)
	result, err := lane.Run(context.Background(), ports.ReviewRequestV1{
		SchemaVersion: "review-request.v1", SubjectID: "age-1", HeadSHA: "deadbeef",
		AcceptanceContract: contract, AcceptanceContractSHA256: contractSHA, DiffPath: diff, DiffSHA256: diffSHA, AuthorContextID: "author-1",
		AuthorFamily: "claude", DiversityMode: "fresh-context", Nonce: "n1",
		EvidenceDir: "/tmp/evidence", ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("Run transport result: %v", err)
	}
	if result.FailureClass != ports.ReviewFailureTransport || result.Disposition != "" {
		t.Fatalf("transport failure became semantic disposition: %#v", result)
	}
}

func TestReviewLaneBoundsNonTerminalWorkerAsTransportFailure(t *testing.T) {
	dir := t.TempDir()
	contract, contractSHA := writeBoundInput(t, dir, "contract.txt", "Given x\n")
	diff, diffSHA := writeBoundInput(t, dir, "diff.patch", "+ change\n")
	worker := newFakeWorker(agentworker.ProviderNTM, agentworker.StatusRunning, "", nil)
	lane := New(worker, agentworker.WorkerKindCodex, agentworker.ProviderNTM)
	lane.pollInterval = time.Millisecond
	lane.waitTimeout = 5 * time.Millisecond

	result, err := lane.Run(context.Background(), ports.ReviewRequestV1{
		SchemaVersion: "review-request.v1", SubjectID: "age-timeout", HeadSHA: "deadbeef",
		AcceptanceContract: contract, AcceptanceContractSHA256: contractSHA,
		DiffPath: diff, DiffSHA256: diffSHA, AuthorContextID: "author-1",
		AuthorFamily: "claude", DiversityMode: "fresh-context", Nonce: "n-timeout",
		EvidenceDir: dir, ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("Run timeout result: %v", err)
	}
	if result.FailureClass != ports.ReviewFailureTransport || result.Disposition != "" || !strings.Contains(result.FailureReason, "deadline exceeded") {
		t.Fatalf("nonterminal timeout must remain transport-only: %#v", result)
	}
}

type fakeWorker struct {
	provider    agentworker.Provider
	status      agentworker.SessionStatus
	statuses    []agentworker.SessionStatus
	transcript  string
	artifacts   []agentworker.Artifact
	lastSession *fakeSession
}

func newFakeWorker(provider agentworker.Provider, status agentworker.SessionStatus, transcript string, artifacts []agentworker.Artifact) *fakeWorker {
	return &fakeWorker{provider: provider, status: status, transcript: transcript, artifacts: artifacts}
}

func (f *fakeWorker) Start(_ context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	f.lastSession = &fakeSession{ref: agentworker.SessionRef{WorkerKind: req.WorkerKind, Provider: f.provider, SessionID: "ntm-review-1", Status: agentworker.StatusRunning}, status: f.status, statuses: f.statuses, transcript: f.transcript, artifacts: f.artifacts}
	return f.lastSession, nil
}
func (f *fakeWorker) Attach(_ context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	return &fakeSession{ref: ref, status: f.status, statuses: f.statuses, transcript: f.transcript, artifacts: f.artifacts}, nil
}

type fakeSession struct {
	ref           agentworker.SessionRef
	status        agentworker.SessionStatus
	statuses      []agentworker.SessionStatus
	terminalCalls int
	transcript    string
	artifacts     []agentworker.Artifact
}

func (f *fakeSession) Ref() agentworker.SessionRef                             { return f.ref }
func (f *fakeSession) Nudge(context.Context, agentworker.NudgeRequest) error   { return nil }
func (f *fakeSession) Cancel(context.Context, agentworker.CancelRequest) error { return nil }
func (f *fakeSession) Stream(context.Context, agentworker.StreamOptions) (<-chan agentworker.Event, error) {
	ch := make(chan agentworker.Event, 1)
	ch <- agentworker.Event{Cursor: "1", At: time.Now(), Type: agentworker.EventTerminal, State: agentworker.TerminalState{Status: f.status}}
	close(ch)
	return ch, nil
}
func (f *fakeSession) Transcript(context.Context) (agentworker.Transcript, error) {
	return agentworker.Transcript{Text: f.transcript}, nil
}
func (f *fakeSession) Artifacts(context.Context) ([]agentworker.Artifact, error) {
	return f.artifacts, nil
}
func (f *fakeSession) TerminalState(context.Context) (agentworker.TerminalState, error) {
	status := f.status
	if len(f.statuses) > 0 {
		index := f.terminalCalls
		if index >= len(f.statuses) {
			index = len(f.statuses) - 1
		}
		status = f.statuses[index]
	}
	f.terminalCalls++
	return agentworker.TerminalState{Status: status}, nil
}

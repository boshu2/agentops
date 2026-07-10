package agentworker_ntm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

type fakeRunner struct {
	calls [][]string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "--robot-help"):
		return []byte(`{"success":true,"commands":["spawn","send","tail","snapshot","interrupt"]}`), nil
	case strings.Contains(joined, "--robot-spawn="):
		return json.Marshal(map[string]any{"success": true, "session": "job-1-attempt-1", "agents": []any{map[string]any{"pane": 1, "type": "codex"}}})
	case strings.Contains(joined, "--robot-send="):
		return []byte(`{"success":true,"session":"job-1-attempt-1","successful":["1"],"failed":[]}`), nil
	case strings.Contains(joined, "--robot-tail="):
		return []byte(`{"success":true,"session":"job-1-attempt-1","panes":[{"index":1,"content":"work complete\nVERDICT: CONFIRMED\nNONCE: n1"}]}`), nil
	case strings.Contains(joined, "--robot-snapshot"):
		return []byte(`{"success":true,"latest_cursor":42,"sessions":[{"name":"job-1-attempt-1","panes":[{"index":1,"state":"completed"}]}]}`), nil
	case strings.Contains(joined, "--robot-interrupt="):
		return []byte(`{"success":true}`), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s %s", name, joined)
	}
}

func TestNTMWorkerLifecycle(t *testing.T) {
	runner := &fakeRunner{}
	worker := New(runner)
	var _ agentworker.AgentWorker = worker
	evidenceDir := t.TempDir()
	evidencePath := filepath.Join(evidenceDir, "review-evidence.txt")

	session, err := worker.Start(context.Background(), agentworker.StartRequest{
		WorkerKind: agentworker.WorkerKindCodex,
		Provider:   agentworker.ProviderNTM,
		JobID:      "job-1",
		AttemptID:  "attempt-1",
		RequestID:  "req-1",
		CWD:        evidenceDir,
		Prompt:     "run $rpi age-123",
		Metadata:   map[string]string{"review_evidence_path": evidencePath},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := session.Ref(); got.Provider != agentworker.ProviderNTM || got.SessionID != "job-1-attempt-1" || got.Status != agentworker.StatusRunning {
		t.Fatalf("unexpected ref: %#v", got)
	}
	if err := session.Nudge(context.Background(), agentworker.NudgeRequest{Message: "show evidence"}); err != nil {
		t.Fatalf("Nudge: %v", err)
	}
	transcript, err := session.Transcript(context.Background())
	if err != nil || !strings.Contains(transcript.Text, "VERDICT: CONFIRMED") {
		t.Fatalf("Transcript: %#v err=%v", transcript, err)
	}
	artifacts, err := session.Artifacts(context.Background())
	if err != nil || len(artifacts) != 1 || artifacts[0].Kind != "review-evidence" || artifacts[0].Path != evidencePath || artifacts[0].ValidationStatus != "valid" {
		t.Fatalf("Artifacts: %#v err=%v", artifacts, err)
	}
	evidence, err := os.ReadFile(evidencePath)
	if err != nil || !strings.Contains(string(evidence), "VERDICT: CONFIRMED") {
		t.Fatalf("durable review evidence: %q err=%v", evidence, err)
	}
	state, err := session.TerminalState(context.Background())
	if err != nil || state.Status != agentworker.StatusCompleted {
		t.Fatalf("TerminalState: %#v err=%v", state, err)
	}
	if err := session.Cancel(context.Background(), agentworker.CancelRequest{Reason: "cleanup"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	joined := make([]string, 0, len(runner.calls))
	for _, call := range runner.calls {
		joined = append(joined, strings.Join(call, " "))
	}
	all := strings.Join(joined, "\n")
	for _, want := range []string{"--robot-help", "--robot-spawn=job-1-attempt-1", "--spawn-cod=1", "--spawn-no-user", "--spawn-wait", "--spawn-dir=" + evidenceDir, "--robot-send=job-1-attempt-1", "--robot-tail=job-1-attempt-1", "--robot-snapshot", "--robot-interrupt=job-1-attempt-1"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing live NTM contract token %q in calls:\n%s", want, all)
		}
	}
	if len(joined) == 0 || !strings.Contains(joined[0], "--robot-help") {
		t.Fatalf("live NTM capability discovery must precede state changes: %#v", runner.calls)
	}
}

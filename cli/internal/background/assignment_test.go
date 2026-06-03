package background

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeAssignmentTransport struct {
	calls   []string
	reserve AssignmentReservationEvidence
	send    AssignmentMessageEvidence
	err     error
}

func (f *fakeAssignmentTransport) Name() string { return "fake-mail" }

func (f *fakeAssignmentTransport) ReserveFiles(_ context.Context, req AssignmentRequest) (AssignmentReservationEvidence, error) {
	f.calls = append(f.calls, "reserve:"+strings.Join(req.Files, ","))
	if f.err != nil {
		return AssignmentReservationEvidence{}, f.err
	}
	return f.reserve, nil
}

func (f *fakeAssignmentTransport) SendMessage(_ context.Context, req AssignmentRequest) (AssignmentMessageEvidence, error) {
	f.calls = append(f.calls, "send:"+strings.Join(req.To, ","))
	if f.err != nil {
		return AssignmentMessageEvidence{}, f.err
	}
	return f.send, nil
}

func TestAssignBackgroundAgent_DryRunReturnsCopyPasteEvidence(t *testing.T) {
	req := AssignmentRequest{
		Bead:       "ag-demo",
		To:         []string{"JadeElk"},
		Branch:     "cursor/ag-demo",
		Files:      []string{"README.md", "docs/3.0.md"},
		Skills:     []string{"implement", "validation"},
		Validation: "go test ./cmd/ao -run Agent",
		Session:    "agentops-bg",
		DryRun:     true,
	}

	got, err := AssignBackgroundAgent(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("AssignBackgroundAgent dry-run: %v", err)
	}
	if got.Transport != "copy-paste" || got.Sent {
		t.Fatalf("dry-run evidence transport/sent = %q/%v, want copy-paste/false", got.Transport, got.Sent)
	}
	if got.Topic != "ag-demo" {
		t.Fatalf("Topic = %q, want ag-demo", got.Topic)
	}
	if got.Reservation.Required || len(got.Reservation.Paths) != 2 {
		t.Fatalf("dry-run reservation evidence = %+v, want paths without required live reservation", got.Reservation)
	}
	if got.CopyPaste == nil {
		t.Fatal("dry-run evidence missing copy-paste fallback")
	}
	for _, want := range []string{"BACKGROUND AGENT ASSIGNMENT", "ag-demo", "JadeElk", "README.md"} {
		if !strings.Contains(got.CopyPaste.Message, want) {
			t.Fatalf("copy-paste message missing %q:\n%s", want, got.CopyPaste.Message)
		}
	}
}

func TestAssignBackgroundAgent_ReservesBeforeSending(t *testing.T) {
	transport := &fakeAssignmentTransport{
		reserve: AssignmentReservationEvidence{Required: true, Granted: true, Paths: []string{"cli/cmd/ao/agent.go"}},
		send:    AssignmentMessageEvidence{MessageID: "msg-1", ThreadID: "ag-demo"},
	}
	req := AssignmentRequest{
		Bead:       "ag-demo",
		To:         []string{"JadeElk"},
		Files:      []string{"cli/cmd/ao/agent.go"},
		Session:    "agentops-bg",
		Validation: "scripts/pre-push-gate.sh --fast",
	}

	got, err := AssignBackgroundAgent(context.Background(), req, transport)
	if err != nil {
		t.Fatalf("AssignBackgroundAgent: %v", err)
	}
	if strings.Join(transport.calls, "|") != "reserve:cli/cmd/ao/agent.go|send:JadeElk" {
		t.Fatalf("call order = %v, want reserve before send", transport.calls)
	}
	if !got.Sent || !got.Reservation.Granted || got.Message == nil || got.Message.MessageID != "msg-1" {
		t.Fatalf("evidence = %+v, want sent with reservation and message id", got)
	}
}

func TestAssignBackgroundAgent_ReservationFailureStopsBeforeSend(t *testing.T) {
	transport := &fakeAssignmentTransport{err: errors.New("reservation conflict")}
	req := AssignmentRequest{
		Bead:    "ag-demo",
		To:      []string{"JadeElk"},
		Files:   []string{"cli/cmd/ao/agent.go"},
		Session: "agentops-bg",
	}

	_, err := AssignBackgroundAgent(context.Background(), req, transport)
	if err == nil || !strings.Contains(err.Error(), "reserve assignment files") {
		t.Fatalf("err = %v, want clear reservation failure", err)
	}
	if strings.Join(transport.calls, "|") != "reserve:cli/cmd/ao/agent.go" {
		t.Fatalf("calls = %v, send should not run after reservation failure", transport.calls)
	}
}

func TestNTMAssignmentTransportBuildsMailAndLockCommands(t *testing.T) {
	runner := &fakeCommandRunner{}
	transport := NewNTMAssignmentTransport(runner, "2h")
	req := AssignmentRequest{
		Bead:       "ag-demo",
		To:         []string{"JadeElk", "JadeBeacon"},
		Files:      []string{"README.md", "docs/3.0.md"},
		Session:    "agentops-bg",
		Validation: "scripts/pre-push-gate.sh --fast",
	}

	if _, err := transport.ReserveFiles(context.Background(), req); err != nil {
		t.Fatalf("ReserveFiles: %v", err)
	}
	if _, err := transport.SendMessage(context.Background(), req); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	joined := strings.Join(runner.calls, "\n")
	for _, want := range []string{
		"ntm lock agentops-bg README.md docs/3.0.md --reason Assignment ag-demo --ttl 2h --json",
		"ntm mail send agentops-bg --to JadeElk --to JadeBeacon --subject Assignment: ag-demo --thread ag-demo --json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("commands missing %q:\n%s", want, joined)
		}
	}
}

type fakeCommandRunner struct {
	calls []string
}

func (f *fakeCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return []byte(`{"ok":true}`), nil
}

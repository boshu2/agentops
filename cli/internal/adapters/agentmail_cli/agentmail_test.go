package agentmail_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type fakeRunner struct {
	calls               [][]string
	reserveSupportsJSON bool
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "capabilities --json"):
		return json.Marshal(map[string]any{
			"schema_version": "am.capabilities.v1",
			"version":        "0.3.10",
			"commands": []map[string]any{{
				"name": "file_reservations reserve", "supports_json_flag": f.reserveSupportsJSON,
			}},
		})
	case strings.HasPrefix(joined, "agents register"):
		return []byte(`{"agent":{"name":"worker-1"},"project":{"human_key":"/repo"}}`), nil
	case strings.HasPrefix(joined, "file_reservations reserve"):
		return []byte(`{"reservation_ids":[7]}`), nil
	case strings.HasPrefix(joined, "mail send"):
		return []byte(`{"message":{"id":42}}`), nil
	case strings.HasPrefix(joined, "mail ack"):
		return []byte(`{"acknowledged":true}`), nil
	case strings.HasPrefix(joined, "file_reservations release"):
		return []byte(`{"released":1}`), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s %s", name, joined)
	}
}

func TestReserveJSONFlagFollowsDiscoveredCapability(t *testing.T) {
	for _, tc := range []struct {
		name     string
		supports bool
		wantFlag bool
	}{
		{name: "current am emits JSON without unsupported flag", supports: false, wantFlag: false},
		{name: "future compatible surface can request JSON", supports: true, wantFlag: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{reserveSupportsJSON: tc.supports}
			mail := New(runner)
			_, err := mail.Reserve(context.Background(), ports.AgentMailReservationRequest{
				Project: "/repo", Agent: "worker-1", Paths: []string{"cli/**"}, TTLSeconds: 60, Exclusive: true,
			})
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			reserveCall := strings.Join(runner.calls[len(runner.calls)-1], " ")
			if got := strings.Contains(reserveCall, "--json"); got != tc.wantFlag {
				t.Fatalf("reserve --json presence=%v want=%v: %s", got, tc.wantFlag, reserveCall)
			}
		})
	}
}

func TestAgentMailCLIIdentityReservationAckLifecycle(t *testing.T) {
	runner := &fakeRunner{}
	mail := New(runner)
	var _ ports.AgentMailPort = mail
	ctx := context.Background()

	if _, err := mail.EnsureIdentity(ctx, ports.AgentMailIdentityRequest{Project: "/repo", Agent: "worker-1", Program: "codex-cli", Model: "gpt-5", Task: "C09"}); err != nil {
		t.Fatalf("EnsureIdentity: %v", err)
	}
	res, err := mail.Reserve(ctx, ports.AgentMailReservationRequest{Project: "/repo", Agent: "worker-1", Paths: []string{"cli/internal/adapters/**"}, TTLSeconds: 900, Exclusive: true, Reason: "C09"})
	if err != nil || len(res.IDs) != 1 {
		t.Fatalf("Reserve: %#v err=%v", res, err)
	}
	msg, err := mail.Send(ctx, ports.AgentMailMessageRequest{Project: "/repo", Sender: "worker-1", Recipients: []string{"orchestrator"}, Subject: "C09 handoff", Body: "evidence", ThreadID: "C09", AckRequired: true})
	if err != nil || msg.ID != 42 {
		t.Fatalf("Send: %#v err=%v", msg, err)
	}
	if err := mail.Acknowledge(ctx, "/repo", "orchestrator", msg.ID); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if err := mail.Release(ctx, "/repo", "worker-1", res.IDs); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if len(runner.calls) == 0 || strings.Join(runner.calls[0], " ") != "am capabilities --json" {
		t.Fatalf("live capability discovery must precede Agent Mail writes: %#v", runner.calls)
	}

	raw, _ := json.Marshal(runner.calls)
	all := string(raw)
	for _, want := range []string{"capabilities", "agents", "register", "file_reservations", "reserve", "--exclusive", "mail", "send", "--ack-required", "ack", "release"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing live Agent Mail contract token %q in %s", want, all)
		}
	}
}

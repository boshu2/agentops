package background

import (
	"strings"
	"testing"
)

func bgEvent(offset int, typ string) Event {
	return Event{
		SchemaVersion: EventSchemaVersion,
		SessionID:     "agentops-bg",
		Offset:        offset,
		Type:          typ,
		BeadID:        "ag-test",
		Worker:        "agentops-codex-ntm-worker",
	}
}

func TestDedupeSkipsReplayedOffsets(t *testing.T) {
	seen := map[string]bool{}
	first, err := Dedupe([]Event{bgEvent(1, "mail_assignment"), bgEvent(2, "reservation")}, seen)
	if err != nil {
		t.Fatalf("first dedupe: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first len = %d, want 2", len(first))
	}
	second, err := Dedupe([]Event{bgEvent(2, "reservation"), bgEvent(3, "check_in")}, seen)
	if err != nil {
		t.Fatalf("second dedupe: %v", err)
	}
	if len(second) != 1 || second[0].Offset != 3 {
		t.Fatalf("second = %+v, want only offset 3", second)
	}
}

func TestSummarizeIncludesWorkerAndArtifacts(t *testing.T) {
	a := bgEvent(1, "mail_assignment")
	a.ArtifactPath = ".agents/swarm/results/ag-test.json"
	b := bgEvent(2, "session_end")
	got, err := Summarize([]Event{a, b})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	for _, want := range []string{
		"background-agent session agentops-bg mirrored 2 event(s)",
		"last=session_end",
		"agentops-codex-ntm-worker",
		".agents/swarm/results/ag-test.json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
}

func TestPlanClosurePassWritesVerdictAndConditionalPush(t *testing.T) {
	e := bgEvent(4, "session_end")
	e.Verdict = "PASS"
	plan, err := PlanClosure(e)
	if err != nil {
		t.Fatalf("plan closure: %v", err)
	}
	if plan.LeaveOpen {
		t.Fatal("PASS closure should not leave bead open")
	}
	if plan.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS", plan.Verdict)
	}
	joined := strings.Join(plan.Commands, "\n")
	for _, want := range []string{"verdict=PASS", "bd comment ag-test", "only if a real Dolt remote is configured"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("commands %q missing %q", joined, want)
		}
	}
}

func TestPlanClosureFailureLeavesOpenForFreshSession(t *testing.T) {
	e := bgEvent(4, "session_end")
	e.Verdict = "FAIL"
	plan, err := PlanClosure(e)
	if err != nil {
		t.Fatalf("plan closure: %v", err)
	}
	if !plan.LeaveOpen {
		t.Fatal("FAIL closure should leave bead open")
	}
	if !strings.Contains(strings.Join(plan.Commands, "\n"), "--status open") {
		t.Fatalf("commands = %v, want reopen/leave-open command", plan.Commands)
	}
}

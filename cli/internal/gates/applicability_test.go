package gates

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestIsAgentOpsRepo(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{name: "empty dir is foreign", setup: func(*testing.T, string) {}, want: false},
		{name: "agentops marker", setup: writeAgentopsMarker, want: true},
		{
			name: "cli/go.mod with a different module is foreign",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "cli"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "cli", "go.mod"), []byte("module example.com/other/cli\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: false,
		},
		{
			name: "root go.mod alone is not the marker",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+agentopsCLIModulePath+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			if got := IsAgentOpsRepo(root); got != tc.want {
				t.Fatalf("IsAgentOpsRepo(%s) = %v, want %v", root, got, tc.want)
			}
		})
	}
}

// foreignRepoRegistry mirrors the real always-run shape: a blocking
// script-backed structural check plus a native check that is applicable in any
// repo (the constraints.enforce analogue).
func foreignRepoRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	for _, c := range []Check{
		{ID: "always.agentops-internal", Tiers: Fast | Full, Blocking: true, Backing: "check-agentops-internal.sh"},
		{ID: "native.portable", Tiers: Fast | Full, Blocking: true, Run: func(context.Context, RunContext) (ports.GateVerdict, error) {
			return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"}, nil
		}},
	} {
		if err := r.Add(c); err != nil {
			t.Fatalf("Add(%s): %v", c.ID, err)
		}
	}
	return r
}

// runForeignRepoOrchestrator is the L2 entry point: a real orchestrator over a
// real ScriptRunner rooted at root, fast mode, no changed files.
func runForeignRepoOrchestrator(t *testing.T, root string) *Report {
	t.Helper()
	o := NewOrchestrator(foreignRepoRegistry(t), NewScriptRunner(root), fakeFiles{}, root)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

// TestGateCheck_ForeignRepo_NotApplicableSkip is the novice-test edge 1
// acceptance: in a repo that is NOT the agentops repo, a script-backed
// always-run check whose backing artifact does not exist becomes a first-class
// not-applicable SKIP, the run passes (exit 0), the summary counts it in the
// skip bucket, and the human report ends with one honest aggregate line.
func TestGateCheck_ForeignRepo_NotApplicableSkip(t *testing.T) {
	rep := runForeignRepoOrchestrator(t, t.TempDir()) // no marker: foreign repo

	res, ok := resultByID(rep, "always.agentops-internal")
	if !ok {
		t.Fatal("always-run check should still be selected and reported")
	}
	if res.Verdict.Status != ports.GateStatusSkip {
		t.Fatalf("status = %s, want SKIP", res.Verdict.Status)
	}
	if res.Verdict.Reason != NotApplicableReason {
		t.Fatalf("reason = %q, want %q", res.Verdict.Reason, NotApplicableReason)
	}
	if rep.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (everything else passes)", rep.ExitCode())
	}
	if s := rep.Summary(); s != (Summary{Total: 2, Passed: 1, Skipped: 1}) {
		t.Fatalf("summary = %+v, want 2 total / 1 pass / 1 skip", s)
	}

	var out bytes.Buffer
	rep.Human(&out)
	human := out.String()
	for _, want := range []string{
		"2 checks — 1 pass, 0 warn, 0 fail, 0 unknown, 1 skip",
		"1 agentops-repo checks not applicable outside the agentops repository",
	} {
		if !strings.Contains(human, want) {
			t.Errorf("human output missing %q:\n%s", want, human)
		}
	}
	// Foreign-repo human output aggregates not-applicable rows instead of
	// printing them (they name backing scripts that don't exist in the user's
	// repo); JSON keeps every row.
	if strings.Contains(human, NotApplicableReason) {
		t.Errorf("human output should suppress per-row not-applicable detail in a foreign repo:\n%s", human)
	}
	if strings.Contains(human, "skipped gates:") {
		t.Errorf("human output should suppress the routing-skip detail block in a foreign repo:\n%s", human)
	}
	raw, err := rep.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !strings.Contains(string(raw), NotApplicableReason) {
		t.Errorf("JSON output must keep the not-applicable row; got:\n%s", raw)
	}
}

// TestGateCheck_AgentopsRepo_MissingScriptStaysUnknown pins the unchanged
// in-repo guard: with the agentops marker present but a backing script missing,
// the check stays UNKNOWN and the blocking run still fails (exit 1) — agentops
// CI must never silently lose a gate.
func TestGateCheck_AgentopsRepo_MissingScriptStaysUnknown(t *testing.T) {
	root := t.TempDir()
	writeAgentopsMarker(t, root)
	rep := runForeignRepoOrchestrator(t, root)

	res, ok := resultByID(rep, "always.agentops-internal")
	if !ok {
		t.Fatal("always-run check should be reported")
	}
	if res.Verdict.Status != ports.GateStatusUnknown {
		t.Fatalf("status = %s, want UNKNOWN (fail-closed inside agentops repo)", res.Verdict.Status)
	}
	if rep.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1 (blocking UNKNOWN fails closed)", rep.ExitCode())
	}
	if s := rep.Summary(); s != (Summary{Total: 2, Passed: 1, Unknown: 1}) {
		t.Fatalf("summary = %+v, want 2 total / 1 pass / 1 unknown", s)
	}

	var out bytes.Buffer
	rep.Human(&out)
	human := out.String()
	if want := "2 checks — 1 pass, 0 warn, 0 fail, 1 unknown, 0 skip"; !strings.Contains(human, want) {
		t.Errorf("human summary missing %q:\n%s", want, human)
	}
	if strings.Contains(human, "not applicable outside the agentops repository") {
		t.Errorf("in-repo run must not claim not-applicable:\n%s", human)
	}
}

// TestReport_UnknownCountedInOwnBucket pins that UNKNOWN renders as its own
// bucket in both the human summary line and the JSON run summary — the line can
// never again claim all-clear while UNKNOWN results exist.
func TestReport_UnknownCountedInOwnBucket(t *testing.T) {
	rep := &Report{
		Mode:      Fast,
		Scope:     ScopeHead,
		StartedAt: time.Unix(0, 0),
		Results: []CheckResult{
			{Check: Check{ID: "ok", Blocking: true}, Verdict: ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"}},
			{Check: Check{ID: "lost.a", Blocking: true}, Verdict: ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: "no script"}},
			{Check: Check{ID: "lost.b", Blocking: true}, Verdict: ports.GateVerdict{Status: ports.GateStatusUnknown, Reason: "no script"}},
		},
	}
	if s := rep.Summary(); s != (Summary{Total: 3, Passed: 1, Unknown: 2}) {
		t.Fatalf("summary = %+v, want 3 total / 1 pass / 2 unknown", s)
	}

	var out bytes.Buffer
	rep.Human(&out)
	if want := "3 checks — 1 pass, 0 warn, 0 fail, 2 unknown, 0 skip"; !strings.Contains(out.String(), want) {
		t.Fatalf("human summary missing %q:\n%s", want, out.String())
	}

	raw, err := rep.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var parsed jsonReport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Run.Summary.Unknown != 2 {
		t.Fatalf("json summary.unknown = %d, want 2", parsed.Run.Summary.Unknown)
	}
}

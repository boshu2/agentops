// practices: [design-by-contract, in-toto-provenance]
package turnverify

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/evidencedturn"
	"github.com/boshu2/agentops/cli/internal/turnstate"
)

// closedTransitions builds a sealed ""->in_progress->validated->closed log
// for the bead and returns it as the transitions field for the input file.
func closedTransitions(t *testing.T, bead string) []turnstate.Transition {
	t.Helper()
	steps := []struct{ from, to, ts string }{
		{turnstate.InitialState, "in_progress", "2026-05-31T00:00:00Z"},
		{"in_progress", "validated", "2026-05-31T01:00:00Z"},
		{"validated", "closed", "2026-05-31T02:00:00Z"},
	}
	var log []turnstate.Transition
	for _, s := range steps {
		var err error
		log, err = turnstate.Append(log, turnstate.Transition{
			ArtifactID: bead, FromState: s.from, ToState: s.to, TS: s.ts,
		})
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	return log
}

// writeTurnInput marshals a turn-input file to a temp path and returns it. It
// records a distinct author/judge by default so the no-self-grading invariant
// passes for otherwise-complete fixtures.
func writeTurnInput(t *testing.T, bead string, transitions []turnstate.Transition, scenarios []evidencedturn.Scenario) string {
	t.Helper()
	return writeTurnInputGraded(t, bead, transitions, scenarios, "session:author", "session:judge")
}

// writeTurnInputGraded is writeTurnInput with explicit author/judge identities,
// for tests of the author_neq_validator predicate.
func writeTurnInputGraded(t *testing.T, bead string, transitions []turnstate.Transition, scenarios []evidencedturn.Scenario, authorID, judgeID string) string {
	t.Helper()
	tf := InputFile{BeadID: bead, Transitions: transitions, Scenarios: scenarios, AuthorID: authorID, JudgeID: judgeID}
	b, err := json.Marshal(tf)
	if err != nil {
		t.Fatalf("marshal turn input: %v", err)
	}
	path := filepath.Join(t.TempDir(), "turn.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write turn input: %v", err)
	}
	return path
}

// writeLedger writes a provenance ledger JSONL fixture (raw lines) to a temp
// path and returns it. Each line is an already-formed JSON object.
func writeLedger(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
	return path
}

// beadEdgeLine builds a sealed provenance-edge ledger line referencing the bead
// as to_id (a "commit implements" edge).
func beadEdgeLine(t *testing.T, bead string) string {
	t.Helper()
	obj := map[string]any{
		"schema_version": "agentops-sdlc-provenance.v1",
		"from_id":        "commit:abc123",
		"from_type":      "commit",
		"to_id":          bead,
		"to_type":        "bead",
		"relation":       "wasRevisionOf",
		"evidence_ref":   "deadbeef",
		"trust_tier":     "inferred",
		"ts":             "2026-05-31T03:00:00Z",
		"prev_hash":      "",
		"payload_hash":   "x",
		"hash":           "y",
	}
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal edge: %v", err)
	}
	return string(b)
}

func writeCleanGraph(t *testing.T, bead string) string {
	t.Helper()
	artifact := "cli/internal/evidencedturn/evidencedturn.go"
	lines := []string{
		`{"record":"node","id":"` + bead + `","type":"bead"}`,
		`{"record":"node","id":"` + artifact + `","type":"artifact","path":"` + artifact + `"}`,
		`{"record":"edge","edge_type":"bead_produces_artifact","from_id":"` + bead + `","to_id":"` + artifact + `"}`,
	}
	return writeLedger(t, lines)
}

func writeOrphanGraph(t *testing.T, bead string) string {
	t.Helper()
	artifact := "cli/orphan.go"
	lines := []string{
		`{"record":"node","id":"` + bead + `","type":"bead"}`,
		`{"record":"node","id":"` + artifact + `","type":"artifact","path":"` + artifact + `"}`,
	}
	return writeLedger(t, lines)
}

func runForTest(bead, input, ledger, graph string, jsonOut, allowSelf bool) (string, error) {
	var out bytes.Buffer
	err := Run(Options{
		BeadID:     bead,
		InputPath:  input,
		LedgerPath: ledger,
		GraphPath:  graph,
		JSON:       jsonOut,
		AllowSelf:  allowSelf,
		Stdout:     &out,
	})
	return out.String(), err
}

func TestRun_CompleteTurnVerifies(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}})
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})
	graph := writeCleanGraph(t, bead)

	out, err := runForTest(bead, input, ledger, graph, false, false)
	if err != nil {
		t.Fatalf("turn verify of a complete turn should succeed, got err=%v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "VERDICT: DONE") {
		t.Errorf("output missing DONE verdict:\n%s", out)
	}
	if !strings.Contains(out, "[PASS] chain_intact") {
		t.Errorf("output missing chain_intact PASS:\n%s", out)
	}
}

func TestRun_IncompleteTurnIsNotDone(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: false, EvidenceResolved: true}})
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})

	out, err := runForTest(bead, input, ledger, "", false, false)
	if err == nil {
		t.Fatalf("turn verify of an incomplete turn must exit non-zero\noutput=%s", out)
	}
	if !strings.Contains(out, "VERDICT: NOT DONE") {
		t.Errorf("output missing NOT DONE verdict:\n%s", out)
	}
	if !strings.Contains(out, "ao-turn-verify") {
		t.Errorf("output should name the uncovered scenario ao-turn-verify:\n%s", out)
	}
	if !strings.Contains(out, "[FAIL] scenarios_covered") {
		t.Errorf("output missing scenarios_covered FAIL:\n%s", out)
	}
}

func TestRun_JSONShape(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}})
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})
	graph := writeCleanGraph(t, bead)

	out, err := runForTest(bead, input, ledger, graph, true, false)
	if err != nil {
		t.Fatalf("turn verify --json should succeed, got err=%v\noutput=%s", err, out)
	}
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no JSON object in output:\n%s", out)
	}
	var v evidencedturn.Verdict
	if err := json.Unmarshal([]byte(out[idx:]), &v); err != nil {
		t.Fatalf("output is not valid Verdict JSON: %v\n%s", err, out)
	}
	if v.SchemaVersion != evidencedturn.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", v.SchemaVersion, evidencedturn.SchemaVersion)
	}
	if v.BeadID != bead {
		t.Errorf("bead_id = %q, want %q", v.BeadID, bead)
	}
	if v.Done != true {
		t.Errorf("done = %v, want true", v.Done)
	}
	if len(v.Predicates) != 7 {
		t.Errorf("len(predicates) = %d, want 7", len(v.Predicates))
	}
}

func TestRun_SelfGradedRefused(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInputGraded(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}},
		"session:same", "session:same")
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})
	graph := writeCleanGraph(t, bead)

	out, err := runForTest(bead, input, ledger, graph, false, false)
	if err == nil {
		t.Fatalf("self-graded verdict must exit non-zero\noutput=%s", out)
	}
	if !strings.Contains(out, "[FAIL] author_neq_validator") {
		t.Errorf("output missing author_neq_validator FAIL:\n%s", out)
	}
}

func TestRun_AllowSelfPasses(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInputGraded(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}},
		"session:same", "session:same")
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})
	graph := writeCleanGraph(t, bead)

	out, err := runForTest(bead, input, ledger, graph, false, true)
	if err != nil {
		t.Fatalf("turn verify --allow-self should succeed, got err=%v\noutput=%s", err, out)
	}
	if !strings.Contains(out, "VERDICT: DONE") {
		t.Errorf("output missing DONE verdict with --allow-self:\n%s", out)
	}
}

func TestRun_NoProvenanceEventFails(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}})
	ledger := writeLedger(t, nil)

	out, err := runForTest(bead, input, ledger, "", false, false)
	if err == nil {
		t.Fatalf("missing provenance event must exit non-zero\noutput=%s", out)
	}
	if !strings.Contains(out, "[FAIL] provenance_event") {
		t.Errorf("output missing provenance_event FAIL:\n%s", out)
	}
}

func TestRun_OrphanArtifactFails(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}})
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})
	graph := writeOrphanGraph(t, bead)

	out, err := runForTest(bead, input, ledger, graph, false, false)
	if err == nil {
		t.Fatalf("orphan artifact must exit non-zero\noutput=%s", out)
	}
	if !strings.Contains(out, "[FAIL] no_orphan") {
		t.Errorf("output missing no_orphan FAIL:\n%s", out)
	}
	if !strings.Contains(out, "cli/orphan.go") {
		t.Errorf("output should name the orphan artifact cli/orphan.go:\n%s", out)
	}
}

func TestRun_NoGraphLeavesOrphanUnchecked(t *testing.T) {
	bead := "ag-lmdx.5"
	input := writeTurnInput(t, bead, closedTransitions(t, bead),
		[]evidencedturn.Scenario{{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true}})
	ledger := writeLedger(t, []string{beadEdgeLine(t, bead)})

	out, err := runForTest(bead, input, ledger, "", false, false)
	if err == nil {
		t.Fatalf("missing --graph leaves no_orphan unchecked -> not done\noutput=%s", out)
	}
	if !strings.Contains(out, "[FAIL] no_orphan") {
		t.Errorf("output missing no_orphan FAIL:\n%s", out)
	}
	if !strings.Contains(out, "orphan audit not run") {
		t.Errorf("output should explain orphan audit not run:\n%s", out)
	}
}

func TestRun_RequiresInputFlag(t *testing.T) {
	out, err := runForTest("ag-lmdx.5", "", "", "", false, false)
	if err == nil {
		t.Fatalf("turn verify without --input must error\noutput=%s", out)
	}
	if !strings.Contains(err.Error(), "--input") {
		t.Errorf("error = %q, want it to mention --input", err.Error())
	}
}

func TestRun_InputBeadMismatchErrors(t *testing.T) {
	input := writeTurnInput(t, "ag-other", closedTransitions(t, "ag-other"),
		[]evidencedturn.Scenario{{Slug: "s", HasPassingTest: true, EvidenceResolved: true}})

	out, err := runForTest("ag-lmdx.5", input, "", "", false, false)
	if err == nil {
		t.Fatalf("mismatched bead must error\noutput=%s", out)
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error = %q, want it to mention the bead mismatch", err.Error())
	}
}

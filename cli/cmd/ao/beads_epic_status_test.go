package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/epicstatus"
	"github.com/spf13/cobra"
)

// jsonLedger renders a set of bead records as newline-delimited issues.jsonl,
// round-tripping through the production ledgerBead reader shape.
func jsonLedger(t *testing.T, beads []ledgerBead) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, b := range beads {
		line, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("marshal bead %s: %v", b.ID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func parentChild(child, parent string) ledgerDep {
	return ledgerDep{IssueID: child, DependsOnID: parent, Type: "parent-child"}
}

func blocks(from, on string) ledgerDep {
	return ledgerDep{IssueID: from, DependsOnID: on, Type: "blocks"}
}

// TestBuildMembers_UnionOfPrefixAndParentChild pins that membership is the
// union of id-prefix children AND parent-child dependency edges, and that the
// epic record's presence is reported.
func TestBuildMembers_UnionOfPrefixAndParentChild(t *testing.T) {
	beads := []ledgerBead{
		{ID: "age-x", Status: "open", IssueType: "epic"},
		{ID: "age-x.1", Status: "closed", Dependencies: []ledgerDep{parentChild("age-x.1", "age-x")}},
		// id-prefix child WITHOUT an explicit edge — must still be a member.
		{ID: "age-x.2", Status: "closed"},
		// parent-child edge to the epic but NON-prefix id — must still be a member.
		{ID: "sib-9", Status: "closed", Dependencies: []ledgerDep{parentChild("sib-9", "age-x")}},
		// unrelated bead — must NOT be a member.
		{ID: "age-y.1", Status: "open"},
	}
	members, present := buildMembers("age-x", beads)
	if !present {
		t.Fatal("epicPresent = false, want true")
	}
	got := map[string]bool{}
	for _, m := range members {
		got[m.ID] = true
	}
	want := []string{"age-x.1", "age-x.2", "sib-9"}
	if len(members) != len(want) {
		t.Fatalf("member ids = %v, want %v", got, want)
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("missing member %q; got %v", id, got)
		}
	}
	if got["age-y.1"] {
		t.Errorf("unrelated bead age-y.1 wrongly included")
	}
}

// TestBuildMembers_EpicAbsent pins that a non-existent epic reports absent.
func TestBuildMembers_EpicAbsent(t *testing.T) {
	beads := []ledgerBead{{ID: "age-x.1", Status: "closed"}}
	members, present := buildMembers("age-x", beads)
	if present {
		t.Errorf("epicPresent = true for absent epic, want false")
	}
	if members != nil {
		t.Errorf("members = %v, want nil for absent epic", members)
	}
}

// TestBuildMembers_DanglingFamilyReferenceIsMissing pins guard 1's real-data
// path: a family id named by a blocks edge but absent from the ledger becomes
// an unresolved (Present=false) member.
func TestBuildMembers_DanglingFamilyReferenceIsMissing(t *testing.T) {
	beads := []ledgerBead{
		{ID: "age-x", Status: "open", IssueType: "epic"},
		{ID: "age-x.1", Status: "closed", Dependencies: []ledgerDep{parentChild("age-x.1", "age-x")}},
		// age-x.2 references a deleted sibling age-x.3 that has no record.
		{ID: "age-x.2", Status: "closed", Dependencies: []ledgerDep{
			parentChild("age-x.2", "age-x"),
			blocks("age-x.2", "age-x.3"),
		}},
	}
	members, present := buildMembers("age-x", beads)
	if !present {
		t.Fatal("epicPresent = false, want true")
	}
	var missing *epicstatus.Member
	for i := range members {
		if members[i].ID == "age-x.3" {
			missing = &members[i]
		}
	}
	if missing == nil {
		t.Fatalf("age-x.3 dangling reference not surfaced as a member; got %+v", members)
	}
	if missing.Present {
		t.Errorf("age-x.3 Present = true, want false (unresolved)")
	}
}

// setEpicStatusFlags flips the package-global cobra flags for one test and
// restores them via t.Cleanup (shuffle-order isolation).
func setEpicStatusFlags(t *testing.T, terminal, jsonOut bool) {
	t.Helper()
	origT, origJ := beadsEpicStatusTerminal, beadsEpicStatusJSON
	t.Cleanup(func() { beadsEpicStatusTerminal, beadsEpicStatusJSON = origT, origJ })
	beadsEpicStatusTerminal, beadsEpicStatusJSON = terminal, jsonOut
}

// stubLedger installs an in-memory ledger reader for one test, restored after.
func stubLedger(t *testing.T, raw []byte, err error) {
	t.Helper()
	orig := beadsEpicStatusReadLedger
	t.Cleanup(func() { beadsEpicStatusReadLedger = orig })
	beadsEpicStatusReadLedger = func(string) ([]byte, error) { return raw, err }
}

// runEpicStatus invokes the command RunE against a captured stdout buffer.
func runEpicStatus(t *testing.T, epic string) (string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runBeadsEpicStatus(cmd, []string{epic})
	return buf.String(), err
}

// TestRunBeadsEpicStatus_TerminalJSON drives the whole command over an
// in-memory ledger and asserts the happy-path JSON verdict.
func TestRunBeadsEpicStatus_TerminalJSON(t *testing.T) {
	setEpicStatusFlags(t, true, true)
	stubLedger(t, jsonLedger(t, []ledgerBead{
		{ID: "age-x", Status: "open", IssueType: "epic"},
		{ID: "age-x.1", Status: "closed", Dependencies: []ledgerDep{parentChild("age-x.1", "age-x")}},
		{ID: "age-x.2", Status: "closed", Dependencies: []ledgerDep{parentChild("age-x.2", "age-x")}},
	}), nil)

	out, err := runEpicStatus(t, "age-x")
	if err != nil {
		t.Fatalf("terminal epic returned error %v (want nil), out=%s", err, out)
	}
	var r epicstatus.Result
	if e := json.Unmarshal([]byte(out), &r); e != nil {
		t.Fatalf("output is not JSON: %v\n%s", e, out)
	}
	if r.Verdict != epicstatus.Terminal || !r.Terminal {
		t.Errorf("Verdict = %q Terminal=%v, want terminal/true", r.Verdict, r.Terminal)
	}
	if r.Total != 2 || r.Done != 2 {
		t.Errorf("Total/Done = %d/%d, want 2/2", r.Total, r.Done)
	}
	if r.Code != epicstatus.ReasonAllTerminal {
		t.Errorf("Code = %q, want %q", r.Code, epicstatus.ReasonAllTerminal)
	}
}

// TestRunBeadsEpicStatus_NotTerminalExitCode pins that --terminal maps a
// not-terminal verdict to exit code 2 via beadsExitError.
func TestRunBeadsEpicStatus_NotTerminalExitCode(t *testing.T) {
	setEpicStatusFlags(t, true, false)
	stubLedger(t, jsonLedger(t, []ledgerBead{
		{ID: "age-x", Status: "open", IssueType: "epic"},
		{ID: "age-x.1", Status: "closed", Dependencies: []ledgerDep{parentChild("age-x.1", "age-x")}},
		{ID: "age-x.2", Status: "open", Labels: []string{"checkpoint"},
			Dependencies: []ledgerDep{parentChild("age-x.2", "age-x")}},
	}), nil)

	out, err := runEpicStatus(t, "age-x")
	var exitErr *beadsExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *beadsExitError", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
	if !strings.Contains(out, "open-checkpoint") {
		t.Errorf("human output missing open-checkpoint class:\n%s", out)
	}
}

// TestRunBeadsEpicStatus_SkippedExitCode pins that a zero-descendant epic with
// --terminal maps to exit code 3 (skipped, materializing).
func TestRunBeadsEpicStatus_SkippedExitCode(t *testing.T) {
	setEpicStatusFlags(t, true, false)
	stubLedger(t, jsonLedger(t, []ledgerBead{
		{ID: "age-x", Status: "open", IssueType: "epic"},
	}), nil)

	_, err := runEpicStatus(t, "age-x")
	var exitErr *beadsExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *beadsExitError", err)
	}
	if exitErr.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.ExitCode())
	}
}

// TestRunBeadsEpicStatus_EpicNotFound pins that an absent epic is a genuine
// error (exit 1 path), not a verdict.
func TestRunBeadsEpicStatus_EpicNotFound(t *testing.T) {
	setEpicStatusFlags(t, true, false)
	stubLedger(t, jsonLedger(t, []ledgerBead{{ID: "age-x.1", Status: "closed"}}), nil)

	_, err := runEpicStatus(t, "age-x")
	if err == nil {
		t.Fatal("err = nil, want epic-not-found error")
	}
	var exitErr *beadsExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("epic-not-found mapped to verdict exit %d; want a plain error", exitErr.ExitCode())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err.Error())
	}
}

// TestParseLedger_MalformedLineFailsClosed pins that a corrupt ledger line is a
// hard error, never silently dropped (unreachable is not absent).
func TestParseLedger_MalformedLineFailsClosed(t *testing.T) {
	raw := []byte(`{"id":"age-x","status":"open"}` + "\n" + `{not json}` + "\n")
	if _, err := parseLedger(raw); err == nil {
		t.Fatal("parseLedger accepted a malformed line; want a hard error")
	}
}

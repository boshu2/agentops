// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// stubReviewerPATH points PATH at a temp bin dir containing instantly-answering
// fake binaries for the given reviewer families, plus the system dirs (git/sh
// stay resolvable). Restoring is handled by t.Setenv.
func stubReviewerPATH(t *testing.T, families ...string) {
	t.Helper()
	bin := t.TempDir()
	for _, f := range families {
		writeFakeBin(t, bin, f, "#!/bin/sh\necho fake 1.0\n")
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", bin+sep+"/usr/bin"+sep+"/bin")
}

// setQuickstartMode sets the package-global quick-start cobra flag vars and
// restores them via t.Cleanup (test-isolation rule: never leak globals).
func setQuickstartMode(t *testing.T, min, noB bool) {
	t.Helper()
	oldMin, oldNo := minimal, noBeads
	minimal, noBeads = min, noB
	t.Cleanup(func() { minimal, noBeads = oldMin, oldNo })
}

func TestQuickstart_FirstVerdict_ReviewerReachable(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")
	setQuickstartMode(t, true, true)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Errorf("runQuickstart: %v", err)
		}
	})

	if !strings.Contains(out, "FINAL STEP: your first verdict") {
		t.Fatalf("quick-start must end on the first-verdict step, got:\n%s", out)
	}
	if !strings.Contains(out, firstVerdictCommand) {
		t.Errorf("output must print the exact next command %q, got:\n%s", firstVerdictCommand, out)
	}
	if !strings.Contains(out, "Reviewer reachable: codex") {
		t.Errorf("output must name the live reviewer family, got:\n%s", out)
	}
	if !strings.Contains(out, "CONFIRMED or REFUTED") {
		t.Errorf("output must explain the verdict outcome, got:\n%s", out)
	}
	if !strings.Contains(out, "nothing runs until you run it") {
		t.Errorf("output must state the reviewer is never auto-run, got:\n%s", out)
	}
	// The ledger parent directory must actually exist after quick-start.
	if _, err := os.Stat(filepath.Join(tmp, "docs", "provenance")); err != nil {
		t.Errorf("ledger parent dir not created: %v", err)
	}
}

func TestQuickstart_FirstVerdict_NoReviewerPrintsInstall(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t) // no reviewer binaries at all
	setQuickstartMode(t, true, true)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Errorf("runQuickstart: %v", err)
		}
	})

	if strings.Contains(out, firstVerdictCommand) {
		t.Errorf("no reviewer reachable must NOT print the verify command, got:\n%s", out)
	}
	if !strings.Contains(out, "No reviewer CLI reachable") {
		t.Errorf("output must state no reviewer is reachable, got:\n%s", out)
	}
	// The doctor corrective install one-liners, verbatim.
	if !strings.Contains(out, "npm install -g @openai/codex && codex login") {
		t.Errorf("output must print the exact codex install one-liner, got:\n%s", out)
	}
	if !strings.Contains(out, "install the AGY CLI, then verify with 'agy models'") {
		t.Errorf("output must print the exact agy install line, got:\n%s", out)
	}
}

func TestQuickstart_FirstVerdict_FullFlowEndsOnVerdictStep(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")
	setQuickstartMode(t, false, true)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Errorf("runQuickstart full: %v", err)
		}
	})

	idxSetup := strings.Index(out, "SETUP COMPLETE")
	idxVerdict := strings.Index(out, "FINAL STEP: your first verdict")
	if idxSetup == -1 || idxVerdict == -1 {
		t.Fatalf("expected both setup-complete and first-verdict sections, got:\n%s", out)
	}
	if idxVerdict < idxSetup {
		t.Errorf("first-verdict step must be the LAST section (end of the golden path)")
	}
	if !strings.Contains(out, firstVerdictCommand) {
		t.Errorf("full flow must print %q, got:\n%s", firstVerdictCommand, out)
	}
}

func TestQuickstart_JSON_FirstVerdictShape(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")

	out, err := executeCommand("quick-start", "--json", "--minimal", "--no-beads")
	if err != nil {
		t.Fatalf("ao quick-start --json --minimal --no-beads: %v\n%s", err, out)
	}
	var result quickstartResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("quick-start JSON not parseable: %v\n%s", err, out)
	}
	fv := result.FirstVerdict
	if fv == nil {
		t.Fatal("expected first_verdict in JSON output")
	}
	if fv.NextCommand != firstVerdictCommand {
		t.Errorf("next_command = %q, want %q", fv.NextCommand, firstVerdictCommand)
	}
	if len(fv.ReviewerLive) != 1 || fv.ReviewerLive[0] != "codex" {
		t.Errorf("reviewer_live = %v, want [codex]", fv.ReviewerLive)
	}
	if !fv.LedgerReady {
		t.Error("ledger_ready = false, want true")
	}
	wantSuffix := filepath.Join("docs", "provenance", "ledger.jsonl")
	if !strings.HasSuffix(fv.LedgerPath, wantSuffix) {
		t.Errorf("ledger_path = %q, want suffix %q", fv.LedgerPath, wantSuffix)
	}
	if len(fv.ReviewerInstall) != 0 {
		t.Errorf("reviewer_install must be empty when a reviewer is live, got %v", fv.ReviewerInstall)
	}
}

func TestQuickstart_DryRunOmitsFirstVerdict(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")

	out, err := executeCommand("quick-start", "--dry-run", "--json", "--no-beads")
	if err != nil {
		t.Fatalf("ao quick-start --dry-run --json: %v\n%s", err, out)
	}
	var result quickstartResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("dry-run JSON not parseable: %v\n%s", err, out)
	}
	if result.FirstVerdict != nil {
		t.Errorf("dry-run must not probe or prepare the first-verdict step, got %+v", result.FirstVerdict)
	}
	if _, err := os.Stat(filepath.Join(tmp, "docs", "provenance")); err == nil {
		t.Error("dry-run must not create the ledger directory")
	}
}

func TestPrepareFirstVerdict_InstallLinesCoverEveryFamily(t *testing.T) {
	chdirTo(t, t.TempDir())
	stubReviewerPATH(t) // nothing reachable
	info := prepareFirstVerdict()
	if len(info.ReviewerInstall) != len(wedgeReviewers) {
		t.Fatalf("got %d install lines, want %d (one per reviewer family)", len(info.ReviewerInstall), len(wedgeReviewers))
	}
	for i, r := range wedgeReviewers {
		want := r.name + ": " + r.installCmd
		if info.ReviewerInstall[i] != want {
			t.Errorf("install[%d] = %q, want %q", i, info.ReviewerInstall[i], want)
		}
	}
	if info.NextCommand != "" {
		t.Errorf("next_command must be empty with no reviewer, got %q", info.NextCommand)
	}
}

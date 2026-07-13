// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setQuickstartVerbose sets the package-global --verbose flag var and restores
// it via t.Cleanup (test-isolation rule: never leak a package global).
func setQuickstartVerbose(t *testing.T, v bool) {
	t.Helper()
	old := quickstartVerbose
	quickstartVerbose = v
	t.Cleanup(func() { quickstartVerbose = old })
}

// stubEnvPATH points PATH at a temp bin dir containing instantly-succeeding
// fake binaries for the given names, plus the system dirs so git/sh stay
// resolvable. Restoration is handled by t.Setenv.
func stubEnvPATH(t *testing.T, names ...string) {
	t.Helper()
	bin := t.TempDir()
	for _, n := range names {
		writeFakeBin(t, bin, n, "#!/bin/sh\nexit 0\n")
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", bin+sep+"/usr/bin"+sep+"/bin")
}

// TestQuickStart_OutputOneScreen asserts the default (diet) quick-start output
// fits one screen (<= 40 lines), carries exactly one Next action, links the
// first-value doc, and drops the long-form banner + philosophy quote.
func TestQuickStart_OutputOneScreen(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex") // reviewer reachable; no br/gt/agy on PATH
	setQuickstartMode(t, false, true)
	setQuickstartVerbose(t, false)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	lines := strings.Count(out, "\n")
	if lines > 40 {
		t.Fatalf("diet output must be <= 40 lines, got %d:\n%s", lines, out)
	}

	nextCount := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "Next:") {
			nextCount++
		}
	}
	if nextCount != 1 {
		t.Fatalf("diet output must have exactly one Next action, got %d:\n%s", nextCount, out)
	}
	if !strings.Contains(out, quickstartNextSkill) {
		t.Errorf("Next action must reference the %s skill, got:\n%s", quickstartNextSkill, out)
	}
	if !strings.Contains(out, quickstartDocsLink) {
		t.Errorf("diet output must link %s, got:\n%s", quickstartDocsLink, out)
	}
	// Long-form chrome must be gone from the default output.
	for _, gone := range []string{"AGENTOPS QUICK START", "LIVE PATH", "Stateful environment"} {
		if strings.Contains(out, gone) {
			t.Errorf("diet output must not contain long-form %q:\n%s", gone, out)
		}
	}
	// Created summary marks the template file.
	if !strings.Contains(out, "GOALS.md") || !strings.Contains(out, "template — edit me") {
		t.Errorf("diet output must mark GOALS.md as a template, got:\n%s", out)
	}
}

// TestQuickStart_GoldenPathsResolveInEnv asserts every rendered "$ run-this"
// golden path is one whose argv[0] resolves on the fixture PATH, and that a
// path whose binary is absent is not rendered as a run-this step.
func TestQuickStart_GoldenPathsResolveInEnv(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	// Env profile: br + codex present; gt + agy absent.
	stubEnvPATH(t, "br", "codex")
	setQuickstartMode(t, false, false) // full flow, beads enabled
	setQuickstartVerbose(t, false)
	// initBeads reads stdin for a prefix prompt; EOF (no tty) takes the default.
	// The stub br exits 0 without creating a ledger, so materialize a br-shaped
	// one (_beads; a bare .beads would make tracker selection pick bd): the
	// br golden path renders only for binary-present AND tracker-initialized
	// (see TestQuickStart_MinimalNoTrackerOmitsBrPath for the inverse).
	if err := os.MkdirAll("_beads", 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	runLines := extractDietRunThis(out)
	if len(runLines) == 0 {
		t.Fatalf("expected at least one rendered golden path, got:\n%s", out)
	}
	for _, cmd := range runLines {
		argv0 := strings.Fields(cmd)[0]
		if argv0 == "ao" || strings.HasPrefix(argv0, "/") {
			continue // self + skill paths are always fine
		}
		if _, err := exec.LookPath(argv0); err != nil {
			t.Errorf("rendered golden path %q does not resolve in the fixture PATH: %v", cmd, err)
		}
	}

	// br + codex resolve → rendered; gt + agy absent → never rendered as run-this.
	joined := strings.Join(runLines, "\n")
	if !strings.Contains(joined, "br ready") {
		t.Errorf("br is on PATH; its golden path must render, got run-lines:\n%s", joined)
	}
	if !strings.Contains(joined, "codex exec") {
		t.Errorf("codex is on PATH; its golden path must render, got run-lines:\n%s", joined)
	}
	for _, absent := range []string{"gt log", "agy -p"} {
		if strings.Contains(joined, absent) {
			t.Errorf("tool for %q is absent; it must not render as a run-this step, got:\n%s", absent, joined)
		}
	}
}

// TestQuickStart_GoldenPaths_AbsentToolEnableLine asserts an absent core tool
// (br) collapses to a single enable line rather than a run-this block.
func TestQuickStart_GoldenPaths_AbsentToolEnableLine(t *testing.T) {
	stubEnvPATH(t) // nothing on PATH but system dirs
	out, _ := captureStdout(t, func() error {
		renderGoldenPaths(true) // tracking enabled
		return nil
	})
	if strings.Contains(out, "$ br ready") {
		t.Errorf("br is absent; it must not render as a run-this step, got:\n%s", out)
	}
	enableCount := strings.Count(out, "br not found — install beads_rust")
	if enableCount != 1 {
		t.Errorf("absent br must produce exactly one enable line, got %d:\n%s", enableCount, out)
	}
}

// TestQuickStart_VerboseKeepsLongForm asserts --verbose restores the full
// step-by-step long form (banner, LIVE PATH journey, philosophy quote).
func TestQuickStart_VerboseKeepsLongForm(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")
	setQuickstartMode(t, false, true)
	setQuickstartVerbose(t, true)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runQuickstart --verbose: %v", err)
		}
	})

	for _, want := range []string{"AGENTOPS QUICK START", "LIVE PATH", "Stateful environment"} {
		if !strings.Contains(out, want) {
			t.Errorf("--verbose must keep long-form %q, got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, firstVerdictCommand) {
		t.Errorf("--verbose must still end on the first-verdict command, got:\n%s", out)
	}
}

// extractDietRunThis returns the run-this command text from each diet
// golden-path line ("  $ <cmd...>  <description>"). The command is the text
// after "$ " up to the two-space description gutter.
func extractDietRunThis(out string) []string {
	var cmds []string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "$ ") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "$ ")
		// The description is separated by a run of >= 2 spaces.
		if idx := strings.Index(rest, "  "); idx >= 0 {
			rest = rest[:idx]
		}
		cmds = append(cmds, strings.TrimSpace(rest))
	}
	return cmds
}

// TestQuickStart_MinimalNoTrackerOmitsBrPath is the reviewer-named contract:
// a resolvable br binary is NOT enough — with --minimal (tracker init skipped,
// no ledger on disk) no beads-dependent golden path may render, because the
// advertised command would fail against an uninitialized tracker.
func TestQuickStart_MinimalNoTrackerOmitsBrPath(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubEnvPATH(t, "codex", "br") // br resolves — but no .beads/_beads ledger exists
	setQuickstartMode(t, true, false)
	setQuickstartVerbose(t, false)

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})
	if strings.Contains(out, "$ br") {
		t.Fatalf("minimal run with no initialized tracker must not advertise a br golden path; output:\n%s", out)
	}
}

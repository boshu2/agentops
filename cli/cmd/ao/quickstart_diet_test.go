// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setQuickstartDryRun sets the package-global root --dry-run flag var and
// restores it via t.Cleanup (test-isolation rule: never leak a package
// global). Mirrors setQuickstartVerbose/setQuickstartMode.
func setQuickstartDryRun(t *testing.T, v bool) {
	t.Helper()
	old := dryRun
	dryRun = v
	t.Cleanup(func() { dryRun = old })
}

// snapshotDir walks dir and returns a sorted list of "kind:relpath:size"
// entries. Two snapshots taken before/after an operation that must not touch
// the filesystem should be identical.
func snapshotDir(t *testing.T, dir string) []string {
	t.Helper()
	var entries []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			entries = append(entries, "d:"+rel)
			return nil
		}
		// Hash contents, not just size — an equal-length overwrite must
		// change the snapshot or "writes nothing" is unproven.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		entries = append(entries, fmt.Sprintf("f:%s:%x", rel, sum))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(entries)
	return entries
}

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

// TestQuickStart_DryRunWritesNothing asserts ao quick-start --dry-run leaves
// the working tree byte-for-byte identical (fs snapshot before/after) and
// never materializes .agents or .doctor, even against a repo with a
// pre-existing (unseeded) CLAUDE.md — the append path is planned, not applied.
func TestQuickStart_DryRunWritesNothing(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	if _, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command("git", "init")
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	setQuickstartMode(t, false, false)
	setQuickstartDryRun(t, true)

	before := snapshotDir(t, tmp)
	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runQuickstart --dry-run: %v", err)
		}
	})
	after := snapshotDir(t, tmp)

	if len(before) != len(after) {
		t.Fatalf("dry-run changed the file count: before=%v after=%v\noutput:\n%s", before, after, out)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("dry-run mutated the tree at entry %d: before=%q after=%q\noutput:\n%s", i, before[i], after[i], out)
		}
	}
	for _, p := range []string{".agents", ".doctor"} {
		if _, statErr := os.Stat(filepath.Join(tmp, p)); statErr == nil {
			t.Errorf("dry-run must not create %s", p)
		}
	}
}

// TestQuickStart_DryRunPlanNamesEveryArtifact asserts the --dry-run file plan
// covers every on-disk artifact quick-start ever touches (.agents/**,
// GOALS.md, CLAUDE.md, .gitignore, beads init), and that a pre-existing
// unseeded CLAUDE.md is reported as "append" with a non-empty preview while
// GOALS.md (which does not exist yet) is reported as "create" — the acceptance
// scenario named in the bead. Checked in both --json (structured) and the
// default table rendering.
func TestQuickStart_DryRunPlanNamesEveryArtifact(t *testing.T) {
	setupRepo := func(t *testing.T) string {
		t.Helper()
		tmp := t.TempDir()
		if _, err := exec.LookPath("git"); err == nil {
			cmd := exec.Command("git", "init")
			cmd.Dir = tmp
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Existing\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return tmp
	}

	t.Run("json", func(t *testing.T) {
		tmp := setupRepo(t)
		chdirTo(t, tmp)

		out, err := executeCommand("quick-start", "--dry-run", "--json")
		if err != nil {
			t.Fatalf("ao quick-start --dry-run --json: %v\n%s", err, out)
		}
		var result quickstartResult
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("dry-run JSON not parseable: %v\n%s", err, out)
		}

		byPath := map[string]quickstartPlanItem{}
		for _, item := range result.Plan {
			byPath[item.Path] = item
		}
		for _, want := range []string{".agents/**", "GOALS.md", "CLAUDE.md", ".gitignore", "beads init"} {
			if _, ok := byPath[want]; !ok {
				t.Errorf("dry-run plan missing artifact %q, got: %+v", want, result.Plan)
			}
		}

		claude := byPath["CLAUDE.md"]
		if claude.Action != "append" {
			t.Errorf("CLAUDE.md action = %q, want append (pre-existing file without seed marker)", claude.Action)
		}
		if claude.Preview == "" {
			t.Error("CLAUDE.md append action must carry a non-empty preview of the appended block")
		}

		goals := byPath["GOALS.md"]
		if goals.Action != "create" {
			t.Errorf("GOALS.md action = %q, want create", goals.Action)
		}

		for _, p := range []string{".agents", "GOALS.md", ".doctor"} {
			if _, statErr := os.Stat(filepath.Join(tmp, p)); statErr == nil {
				t.Errorf("dry-run must not create %s", p)
			}
		}
		data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "# Existing\n" {
			t.Errorf("dry-run must not write to CLAUDE.md, got: %q", string(data))
		}
	})

	t.Run("table", func(t *testing.T) {
		tmp := setupRepo(t)
		chdirTo(t, tmp)
		setQuickstartMode(t, false, false)
		setQuickstartDryRun(t, true)

		out := captureJSONStdout(t, func() {
			if err := runQuickstart(&cobra.Command{}, nil); err != nil {
				t.Fatalf("runQuickstart --dry-run: %v", err)
			}
		})
		if !strings.Contains(out, "CLAUDE.md") || !strings.Contains(out, "append") {
			t.Errorf("table dry-run output must report CLAUDE.md append, got:\n%s", out)
		}
		if !strings.Contains(out, "GOALS.md") || !strings.Contains(out, "create") {
			t.Errorf("table dry-run output must report GOALS.md create, got:\n%s", out)
		}
		if !strings.Contains(out, "No files were created") {
			t.Errorf("table dry-run output must confirm nothing was written, got:\n%s", out)
		}
	})
}

// TestQuickStart_RerunSummarizesNoop asserts that running the full (non-dry,
// non-verbose) quick-start flow a second time against an already-initialized
// repo collapses the output to the "Already set up" summary + readiness
// checklist + single Next line, instead of repeating the full setup ceremony.
func TestQuickStart_RerunSummarizesNoop(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")
	setQuickstartMode(t, false, true) // full flow, --no-beads (avoid needing a tracker binary)
	setQuickstartVerbose(t, false)

	first := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("first runQuickstart: %v", err)
		}
	})
	if strings.Contains(first, "Already set up") {
		t.Fatalf("first run on an empty repo must not claim it is already set up, got:\n%s", first)
	}
	if !strings.Contains(first, "SETUP COMPLETE") {
		t.Fatalf("first run must render the normal setup-complete ceremony, got:\n%s", first)
	}

	second := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("second runQuickstart: %v", err)
		}
	})
	if !strings.Contains(second, "Already set up — nothing changed") {
		t.Fatalf("re-run on an already-initialized repo must print the noop summary, got:\n%s", second)
	}
	if !strings.Contains(second, "Readiness:") {
		t.Errorf("re-run summary must still show the readiness checklist, got:\n%s", second)
	}
	nextCount := 0
	for _, l := range strings.Split(second, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "Next:") {
			nextCount++
		}
	}
	if nextCount != 1 {
		t.Errorf("re-run summary must have exactly one Next line, got %d:\n%s", nextCount, second)
	}
	for _, gone := range []string{"Created:", "SETUP COMPLETE"} {
		if strings.Contains(second, gone) {
			t.Errorf("re-run summary must not repeat %q, got:\n%s", gone, second)
		}
	}
}

// TestQuickStart_RerunNotNoopWhenGitignorePending is the reviewer-named
// contract: a fully seeded git repo whose .gitignore still lacks the
// /.agents/ entry gets that file written on re-run, so the run must NOT
// claim "Already set up — nothing changed."
func TestQuickStart_RerunNotNoopWhenGitignorePending(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex")
	setQuickstartMode(t, false, true)
	setQuickstartVerbose(t, false)

	// First run seeds everything (non-git dir: no .gitignore written).
	_ = captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("first runQuickstart: %v", err)
		}
	})
	// Now make it a git repo with a .gitignore missing the /.agents/ entry.
	if err := exec.Command("git", "-C", tmp, "init", "-q").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureJSONStdout(t, func() {
		if err := runQuickstart(&cobra.Command{}, nil); err != nil {
			t.Fatalf("second runQuickstart: %v", err)
		}
	})
	if strings.Contains(out, "Already set up") {
		t.Fatalf("re-run that writes .gitignore must not claim a no-op; output:\n%s", out)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/.agents/") {
		t.Fatalf(".gitignore should have gained the /.agents/ entry, got:\n%s", data)
	}
}

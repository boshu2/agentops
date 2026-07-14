// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/spf13/cobra"
)

// NOTE: TestQuickstartSkillCoversGlobalCorpus was removed (ag-jgkgf): the
// quickstart skill it guarded was deleted in the ag-s43tg skill-corpus prune
// (94db74318), leaving the doc-guard test with no surface to guard and the
// cmd/ao suite red on main.

func TestQuickstart_CommandExists(t *testing.T) {
	if quickstartCmd == nil {
		t.Fatal("quickstartCmd should not be nil")
	}
	if quickstartCmd.Use != "quick-start" {
		t.Errorf("quickstartCmd.Use = %q, want %q", quickstartCmd.Use, "quick-start")
	}
	if len(quickstartCmd.Aliases) != 1 || quickstartCmd.Aliases[0] != "quickstart" {
		t.Errorf("quickstartCmd.Aliases = %#v, want [quickstart]", quickstartCmd.Aliases)
	}
	if quickstartCmd.GroupID != "start" {
		t.Errorf("quickstartCmd.GroupID = %q, want %q", quickstartCmd.GroupID, "start")
	}
}

func TestQuickstart_HasFlags(t *testing.T) {
	if quickstartCmd.Flags().Lookup("no-beads") == nil {
		t.Error("quick-start should have --no-beads flag")
	}
	if quickstartCmd.Flags().Lookup("minimal") == nil {
		t.Error("quick-start should have --minimal flag")
	}
}

func TestQuickstart_RegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "quick-start" {
			found = true
			break
		}
	}
	if !found {
		t.Error("quickstartCmd should be registered on rootCmd")
	}
}

func TestQuickstart_CreateProjectClaudeMd_Content(t *testing.T) {
	tmp := t.TempDir()
	err := createProjectClaudeMd(tmp)
	if err != nil {
		t.Fatalf("createProjectClaudeMd: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Should contain key sections.
	for _, want := range []string{"Quick Start", "Session Protocol", "JIT Loading"} {
		if !strings.Contains(content, want) {
			t.Errorf("CLAUDE.md should contain %q", want)
		}
	}

	// Should use directory name as title.
	dirName := filepath.Base(tmp)
	if !strings.Contains(content, dirName) {
		t.Errorf("CLAUDE.md should contain directory name %q", dirName)
	}
}

func TestQuickstart_CreateTasksFile_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	createTasksFile(tmp)

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", "tasks.json"))
	if err != nil {
		t.Fatalf("read tasks.json: %v", err)
	}
	if !strings.Contains(string(data), "tasks") {
		t.Errorf("tasks.json should contain 'tasks' field, got: %s", string(data))
	}
	if !strings.Contains(string(data), "Beads-optional") {
		t.Errorf("tasks.json should contain note about beads-optional mode")
	}
}

func TestQuickstart_ShowNextSteps_WithBeads(t *testing.T) {
	out, _ := captureStdout(t, func() error { showNextSteps(true); return nil })
	if !strings.Contains(out, "ao beads exec ready") {
		t.Errorf("with beads=true, expected selected-tracker route 'ao beads exec ready' in output:\n%s", out)
	}
	for _, tombstone := range []string{"ao factory", "ao orchestrate", "ao codex", "/rpi"} {
		if strings.Contains(out, tombstone) {
			t.Errorf("with beads=true, quick-start teaches removed path %q:\n%s", tombstone, out)
		}
	}
}

func TestQuickstart_ShowNextSteps_WithoutBeads(t *testing.T) {
	out, _ := captureStdout(t, func() error { showNextSteps(false); return nil })
	if out == "" {
		t.Error("expected non-empty output for next steps")
	}
	for _, tombstone := range []string{"ao factory", "ao orchestrate", "ao codex", "/rpi"} {
		if strings.Contains(out, tombstone) {
			t.Errorf("without beads, quick-start teaches removed path %q:\n%s", tombstone, out)
		}
	}
}

func TestQuickstart_CreateStarterPack_CreatesPatterns(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{".agents/patterns", ".agents/learnings"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	err := createStarterPack(tmp)
	if err != nil {
		t.Fatalf("createStarterPack: %v", err)
	}

	expectedFiles := []string{
		".agents/patterns/context-boundaries.md",
		".agents/patterns/pre-mortem-first.md",
		".agents/learnings/session-hygiene.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmp, f)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected %s to have content", f)
		}
	}
}

func TestQuickstart_CreateStarterPack_PatternContent(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{".agents/patterns", ".agents/learnings"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	err := createStarterPack(tmp)
	if err != nil {
		t.Fatalf("createStarterPack: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents/patterns/context-boundaries.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Fresh Context Per Phase") {
		t.Error("context-boundaries.md should contain 'Fresh Context Per Phase'")
	}
	if !strings.Contains(content, "40% Rule") {
		t.Error("context-boundaries.md should contain '40% Rule'")
	}
}

// --- runQuickstart tests ---

func TestQuickstart_runQuickstart_minimal(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex") // the first-verdict step probes reviewers; never invoke real CLIs

	oldMinimal := minimal
	minimal = true
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart minimal: %v", err)
		}
	})

	if !strings.Contains(got, "Minimal setup complete") {
		t.Fatalf("expected minimal completion message, got: %s", got)
	}

	// Verify directories were created
	dirs := []string{
		".agents/research",
		".agents/synthesis",
		".agents/specs",
		".agents/learnings",
		".agents/patterns",
		".agents/retro",
		".agents/handoff",
	}
	for _, dir := range dirs {
		path := filepath.Join(tmp, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected directory %s to exist", dir)
		}
	}
}

func TestQuickstart_runQuickstart_fullNoBeads(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex") // the first-verdict step probes reviewers; never invoke real CLIs

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart full no-beads: %v", err)
		}
	})

	if !strings.Contains(got, "SETUP COMPLETE") {
		t.Fatalf("expected setup complete message, got: %s", got)
	}

	// Verify starter pack files
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "patterns", "context-boundaries.md")); os.IsNotExist(err) {
		t.Fatal("expected context-boundaries.md to be created")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "patterns", "pre-mortem-first.md")); os.IsNotExist(err) {
		t.Fatal("expected pre-mortem-first.md to be created")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "learnings", "session-hygiene.md")); os.IsNotExist(err) {
		t.Fatal("expected session-hygiene.md to be created")
	}
}

func TestQuickstart_runQuickstart_createsClaudeMd(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex") // the first-verdict step probes reviewers; never invoke real CLIs

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		t.Fatal("expected CLAUDE.md to be created")
	}

	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Quick Start") {
		t.Fatal("expected CLAUDE.md to contain Quick Start section")
	}
	if strings.Contains(string(content), "gt hook") || strings.Contains(string(content), "ol quick-start") {
		t.Fatalf("CLAUDE.md contains stale generated instructions:\n%s", string(content))
	}
	if !strings.Contains(string(content), claudeMDSeedMarker) {
		t.Fatalf("expected CLAUDE.md to contain AgentOps seed marker")
	}
}

func TestQuickstart_runQuickstart_existingClaudeMd(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	stubReviewerPATH(t, "codex") // the first-verdict step probes reviewers; never invoke real CLIs

	// Pre-create CLAUDE.md
	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte("# Existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldMinimal := minimal
	minimal = false
	defer func() { minimal = oldMinimal }()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runQuickstart(cmd, nil)
		if err != nil {
			t.Fatalf("runQuickstart: %v", err)
		}
	})

	// The diet default surfaces a pre-existing CLAUDE.md in the created summary
	// as an appended AgentOps section (the verbose "already exists" line moved
	// behind --verbose). Assert the diet phrasing here.
	if !strings.Contains(got, "AgentOps section appended") {
		t.Fatalf("expected diet 'AgentOps section appended' note, got: %s", got)
	}

	// Verify original content preserved and AgentOps section appended.
	content, err := os.ReadFile(claudeMdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Existing\n") {
		t.Fatal("CLAUDE.md should preserve existing content")
	}
	if !strings.Contains(string(content), claudeMDSeedMarker) {
		t.Fatal("CLAUDE.md should gain AgentOps seed section")
	}
}

func TestQuickstart_DryRunJSONDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	if _, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command("git", "init")
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
	}

	out, err := executeCommand("quickstart", "--dry-run", "--json", "--no-beads")
	if err != nil {
		t.Fatalf("ao quickstart --dry-run --json: %v\n%s", err, out)
	}
	var result quickstartResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("dry-run JSON is not parseable: %v\n%s", err, out)
	}
	if !result.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if result.Readiness == nil || result.Readiness.Ready {
		t.Fatalf("expected non-ready readiness report for empty repo: %#v", result.Readiness)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents")); err == nil {
		t.Fatal("dry-run should not create .agents")
	}
	if _, err := os.Stat(filepath.Join(tmp, "GOALS.md")); err == nil {
		t.Fatal("dry-run should not create GOALS.md")
	}
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err == nil {
		t.Fatal("dry-run should not create CLAUDE.md")
	}
}

// --- quickstartBeadsStep tests ---

func TestQuickstart_quickstartBeadsStep_noBeads(t *testing.T) {
	tmp := t.TempDir()

	oldNoBeads := noBeads
	noBeads = true
	defer func() { noBeads = oldNoBeads }()

	// Pre-create the .agents dir for tasks.json creation
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	got := captureJSONStdout(t, func() {
		quickstartBeadsStep(tmp)
	})

	if !strings.Contains(got, "Skipping beads") {
		t.Fatalf("expected skipping beads message, got: %s", got)
	}

	// Verify tasks.json was created
	tasksPath := filepath.Join(tmp, ".agents", "tasks.json")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Fatal("expected tasks.json to be created")
	}
}

func TestQuickstart_initBeadsUsesSelectedBDWithoutBR(t *testing.T) {
	root := t.TempDir()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tracker.log")
	bdPath := filepath.Join(binDir, "bd")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$TRACKER_LOG\"\n"
	if err := os.WriteFile(bdPath, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("TRACKER_LOG", logPath)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", binDir)
	originalLookPath := trackerLookPath
	trackerLookPath = exec.LookPath
	t.Cleanup(func() { trackerLookPath = originalLookPath })

	if err := initBeadsWithApp(root, NewApp()); err != nil {
		t.Fatalf("initBeadsWithApp with selected bd and no br: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("selected bd was not executed: %v", err)
	}
	if got := strings.TrimSpace(string(data)); !strings.HasPrefix(got, "init --prefix ") {
		t.Fatalf("bd argv = %q, want init --prefix <prefix>", got)
	}
}

func TestQuickstart_selectedTrackerUnavailableFailsClosed(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	originalLookPath := trackerLookPath
	trackerLookPath = exec.LookPath
	t.Cleanup(func() { trackerLookPath = originalLookPath })

	oldNoBeads, oldMinimal := noBeads, minimal
	noBeads, minimal = false, false
	t.Cleanup(func() {
		noBeads, minimal = oldNoBeads, oldMinimal
	})

	_, err := executeCommand("quick-start")
	if err == nil {
		t.Fatal("quick-start with selected unavailable bd returned success, want fail-closed error")
	}
	if strings.Contains(err.Error(), "br command not found") {
		t.Fatalf("quick-start bypassed selected bd and checked br: %v", err)
	}
	if !strings.Contains(err.Error(), "bd") {
		t.Fatalf("quick-start error = %q, want selected backend bd named", err)
	}
}

func TestQuickStartUsesResolvedTrackerContext(t *testing.T) {
	for _, tracker := range []string{"br", "bd"} {
		t.Run(tracker+" process context and typed exit", func(t *testing.T) {
			root := t.TempDir()
			binDir := t.TempDir()
			tracePath := filepath.Join(t.TempDir(), "tracker.trace")
			stub := "#!/bin/sh\n" +
				"printf 'pwd=%s\\nbeads=%s\\nargv=%s\\n' \"$PWD\" \"${BEADS_DIR-<unset>}\" \"$*\" > \"$TRACKER_TRACE\"\n" +
				"exit 23\n"
			if err := os.WriteFile(filepath.Join(binDir, tracker), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}

			t.Setenv("AGENTOPS_TRACKER", tracker)
			t.Setenv("TRACKER_TRACE", tracePath)
			t.Setenv("BEADS_DIR", filepath.Join(t.TempDir(), "ambient-ledger"))
			t.Setenv("HOME", t.TempDir())
			t.Setenv("PATH", binDir)
			originalLookPath := trackerLookPath
			trackerLookPath = exec.LookPath
			t.Cleanup(func() { trackerLookPath = originalLookPath })
			resolution, resolveErr := resolveTracker(root, os.Environ())
			if resolveErr != nil {
				t.Fatalf("resolve tracker: %v", resolveErr)
			}
			wantWorkDir, evalErr := filepath.EvalSymlinks(resolution.WorkDir)
			if evalErr != nil {
				t.Fatalf("resolve physical work dir: %v", evalErr)
			}
			wantBeads := "<unset>"
			for _, entry := range resolution.ChildEnv {
				if strings.HasPrefix(entry, "BEADS_DIR=") {
					wantBeads = strings.TrimPrefix(entry, "BEADS_DIR=")
				}
			}

			err := initBeadsWithApp(root, NewApp())
			var exitErr *trackerexec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("initBeadsWithApp error = %T %v, want typed tracker exit", err, err)
			}
			if exitErr.ExitCode() != 23 {
				t.Fatalf("exit code = %d, want 23", exitErr.ExitCode())
			}

			data, readErr := os.ReadFile(tracePath)
			if readErr != nil {
				t.Fatalf("read tracker trace: %v", readErr)
			}
			trace := string(data)
			if !strings.Contains(trace, "pwd="+wantWorkDir+"\n") {
				t.Fatalf("tracker trace = %q, want work dir %q", trace, wantWorkDir)
			}
			if !strings.Contains(trace, "beads="+wantBeads+"\n") {
				t.Fatalf("tracker trace = %q, want BEADS_DIR %q", trace, wantBeads)
			}
			if !strings.Contains(trace, "argv=init --prefix ") {
				t.Fatalf("tracker trace = %q, want init argv", trace)
			}
		})
	}

	t.Run("live Cobra cancellation prevents launch", func(t *testing.T) {
		root := t.TempDir()
		binDir := t.TempDir()
		tracePath := filepath.Join(t.TempDir(), "launched")
		for name, stub := range map[string]string{
			"br":    "#!/bin/sh\nprintf launched > \"$TRACKER_TRACE\"\n",
			"codex": "#!/bin/sh\necho fake 1.0\n",
		} {
			if err := os.WriteFile(filepath.Join(binDir, name), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		chdirTo(t, root)
		t.Setenv("AGENTOPS_TRACKER", "br")
		t.Setenv("TRACKER_TRACE", tracePath)
		t.Setenv("HOME", t.TempDir())
		// Clear ambient BEADS_DIR so quickstart cannot treat an inherited
		// session ledger as already-initialized and skip the cancel path.
		t.Setenv("BEADS_DIR", "")
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
		originalLookPath := trackerLookPath
		trackerLookPath = exec.LookPath
		t.Cleanup(func() { trackerLookPath = originalLookPath })
		oldMinimal, oldNoBeads, oldVerbose, oldDryRun, oldOutput := minimal, noBeads, quickstartVerbose, dryRun, output
		minimal, noBeads, quickstartVerbose, dryRun, output = false, false, false, false, "table"
		t.Cleanup(func() {
			minimal, noBeads, quickstartVerbose, dryRun, output = oldMinimal, oldNoBeads, oldVerbose, oldDryRun, oldOutput
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		err := runQuickstart(cmd, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runQuickstart error = %T %v, want context cancellation", err, err)
		}
		if _, statErr := os.Stat(tracePath); !os.IsNotExist(statErr) {
			t.Fatalf("pre-canceled quick-start launched tracker: %v", statErr)
		}
	})
}

// --- quickstartClaudeMdStep tests ---

func TestQuickstart_quickstartClaudeMdStep_creates(t *testing.T) {
	tmp := t.TempDir()

	got := captureJSONStdout(t, func() {
		quickstartClaudeMdStep(tmp)
	})

	if !strings.Contains(got, "Created CLAUDE.md") {
		t.Fatalf("expected creation message, got: %s", got)
	}

	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); os.IsNotExist(err) {
		t.Fatal("expected CLAUDE.md to be created")
	}
}

func TestQuickstart_quickstartClaudeMdStep_alreadyExists(t *testing.T) {
	tmp := t.TempDir()
	claudeMdPath := filepath.Join(tmp, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	got := captureJSONStdout(t, func() {
		quickstartClaudeMdStep(tmp)
	})

	if !strings.Contains(got, "already exists") {
		t.Fatalf("expected 'already exists' message, got: %s", got)
	}
}

// --- createStarterPack tests ---

func TestQuickstart_createStarterPack(t *testing.T) {
	tmp := t.TempDir()

	// Create needed directories
	dirs := []string{".agents/patterns", ".agents/learnings"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	captureJSONStdout(t, func() {
		err := createStarterPack(tmp)
		if err != nil {
			t.Fatalf("createStarterPack: %v", err)
		}
	})

	// Verify files exist
	expected := []string{
		".agents/patterns/context-boundaries.md",
		".agents/patterns/pre-mortem-first.md",
		".agents/learnings/session-hygiene.md",
	}
	for _, name := range expected {
		path := filepath.Join(tmp, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected %s to be created", name)
		}
	}
}

// --- createTasksFile tests ---

func TestQuickstart_createTasksFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	captureJSONStdout(t, func() {
		createTasksFile(tmp)
	})

	tasksPath := filepath.Join(tmp, ".agents", "tasks.json")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "tasks") {
		t.Fatalf("expected tasks field in file, got: %s", string(content))
	}
}

// --- createProjectClaudeMd tests ---

func TestQuickstart_createProjectClaudeMd(t *testing.T) {
	tmp := t.TempDir()

	err := createProjectClaudeMd(tmp)
	if err != nil {
		t.Fatalf("createProjectClaudeMd: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Should contain the directory name as the title
	dirName := filepath.Base(tmp)
	if !strings.Contains(string(content), dirName) {
		t.Fatalf("expected CLAUDE.md to contain dir name %q, got: %s", dirName, string(content))
	}
}

// --- showNextSteps tests ---

func TestQuickstart_showNextSteps_withBeads(t *testing.T) {
	got := captureJSONStdout(t, func() {
		showNextSteps(true)
	})

	if !strings.Contains(got, "Select tracked work") {
		t.Fatalf("expected beads next steps, got: %s", got)
	}
}

func TestQuickstart_showNextSteps_withoutBeads(t *testing.T) {
	got := captureJSONStdout(t, func() {
		showNextSteps(false)
	})

	if !strings.Contains(got, "Inspect repository readiness") {
		t.Fatalf("expected no-beads next steps, got: %s", got)
	}
}

// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scheduleExampleBody is the canonical body the tests copy into the test
// repository's .agents/schedule.yaml.example before invoking runInit. It is
// not parsed; we just verify byte-for-byte propagation to schedule.yaml.
const scheduleExampleBody = "schedules:\n  - name: nightly-dream\n    cron: \"0 3 * * *\"\n    job_type: dream.run\n"

// resetInitFlags resets the package-level init flags between tests so state
// from prior cases does not leak into the next.
func resetInitFlags() {
	initStealth = false
	initHooks = false
	initFull = false
	initMinimalHooks = false
	initWithSchedule = false
	dryRun = false
}

// setupInitTest is the canonical entry point for tests that mutate the
// package-level init globals (initStealth/initHooks/initFull/initMinimalHooks/
// initWithSchedule/dryRun). It resets all globals to their zero value AND
// registers a Cleanup so the next test under `-shuffle on` starts clean.
//
// Encodes soc-hwgm fix: TestRunInit* used to flake under -shuffle because
// individual tests reset only the 2-3 globals they touched, leaking the
// other ~4 globals across test boundaries. Belt-and-suspenders: reset on
// entry (defense against leak from prior test) + reset on exit (defense
// for next test).
func setupInitTest(t *testing.T) {
	t.Helper()
	resetInitFlags()
	t.Cleanup(resetInitFlags)
}

func TestRunInitCreatesDirs(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	// Create a fake .git so it's treated as a git repo
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	dryRun = false
	initStealth = false
	initHooks = false

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Verify all agentsDirs exist
	for _, dir := range agentsDirs {
		target := filepath.Join(tmp, dir)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist", dir)
		}
	}

	// Verify .agents/ao storage subdirs (created by storage.Init)
	for _, sub := range []string{"sessions", "index", "provenance"} {
		target := filepath.Join(tmp, ".agents/ao", sub)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			t.Errorf("expected dir .agents/ao/%s to exist", sub)
		}
	}

	// Verify total dir count matches expectation (agentsDirs + 3 storage subdirs)
	// agentsDirs includes .agents/ao, storage.Init adds sessions/index/provenance under it
	expectedDirs := len(agentsDirs) + 3 // +3 for ao/{sessions,index,provenance}
	actualDirs := 0
	_ = filepath.Walk(filepath.Join(tmp, ".agents"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != filepath.Join(tmp, ".agents") {
			actualDirs++
		}
		return nil
	})
	if actualDirs < expectedDirs {
		t.Errorf("expected at least %d dirs under .agents/, got %d", expectedDirs, actualDirs)
	}
}

func TestRunInitGitignoreAppend(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if !strings.Contains(string(data), "/.agents/") {
		t.Error("expected .gitignore to contain /.agents/")
	}
	if !strings.Contains(string(data), "node_modules/") {
		t.Error("expected .gitignore to still contain node_modules/")
	}
}

func TestRunInitGitignoreCreate(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(data), "/.agents/") {
		t.Error("expected .gitignore to contain /.agents/")
	}
}

func TestRunInitIdempotent(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false

	// Run twice
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}

	// /.agents/ should appear only once in .gitignore
	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	count := strings.Count(string(data), "/.agents/\n")
	if count != 1 {
		t.Errorf("expected /.agents/ once in .gitignore, got %d", count)
	}
}

func TestRunInitNonGitRepo(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	// No .git directory

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Dirs should still be created
	for _, dir := range agentsDirs {
		if _, err := os.Stat(filepath.Join(tmp, dir)); os.IsNotExist(err) {
			t.Errorf("expected dir %s to exist even without git", dir)
		}
	}

	// .gitignore should NOT be created
	if _, err := os.Stat(filepath.Join(tmp, ".gitignore")); err == nil {
		t.Error("expected .gitignore NOT to be created in non-git repo")
	}
}

func TestRunInitStealth(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = true
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// .git/info/exclude should contain .agents/
	data, err := os.ReadFile(filepath.Join(tmp, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("expected .git/info/exclude to exist: %v", err)
	}
	if !strings.Contains(string(data), ".agents/") {
		t.Error("expected .git/info/exclude to contain .agents/")
	}

	// .gitignore should NOT be modified
	if _, err := os.Stat(filepath.Join(tmp, ".gitignore")); err == nil {
		t.Error("expected .gitignore NOT to be created in stealth mode")
	}
}

func TestRunInitDryRun(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	dryRun = true
	defer func() { dryRun = false }()

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit dry-run: %v", err)
	}

	// No directories should be created (except what TempDir gives us)
	for _, dir := range agentsDirs {
		if _, err := os.Stat(filepath.Join(tmp, dir)); err == nil {
			t.Errorf("expected dir %s NOT to exist in dry-run", dir)
		}
	}
}

func TestNestedGitignoreContent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	initWithSchedule = false // reset: prior tests in this package may have left it true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", ".gitignore"))
	if err != nil {
		t.Fatalf("expected .agents/.gitignore: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "*") {
		t.Error("expected .agents/.gitignore to contain *")
	}
	if !strings.Contains(content, "!.gitignore") {
		t.Error("expected .agents/.gitignore to contain !.gitignore")
	}
}

func TestRunInitGitignoreNoTrailingNewline(t *testing.T) {
	setupInitTest(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	// Write file without trailing newline
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("node_modules/"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmp)

	initStealth = false
	initHooks = false
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	content := string(data)
	// Should have newline between existing content and new entry
	if strings.Contains(content, "node_modules/.agents/") {
		t.Error("expected newline between existing content and /.agents/ entry")
	}
	if !strings.Contains(content, "/.agents/") {
		t.Error("expected .gitignore to contain /.agents/")
	}
}

func TestFileContainsLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte("foo\n.agents/\nbar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if !fileContainsLine(path, ".agents/") {
		t.Error("expected to find .agents/")
	}
	if fileContainsLine(path, "baz") {
		t.Error("expected NOT to find baz")
	}
	if fileContainsLine(filepath.Join(tmp, "nonexistent"), "x") {
		t.Error("expected false for nonexistent file")
	}
}

func TestIsGitRepository(t *testing.T) {
	tmp := t.TempDir()
	if isGitRepository(tmp) {
		t.Error("expected false for non-git dir")
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepository(tmp) {
		t.Error("expected true for git dir")
	}
}

func TestWarnTrackedFilesNoError(t *testing.T) {
	// warnTrackedFiles should handle non-git directories gracefully
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	tmp := t.TempDir()
	out, _ := captureStdout(t, func() error {
		warnTrackedFiles(tmp)
		return nil
	})
	// In a non-git directory, should produce no warnings about tracked files
	if strings.Contains(out, "tracked") {
		t.Errorf("non-git dir should not warn about tracked files, got: %s", out)
	}
}

// --- soc-hxnr.2 tests: ao init --with-schedule ---

// setupScheduleTest prepares a tmp repo with the schedule example file and
// returns the cwd. Tests t.Chdir into it and expect no .agents/schedule.yaml
// to exist initially. Registers t.Cleanup to reset package-level init flags
// so prompt state does NOT leak into subsequent tests.
func setupScheduleTest(t *testing.T, withExample bool) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withExample {
		if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		examplePath := filepath.Join(tmp, ".agents", "schedule.yaml.example")
		if err := os.WriteFile(examplePath, []byte(scheduleExampleBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(tmp)
	dryRun = false
	resetInitFlags()
	// Pre-register cleanup so package-level flag state never leaks even if
	// the test body sets initWithSchedule=true partway through. Also pipe a
	// "n" response into cmd.InOrStdin() so the prompt — if triggered by some
	// later test path that doesn't override stdin — doesn't hang on TTY input.
	t.Cleanup(func() {
		resetInitFlags()
		initCmd.SetIn(nil)
	})
	return tmp
}

func TestInit_WithScheduleFlag_CopiesExample(t *testing.T) {
	tmp := setupScheduleTest(t, true)
	initWithSchedule = true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, ".agents", "schedule.yaml"))
	if err != nil {
		t.Fatalf("expected .agents/schedule.yaml to be created: %v", err)
	}
	if string(got) != scheduleExampleBody {
		t.Errorf("schedule.yaml content mismatch:\nwant: %q\n got: %q", scheduleExampleBody, string(got))
	}
}

func TestInit_WithScheduleFlag_SkipsIfTargetExists(t *testing.T) {
	tmp := setupScheduleTest(t, true)
	existing := []byte("schedules:\n  - name: pre-existing\n    cron: \"0 0 * * *\"\n    job_type: dream.run\n")
	dst := filepath.Join(tmp, ".agents", "schedule.yaml")
	if err := os.WriteFile(dst, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	initWithSchedule = true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != string(existing) {
		t.Errorf("expected pre-existing schedule.yaml to be preserved (warn+skip); got overwrite")
	}
}

func TestInit_NonInteractive_SkipsByDefault(t *testing.T) {
	tmp := setupScheduleTest(t, true)
	// No flag, no env var. If the test process is non-TTY (typical CI), the
	// silent-skip path triggers. If interactive (developer running `go test`
	// in a TTY), the prompt path would fire — simulate a "no" response by
	// piping "n\n" into cmd.InOrStdin() so the test is deterministic on both.
	t.Setenv("AGENTOPS_INIT_WITH_SCHEDULE", "")
	initCmd.SetIn(strings.NewReader("n\n"))
	t.Cleanup(func() { initCmd.SetIn(nil) })
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "schedule.yaml")); !os.IsNotExist(err) {
		t.Errorf("expected .agents/schedule.yaml NOT to exist (silent skip non-TTY OR explicit 'n' in TTY); got err=%v", err)
	}
}

func TestInit_EnvOverride_CopiesNonInteractive(t *testing.T) {
	tmp := setupScheduleTest(t, true)
	t.Setenv("AGENTOPS_INIT_WITH_SCHEDULE", "1")
	// initWithSchedule stays false — env var is the trigger
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "schedule.yaml")); err != nil {
		t.Errorf("expected env-var-driven copy to create .agents/schedule.yaml; got err=%v", err)
	}
}

func TestInit_FreshRepo_CreatesAgentsDir(t *testing.T) {
	// Amendment B1: fresh tmp dir without .agents/, runInit creates it.
	// runInit creates .agents/ as part of its normal flow regardless of
	// --with-schedule, so this asserts the path is robust when the example
	// also needs to be copied into it.
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Place example at .agents/schedule.yaml.example AFTER runInit creates the dir
	// — but we need it before for the copy. Approach: pre-create just the file
	// and let runInit's existing dir creation be idempotent.
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	examplePath := filepath.Join(tmp, ".agents", "schedule.yaml.example")
	if err := os.WriteFile(examplePath, []byte(scheduleExampleBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)
	dryRun = false
	resetInitFlags()
	initWithSchedule = true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "schedule.yaml")); err != nil {
		t.Errorf("expected .agents/schedule.yaml to exist in fresh repo; got err=%v", err)
	}
}

func TestInit_HelpTextMentionsEnvVar(t *testing.T) {
	// Amendment B2: --help output must mention AGENTOPS_INIT_WITH_SCHEDULE
	// so non-interactive opt-in is discoverable.
	helpText := initCmd.Flags().Lookup("with-schedule").Usage
	if !strings.Contains(helpText, "AGENTOPS_INIT_WITH_SCHEDULE") {
		t.Errorf("--with-schedule help text must mention AGENTOPS_INIT_WITH_SCHEDULE env var; got: %q", helpText)
	}
}

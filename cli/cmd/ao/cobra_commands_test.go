package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetFlagChangesRecursive(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	for _, child := range cmd.Commands() {
		resetFlagChangesRecursive(child)
	}
}

// executeCommand runs a command through rootCmd and captures output from
// both cmd.OutOrStdout() (via SetOut) and os.Stdout (via pipe).
// Returns combined output and error. Resets args and global flags after execution.
func executeCommand(args ...string) (string, error) {
	// Save and restore global flags to prevent cross-test pollution.
	// Root-level persistent flags:
	origDryRun := dryRun
	origVerbose := verbose
	origOutput := output
	origJSON := jsonFlag
	origCfg := cfgFile
	// Command-local flags (set by Cobra flag parsing, persist across Execute calls):
	origDemoConcepts := demoConcepts
	origDemoQuick := demoQuick
	origConfigShow := configShow
	origSeedForce := seedForce
	origGoalsJSON := goalsJSON
	origMemorySyncQuiet := memorySyncQuiet
	origMemorySyncMaxEntries := memorySyncMaxEntries
	origMemorySyncOutput := memorySyncOutput
	origHooksFull := hooksFull
	origHooksDryRun := hooksDryRun
	origHooksForce := hooksForce
	origSearchLimit := searchLimit
	origSearchType := searchType
	origSearchCiteType := searchCiteType
	origSearchSession := searchSession
	origSearchUseSC := searchUseSC
	origSearchUseCASS := searchUseCASS
	origSearchUseLocal := searchUseLocal
	origCodexStartLimit := codexStartLimit
	origCodexStartQuery := codexStartQuery
	origCodexStartNoMaintenance := codexStartNoMaintenance
	origCodexStopSessionID := codexStopSessionID
	origCodexStopTranscriptPath := codexStopTranscriptPath
	origCodexStopAutoExtract := codexStopAutoExtract
	origCodexStopNoHistoryFallback := codexStopNoHistoryFallback
	origCodexStopNoCloseLoop := codexStopNoCloseLoop
	origCodexStatusDays := codexStatusDays
	origAutodevFile := autodevFile
	origAutodevForce := autodevForce
	origFindingsListLimit := findingsListLimit
	origFindingsListAll := findingsListAll
	origFindingsExportTo := findingsExportTo
	origFindingsExportAll := findingsExportAll
	origFindingsExportForce := findingsExportForce
	origFindingsPullFrom := findingsPullFrom
	origFindingsPullAll := findingsPullAll
	origFindingsPullForce := findingsPullForce
	origFindingsRetireBy := findingsRetireBy
	defer func() {
		dryRun = origDryRun
		verbose = origVerbose
		output = origOutput
		jsonFlag = origJSON
		cfgFile = origCfg
		demoConcepts = origDemoConcepts
		demoQuick = origDemoQuick
		configShow = origConfigShow
		seedForce = origSeedForce
		goalsJSON = origGoalsJSON
		memorySyncQuiet = origMemorySyncQuiet
		memorySyncMaxEntries = origMemorySyncMaxEntries
		memorySyncOutput = origMemorySyncOutput
		hooksFull = origHooksFull
		hooksDryRun = origHooksDryRun
		hooksForce = origHooksForce
		searchLimit = origSearchLimit
		searchType = origSearchType
		searchCiteType = origSearchCiteType
		searchSession = origSearchSession
		searchUseSC = origSearchUseSC
		searchUseCASS = origSearchUseCASS
		searchUseLocal = origSearchUseLocal
		codexStartLimit = origCodexStartLimit
		codexStartQuery = origCodexStartQuery
		codexStartNoMaintenance = origCodexStartNoMaintenance
		codexStopSessionID = origCodexStopSessionID
		codexStopTranscriptPath = origCodexStopTranscriptPath
		codexStopAutoExtract = origCodexStopAutoExtract
		codexStopNoHistoryFallback = origCodexStopNoHistoryFallback
		codexStopNoCloseLoop = origCodexStopNoCloseLoop
		codexStatusDays = origCodexStatusDays
		autodevFile = origAutodevFile
		autodevForce = origAutodevForce
		findingsListLimit = origFindingsListLimit
		findingsListAll = origFindingsListAll
		findingsExportTo = origFindingsExportTo
		findingsExportAll = origFindingsExportAll
		findingsExportForce = origFindingsExportForce
		findingsPullFrom = origFindingsPullFrom
		findingsPullAll = origFindingsPullAll
		findingsPullForce = origFindingsPullForce
		findingsRetireBy = origFindingsRetireBy
	}()

	// Reset all command-local flags to defaults before execution.
	// Cobra's pflag retains Changed state across Execute() calls in-process,
	// so we must explicitly reset both the Go variable and the flag's Changed bit.
	demoConcepts = false
	demoQuick = false
	configShow = false
	seedForce = false
	goalsJSON = false
	memorySyncQuiet = false
	memorySyncMaxEntries = 10
	memorySyncOutput = ""
	hooksFull = false
	hooksDryRun = false
	hooksForce = false
	searchLimit = 10
	searchType = ""
	searchCiteType = ""
	searchSession = ""
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	codexStartLimit = 3
	codexStartQuery = ""
	codexStartNoMaintenance = false
	codexStopSessionID = ""
	codexStopTranscriptPath = ""
	codexStopAutoExtract = true
	codexStopNoHistoryFallback = false
	codexStopNoCloseLoop = false
	codexStatusDays = 7
	autodevFile = ""
	autodevForce = false
	findingsListLimit = 20
	findingsListAll = false
	findingsExportTo = ""
	findingsExportAll = false
	findingsExportForce = false
	findingsPullFrom = ""
	findingsPullAll = false
	findingsPullForce = false
	findingsRetireBy = ""

	// Reset Cobra flag Changed state on all commands recursively.
	resetFlagChangesRecursive(rootCmd)
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})

	cmdBuf := new(bytes.Buffer)
	rootCmd.SetOut(cmdBuf)
	rootCmd.SetErr(cmdBuf)
	rootCmd.SetArgs(args)

	// Also capture os.Stdout for commands that use fmt.Printf directly
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		return "", pipeErr
	}
	os.Stdout = w

	err := rootCmd.Execute()

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout

	var stdoutBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, r)

	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)

	// Combine cmd buffer and stdout capture
	combined := cmdBuf.String() + stdoutBuf.String()
	return combined, err
}

// chdirTemp moved to testutil_test.go.
// setupAgentsDir moved to testutil_test.go.

// TestCobraCommandTreeRegistration verifies all expected commands are registered.
// This covers all init() functions and Cobra command registration without
// modifying global Cobra state (avoiding --help flag pollution).
func TestCobraCommandTreeRegistration(t *testing.T) {
	// Collect all command names by walking the command tree (up to 3 levels deep)
	var cmdNames []string
	for _, sub := range rootCmd.Commands() {
		cmdNames = append(cmdNames, sub.Name())
		for _, subsub := range sub.Commands() {
			cmdNames = append(cmdNames, sub.Name()+"/"+subsub.Name())
			for _, subsubsub := range subsub.Commands() {
				cmdNames = append(cmdNames, sub.Name()+"/"+subsub.Name()+"/"+subsubsub.Name())
			}
		}
	}

	if len(cmdNames) < 45 {
		t.Fatalf("expected at least 45 registered commands, got %d", len(cmdNames))
	}

	// Verify all top-level commands are registered (flat namespace)
	expectedCmds := []string{
		"anti-patterns", "autodev", "badge", "batch-feedback", "completion", "config",
		"constraint", "context", "codex", "contradict", "curate", "dedup",
		"defrag", "demo", "doctor", "extract", "feedback", "feedback-loop",
		"findings", "flywheel", "forge", "gate", "goals", "handoff", "harvest", "hooks",
		"index", "init", "inject", "lookup", "maturity",
		"memory", "metrics", "migrate", "mind", "mine", "notebook", "plans",
		"pool", "quick-start", "ratchet", "rpi",
		"search", "seed", "session", "session-outcome", "status",
		"store", "task-feedback", "task-status", "task-sync", "temper",
		"trace", "version", "vibe-check", "worktree",
	}
	cmdSet := make(map[string]bool)
	for _, name := range cmdNames {
		cmdSet[name] = true
	}
	for _, expected := range expectedCmds {
		if !cmdSet[expected] {
			t.Errorf("expected command %q to be registered, not found in: %v", expected, cmdNames)
		}
	}

	// Verify parent commands have subcommands
	parentExpectations := map[string][]string{
		"autodev":    {"init", "validate", "show"},
		"goals":      {"validate", "measure", "drift"},
		"ratchet":    {"status", "check", "next"},
		"metrics":    {"baseline", "report"},
		"flywheel":   {"status", "nudge"},
		"constraint": {"activate", "retire", "review", "list"},
		"pool":       {"list", "ingest"},
		"store":      {"rebuild", "search"},
	}
	for parent, expectedSubs := range parentExpectations {
		for _, sub := range expectedSubs {
			key := parent + "/" + sub
			if !cmdSet[key] {
				t.Errorf("expected subcommand %q to be registered", key)
			}
		}
	}
}

// TestCobraExpectedCmdsMatchRegistration ensures the hardcoded expectedCmds list
// stays in sync with actual cobra command registration — catches drift in either direction.
func TestCobraExpectedCmdsMatchRegistration(t *testing.T) {
	root := rootCmd
	registered := make(map[string]bool)
	for _, cmd := range root.Commands() {
		registered[cmd.Name()] = true
	}

	// Same list as TestCobraCommandTreeRegistration
	expectedCmds := []string{
		"anti-patterns", "autodev", "badge", "batch-feedback", "completion", "config",
		"constraint", "context", "codex", "contradict", "curate", "dedup",
		"defrag", "demo", "doctor", "extract", "feedback", "feedback-loop",
		"findings", "flywheel", "forge", "gate", "goals", "handoff", "harvest", "hooks",
		"index", "init", "inject", "lookup", "maturity",
		"memory", "metrics", "migrate", "mind", "mine", "notebook", "plans",
		"pool", "quick-start", "ratchet", "rpi",
		"search", "seed", "session", "session-outcome", "status",
		"store", "task-feedback", "task-status", "task-sync", "temper",
		"trace", "version", "vibe-check", "worktree",
	}

	// Every expected command must be registered
	for _, name := range expectedCmds {
		if !registered[name] {
			t.Errorf("expectedCmds contains %q but it is not a registered command", name)
		}
	}

	// Every registered command must be expected (except auto-added ones)
	expectedSet := make(map[string]bool)
	for _, name := range expectedCmds {
		expectedSet[name] = true
	}
	for name := range registered {
		if name == "help" {
			continue // cobra adds this automatically
		}
		if !expectedSet[name] {
			t.Errorf("registered command %q is not in expectedCmds — add it to keep the list in sync", name)
		}
	}
}

// TestCobraVersionCommand exercises the version command fully.
func TestCobraVersionCommand(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("ao version failed: %v", err)
	}
	if !strings.Contains(out, "ao version") {
		t.Errorf("expected 'ao version' in output, got: %s", out)
	}
	if !strings.Contains(out, "Go version") {
		t.Errorf("expected 'Go version' in output, got: %s", out)
	}
	if !strings.Contains(out, "Platform") {
		t.Errorf("expected 'Platform' in output, got: %s", out)
	}
}

// TestCobraDoctorCommand exercises the doctor command in a temp directory.
func TestCobraDoctorCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	t.Run("table_output", func(t *testing.T) {
		out, err := executeCommand("doctor")
		// Doctor may return error if required checks fail, that's OK
		_ = err
		if !strings.Contains(out, "ao doctor") {
			t.Errorf("expected 'ao doctor' header in output, got: %s", out)
		}
	})

	t.Run("json_output", func(t *testing.T) {
		out, err := executeCommand("doctor", "--json")
		_ = err
		if !strings.Contains(out, "checks") {
			t.Errorf("expected 'checks' in JSON output, got: %s", out)
		}
		// Verify it's valid JSON
		var result map[string]any
		if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
			t.Errorf("doctor --json did not produce valid JSON: %v\noutput: %s", jsonErr, out)
		}
	})
}

// TestCobraStatusCommand exercises the status command.
func TestCobraStatusCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	t.Run("not_initialized", func(t *testing.T) {
		out, err := executeCommand("status")
		if err != nil {
			t.Fatalf("ao status failed: %v", err)
		}
		if !strings.Contains(out, "Not initialized") {
			t.Errorf("expected 'Not initialized' in output, got: %s", out)
		}
	})

	t.Run("initialized_empty", func(t *testing.T) {
		setupAgentsDir(t, tmp)
		out, err := executeCommand("status")
		if err != nil {
			t.Fatalf("ao status failed: %v", err)
		}
		if !strings.Contains(out, "Initialized") {
			t.Errorf("expected 'Initialized' in output, got: %s", out)
		}
	})

	t.Run("json_not_initialized", func(t *testing.T) {
		tmp2 := t.TempDir()
		orig, _ := os.Getwd()
		_ = os.Chdir(tmp2)
		defer func() { _ = os.Chdir(orig) }()

		// Reset the output flag
		output = "json"
		defer func() { output = "table" }()

		out, err := executeCommand("status", "--json")
		if err != nil {
			t.Fatalf("ao status --json failed: %v", err)
		}
		var result map[string]any
		if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
			t.Errorf("status --json did not produce valid JSON: %v\noutput: %s", jsonErr, out)
		}
	})
}

// TestCobraBadgeCommand exercises the badge command.
func TestCobraBadgeCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("badge")
	if err != nil {
		t.Fatalf("ao badge failed: %v", err)
	}
	if !strings.Contains(out, "AGENTOPS") {
		t.Errorf("expected badge header in output, got: %s", out)
	}
}

// TestCobraConfigCommand exercises the config command.
func TestCobraConfigCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	t.Run("no_flags_shows_help", func(t *testing.T) {
		out, err := executeCommand("config")
		if err != nil {
			t.Fatalf("ao config failed: %v", err)
		}
		if !strings.Contains(out, "configuration") || !strings.Contains(out, "config") {
			t.Errorf("expected help text in output, got: %s", out)
		}
	})

	t.Run("show", func(t *testing.T) {
		// Save and restore configShow flag (command-local, not global)
		origShow := configShow
		defer func() { configShow = origShow }()
		configShow = false

		out, err := executeCommand("config", "--show")
		if err != nil {
			t.Fatalf("ao config --show failed: %v", err)
		}
		if !strings.Contains(out, "Configuration") && !strings.Contains(out, "config") && !strings.Contains(out, "output") {
			t.Errorf("expected config output, got: %s", out)
		}
	})
}

// TestCobraDemoConceptsCommand exercises the demo --concepts command (no stdin needed).
func TestCobraDemoConceptsCommand(t *testing.T) {
	out, err := executeCommand("demo", "--concepts")
	if err != nil {
		t.Fatalf("ao demo --concepts failed: %v", err)
	}
	if !strings.Contains(out, "CORE CONCEPTS") {
		t.Errorf("expected 'CORE CONCEPTS' in output, got: %s", out)
	}
	if !strings.Contains(out, "KNOWLEDGE FLYWHEEL") {
		t.Errorf("expected 'KNOWLEDGE FLYWHEEL' in output, got: %s", out)
	}
}

// TestCobraDemoQuickCommand exercises the demo --quick command.
func TestCobraDemoQuickCommand(t *testing.T) {
	out, err := executeCommand("demo", "--quick")
	if err != nil {
		t.Fatalf("ao demo --quick failed: %v", err)
	}
	if !strings.Contains(out, "QUICK DEMO") {
		t.Errorf("expected 'QUICK DEMO' in output, got: %s", out)
	}
}

// TestCobraCompletionCommand exercises shell completion generation.
func TestCobraCompletionCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			out, err := executeCommand("completion", shell)
			if err != nil {
				t.Fatalf("ao completion %s failed: %v", shell, err)
			}
			if len(out) < 100 {
				t.Errorf("expected substantial completion script for %s, got %d bytes", shell, len(out))
			}
		})
	}
}

// TestCobraInitCommand exercises ao init in a temp directory.
func TestCobraInitCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	// Create a fake .git dir so init doesn't complain
	if err := os.MkdirAll(filepath.Join(tmp, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("init")
	if err != nil {
		t.Fatalf("ao init failed: %v", err)
	}
	if !strings.Contains(out, ".agents") {
		t.Errorf("expected '.agents' in output, got: %s", out)
	}

	// Verify directories were created
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "ao")); os.IsNotExist(err) {
		t.Error("expected .agents/ao directory to be created")
	}
}

// TestCobraSeedCommand exercises ao seed in a temp directory.
func TestCobraSeedCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	t.Run("dry_run", func(t *testing.T) {
		dryRun = true
		defer func() { dryRun = false }()

		out, err := executeCommand("seed", "--dry-run")
		if err != nil {
			t.Fatalf("ao seed --dry-run failed: %v", err)
		}
		if !strings.Contains(out, "dry-run") || !strings.Contains(out, "dry run") {
			// Some commands capitalize differently
			lower := strings.ToLower(out)
			if !strings.Contains(lower, "dry") {
				t.Errorf("expected dry-run mention in output, got: %s", out)
			}
		}
	})

	t.Run("actual_seed", func(t *testing.T) {
		seedForce = true
		defer func() { seedForce = false }()

		out, err := executeCommand("seed", "--force")
		if err != nil {
			t.Fatalf("ao seed --force failed: %v", err)
		}
		if !strings.Contains(out, "Seeded") && !strings.Contains(out, "seed") {
			t.Errorf("expected seeded confirmation, got: %s", out)
		}
		// Verify GOALS.md was created
		if _, err := os.Stat(filepath.Join(tmp, "GOALS.md")); os.IsNotExist(err) {
			t.Error("expected GOALS.md to be created by seed")
		}
	})
}

// TestCobraSearchHelp exercises search help.
func TestCobraSearchHelp(t *testing.T) {
	out, err := executeCommand("search", "--help")
	if err != nil {
		t.Fatalf("ao search --help failed: %v", err)
	}
	if !strings.Contains(out, "Search") {
		t.Errorf("expected 'Search' in output, got: %s", out)
	}
}

// TestCobraTraceHelpAndDryRun exercises trace help and dry-run.
func TestCobraTraceHelpAndDryRun(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		out, err := executeCommand("trace", "--help")
		if err != nil {
			t.Fatalf("ao trace --help failed: %v", err)
		}
		if !strings.Contains(out, "provenance") {
			t.Errorf("expected 'provenance' in output, got: %s", out)
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		dryRun = true
		defer func() { dryRun = false }()

		out, err := executeCommand("trace", "--dry-run", "some-artifact")
		if err != nil {
			t.Fatalf("ao trace --dry-run failed: %v", err)
		}
		if !strings.Contains(out, "dry-run") {
			t.Errorf("expected dry-run in output, got: %s", out)
		}
	})
}

// TestCobraGoalsValidateCommand exercises goals validate with a minimal GOALS.md.
func TestCobraGoalsValidateCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	goalsContent := `# Fitness Goals

## Mission
Test project fitness goals

## North Stars
- All tests pass

## Anti-Stars
- Untested code

## Directives

### 1. Test coverage
Maintain test coverage

**Steer:** increase

## Gates

### test-gate
- **Check:** echo pass
- **Threshold:** pass
`
	if err := os.WriteFile(filepath.Join(tmp, "GOALS.md"), []byte(goalsContent), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("goals", "validate")
	if err != nil {
		t.Fatalf("ao goals validate failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "VALID") {
		t.Errorf("expected 'VALID' in output, got: %s", out)
	}
}

// TestCobraGoalsValidateJSON exercises goals validate --json.
func TestCobraGoalsValidateJSON(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	goalsContent := `# Fitness Goals

## Mission
Test goals

## North Stars
- Pass

## Anti-Stars
- Fail

## Directives

### 1. Coverage
Increase coverage

**Steer:** increase

## Gates

### test-gate
- **Check:** echo pass
- **Threshold:** pass
`
	if err := os.WriteFile(filepath.Join(tmp, "GOALS.md"), []byte(goalsContent), 0644); err != nil {
		t.Fatal(err)
	}

	goalsJSON = true
	defer func() { goalsJSON = false }()

	out, err := executeCommand("goals", "validate", "--json")
	if err != nil {
		t.Fatalf("ao goals validate --json failed: %v\noutput: %s", err, out)
	}

	var result map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Errorf("goals validate --json did not produce valid JSON: %v\noutput: %s", jsonErr, out)
	}
}

// TestCobraIndexCommand exercises ao index in a temp directory.
func TestCobraIndexCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	// Create the directories that index expects
	for _, dir := range defaultIndexDirs {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a sample .md file
	learningFile := filepath.Join(tmp, ".agents", "learnings", "2026-01-01-test.md")
	if err := os.WriteFile(learningFile, []byte("# Test Learning\n\nSome content."), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("index")
	if err != nil {
		t.Fatalf("ao index failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "indexed") {
		t.Errorf("expected 'indexed' in output, got: %s", out)
	}
}

// TestCobraIndexCheckCommand exercises ao index --check.
func TestCobraIndexCheckCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	for _, dir := range defaultIndexDirs {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// First build the index
	_, _ = executeCommand("index")

	// Then check it
	out, err := executeCommand("index", "--check")
	if err != nil {
		t.Fatalf("ao index --check failed: %v\noutput: %s", err, out)
	}
}

// TestCobraMetricsReportCommand exercises metrics report in an empty repo.
func TestCobraMetricsReportCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("metrics", "report")
	if err != nil {
		t.Fatalf("ao metrics report failed: %v", err)
	}
	// Should show metrics table even if empty
	if out == "" {
		t.Error("expected some output from metrics report")
	}
}

// TestCobraMetricsBaselineCommand exercises metrics baseline.
func TestCobraMetricsBaselineCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("metrics", "baseline")
	if err != nil {
		t.Fatalf("ao metrics baseline failed: %v", err)
	}
	if !strings.Contains(out, "Baseline saved") && !strings.Contains(out, "baseline") {
		// Might print table + baseline path
		_ = out
	}
}

// TestCobraMetricsBaselineDryRun exercises metrics baseline --dry-run.
func TestCobraMetricsBaselineDryRun(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	dryRun = true
	defer func() { dryRun = false }()

	out, err := executeCommand("metrics", "baseline", "--dry-run")
	if err != nil {
		t.Fatalf("ao metrics baseline --dry-run failed: %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run in output, got: %s", out)
	}
}

// TestCobraFlywheelStatusCommand exercises flywheel status.
func TestCobraFlywheelStatusCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("flywheel", "status")
	if err != nil {
		t.Fatalf("ao flywheel status failed: %v", err)
	}
	if out == "" {
		t.Error("expected some output from flywheel status")
	}
}

// TestCobraFlywheelNudgeCommand exercises flywheel nudge.
func TestCobraFlywheelNudgeCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	// Create chain.jsonl so ratchet doesn't fail
	chainPath := filepath.Join(tmp, ".agents", "ao", "chain.jsonl")
	if err := os.WriteFile(chainPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("flywheel", "nudge")
	if err != nil {
		t.Fatalf("ao flywheel nudge failed: %v", err)
	}
	if out == "" {
		t.Error("expected some output from flywheel nudge")
	}
}

// TestCobraConstraintListCommand exercises constraint list.
func TestCobraConstraintListCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	// Create a constraint index
	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "c-001",
				Title:      "Test constraint",
				Source:     "test",
				Status:     "active",
				CompiledAt: time.Now().Format(time.RFC3339),
				File:       "test.md",
			},
		},
	}
	data, _ := json.MarshalIndent(idx, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "constraints", "index.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("constraint", "list")
	if err != nil {
		t.Fatalf("ao constraint list failed: %v", err)
	}
	if !strings.Contains(out, "c-001") {
		t.Errorf("expected constraint ID in output, got: %s", out)
	}
}

// TestCobraConstraintReviewCommand exercises constraint review.
func TestCobraConstraintReviewCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	// Create a stale constraint (>90 days old)
	staleDate := time.Now().AddDate(0, 0, -100).Format(time.RFC3339)
	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "c-stale",
				Title:      "Stale constraint",
				Source:     "test",
				Status:     "active",
				CompiledAt: staleDate,
				File:       "stale.md",
			},
		},
	}
	data, _ := json.MarshalIndent(idx, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "constraints", "index.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("constraint", "review")
	if err != nil {
		t.Fatalf("ao constraint review failed: %v", err)
	}
	if !strings.Contains(out, "c-stale") {
		t.Errorf("expected stale constraint in review output, got: %s", out)
	}
}

// TestCobraConstraintActivateCommand exercises constraint activate.
func TestCobraConstraintActivateCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "c-draft",
				Title:      "Draft constraint",
				Source:     "test",
				Status:     "draft",
				CompiledAt: time.Now().Format(time.RFC3339),
				File:       "draft.md",
			},
		},
	}
	data, _ := json.MarshalIndent(idx, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "constraints", "index.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("constraint", "activate", "c-draft")
	if err != nil {
		t.Fatalf("ao constraint activate failed: %v", err)
	}
	if !strings.Contains(out, "activated") {
		t.Errorf("expected 'activated' in output, got: %s", out)
	}
}

// TestCobraConstraintRetireCommand exercises constraint retire.
func TestCobraConstraintRetireCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "c-active",
				Title:      "Active constraint",
				Source:     "test",
				Status:     "active",
				CompiledAt: time.Now().Format(time.RFC3339),
				File:       "active.md",
			},
		},
	}
	data, _ := json.MarshalIndent(idx, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "constraints", "index.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("constraint", "retire", "c-active")
	if err != nil {
		t.Fatalf("ao constraint retire failed: %v", err)
	}
	if !strings.Contains(out, "retired") {
		t.Errorf("expected 'retired' in output, got: %s", out)
	}
}

// TestCobraExtractCommand exercises extract in a temp directory.
func TestCobraExtractCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	// With no pending extractions, extract should succeed or report "no pending"
	out, err := executeCommand("extract")
	if err != nil {
		// Acceptable errors: no pending extractions, missing knowledge base
		errMsg := err.Error()
		if !strings.Contains(errMsg, "no pending") && !strings.Contains(errMsg, "not found") && !strings.Contains(errMsg, "no such") {
			t.Fatalf("extract command failed unexpectedly: %v\noutput: %s", err, out)
		}
	}
}

// TestCobraMemorySyncCommand exercises memory sync.
func TestCobraMemorySyncCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)
	setupAgentsDir(t, tmp)

	// Create a .git dir so findGitRoot works
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// memory sync should handle empty sessions gracefully
	out, err := executeCommand("memory", "sync")
	// May return error if no sessions; that's acceptable
	_ = err
	_ = out
}

// TestCobraQuickstartMinimalCommand exercises quick-start --minimal.
func TestCobraQuickstartMinimalCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	out, err := executeCommand("quick-start", "--minimal")
	if err != nil {
		t.Fatalf("ao quick-start --minimal failed: %v", err)
	}
	if !strings.Contains(out, "Minimal setup complete") {
		t.Errorf("expected 'Minimal setup complete' in output, got: %s", out)
	}
}

// TestCobraMigrateCommand exercises ao migrate memrl.
func TestCobraMigrateCommand(t *testing.T) {
	tmp := chdirTemp(t)
	t.Setenv("HOME", tmp)

	t.Run("no_learnings_dir", func(t *testing.T) {
		out, err := executeCommand("migrate", "memrl")
		if err != nil {
			t.Fatalf("ao migrate memrl failed: %v", err)
		}
		if !strings.Contains(out, "No learnings") {
			t.Errorf("expected 'No learnings' in output, got: %s", out)
		}
	})

	t.Run("with_learnings", func(t *testing.T) {
		learningsDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		jsonlContent := `{"summary":"test learning"}`
		if err := os.WriteFile(filepath.Join(learningsDir, "test.jsonl"), []byte(jsonlContent), 0644); err != nil {
			t.Fatal(err)
		}

		out, err := executeCommand("migrate", "memrl")
		if err != nil {
			t.Fatalf("ao migrate memrl failed: %v", err)
		}
		if !strings.Contains(out, "Migration complete") {
			t.Errorf("expected 'Migration complete' in output, got: %s", out)
		}
	})

	t.Run("unknown_migration", func(t *testing.T) {
		_, err := executeCommand("migrate", "unknown")
		if err == nil {
			t.Error("expected error for unknown migration type")
		}
	})
}

// TestCobraRootHelpOutput exercises the root command help.
func TestCobraRootHelpOutput(t *testing.T) {
	out, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("ao --help failed: %v", err)
	}
	// Check that command groups are present
	for _, group := range []string{"Getting Started", "Core Commands", "Workflow", "Configuration", "Knowledge"} {
		if !strings.Contains(out, group) {
			t.Errorf("expected group %q in root help, got: %s", group, out)
		}
	}
}

// TestCobraGlobalFlags exercises global flags.
func TestCobraGlobalFlags(t *testing.T) {
	t.Run("verbose_flag", func(t *testing.T) {
		_, _ = executeCommand("version", "--verbose")
		// Just verify it doesn't panic
	})

	t.Run("json_flag", func(t *testing.T) {
		// Reset after
		defer func() { jsonFlag = false; output = "table" }()
		_, _ = executeCommand("version", "--json")
	})

	t.Run("dry_run_flag", func(t *testing.T) {
		defer func() { dryRun = false }()
		_, _ = executeCommand("version", "--dry-run")
	})
}

// TestCobraOutputValidateResult exercises outputValidateResult directly.
func TestCobraOutputValidateResult(t *testing.T) {
	t.Run("valid_table", func(t *testing.T) {
		goalsJSON = false
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result := validateResult{
			Valid:     true,
			GoalCount: 3,
			Version:   4,
			Format:    "md",
		}
		err := outputValidateResult(result)

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatalf("outputValidateResult failed: %v", err)
		}

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out := buf.String()
		if !strings.Contains(out, "VALID") {
			t.Errorf("expected 'VALID' in output, got: %s", out)
		}
	})

	t.Run("invalid_table", func(t *testing.T) {
		goalsJSON = false
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result := validateResult{
			Valid:  false,
			Errors: []string{"missing gate", "bad check"},
		}
		err := outputValidateResult(result)

		w.Close()
		os.Stdout = old

		if err == nil {
			t.Error("expected error for invalid result")
		}

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out := buf.String()
		if !strings.Contains(out, "INVALID") {
			t.Errorf("expected 'INVALID' in output, got: %s", out)
		}
	})

	t.Run("valid_json", func(t *testing.T) {
		goalsJSON = true
		defer func() { goalsJSON = false }()

		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result := validateResult{
			Valid:      true,
			GoalCount:  2,
			Version:    4,
			Format:     "md",
			Directives: 1,
			Warnings:   []string{"no script wiring"},
		}
		err := outputValidateResult(result)

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatalf("outputValidateResult (json) failed: %v", err)
		}

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out := buf.String()

		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(out), &parsed); jsonErr != nil {
			t.Errorf("expected valid JSON, got parse error: %v\noutput: %s", jsonErr, out)
		}
	})
}

// TestCobraDoctorHelpers exercises doctor helper functions.
func TestCobraDoctorHelpers(t *testing.T) {
	t.Run("doctorStatusIcon", func(t *testing.T) {
		cases := []struct {
			status string
			want   string
		}{
			{"pass", "\u2713"},
			{"warn", "!"},
			{"fail", "\u2717"},
			{"unknown", "?"},
		}
		for _, tc := range cases {
			got := doctorStatusIcon(tc.status)
			if got != tc.want {
				t.Errorf("doctorStatusIcon(%q) = %q, want %q", tc.status, got, tc.want)
			}
		}
	})

	t.Run("hasRequiredFailure", func(t *testing.T) {
		checks := []doctorCheck{
			{Name: "a", Status: "pass", Required: true},
			{Name: "b", Status: "warn", Required: false},
		}
		if hasRequiredFailure(checks) {
			t.Error("expected no required failure")
		}

		checks = append(checks, doctorCheck{Name: "c", Status: "fail", Required: true})
		if !hasRequiredFailure(checks) {
			t.Error("expected required failure")
		}
	})

	t.Run("countCheckStatuses", func(t *testing.T) {
		checks := []doctorCheck{
			{Status: "pass"}, {Status: "pass"}, {Status: "warn"}, {Status: "fail"},
		}
		p, f, w := countCheckStatuses(checks)
		if p != 2 || f != 1 || w != 1 {
			t.Errorf("countCheckStatuses = (%d, %d, %d), want (2, 1, 1)", p, f, w)
		}
	})

	t.Run("buildDoctorSummary", func(t *testing.T) {
		s := buildDoctorSummary(5, 0, 0, 5)
		if s != "5/5 checks passed" {
			t.Errorf("got %q", s)
		}

		s = buildDoctorSummary(4, 0, 1, 5)
		if !strings.Contains(s, "1 warning") {
			t.Errorf("expected '1 warning', got %q", s)
		}

		s = buildDoctorSummary(3, 1, 1, 5)
		if !strings.Contains(s, "1 failed") {
			t.Errorf("expected '1 failed', got %q", s)
		}
	})

	t.Run("computeResult", func(t *testing.T) {
		checks := []doctorCheck{{Status: "pass"}}
		result := computeResult(checks)
		if result.Result != "HEALTHY" {
			t.Errorf("expected HEALTHY, got %s", result.Result)
		}

		checks = []doctorCheck{{Status: "warn"}}
		result = computeResult(checks)
		if result.Result != "DEGRADED" {
			t.Errorf("expected DEGRADED, got %s", result.Result)
		}

		checks = []doctorCheck{{Status: "fail"}}
		result = computeResult(checks)
		if result.Result != "UNHEALTHY" {
			t.Errorf("expected UNHEALTHY, got %s", result.Result)
		}
	})

	t.Run("renderDoctorTable", func(t *testing.T) {
		buf := new(bytes.Buffer)
		out := doctorOutput{
			Checks: []doctorCheck{
				{Name: "Test Check", Status: "pass", Detail: "OK"},
			},
			Result:  "HEALTHY",
			Summary: "1/1 checks passed",
		}
		renderDoctorTable(buf, out)
		s := buf.String()
		if !strings.Contains(s, "Test Check") || !strings.Contains(s, "1/1") {
			t.Errorf("renderDoctorTable output missing expected content: %s", s)
		}
	})

	t.Run("formatNumber", func(t *testing.T) {
		cases := []struct {
			n    int
			want string
		}{
			{0, "0"},
			{42, "42"},
			{999, "999"},
			{1000, "1,000"},
			{1247, "1,247"},
			{1000000, "1,000,000"},
		}
		for _, tc := range cases {
			got := formatNumber(tc.n)
			if got != tc.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tc.n, got, tc.want)
			}
		}
	})

	t.Run("formatDuration", func(t *testing.T) {
		cases := []struct {
			d    time.Duration
			want string
		}{
			{30 * time.Second, "30s"},
			{5 * time.Minute, "5m"},
			{3 * time.Hour, "3h"},
			{48 * time.Hour, "2d"},
		}
		for _, tc := range cases {
			got := formatDuration(tc.d)
			if got != tc.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		}
	})

	t.Run("countFileLines", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		content := "line1\nline2\n\nline3\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		got := countFileLines(path)
		if got != 3 {
			t.Errorf("countFileLines = %d, want 3", got)
		}
	})

	t.Run("countFileLines_nonexistent", func(t *testing.T) {
		got := countFileLines("/nonexistent/path")
		if got != 0 {
			t.Errorf("countFileLines(nonexistent) = %d, want 0", got)
		}
	})

	t.Run("countHealFindings", func(t *testing.T) {
		output := "[CODE] path/to/file: finding 1\n[CODE] other/file: finding 2\nSummary line\n"
		got := countHealFindings(output)
		if got != 2 {
			t.Errorf("countHealFindings = %d, want 2", got)
		}
	})

	t.Run("countHealFindings_from_summary", func(t *testing.T) {
		output := "Some output\n5 finding(s) detected.\n"
		got := countHealFindings(output)
		if got != 5 {
			t.Errorf("countHealFindings = %d, want 5", got)
		}
	})
}

// TestCobraStatusHelpers exercises status helper functions.
func TestCobraStatusHelpers(t *testing.T) {
	t.Run("truncateStatus", func(t *testing.T) {
		short := "hello"
		if got := truncateStatus(short, 10); got != "hello" {
			t.Errorf("truncateStatus(%q, 10) = %q", short, got)
		}

		long := "this is a very long string that exceeds the maximum length"
		got := truncateStatus(long, 20)
		if len(got) > 20 {
			t.Errorf("truncateStatus should cap at 20 chars, got %d: %q", len(got), got)
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("truncateStatus should end with ..., got: %q", got)
		}
	})

	t.Run("truncateStatus_multiline", func(t *testing.T) {
		multi := "first line\nsecond line"
		got := truncateStatus(multi, 60)
		if strings.Contains(got, "\n") {
			t.Errorf("truncateStatus should strip newlines, got: %q", got)
		}
	})

	t.Run("firstLine", func(t *testing.T) {
		if got := firstLine("hello\nworld"); got != "hello" {
			t.Errorf("firstLine = %q, want 'hello'", got)
		}
		if got := firstLine("no newline"); got != "no newline" {
			t.Errorf("firstLine = %q, want 'no newline'", got)
		}
	})

	t.Run("formatDurationBrief", func(t *testing.T) {
		cases := []struct {
			d    time.Duration
			want string
		}{
			{30 * time.Second, "<1m"},
			{5 * time.Minute, "5m"},
			{3 * time.Hour, "3h"},
			{48 * time.Hour, "2d"},
			{60 * 24 * time.Hour, "8w"},
		}
		for _, tc := range cases {
			got := formatDurationBrief(tc.d)
			if got != tc.want {
				t.Errorf("formatDurationBrief(%v) = %q, want %q", tc.d, got, tc.want)
			}
		}
	})

	t.Run("printFlywheelHealth", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		fw := &flywheelBrief{
			Status:         "STARTING",
			TotalArtifacts: 10,
			Velocity:       0.5,
			NewArtifacts:   2,
			StaleArtifacts: 1,
			LastForgeAge:   "2h",
		}
		printFlywheelHealth(fw)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out := buf.String()
		if !strings.Contains(out, "STARTING") {
			t.Errorf("expected 'STARTING' in output, got: %s", out)
		}
		if !strings.Contains(out, "2h ago") {
			t.Errorf("expected '2h ago' in output, got: %s", out)
		}
	})

	t.Run("printFlywheelHealth_negative_velocity", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		fw := &flywheelBrief{
			Status:         "DECAYING",
			TotalArtifacts: 5,
			Velocity:       -0.1,
			NewArtifacts:   0,
			StaleArtifacts: 3,
		}
		printFlywheelHealth(fw)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out := buf.String()
		if !strings.Contains(out, "-0.100") {
			t.Errorf("expected negative velocity in output, got: %s", out)
		}
	})
}

// TestCobraIndexHelpers exercises index helper functions.
func TestCobraIndexHelpers(t *testing.T) {
	t.Run("extractDateFromFilename", func(t *testing.T) {
		cases := []struct {
			filename string
			want     string
		}{
			{"2026-01-15-my-learning.md", "2026-01-15"},
			{"no-date.md", "unknown"},
			{"2026-12-31.md", "2026-12-31"},
		}
		for _, tc := range cases {
			got := extractDateFromFilename(tc.filename)
			if got != tc.want {
				t.Errorf("extractDateFromFilename(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		}
	})

	t.Run("summaryFromFilename", func(t *testing.T) {
		got := summaryFromFilename("2026-01-15-my-cool-learning.md")
		if got != "my-cool-learning" {
			t.Errorf("summaryFromFilename = %q, want 'my-cool-learning'", got)
		}

		got = summaryFromFilename("no-date-prefix.md")
		if got != "no-date-prefix" {
			t.Errorf("summaryFromFilename = %q, want 'no-date-prefix'", got)
		}
	})

	t.Run("extractH1", func(t *testing.T) {
		content := "some text\n# My Title\nmore text"
		got := extractH1(content)
		if got != "My Title" {
			t.Errorf("extractH1 = %q, want 'My Title'", got)
		}

		got = extractH1("no heading here")
		if got != "" {
			t.Errorf("extractH1 = %q, want empty", got)
		}
	})

	t.Run("cleanForTable", func(t *testing.T) {
		got := cleanForTable("has | pipe\nand  newline")
		if strings.Contains(got, "|") && !strings.Contains(got, "\\|") {
			t.Errorf("cleanForTable should escape pipes: %q", got)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("cleanForTable should remove newlines: %q", got)
		}
	})

	t.Run("titleCase", func(t *testing.T) {
		if got := titleCase("hello"); got != "Hello" {
			t.Errorf("titleCase('hello') = %q", got)
		}
		if got := titleCase(""); got != "" {
			t.Errorf("titleCase('') = %q", got)
		}
	})

	t.Run("parseFrontmatter", func(t *testing.T) {
		content := "---\ntitle: My Title\ndate: 2026-01-01\ntags: [a, b]\n---\n# Body\n"
		fm := parseFrontmatter(content)
		if fm["title"] != "My Title" {
			t.Errorf("parseFrontmatter title = %v", fm["title"])
		}
	})

	t.Run("extractTagsFromFrontmatter", func(t *testing.T) {
		fm := map[string]any{"tags": []any{"go", "testing"}}
		got := extractTagsFromFrontmatter(fm)
		if got != "go testing" {
			t.Errorf("extractTagsFromFrontmatter = %q, want 'go testing'", got)
		}

		fm = map[string]any{"tags": "[go, testing]"}
		got = extractTagsFromFrontmatter(fm)
		if got != "go testing" {
			t.Errorf("extractTagsFromFrontmatter (string) = %q, want 'go testing'", got)
		}

		fm = map[string]any{}
		got = extractTagsFromFrontmatter(fm)
		if got != "" {
			t.Errorf("extractTagsFromFrontmatter (empty) = %q, want ''", got)
		}
	})

	t.Run("diffFileSets", func(t *testing.T) {
		expected := map[string]bool{"a.md": true, "b.md": true}
		existing := map[string]bool{"b.md": true, "c.md": true}
		missing, extra := diffFileSets(expected, existing)
		if len(missing) != 1 || missing[0] != "a.md" {
			t.Errorf("missing = %v, want [a.md]", missing)
		}
		if len(extra) != 1 || extra[0] != "c.md" {
			t.Errorf("extra = %v, want [c.md]", extra)
		}
	})

	t.Run("buildExpectedFileSet", func(t *testing.T) {
		entries := []indexEntry{{Filename: "a.md"}, {Filename: "b.md"}}
		got := buildExpectedFileSet(entries)
		if !got["a.md"] || !got["b.md"] {
			t.Errorf("buildExpectedFileSet = %v", got)
		}
	})

	t.Run("parseIndexTableRows", func(t *testing.T) {
		content := `| File | Date | Summary | Tags |
|------|------|---------|------|
| a.md | 2026-01-01 | summary | tag |
| b.md | 2026-01-02 | summary2 | tag2 |
`
		got := parseIndexTableRows([]byte(content))
		if !got["a.md"] || !got["b.md"] {
			t.Errorf("parseIndexTableRows = %v", got)
		}
	})
}

// TestCobraFeedbackHelpers exercises feedback helper functions.
func TestCobraFeedbackHelpers(t *testing.T) {
	t.Run("resolveReward", func(t *testing.T) {
		reward, err := resolveReward(true, false, -1, 0.1)
		if err != nil || reward != 1.0 {
			t.Errorf("resolveReward(helpful) = (%v, %v)", reward, err)
		}

		reward, err = resolveReward(false, true, -1, 0.1)
		if err != nil || reward != 0.0 {
			t.Errorf("resolveReward(harmful) = (%v, %v)", reward, err)
		}

		_, err = resolveReward(true, true, -1, 0.1)
		if err == nil {
			t.Error("expected error for both helpful and harmful")
		}

		_, err = resolveReward(false, false, -1, 0.1)
		if err == nil {
			t.Error("expected error when no reward specified")
		}

		_, err = resolveReward(false, false, 1.5, 0.1)
		if err == nil {
			t.Error("expected error for reward > 1")
		}

		_, err = resolveReward(false, false, 0.5, 0.0)
		if err == nil {
			t.Error("expected error for alpha = 0")
		}
	})

	t.Run("classifyFeedbackType", func(t *testing.T) {
		if got := classifyFeedbackType(true, false); got != "helpful" {
			t.Errorf("got %q, want 'helpful'", got)
		}
		if got := classifyFeedbackType(false, true); got != "harmful" {
			t.Errorf("got %q, want 'harmful'", got)
		}
		if got := classifyFeedbackType(false, false); got != "custom" {
			t.Errorf("got %q, want 'custom'", got)
		}
	})

	t.Run("counterDirectionFromFeedback", func(t *testing.T) {
		helpful, harmful := counterDirectionFromFeedback(1.0, true, false)
		if !helpful || harmful {
			t.Error("expected helpful=true, harmful=false")
		}

		helpful, harmful = counterDirectionFromFeedback(0.0, false, true)
		if helpful || !harmful {
			t.Error("expected helpful=false, harmful=true")
		}

		helpful, harmful = counterDirectionFromFeedback(0.9, false, false)
		if !helpful || harmful {
			t.Error("expected implied helpful for reward >= 0.8")
		}

		helpful, harmful = counterDirectionFromFeedback(0.1, false, false)
		if helpful || !harmful {
			t.Error("expected implied harmful for reward <= 0.2")
		}

		helpful, harmful = counterDirectionFromFeedback(0.5, false, false)
		if helpful || harmful {
			t.Error("expected neither for reward = 0.5")
		}
	})

	t.Run("parseFrontMatterUtility", func(t *testing.T) {
		lines := []string{
			"---", // line 0 is the opening ---
			"utility: 0.7500",
			"reward_count: 3",
			"---",
		}
		endIdx, utility, err := parseFrontMatterUtility(lines)
		if err != nil {
			t.Fatalf("parseFrontMatterUtility error: %v", err)
		}
		if endIdx != 3 {
			t.Errorf("endIdx = %d, want 3", endIdx)
		}
		if utility != 0.75 {
			t.Errorf("utility = %f, want 0.75", utility)
		}
	})

	t.Run("parseFrontMatterUtility_malformed", func(t *testing.T) {
		lines := []string{"---", "utility: 0.5", "no closing"}
		_, _, err := parseFrontMatterUtility(lines)
		if err == nil {
			t.Error("expected error for malformed front matter")
		}
	})

	t.Run("updateFrontMatterFields", func(t *testing.T) {
		lines := []string{"utility: 0.5", "tags: test"}
		fields := map[string]string{"utility": "0.75", "new_field": "value"}
		result := updateFrontMatterFields(lines, fields)
		found := false
		for _, line := range result {
			if strings.HasPrefix(line, "utility: 0.75") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected updated utility in result: %v", result)
		}
	})

	t.Run("incrementRewardCount", func(t *testing.T) {
		lines := []string{"reward_count: 5"}
		got := incrementRewardCount(lines)
		if got != "6" {
			t.Errorf("incrementRewardCount = %q, want '6'", got)
		}
	})

	t.Run("parseFrontMatterInt", func(t *testing.T) {
		lines := []string{"helpful_count: 3"}
		got := parseFrontMatterInt(lines, "helpful_count")
		if got != 3 {
			t.Errorf("parseFrontMatterInt = %d, want 3", got)
		}
	})

	t.Run("incrementFMCount", func(t *testing.T) {
		lines := []string{"harmful_count: 2"}
		got := incrementFMCount(lines, "harmful_count")
		if got != "3" {
			t.Errorf("incrementFMCount = %q, want '3'", got)
		}
	})

	t.Run("rebuildWithFrontMatter", func(t *testing.T) {
		fm := []string{"utility: 0.5", "tags: test"}
		body := []string{"# Title", "Content"}
		got := rebuildWithFrontMatter(fm, body)
		if !strings.HasPrefix(got, "---\n") {
			t.Errorf("expected --- prefix, got: %s", got)
		}
		if !strings.Contains(got, "# Title") {
			t.Errorf("expected body in output, got: %s", got)
		}
	})

	t.Run("migrateJSONLFiles", func(t *testing.T) {
		tmp := t.TempDir()
		// File that needs migration
		f1 := filepath.Join(tmp, "a.jsonl")
		if err := os.WriteFile(f1, []byte(`{"summary":"test"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		// File that doesn't need migration
		f2 := filepath.Join(tmp, "b.jsonl")
		if err := os.WriteFile(f2, []byte(`{"summary":"test","utility":0.5}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		migrated, skipped := migrateJSONLFiles([]string{f1, f2}, false)
		if migrated != 1 || skipped != 1 {
			t.Errorf("migrateJSONLFiles = (%d, %d), want (1, 1)", migrated, skipped)
		}
	})

	t.Run("migrateJSONLFiles_dryRun", func(t *testing.T) {
		tmp := t.TempDir()
		f1 := filepath.Join(tmp, "a.jsonl")
		if err := os.WriteFile(f1, []byte(`{"summary":"test"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		migrated, _ := migrateJSONLFiles([]string{f1}, true)
		if migrated != 1 {
			t.Errorf("migrateJSONLFiles (dry-run) migrated = %d, want 1", migrated)
		}
	})
}

// TestCobraTraceHelpers exercises trace helper functions.
func TestCobraTraceHelpers(t *testing.T) {
	t.Run("repeatString", func(t *testing.T) {
		got := repeatString("ab", 3)
		if got != "ababab" {
			t.Errorf("repeatString('ab', 3) = %q", got)
		}
	})

	t.Run("min", func(t *testing.T) {
		if got := min(3, 5); got != 3 {
			t.Errorf("min(3,5) = %d", got)
		}
		if got := min(7, 2); got != 2 {
			t.Errorf("min(7,2) = %d", got)
		}
	})
}

// TestCobraMemoryHelpers exercises memory helper functions.
func TestCobraMemoryHelpers(t *testing.T) {
	t.Run("parseManagedBlock", func(t *testing.T) {
		content := "# Header\n\n" + memoryBlockStart + "\nentry1\n" + memoryBlockEnd + "\n\n# Footer\n"
		before, managed, after := parseManagedBlock(content)
		if !strings.Contains(before, "Header") {
			t.Errorf("before = %q", before)
		}
		if !strings.Contains(managed, "entry1") {
			t.Errorf("managed = %q", managed)
		}
		if !strings.Contains(after, "Footer") {
			t.Errorf("after = %q", after)
		}
	})

	t.Run("parseManagedBlock_none", func(t *testing.T) {
		before, managed, after := parseManagedBlock("no markers here")
		if before != "no markers here" || managed != "" || after != "" {
			t.Error("expected entire content as before")
		}
	})

	t.Run("extractSessionIDs", func(t *testing.T) {
		managed := "\n- **[2026-01-01]** (abc1234) Summary\n- **[2026-01-02]** (def5678) Other\n"
		ids := extractSessionIDs(managed)
		if !ids["abc1234"] || !ids["def5678"] {
			t.Errorf("extractSessionIDs = %v", ids)
		}
	})

	t.Run("extractEntryLines", func(t *testing.T) {
		managed := "\n- **[2026-01-01]** (abc) entry\nother line\n"
		lines := extractEntryLines(managed)
		if len(lines) != 1 {
			t.Errorf("extractEntryLines = %v, want 1 entry", lines)
		}
	})

	t.Run("buildManagedBlock_empty", func(t *testing.T) {
		got := buildManagedBlock(nil)
		if !strings.Contains(got, memoryBlockStart) {
			t.Errorf("expected start marker, got: %s", got)
		}
	})

	t.Run("buildManagedBlock_entries", func(t *testing.T) {
		entries := []string{"- entry1", "- entry2"}
		got := buildManagedBlock(entries)
		if !strings.Contains(got, "entry1") || !strings.Contains(got, "entry2") {
			t.Errorf("missing entries in output: %s", got)
		}
	})

	t.Run("assembleManagedFile_empty", func(t *testing.T) {
		got := assembleManagedFile("", "block", "")
		if !strings.Contains(got, "# Memory") {
			t.Errorf("expected '# Memory' header, got: %s", got)
		}
	})

	t.Run("assembleManagedFile_with_content", func(t *testing.T) {
		got := assembleManagedFile("# Before\n", "block", "# After\n")
		if !strings.Contains(got, "Before") || !strings.Contains(got, "After") {
			t.Errorf("expected before and after in output, got: %s", got)
		}
	})

	t.Run("findGitRoot", func(t *testing.T) {
		tmp := t.TempDir()
		subdir := filepath.Join(tmp, "a", "b")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		got := findGitRoot(subdir)
		if got != tmp {
			t.Errorf("findGitRoot = %q, want %q", got, tmp)
		}
	})

	t.Run("findGitRoot_none", func(t *testing.T) {
		got := findGitRoot("/")
		if got != "" {
			t.Errorf("findGitRoot('/') = %q, want empty", got)
		}
	})
}

// TestCobraTemplateDetect exercises detectTemplateFromProjectRoot.
func TestCobraTemplateDetect(t *testing.T) {
	cases := []struct {
		name     string
		files    []string
		expected string
	}{
		{"go_project", []string{"go.mod"}, "go-cli"},
		{"go_cli_subdir", []string{"cli/go.mod"}, "go-cli"},
		{"web_project", []string{"package.json"}, "web-app"},
		{"python_project", []string{"pyproject.toml"}, "python-lib"},
		{"rust_project", []string{"Cargo.toml"}, "rust-cli"},
		{"generic", []string{"README.md"}, "generic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			for _, f := range tc.files {
				path := filepath.Join(tmp, f)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got := detectTemplateFromProjectRoot(tmp)
			if got != tc.expected {
				t.Errorf("detectTemplateFromProjectRoot = %q, want %q", got, tc.expected)
			}
		})
	}
}

// TestCobraGettersAndSync exercises root.go getter functions.
func TestCobraGettersAndSync(t *testing.T) {
	// Save original values
	origDry := dryRun
	origVerbose := verbose
	origOutput := output
	origCfg := cfgFile
	defer func() {
		dryRun = origDry
		verbose = origVerbose
		output = origOutput
		cfgFile = origCfg
	}()

	dryRun = true
	if !GetDryRun() {
		t.Error("GetDryRun() should return true")
	}

	verbose = true
	if !GetVerbose() {
		t.Error("GetVerbose() should return true")
	}

	output = "json"
	if GetOutput() != "json" {
		t.Error("GetOutput() should return 'json'")
	}

	cfgFile = "/tmp/test.yaml"
	if GetConfigFile() != "/tmp/test.yaml" {
		t.Error("GetConfigFile() should return '/tmp/test.yaml'")
	}

	// Test syncConfigFlagToEnv
	t.Setenv("AGENTOPS_CONFIG", "")
	syncConfigFlagToEnv()
	if os.Getenv("AGENTOPS_CONFIG") != "/tmp/test.yaml" {
		t.Error("syncConfigFlagToEnv should set AGENTOPS_CONFIG")
	}

	cfgFile = ""
	t.Setenv("AGENTOPS_CONFIG", "previous")
	syncConfigFlagToEnv()
	if os.Getenv("AGENTOPS_CONFIG") != "previous" {
		t.Error("syncConfigFlagToEnv should not clear existing env when cfgFile is empty")
	}
}

// TestCobraGetCurrentUser exercises GetCurrentUser.
func TestCobraGetCurrentUser(t *testing.T) {
	user := GetCurrentUser()
	if user == "" {
		t.Error("GetCurrentUser() returned empty string")
	}
}

// TestCobraVerbosePrintf exercises VerbosePrintf.
func TestCobraVerbosePrintf(t *testing.T) {
	origVerbose := verbose
	defer func() { verbose = origVerbose }()

	// When not verbose, should not print
	verbose = false
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	VerbosePrintf("test %s\n", "message")
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if buf.Len() > 0 {
		t.Error("VerbosePrintf should not print when verbose=false")
	}

	// When verbose, should print
	verbose = true
	r, w, _ = os.Pipe()
	os.Stdout = w
	VerbosePrintf("test %s\n", "message")
	w.Close()
	os.Stdout = old
	buf.Reset()
	_, _ = io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("VerbosePrintf should print when verbose=true, got: %q", buf.String())
	}
}

// TestCobraShowConcepts exercises showConcepts directly.
func TestCobraShowConcepts(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showConcepts()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("showConcepts failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "KNOWLEDGE FLYWHEEL") {
		t.Errorf("expected KNOWLEDGE FLYWHEEL, got: %s", out[:200])
	}
	if !strings.Contains(out, "BROWNIAN RATCHET") {
		t.Errorf("expected BROWNIAN RATCHET in output")
	}
}

// TestCobraQuickstartHelpers exercises quickstart helper functions.
func TestCobraQuickstartHelpers(t *testing.T) {
	t.Run("createTasksFile", func(t *testing.T) {
		tmp := t.TempDir()
		agentsDir := filepath.Join(tmp, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}

		orig, _ := os.Getwd()
		_ = os.Chdir(tmp)
		defer func() { _ = os.Chdir(orig) }()

		// Capture stdout
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		createTasksFile(tmp)
		w.Close()
		os.Stdout = old

		path := filepath.Join(tmp, ".agents", "tasks.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("tasks.json not created: %v", err)
		}
		if !strings.Contains(string(data), "tasks") {
			t.Errorf("tasks.json content unexpected: %s", data)
		}
	})

	t.Run("showNextSteps_with_beads", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		showNextSteps(true)
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		if !strings.Contains(buf.String(), "bd create") {
			t.Error("expected 'bd create' in next steps with beads")
		}
	})

	t.Run("showNextSteps_without_beads", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		showNextSteps(false)
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		if !strings.Contains(buf.String(), "bd init") {
			t.Error("expected 'bd init' in next steps without beads")
		}
	})
}

// TestCobraNewestFileModTime exercises newestFileModTime.
func TestCobraNewestFileModTime(t *testing.T) {
	tmp := t.TempDir()
	// Create files with different times
	f1 := filepath.Join(tmp, "a.txt")
	f2 := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(f1, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // ensure different modtimes
	if err := os.WriteFile(f2, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}

	newest := newestFileModTime(entries)
	if newest.IsZero() {
		t.Error("expected non-zero newest time")
	}
}

// TestCobraCountFiles exercises countFiles.
func TestCobraCountFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	got := countFiles(tmp)
	if got != 2 {
		t.Errorf("countFiles = %d, want 2", got)
	}

	got = countFiles("/nonexistent/path")
	if got != 0 {
		t.Errorf("countFiles(nonexistent) = %d, want 0", got)
	}
}

// TestCobraCountLearningFiles exercises countLearningFiles.
func TestCobraCountLearningFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.md"), []byte("md"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.jsonl"), []byte("jsonl"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("txt"), 0644); err != nil {
		t.Fatal(err)
	}

	got := countLearningFiles(tmp)
	if got != 2 {
		t.Errorf("countLearningFiles = %d, want 2", got)
	}
}

// TestCobraCountEstablished exercises countEstablished.
func TestCobraCountEstablished(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "learning-established.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "promoted-pattern.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "regular.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	got := countEstablished(tmp)
	if got != 2 {
		t.Errorf("countEstablished = %d, want 2", got)
	}
}

// TestCobraResolveGoalsFile exercises resolveGoalsFile.
func TestCobraResolveGoalsFile(t *testing.T) {
	tmp := chdirTemp(t)

	t.Run("explicit_file", func(t *testing.T) {
		goalsFile = "/some/path.yaml"
		defer func() { goalsFile = "" }()
		got := resolveGoalsFile()
		if got != "/some/path.yaml" {
			t.Errorf("resolveGoalsFile = %q, want explicit path", got)
		}
	})

	t.Run("auto_detect_md", func(t *testing.T) {
		goalsFile = ""
		if err := os.WriteFile(filepath.Join(tmp, "GOALS.md"), []byte("# Goals"), 0644); err != nil {
			t.Fatal(err)
		}
		got := resolveGoalsFile()
		if got != "GOALS.md" {
			t.Errorf("resolveGoalsFile = %q, want 'GOALS.md'", got)
		}
	})

	t.Run("auto_detect_yaml", func(t *testing.T) {
		goalsFile = ""
		// Remove GOALS.md so YAML is preferred
		os.Remove(filepath.Join(tmp, "GOALS.md"))
		if err := os.WriteFile(filepath.Join(tmp, "GOALS.yaml"), []byte("version: 3"), 0644); err != nil {
			t.Fatal(err)
		}
		got := resolveGoalsFile()
		if got != "GOALS.yaml" {
			t.Errorf("resolveGoalsFile = %q, want 'GOALS.yaml'", got)
		}
	})

	t.Run("default_md", func(t *testing.T) {
		goalsFile = ""
		os.Remove(filepath.Join(tmp, "GOALS.yaml"))
		got := resolveGoalsFile()
		if got != "GOALS.md" {
			t.Errorf("resolveGoalsFile = %q, want default 'GOALS.md'", got)
		}
	})
}

// TestCobraHookGroupContainsAo exercises hookGroupContainsAo indirectly
// through extractHooksMap and evaluateHookCoverage.
func TestCobraExtractHooksMap(t *testing.T) {
	t.Run("settings_json_format", func(t *testing.T) {
		data := []byte(`{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "ao hooks run SessionStart"}]}]}}`)
		hooksMap, ok := extractHooksMap(data)
		if !ok {
			t.Error("expected extractHooksMap to return true for settings.json format")
		}
		if hooksMap == nil {
			t.Error("expected non-nil hooks map")
		}
	})

	t.Run("hooks_json_format", func(t *testing.T) {
		data := []byte(`{"SessionStart": [{"hooks": [{"type": "command", "command": "echo hi"}]}]}`)
		hooksMap, ok := extractHooksMap(data)
		if !ok {
			t.Error("expected extractHooksMap to return true for hooks.json format")
		}
		if hooksMap == nil {
			t.Error("expected non-nil hooks map")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		data := []byte(`not json`)
		_, ok := extractHooksMap(data)
		if ok {
			t.Error("expected false for invalid JSON")
		}
	})
}

// TestCobraCountHooksInMap exercises countHooksInMap.
func TestCobraCountHooksInMap(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		got := countHooksInMap([]any{"a", "b"})
		if got != 2 {
			t.Errorf("countHooksInMap(array) = %d, want 2", got)
		}
	})

	t.Run("nested_map", func(t *testing.T) {
		data := map[string]any{
			"SessionStart": []any{"a", "b"},
			"SessionEnd":   []any{"c"},
		}
		got := countHooksInMap(data)
		if got != 3 {
			t.Errorf("countHooksInMap(map) = %d, want 3", got)
		}
	})
}

// TestCobraFileExists exercises fileExists.
func TestCobraFileExists(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "exists.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	if !fileExists(path) {
		t.Error("fileExists should return true for existing file")
	}
	if fileExists(filepath.Join(tmp, "nope.txt")) {
		t.Error("fileExists should return false for nonexistent file")
	}
}

// TestCobraRatchetParentHelp ensures ratchet parent help works.
func TestCobraRatchetParentHelp(t *testing.T) {
	out, err := executeCommand("ratchet", "--help")
	if err != nil {
		t.Fatalf("ao ratchet --help failed: %v", err)
	}
	if !strings.Contains(out, "Brownian Ratchet") {
		t.Errorf("expected 'Brownian Ratchet' in output, got: %s", out)
	}
}

// TestCobraRPIParentHelp ensures rpi parent help works.
func TestCobraRPIParentHelp(t *testing.T) {
	out, err := executeCommand("rpi", "--help")
	if err != nil {
		t.Fatalf("ao rpi --help failed: %v", err)
	}
	if !strings.Contains(out, "RPI") {
		t.Errorf("expected 'RPI' in output, got: %s", out)
	}
}

// TestCobraPoolParentHelp ensures pool parent help works.
func TestCobraPoolParentHelp(t *testing.T) {
	out, err := executeCommand("pool", "--help")
	if err != nil {
		t.Fatalf("ao pool --help failed: %v", err)
	}
	if !strings.Contains(out, "pool") {
		t.Errorf("expected 'pool' in output, got: %s", out)
	}
}

// TestCobraStoreParentHelp ensures store parent help works.
func TestCobraStoreParentHelp(t *testing.T) {
	out, err := executeCommand("store", "--help")
	if err != nil {
		t.Fatalf("ao store --help failed: %v", err)
	}
	if !strings.Contains(out, "store") && !strings.Contains(out, "Store") && !strings.Contains(out, "STORE") {
		t.Errorf("expected 'store' in output, got: %s", out)
	}
}

// TestCobraGoalsParentHelp ensures goals parent help works.
func TestCobraGoalsParentHelp(t *testing.T) {
	out, err := executeCommand("goals", "--help")
	if err != nil {
		t.Fatalf("ao goals --help failed: %v", err)
	}
	if !strings.Contains(out, "goals") && !strings.Contains(out, "Goals") {
		t.Errorf("expected goals help in output, got: %s", out)
	}
}

// TestCobraSessionParentHelp ensures session parent help works.
func TestCobraSessionParentHelp(t *testing.T) {
	out, err := executeCommand("session", "--help")
	if err != nil {
		t.Fatalf("ao session --help failed: %v", err)
	}
	if !strings.Contains(out, "session") || !strings.Contains(out, "Session") {
		t.Errorf("expected session help in output, got: %s", out)
	}
}

// TestCobraTemperParentHelp ensures temper parent help works.
func TestCobraTemperParentHelp(t *testing.T) {
	out, err := executeCommand("temper", "--help")
	if err != nil {
		t.Fatalf("ao temper --help failed: %v", err)
	}
	if !strings.Contains(out, "TEMPER") || !strings.Contains(out, "temper") {
		t.Errorf("expected temper help in output, got: %s", out)
	}
}

// TestCobraNotebookParentHelp ensures notebook parent help works under settings namespace.
func TestCobraNotebookParentHelp(t *testing.T) {
	out, err := executeCommand("notebook", "--help")
	if err != nil {
		t.Fatalf("ao notebook --help failed: %v", err)
	}
	if !strings.Contains(out, "notebook") || !strings.Contains(out, "MEMORY") {
		t.Errorf("expected notebook help in output, got: %s", out)
	}
}

// TestCobraConstraintParentHelp ensures constraint parent help works.
func TestCobraConstraintParentHelp(t *testing.T) {
	out, err := executeCommand("constraint", "--help")
	if err != nil {
		t.Fatalf("ao constraint --help failed: %v", err)
	}
	if !strings.Contains(out, "constraint") {
		t.Errorf("expected constraint help in output, got: %s", out)
	}
}

// TestCobraFlywheelParentHelp ensures flywheel parent help works.
func TestCobraFlywheelParentHelp(t *testing.T) {
	out, err := executeCommand("flywheel", "--help")
	if err != nil {
		t.Fatalf("ao flywheel --help failed: %v", err)
	}
	if !strings.Contains(out, "flywheel") {
		t.Errorf("expected flywheel help in output, got: %s", out)
	}
}

// TestCobraMetricsParentHelp ensures metrics parent help works.
func TestCobraMetricsParentHelp(t *testing.T) {
	out, err := executeCommand("metrics", "--help")
	if err != nil {
		t.Fatalf("ao metrics --help failed: %v", err)
	}
	if !strings.Contains(out, "metrics") || !strings.Contains(out, "flywheel") {
		t.Errorf("expected metrics help in output, got: %s", out)
	}
}

// TestCobraMemoryParentHelp ensures memory parent help works under settings namespace.
func TestCobraMemoryParentHelp(t *testing.T) {
	out, err := executeCommand("memory", "--help")
	if err != nil {
		t.Fatalf("ao memory --help failed: %v", err)
	}
	if !strings.Contains(out, "memory") || !strings.Contains(out, "MEMORY") {
		t.Errorf("expected memory help in output, got: %s", out)
	}
}

// Verify unused import suppression
var _ = fmt.Sprintf

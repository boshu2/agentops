// practices: [event-sourcing-cqrs, distributed-tracing]
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
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/spf13/cobra"
)

func TestHandoffPropagatesCommandContext(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, nested)
	binDir := t.TempDir()
	tracePath := filepath.Join(t.TempDir(), "tracker.trace")
	stub := `#!/bin/sh
printf 'binary=%s|pwd=%s|beads=%s|argv=%s\n' "${0##*/}" "$(pwd -P)" "${BEADS_DIR-<unset>}" "$*" >> "$TRACKER_TRACE"
case "$1" in
  list) printf '[{"id":"age-active","status":"in_progress","updated_at":"2026-07-14T11:00:00Z"}]\n' ;;
  ready) printf '[{"id":"age-ready"}]\n' ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "br"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("TRACKER_TRACE", tracePath)
	t.Setenv("AGENTOPS_TRACKER", "br")
	t.Setenv("BEADS_DIR", filepath.Join(root, "_beads"))

	origGoal, origCollect := handoffGoal, handoffCollect
	origRPIPhase, origEpicID, origRunID := handoffRPIPhase, handoffEpicID, handoffRunID
	origDryRun, origNoKill := handoffDryRun, handoffNoKill
	origResolve := resolveTracker
	resolveCalls := 0
	resolveTracker = func(cwd string, env []string) (trackerResolution, error) {
		resolveCalls++
		return origResolve(cwd, env)
	}
	handoffGoal = ""
	handoffCollect = true
	handoffRPIPhase = 0
	handoffEpicID = ""
	handoffRunID = ""
	handoffDryRun = true
	handoffNoKill = true
	t.Cleanup(func() {
		handoffGoal, handoffCollect = origGoal, origCollect
		handoffRPIPhase, handoffEpicID, handoffRunID = origRPIPhase, origEpicID, origRunID
		handoffDryRun, handoffNoKill = origDryRun, origNoKill
		resolveTracker = origResolve
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runHandoff(cmd, []string{"preserve caller context"}); err != nil {
		t.Fatalf("run handoff: %v", err)
	}
	if _, err := os.Stat(tracePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pre-canceled handoff launched tracker: %v", err)
	}

	cmd = &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runHandoff(cmd, []string{"preserve caller context"}); err != nil {
		t.Fatalf("run BR handoff: %v", err)
	}
	physicalNested, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatal(err)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	brPrefix := "binary=br|pwd=" + physicalNested + "|beads=" + filepath.Join(root, "_beads") + "|argv="
	if len(lines) != 2 || lines[0] != brPrefix+"list --status in_progress --json" || lines[1] != brPrefix+"ready --json" {
		t.Fatalf("BR tracker trace = %q, want one canonical list/ready pair", lines)
	}

	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_TRACKER", "bd")
	if err := runHandoff(cmd, []string{"preserve caller context"}); err != nil {
		t.Fatalf("run BD handoff: %v", err)
	}
	data, err = os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	bdPrefix := "binary=bd|pwd=" + physicalRoot + "|beads=<unset>|argv="
	if len(lines) != 2 || lines[0] != bdPrefix+"list --status in_progress --json" || lines[1] != bdPrefix+"ready --json" {
		t.Fatalf("BD tracker trace = %q, want one canonical list/ready pair", lines)
	}

	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	directCmd := &cobra.Command{}
	if err := runHandoff(directCmd, []string{"context-less direct RunE"}); err != nil {
		t.Fatalf("direct RunE with nil context: %v", err)
	}
	data, err = os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != bdPrefix+"list --status in_progress --json" || lines[1] != bdPrefix+"ready --json" {
		t.Fatalf("direct RunE trace = %q, want canonical list/ready", lines)
	}
	if resolveCalls != 4 {
		t.Fatalf("tracker resolutions after four collections = %d, want one per collection", resolveCalls)
	}

	exitStub := "#!/bin/sh\nexit 23\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(exitStub), 0o755); err != nil {
		t.Fatal(err)
	}
	resolution, err := resolveTracker(nested, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	_, err = execHandoffTracker(context.Background(), resolution, 1500*time.Millisecond, "ready", "--json")
	var exitErr *trackerexec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Fatalf("tracker error = %T %v, want *trackerexec.ExitError(23)", err, err)
	}
	state := collectHandoffStateContext(context.Background(), nested)
	if state.ActiveBead != "" || state.OpenBeadsCount != 0 {
		t.Fatalf("failed tracker must degrade to empty state, got %+v", state)
	}
	if resolveCalls != 6 {
		t.Fatalf("tracker resolutions including typed-exit and degraded collection = %d, want 6", resolveCalls)
	}
}

func TestRunHandoff_WritesArtifact(t *testing.T) {
	dir := t.TempDir()

	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T191026Z",
		CreatedAt:     "2026-03-03T19:10:26Z",
		Type:          "manual",
		Goal:          "test goal",
		Summary:       "test summary",
		Consumed:      false,
	}

	path, err := writeHandoffArtifact(dir, artifact)
	if err != nil {
		t.Fatalf("writeHandoffArtifact failed: %v", err)
	}

	// Verify file exists
	if !quality.FileExists(path) {
		t.Fatalf("artifact file not found at %s", path)
	}

	// Verify path is under .agents/handoff/
	expectedDir := filepath.Join(dir, ".agents", "handoff")
	if !strings.HasPrefix(path, expectedDir) {
		t.Errorf("path %q not under expected dir %q", path, expectedDir)
	}

	// Verify JSON content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	var read handoffArtifact
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	if read.Goal != "test goal" {
		t.Errorf("goal = %q, want %q", read.Goal, "test goal")
	}
	if read.Summary != "test summary" {
		t.Errorf("summary = %q, want %q", read.Summary, "test summary")
	}
	if read.Type != "manual" {
		t.Errorf("type = %q, want %q", read.Type, "manual")
	}
}

func TestRunHandoff_DryRun(t *testing.T) {
	dir := t.TempDir()

	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T191026Z",
		CreatedAt:     "2026-03-03T19:10:26Z",
		Type:          "manual",
		Summary:       "dry run test",
		Consumed:      false,
	}

	// Marshal to verify JSON is valid (what --dry-run would print)
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "dry run test") {
		t.Error("marshalled output missing summary")
	}

	// Verify NO file is written
	handoffDir := filepath.Join(dir, ".agents", "handoff")
	if quality.FileExists(handoffDir) {
		t.Error("handoff dir should not exist in dry-run mode")
	}
}

func TestRunHandoff_NoKill(t *testing.T) {
	dir := t.TempDir()

	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T191026Z",
		CreatedAt:     "2026-03-03T19:10:26Z",
		Type:          "manual",
		Summary:       "no-kill test",
		Consumed:      false,
	}

	path, err := writeHandoffArtifact(dir, artifact)
	if err != nil {
		t.Fatalf("writeHandoffArtifact failed: %v", err)
	}

	// Verify artifact was written (no-kill still writes)
	if !quality.FileExists(path) {
		t.Fatal("artifact should exist with --no-kill")
	}
}

func TestRunHandoff_NonTmux(t *testing.T) {
	// Unset TMUX_PANE to simulate non-tmux environment
	origPane := os.Getenv("TMUX_PANE")
	os.Unsetenv("TMUX_PANE")
	defer func() {
		if origPane != "" {
			os.Setenv("TMUX_PANE", origPane)
		}
	}()

	err := killSessionViaTmux("/tmp")
	if err == nil {
		t.Error("expected error when TMUX_PANE is unset")
	}
	if !strings.Contains(err.Error(), "not in tmux") {
		t.Errorf("error = %q, want 'not in tmux'", err.Error())
	}
}

func TestRunHandoff_CollectState(t *testing.T) {
	dir := t.TempDir()

	// Create a git repo for collectHandoffState to work with
	state := collectHandoffState(dir)

	if state == nil {
		t.Fatal("collectHandoffState returned nil")
	}

	// In a temp dir with no git repo, branch should be empty and dirty should be false
	if state.GitDirty {
		t.Error("expected GitDirty=false in empty temp dir")
	}
}

func TestRunHandoff_RPIPhase(t *testing.T) {
	dir := t.TempDir()

	// Create phased-state.json with verdicts
	rpiDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateData := `{
		"verdicts": {"security": "PASS", "complexity": "WARN"},
		"epic_id": "na-abc",
		"run_id": "r12345"
	}`
	if err := os.WriteFile(filepath.Join(rpiDir, "phased-state.json"), []byte(stateData), 0o644); err != nil {
		t.Fatal(err)
	}

	rpi := buildHandoffRPIContext(dir, 2, "", "")
	if rpi == nil {
		t.Fatal("buildHandoffRPIContext returned nil")
	}

	if rpi.Phase != 2 {
		t.Errorf("phase = %d, want 2", rpi.Phase)
	}
	if rpi.PhaseName != "implementation" {
		t.Errorf("phase_name = %q, want %q", rpi.PhaseName, "implementation")
	}
	if rpi.EpicID != "na-abc" {
		t.Errorf("epic_id = %q, want %q", rpi.EpicID, "na-abc")
	}
	if rpi.RunID != "r12345" {
		t.Errorf("run_id = %q, want %q", rpi.RunID, "r12345")
	}
	if len(rpi.Verdicts) != 2 {
		t.Errorf("verdicts count = %d, want 2", len(rpi.Verdicts))
	}
	if rpi.Verdicts["security"] != "PASS" {
		t.Errorf("verdicts[security] = %q, want %q", rpi.Verdicts["security"], "PASS")
	}
}

func TestRunHandoff_RPIPhase_FlagOverride(t *testing.T) {
	dir := t.TempDir()

	// Create phased-state.json
	rpiDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateData := `{"verdicts": {}, "epic_id": "from-file", "run_id": "from-file"}`
	if err := os.WriteFile(filepath.Join(rpiDir, "phased-state.json"), []byte(stateData), 0o644); err != nil {
		t.Fatal(err)
	}

	// Flags should take precedence over file values
	rpi := buildHandoffRPIContext(dir, 1, "from-flag", "run-flag")
	if rpi.EpicID != "from-flag" {
		t.Errorf("epic_id = %q, want %q (flag override)", rpi.EpicID, "from-flag")
	}
	if rpi.RunID != "run-flag" {
		t.Errorf("run_id = %q, want %q (flag override)", rpi.RunID, "run-flag")
	}
}

func TestRunHandoff_SchemaVersion(t *testing.T) {
	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T191026Z",
		CreatedAt:     "2026-03-03T19:10:26Z",
		Type:          "manual",
		Consumed:      false,
	}

	data, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	sv, ok := parsed["schema_version"]
	if !ok {
		t.Fatal("schema_version field missing")
	}
	if sv != float64(1) {
		t.Errorf("schema_version = %v, want 1", sv)
	}
}

func TestHandoffArtifact_JSONRoundTrip(t *testing.T) {
	original := handoffJSONRoundTripArtifact()

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundTripped handoffArtifact
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	assertHandoffArtifactRoundTrip(t, roundTripped, original)
}

func handoffJSONRoundTripArtifact() handoffArtifact {
	return handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T191026Z",
		CreatedAt:     "2026-03-03T19:10:26Z",
		Type:          "rpi",
		Goal:          "build auth module",
		Summary:       "completed JWT flow",
		Continuation:  "add refresh tokens next",
		ArtifactsProduced: []string{
			"cli/cmd/ao/auth.go",
			"cli/cmd/ao/auth_test.go",
		},
		DecisionsMade: []string{"used HS256 over RS256 for simplicity"},
		OpenRisks:     []string{"token rotation not implemented"},
		RPI: &handoffRPI{
			Phase:     2,
			PhaseName: "implementation",
			EpicID:    "na-auth",
			RunID:     "abc123",
			Verdicts:  map[string]string{"security": "PASS", "complexity": "WARN"},
		},
		State: &handoffState{
			GitBranch:      "feat/auth",
			GitDirty:       true,
			ModifiedFiles:  []string{"auth.go", "auth_test.go"},
			ActiveBead:     "na-auth.3",
			OpenBeadsCount: 5,
			RecentCommits:  []string{"abc123 add JWT signing", "def456 add token validation"},
		},
		Consumed:   false,
		ConsumedAt: nil,
		ConsumedBy: nil,
	}
}

func assertHandoffArtifactRoundTrip(t *testing.T, got, want handoffArtifact) {
	t.Helper()

	assertHandoffArtifactScalars(t, got, want)
	assertHandoffArtifactListLengths(t, got, want)
	assertHandoffRPIRoundTrip(t, got.RPI, want.RPI)
	assertHandoffStateRoundTrip(t, got.State, want.State)
	assertHandoffEqual(t, "consumed", got.Consumed, want.Consumed)
	assertHandoffNilPointer(t, "consumed_at", got.ConsumedAt)
	assertHandoffNilPointer(t, "consumed_by", got.ConsumedBy)
}

func assertHandoffArtifactScalars(t *testing.T, got, want handoffArtifact) {
	t.Helper()

	assertHandoffEqual(t, "schema_version", got.SchemaVersion, want.SchemaVersion)
	assertHandoffEqual(t, "id", got.ID, want.ID)
	assertHandoffEqual(t, "created_at", got.CreatedAt, want.CreatedAt)
	assertHandoffEqual(t, "type", got.Type, want.Type)
	assertHandoffEqual(t, "goal", got.Goal, want.Goal)
	assertHandoffEqual(t, "summary", got.Summary, want.Summary)
	assertHandoffEqual(t, "continuation", got.Continuation, want.Continuation)
}

func assertHandoffArtifactListLengths(t *testing.T, got, want handoffArtifact) {
	t.Helper()

	assertHandoffEqual(t, "artifacts_produced", len(got.ArtifactsProduced), len(want.ArtifactsProduced))
	assertHandoffEqual(t, "decisions_made", len(got.DecisionsMade), len(want.DecisionsMade))
	assertHandoffEqual(t, "open_risks", len(got.OpenRisks), len(want.OpenRisks))
}

func assertHandoffRPIRoundTrip(t *testing.T, got, want *handoffRPI) {
	t.Helper()

	if got == nil {
		t.Fatal("rpi: got nil, want non-nil")
	}
	assertHandoffEqual(t, "rpi.phase", got.Phase, want.Phase)
	assertHandoffEqual(t, "rpi.phase_name", got.PhaseName, want.PhaseName)
	assertHandoffEqual(t, "rpi.epic_id", got.EpicID, want.EpicID)
	assertHandoffEqual(t, "rpi.run_id", got.RunID, want.RunID)
	assertHandoffEqual(t, "rpi.verdicts", len(got.Verdicts), len(want.Verdicts))
}

func assertHandoffStateRoundTrip(t *testing.T, got, want *handoffState) {
	t.Helper()

	if got == nil {
		t.Fatal("state: got nil, want non-nil")
	}

	assertHandoffEqual(t, "state.git_branch", got.GitBranch, want.GitBranch)
	assertHandoffEqual(t, "state.git_dirty", got.GitDirty, want.GitDirty)
	assertHandoffEqual(t, "state.modified_files", len(got.ModifiedFiles), len(want.ModifiedFiles))
	assertHandoffEqual(t, "state.active_bead", got.ActiveBead, want.ActiveBead)
	assertHandoffEqual(t, "state.open_beads_count", got.OpenBeadsCount, want.OpenBeadsCount)
	assertHandoffEqual(t, "state.recent_commits", len(got.RecentCommits), len(want.RecentCommits))
}

func assertHandoffEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()

	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func assertHandoffNilPointer[T any](t *testing.T, label string, got *T) {
	t.Helper()

	if got != nil {
		t.Errorf("%s: got non-nil, want nil", label)
	}
}

func TestCollectHandoffState_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Hermetic: point the bead tracker at an empty dir so `br` finds no ledger.
	// (Before bd→br, ag-8c00a, this test passed only because the dead `bd` call
	// always returned empty; now that br actually resolves a ledger, the test
	// must isolate from the ambient real ledger.)
	t.Setenv("BEADS_DIR", t.TempDir())

	state := collectHandoffState(dir)
	if state == nil {
		t.Fatal("collectHandoffState returned nil for empty dir")
	}
	if state.GitDirty {
		t.Error("expected GitDirty=false for non-git dir")
	}
	if state.GitBranch != "" {
		t.Errorf("expected empty branch for non-git dir, got %q", state.GitBranch)
	}
	if state.ActiveBead != "" {
		t.Errorf("expected empty active bead with no ledger, got %q", state.ActiveBead)
	}
}

func TestWriteHandoffArtifact_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	artifact := &handoffArtifact{
		SchemaVersion: 1,
		ID:            "handoff-20260303T200000Z",
		CreatedAt:     "2026-03-03T20:00:00Z",
		Type:          "manual",
		Summary:       "atomic test",
		Consumed:      false,
	}

	path, err := writeHandoffArtifact(dir, artifact)
	if err != nil {
		t.Fatalf("writeHandoffArtifact failed: %v", err)
	}

	// Verify no .tmp file lingering
	tmpPath := path + ".tmp"
	if quality.FileExists(tmpPath) {
		t.Error(".tmp file should not exist after successful write")
	}

	// Verify final file is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var check handoffArtifact
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if check.Summary != "atomic test" {
		t.Errorf("summary = %q, want %q", check.Summary, "atomic test")
	}
}

func TestHandoffShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
	}

	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestKillSessionViaTmux_NotInTmux(t *testing.T) {
	// Save and clear TMUX-related env vars
	origTmux := os.Getenv("TMUX")
	origPane := os.Getenv("TMUX_PANE")
	os.Unsetenv("TMUX")
	os.Unsetenv("TMUX_PANE")
	defer func() {
		if origTmux != "" {
			os.Setenv("TMUX", origTmux)
		}
		if origPane != "" {
			os.Setenv("TMUX_PANE", origPane)
		}
	}()

	err := killSessionViaTmux(t.TempDir())
	if err == nil {
		t.Fatal("expected error when not in tmux, got nil")
	}
	if err.Error() != "not in tmux" {
		t.Errorf("error message = %q, want %q", err.Error(), "not in tmux")
	}
}

func TestRunHandoff_BasicFlow(t *testing.T) {
	dir := t.TempDir()

	// Save and restore global flag vars (runHandoff reads these)
	origGoal := handoffGoal
	origCollect := handoffCollect
	origRPIPhase := handoffRPIPhase
	origEpicID := handoffEpicID
	origRunID := handoffRunID
	origDryRun := handoffDryRun
	origNoKill := handoffNoKill
	defer func() {
		handoffGoal = origGoal
		handoffCollect = origCollect
		handoffRPIPhase = origRPIPhase
		handoffEpicID = origEpicID
		handoffRunID = origRunID
		handoffDryRun = origDryRun
		handoffNoKill = origNoKill
	}()

	// Set flags for a basic handoff: no-kill to avoid tmux, set a goal
	handoffGoal = "test basic flow"
	handoffCollect = false
	handoffRPIPhase = 0
	handoffEpicID = ""
	handoffRunID = ""
	handoffDryRun = false
	handoffNoKill = true // avoid tmux kill attempt

	// Change to temp dir so runHandoff's os.Getwd() uses it
	t.Chdir(dir)

	// Call runHandoff — cmd parameter is unused by the function body
	err := runHandoff(nil, []string{"basic flow summary"})
	if err != nil {
		t.Fatalf("runHandoff returned error: %v", err)
	}

	// Verify a handoff artifact was written under .agents/handoff/
	handoffDir := filepath.Join(dir, ".agents", "handoff")
	entries, err := os.ReadDir(handoffDir)
	if err != nil {
		t.Fatalf("handoff dir not created: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 handoff file, got %d", len(entries))
	}

	// Read and verify the artifact content
	data, err := os.ReadFile(filepath.Join(handoffDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var artifact handoffArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	if artifact.Summary != "basic flow summary" {
		t.Errorf("summary = %q, want %q", artifact.Summary, "basic flow summary")
	}
	if artifact.Goal != "test basic flow" {
		t.Errorf("goal = %q, want %q", artifact.Goal, "test basic flow")
	}
	if artifact.Type != "manual" {
		t.Errorf("type = %q, want %q", artifact.Type, "manual")
	}
	if artifact.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", artifact.SchemaVersion)
	}
	if artifact.Consumed {
		t.Error("consumed should be false for newly written artifact")
	}
	if !strings.HasPrefix(artifact.ID, "handoff-") {
		t.Errorf("id = %q, expected prefix 'handoff-'", artifact.ID)
	}
	if artifact.CreatedAt == "" {
		t.Error("created_at should not be empty")
	}
}

func TestRunHandoff_DryRunViaRunHandoff(t *testing.T) {
	dir := t.TempDir()

	// Save and restore global flag vars
	origGoal := handoffGoal
	origDryRun := handoffDryRun
	origNoKill := handoffNoKill
	origCollect := handoffCollect
	origRPIPhase := handoffRPIPhase
	defer func() {
		handoffGoal = origGoal
		handoffDryRun = origDryRun
		handoffNoKill = origNoKill
		handoffCollect = origCollect
		handoffRPIPhase = origRPIPhase
	}()

	handoffGoal = "dry run goal"
	handoffDryRun = true
	handoffNoKill = true
	handoffCollect = false
	handoffRPIPhase = 0

	// Change to temp dir
	t.Chdir(dir)

	// Call runHandoff with --dry-run set — cmd parameter unused
	err := runHandoff(nil, []string{"dry run summary"})
	if err != nil {
		t.Fatalf("runHandoff --dry-run returned error: %v", err)
	}

	// Verify NO files were written
	handoffDir := filepath.Join(dir, ".agents", "handoff")
	if quality.FileExists(handoffDir) {
		t.Error("handoff dir should not exist when --dry-run is set")
	}
	agentsDir := filepath.Join(dir, ".agents")
	if quality.FileExists(agentsDir) {
		t.Error(".agents dir should not exist when --dry-run is set")
	}
}

func TestBuildHandoffRPIContext_NoStateFile(t *testing.T) {
	dir := t.TempDir()

	// No phased-state.json exists
	rpi := buildHandoffRPIContext(dir, 3, "na-xyz", "run-abc")
	if rpi == nil {
		t.Fatal("expected non-nil RPI even without state file")
	}
	if rpi.Phase != 3 {
		t.Errorf("phase = %d, want 3", rpi.Phase)
	}
	if rpi.PhaseName != "validation" {
		t.Errorf("phase_name = %q, want %q", rpi.PhaseName, "validation")
	}
	if rpi.EpicID != "na-xyz" {
		t.Errorf("epic_id = %q, want %q", rpi.EpicID, "na-xyz")
	}
	if rpi.RunID != "run-abc" {
		t.Errorf("run_id = %q, want %q", rpi.RunID, "run-abc")
	}
}

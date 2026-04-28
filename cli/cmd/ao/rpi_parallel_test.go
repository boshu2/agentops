package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalSlug(t *testing.T) {
	tests := []struct {
		goal string
		want string
	}{
		{"add evolve watchdog", "evolve-watchdog"},
		{"Add CLI dashboard and reporting", "cli-dashboard-reporting"},
		{"consolidate skills to 40", "consolidate-skills-40"},
		{"Create a Codex-native plugin", "create-codexnative-plugin"},
		{"fix the bug", "fix-bug"},
		{"", ""},
		{"the a an to for", ""},
	}
	for _, tt := range tests {
		got := goalSlug(tt.goal)
		if got != tt.want {
			t.Errorf("goalSlug(%q) = %q, want %q", tt.goal, got, tt.want)
		}
	}
}

func TestResolveParallelEpics_Args(t *testing.T) {
	// Reset global.
	parallelManifest = ""

	args := []string{"add evolve watchdog", "add CLI dashboard"}
	epics, err := resolveParallelEpics(args)
	if err != nil {
		t.Fatalf("resolveParallelEpics: %v", err)
	}
	if len(epics) != 2 {
		t.Fatalf("expected 2 epics, got %d", len(epics))
	}
	if epics[0].Goal != "add evolve watchdog" {
		t.Errorf("epic 0 goal = %q, want %q", epics[0].Goal, "add evolve watchdog")
	}
	if epics[0].MergeOrder != 1 {
		t.Errorf("epic 0 merge_order = %d, want 1", epics[0].MergeOrder)
	}
	if epics[1].MergeOrder != 2 {
		t.Errorf("epic 1 merge_order = %d, want 2", epics[1].MergeOrder)
	}
}

func TestResolveParallelEpics_Manifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := parallelManifestFile{
		Epics: []parallelEpic{
			{Name: "evolve", Goal: "Add watchdog", MergeOrder: 2},
			{Name: "cli-obs", Goal: "Add dashboard", MergeOrder: 1},
		},
	}
	data, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(tmpDir, "epics.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	parallelManifest = manifestPath
	defer func() { parallelManifest = "" }()

	epics, err := resolveParallelEpics(nil)
	if err != nil {
		t.Fatalf("resolveParallelEpics manifest: %v", err)
	}
	if len(epics) != 2 {
		t.Fatalf("expected 2 epics, got %d", len(epics))
	}
	if epics[0].Name != "evolve" {
		t.Errorf("epic 0 name = %q, want %q", epics[0].Name, "evolve")
	}
	if epics[1].MergeOrder != 1 {
		t.Errorf("epic 1 merge_order = %d, want 1", epics[1].MergeOrder)
	}
}

func TestResolveParallelEpics_ManifestDuplicateName(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := parallelManifestFile{
		Epics: []parallelEpic{
			{Name: "evolve", Goal: "Goal 1"},
			{Name: "evolve", Goal: "Goal 2"},
		},
	}
	data, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(tmpDir, "epics.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	parallelManifest = manifestPath
	defer func() { parallelManifest = "" }()

	_, err := resolveParallelEpics(nil)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestResolveParallelEpics_Empty(t *testing.T) {
	parallelManifest = ""
	epics, err := resolveParallelEpics(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(epics) != 0 {
		t.Fatalf("expected 0 epics, got %d", len(epics))
	}
}

func TestResolveMergeOrder_Default(t *testing.T) {
	epics := []parallelEpic{
		{Name: "c", MergeOrder: 3},
		{Name: "a", MergeOrder: 1},
		{Name: "b", MergeOrder: 2},
	}
	results := []parallelResult{
		{Success: true},
		{Success: true},
		{Success: true},
	}
	parallelMergeOrder = ""
	order := resolveMergeOrder(epics, results)
	// Should be sorted by MergeOrder: a(1), b(2), c(3)
	if order[0] != 1 || order[1] != 2 || order[2] != 0 {
		t.Errorf("merge order = %v, want [1 2 0]", order)
	}
}

func TestResolveMergeOrder_Explicit(t *testing.T) {
	epics := []parallelEpic{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}
	results := []parallelResult{{}, {}, {}}
	parallelMergeOrder = "gamma,alpha,beta"
	defer func() { parallelMergeOrder = "" }()

	order := resolveMergeOrder(epics, results)
	if order[0] != 2 || order[1] != 0 || order[2] != 1 {
		t.Errorf("merge order = %v, want [2 0 1]", order)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's", "'it'\\''s'"},
		{"back`tick", "'back`tick'"},
		{`double"quote`, `'double"quote'`},
		{"multi'ple'quotes", "'multi'\\''ple'\\''quotes'"},
		{"$VAR", "'$VAR'"},
	}
	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTmuxPaneIsDead_Parsing(t *testing.T) {
	// tmuxPaneIsDead calls exec.Command internally, so we test the parsing
	// logic by verifying the function handles missing tmux gracefully.
	// With a non-existent tmux binary, the pane is considered dead.
	dead, code := tmuxPaneIsDead("__nonexistent_tmux_binary__", "nosession:nowindow")
	if !dead {
		t.Error("expected dead=true for missing tmux binary")
	}
	if code != 1 {
		t.Errorf("expected exitCode=1, got %d", code)
	}
}

// --- Tests for extracted helpers ---

func TestValidateParallelPrereqs_NoArgs(t *testing.T) {
	// With no args and no manifest, validateParallelPrereqs should return an error.
	old := parallelManifest
	parallelManifest = ""
	defer func() { parallelManifest = old }()

	_, _, _, err := validateParallelPrereqs(nil)
	if err == nil {
		t.Fatal("expected error for empty args with no manifest, got nil")
	}
	if got := err.Error(); got != "no epics to run (provide goals as arguments or use --manifest)" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestValidateParallelPrereqs_EmptySlice(t *testing.T) {
	old := parallelManifest
	parallelManifest = ""
	defer func() { parallelManifest = old }()

	_, _, _, err := validateParallelPrereqs([]string{})
	if err == nil {
		t.Fatal("expected error for empty slice, got nil")
	}
}

func TestCreateParallelWorktrees_CreatesAndCleansUp(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize a git repo in the temp dir with an initial commit.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, string(out), err)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("config", "commit.gpgsign", "false")

	// Create initial commit (required for worktree add).
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "init")

	// Must chdir to the git repo since createParallelWorktrees
	// runs git commands relative to CWD.
	t.Chdir(tmpDir)

	epics := []parallelEpic{
		{Name: "wt-alpha", Goal: "do alpha"},
		{Name: "wt-beta", Goal: "do beta"},
	}

	worktrees, err := createParallelWorktrees(tmpDir, epics, "claude")
	if err != nil {
		t.Fatalf("createParallelWorktrees: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify worktree paths exist.
	for i, wt := range worktrees {
		if _, err := os.Stat(wt.path); os.IsNotExist(err) {
			t.Errorf("worktree %d path does not exist: %s", i, wt.path)
		}
		expectedPath := filepath.Join(tmpDir, ".claude", "worktrees", epics[i].Name)
		if wt.path != expectedPath {
			t.Errorf("worktree %d path = %q, want %q", i, wt.path, expectedPath)
		}
		expectedBranch := "epic/" + epics[i].Name
		if wt.branch != expectedBranch {
			t.Errorf("worktree %d branch = %q, want %q", i, wt.branch, expectedBranch)
		}
	}

	// Clean up worktrees (so t.TempDir cleanup works).
	for _, wt := range worktrees {
		cmd := exec.Command("git", "worktree", "remove", "--force", wt.path)
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "branch", "-D", wt.branch)
		cmd.Dir = tmpDir
		_ = cmd.Run()
	}
}

func TestParallelWorktreeRoot(t *testing.T) {
	tmpDir := t.TempDir()
	tests := []struct {
		name       string
		runtimeCmd string
		want       string
	}{
		{
			name:       "default claude root",
			runtimeCmd: "claude",
			want:       filepath.Join(tmpDir, ".claude", "worktrees"),
		},
		{
			name:       "codex root",
			runtimeCmd: "codex",
			want:       filepath.Join(tmpDir, ".codex", "worktrees"),
		},
		{
			name:       "path runtime uses basename",
			runtimeCmd: "/usr/local/bin/codex",
			want:       filepath.Join(tmpDir, ".codex", "worktrees"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parallelWorktreeRoot(tmpDir, tt.runtimeCmd)
			if got != tt.want {
				t.Fatalf("parallelWorktreeRoot(%q) = %q, want %q", tt.runtimeCmd, got, tt.want)
			}
		})
	}
}

func TestMergeParallelWorktrees_SkipsFailedEpics(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, string(out), err)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("config", "commit.gpgsign", "false")

	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "init")

	// Use whatever default branch this git config created (main/master/etc.).
	currentBranch := func() string {
		t.Helper()
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git [branch --show-current]: %s: %v", string(out), err)
		}
		branch := strings.TrimSpace(string(out))
		if branch == "" {
			t.Fatal("default branch is empty")
		}
		return branch
	}
	defaultBranch := currentBranch()

	// Create a branch with a commit to merge.
	runGit("checkout", "-b", "epic/alpha")
	alphaFile := filepath.Join(tmpDir, "alpha.txt")
	if err := os.WriteFile(alphaFile, []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "alpha.txt")
	runGit("commit", "-m", "alpha work")
	runGit("checkout", defaultBranch)

	// Reset merge-order global.
	old := parallelMergeOrder
	parallelMergeOrder = ""
	defer func() { parallelMergeOrder = old }()

	epics := []parallelEpic{
		{Name: "alpha", Goal: "do alpha", MergeOrder: 1},
		{Name: "beta", Goal: "do beta", MergeOrder: 2},
	}
	results := []parallelResult{
		{Epic: epics[0], Success: true},
		{Epic: epics[1], Success: false, Error: fmt.Errorf("beta failed")},
	}
	worktrees := []worktreeInfo{
		{path: filepath.Join(tmpDir, ".wt", "alpha"), branch: "epic/alpha"},
		{path: filepath.Join(tmpDir, ".wt", "beta"), branch: "epic/beta"},
	}

	// Change to tmpDir so git merge runs in the right repo.
	t.Chdir(tmpDir)

	mergedCount, err := mergeParallelWorktrees(epics, results, worktrees)
	if err != nil {
		t.Fatalf("mergeParallelWorktrees: %v", err)
	}
	if mergedCount != 1 {
		t.Errorf("expected 1 merged, got %d", mergedCount)
	}
}

func TestSetupTmuxSession_NotRequested(t *testing.T) {
	// When parallelTmux is false, setupTmuxSession should return empty string and no error.
	old := parallelTmux
	parallelTmux = false
	defer func() { parallelTmux = old }()

	tmuxCmd, err := setupTmuxSession("test-session")
	if err != nil {
		t.Fatalf("setupTmuxSession: %v", err)
	}
	if tmuxCmd != "" {
		t.Errorf("expected empty tmuxCmd, got %q", tmuxCmd)
	}
}

func TestReportParallelResults_AllSuccess(t *testing.T) {
	results := []parallelResult{
		{Epic: parallelEpic{Name: "a"}, Success: true, CommitSHA: "abc1234", Duration: 0},
		{Epic: parallelEpic{Name: "b"}, Success: true, CommitSHA: "def5678", Duration: 0},
	}
	epics := []parallelEpic{{Name: "a"}, {Name: "b"}}

	allOK := reportParallelResults(results, epics, false, "", "")
	if !allOK {
		t.Error("expected allSuccess=true when all results succeed")
	}
}

func TestReportParallelResults_SomeFailed(t *testing.T) {
	results := []parallelResult{
		{Epic: parallelEpic{Name: "a"}, Success: true},
		{Epic: parallelEpic{Name: "b"}, Success: false, Error: fmt.Errorf("failed"), LogFile: "/tmp/b.log"},
	}
	epics := []parallelEpic{{Name: "a"}, {Name: "b"}}

	allOK := reportParallelResults(results, epics, false, "", "")
	if allOK {
		t.Error("expected allSuccess=false when some results failed")
	}
}

func TestSpawnParallelEpics_ArgumentConstruction(t *testing.T) {
	// spawnParallelEpics uses goroutines internally. We verify it returns
	// results of the right length even with an empty epic list.
	results := spawnParallelEpics(nil, nil, "echo", t.TempDir(), "", "", 1)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil epics, got %d", len(results))
	}
}

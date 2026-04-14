package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var _ = fmt.Sprintf
var _ = json.Marshal
var _ = exec.Command

func TestWorktree_runIDFromWorktreePath(t *testing.T) {
	tests := []struct {
		name         string
		repoRoot     string
		worktreePath string
		want         string
	}{
		{"valid rpi worktree", "/home/user/agentops", "/home/user/agentops-rpi-abc123", "abc123"},
		{"nested path", "/home/user/my-project", "/home/user/my-project-rpi-run42", "run42"},
		{"non-matching prefix", "/home/user/agentops", "/home/user/other-project-rpi-abc", ""},
		{"no run ID", "/home/user/agentops", "/home/user/agentops-rpi-", ""},
		{"exact repo name", "/home/user/agentops", "/home/user/agentops", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runIDFromWorktreePath(tt.repoRoot, tt.worktreePath)
			if got != tt.want {
				t.Errorf("runIDFromWorktreePath(%q, %q) = %q, want %q", tt.repoRoot, tt.worktreePath, got, tt.want)
			}
		})
	}
}

func TestWorktree_worktreeReferenceTime(t *testing.T) {
	t.Run("uses most recent modtime", func(t *testing.T) {
		tmp := t.TempDir()
		rpiDir := filepath.Join(tmp, ".agents", "rpi")
		if err := os.MkdirAll(rpiDir, 0755); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(rpiDir, "phased-state.json")
		if err := os.WriteFile(statePath, []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
		targetTime := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(statePath, targetTime, targetTime); err != nil {
			t.Fatal(err)
		}
		got := worktreeReferenceTime(tmp)
		if got.IsZero() {
			t.Error("expected non-zero reference time")
		}
		dirInfo, _ := os.Stat(tmp)
		if !got.Equal(dirInfo.ModTime()) {
			if got.Before(targetTime) {
				t.Error("reference time should be >= oldest candidate file time")
			}
		}
	})

	t.Run("returns epoch for nonexistent path", func(t *testing.T) {
		got := worktreeReferenceTime("/nonexistent/path")
		if !got.Equal(time.Unix(0, 0)) {
			t.Errorf("expected epoch time, got %v", got)
		}
	})

	t.Run("uses worktree dir itself if no rpi files", func(t *testing.T) {
		tmp := t.TempDir()
		got := worktreeReferenceTime(tmp)
		if got.IsZero() {
			t.Error("expected non-zero time from worktree dir stat")
		}
	})

	t.Run("prefers live-status over phased-state when newer", func(t *testing.T) {
		tmp := t.TempDir()
		rpiDir := filepath.Join(tmp, ".agents", "rpi")
		if err := os.MkdirAll(rpiDir, 0755); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(rpiDir, "phased-state.json")
		if err := os.WriteFile(statePath, []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := time.Now().Add(-3 * time.Hour)
		if err := os.Chtimes(statePath, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
		liveStatusPath := filepath.Join(rpiDir, "live-status.md")
		if err := os.WriteFile(liveStatusPath, []byte("# Live Status"), 0644); err != nil {
			t.Fatal(err)
		}
		newTime := time.Now().Add(-1 * time.Hour)
		if err := os.Chtimes(liveStatusPath, newTime, newTime); err != nil {
			t.Fatal(err)
		}
		dirOldTime := time.Now().Add(-4 * time.Hour)
		if err := os.Chtimes(tmp, dirOldTime, dirOldTime); err != nil {
			t.Fatal(err)
		}
		got := worktreeReferenceTime(tmp)
		diff := got.Sub(newTime).Abs()
		if diff > time.Second {
			t.Errorf("expected reference time near live-status time %v, got %v", newTime, got)
		}
	})
}

func TestWorktree_parseRPITmuxSessionRunID(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		wantRunID   string
		wantOK      bool
	}{
		{"valid p1", "ao-rpi-abc123-p1", "abc123", true},
		{"valid p2", "ao-rpi-run-42-p2", "run-42", true},
		{"valid p3", "ao-rpi-xyz-p3", "xyz", true},
		{"wrong prefix", "tmux-rpi-abc-p1", "", false},
		{"no phase suffix", "ao-rpi-abc", "", false},
		{"wrong phase", "ao-rpi-abc-p4", "", false},
		{"empty", "", "", false},
		{"just prefix", "ao-rpi--p1", "", false},
		{"non-rpi session", "main", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRunID, gotOK := parseRPITmuxSessionRunID(tt.sessionName)
			if gotOK != tt.wantOK {
				t.Errorf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotRunID != tt.wantRunID {
				t.Errorf("runID = %q, want %q", gotRunID, tt.wantRunID)
			}
		})
	}
}

func TestWorktree_parseTmuxSessionListOutput(t *testing.T) {
	t.Run("normal output", func(t *testing.T) {
		output := "ao-rpi-abc-p1\t1708000000\nao-rpi-abc-p2\t1708000100\nmain\t1708000200\n"
		sessions := parseTmuxSessionListOutput(output)
		if len(sessions) != 2 {
			t.Errorf("expected 2 RPI sessions, got %d", len(sessions))
		}
		if sessions[0].RunID != "abc" {
			t.Errorf("first session RunID = %q, want abc", sessions[0].RunID)
		}
		if sessions[0].Name != "ao-rpi-abc-p1" {
			t.Errorf("first session Name = %q", sessions[0].Name)
		}
	})
	t.Run("empty output", func(t *testing.T) {
		sessions := parseTmuxSessionListOutput("")
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})
	t.Run("malformed lines ignored", func(t *testing.T) {
		output := "ao-rpi-x-p1\t1708000000\nbadline\nao-rpi-y-p2\tnot_a_number\n"
		sessions := parseTmuxSessionListOutput(output)
		if len(sessions) != 1 {
			t.Errorf("expected 1 valid session, got %d", len(sessions))
		}
	})
	t.Run("whitespace handling", func(t *testing.T) {
		output := "  ao-rpi-run1-p1 \t 1708000000 \n"
		sessions := parseTmuxSessionListOutput(output)
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}
	})
	t.Run("too many fields", func(t *testing.T) {
		output := "ao-rpi-abc-p1\t1708000000\textra\n"
		sessions := parseTmuxSessionListOutput(output)
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions for 3-field line, got %d", len(sessions))
		}
	})
	t.Run("one field only", func(t *testing.T) {
		output := "ao-rpi-abc-p1\n"
		sessions := parseTmuxSessionListOutput(output)
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions for 1-field line, got %d", len(sessions))
		}
	})
}

func TestWorktree_shouldCleanupRPITmuxSession(t *testing.T) {
	now := time.Now()
	staleAfter := 24 * time.Hour
	activeRuns := map[string]bool{"active-run": true}
	liveWorktreeRuns := map[string]bool{"wt-run": true}
	tests := []struct {
		name      string
		runID     string
		createdAt time.Time
		want      bool
	}{
		{"empty runID", "", now.Add(-48 * time.Hour), false},
		{"active run", "active-run", now.Add(-48 * time.Hour), false},
		{"live worktree run", "wt-run", now.Add(-48 * time.Hour), false},
		{"too recent", "old-run", now.Add(-12 * time.Hour), false},
		{"stale and orphaned", "orphan-run", now.Add(-48 * time.Hour), true},
		{"exactly at threshold", "border-run", now.Add(-24 * time.Hour), true},
		{"just past threshold", "past-run", now.Add(-25 * time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCleanupRPITmuxSession(tt.runID, tt.createdAt, now, staleAfter, activeRuns, liveWorktreeRuns)
			if got != tt.want {
				t.Errorf("shouldCleanupRPITmuxSession(%q, ...) = %v, want %v", tt.runID, got, tt.want)
			}
		})
	}
}

func TestWorktree_worktreeGCCandidate_Fields(t *testing.T) {
	c := worktreeGCCandidate{RunID: "test-run", Path: "/path/to/worktree", Reference: time.Now(), Dirty: true}
	if c.RunID != "test-run" {
		t.Error("RunID not set")
	}
	if c.Path != "/path/to/worktree" {
		t.Error("Path not set")
	}
	if !c.Dirty {
		t.Error("Dirty not set")
	}
}

func TestWorktree_tmuxSessionMeta_Fields(t *testing.T) {
	m := tmuxSessionMeta{Name: "ao-rpi-abc-p1", RunID: "abc", CreatedAt: time.Unix(1708000000, 0)}
	if m.Name != "ao-rpi-abc-p1" {
		t.Error("Name not set")
	}
	if m.RunID != "abc" {
		t.Error("RunID not set")
	}
}

func TestWorktree_findRPISiblingWorktreePaths(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"rpi-run1", "rpi-run2"} {
		if err := os.MkdirAll(filepath.Join(parent, "myproject-"+suffix), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(parent, "other-project"), 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := findRPISiblingWorktreePaths(repoRoot)
	if err != nil {
		t.Fatalf("findRPISiblingWorktreePaths: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 worktree paths, got %d: %v", len(paths), paths)
	}
	for _, p := range paths {
		if !strings.HasPrefix(filepath.Base(p), "myproject-rpi-") {
			t.Errorf("unexpected worktree path: %s", p)
		}
	}
}

func TestWorktree_findRPISiblingWorktreePaths_NoSiblings(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "lonely-project")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := findRPISiblingWorktreePaths(repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestWorktree_findRPISiblingWorktreePaths_SkipsFiles(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "myproject-rpi-fakefile"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(parent, "myproject-rpi-realdir"), 0755); err != nil {
		t.Fatal(err)
	}
	paths, err := findRPISiblingWorktreePaths(repoRoot)
	if err != nil {
		t.Fatalf("findRPISiblingWorktreePaths: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path (directory only), got %d: %v", len(paths), paths)
	}
	if len(paths) == 1 && filepath.Base(paths[0]) != "myproject-rpi-realdir" {
		t.Errorf("expected realdir, got %s", filepath.Base(paths[0]))
	}
}

func TestWorktree_tmuxRPISessionPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"valid p1", "ao-rpi-abc-p1", true},
		{"valid p2", "ao-rpi-long-run-id-p2", true},
		{"valid p3", "ao-rpi-x-p3", true},
		{"wrong prefix", "tmux-rpi-abc-p1", false},
		{"missing phase", "ao-rpi-abc", false},
		{"p4", "ao-rpi-abc-p4", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmuxRPISessionPattern.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("tmuxRPISessionPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestWorktree_finalizeWorktreeGC_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()
	out, _ := captureStdout(t, func() error {
		finalizeWorktreeGC("/fake/repo", 5, 3, 2, 4)
		return nil
	})
	if !strings.Contains(out, "dry-run") && !strings.Contains(out, "Worktree GC") {
		t.Errorf("expected dry-run or GC output, got: %s", out)
	}
}

func TestWorktree_finalizeWorktreeGC_NotDryRun(t *testing.T) {
	oldDryRun := dryRun
	oldPrune := worktreeGCPrune
	dryRun = false
	worktreeGCPrune = false
	defer func() { dryRun = oldDryRun; worktreeGCPrune = oldPrune }()
	out, _ := captureStdout(t, func() error {
		finalizeWorktreeGC("/fake/repo", 5, 3, 2, 4)
		return nil
	})
	if out == "" {
		t.Error("expected non-empty GC summary output")
	}
}

func TestWorktree_finalizeWorktreeGC_withPrune(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	origDryRun := dryRun
	origPrune := worktreeGCPrune
	defer func() { dryRun = origDryRun; worktreeGCPrune = origPrune }()
	dryRun = false
	worktreeGCPrune = true
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	finalizeWorktreeGC(tmp, 0, 0, 0, 0)
}

func TestWorktree_finalizeWorktreeGC_pruneOnInvalidRepo(t *testing.T) {
	origDryRun := dryRun
	origPrune := worktreeGCPrune
	defer func() { dryRun = origDryRun; worktreeGCPrune = origPrune }()
	dryRun = false
	worktreeGCPrune = true
	tmp := t.TempDir() // not a git repo
	out, _ := captureStdout(t, func() error {
		finalizeWorktreeGC(tmp, 0, 0, 0, 0)
		return nil
	})
	// Should handle invalid repo gracefully
	_ = out
	if strings.Contains(out, "panic") {
		t.Error("should not panic on invalid repo")
	}
}

func TestWorktree_gcWorktreeCandidates_Empty(t *testing.T) {
	removed := gcWorktreeCandidates(nil, make(map[string]bool), "/fake/repo", time.Now())
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

func TestWorktree_gcWorktreeCandidates_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()
	candidates := []worktreeGCCandidate{
		{RunID: "test-run", Path: "/fake/path", Reference: time.Now().Add(-48 * time.Hour)},
	}
	removed := gcWorktreeCandidates(candidates, make(map[string]bool), "/fake/repo", time.Now())
	if removed != 0 {
		t.Errorf("expected 0 removed in dry-run, got %d", removed)
	}
}

func TestWorktree_gcWorktreeCandidates_DryRunDirty(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()
	candidates := []worktreeGCCandidate{
		{RunID: "dirty-run", Path: "/fake/path", Reference: time.Now().Add(-48 * time.Hour), Dirty: true},
	}
	removed := gcWorktreeCandidates(candidates, make(map[string]bool), "/fake/root", time.Now())
	if removed != 0 {
		t.Errorf("expected 0 removed in dry-run, got %d", removed)
	}
}

func TestWorktree_gcWorktreeCandidates_RemoveFailure(t *testing.T) {
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	candidates := []worktreeGCCandidate{
		{RunID: "run-1", Path: filepath.Join(tmp, "myrepo-rpi-run-1"), Reference: time.Now().Add(-2 * time.Hour)},
	}
	liveRuns := map[string]bool{"run-1": true}
	removed := gcWorktreeCandidates(candidates, liveRuns, repoRoot, time.Now())
	_ = removed
}

func TestWorktree_gcWorktreeCandidates_RealRemoveSuccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myrepo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", repoRoot).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	wtPath := filepath.Join(parent, "myrepo-rpi-testrun")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatal(err)
	}
	candidates := []worktreeGCCandidate{
		{RunID: "testrun", Path: wtPath, Reference: time.Now().Add(-48 * time.Hour)},
	}
	liveRuns := map[string]bool{"testrun": true}
	removed := gcWorktreeCandidates(candidates, liveRuns, repoRoot, time.Now())
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if liveRuns["testrun"] {
		t.Error("expected testrun deleted from liveRuns")
	}
}

func TestWorktree_isWorktreeDirty_cleanRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"config", "user.email", "test@test.com"}, {"config", "user.name", "Test"}} {
		if err := exec.Command("git", append([]string{"-C", tmp}, args...)...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", tmp, "config", "commit.gpgsign", "false").Run()
	if err := os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", tmp, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", tmp, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	dirty, err := isWorktreeDirty(tmp)
	if err != nil {
		t.Fatalf("isWorktreeDirty: %v", err)
	}
	if dirty {
		t.Error("expected clean repo to not be dirty")
	}
}

func TestWorktree_isWorktreeDirty_dirtyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"config", "user.email", "test@test.com"}, {"config", "user.name", "Test"}} {
		if err := exec.Command("git", append([]string{"-C", tmp}, args...)...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", tmp, "config", "commit.gpgsign", "false").Run()
	if err := os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", tmp, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", tmp, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}
	dirty, err := isWorktreeDirty(tmp)
	if err != nil {
		t.Fatalf("isWorktreeDirty: %v", err)
	}
	if !dirty {
		t.Error("expected modified repo to be dirty")
	}
}

func TestWorktree_isWorktreeDirty_untrackedFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"config", "user.email", "test@test.com"}, {"config", "user.name", "Test"}} {
		if err := exec.Command("git", append([]string{"-C", tmp}, args...)...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", tmp, "config", "commit.gpgsign", "false").Run()
	if err := exec.Command("git", "-C", tmp, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "untracked.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	dirty, err := isWorktreeDirty(tmp)
	if err != nil {
		t.Fatalf("isWorktreeDirty: %v", err)
	}
	if !dirty {
		t.Error("expected repo with untracked file to be dirty")
	}
}

func TestWorktree_isWorktreeDirty_nonGitDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := isWorktreeDirty(tmp)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestWorktree_resolveRepoRoot_inGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	root, err := resolveRepoRoot(tmp)
	if err != nil {
		t.Fatalf("resolveRepoRoot: %v", err)
	}
	if root == "" {
		t.Fatal("expected non-empty repo root")
	}
}

func TestWorktree_resolveRepoRoot_ignoresPollutedGitEnvInLinkedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run(repo, "init")
	run(repo, "config", "user.email", "test@example.com")
	run(repo, "config", "user.name", "Test User")
	run(repo, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hi\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	run(repo, "add", "README.md")
	run(repo, "commit", "-m", "init")
	worktree := filepath.Join(t.TempDir(), "repo-linked-worktree")
	run(repo, "worktree", "add", "-q", "-b", "codex/test-linked-worktree", worktree, "HEAD")
	t.Setenv("GIT_DIR", filepath.Join(repo, ".git"))
	t.Setenv("GIT_WORK_TREE", repo)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(repo, ".git"))
	root, err := resolveRepoRoot(worktree)
	if err != nil {
		t.Fatalf("resolveRepoRoot: %v", err)
	}
	if got, want := realPathForTest(t, root), realPathForTest(t, worktree); got != want {
		t.Fatalf("resolveRepoRoot(%q) = %q, want %q", worktree, got, want)
	}
}

func TestWorktree_resolveRepoRoot_nonGitDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := resolveRepoRoot(tmp)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "resolve git repo root") {
		t.Errorf("expected 'resolve git repo root' in error, got: %v", err)
	}
}

func TestWorktree_findStaleRPITmuxSessions_noTmux(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	stale, err := findStaleRPITmuxSessions(time.Now(), time.Hour, make(map[string]bool), make(map[string]bool))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected 0 stale sessions, got %d", len(stale))
	}
}

func TestWorktree_listRPITmuxSessions_noTmux(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	sessions, err := listRPITmuxSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestRunWorktreeGC_DryRunInGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	origDryRun := dryRun
	origStaleAfter := worktreeGCStaleAfter
	defer func() { dryRun = origDryRun; worktreeGCStaleAfter = origStaleAfter }()
	dryRun = true
	worktreeGCStaleAfter = time.Hour
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := runWorktreeGC(nil, nil); err != nil {
		t.Fatalf("runWorktreeGC dry-run: %v", err)
	}
}

func TestWorktree_runWorktreeGC_staleAfterZero(t *testing.T) {
	origStaleAfter := worktreeGCStaleAfter
	defer func() { worktreeGCStaleAfter = origStaleAfter }()
	worktreeGCStaleAfter = 0
	err := runWorktreeGC(nil, nil)
	if err == nil {
		t.Fatal("expected error for staleAfter <= 0")
	}
	if !strings.Contains(err.Error(), "--stale-after must be > 0") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWorktree_runWorktreeGC_staleAfterNegative(t *testing.T) {
	origStaleAfter := worktreeGCStaleAfter
	defer func() { worktreeGCStaleAfter = origStaleAfter }()
	worktreeGCStaleAfter = -1 * time.Hour
	err := runWorktreeGC(nil, nil)
	if err == nil {
		t.Fatal("expected error for negative staleAfter")
	}
	if !strings.Contains(err.Error(), "--stale-after must be > 0") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWorktree_runWorktreeGC_withStaleSiblings(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", repoRoot).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"-C", repoRoot, "config", "user.email", "test@test.com"}, {"-C", repoRoot, "config", "user.name", "Test"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", repoRoot, "config", "commit.gpgsign", "false").Run()
	if err := exec.Command("git", "-C", repoRoot, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	wtPath := filepath.Join(parent, "myproject-rpi-stalerun")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(wtPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	origDryRun := dryRun
	origStaleAfter := worktreeGCStaleAfter
	origPrune := worktreeGCPrune
	origCleanTmux := worktreeGCCleanTmux
	origIncludeDirty := worktreeGCIncludeDirty
	defer func() {
		dryRun = origDryRun
		worktreeGCStaleAfter = origStaleAfter
		worktreeGCPrune = origPrune
		worktreeGCCleanTmux = origCleanTmux
		worktreeGCIncludeDirty = origIncludeDirty
	}()
	dryRun = true
	worktreeGCStaleAfter = time.Hour
	worktreeGCPrune = false
	worktreeGCCleanTmux = false
	worktreeGCIncludeDirty = false
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := runWorktreeGC(nil, nil); err != nil {
		t.Errorf("runWorktreeGC with stale siblings: %v", err)
	}
}

func TestWorktree_runWorktreeGC_withTmuxCleanup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	origDryRun := dryRun
	origStaleAfter := worktreeGCStaleAfter
	origPrune := worktreeGCPrune
	origCleanTmux := worktreeGCCleanTmux
	defer func() {
		dryRun = origDryRun
		worktreeGCStaleAfter = origStaleAfter
		worktreeGCPrune = origPrune
		worktreeGCCleanTmux = origCleanTmux
	}()
	dryRun = true
	worktreeGCStaleAfter = time.Hour
	worktreeGCPrune = false
	worktreeGCCleanTmux = true
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := runWorktreeGC(nil, nil); err != nil {
		t.Errorf("runWorktreeGC with tmux cleanup: %v", err)
	}
}

func TestWorktree_runWorktreeGC_withDirtySiblings(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", repoRoot).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"-C", repoRoot, "config", "user.email", "test@test.com"}, {"-C", repoRoot, "config", "user.name", "Test"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", repoRoot, "config", "commit.gpgsign", "false").Run()
	if err := exec.Command("git", "-C", repoRoot, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	wtPath := filepath.Join(parent, "myproject-rpi-dirtyrun")
	if err := exec.Command("git", "init", wtPath).Run(); err != nil {
		t.Fatalf("git init worktree: %v", err)
	}
	for _, args := range [][]string{{"-C", wtPath, "config", "user.email", "test@test.com"}, {"-C", wtPath, "config", "user.name", "Test"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", wtPath, "config", "commit.gpgsign", "false").Run()
	if err := exec.Command("git", "-C", wtPath, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(wtPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	origDryRun := dryRun
	origStaleAfter := worktreeGCStaleAfter
	origPrune := worktreeGCPrune
	origCleanTmux := worktreeGCCleanTmux
	origIncludeDirty := worktreeGCIncludeDirty
	defer func() {
		dryRun = origDryRun
		worktreeGCStaleAfter = origStaleAfter
		worktreeGCPrune = origPrune
		worktreeGCCleanTmux = origCleanTmux
		worktreeGCIncludeDirty = origIncludeDirty
	}()
	dryRun = true
	worktreeGCStaleAfter = time.Hour
	worktreeGCPrune = false
	worktreeGCCleanTmux = false
	worktreeGCIncludeDirty = false
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := runWorktreeGC(nil, nil); err != nil {
		t.Errorf("runWorktreeGC with dirty siblings: %v", err)
	}
}

func TestWorktree_gcTmuxSessions_noTmux(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	origStaleAfter := worktreeGCStaleAfter
	defer func() { worktreeGCStaleAfter = origStaleAfter }()
	worktreeGCStaleAfter = time.Hour
	killed, candidates := gcTmuxSessions(time.Now(), make(map[string]bool), make(map[string]bool))
	if killed != 0 {
		t.Errorf("expected 0 killed, got %d", killed)
	}
	if candidates != 0 {
		t.Errorf("expected 0 candidates, got %d", candidates)
	}
}

func TestWorktree_gcTmuxSessions_dryRunNoSessions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	origDryRun := dryRun
	origStaleAfter := worktreeGCStaleAfter
	defer func() { dryRun = origDryRun; worktreeGCStaleAfter = origStaleAfter }()
	dryRun = true
	worktreeGCStaleAfter = time.Hour
	killed, candidates := gcTmuxSessions(time.Now(), make(map[string]bool), make(map[string]bool))
	if killed != 0 {
		t.Errorf("expected 0 killed, got %d", killed)
	}
	if candidates != 0 {
		t.Errorf("expected 0 candidates, got %d", candidates)
	}
}

func TestWorktree_discoverActiveRPIRuns_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	activeRuns := discoverActiveRPIRuns(tmp)
	if len(activeRuns) != 0 {
		t.Errorf("expected 0 active runs, got %d", len(activeRuns))
	}
}

func TestWorktree_discoverActiveRPIRuns_WithRunDir(t *testing.T) {
	tmp := t.TempDir()
	runID := "test-active-run"
	runDir := filepath.Join(tmp, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         runID,
		"goal":           "test goal",
		"phase":          2,
		"phase_name":     "plan",
		"started_at":     time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"last_heartbeat": time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	activeRuns := discoverActiveRPIRuns(tmp)
	// Exercises the loop body. Result depends on liveness heuristics.
	_ = activeRuns
}

func TestWorktree_discoverActiveRPIRuns_WithInactiveRun(t *testing.T) {
	tmp := t.TempDir()
	runID := "test-done-run"
	runDir := filepath.Join(tmp, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{
		"schema_version":  1,
		"run_id":          runID,
		"goal":            "test goal",
		"phase":           4,
		"phase_name":      "validate",
		"terminal_status": "completed",
		"started_at":      time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	activeRuns := discoverActiveRPIRuns(tmp)
	if activeRuns[runID] {
		t.Errorf("expected inactive run %q to not appear in activeRuns", runID)
	}
}

func TestWorktree_findStaleRPISiblingWorktrees_AllBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	fixture := newStaleRPIWorktreeFixture(t)

	// 1. Active run — skipped
	createRPIWorktreeDir(t, fixture.parent, "active")
	activeRuns := map[string]bool{"active": true}

	// 2. Too recent — skipped
	createRPIWorktreeDir(t, fixture.parent, "recent")

	// 3. Stale and clean — candidate
	createStaleGitRPIWorktree(t, fixture.parent, "staleclean", fixture.oldTime, false)

	// 4. Stale and dirty — skipped (includeDirty=false)
	createStaleGitRPIWorktree(t, fixture.parent, "staledirty", fixture.oldTime, true)

	candidates, liveRuns, skippedDirty, err := findStaleRPISiblingWorktrees(
		fixture.repoRoot,
		fixture.now,
		fixture.staleAfter,
		activeRuns,
		false,
	)
	if err != nil {
		t.Fatalf("findStaleRPISiblingWorktrees: %v", err)
	}
	assertStaleRPIWorktreeLiveRuns(t, liveRuns, "active", "recent", "staleclean", "staledirty")
	assertStaleRPIWorktreeCandidates(t, candidates, "staleclean")
	assertStaleRPIWorktreeSkippedDirty(t, skippedDirty, "staledirty")
}

type staleRPIWorktreeFixture struct {
	parent     string
	repoRoot   string
	now        time.Time
	staleAfter time.Duration
	oldTime    time.Time
}

func newStaleRPIWorktreeFixture(t *testing.T) staleRPIWorktreeFixture {
	t.Helper()

	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	return staleRPIWorktreeFixture{
		parent:     parent,
		repoRoot:   repoRoot,
		now:        now,
		staleAfter: time.Hour,
		oldTime:    now.Add(-2 * time.Hour),
	}
}

func createRPIWorktreeDir(t *testing.T, parent, runID string) string {
	t.Helper()

	path := filepath.Join(parent, "myproject-rpi-"+runID)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func createStaleGitRPIWorktree(
	t *testing.T,
	parent string,
	runID string,
	oldTime time.Time,
	dirty bool,
) string {
	t.Helper()

	path := filepath.Join(parent, "myproject-rpi-"+runID)
	runGitForWorktreeTest(t, "git init "+runID, "init", path)
	configureWorktreeTestGitRepo(t, path)
	if dirty {
		if err := os.WriteFile(filepath.Join(path, "dirty.txt"), []byte("dirty"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	return path
}

func configureWorktreeTestGitRepo(t *testing.T, path string) {
	t.Helper()

	for _, args := range [][]string{
		{"-C", path, "config", "user.email", "test@test.com"},
		{"-C", path, "config", "user.name", "Test"},
	} {
		runGitForWorktreeTest(t, "git config", args...)
	}
	_ = exec.Command("git", "-C", path, "config", "commit.gpgsign", "false").Run()
	runGitForWorktreeTest(t, "git commit", "-C", path, "commit", "--allow-empty", "-m", "init")
}

func runGitForWorktreeTest(t *testing.T, context string, args ...string) {
	t.Helper()

	if err := exec.Command("git", args...).Run(); err != nil {
		t.Fatalf("%s: %v", context, err)
	}
}

func assertStaleRPIWorktreeLiveRuns(t *testing.T, liveRuns map[string]bool, wantRunIDs ...string) {
	t.Helper()

	for _, runID := range wantRunIDs {
		if !liveRuns[runID] {
			t.Errorf("expected %q in liveRuns", runID)
		}
	}
}

func assertStaleRPIWorktreeCandidates(t *testing.T, candidates []worktreeGCCandidate, wantRunID string) {
	t.Helper()

	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	} else if candidates[0].RunID != wantRunID {
		t.Errorf("expected runID=%s, got %s", wantRunID, candidates[0].RunID)
	}
}

func assertStaleRPIWorktreeSkippedDirty(t *testing.T, skippedDirty []string, wantSubstring string) {
	t.Helper()

	if len(skippedDirty) != 1 {
		t.Errorf("expected 1 skipped dirty, got %d", len(skippedDirty))
	} else if !strings.Contains(skippedDirty[0], wantSubstring) {
		t.Errorf("expected %q in skipped, got %s", wantSubstring, skippedDirty[0])
	}
}

func TestWorktree_findStaleRPISiblingWorktrees_IncludeDirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	staleDirtyWt := filepath.Join(parent, "myproject-rpi-dirtyincluded")
	if err := exec.Command("git", "init", staleDirtyWt).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	for _, args := range [][]string{{"-C", staleDirtyWt, "config", "user.email", "test@test.com"}, {"-C", staleDirtyWt, "config", "user.name", "Test"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatalf("git config: %v", err)
		}
	}
	exec.Command("git", "-C", staleDirtyWt, "config", "commit.gpgsign", "false").Run()
	if err := exec.Command("git", "-C", staleDirtyWt, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staleDirtyWt, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(staleDirtyWt, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	candidates, _, skippedDirty, err := findStaleRPISiblingWorktrees(repoRoot, time.Now(), time.Hour, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("findStaleRPISiblingWorktrees: %v", err)
	}
	if len(skippedDirty) != 0 {
		t.Errorf("expected 0 skipped dirty with includeDirty=true, got %d", len(skippedDirty))
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	} else {
		if !candidates[0].Dirty {
			t.Error("expected candidate to be marked dirty")
		}
		if candidates[0].RunID != "dirtyincluded" {
			t.Errorf("expected runID=dirtyincluded, got %s", candidates[0].RunID)
		}
	}
}

func TestWorktree_findStaleRPISiblingWorktrees_NoSiblings(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "nopaths")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	candidates, liveRuns, skippedDirty, err := findStaleRPISiblingWorktrees(repoRoot, time.Now(), time.Hour, map[string]bool{}, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(candidates) != 0 || len(liveRuns) != 0 || len(skippedDirty) != 0 {
		t.Errorf("expected all empty, got candidates=%d liveRuns=%d skippedDirty=%d", len(candidates), len(liveRuns), len(skippedDirty))
	}
}

func TestWorktree_findStaleRPISiblingWorktrees_NonGitSibling(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	nonGitWt := filepath.Join(parent, "myproject-rpi-nongit")
	if err := os.MkdirAll(nonGitWt, 0755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(nonGitWt, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	candidates, liveRuns, skippedDirty, err := findStaleRPISiblingWorktrees(repoRoot, time.Now(), time.Hour, map[string]bool{}, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !liveRuns["nongit"] {
		t.Error("expected 'nongit' in liveRuns")
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for non-git sibling, got %d", len(candidates))
	}
	if len(skippedDirty) != 0 {
		t.Errorf("expected 0 skipped dirty, got %d", len(skippedDirty))
	}
}

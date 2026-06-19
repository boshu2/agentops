package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

// findCheck returns the CheckStatus with the given id, or a zero value + false.
func findCheck(checks []CheckStatus, id string) (CheckStatus, bool) {
	for _, c := range checks {
		if c.ID == id {
			return c, true
		}
	}
	return CheckStatus{}, false
}

func TestConcurrencyChecks_WorktreeIsolation(t *testing.T) {
	tests := []struct {
		name       string
		gitIsDir   bool // true: .git is a directory (canonical checkout); false: a file (linked worktree)
		wantStatus string
	}{
		{name: "canonical checkout (.git dir) warns", gitIsDir: true, wantStatus: VerdictStatusWarn},
		{name: "linked worktree (.git file) passes", gitIsDir: false, wantStatus: VerdictStatusPass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			gitPath := filepath.Join(root, ".git")
			if tt.gitIsDir {
				if err := os.Mkdir(gitPath, 0o755); err != nil {
					t.Fatalf("mkdir .git: %v", err)
				}
			} else {
				if err := os.WriteFile(gitPath, []byte("gitdir: /elsewhere/.git/worktrees/x\n"), 0o644); err != nil {
					t.Fatalf("write .git file: %v", err)
				}
			}
			// Isolate the push-lock check from the host: point TMPDIR at a clean dir.
			t.Setenv("TMPDIR", t.TempDir())

			checks := concurrencyChecks(root)
			got, ok := findCheck(checks, "worktree_isolation")
			if !ok {
				t.Fatal("worktree_isolation check missing")
			}
			if got.Status != tt.wantStatus {
				t.Errorf("worktree_isolation status = %q, want %q (detail: %q)", got.Status, tt.wantStatus, got.Detail)
			}
		})
	}
}

func TestConcurrencyChecks_PushLock(t *testing.T) {
	tests := []struct {
		name       string
		lockHeld   bool
		wantStatus string
	}{
		{name: "lock free passes", lockHeld: false, wantStatus: VerdictStatusPass},
		{name: "lock held warns", lockHeld: true, wantStatus: VerdictStatusWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			// Give worktree_isolation a deterministic PASS (linked-worktree .git file)
			// so it doesn't affect the push_lock assertion.
			if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: x\n"), 0o644); err != nil {
				t.Fatalf("write .git: %v", err)
			}
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			if tt.lockHeld {
				if err := os.Mkdir(filepath.Join(tmp, "agentops-push.lock"), 0o755); err != nil {
					t.Fatalf("mkdir lock: %v", err)
				}
			}

			checks := concurrencyChecks(root)
			got, ok := findCheck(checks, "push_lock")
			if !ok {
				t.Fatal("push_lock check missing")
			}
			if got.Status != tt.wantStatus {
				t.Errorf("push_lock status = %q, want %q", got.Status, tt.wantStatus)
			}
		})
	}
}

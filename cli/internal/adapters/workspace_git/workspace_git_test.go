// practices: [hexagonal-architecture, tdd]
package workspace_git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// initTestRepo creates a real git repo with one commit in a temp dir and
// returns its root. It skips the test when git is unavailable.
func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "seed")
	return root
}

func TestAdapter_SetupCreatesWorktreeOnBranch(t *testing.T) {
	root := initTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-feature")
	a := New(root)

	res, err := a.Setup(context.Background(), ports.WorkspaceRequest{
		WorkspaceID: "run-1",
		Path:        wtPath,
		Metadata:    map[string]string{"branch": "feature/x"},
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if res.Status != "setup" {
		t.Fatalf("status = %q, want setup", res.Status)
	}
	if res.WorkspaceID != "run-1" || res.Path != wtPath {
		t.Fatalf("result = %+v, want id=run-1 path=%s", res, wtPath)
	}
	// The worktree directory and a checkout of README must exist on disk.
	if _, err := os.Stat(filepath.Join(wtPath, "README.md")); err != nil {
		t.Fatalf("worktree checkout missing: %v", err)
	}
	// The new branch must exist.
	if !branchExists(t, root, "feature/x") {
		t.Fatal("branch feature/x was not created")
	}
}

func TestAdapter_CleanupRemovesWorktreeAndBranch(t *testing.T) {
	root := initTestRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-feature")
	a := New(root)

	if _, err := a.Setup(context.Background(), ports.WorkspaceRequest{
		WorkspaceID: "run-1",
		Path:        wtPath,
		Metadata:    map[string]string{"branch": "feature/x"},
	}); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	res, err := a.Cleanup(context.Background(), ports.WorkspaceRequest{
		WorkspaceID: "run-1",
		Path:        wtPath,
		Metadata:    map[string]string{"branch": "feature/x", "prune": "true"},
	})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if res.Status != "cleanup" {
		t.Fatalf("status = %q, want cleanup", res.Status)
	}
	// Directory must be gone.
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still present: err=%v", err)
	}
	// Branch must be deleted.
	if branchExists(t, root, "feature/x") {
		t.Fatal("branch feature/x was not deleted")
	}
}

func TestAdapter_CleanupFallsBackWhenWorktreeUnknown(t *testing.T) {
	root := initTestRepo(t)
	// A directory git does not track as a worktree.
	orphan := filepath.Join(t.TempDir(), "orphan")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(root)

	res, err := a.Cleanup(context.Background(), ports.WorkspaceRequest{
		WorkspaceID: "run-1",
		Path:        orphan,
	})
	if err != nil {
		t.Fatalf("Cleanup fallback: %v", err)
	}
	if res.Status != "cleanup" {
		t.Fatalf("status = %q, want cleanup", res.Status)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan dir still present after fallback: err=%v", err)
	}
}

func TestAdapter_RejectsEmptyWorkspaceID(t *testing.T) {
	a := New("")
	if _, err := a.Setup(context.Background(), ports.WorkspaceRequest{Path: "/tmp/x"}); err == nil {
		t.Fatal("expected Setup error for empty workspace id")
	}
	if _, err := a.Cleanup(context.Background(), ports.WorkspaceRequest{Path: "/tmp/x"}); err == nil {
		t.Fatal("expected Cleanup error for empty workspace id")
	}
}

func TestAdapter_RejectsMissingPath(t *testing.T) {
	a := New("")
	_, err := a.Setup(context.Background(), ports.WorkspaceRequest{WorkspaceID: "run-1"})
	if !errors.Is(err, ErrMissingPath) {
		t.Fatalf("Setup err = %v, want ErrMissingPath", err)
	}
	_, err = a.Cleanup(context.Background(), ports.WorkspaceRequest{WorkspaceID: "run-1"})
	if !errors.Is(err, ErrMissingPath) {
		t.Fatalf("Cleanup err = %v, want ErrMissingPath", err)
	}
}

func TestAdapter_HonorsContextCancellation(t *testing.T) {
	a := New("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Setup(ctx, ports.WorkspaceRequest{WorkspaceID: "run-1", Path: "/tmp/x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Setup err = %v, want context.Canceled", err)
	}
	if _, err := a.Cleanup(ctx, ports.WorkspaceRequest{WorkspaceID: "run-1", Path: "/tmp/x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Cleanup err = %v, want context.Canceled", err)
	}
}

// branchExists reports whether refs/heads/<name> resolves in root.
func branchExists(t *testing.T, root, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = root
	return cmd.Run() == nil
}

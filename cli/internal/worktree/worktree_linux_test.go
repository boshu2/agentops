//go:build linux

package worktree

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// TestResolveAbsPath_DoubleFailure_Linux exercises the branch where both
// filepath.EvalSymlinks AND filepath.Abs fail. On Linux, os.Getwd() fails
// after the working directory is deleted, causing filepath.Abs to error on
// relative paths. This branch is unreachable on macOS because the kernel
// caches the cwd path even after the directory is removed.
func TestResolveAbsPath_DoubleFailure_Linux(t *testing.T) {
	// Lock to this OS thread so Chdir only affects us.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save original working directory to restore later.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = syscall.Chdir(origWD) }()

	// Create a temporary directory, chdir into it, then delete it.
	tmp, err := os.MkdirTemp("", "resolve-abs-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.RemoveAll(tmp); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	// On Linux, Getwd should now fail because the cwd no longer exists.
	if _, err := os.Getwd(); err == nil {
		t.Skip("os.Getwd() succeeded after cwd deletion; branch unreachable on this kernel")
	}

	// Use a relative path so EvalSymlinks fails (no such file) and
	// Abs fails (Getwd error). This exercises worktree.go:427-429.
	_, err = resolveAbsPath("nonexistent-relative-path")
	if err == nil {
		t.Fatal("expected error from resolveAbsPath when both EvalSymlinks and Abs fail")
	}
	if !strings.Contains(err.Error(), "invalid worktree path") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestResolveRemovePaths_AbsPathError_Linux exercises the resolveAbsPath
// error propagation path in resolveRemovePaths (worktree.go:438-439).
// It requires the same deleted-cwd trick as the resolveAbsPath test above.
func TestResolveRemovePaths_AbsPathError_Linux(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = syscall.Chdir(origWD) }()

	tmp, err := os.MkdirTemp("", "remove-paths-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.RemoveAll(tmp); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	if _, err := os.Getwd(); err == nil {
		t.Skip("os.Getwd() succeeded after cwd deletion; branch unreachable on this kernel")
	}

	// resolveRemovePaths calls resolveAbsPath internally.
	// With a relative worktree path and deleted cwd, resolveAbsPath fails,
	// and resolveRemovePaths should propagate that error.
	_, _, _, err = resolveRemovePaths("/some/repo", "relative-worktree", "run123")
	if err == nil {
		t.Fatal("expected error from resolveRemovePaths when resolveAbsPath fails")
	}
	if !strings.Contains(err.Error(), "invalid worktree path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

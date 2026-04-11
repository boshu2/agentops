//go:build !windows

package rpi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// TestAcquireMergeLock_LockFileFails_FIFO exercises the lockFile error branch
// in acquireMergeLock (line 246-248) by pre-creating the lock file as a FIFO.
// On macOS/BSD, flock on a FIFO returns ENOTSUP, causing lockFile to fail
// after OpenFile succeeds.
func TestAcquireMergeLock_LockFileFails_FIFO(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("flock on FIFO only fails on macOS/BSD (ENOTSUP); Linux allows it")
	}
	tmp := t.TempDir()
	lockDir := filepath.Join(tmp, ".git", "agentops")
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, "merge.lock")
	if err := syscall.Mkfifo(lockPath, 0o600); err != nil {
		t.Skipf("cannot create FIFO (platform unsupported): %v", err)
	}

	f, err := acquireMergeLock(tmp)
	if err == nil {
		releaseMergeLock(f)
		t.Fatal("expected error from acquireMergeLock on FIFO lock file")
	}
	if !strings.Contains(err.Error(), "acquire merge lock") {
		t.Errorf("expected 'acquire merge lock' error, got: %v", err)
	}
}

func TestAcquireMergeLock_OpenFileFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Exercise acquireMergeLock where os.OpenFile fails. This relies on Unix
	// chmod semantics; Windows ACLs do not make the directory unwritable here.
	tmp := t.TempDir()
	lockDir := filepath.Join(tmp, ".git", "agentops")
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(lockDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockDir, 0o750) })

	_, err := acquireMergeLock(tmp)
	if err == nil {
		t.Fatal("expected error when lock file cannot be opened")
	}
	if !strings.Contains(err.Error(), "open merge lock") {
		t.Errorf("expected 'open merge lock' error, got: %v", err)
	}
}

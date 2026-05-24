package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsStale_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	stale, err := IsStale(filepath.Join(dir, "nope.lock"), time.Minute)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stale {
		t.Error("expected not stale for missing file")
	}
}

func TestIsStale_FreshLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "fresh.lock")
	if err := WritePID(lockPath); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	stale, err := IsStale(lockPath, 24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stale {
		t.Error("expected fresh lock to be not stale")
	}
}

func TestIsStale_OldLockLivePID(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "old-live.lock")
	if err := WritePID(lockPath); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	// Backdate mtime to 24h ago.
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	stale, err := IsStale(lockPath, time.Hour)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stale {
		t.Error("expected not stale: pid is current (live) process")
	}
}

func TestIsStale_OldLockDeadPID(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "old-dead.lock")
	// Write a definitely-dead PID (impossibly large).
	if err := os.WriteFile(lockPath, []byte("2147480000\n"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	stale, err := IsStale(lockPath, time.Hour)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !stale {
		t.Error("expected stale for old lock with dead PID")
	}
}

func TestReadPID_Missing(t *testing.T) {
	dir := t.TempDir()
	if pid := ReadPID(filepath.Join(dir, "nope.lock")); pid != 0 {
		t.Errorf("expected 0, got %d", pid)
	}
}

func TestReadPID_Malformed(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "bad.lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if pid := ReadPID(lockPath); pid != 0 {
		t.Errorf("expected 0 for malformed, got %d", pid)
	}
}

func TestReadPID_Valid(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "valid.lock")
	if err := os.WriteFile(lockPath, []byte("12345\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if pid := ReadPID(lockPath); pid != 12345 {
		t.Errorf("expected 12345, got %d", pid)
	}
}

func TestWritePID_CreatesAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "sub", "pid.lock")

	if err := WritePID(lockPath); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first := ReadPID(lockPath)
	if first != os.Getpid() {
		t.Errorf("expected %d, got %d", os.Getpid(), first)
	}

	// Stuff some extra content in, then overwrite — must not append.
	if err := os.WriteFile(lockPath, []byte("111\nLEFTOVER\nMORE\n"), 0o644); err != nil {
		t.Fatalf("seed leftover: %v", err)
	}
	if err := WritePID(lockPath); err != nil {
		t.Fatalf("second write: %v", err)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "LEFTOVER") {
		t.Errorf("expected truncation, got %q", data)
	}
	if pid := ReadPID(lockPath); pid != os.Getpid() {
		t.Errorf("expected %d after rewrite, got %d", os.Getpid(), pid)
	}
}

func TestProcessAlive_CurrentProcess(t *testing.T) {
	if !ProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestProcessAlive_DefinitelyDead(t *testing.T) {
	// A PID of 0 is never a valid target.
	if ProcessAlive(0) {
		t.Error("pid 0 should not be alive")
	}
	if ProcessAlive(-1) {
		t.Error("negative pid should not be alive")
	}
	// Very large PID. Most systems cap at ~4M; 2_147_480_000 is safely dead.
	if ProcessAlive(2147480000) {
		t.Error("huge pid should not be alive")
	}
}

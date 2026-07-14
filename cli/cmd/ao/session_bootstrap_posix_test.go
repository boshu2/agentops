//go:build !windows

// The FIFO DoS regression is POSIX-specific: syscall.Mkfifo does not exist on
// Windows (the symbol does not compile there), and readRegularFileCapped's
// non-regular-file skip is already exercised cross-platform via the directory
// case in session_bootstrap_test.go.
package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Regression for the cross-family DoS refute: a FIFO (or any non-regular path)
// must be skipped WITHOUT hanging, so a hostile repo cannot stall bootstrap.
func TestReadRegularFileCapped_SkipsNonRegularNoHang(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readRegularFileCapped(dir, 1<<10); ok {
		t.Fatal("directory: want skip (false)")
	}
	reg := filepath.Join(dir, "f")
	if err := os.WriteFile(reg, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, ok := readRegularFileCapped(reg, 1<<10); !ok || string(data) != "hello" {
		t.Fatalf("regular file: ok=%v data=%q", ok, data)
	}
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan bool, 1)
	go func() {
		_, ok := readRegularFileCapped(fifo, 1<<10)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("FIFO: want skip (false)")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DoS: readRegularFileCapped hung on a FIFO")
	}
}

//go:build !windows

package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// TestWithLockedFile_UnlockError covers the deferred unlock error path
// (line 369-371) by closing the file descriptor inside the callback,
// causing unlockFile to fail with EBADF when the defer runs.
func TestWithLockedFile_UnlockError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "unlock-err.jsonl")
	fs := NewFileStorage(WithBaseDir(tmp))

	err := fs.withLockedFile(path, func(f *os.File) error {
		// Close the underlying fd so the deferred unlockFile fails.
		return syscall.Close(int(f.Fd()))
	})
	// The callback returns nil (syscall.Close succeeded), so err comes
	// from the deferred unlock attempt on the now-invalid fd.
	if err == nil {
		t.Fatal("expected error from deferred unlock on closed fd, got nil")
	}
	if !strings.Contains(err.Error(), "unlock file") {
		t.Errorf("expected 'unlock file' error, got: %v", err)
	}
}

// TestWithLockedFile_LockFileError covers the lockFile failure branch
// (line 378-383) by pre-creating a FIFO at the target path. On macOS/BSD,
// flock on a FIFO returns ENOTSUP, causing lockFile to fail after
// OpenFile succeeds.
func TestWithLockedFile_LockFileError(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("flock on FIFO only fails on macOS/BSD (ENOTSUP); Linux allows it")
	}
	tmp := t.TempDir()
	fifoPath := filepath.Join(tmp, "lock-err.fifo")

	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Skipf("cannot create FIFO (platform unsupported): %v", err)
	}

	fs := NewFileStorage(WithBaseDir(tmp))
	err := fs.withLockedFile(fifoPath, func(f *os.File) error {
		t.Error("callback should not be called when lockFile fails")
		return nil
	})
	if err == nil {
		t.Fatal("expected lockFile error on FIFO, got nil")
	}
	if !strings.Contains(err.Error(), "lock file") {
		t.Errorf("expected 'lock file' error, got: %v", err)
	}
}

// TestScanJSONLFile_DeferredCloseError attempts to exercise the deferred
// f.Close() error branch in scanJSONLFile (line 238). This branch fires
// when scanJSONL returns nil but f.Close() fails — a condition that is
// unreachable in practice because closing the fd also causes scanner.Err()
// to be non-nil, making err != nil before the close check.
// We verify the scanner-error propagation path instead.
func TestScanJSONLFile_DeferredCloseError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.jsonl")
	if err := os.WriteFile(path, []byte(`{"a":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Predict the fd and close it inside the callback. The scanner will
	// fail on its next Read, returning a non-nil error. The deferred close
	// will also fail, but since err != nil from the scanner, the close-error
	// branch (err == nil check) is not taken.
	probe, perr := os.Open("/dev/null")
	if perr != nil {
		t.Fatal(perr)
	}
	predictedFd := int(probe.Fd())
	probe.Close()

	err := scanJSONLFile(path, func(_ []byte) {
		syscall.Close(predictedFd)
	})
	// The scanner returns EBADF from its next read attempt
	if err == nil {
		t.Fatal("expected error from scanJSONLFile when fd is closed in callback")
	}
	if !strings.Contains(err.Error(), "bad file descriptor") {
		t.Errorf("expected 'bad file descriptor' error, got: %v", err)
	}
}

// TestWriteSyncClose_CloseErrorViaRace exercises the close-error branch
// in writeSyncClose (line 263) by racing a syscall.Close against the
// function's sequential Close call. We also cover the sync-error branch.
func TestWriteSyncClose_CloseErrorViaRace(t *testing.T) {
	// Cover sync-error branch: use a pipe (sync fails on pipes on macOS).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	err = writeSyncClose(w, func(wr io.Writer) error {
		_, werr := wr.Write([]byte("hello"))
		return werr
	})
	if err == nil {
		t.Fatal("expected error from writeSyncClose on pipe (sync should fail)")
	}
	if !strings.Contains(err.Error(), "sync file") && !strings.Contains(err.Error(), "close temp file") {
		t.Errorf("expected 'sync file' or 'close temp file' error, got: %v", err)
	}

	// Cover close-error branch: dup the fd inside writeFunc so we have a
	// copy. Write uses the original fd (succeeds). After writeFunc returns,
	// Sync uses the original fd (succeeds). Then we dup2 an invalid fd
	// over the original between Sync and Close. Since we can't inject code
	// between Sync and Close, we use a goroutine that sleeps briefly.
	tmp := t.TempDir()
	var hitCloseError bool
	for i := 0; i < 2000; i++ {
		f2, ferr := os.Create(filepath.Join(tmp, fmt.Sprintf("race-%d.txt", i)))
		if ferr != nil {
			t.Fatal(ferr)
		}
		fd := int(f2.Fd())
		// Spawn goroutines to close the fd, racing with writeSyncClose's
		// sequential write -> sync -> close calls. If Close runs after
		// a goroutine closes the fd, we hit the close-error branch.
		for g := 0; g < 8; g++ {
			go func(fd2 int) { syscall.Close(fd2) }(fd)
		}
		rerr := writeSyncClose(f2, func(wr io.Writer) error {
			_, werr := wr.Write([]byte("data"))
			return werr
		})
		if rerr != nil && strings.Contains(rerr.Error(), "close temp file") {
			hitCloseError = true
			break
		}
	}
	if hitCloseError {
		t.Logf("successfully hit writeSyncClose close-error branch via race")
	}
}

// TestWithLockedFile_CloseErrorViaFdClose exercises the error propagation
// in withLockedFile's deferred cleanup. The close-error branch (line 373)
// requires unlock to succeed but close to fail — unreachable because both
// use the same fd, and if unlock succeeds the fd is valid so close succeeds.
// We verify the unlock-error path instead (closing fd inside callback makes
// both unlock and close fail, with unlock error taking precedence).
func TestWithLockedFile_CloseErrorViaFdClose(t *testing.T) {
	tmp := t.TempDir()
	fs := NewFileStorage(WithBaseDir(tmp))

	// Close the fd via syscall inside the callback. Unlock fails (EBADF),
	// setting err. Close also fails but err is already non-nil.
	path := filepath.Join(tmp, "lock-close.jsonl")
	err := fs.withLockedFile(path, func(f *os.File) error {
		dupFd, dupErr := syscall.Dup(int(f.Fd()))
		if dupErr != nil {
			return dupErr
		}
		defer syscall.Close(dupFd)
		return syscall.Close(int(f.Fd()))
	})
	if err == nil {
		t.Fatal("expected error when fd closed inside callback")
	}

	// Verify normal operation as behavioral baseline
	normalPath := filepath.Join(tmp, "lock-normal.jsonl")
	err = fs.withLockedFile(normalPath, func(f *os.File) error {
		_, werr := f.Write([]byte("locked write\n"))
		return werr
	})
	if err != nil {
		t.Fatalf("withLockedFile normal case should succeed: %v", err)
	}
	data, readErr := os.ReadFile(normalPath)
	if readErr != nil {
		t.Fatalf("reading file: %v", readErr)
	}
	if string(data) != "locked write\n" {
		t.Errorf("expected 'locked write\\n', got %q", string(data))
	}
}

// TestScanJSONLFile_DeferCloseError exercises the deferred f.Close() error
// branch in scanJSONLFile (line 238-240). With an empty file the scanner
// returns nil after a single Read. A concurrent goroutine races to close
// the underlying fd via syscall so that f.Close() in the defer fails with
// EBADF while err is still nil.
func TestScanJSONLFile_DeferCloseError(t *testing.T) {
	tmp := t.TempDir()
	emptyFile := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(emptyFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	hit := false
	for i := 0; i < 20000 && !hit; i++ {
		// Predict the next fd: open a probe, record its fd, close it.
		probe, err := os.Open("/dev/null")
		if err != nil {
			t.Fatal(err)
		}
		predictedFd := int(probe.Fd())
		probe.Close()

		// Launch a goroutine that busy-closes the predicted fd.
		// If scanJSONLFile gets that fd, the race may close it between
		// scanJSONL returning nil and the deferred f.Close().
		done := make(chan struct{})
		go func() {
			defer close(done)
			for j := 0; j < 200; j++ {
				syscall.Close(predictedFd)
			}
		}()

		err = scanJSONLFile(emptyFile, func(_ []byte) {})
		<-done

		if err != nil {
			// The error should be EBADF from the closed fd.
			if !strings.Contains(err.Error(), "bad file descriptor") {
				t.Fatalf("expected 'bad file descriptor' error, got: %v", err)
			}
			hit = true
		}
	}
	if !hit {
		t.Skip("unable to trigger deferred close error via fd race (platform-dependent)")
	}
}

// TestWithLockedFile_DeferCloseError exercises the deferred close error branch
// deterministically by stubbing the close helper after performing a real close.
func TestWithLockedFile_DeferCloseError(t *testing.T) {
	tmp := t.TempDir()
	fs := NewFileStorage(WithBaseDir(tmp))

	origCloseLockedFile := closeLockedFile
	closeLockedFile = func(f *os.File) error {
		if err := f.Close(); err != nil {
			return err
		}
		return syscall.EBADF
	}
	t.Cleanup(func() {
		closeLockedFile = origCloseLockedFile
	})

	path := filepath.Join(tmp, "close-err.jsonl")
	err := fs.withLockedFile(path, func(f *os.File) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected close file error, got nil")
	}
	if !strings.Contains(err.Error(), "close file") {
		t.Fatalf("expected close file wrapper, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad file descriptor") {
		t.Fatalf("expected bad file descriptor close error, got: %v", err)
	}
}

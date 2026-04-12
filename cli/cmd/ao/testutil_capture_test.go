package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCaptureStdoutRestoresStdoutAfterReturn(t *testing.T) {
	original := os.Stdout

	out, err := captureStdout(t, func() error {
		_, writeErr := fmt.Fprint(os.Stdout, "hello")
		return writeErr
	})
	if err != nil {
		t.Fatalf("captureStdout returned error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("expected captured output %q, got %q", "hello", out)
	}
	if os.Stdout != original {
		t.Fatal("stdout was not restored after captureStdout returned")
	}
}

func TestCaptureJSONStdoutRepeatedUseDoesNotLeakOutput(t *testing.T) {
	first := captureJSONStdout(t, func() {
		_, _ = fmt.Fprint(os.Stdout, `{"run":1}`)
	})
	second := captureJSONStdout(t, func() {
		_, _ = fmt.Fprint(os.Stdout, `{"run":2}`)
	})

	if first != `{"run":1}` {
		t.Fatalf("unexpected first capture: %q", first)
	}
	if second != `{"run":2}` {
		t.Fatalf("unexpected second capture: %q", second)
	}
}

func TestCaptureStdoutHandlesLargeOutputWithoutDeadlock(t *testing.T) {
	payload := strings.Repeat("stdout-payload-", 8192)

	out, err := captureStdout(t, func() error {
		_, writeErr := fmt.Fprint(os.Stdout, payload)
		return writeErr
	})
	if err != nil {
		t.Fatalf("captureStdout returned error: %v", err)
	}
	if out != payload {
		t.Fatalf("expected %d bytes, got %d", len(payload), len(out))
	}
}

func TestBeginStdoutCaptureSessionSerializesConcurrentUse(t *testing.T) {
	session, err := beginStdoutCaptureSession()
	if err != nil {
		t.Fatalf("beginStdoutCaptureSession returned error: %v", err)
	}

	secondReady := make(chan struct{})
	secondSession := make(chan *stdoutCaptureSession, 1)
	secondErr := make(chan error, 1)

	go func() {
		close(secondReady)
		session, err := beginStdoutCaptureSession()
		secondSession <- session
		secondErr <- err
	}()

	<-secondReady
	select {
	case session := <-secondSession:
		if session != nil {
			session.closeAndRestore()
		}
		t.Fatal("second stdout capture started before the first restored stdout")
	case <-time.After(50 * time.Millisecond):
		// Expected: the second session is waiting on stdoutCaptureMu.
	}

	session.closeAndRestore()

	session = <-secondSession
	if err := <-secondErr; err != nil {
		t.Fatalf("second stdout capture returned error: %v", err)
	}
	if session == nil {
		t.Fatal("second stdout capture returned nil session")
	}
	session.closeAndRestore()
}

func TestCaptureStdoutRestoresStdoutAfterPanic(t *testing.T) {
	original := os.Stdout

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic from captureStdout callback")
		}
		if os.Stdout != original {
			t.Fatal("stdout was not restored after panic")
		}
	}()

	_, _ = captureStdout(t, func() error {
		_, _ = fmt.Fprint(os.Stdout, "boom")
		panic("boom")
	})
}

func TestCaptureJSONStdoutRestoresStdoutAfterPanic(t *testing.T) {
	original := os.Stdout

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic from captureJSONStdout callback")
		}
		if os.Stdout != original {
			t.Fatal("stdout was not restored after JSON capture panic")
		}
	}()

	_ = captureJSONStdout(t, func() {
		_, _ = fmt.Fprint(os.Stdout, `{"boom":true}`)
		panic("boom")
	})
}

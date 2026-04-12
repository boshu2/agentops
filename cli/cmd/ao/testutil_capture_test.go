package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
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

func TestBeginStdoutCaptureSessionRejectsNestedUse(t *testing.T) {
	session, err := beginStdoutCaptureSession()
	if err != nil {
		t.Fatalf("beginStdoutCaptureSession returned error: %v", err)
	}
	defer session.closeAndRestore()

	_, err = beginStdoutCaptureSession()
	if err == nil {
		t.Fatal("expected nested stdout capture to fail")
	}
	if !strings.Contains(err.Error(), "nested stdout capture") {
		t.Fatalf("expected nested capture error, got: %v", err)
	}
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

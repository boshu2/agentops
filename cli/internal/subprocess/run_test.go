package subprocess

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

const helperEnv = "GO_WANT_AGENTOPS_SUBPROCESS_HELPER"

func TestCaptureRetainsBoundedPrefixAndSuffix(t *testing.T) {
	capture := newCapture(CaptureLimit{HeadBytes: 4, TailBytes: 5})
	if _, err := capture.Write([]byte("abcdefghijklmnop")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	output := capture.snapshot()
	if got, want := string(output.Prefix), "abcd"; got != want {
		t.Fatalf("prefix = %q, want %q", got, want)
	}
	if got, want := string(output.Suffix), "lmnop"; got != want {
		t.Fatalf("suffix = %q, want %q", got, want)
	}
	if output.TotalBytes != 16 || !output.Truncated {
		t.Fatalf("output = %#v, want 16 total bytes and truncation", output)
	}
	if output.RetainedBytes() != 9 {
		t.Fatalf("retained bytes = %d, want 9", output.RetainedBytes())
	}
	if rendered := output.String(); !strings.Contains(rendered, "7 bytes omitted") {
		t.Fatalf("rendered output %q lacks explicit truncation telemetry", rendered)
	}
}

func TestRunBoundsHighOutputWhileStreaming(t *testing.T) {
	command := helperCommand(t, "high-output")
	command.CombinedOutput = true
	command.OutputLimit = CaptureLimit{HeadBytes: 128, TailBytes: 128}

	result, err := Run(context.Background(), command)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	const total = 16 * 1024 * 1024
	if result.Combined.TotalBytes != total {
		t.Fatalf("total bytes = %d, want %d", result.Combined.TotalBytes, total)
	}
	if !result.Combined.Truncated {
		t.Fatal("high output was not reported as truncated")
	}
	if result.Combined.RetainedBytes() > 256 {
		t.Fatalf("retained %d bytes, hard bound is 256", result.Combined.RetainedBytes())
	}
	if !strings.HasPrefix(string(result.Combined.Prefix), "HEAD") {
		t.Fatalf("prefix lost: %q", result.Combined.Prefix)
	}
	if !strings.HasSuffix(string(result.Combined.Suffix), "TAIL") {
		t.Fatalf("suffix lost: %q", result.Combined.Suffix)
	}
}

func TestRunPreservesAbnormalExit(t *testing.T) {
	command := helperCommand(t, "exit-seven")
	command.CombinedOutput = true
	command.OutputLimit = CaptureLimit{HeadBytes: 64, TailBytes: 64}

	result, err := Run(context.Background(), command)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run error = %v, want *exec.ExitError", err)
	}
	if result.ExitCode != 7 || exitErr.ExitCode() != 7 {
		t.Fatalf("exit codes = result:%d error:%d, want 7", result.ExitCode, exitErr.ExitCode())
	}
	if got := result.Combined.String(); !strings.Contains(got, "abnormal output") {
		t.Fatalf("combined output = %q, want abnormal diagnostic", got)
	}
}

func TestRunReturnsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command := helperCommand(t, "block")

	_, err := Run(ctx, command)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
}

func TestRunDoesNotLeakGoroutinesAfterCancellation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	for range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		_, err := Run(ctx, helperCommand(t, "block"))
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run error = %v, want context deadline", err)
		}
	}
}

func helperCommand(t *testing.T, mode string) Command {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return Command{
		Name: executable,
		Args: []string{"-test.run=^TestSubprocessHelperProcess$", "--", mode},
		Env:  append(os.Environ(), helperEnv+"=1"),
	}
}

func TestSubprocessHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	switch mode {
	case "high-output":
		chunk := []byte(strings.Repeat("x", 64*1024))
		copy(chunk, "HEAD")
		for i := range 256 {
			if i == 255 {
				copy(chunk[len(chunk)-4:], "TAIL")
			}
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(90)
			}
		}
	case "exit-seven":
		_, _ = fmt.Fprintln(os.Stdout, "abnormal output")
		os.Exit(7)
	case "block":
		for {
			time.Sleep(time.Second)
		}
	default:
		os.Exit(92)
	}
	os.Exit(0)
}

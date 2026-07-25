package subprocess

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
	assertCleanupCompleted(t, result.Cleanup)
}

func TestRunStartFailureRecordsCleanupNotStarted(t *testing.T) {
	result, err := Run(context.Background(), Command{Name: filepath.Join(t.TempDir(), "missing-command")})
	if err == nil {
		t.Fatal("Run error = nil, want start failure")
	}
	if result.Cleanup.Status != CleanupNotStarted || result.Cleanup.Attempted || result.Cleanup.Completed {
		t.Fatalf("cleanup = %#v, want explicit not-started outcome", result.Cleanup)
	}
}

func TestRunOrdinarySuccessRecordsCleanupCompleted(t *testing.T) {
	result, err := Run(context.Background(), helperCommand(t, "success"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertCleanupCompleted(t, result.Cleanup)
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
	assertCleanupCompleted(t, result.Cleanup)
}

func TestRunReturnsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	command := helperCommand(t, "block")
	command.OnStart = func(int) { cancel() }

	result, err := Run(ctx, command)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	assertCleanupCompleted(t, result.Cleanup)
}

func TestRunReturnsDeadlineAndCleanupFacts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	result, err := Run(ctx, helperCommand(t, "block"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context.DeadlineExceeded", err)
	}
	assertCleanupCompleted(t, result.Cleanup)
}

func TestRunReturnsInjectedCleanupFailure(t *testing.T) {
	cleanupErr := errors.New("injected cleanup failure")
	result, err := runWithCleanup(context.Background(), helperCommand(t, "success"), func(*exec.Cmd) error {
		return cleanupErr
	})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("Run error = %v, want injected cleanup error", err)
	}
	if result.Cleanup.Status != CleanupFailed || !result.Cleanup.Attempted || result.Cleanup.Completed {
		t.Fatalf("cleanup = %#v, want failed outcome", result.Cleanup)
	}
	if result.Cleanup.Error != cleanupErr.Error() {
		t.Fatalf("cleanup diagnostic = %q, want %q", result.Cleanup.Error, cleanupErr)
	}
}

func TestRunJoinsPrimaryAndCleanupFailures(t *testing.T) {
	cleanupErr := errors.New("injected cleanup failure")
	result, err := runWithCleanup(context.Background(), helperCommand(t, "exit-seven"), func(*exec.Cmd) error {
		return cleanupErr
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("Run error = %v, want exit code 7 identity", err)
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("Run error = %v, want cleanup error identity", err)
	}
	if result.Cleanup.Status != CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed outcome", result.Cleanup)
	}
}

func TestRunJoinsCancellationAndCleanupFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	command := helperCommand(t, "block")
	command.OnStart = func(int) { cancel() }
	cleanupErr := errors.New("injected cleanup failure")

	result, err := runWithCleanup(ctx, command, func(*exec.Cmd) error {
		return cleanupErr
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Run error = %v, want cancellation and cleanup identities", err)
	}
	if result.Cleanup.Status != CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed outcome", result.Cleanup)
	}
}

func TestRunJoinsDeadlineAndCleanupFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	cleanupErr := errors.New("injected cleanup failure")

	result, err := runWithCleanup(ctx, helperCommand(t, "block"), func(*exec.Cmd) error {
		return cleanupErr
	})
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Run error = %v, want deadline and cleanup identities", err)
	}
	if result.Cleanup.Status != CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed outcome", result.Cleanup)
	}
}

func TestAttachFailureSkipsWaitAfterIneffectiveKillAndJoinsErrors(t *testing.T) {
	attachErr := errors.New("attach sentinel")
	killErr := errors.New("kill sentinel")
	releaseErr := errors.New("release sentinel")
	waitCalled := false

	waitErr, cleanupErr, waited := handleAttachFailure(
		attachErr,
		func() error { return killErr },
		func() error {
			waitCalled = true
			return errors.New("wait must not run")
		},
		func() error { return releaseErr },
	)
	if waited || waitCalled {
		t.Fatal("attach failure entered Wait after Process.Kill was ineffective")
	}
	if waitErr != nil {
		t.Fatalf("wait error = %v, want nil because Wait was skipped", waitErr)
	}
	for _, want := range []error{attachErr, killErr, releaseErr} {
		if !errors.Is(cleanupErr, want) {
			t.Fatalf("cleanup error = %v, want identity %v", cleanupErr, want)
		}
	}
}

func TestAttachFailureWaitsAfterEffectiveKillAndPreservesWaitError(t *testing.T) {
	attachErr := errors.New("attach sentinel")
	waitSentinel := errors.New("wait sentinel")
	waitErr, cleanupErr, waited := handleAttachFailure(
		attachErr,
		func() error { return nil },
		func() error { return waitSentinel },
		func() error {
			t.Fatal("release must not run after effective kill")
			return nil
		},
	)
	if !waited || !errors.Is(waitErr, waitSentinel) {
		t.Fatalf("waited/error = %v/%v, want true and wait sentinel", waited, waitErr)
	}
	if !errors.Is(cleanupErr, attachErr) {
		t.Fatalf("cleanup error = %v, want attach identity", cleanupErr)
	}
}

func TestCleanupDiagnosticIsBoundedAndValidUTF8(t *testing.T) {
	message := strings.Repeat("界", maxCleanupDiagnosticLen)
	outcome := cleanupOutcome(errors.New(message))
	if len(outcome.Error) > maxCleanupDiagnosticLen {
		t.Fatalf("cleanup diagnostic length = %d, want <= %d", len(outcome.Error), maxCleanupDiagnosticLen)
	}
	if !utf8.ValidString(outcome.Error) {
		t.Fatalf("cleanup diagnostic is not valid UTF-8: %q", outcome.Error)
	}
	if !strings.HasSuffix(outcome.Error, cleanupTruncationMarker) {
		t.Fatalf("cleanup diagnostic = %q, want truncation marker", outcome.Error)
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
	case "success":
		_, _ = fmt.Fprintln(os.Stdout, "success")
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

func assertCleanupCompleted(t *testing.T, outcome CleanupOutcome) {
	t.Helper()
	if outcome.Status != CleanupCompleted || !outcome.Attempted || !outcome.Completed || outcome.Error != "" {
		t.Fatalf("cleanup = %#v, want completed outcome", outcome)
	}
}

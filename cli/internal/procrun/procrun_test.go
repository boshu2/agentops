package procrun

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// shellCmd builds a ctx-bound command running script through the platform shell.
func shellCmd(ctx context.Context, script string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", script)
	}
	return exec.CommandContext(ctx, "sh", "-c", script)
}

// TestCapture_ByteIdenticalWhenUnderCap is witness (4): a fast successful run
// under the cap yields output byte-identical to the child's raw bytes, with no
// truncation.
func TestRun_FastSuccessIsByteIdentical(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	ctx := context.Background()
	cmd := shellCmd(ctx, "printf 'hello world'")
	res, err := Run(ctx, cmd, Options{Combined: true, MaxCaptureBytes: 4096})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(res.Combined); got != "hello world" {
		t.Fatalf("Combined = %q, want %q", got, "hello world")
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Truncated {
		t.Fatalf("Truncated = true, want false")
	}
	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
}

// TestRun_SeparateStreams verifies stdout and stderr are captured distinctly and
// that a non-zero exit is reported through ExitCode, not the top-level error.
func TestRun_SeparateStreamsAndExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	ctx := context.Background()
	cmd := shellCmd(ctx, "printf out; printf err 1>&2; exit 3")
	res, err := Run(ctx, cmd, Options{MaxCaptureBytes: 4096})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(res.Stdout) != "out" {
		t.Fatalf("Stdout = %q, want %q", res.Stdout, "out")
	}
	if string(res.Stderr) != "err" {
		t.Fatalf("Stderr = %q, want %q", res.Stderr, "err")
	}
	if res.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", res.ExitCode)
	}
}

// TestRun_BoundedCaptureUnderRunawayOutput is witness (1): a child that emits
// far more than the cap does not grow retained memory unboundedly and the result
// reflects the documented head+marker+tail truncation semantics.
func TestRun_BoundedCaptureUnderRunawayOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	ctx := context.Background()
	// Emit ~2 MiB of 'x' framed by a distinct head and tail so we can prove the
	// true head and true tail survive.
	script := "printf HEADSTART; " +
		"for i in $(seq 1 2048); do printf '%1024d' 0 | tr ' ' x; done; " +
		"printf TAILEND"
	cmd := shellCmd(ctx, script)
	const cap = 64
	res, err := Run(ctx, cmd, Options{Combined: true, MaxCaptureBytes: cap})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("Truncated = false, want true for runaway output")
	}
	// Retained bytes are bounded to head+tail+marker regardless of the ~2 MiB
	// emitted.
	maxRetained := cap + len(truncationMarker)
	if len(res.Combined) > maxRetained {
		t.Fatalf("retained %d bytes, want <= %d (memory not bounded)", len(res.Combined), maxRetained)
	}
	if !bytes.HasPrefix(res.Combined, []byte("HEADSTART")) {
		t.Fatalf("retained head lost: %q", head64(res.Combined))
	}
	if !bytes.HasSuffix(res.Combined, []byte("TAILEND")) {
		t.Fatalf("retained tail lost: %q", tail64(res.Combined))
	}
	if !bytes.Contains(res.Combined, []byte(truncationMarker)) {
		t.Fatalf("truncation marker missing from %q", res.Combined)
	}
}

// TestCapture_HeadTailAssembly exercises the capture ring directly across the
// boundary cases: everything-in-head, head+tail-no-loss, and wrapped-with-loss.
func TestCapture_HeadTailAssembly(t *testing.T) {
	tests := []struct {
		name       string
		head, tail int
		writes     []string
		want       string
		truncated  bool
	}{
		{name: "all in head", head: 4, tail: 4, writes: []string{"ab"}, want: "ab", truncated: false},
		{name: "exact head only", head: 4, tail: 4, writes: []string{"abcd"}, want: "abcd", truncated: false},
		{name: "head plus tail no loss", head: 4, tail: 4, writes: []string{"abcdWXYZ"}, want: "abcdWXYZ", truncated: false},
		{name: "partial tail no loss", head: 4, tail: 4, writes: []string{"abcdWX"}, want: "abcdWX", truncated: false},
		{name: "wrapped with loss", head: 4, tail: 4, writes: []string{"abcd", "1234567", "WXYZ"}, want: "abcd" + truncationMarker + "WXYZ", truncated: true},
		{name: "single oversize write", head: 4, tail: 4, writes: []string{"abcdefghijkWXYZ"}, want: "abcd" + truncationMarker + "WXYZ", truncated: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCapture(tc.head, tc.tail)
			for _, w := range tc.writes {
				n, err := c.Write([]byte(w))
				if err != nil || n != len(w) {
					t.Fatalf("Write(%q) = %d, %v", w, n, err)
				}
			}
			if got := string(c.Bytes()); got != tc.want {
				t.Fatalf("Bytes() = %q, want %q", got, tc.want)
			}
			if c.Truncated() != tc.truncated {
				t.Fatalf("Truncated() = %v, want %v", c.Truncated(), tc.truncated)
			}
		})
	}
}

func head64(b []byte) string {
	if len(b) > 64 {
		return string(b[:64])
	}
	return string(b)
}

func tail64(b []byte) string {
	if len(b) > 64 {
		return string(b[len(b)-64:])
	}
	return string(b)
}

// TestRun_RejectsPreCancelledContext confirms the fail-fast guard.
func TestRun_RejectsPreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := shellCmd(ctx, "true")
	if _, err := Run(ctx, cmd, Options{Combined: true}); err == nil {
		t.Fatalf("Run with cancelled ctx = nil error, want ctx error")
	} else if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Run error = %v, want context canceled", err)
	}
}

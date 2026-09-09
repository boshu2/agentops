//go:build darwin || linux

package provenanceapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMineSession_CheckpointPartialWrite(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	sess := writeMineSession(t, dir, "session.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n")
	if _, err := mine(t, MineOptions{File: sess, State: state}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	writeMineSession(t, dir, "session.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n{\"type\":\"tool_use\",\"tool_name\":\"Bash\",\"tool_input\":{}}\n")
	var parentLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &parentLimit); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMineSession_CheckpointPartialWriteChild$")
	cmd.Env = append(os.Environ(), "AO_MINE_PARTIAL_WRITE_DIR="+dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	emitted, err := cmd.Output()
	if err != nil {
		t.Fatalf("partial-write child: %v; stderr: %s", err, &stderr)
	}
	if got := stderr.String(); got != "checkpoint write returned EFBIG\n" {
		t.Fatalf("child did not observe a real write error: %q", got)
	}
	var afterLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &afterLimit); err != nil {
		t.Fatal(err)
	}
	if afterLimit != parentLimit {
		t.Errorf("parent file-size limit changed: %+v -> %+v", parentLimit, afterLimit)
	}
	after, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("partial write changed valid checkpoint: before %d bytes, after %d bytes", len(before), len(after))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "session.jsonl" || entries[1].Name() != "state.json" {
		t.Errorf("ordinary-error cleanup left unexpected directory entries: %v", entries)
	}
	failedEvents := parseMineEvents(t, string(emitted))
	if len(failedEvents) != 1 || failedEvents[0].Tool != "Bash" {
		t.Fatalf("failed attempt events = %+v, want one new Bash", failedEvents)
	}
	retry, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if retry != string(emitted) {
		t.Errorf("retry must repeat only uncheckpointed events with identical IDs:\nfailed: %s\nretry: %s", emitted, retry)
	}
	again, err := mine(t, MineOptions{File: sess, State: state})
	if err != nil || again != "" {
		t.Errorf("successful retry did not checkpoint: output %q, error %v", again, err)
	}
}

// The limit belongs only to this bounded child. Ignoring SIGXFSZ makes the
// kernel return a real partial-write error instead of terminating the process.
func TestMineSession_CheckpointPartialWriteChild(t *testing.T) {
	dir := os.Getenv("AO_MINE_PARTIAL_WRITE_DIR")
	if dir == "" {
		return
	}
	signal.Ignore(syscall.SIGXFSZ)
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	originalLimit := limit
	limit.Cur = 32
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	err := MineSession(MineOptions{File: filepath.Join(dir, "session.jsonl"), State: filepath.Join(dir, "state.json"), JSON: true}, os.Stdout)
	// Coverage data is flushed at exit; only the checkpoint write is limited.
	if restoreErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &originalLimit); restoreErr != nil {
		t.Fatalf("restore file-size limit: %v", restoreErr)
	}
	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("expected file-size write error, got %v", err)
	}
	fmt.Fprintln(os.Stderr, "checkpoint write returned EFBIG")
	os.Exit(0)
}

func TestWriteMineState_Permissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o640, 0o644} {
		t.Run(fmt.Sprintf("existing_%o", mode), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte("old"), mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			if err := writeMineState(path, mineState{LastLine: 2}); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != mode {
				t.Fatalf("checkpoint mode = %o, want unchanged %o", got, mode)
			}
		})
	}
	t.Run("new_checkpoint_is_private", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := writeMineState(path, mineState{}); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("new checkpoint mode = %o, want 600", got)
		}
	})
}

func TestMineSession_CheckpointSymlinkRejected(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		t.Run(fmt.Sprintf("dangling_%t", dangling), func(t *testing.T) {
			dir := t.TempDir()
			sess := writeMineSession(t, dir, "s.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n")
			target := filepath.Join(dir, "target.json")
			if !dangling {
				if _, err := mine(t, MineOptions{File: sess, State: target}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(target)
			state := filepath.Join(dir, "state.json")
			if err := os.Symlink(target, state); err != nil {
				t.Fatal(err)
			}
			out, err := mine(t, MineOptions{File: sess, State: state})
			if err == nil || !strings.Contains(err.Error(), "checkpoint must be a regular file") || out != "" {
				t.Fatalf("symlink checkpoint must be rejected: output %q, error %v", out, err)
			}
			if got, err := os.Readlink(state); err != nil || got != target {
				t.Fatalf("checkpoint symlink changed: %q, %v", got, err)
			}
			after, err := os.ReadFile(target)
			if dangling {
				if !os.IsNotExist(err) {
					t.Fatalf("dangling target created: %v", err)
				}
			} else if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("symlink target changed: before %q, after %q, error %v", before, after, err)
			}
		})
	}
}

func TestMineSession_CheckpointFIFORejected(t *testing.T) {
	if dir := os.Getenv("AO_MINE_FIFO_DIR"); dir != "" {
		out, err := mine(t, MineOptions{File: filepath.Join(dir, "s.jsonl"), State: filepath.Join(dir, "state.json")})
		if err == nil || !strings.Contains(err.Error(), "checkpoint must be a regular file") || out != "" {
			t.Fatalf("FIFO checkpoint must be rejected before reading: output %q, error %v", out, err)
		}
		return
	}
	dir := t.TempDir()
	writeMineSession(t, dir, "s.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n")
	state := filepath.Join(dir, "state.json")
	if err := syscall.Mkfifo(state, 0o600); err != nil {
		t.Fatal(err)
	}
	// Directly test the writer too: atomic rename must not replace a FIFO.
	if err := writeMineState(state, mineState{}); err == nil || !strings.Contains(err.Error(), "checkpoint must be a regular file") {
		t.Fatalf("FIFO write must be rejected: %v", err)
	}
	// A bounded child catches regressions that would block while opening a FIFO.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMineSession_CheckpointFIFORejected$")
	cmd.Env = append(os.Environ(), "AO_MINE_FIFO_DIR="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("FIFO reader child: %v; output: %s", err, output)
	}
	info, err := os.Lstat(state)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("FIFO checkpoint replaced: %v, %v", info, err)
	}
}

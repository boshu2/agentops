package close

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
)

func TestTrackerUsesSharedResolvedCommandForCloseStatusAndSync(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "tracker.log")
	script := filepath.Join(root, "br")
	scriptBody := `#!/bin/sh
printf 'cwd=%s marker=%s beads=%s args=%s\n' "$PWD" "$MARKER" "${BEADS_DIR-unset}" "$*" >> "$TRACKER_LOG"
case "${1:-}" in
  show) printf '{"id":"age-close","status":"closed"}\n' ;;
  close) printf 'close-stdout\n'; printf 'close-stderr\n' >&2; exit 23 ;;
  sync) printf 'sync-stderr\n' >&2; exit 24 ;;
  *) exit 25 ;;
esac
`
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatal(err)
	}
	resolution := closeapp.Resolution{
		Backend: "br", Binary: script, LedgerDir: filepath.Join(root, "ledger"),
		WorkDir: workDir, ChildEnv: []string{
			"MARKER=canonical", "BEADS_DIR=" + filepath.Join(root, "ledger"), "TRACKER_LOG=" + logPath,
		},
	}
	tracker := NewTracker()
	closed, statusErr := tracker.Status(context.Background(), resolution, "age-close")
	if statusErr != nil || !closed {
		t.Fatalf("Status() = closed %v, error %v", closed, statusErr)
	}
	closeErr := tracker.Close(context.Background(), resolution, "age-close", "evidence: proof.md")
	syncErr := tracker.Sync(context.Background(), resolution)

	var closeExit *trackerexec.ExitError
	if !errors.As(closeErr, &closeExit) || closeExit.ExitCode() != 23 {
		t.Errorf("Close() error = %T %v, want shared *trackerexec.ExitError(23)", closeErr, closeErr)
	}
	if !strings.Contains(closeErr.Error(), "close-stdout") || !strings.Contains(closeErr.Error(), "close-stderr") {
		t.Errorf("Close() error = %q, want combined child output", closeErr)
	}
	var syncExit *trackerexec.ExitError
	if !errors.As(syncErr, &syncExit) || syncExit.ExitCode() != 24 {
		t.Errorf("Sync() error = %T %v, want shared *trackerexec.ExitError(24)", syncErr, syncErr)
	}
	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logBody)
	for _, want := range []string{
		"cwd=" + workDir + " marker=canonical beads=" + resolution.LedgerDir + " args=show age-close --json",
		"cwd=" + workDir + " marker=canonical beads=" + resolution.LedgerDir + " args=close age-close --reason evidence: proof.md",
		"cwd=" + workDir + " marker=canonical beads=" + resolution.LedgerDir + " args=sync --flush-only",
		"cwd=" + workDir + " marker=canonical beads=" + resolution.LedgerDir + " args=sync",
	} {
		if !strings.Contains(logText, want) {
			t.Errorf("tracker log missing %q:\n%s", want, logText)
		}
	}

	marker := filepath.Join(root, "canceled-command-ran")
	cancelScript := filepath.Join(root, "cancel-br")
	if err := os.WriteFile(cancelScript, []byte("#!/bin/sh\nprintf ran > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cancelResolution := resolution
	cancelResolution.Binary = cancelScript
	cancelErr := tracker.Close(canceled, cancelResolution, marker, "unused")
	if !errors.Is(cancelErr, context.Canceled) {
		t.Errorf("canceled Close() error = %T %v, want context.Canceled", cancelErr, cancelErr)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("canceled Close() launched tracker child: stat error %v", err)
	}
}

func TestParseRecordsAcceptsSingleShowObject(t *testing.T) {
	records := parseRecords([]byte(`{"id":"agentops-1","status":"closed"}`))
	if len(records) != 1 || records[0].ID != "agentops-1" || records[0].Status != "closed" {
		t.Fatalf("parseRecords(single object) = %+v", records)
	}
}

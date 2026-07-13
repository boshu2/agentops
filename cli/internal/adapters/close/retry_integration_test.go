package close

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

func TestBRSyncFailureRetryUsesTrackerStateAndClosesExactlyOnce(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "close-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "close@test")
	t.Setenv("GIT_COMMITTER_NAME", "close-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "close@test")
	dir := t.TempDir()
	ledger := filepath.Join(dir, "_beads")
	fakebin := filepath.Join(dir, "fakebin")
	for _, path := range []string{ledger, fakebin, filepath.Join(dir, "docs")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	runGit := func(workDir string, args ...string) {
		t.Helper()
		command := exec.Command(realGit, args...)
		command.Dir = workDir
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, workDir, err, out)
		}
	}
	runGit(dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "proof.md"), []byte("proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(dir, "add", "proof.md")
	runGit(dir, "commit", "-q", "-m", "seed public")
	if err := os.WriteFile(filepath.Join(dir, "docs", "result.md"), []byte("result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(ledger, "init", "-q")
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(`{"id":"age-retry","status":"open"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(ledger, "add", "issues.jsonl")
	runGit(ledger, "commit", "-q", "-m", "seed ledger")

	stateFile := filepath.Join(dir, "tracker-state")
	syncCount := filepath.Join(dir, "sync-count")
	closeLog := filepath.Join(dir, "close.log")
	if err := os.WriteFile(stateFile, []byte("open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(syncCount, []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBR := `#!/usr/bin/env bash
case "${1:-}" in
  show)
    status="$(tr -d '\n' < "${CLOSE_TEST_STATE:?}")"
    printf '{"id":"%s","status":"%s"}\n' "${2:-}" "$status"
    ;;
  close)
    printf '%s\n' "$*" >> "${CLOSE_TEST_CLOSE_LOG:?}"
    printf 'closed\n' > "${CLOSE_TEST_STATE:?}"
    ;;
  sync)
    count="$(tr -d '\n' < "${CLOSE_TEST_SYNC_COUNT:?}")"
    count=$((count + 1))
    printf '%s\n' "$count" > "${CLOSE_TEST_SYNC_COUNT:?}"
    if [ "$count" -le 2 ]; then exit 77; fi
    sed 's/"status":"open"/"status":"closed"/' "${BEADS_DIR:?}/issues.jsonl" > "${BEADS_DIR:?}/issues.tmp"
    mv "${BEADS_DIR:?}/issues.tmp" "${BEADS_DIR:?}/issues.jsonl"
    ;;
  list) exit 78 ;;
  *) echo "unexpected br call: $*" >&2; exit 79 ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"PATH="+fakebin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AGENTOPS_TRACKER=br", "BEADS_DIR="+ledger,
		"CLOSE_TEST_STATE="+stateFile, "CLOSE_TEST_SYNC_COUNT="+syncCount, "CLOSE_TEST_CLOSE_LOG="+closeLog,
	)
	service := closeapp.NewService(StaticRuntime{WorkDir: dir, Env: env}, Tracker{}, Repository{})
	request := closeapp.Request{
		ID: "age-retry", Message: "finish close", Evidence: "proof.md", Paths: []string{"docs/result.md"}, Mode: closeapp.ModeEnsure,
	}
	_, err = service.Execute(context.Background(), request)
	var failure *closeapp.Failure
	if !errors.As(err, &failure) || failure.Code != closeapp.ExitPersistence {
		t.Fatalf("first error = %#v, want persistence failure", err)
	}
	if result, err := service.Execute(context.Background(), request); err != nil {
		t.Fatalf("retry: %v", err)
	} else if !result.AlreadyClosed {
		t.Fatalf("retry result = %+v, want already closed", result)
	}
	logBody, err := os.ReadFile(closeLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimSpace(string(logBody)), "close age-retry"); got != 1 {
		t.Fatalf("close calls = %d, want 1; log=%q", got, logBody)
	}
}

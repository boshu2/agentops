package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTickCommandSurfaceCovered(t *testing.T) {
	// These literals are intentionally full leaf-command names: the command
	// surface parity gate scans *_test.go for this coverage. The `tick`
	// subcommands are archived behind //go:build legacy — their coverage lives
	// in tick_cmd_legacy_test.go (age-h4y3).
	_ = []string{
		"ao chaos-test",
		"ao close",
		"ao council-gate",
		"ao guard-status",
		"ao install-guards",
		"ao ready",
		"ao verdict-gate",
	}
	covered := []string{
		"chaos-test",
		"close",
		"council-gate",
		"guard-status",
		"install-guards",
		"ready",
		"verdict-gate",
	}
	registered := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		registered[cmd.Name()] = true
	}
	for _, name := range covered {
		if !registered[name] {
			t.Fatalf("expected registered command %q", name)
		}
	}
}

func TestTickReadyStateCounts(t *testing.T) {
	ready := []tickBead{{ID: "cp-ready", Status: "open"}}
	all := []tickBead{
		{ID: "cp-ready", Status: "open"},
		{ID: "cp-open", Status: "open"},
		{ID: "cp-work", Status: "in_progress"},
		{ID: "cp-done", Status: "closed"},
	}
	counts := tickCountBeads(ready, all)
	if counts.Ready != 1 || counts.Open != 2 || counts.InProgress != 1 || counts.Closed != 1 {
		t.Fatalf("tickCountBeads() = %+v, want ready=1 open=2 in_progress=1 closed=1", counts)
	}
}

func TestTickReadyReportsResolvedBDStateSource(t *testing.T) {
	t.Setenv("AGENTOPS_TRACKER", "bd")
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBD := `#!/usr/bin/env bash
case "${1:-}" in
  ready) printf '%s\n' '[{"id":"agentops-next","status":"open"}]' ;;
  list)  printf '%s\n' '[{"id":"agentops-next","status":"open"},{"id":"agentops-done","status":"closed"}]' ;;
  *) echo "unexpected bd call: $*" >&2; exit 43 ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakebin, "bd"), []byte(fakeBD), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &stderr}
	if err := tickReady(rt); err != nil {
		t.Fatalf("tickReady() error: %v (stderr=%q)", err, stderr.String())
	}
	var state tickReadyState
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		t.Fatalf("decode ready state: %v\n%s", err, stdout.String())
	}
	if state.StateSource != trackerBD {
		t.Fatalf("state_source = %q, want %q", state.StateSource, trackerBD)
	}
	if state.Next != "agentops-next" {
		t.Fatalf("next = %q, want agentops-next", state.Next)
	}
}

// tickTestLedgerLine reproduces the real persisted br issues.jsonl line shape
// (full field set as flushed by `br sync --flush-only`), not a minimal
// hand-built marker — see standards test-pyramid "Fixture Fidelity".
func tickTestLedgerLine(id, status string) string {
	return `{"_type": "issue", "id": "` + id + `", "title": "Node 2", "status": "` + status + `", "priority": 2, "issue_type": "task", "created_at": "2026-05-08T12:09:00Z", "updated_at": "2026-05-30T12:57:05Z", "closed_at": "2026-05-08T12:09:11Z", "close_reason": "Closed", "dependency_count": 0, "dependent_count": 1, "comment_count": 0}` + "\n"
}

func TestTickClosePortAlreadyClosedCompletesPendingPersistence(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	t.Setenv("GIT_AUTHOR_NAME", "tick-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "tick@test")
	t.Setenv("GIT_COMMITTER_NAME", "tick-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "tick@test")
	dir := t.TempDir()
	ledger := filepath.Join(dir, "_beads")
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(workDir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = workDir
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, workDir, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit(dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "proof.md"), []byte("proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(dir, "add", "proof.md")
	runGit(dir, "commit", "-q", "-m", "seed public")
	publicBefore := runGit(dir, "rev-parse", "HEAD")

	runGit(ledger, "init", "-q")
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-done", "open")), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(ledger, "add", "issues.jsonl")
	runGit(ledger, "commit", "-q", "-m", "seed ledger")
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-done", "closed")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "close.md"), []byte("pending public persistence\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakebin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in show) printf '%s\\n' '{\"id\":\"cp-done\",\"status\":\"closed\"}' ;; sync) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &stderr}
	if err := tickClosePort(rt, "cp-done", "finish close", "proof.md", []string{"docs/close.md"}); err != nil {
		t.Fatalf("tickClosePort() error: %v (stderr=%q)", err, stderr.String())
	}
	ledgerAfter := runGit(ledger, "rev-parse", "HEAD")
	if ledgerAfter == "" || ledgerAfter == publicBefore {
		t.Fatalf("ledger close was not committed: ledger=%q public-before=%q", ledgerAfter, publicBefore)
	}
	publicAfter := runGit(dir, "rev-parse", "HEAD")
	if publicAfter == publicBefore {
		t.Fatal("pending public paths were not committed on retry")
	}
	if got, want := stdout.String(), "already closed cp-done @ "+tickShortSHA(ledgerAfter)+"\n"; got != want {
		t.Fatalf("tickClosePort() stdout = %q, want %q", got, want)
	}
}

func TestTickClosePortBDDoesNotTrustOrCommitBRLedger(t *testing.T) {
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "/foreign/br-ledger")
	dir := t.TempDir()
	for _, sub := range []string{"_beads", ".beads", "fakebin", "nested"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A stale/matching BR record must never short-circuit a BD close.
	if err := os.WriteFile(filepath.Join(dir, "_beads", "issues.jsonl"), []byte(tickTestLedgerLine("agentops-123", "closed")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proof.md"), []byte("durable proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	runGit("add", "proof.md")
	runGit("commit", "-q", "-m", "seed")

	stateFile := filepath.Join(dir, "bd-state")
	logFile := filepath.Join(dir, "bd.log")
	if err := os.WriteFile(stateFile, []byte("open\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBD := `#!/usr/bin/env bash
printf 'pwd=%s beads=%s args=%s\n' "$PWD" "${BEADS_DIR-unset}" "$*" >> "${TICK_TEST_BD_LOG:?}"
case "${1:-}" in
  show)
    status="$(tr -d '\n' < "${TICK_TEST_BD_STATE:?}")"
    printf '[{"id":"%s","status":"%s"}]\n' "${2:-}" "$status"
    ;;
  close) printf 'closed\n' > "${TICK_TEST_BD_STATE:?}" ;;
  update) printf 'open\n' > "${TICK_TEST_BD_STATE:?}" ;;
  list)
    status="$(tr -d '\n' < "${TICK_TEST_BD_STATE:?}")"
    printf '[{"id":"agentops-123","status":"%s"}]\n' "$status"
    ;;
  *) echo "unexpected bd call: $*" >&2; exit 43 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "fakebin", "bd"), []byte(fakeBD), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TICK_TEST_BD_STATE", stateFile)
	t.Setenv("TICK_TEST_BD_LOG", logFile)

	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: filepath.Join(dir, "nested"), stdout: &stdout, stderr: &stderr}
	if err := tickClosePort(rt, "agentops-123", "close bd issue", filepath.Join(dir, "proof.md"), nil); err != nil {
		t.Fatalf("tickClosePort() error: %v (stderr=%q)", err, stderr.String())
	}
	logBody, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("bd was never invoked: %v", err)
	}
	logText := string(logBody)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logText, "close agentops-123 --reason evidence: "+filepath.Join(dir, "proof.md")) {
		t.Fatalf("BD close missing; calls:\n%s", logText)
	}
	for _, line := range strings.Split(strings.TrimSpace(logText), "\n") {
		if !strings.Contains(line, "pwd="+resolvedDir+" ") || !strings.Contains(line, "beads=unset ") {
			t.Fatalf("BD child did not use canonical repo root with BEADS_DIR stripped: %q", line)
		}
	}
	if strings.Contains(logText, "sync") {
		t.Fatalf("BR-only sync leaked into BD close; calls:\n%s", logText)
	}
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git diff --cached: %v\n%s", err, out)
	} else if strings.Contains(string(out), ".beads") || strings.Contains(string(out), "_beads") {
		t.Fatalf("tracker ledger was staged in public repo: %s", out)
	}
}

func TestTickPublicStagePathsSkipsGitignoredRuntimeCorpusEvidence(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".agents/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "evidence", "close.md"), []byte("runtime proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q")
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	got := tickPublicStagePaths(rt, filepath.Join(dir, "_beads"), ".agents/evidence/close.md", []string{"docs/result.md"})
	want := []string{"docs/result.md"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tickPublicStagePaths() = %q, want %q", got, want)
	}
}

func TestTickClosePortPreflightsExplicitPathsBeforeTrackerMutation(t *testing.T) {
	for _, status := range []string{"open", "closed"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv("BEADS_DIR", "")
			dir := t.TempDir()
			ledger := filepath.Join(dir, "_beads")
			fakebin := filepath.Join(dir, "fakebin")
			if err := os.MkdirAll(ledger, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(fakebin, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-path", status)), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "proof.md"), []byte("proof\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			logFile := filepath.Join(dir, "br.log")
			fakeBR := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> \"${TICK_TEST_BR_LOG:?}\"\nexit 0\n"
			if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TICK_TEST_BR_LOG", logFile)
			var stdout, stderr bytes.Buffer
			rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &stderr}
			err := tickClosePort(rt, "cp-path", "close", "proof.md", []string{"missing-result.md"})
			if code := tickExitCode(err); code != tickExitCloseRef {
				t.Fatalf("exit = %d, want %d (err=%v stderr=%q)", code, tickExitCloseRef, err, stderr.String())
			}
			if _, err := os.Stat(logFile); !os.IsNotExist(err) {
				t.Fatalf("tracker was invoked before preflight completed: err=%v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("success output emitted on refusal: %q", stdout.String())
			}
		})
	}
}

func TestTickClosePortRetriesForwardAfterPublicPersistenceFailure(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	t.Setenv("GIT_AUTHOR_NAME", "tick-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "tick@test")
	t.Setenv("GIT_COMMITTER_NAME", "tick-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "tick@test")
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
	runGit := func(workDir string, args ...string) string {
		t.Helper()
		command := exec.Command(realGit, args...)
		command.Dir = workDir
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, workDir, err, out)
		}
		return strings.TrimSpace(string(out))
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
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-retry", "open")), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(ledger, "add", "issues.jsonl")
	runGit(ledger, "commit", "-q", "-m", "seed ledger")

	brLog := filepath.Join(dir, "br.log")
	failOnce := filepath.Join(dir, "fail-public-add-once")
	if err := os.WriteFile(failOnce, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBR := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TICK_TEST_BR_LOG:?}"
case "${1:-}" in
  show)
    if grep -q '"status": "closed"' "${TICK_TEST_LEDGER:?}/issues.jsonl"; then status=closed; else status=open; fi
    printf '{"id":"%s","status":"%s"}\n' "${2:-}" "$status"
    ;;
  close) sed 's/"status": "open"/"status": "closed"/' "${TICK_TEST_LEDGER:?}/issues.jsonl" > "${TICK_TEST_LEDGER:?}/issues.tmp" && mv "${TICK_TEST_LEDGER:?}/issues.tmp" "${TICK_TEST_LEDGER:?}/issues.jsonl" ;;
  sync) ;;
  *) echo "unexpected br call: $*" >&2; exit 43 ;;
esac
`
	fakeGit := `#!/usr/bin/env bash
if [ "${1:-}" = "add" ] && [ -f "${TICK_TEST_FAIL_ONCE:?}" ]; then
  rm -f "${TICK_TEST_FAIL_ONCE:?}"
  exit 77
fi
exec "${TICK_TEST_REAL_GIT:?}" "$@"
`
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TICK_TEST_BR_LOG", brLog)
	t.Setenv("TICK_TEST_LEDGER", ledger)
	t.Setenv("TICK_TEST_FAIL_ONCE", failOnce)
	t.Setenv("TICK_TEST_REAL_GIT", realGit)

	var firstOut, firstErr bytes.Buffer
	firstRT := tickRuntime{workDir: dir, stdout: &firstOut, stderr: &firstErr}
	err = tickClosePort(firstRT, "cp-retry", "finish close", "proof.md", []string{"docs/result.md"})
	if code := tickExitCode(err); code != tickExitNoCommit {
		t.Fatalf("first exit = %d, want %d (err=%v stderr=%q)", code, tickExitNoCommit, err, firstErr.String())
	}
	if firstOut.Len() != 0 {
		t.Fatalf("success output emitted on persistence failure: %q", firstOut.String())
	}
	if !tickLedgerShowsClosed(filepath.Join(ledger, "issues.jsonl"), "cp-retry") {
		t.Fatal("persistence failure reopened a proven-closed BR bead")
	}

	var retryOut, retryErr bytes.Buffer
	retryRT := tickRuntime{workDir: dir, stdout: &retryOut, stderr: &retryErr}
	if err := tickClosePort(retryRT, "cp-retry", "finish close", "proof.md", []string{"docs/result.md"}); err != nil {
		t.Fatalf("retry error: %v (stderr=%q)", err, retryErr.String())
	}
	logBody, err := os.ReadFile(brLog)
	if err != nil {
		t.Fatal(err)
	}
	closeCalls := 0
	for _, line := range strings.Split(strings.TrimSpace(string(logBody)), "\n") {
		if strings.HasPrefix(line, "close ") {
			closeCalls++
		}
		if strings.HasPrefix(line, "update ") {
			t.Fatalf("retry-forward state machine attempted rollback: %q", line)
		}
	}
	if closeCalls != 1 {
		t.Fatalf("tracker close calls = %d, want exactly 1 across failure+retry\n%s", closeCalls, logBody)
	}
	ledgerHead := runGit(ledger, "rev-parse", "HEAD")
	if got, want := retryOut.String(), "already closed cp-retry @ "+tickShortSHA(ledgerHead)+"\n"; got != want {
		t.Fatalf("retry stdout = %q, want %q", got, want)
	}
}

func TestTickClosePortTrackerStatusFailureIsNotTreatedAsOpen(t *testing.T) {
	t.Setenv("AGENTOPS_TRACKER", "bd")
	t.Setenv("BEADS_DIR", "/foreign/br-ledger")
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proof.md"), []byte("proof\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(dir, "bd.log")
	fakeBD := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> \"${TICK_TEST_BD_LOG:?}\"\nexit 77\n"
	if err := os.WriteFile(filepath.Join(fakebin, "bd"), []byte(fakeBD), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TICK_TEST_BD_LOG", logFile)
	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &stderr}
	err := tickClosePort(rt, "agentops-unknown", "close", "proof.md", nil)
	if code := tickExitCode(err); code != tickExitCloseFail {
		t.Fatalf("exit = %d, want %d (err=%v stderr=%q)", code, tickExitCloseFail, err, stderr.String())
	}
	logBody, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logBody), "close ") {
		t.Fatalf("unknown status was treated as open:\n%s", logBody)
	}
	if stdout.Len() != 0 {
		t.Fatalf("success output emitted on status failure: %q", stdout.String())
	}
}

func TestTickLedgerDirResolution(t *testing.T) {
	tests := []struct {
		name       string
		processEnv string   // value for the BEADS_DIR process env ("" = unset)
		rtEnv      []string // tickRuntime env overlay
		mkdirs     []string // directories created under workDir
		want       func(workDir string) string
	}{
		{
			name:   "uses _beads when it exists",
			mkdirs: []string{"_beads", ".beads"},
			want:   func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name:   "ignores retired .beads fallback",
			mkdirs: []string{".beads"},
			want:   func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name: "defaults to _beads when nothing exists",
			want: func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name:       "process BEADS_DIR wins over directory probe",
			processEnv: "custom_beads",
			mkdirs:     []string{"_beads"},
			want:       func(d string) string { return filepath.Join(d, "custom_beads") },
		},
		{
			name:       "rt.env BEADS_DIR wins over process env",
			processEnv: "process_beads",
			rtEnv:      []string{"BEADS_DIR=overlay_beads"},
			mkdirs:     []string{"_beads"},
			want:       func(d string) string { return filepath.Join(d, "overlay_beads") },
		},
		{
			name:  "last rt.env entry wins",
			rtEnv: []string{"BEADS_DIR=first", "BEADS_DIR=second"},
			want:  func(d string) string { return filepath.Join(d, "second") },
		},
		{
			name:       "absolute BEADS_DIR used verbatim",
			processEnv: "/abs/ledger",
			want:       func(string) string { return "/abs/ledger" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", tc.processEnv)
			dir := t.TempDir()
			for _, sub := range tc.mkdirs {
				if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			rt := tickRuntime{workDir: dir, env: tc.rtEnv}
			if got, want := tickLedgerDir(rt), tc.want(dir); got != want {
				t.Fatalf("tickLedgerDir() = %q, want %q", got, want)
			}
		})
	}
}

func TestTickRunTrackerResolvedBRPropagatesLedgerDir(t *testing.T) {
	t.Setenv("AGENTOPS_TRACKER", "br")
	t.Setenv("BEADS_DIR", "")
	dir := t.TempDir()
	ledger := filepath.Join(dir, "_beads")
	fakebin := filepath.Join(dir, "fakebin")
	for _, path := range []string{ledger, fakebin} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fakeBR := `#!/usr/bin/env bash
printf '%s\n' "${BEADS_DIR:-missing}"
`
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	rt := tickRuntime{workDir: dir}
	out, code, err := rt.runTracker("ready", "--json")
	if err != nil || code != 0 {
		t.Fatalf("runTracker() error=%v code=%d out=%q", err, code, out)
	}
	if got := strings.TrimSpace(string(out)); got != ledger {
		t.Fatalf("resolved absolute BR saw BEADS_DIR=%q, want %q", got, ledger)
	}
}

func TestTickStagePath(t *testing.T) {
	tests := []struct {
		name string
		root string
		path string
		want string
	}{
		{name: "under root is relative", root: "/repo", path: "/repo/_beads/issues.jsonl", want: filepath.Join("_beads", "issues.jsonl")},
		{name: "outside root stays absolute", root: "/repo", path: "/elsewhere/issues.jsonl", want: "/elsewhere/issues.jsonl"},
		{name: "root itself stays absolute via dot", root: "/repo", path: "/repo", want: "."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickStagePath(tc.root, tc.path); got != tc.want {
				t.Fatalf("tickStagePath(%q, %q) = %q, want %q", tc.root, tc.path, got, tc.want)
			}
		})
	}
}

// TestTickClose_PersistsLedgerInOwnRepoNotPublic is the L2 regression for the
// private br ledger contract: tickClose must persist _beads through its nested
// git repo and must never stage ledger files in the public repo.
func TestTickClose_PersistsLedgerInOwnRepoNotPublic(t *testing.T) {
	tests := []struct {
		name       string
		ledgerDir  string
		rtEnv      bool // set BEADS_DIR in rt.env to the ledger dir
		commitNoop bool
	}{
		{
			name:      "br workspace _beads",
			ledgerDir: "_beads",
		},
		{
			name:      "explicit BEADS_DIR override",
			ledgerDir: "custom_beads",
			rtEnv:     true,
		},
		{
			name:       "empty ledger commit is ok when ledger shows closed",
			ledgerDir:  "_beads",
			commitNoop: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", "")
			dir := t.TempDir()
			ledger := filepath.Join(dir, tc.ledgerDir)
			if err := os.MkdirAll(ledger, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-ledger", "closed")), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "metadata.json"), []byte(`{"database":"beads"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "evidence", "proof.md"), []byte("proof"), 0o644); err != nil {
				t.Fatal(err)
			}

			fakebin := filepath.Join(dir, "fakebin")
			if err := os.MkdirAll(fakebin, 0o755); err != nil {
				t.Fatal(err)
			}
			publicHeadFile := filepath.Join(dir, "fake-public-head")
			ledgerHeadFile := filepath.Join(dir, "fake-ledger-head")
			gitLog := filepath.Join(dir, "fake-git.log")
			if err := os.WriteFile(publicHeadFile, []byte("public1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ledgerHeadFile, []byte("ledger1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in close|sync|update) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
			fakeGit := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TICK_TEST_GIT_LOG:?}"
if [ "${1:-}" = "-C" ]; then
  shift 2
  case "${1:-}" in
    rev-parse) cat "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    add) exit 0 ;;
    commit)
      if [ "${TICK_TEST_LEDGER_COMMIT_NOOP:-}" = "1" ]; then
        echo "nothing to commit, working tree clean"
        exit 1
      fi
      echo ledger2 > "${TICK_TEST_LEDGER_HEAD_FILE:?}"
      ;;
    *) : ;;
  esac
  exit 0
fi
case "${1:-}" in
  rev-parse) cat "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  add) exit 0 ;;
  commit) echo public2 > "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  *) : ;;
esac
`
			if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
				t.Fatal(err)
			}

			// exec.Command resolves binaries via the parent process PATH
			// (not c.Env), so the stub PATH must go through t.Setenv.
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TICK_TEST_PUBLIC_HEAD_FILE", publicHeadFile)
			t.Setenv("TICK_TEST_LEDGER_HEAD_FILE", ledgerHeadFile)
			t.Setenv("TICK_TEST_GIT_LOG", gitLog)
			if tc.commitNoop {
				t.Setenv("TICK_TEST_LEDGER_COMMIT_NOOP", "1")
			}
			var stdout, stderr bytes.Buffer
			rt := tickRuntime{
				workDir: dir,
				stdout:  &stdout,
				stderr:  &stderr,
			}
			if tc.rtEnv {
				rt.env = []string{"BEADS_DIR=" + tc.ledgerDir}
			}
			if err := tickClose(rt, "cp-ledger", "close msg", "evidence/proof.md", []string{"docs/close.md"}); err != nil {
				t.Fatalf("tickClose() error: %v (stderr=%q)", err, stderr.String())
			}
			got, err := os.ReadFile(gitLog)
			if err != nil {
				t.Fatalf("git was never invoked: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(got)), "\n")
			wantLedgerAdd := "-C " + ledger + " add -- issues.jsonl metadata.json"
			wantLedgerCommit := "-C " + ledger + " commit -q -m close msg"
			if !tickTestLinesContain(lines, wantLedgerAdd) {
				t.Fatalf("git calls missing ledger add %q; got %q", wantLedgerAdd, strings.TrimSpace(string(got)))
			}
			if !tickTestLinesContain(lines, wantLedgerCommit) {
				t.Fatalf("git calls missing ledger commit %q; got %q", wantLedgerCommit, strings.TrimSpace(string(got)))
			}
			for _, line := range lines {
				if strings.HasPrefix(line, "add ") {
					if strings.Contains(line, "_beads") || strings.Contains(line, ".beads") ||
						strings.Contains(line, "issues.jsonl") || strings.Contains(line, "metadata.json") {
						t.Fatalf("public git add staged ledger path: %q", line)
					}
					if line != "add -- docs/close.md" {
						t.Fatalf("public git add = %q, want caller path only", line)
					}
				}
			}
			wantSHA := "ledger2"
			if tc.commitNoop {
				wantSHA = "ledger1"
			}
			if want := "closed cp-ledger @ " + wantSHA + "\n"; stdout.String() != want {
				t.Fatalf("tickClose() stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
}

// TestTickEvidenceRefusal_DurableBinding pins the durable close-evidence
// binding (age-l7yh): a close is refused when the cited evidence is a
// repo-internal path present in the working tree but NOT a committed git blob
// in HEAD (an ancestor of the close commit). Committed blobs are durable and
// allowed; the only exempt class is the gitignored .agents/** runtime corpus
// (ephemeral by design). Everything else is refused — uncommitted, modified,
// directory (tree), committed symlink, evidence/-substring, and anything that
// resolves outside the repo (including via a crafted symlink). Uses a real git
// repo to exercise the actual git incantations.
func TestTickEvidenceRefusal_DurableBinding(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	// A committed evidence blob (durable, an ancestor of any future commit).
	if err := os.WriteFile(filepath.Join(dir, "committed.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "committed.md")
	runGit("commit", "-q", "-m", "seed")
	// A repo-internal file present on disk but NOT committed (and not ignored).
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A gitignored runtime-corpus evidence file (.agents/** — ephemeral by
	// design, exempt) and a gitignored NON-corpus build artifact (logs/ — NOT
	// exempt: must be committed to serve as durable evidence).
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".agents/\nlogs/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "ev.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "build.log"), []byte("artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An uncommitted file under an evidence/ staging dir — the old leaky
	// substring exemption let this through; the durable binding now refuses it.
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "staged.md"), []byte("staged"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An evidence file OUTSIDE the repo (the git-blob binding cannot govern it).
	external := filepath.Join(t.TempDir(), "external.md")
	if err := os.WriteFile(external, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	cases := []struct {
		name       string
		evidence   string
		wantRefuse bool
	}{
		{"committed blob is durable -> allowed", "committed.md", false},
		{"uncommitted repo file -> refused (durable binding)", "uncommitted.md", true},
		{"absolute path inside repo, uncommitted -> refused (no abs bypass)", filepath.Join(dir, "uncommitted.md"), true},
		{"gitignored .agents evidence -> allowed (exempt)", ".agents/ev.md", false},
		{"gitignored non-corpus artifact -> refused (no fail-open)", "logs/build.log", true},
		{"uncommitted evidence/ staging file -> refused (no substring bypass)", "evidence/staged.md", true},
		{"evidence/ substring on uncommitted file -> refused (token, not text)", "uncommitted.md (see evidence/)", true},
		{"external (out-of-repo) absolute path -> refused (not in repo)", external, true},
		{"missing artifact -> refused", "nope.md", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evFirst := tickFirstEvidenceToken(tc.evidence)
			reason := tickEvidenceRefusal(rt, tc.evidence, evFirst)
			if (reason != "") != tc.wantRefuse {
				t.Fatalf("tickEvidenceRefusal(%q) reason=%q, wantRefuse=%v", tc.evidence, reason, tc.wantRefuse)
			}
		})
	}

	// Regression (pawl REFUTED round 1): evidence cited from a SUBDIRECTORY must
	// bind cwd-relative, not root-relative. A file committed inside sub/ and
	// cited as its subdir-relative name from rt.workDir=sub/ must be recognized
	// as committed (durable) and allowed — not falsely refused because HEAD:name
	// looked at the repo root.
	t.Run("committed evidence in subdir binds cwd-relative (allowed)", func(t *testing.T) {
		sub := filepath.Join(dir, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "subproof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", "sub/subproof.md")
		runGit("commit", "-q", "-m", "subdir evidence")
		subRT := tickRuntime{workDir: sub, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		if reason := tickEvidenceRefusal(subRT, "subproof.md", "subproof.md"); reason != "" {
			t.Fatalf("committed subdir evidence cited cwd-relative = %q, want allowed", reason)
		}
	})

	// Regression (pawl REFUTED round 8): a tracked file that is committed but has
	// uncommitted working-tree changes is NOT durably in history — the cited
	// content differs from HEAD — and must be refused.
	t.Run("committed-but-modified file -> refused (dirty != durable)", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "committed.md"), []byte("MODIFIED uncommitted"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.WriteFile(filepath.Join(dir, "committed.md"), []byte("proof"), 0o644) }()
		if reason := tickEvidenceRefusal(rt, "committed.md", "committed.md"); reason == "" {
			t.Fatalf("committed-but-modified evidence = allowed, want refused (working tree != HEAD)")
		}
	})

	// Regression (pawl REFUTED round 6): a committed DIRECTORY (tree) is not a
	// durable evidence blob. Citing a committed directory must be refused — the
	// gate requires a committed file (blob), not a whole directory.
	t.Run("committed directory (tree) -> refused (blob required)", func(t *testing.T) {
		treedir := filepath.Join(dir, "treedir")
		if err := os.MkdirAll(treedir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(treedir, "inner.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", "treedir/inner.md")
		runGit("commit", "-q", "-m", "tree evidence")
		if reason := tickEvidenceRefusal(rt, "treedir", "treedir"); reason == "" {
			t.Fatalf("committed directory cited as evidence = allowed, want refused (tree is not a blob)")
		}
	})

	// Regression (pawl REFUTED round 7): an in-repo SYMLINK whose target is
	// outside the repo must not bypass the binding. The evidence path is
	// classified lexically (the leaf symlink is not followed), so it stays
	// in-repo and — being uncommitted — is refused.
	t.Run("in-repo symlink to external target -> refused (no symlink escape)", func(t *testing.T) {
		external := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(external, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "linkout.md")
		if err := os.Symlink(external, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		if reason := tickEvidenceRefusal(rt, "linkout.md", "linkout.md"); reason == "" {
			t.Fatalf("in-repo symlink to external = allowed, want refused (symlink escape)")
		}
	})

	// Regression (pawl REFUTED round 10): a COMMITTED SYMLINK is a blob, but its
	// bytes are only the target path — the pointed-at content is not in history.
	// Citing a committed symlink must be refused (tree mode 120000, not 100xxx).
	t.Run("committed symlink -> refused (target not in history)", func(t *testing.T) {
		linktarget := filepath.Join(t.TempDir(), "target.md")
		if err := os.WriteFile(linktarget, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		syml := filepath.Join(dir, "committed-link.md")
		if err := os.Symlink(linktarget, syml); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		runGit("add", "committed-link.md")
		runGit("commit", "-q", "-m", "commit symlink evidence")
		if reason := tickEvidenceRefusal(rt, "committed-link.md", "committed-link.md"); reason == "" {
			t.Fatalf("committed symlink cited as evidence = allowed, want refused (mode 120000)")
		}
	})

	// Regression (pawl REFUTED round 10): an ABSOLUTE evidence path routed through
	// an in-repo SYMLINKED DIRECTORY whose target is outside the repo must not be
	// exempted as "outside" — the parent is resolved, the path resolves outside,
	// and outside now means refused (not allowed).
	t.Run("absolute path via in-repo symlinked dir -> refused", func(t *testing.T) {
		outdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(outdir, "proof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkdir := filepath.Join(dir, "linkdir")
		if err := os.Symlink(outdir, linkdir); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		abs := filepath.Join(linkdir, "proof.md")
		if reason := tickEvidenceRefusal(rt, abs, abs); reason == "" {
			t.Fatalf("absolute path via symlinked dir = allowed, want refused (escapes repo)")
		}
	})

	// Guard: outside a git repo (no resolvable HEAD) the durable binding must
	// NOT fire — there is no history to bind to, so an on-disk repo-internal
	// evidence file falls back to the existence check and is allowed.
	t.Run("no git history -> binding skipped (allowed)", func(t *testing.T) {
		plain := t.TempDir()
		if err := os.WriteFile(filepath.Join(plain, "proof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		plainRT := tickRuntime{workDir: plain, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		if reason := tickEvidenceRefusal(plainRT, "proof.md", "proof.md"); reason != "" {
			t.Fatalf("tickEvidenceRefusal in non-git dir = %q, want allowed", reason)
		}
	})
}

func TestTickCloseLinkedWorktreeUsesCanonicalBeadsLedger(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	root, lane := makeGitRepoWithLinkedWorktree(t)
	ledger := filepath.Join(root, "_beads")
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-linked", "closed")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "metadata.json"), []byte(`{"database":"beads"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(lane, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "evidence", "proof.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakebin := filepath.Join(t.TempDir(), "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	publicHeadFile := filepath.Join(fakebin, "fake-public-head")
	ledgerHeadFile := filepath.Join(fakebin, "fake-ledger-head")
	gitLog := filepath.Join(fakebin, "fake-git.log")
	if err := os.WriteFile(publicHeadFile, []byte("public1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerHeadFile, []byte("ledger1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in close|sync|update) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
	fakeGit := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TICK_TEST_GIT_LOG:?}"
if [ "${1:-}" = "-C" ]; then
  dir="$2"
  shift 2
  if [ "$dir" = "${TICK_TEST_WORKDIR:?}" ] && [ "${1:-}" = "rev-parse" ] && [ "${2:-}" = "--git-common-dir" ]; then
    printf '%s\n' "${TICK_TEST_GIT_COMMON_DIR:?}"
    exit 0
  fi
  if [ "$dir" = "${TICK_TEST_WORKDIR:?}" ] && [ "${1:-}" = "rev-parse" ] && [ "${2:-}" = "--show-toplevel" ]; then
    printf '%s\n' "${TICK_TEST_WORKDIR:?}"
    exit 0
  fi
  case "${1:-}" in
    rev-parse) cat "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    add) exit 0 ;;
    commit) echo ledger2 > "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    *) : ;;
  esac
  exit 0
fi
case "${1:-}" in
  rev-parse) cat "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  add) exit 0 ;;
  commit) echo public2 > "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  *) : ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TICK_TEST_WORKDIR", lane)
	t.Setenv("TICK_TEST_GIT_COMMON_DIR", filepath.Join(root, ".git"))
	t.Setenv("TICK_TEST_PUBLIC_HEAD_FILE", publicHeadFile)
	t.Setenv("TICK_TEST_LEDGER_HEAD_FILE", ledgerHeadFile)
	t.Setenv("TICK_TEST_GIT_LOG", gitLog)

	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: lane, stdout: &stdout, stderr: &stderr}
	if err := tickClose(rt, "cp-linked", "close msg", "evidence/proof.md", []string{"docs/close.md"}); err != nil {
		t.Fatalf("tickClose() error: %v (stderr=%q)", err, stderr.String())
	}
	got, err := os.ReadFile(gitLog)
	if err != nil {
		t.Fatalf("git was never invoked: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	wantCommonDir := "-C " + lane + " rev-parse --git-common-dir"
	if !tickTestLinesContain(lines, wantCommonDir) {
		t.Fatalf("git calls missing linked-worktree common-dir lookup %q; got %q", wantCommonDir, strings.TrimSpace(string(got)))
	}
	wantLedgerAdd := "-C " + ledger + " add -- issues.jsonl metadata.json"
	if !tickTestLinesContain(lines, wantLedgerAdd) {
		t.Fatalf("git calls missing canonical ledger add %q; got %q", wantLedgerAdd, strings.TrimSpace(string(got)))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "add ") && (strings.Contains(line, "_beads") ||
			strings.Contains(line, "issues.jsonl") || strings.Contains(line, "metadata.json")) {
			t.Fatalf("public git add staged ledger path: %q", line)
		}
	}
}

func tickTestLinesContain(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func TestTickFirstReadyFromJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "array",
			body: `[{"id":"cp-123","status":"open"},{"id":"cp-456","status":"open"}]`,
			want: "cp-123",
		},
		{
			name: "wrapped issues",
			body: `{"issues":[{"id":"cp-789","status":"open"}]}`,
			want: "cp-789",
		},
		{
			name: "empty",
			body: `[]`,
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickFirstReadyFromJSON([]byte(tc.body)); got != tc.want {
				t.Fatalf("tickFirstReadyFromJSON() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTickVerdictHasCommandsRun(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "missing",
			body: "VERDICT: PASS\nREASONS: ok\n",
			want: false,
		},
		{
			name: "empty",
			body: "COMMANDS RUN:\nREASONS:\nok\n",
			want: false,
		},
		{
			name: "inline",
			body: "COMMANDS RUN: ao tick guard-status\nREASONS: ok\n",
			want: true,
		},
		{
			name: "body",
			body: "COMMANDS RUN:\n  ao tick smoke\nREASONS: ok\n",
			want: true,
		},
		{
			name: "hyphenated heading",
			body: "commands-run:\n  git status\n",
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickVerdictHasCommandsRun(tc.body); got != tc.want {
				t.Fatalf("tickVerdictHasCommandsRun() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTickVerdictTokenCounts(t *testing.T) {
	pass, fail := tickVerdictTokenCounts("VERDICT: PASS\n verdict: fail\nnot VERDICT: PASS\n")
	if pass != 1 || fail != 1 {
		t.Fatalf("tickVerdictTokenCounts() = pass %d fail %d, want 1/1", pass, fail)
	}
}

func tickTestVerdict(author, judge, program, family, verdict, command string) string {
	// Each distinct judge gets a distinct non-author context so the typed-judge
	// tuple satisfies the context-floor (the independence axis is fresh context).
	return fmt.Sprintf("author: %s\njudge: %s\njudge_program: %s\njudge_model_family: %s\ncontext_id: ctx-%s\nVERDICT: %s\nCOMMANDS RUN:\n  %s\n", author, judge, program, family, judge, verdict, command)
}

func TestTickVerdictIdentityRequiresTypedIndependentJudge(t *testing.T) {
	valid := tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status")
	identity, gaps := tickVerdictIdentity(valid)
	if len(gaps) != 0 {
		t.Fatalf("valid identity gaps = %v, want none", gaps)
	}
	if identity.Author != "codex" || identity.JudgeName != "athena" || identity.JudgeProgram != "claude-code" || identity.JudgeModelFamily != "claude" {
		t.Fatalf("identity = %+v, want typed judge tuple", identity)
	}

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing judge program",
			body: "author: codex\njudge: athena\njudge_model_family: claude\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n",
		},
		{
			name: "self judge rejected despite inline waiver",
			body: "author: codex\njudge: codex\njudge_program: codex-cli\njudge_model_family: codex\nallow_self: true\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n",
		},
		{
			name: "unknown family rejected",
			body: tickTestVerdict("codex", "athena", "claude-code", "unknown", "PASS", "ao tick guard-status"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, gaps := tickVerdictIdentity(tc.body); len(gaps) == 0 {
				t.Fatal("tickVerdictIdentity() gaps = none, want fail-closed gap")
			}
		})
	}
}

func TestTickCouncilGateMatrix(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pass1 := write("pass1.md", tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status"))
	pass2 := write("pass2.md", tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "PASS", "ao tick verdict-gate -"))
	fail1 := write("fail1.md", tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "FAIL", "ao tick guard-status"))
	unverified := write("unverified.md", "VERDICT: PASS\n")
	contradictory := write("contradictory.md", "author: codex\njudge: athena\njudge_program: claude-code\njudge_model_family: claude\nVERDICT: FAIL\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")

	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, pass2})); code != 0 {
		t.Fatalf("two-pass council code = %d, want 0", code)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, fail1})); code != tickExitDisagree {
		t.Fatalf("mixed council code = %d, want %d", code, tickExitDisagree)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, unverified})); code != tickExitCouncil {
		t.Fatalf("unverified council code = %d, want %d", code, tickExitCouncil)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, contradictory})); code != tickExitCouncil {
		t.Fatalf("contradictory council code = %d, want %d", code, tickExitCouncil)
	}
}

// Retired: the family-floor council test (same-family two-context -> reject) was
// flipped to the context-floor in TestTickCouncilGateContextFloorFlip (the staged
// S1b.4 acceptance test). Self-judge (by context) and duplicate-context
// fail-closed are covered there.

// TestCouncilVerdictOversizedLine proves the council verdict parser fails closed
// on an artifact it cannot fully scan. A default bufio.Scanner stops at 64KB and,
// with an unchecked scanner.Err(), silently truncates the rest of the file — so a
// verdict whose well-formed PASS prelude is followed by an oversized line then a
// trailing FAIL was counted as a clean PASS (fail-open, Codex sweep F-03). The
// well-formed prelude is built via the production tickTestVerdict writer so the
// fixture is faithful (a control assertion proves it genuinely PASSes on its own).
func TestCouncilVerdictOversizedLine(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}

	// A genuine second PASS with a distinct judge context so the council has the
	// two independent contexts a PASS quorum requires.
	otherPass := write("other-pass.md", tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status"))

	// The well-formed prelude the attack is built on — produced by the production
	// verdict writer, distinct context from otherPass.
	prelude := tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "PASS", "ao tick verdict-gate -")

	// Control: the prelude alone is a genuine, independently-verifiable PASS. If
	// this does not PASS, the fixture is malformed and the attack cases prove
	// nothing.
	control := write("control.md", prelude)
	if code := tickExitCode(tickCouncilGate(rt, []string{otherPass, control})); code != 0 {
		t.Fatalf("control council code = %d, want 0 — well-formed prelude is not a genuine PASS; fixture is not faithful", code)
	}

	// Attack A (the F-03 reproduction): the same prelude, then a 70000-byte line
	// (past the 64KB default scan token), then a trailing FAIL. Pre-fix the
	// oversized line stops the scanner before the FAIL, so the verdict counts as a
	// clean PASS and the council PASSes (code 0) — a fail-open. Post-fix the
	// council must not PASS.
	hiddenFail := write("hidden-fail.md", prelude+strings.Repeat("A", 70000)+"\nVERDICT: FAIL\n")
	if code := tickExitCode(tickCouncilGate(rt, []string{otherPass, hiddenFail})); code == 0 {
		t.Fatal("council PASSed (code 0) with a 70000-byte line hiding a trailing FAIL — fail-open: the scanner truncated the artifact and never saw the FAIL")
	}

	// Attack B (the scanner.Err() path directly): a line past the deliberate scan
	// cap (1 MiB, tickVerdictLineCap) with no hidden FAIL. An artifact that cannot
	// be fully scanned must never be treated as a PASS. Pre-fix the overflow is
	// silently ignored and the prelude's PASS is honored (code 0); post-fix the
	// overflow must fail closed.
	overflow := write("overflow.md", prelude+strings.Repeat("A", (1<<20)+1)+"\n")
	if code := tickExitCode(tickCouncilGate(rt, []string{otherPass, overflow})); code == 0 {
		t.Fatal("council PASSed (code 0) with a line past the scan cap — fail-open: scanner overflow did not surface")
	}
}

func TestTickLedgerShowsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issues.jsonl")
	body := `{"id":"cp-open","status":"open"}` + "\n" +
		`{"id":"cp-done","status":"closed"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if !tickLedgerShowsClosed(path, "cp-done") {
		t.Fatal("expected closed bead to resolve")
	}
	if tickLedgerShowsClosed(path, "cp-open") {
		t.Fatal("open bead resolved as closed")
	}
}

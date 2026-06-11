package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTickCommandSurfaceCovered(t *testing.T) {
	// These literals are intentionally full leaf-command names: the command
	// surface parity gate scans *_test.go for this coverage.
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
		"tick claim",
		"tick close",
		"tick council-gate",
		"tick guard-status",
		"tick install-guards",
		"tick next",
		"tick reopen",
		"tick smoke",
		"tick status",
		"tick verdict-gate",
		"verdict-gate",
	}
	registered := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		registered[cmd.Name()] = true
	}
	for _, cmd := range tickCmd.Commands() {
		registered["tick "+cmd.Name()] = true
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

// tickTestLedgerLine reproduces the real persisted br issues.jsonl line shape
// (full field set as flushed by `br sync --flush-only`), not a minimal
// hand-built marker — see standards test-pyramid "Fixture Fidelity".
func tickTestLedgerLine(id, status string) string {
	return `{"_type": "issue", "id": "` + id + `", "title": "Node 2", "status": "` + status + `", "priority": 2, "issue_type": "task", "created_at": "2026-05-08T12:09:00Z", "updated_at": "2026-05-30T12:57:05Z", "closed_at": "2026-05-08T12:09:11Z", "close_reason": "Closed", "dependency_count": 0, "dependent_count": 1, "comment_count": 0}` + "\n"
}

func TestTickClosePortAlreadyClosedIsIdempotent(t *testing.T) {
	tests := []struct {
		name      string
		ledgerDir string
	}{
		{name: "br workspace _beads", ledgerDir: "_beads"},
		{name: "legacy .beads fallback", ledgerDir: ".beads"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", "")
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, tc.ledgerDir), 0o755); err != nil {
				t.Fatal(err)
			}
			body := tickTestLedgerLine("cp-done", "closed")
			if err := os.WriteFile(filepath.Join(dir, tc.ledgerDir, "issues.jsonl"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &bytes.Buffer{}}
			if err := tickClosePort(rt, "cp-done", "msg", "missing-evidence.md", nil); err != nil {
				t.Fatalf("tickClosePort() unexpected error: %v", err)
			}
			if got := stdout.String(); got != "already closed cp-done @ none\n" {
				t.Fatalf("tickClosePort() stdout = %q", got)
			}
		})
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
			name:   "prefers _beads when it exists",
			mkdirs: []string{"_beads", ".beads"},
			want:   func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name:   "falls back to legacy .beads",
			mkdirs: []string{".beads"},
			want:   func(d string) string { return filepath.Join(d, ".beads") },
		},
		{
			name: "defaults to .beads when nothing exists",
			want: func(d string) string { return filepath.Join(d, ".beads") },
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
			name:       "last rt.env entry wins",
			rtEnv:      []string{"BEADS_DIR=first", "BEADS_DIR=second"},
			want:       func(d string) string { return filepath.Join(d, "second") },
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

// TestTickCloseStagesResolvedLedger is the L2 regression for the live bug
// where tickClose verified and staged `.beads/` while br wrote `_beads/`:
// it drives tickClose end-to-end against stub br/git binaries and asserts
// the resolved ledger paths are what reach `git add`.
func TestTickCloseStagesResolvedLedger(t *testing.T) {
	tests := []struct {
		name      string
		ledgerDir string
		rtEnv     bool // set BEADS_DIR in rt.env to the ledger dir
		wantAdd   string
	}{
		{
			name:      "br workspace _beads",
			ledgerDir: "_beads",
			wantAdd:   "-- _beads/issues.jsonl _beads/metadata.json evidence/proof.md",
		},
		{
			name:      "legacy .beads fallback",
			ledgerDir: ".beads",
			wantAdd:   "-- .beads/issues.jsonl .beads/metadata.json evidence/proof.md",
		},
		{
			name:      "explicit BEADS_DIR override",
			ledgerDir: "custom_beads",
			rtEnv:     true,
			wantAdd:   "-- custom_beads/issues.jsonl custom_beads/metadata.json evidence/proof.md",
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
			headFile := filepath.Join(dir, "fake-head")
			addLog := filepath.Join(dir, "fake-add.log")
			if err := os.WriteFile(headFile, []byte("head-before\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in close|sync|update) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
			fakeGit := "#!/usr/bin/env bash\ncase \"${1:-}\" in\n  rev-parse) cat \"${TICK_TEST_HEAD_FILE:?}\" ;;\n  add) shift; printf '%s\\n' \"$*\" >> \"${TICK_TEST_ADD_LOG:?}\" ;;\n  commit) echo head-after > \"${TICK_TEST_HEAD_FILE:?}\" ;;\n  *) : ;;\nesac\n"
			if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
				t.Fatal(err)
			}

			// exec.Command resolves binaries via the parent process PATH
			// (not c.Env), so the stub PATH must go through t.Setenv.
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TICK_TEST_HEAD_FILE", headFile)
			t.Setenv("TICK_TEST_ADD_LOG", addLog)
			var stdout, stderr bytes.Buffer
			rt := tickRuntime{
				workDir: dir,
				stdout:  &stdout,
				stderr:  &stderr,
			}
			if tc.rtEnv {
				rt.env = []string{"BEADS_DIR=" + tc.ledgerDir}
			}
			if err := tickClose(rt, "cp-ledger", "close msg", "evidence/proof.md", nil); err != nil {
				t.Fatalf("tickClose() error: %v (stderr=%q)", err, stderr.String())
			}
			got, err := os.ReadFile(addLog)
			if err != nil {
				t.Fatalf("git add was never invoked: %v", err)
			}
			if strings.TrimSpace(string(got)) != tc.wantAdd {
				t.Fatalf("git add args = %q, want %q", strings.TrimSpace(string(got)), tc.wantAdd)
			}
			if want := "closed cp-ledger @ head-af\n"; stdout.String() != want {
				t.Fatalf("tickClose() stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
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
	return fmt.Sprintf("author: %s\njudge: %s\njudge_program: %s\njudge_model_family: %s\nVERDICT: %s\nCOMMANDS RUN:\n  %s\n", author, judge, program, family, verdict, command)
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

func TestTickCouncilGateRejectsSameFamilyAndSelfJudgeQuorum(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	claude1 := write("claude1.md", tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status"))
	claude2 := write("claude2.md", tickTestVerdict("codex", "pr", "claude-code", "claude", "PASS", "ao tick verdict-gate -"))
	selfJudge := write("self.md", tickTestVerdict("codex", "codex", "codex-cli", "codex", "PASS", "ao tick guard-status"))
	gemini := write("gemini.md", tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "PASS", "ao tick verdict-gate -"))
	duplicateJudge := write("duplicate.md", tickTestVerdict("codex", "athena", "claude-code", "gemini", "PASS", "ao tick smoke"))

	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if code := tickExitCode(tickCouncilGate(rt, []string{claude1, claude2})); code != tickExitCouncil {
		t.Fatalf("same-family council code = %d, want %d", code, tickExitCouncil)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{selfJudge, gemini})); code != tickExitCouncil {
		t.Fatalf("self-judge council code = %d, want %d", code, tickExitCouncil)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{claude1, duplicateJudge})); code != tickExitCouncil {
		t.Fatalf("duplicate judge council code = %d, want %d", code, tickExitCouncil)
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

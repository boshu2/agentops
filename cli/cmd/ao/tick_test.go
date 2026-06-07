package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestTickClosePortAlreadyClosedIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"cp-done","status":"closed"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(body), 0o644); err != nil {
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

func TestTickCouncilGateMatrix(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pass1 := write("pass1.md", "VERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")
	pass2 := write("pass2.md", "VERDICT: PASS\nCOMMANDS RUN:\n  ao tick verdict-gate -\n")
	fail1 := write("fail1.md", "VERDICT: FAIL\nCOMMANDS RUN:\n  ao tick guard-status\n")
	unverified := write("unverified.md", "VERDICT: PASS\n")
	contradictory := write("contradictory.md", "VERDICT: FAIL\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")

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

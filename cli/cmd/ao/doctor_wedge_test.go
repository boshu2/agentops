// practices: [sre, resilience-patterns]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/quality"
)

// writeFakeBin writes an executable stub named name into dir. The stub's
// process replaces itself via exec so a CommandContext kill lands on the
// actual long-running process, not a wrapping shell.
func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

// ---------------------------------------------------------------------------
// Reviewer reachability (L2: PATH-stubbed fake binaries, no live CLIs)
// ---------------------------------------------------------------------------

func TestCheckReviewerReachable_LiveFakeBinary(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "codex", "#!/bin/sh\necho fake-codex 1.0\n")
	t.Setenv("PATH", bin)

	c, live := checkReviewerReachable(reviewerProbe{name: "codex", installCmd: "install-codex"}, 5*time.Second)
	if !live {
		t.Error("live = false, want true for an instantly-answering binary")
	}
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass (detail: %s)", c.Status, c.Detail)
	}
	if c.Name != "Reviewer: codex" {
		t.Errorf("name = %q, want 'Reviewer: codex'", c.Name)
	}
	if !strings.Contains(c.Detail, "reachable") {
		t.Errorf("detail = %q, want it to say reachable", c.Detail)
	}
	if c.Required {
		t.Error("reviewer check must not be Required")
	}
}

func TestCheckReviewerReachable_HangingBinaryTimesOut(t *testing.T) {
	bin := t.TempDir()
	// Absolute /bin/sleep: the stubbed PATH must not break the hang itself.
	writeFakeBin(t, bin, "codex", "#!/bin/sh\nexec /bin/sleep 60\n")
	t.Setenv("PATH", bin)

	start := time.Now()
	c, live := checkReviewerReachable(reviewerProbe{name: "codex", installCmd: "install-codex"}, 300*time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Fatalf("doctor wedged on a hanging binary: probe took %s (timeout was 300ms)", elapsed)
	}
	if live {
		t.Error("live = true, want false for a hanging binary")
	}
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail (detail: %s)", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "timed out") {
		t.Errorf("detail = %q, want it to report the timeout", c.Detail)
	}
	// Directive 13: the failure must teach the exact corrective command.
	if !strings.Contains(c.Detail, "codex --version") {
		t.Errorf("detail = %q, want the corrective 'codex --version' command named", c.Detail)
	}
}

func TestCheckReviewerReachable_AbsentIsDegradedNotHung(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir: binary absent

	start := time.Now()
	c, live := checkReviewerReachable(reviewerProbe{name: "codex", installCmd: "npm install -g @openai/codex && codex login"}, 5*time.Second)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("absent binary must short-circuit, took %s", elapsed)
	}
	if live {
		t.Error("live = true, want false when binary is absent")
	}
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn (absent = degraded, not broken)", c.Status)
	}
	if !strings.Contains(c.Detail, "npm install -g @openai/codex") {
		t.Errorf("detail = %q, want the exact install one-liner named", c.Detail)
	}
}

func TestCheckReviewerReachable_BrokenBinaryFails(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "agy", "#!/bin/sh\nexit 7\n")
	t.Setenv("PATH", bin)

	c, live := checkReviewerReachable(reviewerProbe{name: "agy", installCmd: "reinstall-agy"}, 5*time.Second)
	if live {
		t.Error("live = true, want false for a binary that errors")
	}
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail (detail: %s)", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "reinstall-agy") {
		t.Errorf("detail = %q, want the corrective install command named", c.Detail)
	}
}

func TestReviewerReachabilityChecks_MixedFamilies(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "codex", "#!/bin/sh\nexit 0\n")
	// agy deliberately absent.
	t.Setenv("PATH", bin)

	checks, live := reviewerReachabilityChecks(5 * time.Second)
	if len(checks) != len(wedgeReviewers) {
		t.Fatalf("got %d checks, want %d (one per reviewer family)", len(checks), len(wedgeReviewers))
	}
	if len(live) != 1 || live[0] != "codex" {
		t.Errorf("live = %v, want [codex]", live)
	}
	if checks[0].Status != "pass" {
		t.Errorf("codex check status = %q, want pass (detail: %s)", checks[0].Status, checks[0].Detail)
	}
	if checks[1].Status != "warn" {
		t.Errorf("agy check status = %q, want warn for absent (detail: %s)", checks[1].Status, checks[1].Detail)
	}
}

// ---------------------------------------------------------------------------
// Cross-family summary
// ---------------------------------------------------------------------------

func TestCrossFamilyCheck(t *testing.T) {
	tests := []struct {
		name       string
		live       []string
		wantStatus string
		wantDetail string
	}{
		{"both live", []string{"codex", "agy"}, "pass", "cross-family capable: yes (live families: codex, agy)"},
		{"one live", []string{"agy"}, "pass", "cross-family capable: yes (live families: agy)"},
		{"none live", nil, "warn", "cross-family capable: no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := crossFamilyCheck(tt.live)
			if c.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", c.Status, tt.wantStatus)
			}
			if !strings.Contains(c.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", c.Detail, tt.wantDetail)
			}
		})
	}
	// The degraded report must teach the corrective install command.
	c := crossFamilyCheck(nil)
	if !strings.Contains(c.Detail, "npm install -g @openai/codex") {
		t.Errorf("degraded detail = %q, want the install one-liner named", c.Detail)
	}
}

// ---------------------------------------------------------------------------
// Binary freshness
// ---------------------------------------------------------------------------

// makeFakeAgentopsRepo scaffolds the two files binary-freshness detection
// reads: cli/go.mod with the agentops module line and cli/cmd/ao/main.go
// declaring `declared` as the version.
func makeFakeAgentopsRepo(t *testing.T, declared string) string {
	t.Helper()
	root := t.TempDir()
	aoDir := filepath.Join(root, "cli", "cmd", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gomod := agentopsModuleLine + "\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(root, "cli", "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	maingo := "package main\n\nvar version = \"" + declared + "\"\n"
	if err := os.WriteFile(filepath.Join(aoDir, "main.go"), []byte(maingo), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBinaryFreshnessCheck(t *testing.T) {
	tests := []struct {
		name       string
		declared   string // "" = no repo (bare temp dir)
		running    string
		wantStatus string
		wantDetail string
	}{
		{"match inside repo", "9.9.9", "9.9.9", "pass", "matches the repo's declared version"},
		{"stale binary inside repo", "9.9.9", "1.0.0", "warn", "scripts/preflight-uat-binary.sh"},
		{"outside repo reports version only", "", "1.0.0", "pass", "outside the agentops repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.declared != "" {
				dir = makeFakeAgentopsRepo(t, tt.declared)
			}
			c := binaryFreshnessCheck(dir, tt.running)
			if c.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (detail: %s)", c.Status, tt.wantStatus, c.Detail)
			}
			if !strings.Contains(c.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", c.Detail, tt.wantDetail)
			}
		})
	}
}

func TestBinaryFreshnessCheck_StaleNamesBothFixes(t *testing.T) {
	root := makeFakeAgentopsRepo(t, "9.9.9")
	c := binaryFreshnessCheck(root, "1.0.0")
	if !strings.Contains(c.Detail, "scripts/preflight-uat-binary.sh") || !strings.Contains(c.Detail, "brew upgrade agentops") {
		t.Errorf("detail = %q, want both corrective commands named", c.Detail)
	}
	if !strings.Contains(c.Detail, "1.0.0") || !strings.Contains(c.Detail, "9.9.9") {
		t.Errorf("detail = %q, want both versions named", c.Detail)
	}
}

func TestBinaryFreshnessCheck_ResolvesRootFromSubdir(t *testing.T) {
	root := makeFakeAgentopsRepo(t, "9.9.9")
	sub := filepath.Join(root, "cli", "cmd", "ao")
	c := binaryFreshnessCheck(sub, "9.9.9")
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass from a repo subdir (detail: %s)", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "matches the repo's declared version") {
		t.Errorf("detail = %q, want repo-match wording", c.Detail)
	}
}

// ---------------------------------------------------------------------------
// Ledger health (fixture fidelity: round-trip the PRODUCTION writer)
// ---------------------------------------------------------------------------

// appendFixtureEdges writes n chain-valid edges through the production
// provenancegraph writer (Store.Append seals + chains), never a hand-built
// on-disk shape.
func appendFixtureEdges(t *testing.T, path string, n int) {
	t.Helper()
	store := provenancegraph.NewStore(path)
	for i := 0; i < n; i++ {
		_, err := store.Append(provenancegraph.Edge{
			FromID:    fmt.Sprintf("age-test.%d", i),
			FromType:  "bead",
			ToID:      "abc1234",
			ToType:    "commit",
			Relation:  "wasGeneratedBy",
			TrustTier: "authored",
			TS:        time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("production append %d: %v", i, err)
		}
	}
}

func TestCheckLedgerHealth_IntactChainPassesWithRecency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs", "provenance", "ledger.jsonl")
	appendFixtureEdges(t, path, 2)

	c := checkLedgerHealth(path)
	if c.Status != "pass" {
		t.Fatalf("status = %q, want pass (detail: %s)", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "2 records") {
		t.Errorf("detail = %q, want the record count", c.Detail)
	}
	if !strings.Contains(c.Detail, "newest record") {
		t.Errorf("detail = %q, want feed recency reported", c.Detail)
	}
}

func TestCheckLedgerHealth_MissingLedgerPasses(t *testing.T) {
	c := checkLedgerHealth(filepath.Join(t.TempDir(), "docs", "provenance", "ledger.jsonl"))
	if c.Status != "pass" {
		t.Errorf("status = %q, want pass for an absent ledger (detail: %s)", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "no ledger records yet") {
		t.Errorf("detail = %q, want the empty-chain wording", c.Detail)
	}
}

func TestCheckLedgerHealth_TamperedChainFailsAndTeaches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs", "provenance", "ledger.jsonl")
	appendFixtureEdges(t, path, 2)

	// Tamper with committed content: alter a payload field in place. The
	// verifier (the same code path as `ao provenance verify`) must catch it.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(`"to_id":"abc1234"`), []byte(`"to_id":"abc1235"`), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("fixture did not contain the expected to_id field to tamper")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}

	c := checkLedgerHealth(path)
	if c.Status != "fail" {
		t.Fatalf("status = %q, want fail for a tampered chain (detail: %s)", c.Status, c.Detail)
	}
	if !c.Required {
		t.Error("a broken ledger chain must be a Required failure")
	}
	if !strings.Contains(c.Detail, "chain breaks at line 1") {
		t.Errorf("detail = %q, want the broken line named", c.Detail)
	}
	if !strings.Contains(c.Detail, "ao provenance verify") {
		t.Errorf("detail = %q, want the corrective 'ao provenance verify' named", c.Detail)
	}
}

// ---------------------------------------------------------------------------
// LAW-0 guard
// ---------------------------------------------------------------------------

func TestCheckLaw0Guard(t *testing.T) {
	tests := []struct {
		name       string
		environ    []string
		wantStatus string
		wantDetail string
	}{
		{
			name:       "clean env passes",
			environ:    []string{"PAWL_NO_SERVICE=1", "HOME=/home/x", "AGENTOPS_REPO_ROOT=/repo"},
			wantStatus: "pass",
			wantDetail: "no reviewer path configured through claude print-mode",
		},
		{
			name:       "empty env passes",
			environ:    nil,
			wantStatus: "pass",
			wantDetail: "no reviewer path configured",
		},
		{
			name:       "pawl var with claude print-flag fails",
			environ:    []string{"PAWL_REVIEWER_CMD=claude" + " -p review"},
			wantStatus: "fail",
			wantDetail: "unset PAWL_REVIEWER_CMD",
		},
		{
			name:       "reviewer var with claude print-word fails",
			environ:    []string{"MY_REVIEWER_BIN=claude" + " --print"},
			wantStatus: "fail",
			wantDetail: "unset MY_REVIEWER_BIN",
		},
		{
			name:       "unscoped var mentioning claude -p is ignored",
			environ:    []string{"SHELL_HISTORY=claude -p something"},
			wantStatus: "pass",
			wantDetail: "no reviewer path configured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := checkLaw0Guard(tt.environ)
			if c.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (detail: %s)", c.Status, tt.wantStatus, c.Detail)
			}
			if !strings.Contains(c.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", c.Detail, tt.wantDetail)
			}
			if !c.Required {
				t.Error("the LAW-0 guard is a Required check")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON schema + gather integration
// ---------------------------------------------------------------------------

// TestWedgeChecks_JSONSchemaMatchesExistingChecks proves the new checks
// serialize through the SAME quality.Check schema as every existing doctor
// check (name/status/detail/required keys).
func TestWedgeChecks_JSONSchemaMatchesExistingChecks(t *testing.T) {
	checks := []doctorCheck{
		crossFamilyCheck([]string{"codex"}),
		checkLaw0Guard(nil),
		binaryFreshnessCheck(t.TempDir(), "1.2.3"),
	}
	var buf bytes.Buffer
	if err := quality.RunDoctor(quality.DoctorOptions{JSON: true, Checks: checks, Stdout: &buf}); err != nil {
		t.Fatalf("RunDoctor JSON: %v", err)
	}
	var out quality.DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v\n%s", err, buf.String())
	}
	if len(out.Checks) != 3 {
		t.Fatalf("got %d checks in JSON, want 3", len(out.Checks))
	}
	if out.Checks[0].Name != "Cross-Family Review" || out.Checks[0].Status != "pass" {
		t.Errorf("check[0] = %+v, want Cross-Family Review pass", out.Checks[0])
	}
	for _, key := range []string{`"name"`, `"status"`, `"detail"`, `"required"`} {
		if !strings.Contains(buf.String(), key) {
			t.Errorf("JSON output missing schema key %s:\n%s", key, buf.String())
		}
	}
}

// TestGatherDoctorChecks_IncludesWedgeSection is the L2 wire-up proof: the
// wedge checks ride the same gather path the doctor command renders. PATH is
// stubbed so no real reviewer CLI is ever invoked.
func TestGatherDoctorChecks_IncludesWedgeSection(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	writeFakeBin(t, bin, "codex", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	checks := gatherDoctorChecks()
	got := make(map[string]doctorCheck, len(checks))
	for _, c := range checks {
		got[c.Name] = c
	}
	for _, want := range []string{
		"Reviewer: codex", "Reviewer: agy", "Cross-Family Review",
		"Binary Freshness", "Ledger Health", "LAW-0 Guard",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("gatherDoctorChecks missing wedge check %q", want)
		}
	}
	if c := got["Reviewer: codex"]; c.Status != "pass" {
		t.Errorf("stubbed codex status = %q, want pass (detail: %s)", c.Status, c.Detail)
	}
	if c := got["Reviewer: agy"]; c.Status != "warn" {
		t.Errorf("absent agy status = %q, want warn (detail: %s)", c.Status, c.Detail)
	}
	if c := got["Cross-Family Review"]; !strings.Contains(c.Detail, "cross-family capable: yes (live families: codex)") {
		t.Errorf("cross-family detail = %q, want codex live", c.Detail)
	}
}

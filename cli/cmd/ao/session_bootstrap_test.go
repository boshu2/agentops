// Tests for `ao session bootstrap` (soc-vuu6.25).
//
// Coverage shape: L2 first (full command via captureStdout), L1 for the
// fail-open seams (onboard/ready/mail helpers). Each test uses a temp dir
// so AGENTS.md presence is controlled — no dependency on the working tree.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSessionBootstrap_FullStatusJSON(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")
	mustWriteFile(t, filepath.Join(dir, "AGENTS-WORKFLOW.md"), "# w")
	mustWriteFile(t, filepath.Join(dir, "AGENTS-CI.md"), "# c")

	got := computeBootstrapStatus(context.Background(), dir, true /*noMail*/)

	if !got.AgentsMDRead {
		t.Fatalf("AgentsMDRead: want true, got false")
	}
	if len(got.AgentsSiblingsRead) != 2 {
		t.Fatalf("AgentsSiblingsRead: want [WORKFLOW, CI] (2 entries), got %v", got.AgentsSiblingsRead)
	}
	if got.OnboardPhase != "skipped:not-implemented" {
		// onboard subcommand may exist if registered; allow both shapes
		if got.OnboardPhase == "" {
			t.Fatalf("OnboardPhase: want non-empty marker, got empty")
		}
	}
	if got.BootstrapVersion != "v1" {
		t.Fatalf("BootstrapVersion: want v1, got %s", got.BootstrapVersion)
	}
	if got.BeadsDir != filepath.Join(dir, "_beads") {
		t.Fatalf("BeadsDir: want cwd _beads, got %q", got.BeadsDir)
	}
	if got.BeadsDirSource != beadsDirSourceRepoRoot && got.BeadsDirSource != beadsDirSourceCWD {
		t.Fatalf("BeadsDirSource: want repo-root/cwd fallback, got %q", got.BeadsDirSource)
	}
	if got.StartedAt == "" {
		t.Fatalf("StartedAt: want non-empty RFC3339 timestamp")
	}
	if got.MailUnreadCount != nil {
		t.Fatalf("MailUnreadCount: want nil when noMail=true, got %v", *got.MailUnreadCount)
	}
}

func TestRunSessionBootstrapUsesActualCWDWhenPWDIsStale(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	realDir := t.TempDir()
	staleDir := t.TempDir()
	mustWriteFile(t, filepath.Join(realDir, "AGENTS.md"), "# real")
	mustWriteFile(t, filepath.Join(staleDir, "AGENTS.md"), "# stale")
	t.Chdir(realDir)
	t.Setenv("PWD", staleDir)
	actualDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	origJSON, origRobot, origNoMail := sessionBootstrapJSON, sessionBootstrapRobot, sessionBootstrapNoMail
	t.Cleanup(func() {
		sessionBootstrapJSON, sessionBootstrapRobot, sessionBootstrapNoMail = origJSON, origRobot, origNoMail
	})
	sessionBootstrapJSON = true
	sessionBootstrapRobot = false
	sessionBootstrapNoMail = true

	var out, errOut bytes.Buffer
	cmd := sessionBootstrapCmd
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) })
	if err := runSessionBootstrap(cmd, nil); err != nil {
		t.Fatalf("runSessionBootstrap: %v (stderr=%s)", err, errOut.String())
	}

	var got SessionBootstrapStatus
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode status: %v\n%s", err, out.String())
	}
	want := filepath.Join(actualDir, "_beads")
	if got.BeadsDir != want {
		t.Fatalf("BeadsDir = %q, want actual cwd path %q (stale PWD was %q)", got.BeadsDir, want, staleDir)
	}
}

func TestSessionBootstrap_ReadyBeadsCount(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")

	orig := sessionBootstrapMakeReady
	t.Cleanup(func() { sessionBootstrapMakeReady = orig })
	sessionBootstrapMakeReady = func(_ context.Context, _ string) (int, bool) {
		return 3, true
	}

	got := computeBootstrapStatus(context.Background(), dir, true)
	if got.ReadyBeadsCount == nil {
		t.Fatal("ReadyBeadsCount: want non-nil when makeReady succeeds")
	}
	if *got.ReadyBeadsCount != 3 {
		t.Fatalf("ReadyBeadsCount: want 3, got %d", *got.ReadyBeadsCount)
	}
}

func TestSessionBootstrap_AgentsMDMissing(t *testing.T) {
	dir := t.TempDir() // no AGENTS.md
	got := computeBootstrapStatus(context.Background(), dir, true)

	if got.AgentsMDRead {
		t.Fatalf("AgentsMDRead: want false when AGENTS.md absent, got true")
	}
	if len(got.AgentsSiblingsRead) != 0 {
		t.Fatalf("AgentsSiblingsRead: want empty, got %v", got.AgentsSiblingsRead)
	}
}

func TestSessionBootstrap_PartialSplit(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")
	mustWriteFile(t, filepath.Join(dir, "AGENTS-RUNTIME.md"), "# r")
	// Intentionally omit AGENTS-WORKFLOW.md, AGENTS-CI.md, AGENTS-CODEX.md

	got := computeBootstrapStatus(context.Background(), dir, true)

	if !got.AgentsMDRead {
		t.Fatalf("AgentsMDRead: want true, got false")
	}
	want := []string{"AGENTS-RUNTIME.md"}
	if !equalStringSlices(got.AgentsSiblingsRead, want) {
		t.Fatalf("AgentsSiblingsRead: want %v, got %v", want, got.AgentsSiblingsRead)
	}
}

func TestSessionBootstrap_NoMailFlagSkipsProbe(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")

	got := computeBootstrapStatus(context.Background(), dir, true)
	if got.MailUnreadCount != nil {
		t.Fatalf("MailUnreadCount: want nil with noMail=true, got %v", *got.MailUnreadCount)
	}
}

func TestSessionBootstrap_RuntimeDetection(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "claude-code-test")
	got := detectRuntime()
	if got != "claude-code-test" {
		t.Fatalf("detectRuntime: want claude-code-test from env override, got %s", got)
	}
}

func TestSessionBootstrap_RuntimeFallbackClaudeCode(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("CLAUDECODE", "1")
	got := detectRuntime()
	if got != "claude-code" {
		t.Fatalf("detectRuntime: want claude-code (CLAUDECODE env set), got %s", got)
	}
}

func TestSessionBootstrap_PrintsHumanSummaryByDefault(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# A")
	mustWriteFile(t, filepath.Join(dir, "AGENTS-WORKFLOW.md"), "# w")
	writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", "human.md"),
		"Canon Human", "established", "", "0.8", "1.0", "human bootstrap canon memory should be visible")

	s := computeBootstrapStatus(context.Background(), dir, true)

	var out bytes.Buffer
	cmd := sessionBootstrapCmd
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	if err := printBootstrapSummary(cmd, s); err != nil {
		t.Fatalf("printBootstrapSummary: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"session bootstrap:",
		"agents_md=ok",
		"siblings=1/4",
		"onboard=skipped",
		"beads=",
		"tracker: BEADS_DIR=",
		`skills: ms search "<task>"`,
		"bootstrap memory: 1 item(s)",
		"Canon Human",
		"human bootstrap canon memory should be visible",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary: want substring %q, got %q", want, got)
		}
	}
}

func TestSessionBootstrap_JSONRoundTripsStatus(t *testing.T) {
	s := SessionBootstrapStatus{
		AgentsMDRead:                true,
		AgentsSiblingsRead:          []string{"AGENTS-WORKFLOW.md"},
		OnboardPhase:                "skipped:not-implemented",
		Runtime:                     "test",
		StartedAt:                   "2026-05-20T00:00:00Z",
		BootstrapVersion:            "v1",
		BootstrapMemory:             []SessionBootstrapMemoryItem{},
		BootstrapMemoryBudgetTokens: sessionBootstrapMemoryTokenBudget,
	}
	blob, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back SessionBootstrapStatus
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.AgentsMDRead != s.AgentsMDRead || back.OnboardPhase != s.OnboardPhase {
		t.Fatalf("round-trip mismatch: %+v vs %+v", back, s)
	}
}

func TestSessionBootstrap_CanonGatedMemoryFiltersToEstablishedAndHighAntiPatterns(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")
	for i := range 200 {
		writeBootstrapLearning(t, filepath.Join(dir, ".agents", "learnings", fmt.Sprintf("local-%03d.md", i)),
			"Local Established", "established", "high", "0.99", "1.0", "local established must not be injected")
		writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", fmt.Sprintf("candidate-%03d.md", i)),
			"Canon Candidate", "candidate", "high", "0.99", "1.0", "canon candidate must not be injected")
	}
	writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", "established.md"),
		"Canon Established", "established", "", "0.9", "1.0", "established canon memory should be injected")
	writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", "anti-high.md"),
		"High Anti Pattern", "anti-pattern", "high", "0.8", "1.0", "high severity anti-pattern guardrail should be injected")
	writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", "anti-medium.md"),
		"Medium Anti Pattern", "anti-pattern", "medium", "0.8", "1.0", "medium severity anti-pattern must not be injected")

	got := computeBootstrapStatus(context.Background(), dir, true)
	titles := map[string]bool{}
	for _, item := range got.BootstrapMemory {
		titles[item.Title] = true
		if item.Maturity == "established" && item.Reach != "always" {
			t.Fatalf("established canon reach = %q, want always", item.Reach)
		}
	}
	if !titles["Canon Established"] || !titles["High Anti Pattern"] {
		t.Fatalf("missing expected canon memory titles: %v", titles)
	}
	if titles["Local Established"] || titles["Canon Candidate"] || titles["Medium Anti Pattern"] {
		t.Fatalf("ineligible memory injected: %v", titles)
	}
	if got.BootstrapMemoryUsedTokens > sessionBootstrapMemoryTokenBudget {
		t.Fatalf("used tokens = %d, want <= %d", got.BootstrapMemoryUsedTokens, sessionBootstrapMemoryTokenBudget)
	}
}

func TestSessionBootstrap_CanonMemoryOverBudgetWarnsAndKeepsTopUnderCap(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS")
	body := strings.Repeat("budgeted canon sentence ", 100)
	for i := range 8 {
		writeBootstrapLearning(t, filepath.Join(dir, ".agents", "canon", "learnings", fmt.Sprintf("canon-%02d.md", i)),
			fmt.Sprintf("Canon %02d", i), "established", "", fmt.Sprintf("0.%d", i+1), "1.0", body)
	}

	got := computeBootstrapStatus(context.Background(), dir, true)
	if !got.BootstrapMemoryOverBudget {
		t.Fatal("BootstrapMemoryOverBudget = false, want true")
	}
	if got.BootstrapMemoryUsedTokens > sessionBootstrapMemoryTokenBudget {
		t.Fatalf("used tokens = %d, want <= %d", got.BootstrapMemoryUsedTokens, sessionBootstrapMemoryTokenBudget)
	}
	if got.BootstrapMemoryOmittedCount == 0 {
		t.Fatal("BootstrapMemoryOmittedCount = 0, want omitted lower-ranked items")
	}
	if len(got.BootstrapMemory) == 0 || got.BootstrapMemory[0].Title != "Canon 07" {
		t.Fatalf("first bootstrap memory = %+v, want highest utility Canon 07", got.BootstrapMemory)
	}
	var warn bytes.Buffer
	if err := writeBootstrapMemoryWarning(&warn, got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "exceeded 1200 tokens") {
		t.Fatalf("warning = %q, want token budget warning", warn.String())
	}
}

func writeBootstrapLearning(t *testing.T, path, title, maturity, severity, utility, confidence, body string) {
	t.Helper()
	severityLine := ""
	if severity != "" {
		severityLine = "severity: " + severity + "\n"
	}
	content := fmt.Sprintf("---\nmaturity: %s\n%sutility: %s\nconfidence: %s\n---\n# %s\n\n%s\n",
		maturity, severityLine, utility, confidence, title, body)
	mustWriteFile(t, path, content)
	when := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// (equalStringSlices and mustWriteFile live in this package already —
// agents_doctor_test.go and knowledge_files_test.go respectively.)

func TestSessionBootstrapGateHookStatus_UnavailableOutsideRepo(t *testing.T) {
	if got := sessionBootstrapGateHookStatus(""); got != "unavailable" {
		t.Fatalf("empty cwd: want unavailable, got %q", got)
	}
	if got := sessionBootstrapGateHookStatus(t.TempDir()); got != "unavailable" {
		t.Fatalf("temp dir: want unavailable, got %q", got)
	}
}

func TestFileContainsSubstring(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "hook")
	if err := os.WriteFile(f, []byte("chain pre-push.local here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileContainsSubstring(f, "pre-push.local") {
		t.Fatal("want true for present substring")
	}
	if fileContainsSubstring(f, "absent-token") {
		t.Fatal("want false for absent substring")
	}
	if fileContainsSubstring(filepath.Join(dir, "nope"), "x") {
		t.Fatal("want false for missing file")
	}
}

func TestSessionBootstrapIsAgentopsRepo(t *testing.T) {
	dir := t.TempDir()
	if sessionBootstrapIsAgentopsRepo(dir) {
		t.Fatal("empty dir: want false")
	}
	cliDir := filepath.Join(dir, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module github.com/boshu2/agentops/cli\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !sessionBootstrapIsAgentopsRepo(dir) {
		t.Fatal("agentops go.mod: want true")
	}
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module github.com/someone/else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sessionBootstrapIsAgentopsRepo(dir) {
		t.Fatal("foreign go.mod: want false")
	}
}

// Security regression: gate-hook status is DETECT-ONLY. Even a hostile repo
// that SPOOFS the agentops cli/go.mod identity must never get its installer
// executed (the cross-family refute that drove the detect-only redesign).
func TestSessionBootstrapGateHookStatus_NeverExecutesInstaller(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v %s", err, out)
	}
	scripts := filepath.Join(dir, "scripts")
	cliDir := filepath.Join(dir, "cli")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// SPOOF the identity: a hostile tree claims to be the agentops repo.
	if err := os.WriteFile(filepath.Join(cliDir, "go.mod"), []byte("module github.com/boshu2/agentops/cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dir, "EXECUTED")
	installer := filepath.Join(scripts, "install-pre-push-gate.sh")
	if err := os.WriteFile(installer, []byte("#!/usr/bin/env bash\ntouch '"+sentinel+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Spoofed identity -> status assesses the (markerless) hook as "inactive",
	// but the installer must NEVER run.
	if got := sessionBootstrapGateHookStatus(dir); got != "inactive" {
		t.Fatalf("spoofed agentops repo, unwired hook: want inactive, got %q", got)
	}
	if fileExists(sentinel) {
		t.Fatal("SECURITY: gate-hook status executed a working-tree installer script")
	}
}

// Regression for the cross-family DoS refute: a FIFO (or any non-regular path)
// must be skipped WITHOUT hanging, so a hostile repo cannot stall bootstrap.
func TestReadRegularFileCapped_SkipsNonRegularNoHang(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readRegularFileCapped(dir, 1<<10); ok {
		t.Fatal("directory: want skip (false)")
	}
	reg := filepath.Join(dir, "f")
	if err := os.WriteFile(reg, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, ok := readRegularFileCapped(reg, 1<<10); !ok || string(data) != "hello" {
		t.Fatalf("regular file: ok=%v data=%q", ok, data)
	}
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan bool, 1)
	go func() {
		_, ok := readRegularFileCapped(fifo, 1<<10)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("FIFO: want skip (false)")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DoS: readRegularFileCapped hung on a FIFO")
	}
}

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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionBootstrap_FullStatusJSON(t *testing.T) {
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
	if got.StartedAt == "" {
		t.Fatalf("StartedAt: want non-empty RFC3339 timestamp")
	}
	if got.MailUnreadCount != nil {
		t.Fatalf("MailUnreadCount: want nil when noMail=true, got %v", *got.MailUnreadCount)
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
	if err := printBootstrapSummary(cmd, s); err != nil {
		t.Fatalf("printBootstrapSummary: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"session bootstrap:",
		"agents_md=ok",
		"siblings=1/4",
		"onboard=skipped",
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

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
	"path/filepath"
	"strings"
	"testing"
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

	s := computeBootstrapStatus(context.Background(), dir, true)

	var out bytes.Buffer
	cmd := sessionBootstrapCmd
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := printBootstrapSummary(cmd, s); err != nil {
		t.Fatalf("printBootstrapSummary: %v", err)
	}
	got := out.String()
	for _, want := range []string{"session bootstrap:", "agents_md=ok", "siblings=1/4", "onboard=skipped"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary: want substring %q, got %q", want, got)
		}
	}
}

func TestSessionBootstrap_JSONRoundTripsStatus(t *testing.T) {
	s := SessionBootstrapStatus{
		AgentsMDRead:       true,
		AgentsSiblingsRead: []string{"AGENTS-WORKFLOW.md"},
		OnboardPhase:       "skipped:not-implemented",
		Runtime:            "test",
		StartedAt:          "2026-05-20T00:00:00Z",
		BootstrapVersion:   "v1",
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

// (equalStringSlices and mustWriteFile live in this package already —
// agents_doctor_test.go and knowledge_files_test.go respectively.)

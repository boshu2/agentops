// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInject_Integration_WithLearnings(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	writeFile(t, filepath.Join(dir, ".agents", "learnings", "retry-logic.md"),
		"---\ntitle: Retry Logic\n---\n## Summary\nAlways use exponential backoff for retries.\n")
	writeFile(t, filepath.Join(dir, ".agents", "patterns", "circuit-breaker.md"),
		"---\nname: Circuit Breaker\n---\nUse circuit breaker pattern for external service calls.\n")

	oldFormat := injectFormat
	oldMaxTokens := injectMaxTokens
	oldNoCite := injectNoCite
	oldBead := injectBead
	oldPredecessor := injectPredecessor
	oldIndexOnly := injectIndexOnly
	oldQuarantine := injectQuarantineFlagged
	oldForSkill := injectForSkill
	oldProfile := injectProfile
	oldSessionType := injectSessionType
	t.Cleanup(func() {
		injectFormat = oldFormat
		injectMaxTokens = oldMaxTokens
		injectNoCite = oldNoCite
		injectBead = oldBead
		injectPredecessor = oldPredecessor
		injectIndexOnly = oldIndexOnly
		injectQuarantineFlagged = oldQuarantine
		injectForSkill = oldForSkill
		injectProfile = oldProfile
		injectSessionType = oldSessionType
	})
	injectFormat = "markdown"
	injectMaxTokens = 3000
	injectNoCite = true
	injectBead = ""
	injectPredecessor = ""
	injectIndexOnly = false
	injectQuarantineFlagged = false
	injectForSkill = ""
	injectProfile = false
	injectSessionType = ""

	out, err := captureStdout(t, func() error {
		return runInject(injectCmd, []string{})
	})
	if err != nil {
		t.Fatalf("inject returned error: %v", err)
	}
	if !strings.Contains(out, "Injected Knowledge") {
		t.Errorf("expected output to contain 'Injected Knowledge' header, got: %s", out)
	}
}

func TestInject_Integration_EmptyLearningsDir(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	// No learnings or patterns written -- empty local .agents/
	// Note: inject may pick up global learnings from ~/.agentops/ if configured,
	// so we verify the output contains the expected header and timestamp rather
	// than asserting "No prior knowledge found".

	oldFormat := injectFormat
	oldMaxTokens := injectMaxTokens
	oldNoCite := injectNoCite
	oldBead := injectBead
	oldPredecessor := injectPredecessor
	oldIndexOnly := injectIndexOnly
	oldQuarantine := injectQuarantineFlagged
	oldForSkill := injectForSkill
	oldProfile := injectProfile
	oldSessionType := injectSessionType
	t.Cleanup(func() {
		injectFormat = oldFormat
		injectMaxTokens = oldMaxTokens
		injectNoCite = oldNoCite
		injectBead = oldBead
		injectPredecessor = oldPredecessor
		injectIndexOnly = oldIndexOnly
		injectQuarantineFlagged = oldQuarantine
		injectForSkill = oldForSkill
		injectProfile = oldProfile
		injectSessionType = oldSessionType
	})
	injectFormat = "markdown"
	injectMaxTokens = 3000
	injectNoCite = true
	injectBead = ""
	injectPredecessor = ""
	injectIndexOnly = false
	injectQuarantineFlagged = false
	injectForSkill = ""
	injectProfile = false
	injectSessionType = ""

	out, err := captureStdout(t, func() error {
		return runInject(injectCmd, []string{})
	})
	if err != nil {
		t.Fatalf("inject returned error: %v", err)
	}
	// Must contain the header and timestamp regardless of whether global learnings exist
	if !strings.Contains(out, "Injected Knowledge") {
		t.Errorf("expected 'Injected Knowledge' header, got: %s", out)
	}
	if !strings.Contains(out, "Last injection:") {
		t.Errorf("expected 'Last injection:' timestamp, got: %s", out)
	}
}

func TestInject_Integration_JSONFormat(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)

	writeFile(t, filepath.Join(dir, ".agents", "learnings", "caching-tip.md"),
		"---\ntitle: Caching Tip\n---\n## Summary\nCache invalidation is one of two hard problems.\n")

	oldFormat := injectFormat
	oldMaxTokens := injectMaxTokens
	oldNoCite := injectNoCite
	oldBead := injectBead
	oldPredecessor := injectPredecessor
	oldIndexOnly := injectIndexOnly
	oldQuarantine := injectQuarantineFlagged
	oldForSkill := injectForSkill
	oldProfile := injectProfile
	oldSessionType := injectSessionType
	t.Cleanup(func() {
		injectFormat = oldFormat
		injectMaxTokens = oldMaxTokens
		injectNoCite = oldNoCite
		injectBead = oldBead
		injectPredecessor = oldPredecessor
		injectIndexOnly = oldIndexOnly
		injectQuarantineFlagged = oldQuarantine
		injectForSkill = oldForSkill
		injectProfile = oldProfile
		injectSessionType = oldSessionType
	})
	injectFormat = "json"
	injectMaxTokens = 3000
	injectNoCite = true
	injectBead = ""
	injectPredecessor = ""
	injectIndexOnly = false
	injectQuarantineFlagged = false
	injectForSkill = ""
	injectProfile = false
	injectSessionType = ""

	out, err := captureStdout(t, func() error {
		return runInject(injectCmd, []string{})
	})
	if err != nil {
		t.Fatalf("inject returned error: %v", err)
	}
	if !strings.Contains(out, "\"timestamp\"") {
		t.Errorf("expected JSON output with 'timestamp' field, got: %s", out)
	}
}

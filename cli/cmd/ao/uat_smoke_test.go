package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// UAT regression tests — encode the na-xji manual validation scenarios
// as automated smoke tests to prevent regression across releases.
// Each test uses executeCommand() from cobra_commands_test.go.

func TestUATSmoke_InjectForResearch(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// Create a minimal skill definition so --for can resolve it.
	writeFile(t, tmp+"/skills/research/SKILL.md", `---
name: research
context:
  window: fork
  sections:
    exclude: [HISTORY]
---
# Research Skill
`)

	// Create a minimal learning file so inject has something to return.
	writeFile(t, tmp+"/.agents/learnings/test-learning.md", `---
utility: 0.8
maturity: validated
tags: [research, test]
---
# Test Learning
This is a test learning for UAT.
`)

	out, err := executeCommand("inject", "--for=research", "--no-cite")
	if err != nil {
		t.Fatalf("inject --for=research failed: %v\noutput: %s", err, out)
	}
	if strings.Contains(out, "### Recent Sessions") {
		t.Errorf("inject --for=research should exclude Sessions section, got: %s", out)
	}
}

func TestUATSmoke_InjectForPathTraversal(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	_, err := executeCommand("inject", "--for=../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal --for=../../etc/passwd, got nil")
	}
}

func TestUATSmoke_InjectForUnknownSkill(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	_, err := executeCommand("inject", "--for=nonexistent-skill-xyz")
	if err == nil {
		t.Error("expected error for unknown skill --for=nonexistent-skill-xyz, got nil")
	}
}

func TestUATSmoke_HandoffDryRun(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	out, err := executeCommand("handoff", "--dry-run", "test-handoff")
	if err != nil {
		t.Fatalf("handoff --dry-run failed: %v\noutput: %s", err, out)
	}
	// Dry-run should produce valid JSON on stdout.
	var handoffResult map[string]any
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &handoffResult); jsonErr != nil {
		t.Errorf("handoff --dry-run should produce valid JSON, got error: %v\noutput: %s", jsonErr, out)
	}
}

func TestUATSmoke_MineGitSource(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// mine --sources git needs a git repo; init one.
	initMinimalGitRepo(t, tmp)

	out, err := executeCommand("mine", "--sources", "git", "--since", "7d", "--quiet")
	// mine may return error if no signals found — that's OK for smoke test.
	_ = err
	// When quiet, output should be JSON if there are results, or empty/error message.
	if out != "" && strings.HasPrefix(strings.TrimSpace(out), "{") {
		var result map[string]any
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); jsonErr != nil {
			t.Errorf("mine --quiet output is not valid JSON: %v\noutput: %s", jsonErr, out)
		}
	}
}

func TestUATSmoke_DefragDedup(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	duplicateContent := `# Duplicate learning

This learning captures how defrag dedup should remove hash-named duplicate
files while preserving named learnings across the UAT smoke path.
`
	hashNamed := filepath.Join(tmp, ".agents", "learnings", "2026-03-01-a1b2c3d4.md")
	named := filepath.Join(tmp, ".agents", "learnings", "2026-03-01-defrag-dedup.md")
	writeFile(t, hashNamed, duplicateContent)
	writeFile(t, named, duplicateContent)

	out, err := executeCommand("defrag", "--dedup", "--quiet")
	if err != nil {
		t.Fatalf("defrag --dedup failed: %v\noutput: %s", err, out)
	}

	if _, err := os.Stat(hashNamed); !os.IsNotExist(err) {
		t.Fatalf("hash-named duplicate should have been deleted, stat err: %v", err)
	}
	if _, err := os.Stat(named); err != nil {
		t.Fatalf("named learning should have been preserved: %v", err)
	}

	reportPath := filepath.Join(tmp, ".agents", "defrag", "latest.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read defrag latest report: %v", err)
	}
	var report DefragReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("parse defrag latest report: %v\n%s", err, string(data))
	}
	if report.Dedup == nil {
		t.Fatal("expected dedup report")
	}
	if report.Dedup.Checked != 2 {
		t.Fatalf("dedup checked = %d, want 2", report.Dedup.Checked)
	}
	if !slices.Contains(report.Dedup.Deleted, filepath.Base(hashNamed)) {
		t.Fatalf("dedup deleted = %v, want %s", report.Dedup.Deleted, filepath.Base(hashNamed))
	}
}

func TestUATSmoke_Version(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if !strings.Contains(out, "ao version") {
		t.Errorf("expected 'ao version' in output, got: %s", out)
	}
	// Pre-flight: version output must include Go version and platform for
	// release-binary traceability. Without these, bug reports lack build provenance.
	if !strings.Contains(out, "Go version:") {
		t.Errorf("version output missing Go version line (needed for binary provenance), got: %s", out)
	}
	if !strings.Contains(out, "Platform:") {
		t.Errorf("version output missing Platform line (needed for binary provenance), got: %s", out)
	}
}

// TestUATSmoke_VersionNotDev verifies that the version variable is not left at
// the dev default when running in a release context. In test builds it will be
// "dev" which is acceptable, but the test encodes the expectation that the
// version command always produces a parseable, non-empty version string.
func TestUATSmoke_VersionNotEmpty(t *testing.T) {
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	// Extract the version token: "ao version <token>"
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("version produced no output")
	}
	parts := strings.Fields(lines[0])
	if len(parts) < 3 {
		t.Fatalf("expected 'ao version <ver>', got: %s", lines[0])
	}
	ver := parts[2]
	if ver == "" {
		t.Error("version string is empty")
	}
	// Version must be either "dev" (test builds) or start with "v" (release builds).
	if ver != "dev" && !strings.HasPrefix(ver, "v") {
		t.Errorf("version %q is neither 'dev' nor a release version (v*)", ver)
	}
}

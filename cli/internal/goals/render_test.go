package goals

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renderFixtureGoals is a minimal GOALS.md with a Directives section holding
// the directives the render tests exercise.
const renderFixtureGoals = `# Project Goals

## Directives

### 1. Fitness gate is BDD-driven

**Directive ID:** d-fitness-gate-bdd
**Steer:** keep gates honest
**Scenarios:** s-2026-05-17-001

### 2. Renderer tolerates plain scenarios

**Directive ID:** d-plain-scenario
**Scenarios:** s-2026-05-17-002

### 3. Lonely directive

**Directive ID:** d-lonely
**Steer:** has nothing linked
`

// writeRenderFixture writes a GOALS.md plus the two scenario JSON files the
// render tests reference under a fresh project root, returning that root.
func writeRenderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRenderFile(t, filepath.Join(root, "GOALS.md"), renderFixtureGoals)
	specDir := filepath.Join(root, "spec", "scenarios")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/scenarios: %v", err)
	}
	writeRenderFile(t, filepath.Join(specDir, "s-2026-05-17-001.json"), `{
  "id": "s-2026-05-17-001",
  "directive_id": "d-fitness-gate-bdd",
  "version": 1,
  "date": "2026-05-17",
  "goal": "Fitness gate fails on unsatisfied scenarios",
  "narrative": "n",
  "expected_outcome": "e",
  "satisfaction_threshold": 0.8,
  "status": "active",
  "given": ["a GOALS.md directive with a linked scenario", "the scenario is unsatisfied"],
  "when": ["the operator runs ao goals measure"],
  "then": ["the fitness gate reports a failure", "the exit code is non-zero"]
}
`)
	writeRenderFile(t, filepath.Join(specDir, "s-2026-05-17-002.json"), `{
  "id": "s-2026-05-17-002",
  "directive_id": "d-plain-scenario",
  "version": 1,
  "date": "2026-05-17",
  "goal": "Plain scenario without structured steps",
  "narrative": "n",
  "expected_outcome": "e",
  "satisfaction_threshold": 0.8,
  "status": "active"
}
`)
	return root
}

func writeRenderFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// renderTo runs RunRender for the fixture at root, capturing stdout in buf.
func renderTo(t *testing.T, root, out string, buf *bytes.Buffer) error {
	t.Helper()
	return RunRender(RenderOptions{
		GoalsFile:   filepath.Join(root, "GOALS.md"),
		ProjectRoot: root,
		Out:         out,
		Stdout:      buf,
	})
}

func TestRunRender_GherkinContent(t *testing.T) {
	root := writeRenderFixture(t)
	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	out := buf.String()

	tests := []struct {
		name string
		want string
	}{
		{"structured feature tag", "@d-fitness-gate-bdd\nFeature: Fitness gate is BDD-driven\n"},
		{"steer comment", "  # Steer: keep gates honest\n"},
		{"structured scenario tag", "  @s-2026-05-17-001\n  Scenario: Fitness gate fails on unsatisfied scenarios\n"},
		{"first given step", "    Given a GOALS.md directive with a linked scenario\n"},
		{"second given becomes And", "    And the scenario is unsatisfied\n"},
		{"when step", "    When the operator runs ao goals measure\n"},
		{"then step", "    Then the fitness gate reports a failure\n"},
		{"second then becomes And", "    And the exit code is non-zero\n"},
		{"fallback comment", "    # Scenario lacks structured given/when/then; no Gherkin steps to render.\n"},
		{"fallback goal comment", "    # Goal: Plain scenario without structured steps\n"},
		{"no-scenario directive feature", "@d-lonely\nFeature: Lonely directive\n"},
		{"no-scenario comment", "  # No scenarios linked to this directive.\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(out, tc.want) {
				t.Errorf("rendered Gherkin missing %q\n--- full output ---\n%s", tc.want, out)
			}
		})
	}
}

func TestRunRender_PlainScenarioHasNoFabricatedSteps(t *testing.T) {
	root := writeRenderFixture(t)
	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	out := buf.String()
	plainIdx := strings.Index(out, "@s-2026-05-17-002")
	if plainIdx < 0 {
		t.Fatalf("plain scenario tag missing from output:\n%s", out)
	}
	plainBlock := out[plainIdx:]
	if end := strings.Index(plainBlock[1:], "Feature:"); end >= 0 {
		plainBlock = plainBlock[:end+1]
	}
	for _, kw := range []string{"    Given ", "    When ", "    Then ", "    And "} {
		if strings.Contains(plainBlock, kw) {
			t.Errorf("plain scenario block fabricated a %q step:\n%s", strings.TrimSpace(kw), plainBlock)
		}
	}
}

func TestRunRender_OutWritesFile(t *testing.T) {
	root := writeRenderFixture(t)
	outPath := filepath.Join(root, "spec.feature")
	buf := &bytes.Buffer{}
	if err := renderTo(t, root, outPath, buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	if !strings.Contains(buf.String(), "Wrote Gherkin spec to "+outPath) {
		t.Errorf("stdout = %q, want confirmation of file write", buf.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}
	if !strings.Contains(string(data), "@d-fitness-gate-bdd\nFeature: Fitness gate is BDD-driven\n") {
		t.Errorf("rendered file missing expected Feature block:\n%s", data)
	}
	if strings.Contains(buf.String(), "Feature:") {
		t.Errorf("--out should suppress Gherkin on stdout, got: %s", buf.String())
	}
}

func TestRunRender_StdoutDefault(t *testing.T) {
	root := writeRenderFixture(t)
	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "# Generated by `ao goals render`") {
		prefix := out
		if len(prefix) > 60 {
			prefix = prefix[:60]
		}
		t.Errorf("stdout default should emit Gherkin directly, got prefix: %q", prefix)
	}
	if strings.Contains(out, "Wrote Gherkin spec to") {
		t.Error("stdout default must not print a file-write confirmation")
	}
}

func TestRunRender_NeverModifiesGoalsMD(t *testing.T) {
	root := writeRenderFixture(t)
	goalsPath := filepath.Join(root, "GOALS.md")
	before, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("reading GOALS.md: %v", err)
	}
	infoBefore, err := os.Stat(goalsPath)
	if err != nil {
		t.Fatalf("stat GOALS.md: %v", err)
	}

	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}

	after, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("re-reading GOALS.md: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("GOALS.md content changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	infoAfter, err := os.Stat(goalsPath)
	if err != nil {
		t.Fatalf("re-stat GOALS.md: %v", err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Errorf("GOALS.md mtime changed: %v -> %v", infoBefore.ModTime(), infoAfter.ModTime())
	}
}

// TestRunRender_UnresolvedScenarioEmitsFallbackComment verifies that when a
// directive references a scenario that does not exist in spec/scenarios/ or
// .agents/holdout/, the renderer emits a graceful fallback comment rather than
// crashing or producing invalid Gherkin.
func TestRunRender_UnresolvedScenarioEmitsFallbackComment(t *testing.T) {
	root := t.TempDir()
	writeRenderFile(t, filepath.Join(root, "GOALS.md"), `# Goals

## Directives

### 1. Directive with missing scenario

**Directive ID:** d-missing-scenario
**Steer:** test unresolved
**Scenarios:** s-9999-99-99-001
`)

	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "@s-9999-99-99-001") {
		t.Errorf("missing scenario tag in output:\n%s", out)
	}
	if !strings.Contains(out, "does not resolve") {
		t.Errorf("expected 'does not resolve' fallback comment in output:\n%s", out)
	}
	for _, kw := range []string{"    Given ", "    When ", "    Then "} {
		if strings.Contains(out, kw) {
			t.Errorf("fabricated %q step for unresolved scenario:\n%s", strings.TrimSpace(kw), out)
		}
	}
}

// TestRunRender_HoldoutScenarioResolvedAsGherkin verifies that a scenario
// present only in .agents/holdout/ (not promoted to spec/scenarios/) is still
// rendered with its structured steps.
func TestRunRender_HoldoutScenarioResolvedAsGherkin(t *testing.T) {
	root := t.TempDir()
	writeRenderFile(t, filepath.Join(root, "GOALS.md"), `# Goals

## Directives

### 1. Holdout scenario directive

**Directive ID:** d-holdout-test
**Steer:** test holdout
**Scenarios:** s-2026-01-01-099
`)
	holdoutDir := filepath.Join(root, ".agents", "holdout")
	if err := os.MkdirAll(holdoutDir, 0o755); err != nil {
		t.Fatalf("mkdir .agents/holdout: %v", err)
	}
	writeRenderFile(t, filepath.Join(holdoutDir, "s-2026-01-01-099.json"), `{
  "id": "s-2026-01-01-099",
  "directive_id": "d-holdout-test",
  "version": 1,
  "date": "2026-01-01",
  "goal": "Holdout scenario goal",
  "narrative": "n",
  "expected_outcome": "e",
  "satisfaction_threshold": 0.8,
  "status": "active",
  "given": ["a holdout scenario exists"],
  "when": ["render is run"],
  "then": ["the Gherkin is emitted"]
}
`)

	buf := &bytes.Buffer{}
	if err := renderTo(t, root, "", buf); err != nil {
		t.Fatalf("RunRender error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Scenario: Holdout scenario goal") {
		t.Errorf("holdout scenario goal not rendered:\n%s", out)
	}
	if !strings.Contains(out, "    Given a holdout scenario exists") {
		t.Errorf("holdout Given step missing:\n%s", out)
	}
	if !strings.Contains(out, "    When render is run") {
		t.Errorf("holdout When step missing:\n%s", out)
	}
	if !strings.Contains(out, "    Then the Gherkin is emitted") {
		t.Errorf("holdout Then step missing:\n%s", out)
	}
}

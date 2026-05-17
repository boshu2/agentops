package goals

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedScenarioClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC) }
}

func TestRunScenarioCreate_BidirectionalLink(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	specDir := filepath.Join(tmp, "spec", "scenarios")
	src := "# Goals\n\nm\n\n## Directives\n\n" +
		"### 1. First directive\n\nbody\n\n**Steer:** increase (x)\n\n" +
		"### 2. Target directive\n\nbody\n\n**Steer:** decrease (y)\n\n" +
		"## Three-Gap Contract Proof Surface\n\nkeep me verbatim\n"
	if err := os.WriteFile(goalsPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := RunScenarioCreate(ScenarioCreateOptions{
		GoalsFile: goalsPath, DirectiveNum: 2, Goal: "target behavior",
		Threshold: 0.8, Status: "draft", Source: "human",
		SpecDir: specDir, JSON: true, Stdout: &out, Now: fixedScenarioClock(),
	})
	if err != nil {
		t.Fatalf("RunScenarioCreate: %v", err)
	}
	var res ScenarioCreateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result JSON: %v\n%s", err, out.String())
	}
	if !res.Linked || res.DirectiveID == "" {
		t.Fatalf("result = %+v, want Linked with a directive_id", res)
	}

	// Endpoint 1: the scenario JSON carries the directive's stable ID.
	scData, err := os.ReadFile(res.ScenarioPath)
	if err != nil {
		t.Fatalf("scenario file: %v", err)
	}
	var sc struct {
		DirectiveID string `json:"directive_id"`
	}
	if err := json.Unmarshal(scData, &sc); err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if sc.DirectiveID != res.DirectiveID {
		t.Errorf("scenario directive_id = %q, want %q", sc.DirectiveID, res.DirectiveID)
	}

	// Endpoint 2: directive #2's Scenarios line lists the scenario.
	patched, _ := os.ReadFile(goalsPath)
	if !strings.Contains(string(patched), "**Scenarios:** "+res.ScenarioID) {
		t.Errorf("directive Scenarios line missing %s:\n%s", res.ScenarioID, patched)
	}
	// Non-target content survives the patch byte-for-byte.
	if !strings.Contains(string(patched), "## Three-Gap Contract Proof Surface\n\nkeep me verbatim") {
		t.Error("Three-Gap section not preserved by the patch")
	}
	// Directive #2 gained a stable ID; directive #1 is untouched.
	p, err := NewGoalsPatcher(patched)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if d2, _ := p.DirectiveByNumber(2); d2.StableID != res.DirectiveID {
		t.Errorf("directive 2 StableID = %q, want %q", d2.StableID, res.DirectiveID)
	}
	if d1, _ := p.DirectiveByNumber(1); d1.StableID != "" {
		t.Errorf("directive 1 must be untouched, got StableID %q", d1.StableID)
	}
}

func TestRunScenarioCreate_UnknownDirective(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte("# Goals\n\nm\n\n## Directives\n\n### 1. Only\n\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(goalsPath)
	err := RunScenarioCreate(ScenarioCreateOptions{
		GoalsFile: goalsPath, DirectiveNum: 9, Goal: "g",
		Threshold: 0.8, Status: "draft", Source: "human",
		SpecDir: filepath.Join(tmp, "spec"), Stdout: &bytes.Buffer{}, Now: fixedScenarioClock(),
	})
	if err == nil || !strings.Contains(err.Error(), "directive #9 not found") {
		t.Errorf("expected a not-found error, got %v", err)
	}
	if after, _ := os.ReadFile(goalsPath); string(before) != string(after) {
		t.Error("GOALS.md must be untouched when the target directive is not found")
	}
}

func TestRunScenarioCreate_CreationFailureLeavesGoalsUntouched(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte("# Goals\n\nm\n\n## Directives\n\n### 1. Only\n\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(goalsPath)
	// An invalid status fails scenario.Create after the in-memory stable-ID
	// edit — GOALS.md on disk must remain byte-identical.
	err := RunScenarioCreate(ScenarioCreateOptions{
		GoalsFile: goalsPath, DirectiveNum: 1, Goal: "g",
		Threshold: 0.8, Status: "bogus", Source: "human",
		SpecDir: filepath.Join(tmp, "spec"), Stdout: &bytes.Buffer{}, Now: fixedScenarioClock(),
	})
	if err == nil || !strings.Contains(err.Error(), "creating scenario") {
		t.Errorf("expected a scenario-creation error, got %v", err)
	}
	if after, _ := os.ReadFile(goalsPath); string(before) != string(after) {
		t.Errorf("GOALS.md must be untouched on scenario-creation failure\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// practices: [dora-metrics, continuous-delivery]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

// --- TestRatchetCheck_AllowedPhase ---

func TestRatchetCheck_AllowedPhase(t *testing.T) {
	// Research gate always passes (chaos phase, no prerequisites).
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepResearch)
	if err != nil {
		t.Fatalf("Check(research): %v", err)
	}
	if !result.Passed {
		t.Errorf("research gate should always pass, got Passed=false: %s", result.Message)
	}
	if result.Step != ratchet.StepResearch {
		t.Errorf("Step = %q, want research", result.Step)
	}
}

func TestRatchetCheck_VibeGateAlwaysPasses(t *testing.T) {
	// Vibe is a soft gate — always passes regardless.
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepVibe)
	if err != nil {
		t.Fatalf("Check(vibe): %v", err)
	}
	if !result.Passed {
		t.Errorf("vibe gate should always pass (soft gate), got Passed=false: %s", result.Message)
	}
}

func TestRatchetCheck_PostMortemSoftGate(t *testing.T) {
	// Post-mortem is a soft gate — always passes even without closed epic.
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepPostMortem)
	if err != nil {
		t.Fatalf("Check(post-mortem): %v", err)
	}
	if !result.Passed {
		t.Errorf("post-mortem gate should be soft (always passes), got Passed=false: %s", result.Message)
	}
}

// --- TestRatchetCheck_BlockedPhase ---

func TestRatchetCheck_BlockedPhase(t *testing.T) {
	// Implement gate requires an open epic via `bd` CLI, which is not
	// available in the test environment. Verify it returns a result
	// (not an error) and that the result has the correct step.
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	// Implement requires an epic from bd CLI -- should fail gracefully
	result, err := checker.Check(ratchet.StepImplement)
	if err != nil {
		t.Fatalf("Check(implement): %v", err)
	}
	if result.Step != ratchet.StepImplement {
		t.Errorf("Step = %q, want implement", result.Step)
	}
	// Without bd CLI, implement should not pass
	if result.Passed {
		t.Logf("implement gate passed (bd CLI may be available): %s", result.Message)
	}
}

func TestRatchetCheck_PreMortemPassesWithResearch(t *testing.T) {
	// Pre-mortem should pass when a research artifact exists in the local .agents/.
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "topic.md"), []byte("# Research\n\nFindings here."), 0o644); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepPreMortem)
	if err != nil {
		t.Fatalf("Check(pre-mortem): %v", err)
	}
	if !result.Passed {
		t.Errorf("pre-mortem gate should pass with research artifact, got: %s", result.Message)
	}
	if result.Input == "" {
		t.Error("expected Input to be set when gate passes")
	}
}

func TestRatchetCheck_PlanPassesWithSpec(t *testing.T) {
	// Plan should pass when a spec artifact exists.
	dir := t.TempDir()
	specsDir := filepath.Join(dir, ".agents", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "topic-v2.md"), []byte("# Spec\n\nPlan here."), 0o644); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepPlan)
	if err != nil {
		t.Fatalf("Check(plan): %v", err)
	}
	if !result.Passed {
		t.Errorf("plan gate should pass with spec artifact, got: %s", result.Message)
	}
	if result.Input == "" {
		t.Error("expected Input to be set when gate passes")
	}
}

func TestRatchetCheck_UnknownStep(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := ratchet.NewGateChecker(dir)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.Step("nonexistent"))
	if err != nil {
		t.Fatalf("Check(nonexistent): %v", err)
	}
	if result.Passed {
		t.Error("unknown step should not pass gate check")
	}
}

// --- TestRatchetRecord_WritesChain ---

func TestRatchetRecord_WritesChain(t *testing.T) {
	dir := t.TempDir()
	chainDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	chain := &ratchet.Chain{
		ID:      "test-chain-1",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{},
	}
	chainPath := filepath.Join(chainDir, ratchet.ChainFile)
	chain.SetPath(chainPath)

	entry := ratchet.ChainEntry{
		Step:      ratchet.StepResearch,
		Timestamp: time.Now(),
		Output:    ".agents/research/topic.md",
		Locked:    true,
	}

	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if content == "" {
		t.Fatal("chain file is empty")
	}

	// Verify chain has the entry in memory
	if len(chain.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(chain.Entries))
	}
	if chain.Entries[0].Step != ratchet.StepResearch {
		t.Errorf("Step = %q, want research", chain.Entries[0].Step)
	}
	if chain.Entries[0].Output != ".agents/research/topic.md" {
		t.Errorf("Output = %q, want .agents/research/topic.md", chain.Entries[0].Output)
	}
	if !chain.Entries[0].Locked {
		t.Error("expected Locked=true")
	}

	// Verify the chain can be reloaded
	loaded, err := ratchet.LoadChain(dir)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("reloaded chain: expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Step != ratchet.StepResearch {
		t.Errorf("reloaded Step = %q, want research", loaded.Entries[0].Step)
	}
}

// --- TestRatchetRecord_DuplicatePhase ---

func TestRatchetRecord_DuplicatePhase(t *testing.T) {
	dir := t.TempDir()
	chainDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	chain := &ratchet.Chain{
		ID:      "test-chain-dup",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{},
	}
	chainPath := filepath.Join(chainDir, ratchet.ChainFile)
	chain.SetPath(chainPath)

	// Record the same step twice
	for i := 0; i < 2; i++ {
		entry := ratchet.ChainEntry{
			Step:      ratchet.StepResearch,
			Timestamp: time.Now(),
			Output:    ".agents/research/topic.md",
			Locked:    true,
		}
		if err := chain.Append(entry); err != nil {
			t.Fatalf("Append #%d: %v", i+1, err)
		}
	}

	// Both entries should be recorded (chain is append-only, duplicates allowed)
	if len(chain.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(chain.Entries))
	}

	// GetLatest should return the most recent entry
	latest := chain.GetLatest(ratchet.StepResearch)
	if latest == nil {
		t.Fatal("GetLatest returned nil")
	}

	// Verify IsLocked returns true
	if !chain.IsLocked(ratchet.StepResearch) {
		t.Error("expected research step to be locked after recording")
	}
}

// --- TestRatchetValidate_ValidChain ---

func TestRatchetValidate_ValidChain(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a research artifact with required sections and schema_version
	researchDir := filepath.Join(agentsDir, "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := `---
schema_version: 1
---

# Research Topic

## Summary

This is a summary of the research findings.

## Key Findings

- Finding one with detailed explanation
- Finding two with more context
- Finding three with supporting evidence

## Recommendations

Based on the findings, we recommend the following steps.

Source: https://example.com/reference
`
	artifactPath := filepath.Join(researchDir, "topic.md")
	if err := os.WriteFile(artifactPath, []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}

	validator, err := ratchet.NewValidator(dir)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	result, err := validator.Validate(ratchet.StepResearch, artifactPath)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid, got invalid. Issues: %v", result.Issues)
	}
	if result.Tier == nil {
		t.Error("expected Tier to be set")
	}
}

// --- TestRatchetValidate_SkippedPhase ---

func TestRatchetValidate_SkippedPhase(t *testing.T) {
	// Chain status for a step that was skipped
	chain := &ratchet.Chain{
		ID:      "test-chain-skip",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{
			{
				Step:    ratchet.StepPreMortem,
				Skipped: true,
				Reason:  "Small change, pre-mortem not needed",
				Output:  "skipped",
				Locked:  false,
			},
		},
	}

	status := chain.GetStatus(ratchet.StepPreMortem)
	if status != ratchet.StatusSkipped {
		t.Errorf("Status = %q, want skipped", status)
	}

	// Skipped steps should not be locked
	if chain.IsLocked(ratchet.StepPreMortem) {
		t.Error("skipped step should not be locked")
	}

	// Steps that were never recorded should be pending
	status = chain.GetStatus(ratchet.StepResearch)
	if status != ratchet.StatusPending {
		t.Errorf("unrecorded step status = %q, want pending", status)
	}
}

func TestRatchetValidate_MissingArtifact(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validator, err := ratchet.NewValidator(dir)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	result, err := validator.Validate(ratchet.StepResearch, filepath.Join(dir, "nonexistent.md"))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for missing artifact")
	}
	if len(result.Issues) == 0 {
		t.Error("expected at least one issue for missing artifact")
	}
}

func TestRatchetValidate_StrictModeRejectsNoSchema(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create artifact WITHOUT schema_version
	artifactPath := filepath.Join(agentsDir, "research.md")
	content := "# Research\n\n## Summary\n\nSome findings.\n"
	if err := os.WriteFile(artifactPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	validator, err := ratchet.NewValidator(dir)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	// Strict mode (default) should fail for missing schema_version
	result, err := validator.Validate(ratchet.StepResearch, artifactPath)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.Valid {
		t.Error("strict mode should reject artifact without schema_version")
	}
}

func TestRatchetValidate_LenientModeAllowsNoSchema(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create artifact WITHOUT schema_version
	artifactPath := filepath.Join(agentsDir, "research.md")
	content := "# Research\n\n## Summary\n\nSome findings.\n## Key Findings\n\n- One.\n\n## Recommendations\n\nDo things.\n\nSource: https://example.com\n"
	if err := os.WriteFile(artifactPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	validator, err := ratchet.NewValidator(dir)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	// Lenient mode should allow missing schema_version
	opts := &ratchet.ValidateOptions{Lenient: true}
	result, err := validator.ValidateWithOptions(ratchet.StepResearch, artifactPath, opts)
	if err != nil {
		t.Fatalf("ValidateWithOptions: %v", err)
	}
	if !result.Valid {
		t.Errorf("lenient mode should pass without schema_version. Issues: %v", result.Issues)
	}
	if !result.Lenient {
		t.Error("expected Lenient=true in result")
	}
}

// --- Chain status comprehensive tests ---

func TestRatchetChain_GetAllStatus(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "test-all-status",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{
			{Step: ratchet.StepResearch, Locked: true, Output: "research.md"},
			{Step: ratchet.StepPreMortem, Skipped: true, Output: "skipped", Reason: "small"},
			{Step: ratchet.StepPlan, Locked: false, Output: "epic:test-001"},
		},
	}

	allStatus := chain.GetAllStatus()

	if allStatus[ratchet.StepResearch] != ratchet.StatusLocked {
		t.Errorf("research status = %q, want locked", allStatus[ratchet.StepResearch])
	}
	if allStatus[ratchet.StepPreMortem] != ratchet.StatusSkipped {
		t.Errorf("pre-mortem status = %q, want skipped", allStatus[ratchet.StepPreMortem])
	}
	if allStatus[ratchet.StepPlan] != ratchet.StatusInProgress {
		t.Errorf("plan status = %q, want in_progress", allStatus[ratchet.StepPlan])
	}
	if allStatus[ratchet.StepImplement] != ratchet.StatusPending {
		t.Errorf("implement status = %q, want pending", allStatus[ratchet.StepImplement])
	}
}

// --- ParseStep tests ---

func TestParseStep_Aliases(t *testing.T) {
	tests := []struct {
		input string
		want  ratchet.Step
	}{
		{"research", ratchet.StepResearch},
		{"pre-mortem", ratchet.StepPreMortem},
		{"premortem", ratchet.StepPreMortem},
		{"pre_mortem", ratchet.StepPreMortem},
		{"plan", ratchet.StepPlan},
		{"formulate", ratchet.StepPlan},
		{"implement", ratchet.StepImplement},
		{"crank", ratchet.StepCrank},
		{"autopilot", ratchet.StepCrank},
		{"execute", ratchet.StepCrank},
		{"vibe", ratchet.StepVibe},
		{"validate", ratchet.StepVibe},
		{"post-mortem", ratchet.StepPostMortem},
		{"postmortem", ratchet.StepPostMortem},
		{"review", ratchet.StepPostMortem},
		{"", ""},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ratchet.ParseStep(tt.input)
			if got != tt.want {
				t.Errorf("ParseStep(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Chain JSONL round-trip ---

func TestRatchetChain_JSONLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	chainDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second) // Truncate for JSON round-trip
	tier := ratchet.TierLearning

	chain := &ratchet.Chain{
		ID:      "roundtrip-chain",
		Started: now,
		EpicID:  "ag-test",
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: now,
				Output:    ".agents/research/topic.md",
				Locked:    true,
				Tier:      &tier,
			},
			{
				Step:      ratchet.StepPreMortem,
				Timestamp: now.Add(time.Minute),
				Input:     ".agents/research/topic.md",
				Output:    ".agents/specs/topic-v2.md",
				Locked:    true,
				Cycle:     1,
			},
		},
	}

	chainPath := filepath.Join(chainDir, ratchet.ChainFile)
	chain.SetPath(chainPath)
	if err := chain.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := ratchet.LoadChain(dir)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	if loaded.ID != chain.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, chain.ID)
	}
	if loaded.EpicID != chain.EpicID {
		t.Errorf("EpicID = %q, want %q", loaded.EpicID, chain.EpicID)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Step != ratchet.StepResearch {
		t.Errorf("Entry[0].Step = %q, want research", loaded.Entries[0].Step)
	}
	if loaded.Entries[0].Tier == nil {
		t.Error("Entry[0].Tier should be preserved through round-trip")
	} else if *loaded.Entries[0].Tier != tier {
		t.Errorf("Entry[0].Tier = %d, want %d", *loaded.Entries[0].Tier, tier)
	}
	if loaded.Entries[1].Cycle != 1 {
		t.Errorf("Entry[1].Cycle = %d, want 1", loaded.Entries[1].Cycle)
	}
}

// --- GateResult JSON output ---

func TestGateResult_JSONSerialization(t *testing.T) {
	result := ratchet.GateResult{
		Step:     ratchet.StepResearch,
		Passed:   true,
		Message:  "Research has no prerequisites",
		Input:    "",
		Location: "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ratchet.GateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Step != result.Step {
		t.Errorf("Step = %q, want %q", decoded.Step, result.Step)
	}
	if decoded.Passed != result.Passed {
		t.Errorf("Passed = %v, want %v", decoded.Passed, result.Passed)
	}
}

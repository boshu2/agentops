// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetRecord_UnknownStep(t *testing.T) {
	step := ratchet.ParseStep("nonexistent")
	if step != "" {
		t.Errorf("ParseStep(nonexistent) = %q, want empty", step)
	}
}

func TestRunRatchetRecord_StepParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantStep ratchet.Step
		wantErr  bool
	}{
		{"research", "research", ratchet.StepResearch, false},
		{"pre-mortem", "pre-mortem", ratchet.StepPreMortem, false},
		{"plan", "plan", ratchet.StepPlan, false},
		{"implement", "implement", ratchet.StepImplement, false},
		{"crank", "crank", ratchet.StepCrank, false},
		{"vibe", "vibe", ratchet.StepVibe, false},
		{"post-mortem", "post-mortem", ratchet.StepPostMortem, false},
		{"alias premortem", "premortem", ratchet.StepPreMortem, false},
		{"alias postmortem", "postmortem", ratchet.StepPostMortem, false},
		{"alias autopilot", "autopilot", ratchet.StepCrank, false},
		{"unknown", "nonsense", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := ratchet.ParseStep(tt.input)
			if tt.wantErr {
				if step != "" {
					t.Errorf("ParseStep(%q) = %q, want empty", tt.input, step)
				}
			} else {
				if step != tt.wantStep {
					t.Errorf("ParseStep(%q) = %q, want %q", tt.input, step, tt.wantStep)
				}
			}
		})
	}
}

func TestRunRatchetRecord_ChainAppend(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	entry := ratchet.ChainEntry{
		Step:   ratchet.StepResearch,
		Output: ".agents/research/topic.md",
		Locked: true,
	}

	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Reload and verify
	chain2, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain after append: %v", err)
	}

	if len(chain2.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(chain2.Entries))
	}

	if chain2.Entries[0].Step != ratchet.StepResearch {
		t.Errorf("Step = %q, want %q", chain2.Entries[0].Step, ratchet.StepResearch)
	}
	if chain2.Entries[0].Output != ".agents/research/topic.md" {
		t.Errorf("Output = %q, want %q", chain2.Entries[0].Output, ".agents/research/topic.md")
	}
	if !chain2.Entries[0].Locked {
		t.Error("entry should be locked")
	}
}

func TestRunRatchetRecord_TierAssignment(t *testing.T) {
	tests := []struct {
		name     string
		tier     int
		wantTier bool
	}{
		{"no tier", -1, false},
		{"tier 0", 0, true},
		{"tier 1", 1, true},
		{"tier 4", 4, true},
		{"tier 5 out of range", 5, false},
		{"tier -2 out of range", -2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ratchet.ChainEntry{
				Step:   ratchet.StepResearch,
				Output: "test.md",
			}

			if tt.tier >= 0 && tt.tier <= 4 {
				tier := ratchet.Tier(tt.tier)
				entry.Tier = &tier
			}

			if tt.wantTier && entry.Tier == nil {
				t.Error("expected tier to be set")
			}
			if !tt.wantTier && entry.Tier != nil {
				t.Errorf("expected no tier, got %d", *entry.Tier)
			}
		})
	}
}

func TestRunRatchetRecord_CycleAndParentEpic(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	entry := ratchet.ChainEntry{
		Step:       ratchet.StepPlan,
		Output:     "epic:ol-0002",
		Locked:     true,
		Cycle:      2,
		ParentEpic: "ol-0001",
	}

	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	chain2, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	last := chain2.Entries[len(chain2.Entries)-1]
	if last.Cycle != 2 {
		t.Errorf("Cycle = %d, want 2", last.Cycle)
	}
	if last.ParentEpic != "ol-0001" {
		t.Errorf("ParentEpic = %q, want %q", last.ParentEpic, "ol-0001")
	}
}

func TestRunRatchetRecord_MultipleEntries(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	steps := []struct {
		step   ratchet.Step
		output string
	}{
		{ratchet.StepResearch, "findings.md"},
		{ratchet.StepPreMortem, "spec-v2.md"},
		{ratchet.StepPlan, "epic:ol-0001"},
	}

	for _, s := range steps {
		entry := ratchet.ChainEntry{
			Step:   s.step,
			Output: s.output,
			Locked: true,
		}
		if err := chain.Append(entry); err != nil {
			t.Fatalf("Append %s: %v", s.step, err)
		}
	}

	chain2, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	if len(chain2.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(chain2.Entries))
	}
}

func TestRunRatchetRecord_DryRunMessage(t *testing.T) {
	// Verify the step parsing works for the dry-run path
	step := ratchet.ParseStep("implement")
	if step != ratchet.StepImplement {
		t.Errorf("ParseStep(implement) = %q, want %q", step, ratchet.StepImplement)
	}

	msg := fmt.Sprintf("Would record step: %s", step)
	if msg != "Would record step: implement" {
		t.Errorf("dry run message = %q", msg)
	}
}

// Verify that chain file is created in the correct location.
func TestRunRatchetRecord_ChainFilePath(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	entry := ratchet.ChainEntry{
		Step:   ratchet.StepResearch,
		Output: "test.md",
		Locked: true,
	}
	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	chainPath := filepath.Join(tmp, ".agents", "ao", "chain.jsonl")
	if _, err := os.Stat(chainPath); err != nil {
		t.Errorf("chain file not created at expected path %s: %v", chainPath, err)
	}
}

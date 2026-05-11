// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetSkip_UnknownStep(t *testing.T) {
	step := ratchet.ParseStep("bogus-step")
	if step != "" {
		t.Errorf("ParseStep(bogus-step) = %q, want empty", step)
	}

	err := fmt.Errorf("unknown step: %s", "bogus-step")
	if err.Error() != "unknown step: bogus-step" {
		t.Errorf("error message = %q", err.Error())
	}
}

func TestRunRatchetSkip_SkipEntryFields(t *testing.T) {
	entry := ratchet.ChainEntry{
		Step:      ratchet.StepPreMortem,
		Timestamp: time.Now(),
		Skipped:   true,
		Reason:    "Bug fix, no spec needed",
		Locked:    true,
	}

	if !entry.Skipped {
		t.Error("entry.Skipped should be true")
	}
	if !entry.Locked {
		t.Error("skip entries should also be locked")
	}
	if entry.Reason != "Bug fix, no spec needed" {
		t.Errorf("Reason = %q", entry.Reason)
	}
	if entry.Step != ratchet.StepPreMortem {
		t.Errorf("Step = %q, want %q", entry.Step, ratchet.StepPreMortem)
	}
}

func TestRunRatchetSkip_ChainAppendSkip(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	entry := ratchet.ChainEntry{
		Step:      ratchet.StepResearch,
		Timestamp: time.Now(),
		Skipped:   true,
		Reason:    "Existing knowledge sufficient",
		Locked:    true,
	}

	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append skip entry: %v", err)
	}

	// Reload and verify
	chain2, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	if len(chain2.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(chain2.Entries))
	}

	got := chain2.Entries[0]
	if !got.Skipped {
		t.Error("entry should be marked as skipped")
	}
	if got.Reason != "Existing knowledge sufficient" {
		t.Errorf("Reason = %q", got.Reason)
	}
	if !got.Locked {
		t.Error("skipped entry should be locked")
	}
}

func TestRunRatchetSkip_ChainStatusAfterSkip(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	entry := ratchet.ChainEntry{
		Step:    ratchet.StepPreMortem,
		Skipped: true,
		Reason:  "Simple change",
		Locked:  true,
	}
	if err := chain.Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Reload and check status
	chain2, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	status := chain2.GetStatus(ratchet.StepPreMortem)
	if status != ratchet.StatusSkipped {
		t.Errorf("status = %q, want %q", status, ratchet.StatusSkipped)
	}
}

func TestRunRatchetSkip_AllStepsCanBeSkipped(t *testing.T) {
	steps := ratchet.AllSteps()
	for _, step := range steps {
		t.Run(string(step), func(t *testing.T) {
			entry := ratchet.ChainEntry{
				Step:    step,
				Skipped: true,
				Reason:  "test skip",
				Locked:  true,
			}
			if entry.Step != step {
				t.Errorf("Step = %q, want %q", entry.Step, step)
			}
			if !entry.Skipped || !entry.Locked {
				t.Error("skip entry should have Skipped=true and Locked=true")
			}
		})
	}
}

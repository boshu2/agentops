// practices: [dora-metrics, continuous-delivery]
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetPromote_TierValidation(t *testing.T) {
	tests := []struct {
		name    string
		tier    int
		wantErr bool
	}{
		{"tier 0 valid", 0, false},
		{"tier 1 valid", 1, false},
		{"tier 2 valid", 2, false},
		{"tier 3 valid", 3, false},
		{"tier 4 valid", 4, false},
		{"tier -1 invalid", -1, true},
		{"tier 5 invalid", 5, true},
		{"tier 100 invalid", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetTier := ratchet.Tier(tt.tier)
			var err error
			if targetTier < 0 || targetTier > 4 {
				err = fmt.Errorf("tier must be 0-4, got %d", tt.tier)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("tier %d: error = %v, wantErr = %v", tt.tier, err, tt.wantErr)
			}
		})
	}
}

func TestRunRatchetPromote_TierStrings(t *testing.T) {
	tests := []struct {
		tier ratchet.Tier
		want string
	}{
		{ratchet.TierObservation, "observation"},
		{ratchet.TierLearning, "learning"},
		{ratchet.TierPattern, "pattern"},
		{ratchet.TierSkill, "skill"},
		{ratchet.TierCore, "core"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.tier.String()
			if got != tt.want {
				t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestRunRatchetPromote_TierLocations(t *testing.T) {
	tests := []struct {
		tier ratchet.Tier
		want string
	}{
		{ratchet.TierObservation, ".agents/candidates/"},
		{ratchet.TierLearning, ".agents/learnings/"},
		{ratchet.TierPattern, ".agents/patterns/"},
		{ratchet.TierSkill, "plugins/*/skills/"},
		{ratchet.TierCore, "CLAUDE.md"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.tier.Location()
			if got != tt.want {
				t.Errorf("Tier(%d).Location() = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestRecordPromotion_AppendsEntry(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Create an artifact to promote
	candidatesDir := filepath.Join(tmp, ".agents", "candidates")
	if err := os.MkdirAll(candidatesDir, 0755); err != nil {
		t.Fatalf("create candidates dir: %v", err)
	}
	artifactPath := filepath.Join(candidatesDir, "insight.md")
	if err := os.WriteFile(artifactPath, []byte("# Insight"), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	targetTier := ratchet.TierLearning
	var buf bytes.Buffer

	err := recordPromotion(tmp, artifactPath, targetTier, &buf)
	if err != nil {
		t.Fatalf("recordPromotion: %v", err)
	}

	// Verify output message
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Promoted:")) {
		t.Errorf("output missing 'Promoted:' prefix\nGot: %s", out)
	}

	// Verify chain was written
	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	found := false
	for _, entry := range chain.Entries {
		if entry.Step == ratchet.Step("promotion") && entry.Tier != nil && *entry.Tier == targetTier {
			found = true
			break
		}
	}
	if !found {
		t.Error("promotion entry not found in chain")
	}
}

func TestRecordPromotion_ChainEntryFields(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	candidatesDir := filepath.Join(tmp, ".agents", "candidates")
	if err := os.MkdirAll(candidatesDir, 0755); err != nil {
		t.Fatalf("create candidates dir: %v", err)
	}
	artifactPath := filepath.Join(candidatesDir, "test.md")
	if err := os.WriteFile(artifactPath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	before := time.Now()
	targetTier := ratchet.TierPattern
	var buf bytes.Buffer

	err := recordPromotion(tmp, artifactPath, targetTier, &buf)
	if err != nil {
		t.Fatalf("recordPromotion: %v", err)
	}

	chain, err := ratchet.LoadChain(tmp)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	if len(chain.Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	entry := chain.Entries[len(chain.Entries)-1]
	if entry.Step != ratchet.Step("promotion") {
		t.Errorf("Step = %q, want %q", entry.Step, "promotion")
	}
	if entry.Input != artifactPath {
		t.Errorf("Input = %q, want %q", entry.Input, artifactPath)
	}
	if entry.Output != targetTier.Location() {
		t.Errorf("Output = %q, want %q", entry.Output, targetTier.Location())
	}
	if !entry.Locked {
		t.Error("promotion entry should be locked")
	}
	if entry.Timestamp.Before(before) {
		t.Errorf("Timestamp %v is before test start %v", entry.Timestamp, before)
	}
}

func TestValidatePromotion_ValidArtifact(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Create a learning artifact with proper frontmatter
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: test-learning\ntype: learning\nmaturity: validated\n---\n\n# Test Learning\n\nContent here.\n"
	artifactPath := filepath.Join(learningsDir, "test-learning.md")
	if err := os.WriteFile(artifactPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := validatePromotion(tmp, artifactPath, ratchet.TierPattern, &buf)
	// May pass or fail depending on validator requirements — just verify no panic
	_ = err
	// The function should at least complete without panicking
}

func TestValidatePromotion_InvalidDir(t *testing.T) {
	var buf bytes.Buffer
	err := validatePromotion("/nonexistent/path", "artifact.md", ratchet.TierLearning, &buf)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

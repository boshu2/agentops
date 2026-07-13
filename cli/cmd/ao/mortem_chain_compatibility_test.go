package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func writeLegacyMortemJSONLChain(t *testing.T, step string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".agents", "ao")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir chain dir: %v", err)
	}
	data := `{"id":"legacy-mortem-jsonl","started":"2026-07-12T00:00:00Z"}` + "\n" +
		`{"step":"` + step + `","timestamp":"2026-07-12T00:01:00Z","output":"legacy.md","locked":true}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, ratchet.ChainFile), []byte(data), 0o600); err != nil {
		t.Fatalf("write chain: %v", err)
	}
	return root
}

func TestMortemJSONLChainLoad_NormalizesLegacyPremortem(t *testing.T) {
	root := writeLegacyMortemJSONLChain(t, "pre-mortem")
	chain, err := ratchet.LoadChain(root)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if latest := chain.GetLatest(ratchet.StepPreMortem); latest == nil {
		t.Fatal("GetLatest(premortem) lost the legacy pre-mortem JSONL entry")
	}
	if got := chain.GetStatus(ratchet.StepPreMortem); got != ratchet.StatusLocked {
		t.Errorf("GetStatus(premortem) = %q, want %q", got, ratchet.StatusLocked)
	}
	result := computeNextStep(chain)
	if result.Next != "plan" || result.Skill != "/plan" {
		t.Errorf("computeNextStep after legacy pre-mortem = next %q skill %q, want plan /plan", result.Next, result.Skill)
	}
}

func TestMortemJSONLChainLoad_NormalizesLegacyPostmortem(t *testing.T) {
	root := writeLegacyMortemJSONLChain(t, "post-mortem")
	chain, err := ratchet.LoadChain(root)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if latest := chain.GetLatest(ratchet.StepPostMortem); latest == nil {
		t.Fatal("GetLatest(postmortem) lost the legacy post-mortem JSONL entry")
	}
	if got := chain.GetStatus(ratchet.StepPostMortem); got != ratchet.StatusLocked {
		t.Errorf("GetStatus(postmortem) = %q, want %q", got, ratchet.StatusLocked)
	}
	result := computeNextStep(chain)
	if !result.Complete || result.Next != "" || result.LastStep != "postmortem" {
		t.Errorf("computeNextStep after legacy post-mortem = %+v, want canonical completed postmortem", result)
	}
}

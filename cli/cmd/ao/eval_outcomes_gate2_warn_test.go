package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestUncheckedJudgeHashWarning is the gate #2 advisory unit (council
// 2026-05-30 caveat #2): a score that carries a judge_content_hash but no
// configured --expect-judge-hash warns; a configured expected hash (parity
// enforced by requireJudgeHashParity) or a hashless score does not.
func TestUncheckedJudgeHashWarning(t *testing.T) {
	cases := []struct {
		name      string
		scoreHash string
		expected  string
		wantWarn  bool
	}{
		{"hash present, no expect → warn", "sha256:abc", "", true},
		{"hash present, expect set → no warn (parity enforced)", "sha256:abc", "sha256:abc", false},
		{"no hash, no expect → no warn", "", "", false},
		{"no hash, expect set → no warn", "", "sha256:abc", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := uncheckedJudgeHashWarning(c.scoreHash, c.expected)
			if c.wantWarn && got == "" {
				t.Errorf("uncheckedJudgeHashWarning(%q,%q) = \"\", want a warning", c.scoreHash, c.expected)
			}
			if !c.wantWarn && got != "" {
				t.Errorf("uncheckedJudgeHashWarning(%q,%q) = %q, want \"\"", c.scoreHash, c.expected, got)
			}
		})
	}
}

// TestRunEvalOutcomesIngest_WarnsWhenParityUnchecked exercises gate #2's advisory
// end-to-end through the command: a score carrying a judge_content_hash ingested
// WITHOUT --expect-judge-hash still succeeds (the warning is non-fatal) but emits
// a visible gate #2 warning to stderr, so the unenforced-parity gap is not
// silent. With --expect-judge-hash matching, no warning is emitted.
func TestRunEvalOutcomesIngest_WarnsWhenParityUnchecked(t *testing.T) {
	dir := t.TempDir()
	scorePath := filepath.Join(dir, "score.json")
	if err := os.WriteFile(scorePath, []byte(`{"source_task_id":"t","judge_content_hash":"sha256:aaa","aggregate":0.9,"threshold":0.8}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	origHash, origLedger := evalOutcomesIngestExpectHash, evalOutcomesIngestBurnLedger
	t.Cleanup(func() {
		evalOutcomesIngestExpectHash = origHash
		evalOutcomesIngestBurnLedger = origLedger
	})
	evalOutcomesIngestBurnLedger = ""

	// No --expect-judge-hash → warns on stderr, still ingests.
	evalOutcomesIngestExpectHash = ""
	var stderr, stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("ingest without --expect-judge-hash must still succeed (warning is non-fatal): %v", err)
	}
	if !strings.Contains(stderr.String(), "gate #2") || !strings.Contains(stderr.String(), "sha256:aaa") {
		t.Errorf("expected a gate #2 unchecked-parity warning naming the hash on stderr, got: %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("a verdict must still be emitted to stdout despite the warning")
	}

	// With matching --expect-judge-hash → parity enforced, no warning.
	evalOutcomesIngestExpectHash = "sha256:aaa"
	var stderr2 bytes.Buffer
	cmd2 := &cobra.Command{}
	cmd2.SetOut(&cobraDiscard{})
	cmd2.SetErr(&stderr2)
	if err := runEvalOutcomesIngest(cmd2, []string{scorePath}); err != nil {
		t.Fatalf("matching --expect-judge-hash must ingest cleanly: %v", err)
	}
	if stderr2.Len() != 0 {
		t.Errorf("no gate #2 warning expected when parity is configured, got: %q", stderr2.String())
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestRequireJudgeHashParity is the gate #2 (rubric-drift parity) unit: a score
// graded against a different judge_content_hash than the active rubric is
// refused; a matching hash passes; an empty expected hash is a no-op (parity not
// configured), so legacy/dev flows are unaffected.
func TestRequireJudgeHashParity(t *testing.T) {
	cases := []struct {
		name      string
		scoreHash string
		expected  string
		wantErr   bool
	}{
		{"match", "sha256:abc", "sha256:abc", false},
		{"mismatch refuses", "sha256:abc", "sha256:def", true},
		{"empty expected is no-op", "sha256:abc", "", false},
		{"empty score vs configured expected refuses", "", "sha256:def", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := requireJudgeHashParity(c.scoreHash, c.expected)
			if c.wantErr && err == nil {
				t.Errorf("requireJudgeHashParity(%q,%q) = nil, want error", c.scoreHash, c.expected)
			}
			if !c.wantErr && err != nil {
				t.Errorf("requireJudgeHashParity(%q,%q) = %v, want nil", c.scoreHash, c.expected, err)
			}
		})
	}
}

// TestRunEvalOutcomesIngest_RefusesOnHashMismatch exercises gate #2 end-to-end
// through the command: with --expect-judge-hash set to a value that differs from
// the score's judge_content_hash, ingest refuses and emits NO verdict (a stale
// rubric must not feed the Knowledge Flywheel). With a matching hash, it ingests.
func TestRunEvalOutcomesIngest_RefusesOnHashMismatch(t *testing.T) {
	dir := t.TempDir()
	scorePath := filepath.Join(dir, "score.json")
	if err := os.WriteFile(scorePath, []byte(`{"source_task_id":"t","judge_content_hash":"sha256:aaa","aggregate":0.9,"threshold":0.8}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Restore the package flag var after the test (it is process-global).
	orig := evalOutcomesIngestExpectHash
	t.Cleanup(func() { evalOutcomesIngestExpectHash = orig })

	cmd := &cobra.Command{}
	cmd.SetOut(&cobraDiscard{})

	// Mismatch → refuse.
	evalOutcomesIngestExpectHash = "sha256:bbb"
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err == nil {
		t.Fatal("ingest must refuse when judge_content_hash mismatches --expect-judge-hash (gate #2)")
	}

	// Match → ingest succeeds.
	evalOutcomesIngestExpectHash = "sha256:aaa"
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("matching judge_content_hash must ingest cleanly: %v", err)
	}

	// Unset → no parity check (legacy/dev path).
	evalOutcomesIngestExpectHash = ""
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("unset --expect-judge-hash must not enforce parity: %v", err)
	}
}

// cobraDiscard is an io.Writer that drops the verdict JSON the command prints.
type cobraDiscard struct{}

func (cobraDiscard) Write(p []byte) (int, error) { return len(p), nil }

// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

func TestEvaluateFlywheelGate(t *testing.T) {
	report := benchReport{
		Queries:    6,
		TargetPAtK: 0.67,
		Splits: map[string]benchSplitSummary{
			"holdout": {
				Cases:   3,
				AvgPAtK: 0.83,
				AvgMRR:  0.75,
			},
		},
	}
	metrics := &types.FlywheelMetrics{
		Rho: 0.60,
		GoldenSignals: &types.GoldenSignals{
			ClosureVerdict: "mining",
		},
	}

	result := evaluateFlywheelGate(metrics, report, "/tmp/corpus")
	if !result.Pass {
		t.Fatalf("expected pass, got reasons: %v", result.Reasons)
	}
	if result.ClosureVerdict != "mining" {
		t.Fatalf("ClosureVerdict = %q, want mining", result.ClosureVerdict)
	}
	if result.Benchmark.HoldoutPrecision < result.Benchmark.TargetPrecisionAtK {
		t.Fatalf("holdout precision = %.2f, want >= %.2f", result.Benchmark.HoldoutPrecision, result.Benchmark.TargetPrecisionAtK)
	}
}

func TestEvaluateFlywheelGate_FailsOnThresholds(t *testing.T) {
	report := benchReport{
		Queries:    4,
		TargetPAtK: 0.67,
		Splits: map[string]benchSplitSummary{
			"holdout": {
				Cases:   2,
				AvgPAtK: 0.50,
				AvgMRR:  0.50,
			},
		},
	}
	metrics := &types.FlywheelMetrics{
		Rho: 0.40,
		GoldenSignals: &types.GoldenSignals{
			ClosureVerdict: "unmined",
		},
	}

	result := evaluateFlywheelGate(metrics, report, "/tmp/corpus")
	if result.Pass {
		t.Fatal("expected failure, got pass")
	}
	wantReasons := []string{"research closure", "rho", "holdout precision"}
	for _, want := range wantReasons {
		found := false
		for _, reason := range result.Reasons {
			if strings.Contains(strings.ToLower(reason), want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected reason containing %q, got %v", want, result.Reasons)
		}
	}
}

func TestRunFlywheelGateCommand_PassesWithHealthyWorkspace(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0o755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o755); err != nil {
		t.Fatalf("mkdir research: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0o755); err != nil {
		t.Fatalf("mkdir ao: %v", err)
	}

	researchPath := filepath.Join(tmp, ".agents", "research", "r1.md")
	if err := os.WriteFile(researchPath, []byte("# Research R1\n"), 0o644); err != nil {
		t.Fatalf("write research: %v", err)
	}

	learningA := filepath.Join(tmp, ".agents", "learnings", "a.md")
	learningB := filepath.Join(tmp, ".agents", "learnings", "b.md")
	if err := os.WriteFile(learningA, []byte("Referenced research: .agents/research/r1.md\n"), 0o644); err != nil {
		t.Fatalf("write learning a: %v", err)
	}
	if err := os.WriteFile(learningB, []byte("# Learning B\n"), 0o644); err != nil {
		t.Fatalf("write learning b: %v", err)
	}

	citations := []types.CitationEvent{
		{ArtifactPath: learningA, SessionID: "s1", CitedAt: time.Now().Add(-time.Minute), CitationType: "reference", MatchConfidence: 0.9, MatchProvenance: "lookup:query"},
		{ArtifactPath: learningB, SessionID: "s2", CitedAt: time.Now(), CitationType: "applied", MatchConfidence: 0.9, MatchProvenance: "lookup:query"},
	}
	for _, citation := range citations {
		if err := ratchet.RecordCitation(tmp, citation); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	citationsPath := filepath.Join(tmp, ".agents", "ao", "citations.jsonl")
	if _, err := os.Stat(citationsPath); err != nil {
		t.Fatalf("stat citations file: %v", err)
	}

	loaded, err := ratchet.LoadCitations(tmp)
	if err != nil {
		t.Fatalf("load citations: %v", err)
	}
	metrics, err := computeMetrics(tmp, 7)
	if err != nil {
		t.Fatalf("compute metrics: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded citations = %d, want 2", len(loaded))
	}
	if metrics.Rho != 1 {
		t.Fatalf("computeMetrics rho = %.2f, want 1.00", metrics.Rho)
	}

	corpus := filepath.Join(origWD, "testdata", "retrieval-bench")
	if _, err := os.Stat(corpus); err != nil {
		t.Fatalf("stat corpus: %v", err)
	}

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.Flags().String("corpus", "", "")
	if err := cmd.Flags().Set("corpus", corpus); err != nil {
		t.Fatalf("set corpus flag: %v", err)
	}

	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)

	err = runFlywheelGate(cmd, nil)
	out := outBuf.String()
	if err != nil {
		t.Fatalf("ao flywheel gate failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Flywheel Gate: PASS") {
		t.Fatalf("expected PASS output, got:\n%s", out)
	}
}

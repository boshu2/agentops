package quality

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestCanonicalCitationType(t *testing.T) {
	cases := map[string]string{
		"applied":   "applied",
		"apply":     "applied",
		"APPLIED":   "applied",
		"retrieved": "retrieved",
		"retrieve":  "retrieved",
		"pulled":    "retrieved",
		"pull":      "retrieved",
		"unknown":   "unknown",
		"  APPLY ":  "applied",
	}
	for in, want := range cases {
		if got := CanonicalCitationType(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestLinearRegressionSlope(t *testing.T) {
	// Perfect increasing line y = 2x + 1
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{3, 5, 7, 9, 11}
	got := LinearRegressionSlope(xs, ys)
	if math.Abs(got-2.0) > 0.001 {
		t.Errorf("got %v, want ~2.0", got)
	}

	// Flat line
	got2 := LinearRegressionSlope([]float64{1, 2, 3}, []float64{5, 5, 5})
	if math.Abs(got2) > 0.001 {
		t.Errorf("flat: got %v, want 0", got2)
	}

	// Insufficient data
	if got := LinearRegressionSlope([]float64{1}, []float64{5}); got != 0 {
		t.Errorf("single point: got %v, want 0", got)
	}

	// Zero denominator (all xs equal)
	if got := LinearRegressionSlope([]float64{3, 3, 3}, []float64{1, 2, 3}); got != 0 {
		t.Errorf("vertical: got %v, want 0", got)
	}
}

func TestGiniCoefficient(t *testing.T) {
	// Perfect equality
	if got := GiniCoefficient([]float64{5, 5, 5, 5}); math.Abs(got) > 0.001 {
		t.Errorf("equal: got %v, want ~0", got)
	}

	// Empty
	if got := GiniCoefficient(nil); got != 0 {
		t.Errorf("empty: got %v", got)
	}

	// All zeros
	if got := GiniCoefficient([]float64{0, 0, 0}); got != 0 {
		t.Errorf("zeros: got %v", got)
	}

	// Max inequality: one person has everything
	got := GiniCoefficient([]float64{0, 0, 0, 10})
	if got < 0.5 {
		t.Errorf("unequal: got %v, want >0.5", got)
	}
	if got > 1.0 {
		t.Errorf("gini must be ≤1, got %v", got)
	}
}

func TestTop10BottomRatio(t *testing.T) {
	// Empty / small
	if got := Top10BottomRatio(nil); got != 0 {
		t.Errorf("nil: got %v", got)
	}

	// Uniform ascending
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := Top10BottomRatio(sorted)
	if got <= 0 {
		t.Errorf("should be positive, got %v", got)
	}

	// All zero bottom -> 999
	zeros := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 5}
	if got := Top10BottomRatio(zeros); got != 999.0 {
		t.Errorf("all-zero bottom: got %v, want 999", got)
	}
}

func TestExtractResearchRefsFromText(t *testing.T) {
	text := `See .agents/research/alpha.md for details.
Also /home/user/proj/.agents/research/beta.md matters.
And .agents/research/alpha.md again (should dedup).
Non-match: some other file.`
	refs := ExtractResearchRefsFromText(text)
	if len(refs) < 2 {
		t.Errorf("expected at least 2 refs, got %v", refs)
	}
}

func TestExtractResearchRefsFromText_Empty(t *testing.T) {
	if got := ExtractResearchRefsFromText("no refs here"); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestCountCitationsInPeriod(t *testing.T) {
	tmp := t.TempDir()
	citPath := filepath.Join(tmp, "citations.jsonl")
	recent := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	old := time.Now().Add(-365 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"artifact_path":".agents/learnings/a.md","cited_at":"` + recent + `"}
{"artifact_path":".agents/learnings/b.md","cited_at":"` + recent + `"}
{"artifact_path":".agents/learnings/a.md","cited_at":"` + old + `"}
not-json-line
`
	_ = os.WriteFile(citPath, []byte(body), 0o600)

	cutoff := time.Now().Add(-2 * time.Hour)
	counts := CountCitationsInPeriod(citPath, cutoff)
	if counts["a.md"] != 1 {
		t.Errorf("a.md = %d, want 1 (old one filtered)", counts["a.md"])
	}
	if counts["b.md"] != 1 {
		t.Errorf("b.md = %d", counts["b.md"])
	}
}

func TestLearningCitationCounts(t *testing.T) {
	tmp := t.TempDir()
	names := []string{"a.md", "b.md", "skip.txt"}
	for _, n := range names {
		_ = os.WriteFile(filepath.Join(tmp, n), []byte("x"), 0o600)
	}
	_ = os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755)

	entries, _ := os.ReadDir(tmp)
	citCounts := map[string]int{"a.md": 5, "b.md": 0}

	counts, cited, total := LearningCitationCounts(entries, citCounts)
	if total != 2 {
		t.Errorf("total = %d (should skip subdir and .txt)", total)
	}
	if cited != 1 {
		t.Errorf("cited = %d", cited)
	}
	if len(counts) != 2 {
		t.Errorf("counts len = %d", len(counts))
	}
}

func TestComputeOverallVerdict(t *testing.T) {
	// 3+ positives -> compounding
	gs := &types.GoldenSignals{
		TrendVerdict:         "compounding",
		PipelineVerdict:      "reinforcing",
		ClosureVerdict:       "mining",
		ConcentrationVerdict: "concentrated",
	}
	if got := ComputeOverallVerdict(gs); got != "compounding" {
		t.Errorf("got %q", got)
	}

	// 3+ negatives -> decaying
	gs2 := &types.GoldenSignals{
		TrendVerdict:         "decaying",
		PipelineVerdict:      "degrading",
		ClosureVerdict:       "hoarding",
		ConcentrationVerdict: "dormant",
	}
	if got := ComputeOverallVerdict(gs2); got != "decaying" {
		t.Errorf("got %q", got)
	}

	// Neutral -> accumulating
	gs3 := &types.GoldenSignals{
		TrendVerdict:         "stable",
		PipelineVerdict:      "stable",
		ClosureVerdict:       "stable",
		ConcentrationVerdict: "stable",
	}
	if got := ComputeOverallVerdict(gs3); got != "accumulating" {
		t.Errorf("got %q", got)
	}
}

func TestFprintGoldenSignals_Nil(t *testing.T) {
	var buf bytes.Buffer
	FprintGoldenSignals(&buf, nil)
	if !strings.Contains(buf.String(), "insufficient data") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestFprintGoldenSignals_Populated(t *testing.T) {
	var buf bytes.Buffer
	gs := &types.GoldenSignals{
		TrendVerdict:         "compounding",
		PipelineVerdict:      "reinforcing",
		ClosureVerdict:       "mining",
		ConcentrationVerdict: "concentrated",
		OverallVerdict:       "compounding",
	}
	FprintGoldenSignals(&buf, gs)
	out := buf.String()
	for _, expected := range []string{"GOLDEN SIGNALS", "Velocity Trend", "compounding", "OVERALL"} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q in output: %q", expected, out)
		}
	}
}

func TestReadJSONLFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.jsonl")
	_ = os.WriteFile(path, []byte(`{"a":1}

{"b":2}
`), 0o600)

	got := ReadJSONLFile(path)
	if len(got) != 2 {
		t.Errorf("got %d lines, want 2", len(got))
	}

	// Missing file returns nil
	if got := ReadJSONLFile(filepath.Join(tmp, "missing.jsonl")); got != nil {
		t.Errorf("missing file should return nil, got %v", got)
	}
}

func TestComputeVelocityTrend_NoMetrics(t *testing.T) {
	tmp := t.TempDir()
	t7, t30, verdict, err := ComputeVelocityTrend(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if t7 != 0 || t30 != 0 {
		t.Errorf("should be zero, got %v %v", t7, t30)
	}
	if verdict != "stagnant" {
		t.Errorf("got %q", verdict)
	}
}

func TestComputeReuseConcentration_NoLearnings(t *testing.T) {
	tmp := t.TempDir()
	gini, pct, tb, verdict := ComputeReuseConcentration(tmp, 30)
	if gini != 0 || pct != 0 || tb != 0 {
		t.Errorf("all zeros expected, got %v %v %v", gini, pct, tb)
	}
	if verdict != "dormant" {
		t.Errorf("got %q", verdict)
	}
}

// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestLinearRegressionSlope_Positive(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{2, 4, 6, 8, 10}
	slope := linearRegressionSlope(xs, ys)
	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("expected slope ~2.0, got %f", slope)
	}
}

func TestLinearRegressionSlope_Negative(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{10, 8, 6, 4, 2}
	slope := linearRegressionSlope(xs, ys)
	if math.Abs(slope-(-2.0)) > 0.001 {
		t.Errorf("expected slope ~-2.0, got %f", slope)
	}
}

func TestLinearRegressionSlope_Flat(t *testing.T) {
	xs := []float64{1, 2, 3}
	ys := []float64{5, 5, 5}
	slope := linearRegressionSlope(xs, ys)
	if math.Abs(slope) > 0.001 {
		t.Errorf("expected slope ~0, got %f", slope)
	}
}

func TestLinearRegressionSlope_InsufficientPoints(t *testing.T) {
	slope := linearRegressionSlope([]float64{1}, []float64{2})
	if slope != 0 {
		t.Errorf("expected 0 for single point, got %f", slope)
	}
}

func TestGiniCoefficient_Equal(t *testing.T) {
	values := []float64{10, 10, 10, 10, 10}
	g := giniCoefficient(values)
	if math.Abs(g) > 0.001 {
		t.Errorf("expected Gini ~0 for equal values, got %f", g)
	}
}

func TestGiniCoefficient_MaxInequality(t *testing.T) {
	// One person has everything
	values := []float64{0, 0, 0, 0, 100}
	g := giniCoefficient(values)
	if g < 0.7 {
		t.Errorf("expected high Gini for max inequality, got %f", g)
	}
}

func TestGiniCoefficient_Moderate(t *testing.T) {
	values := []float64{1, 2, 3, 4, 10}
	g := giniCoefficient(values)
	if g < 0.1 || g > 0.6 {
		t.Errorf("expected moderate Gini, got %f", g)
	}
}

func TestGiniCoefficient_Empty(t *testing.T) {
	g := giniCoefficient(nil)
	if g != 0 {
		t.Errorf("expected 0 for empty, got %f", g)
	}
}

func TestGiniCoefficient_AllZero(t *testing.T) {
	g := giniCoefficient([]float64{0, 0, 0})
	if g != 0 {
		t.Errorf("expected 0 for all zeros, got %f", g)
	}
}

func TestComputeVelocityTrend_Compounding(t *testing.T) {
	dir := t.TempDir()
	metricsDir := filepath.Join(dir, ".agents", "ao", "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write 5 baselines with increasing velocity
	for i := 0; i < 5; i++ {
		ts := time.Now().AddDate(0, 0, -i)
		m := types.FlywheelMetrics{
			Timestamp: ts,
			Velocity:  0.1 + float64(i)*0.05, // older = higher velocity when sorted by dayOffset
		}
		// Wait: dayOffset increases with i, so older baselines have higher velocity
		// But we want velocity INCREASING over time (recent > old)
		// dayOffset 0 = most recent. We need recent baselines to have higher velocity.
		m.Velocity = 0.3 - float64(i)*0.05 // day0=0.30, day1=0.25, day2=0.20, day3=0.15, day4=0.10
		data, _ := json.Marshal(m)
		filename := "baseline-" + ts.Format("2006-01-02") + ".json"
		os.WriteFile(filepath.Join(metricsDir, filename), data, 0644)
	}

	_, trend30d, verdict, err := computeVelocityTrend(dir)
	if err != nil {
		t.Fatal(err)
	}
	if trend30d <= 0 {
		t.Errorf("expected positive 30d trend for compounding, got %f", trend30d)
	}
	if verdict != "compounding" {
		t.Errorf("expected 'compounding' verdict, got %q", verdict)
	}
}

func TestComputeVelocityTrend_Decaying(t *testing.T) {
	dir := t.TempDir()
	metricsDir := filepath.Join(dir, ".agents", "ao", "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		ts := time.Now().AddDate(0, 0, -i)
		m := types.FlywheelMetrics{
			Timestamp: ts,
			Velocity:  0.1 + float64(i)*0.05, // recent=low, old=high → decaying
		}
		data, _ := json.Marshal(m)
		filename := "baseline-" + ts.Format("2006-01-02") + ".json"
		os.WriteFile(filepath.Join(metricsDir, filename), data, 0644)
	}

	_, trend30d, verdict, err := computeVelocityTrend(dir)
	if err != nil {
		t.Fatal(err)
	}
	if trend30d >= 0 {
		t.Errorf("expected negative 30d trend for decaying, got %f", trend30d)
	}
	if verdict != "decaying" {
		t.Errorf("expected 'decaying' verdict, got %q", verdict)
	}
}

func TestComputeVelocityTrend_InsufficientBaselines(t *testing.T) {
	dir := t.TempDir()
	metricsDir := filepath.Join(dir, ".agents", "ao", "metrics")
	os.MkdirAll(metricsDir, 0755)

	// Only 2 baselines — insufficient
	for i := 0; i < 2; i++ {
		ts := time.Now().AddDate(0, 0, -i)
		m := types.FlywheelMetrics{Timestamp: ts, Velocity: 0.1}
		data, _ := json.Marshal(m)
		os.WriteFile(filepath.Join(metricsDir, "baseline-"+ts.Format("2006-01-02")+".json"), data, 0644)
	}

	_, _, verdict, _ := computeVelocityTrend(dir)
	if verdict != "stagnant" {
		t.Errorf("expected 'stagnant' for insufficient baselines, got %q", verdict)
	}
}

func TestComputeCitationPipeline_Reinforcing(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	os.MkdirAll(aoDir, 0755)

	// Write citations
	citFile, _ := os.Create(filepath.Join(aoDir, "citations.jsonl"))
	now := time.Now()
	for i := 0; i < 10; i++ {
		ct := "retrieved"
		if i < 3 {
			ct = "applied"
		}
		c := map[string]interface{}{
			"cited_at":      now.Add(-time.Duration(i) * time.Hour),
			"citation_type": ct,
			"artifact_path": "/test/learning.md",
		}
		data, _ := json.Marshal(c)
		citFile.Write(data)
		citFile.WriteString("\n")
	}
	citFile.Close()

	// Write feedback with >60% high-utility
	fbFile, _ := os.Create(filepath.Join(aoDir, "feedback.jsonl"))
	for i := 0; i < 10; i++ {
		reward := 0.8 // 70% high utility
		if i >= 7 {
			reward = 0.3
		}
		f := map[string]interface{}{
			"recorded_at":    now.Add(-time.Duration(i) * time.Hour),
			"reward":         reward,
			"utility_before": 0.5,
			"utility_after":  0.5 + reward*0.1,
		}
		data, _ := json.Marshal(f)
		fbFile.Write(data)
		fbFile.WriteString("\n")
	}
	fbFile.Close()

	highPct, _, _, verdict := computeCitationPipeline(dir, 7)
	if highPct <= 60 {
		t.Errorf("expected >60%% high-utility, got %.1f%%", highPct)
	}
	if verdict != "reinforcing" {
		t.Errorf("expected 'reinforcing', got %q", verdict)
	}
}

func TestComputeCitationPipeline_Degrading(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	os.MkdirAll(aoDir, 0755)

	// Empty citations
	os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte{}, 0644)

	// Write feedback with <30% high-utility
	fbFile, _ := os.Create(filepath.Join(aoDir, "feedback.jsonl"))
	now := time.Now()
	for i := 0; i < 10; i++ {
		reward := 0.2 // all low
		f := map[string]interface{}{
			"recorded_at":    now.Add(-time.Duration(i) * time.Hour),
			"reward":         reward,
			"utility_before": 0.5,
			"utility_after":  0.48,
		}
		data, _ := json.Marshal(f)
		fbFile.Write(data)
		fbFile.WriteString("\n")
	}
	fbFile.Close()

	_, _, _, verdict := computeCitationPipeline(dir, 7)
	if verdict != "degrading" {
		t.Errorf("expected 'degrading', got %q", verdict)
	}
}

func TestComputeCitationPipeline_InsufficientData(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	os.MkdirAll(aoDir, 0755)

	// Empty files — no feedback data at all
	os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte{}, 0644)
	os.WriteFile(filepath.Join(aoDir, "feedback.jsonl"), []byte{}, 0644)

	_, _, _, verdict := computeCitationPipeline(dir, 7)
	if verdict != "insufficient-data" {
		t.Errorf("expected 'insufficient-data' for no feedback, got %q", verdict)
	}
}

func TestComputeResearchClosure_Mining(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	os.MkdirAll(researchDir, 0755)
	os.MkdirAll(learningsDir, 0755)

	// 10 research files, 9 have matching learnings (source backlink)
	for i := 0; i < 10; i++ {
		rName := "2026-03-01-topic-" + string(rune('a'+i)) + ".md"
		os.WriteFile(filepath.Join(researchDir, rName), []byte("# Research"), 0644)

		if i < 9 { // 9 out of 10 are closed
			lContent := "---\nsource: \"[[.agents/research/" + rName + "]]\"\n---\n# Learning"
			lName := "2026-03-02-learning-" + string(rune('a'+i)) + ".md"
			os.WriteFile(filepath.Join(learningsDir, lName), []byte(lContent), 0644)
		}
	}

	orphanCount, orphanPct, _, verdict := computeResearchClosure(dir)
	if orphanCount != 1 {
		t.Errorf("expected 1 orphan, got %d", orphanCount)
	}
	if orphanPct > 10 {
		t.Errorf("expected <=10%% orphan rate, got %.1f%%", orphanPct)
	}
	if verdict != "mining" {
		t.Errorf("expected 'mining', got %q", verdict)
	}
}

func TestComputeResearchClosure_Hoarding(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	os.MkdirAll(researchDir, 0755)
	os.MkdirAll(learningsDir, 0755)

	// 10 research files, only 2 have learnings
	for i := 0; i < 10; i++ {
		rName := "2026-03-01-topic-" + string(rune('a'+i)) + ".md"
		os.WriteFile(filepath.Join(researchDir, rName), []byte("# Research"), 0644)

		if i < 2 {
			lContent := "---\nsource: \"[[.agents/research/" + rName + "]]\"\n---\n# Learning"
			os.WriteFile(filepath.Join(learningsDir, "learning-"+string(rune('a'+i))+".md"), []byte(lContent), 0644)
		}
	}

	_, orphanPct, _, verdict := computeResearchClosure(dir)
	if orphanPct < 50 {
		t.Errorf("expected >=50%% orphan rate, got %.1f%%", orphanPct)
	}
	if verdict != "unmined" {
		t.Errorf("expected 'unmined', got %q", verdict)
	}
}

func TestComputeReuseConcentration_Distributed(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	os.MkdirAll(aoDir, 0755)
	os.MkdirAll(learningsDir, 0755)

	now := time.Now()

	// Create 10 learnings, each cited roughly equally
	for i := 0; i < 10; i++ {
		name := "2026-03-01-learning-" + string(rune('a'+i)) + ".md"
		os.WriteFile(filepath.Join(learningsDir, name), []byte("# L"), 0644)
	}

	// Write citations: each learning cited 3 times
	citFile, _ := os.Create(filepath.Join(aoDir, "citations.jsonl"))
	for i := 0; i < 10; i++ {
		name := "2026-03-01-learning-" + string(rune('a'+i)) + ".md"
		for j := 0; j < 3; j++ {
			c := map[string]interface{}{
				"artifact_path": "/test/.agents/learnings/" + name,
				"cited_at":      now.Add(-time.Duration(j) * time.Hour),
			}
			data, _ := json.Marshal(c)
			citFile.Write(data)
			citFile.WriteString("\n")
		}
	}
	citFile.Close()

	gini, activePct, _, verdict := computeReuseConcentration(dir, 7)
	if gini > 0.1 {
		t.Errorf("expected low Gini for equal distribution, got %f", gini)
	}
	if activePct < 90 {
		t.Errorf("expected >90%% active pool, got %.1f%%", activePct)
	}
	if verdict != "distributed" {
		t.Errorf("expected 'distributed', got %q", verdict)
	}
}

func TestComputeReuseConcentration_Concentrated(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	os.MkdirAll(aoDir, 0755)
	os.MkdirAll(learningsDir, 0755)

	now := time.Now()

	// Create 20 learnings
	for i := 0; i < 20; i++ {
		name := "2026-03-01-learning-" + string(rune('a'+i)) + ".md"
		os.WriteFile(filepath.Join(learningsDir, name), []byte("# L"), 0644)
	}

	// Only 2 learnings get all the citations
	citFile, _ := os.Create(filepath.Join(aoDir, "citations.jsonl"))
	for _, name := range []string{"2026-03-01-learning-a.md", "2026-03-01-learning-b.md"} {
		for j := 0; j < 50; j++ {
			c := map[string]interface{}{
				"artifact_path": "/test/.agents/learnings/" + name,
				"cited_at":      now.Add(-time.Duration(j) * time.Minute),
			}
			data, _ := json.Marshal(c)
			citFile.Write(data)
			citFile.WriteString("\n")
		}
	}
	citFile.Close()

	_, activePct, _, verdict := computeReuseConcentration(dir, 7)
	if activePct > 15 {
		t.Errorf("expected low active pool %%, got %.1f%%", activePct)
	}
	// 2 of 20 cited = 10% active pool. With Gini fix (includes zero-citation
	// learnings), Gini is high (18 zeros + 2 equal values) → "dormant"
	if verdict != "dormant" {
		t.Errorf("expected 'dormant', got %q", verdict)
	}
}

func TestOverallVerdict_Compounding(t *testing.T) {
	gs := &types.GoldenSignals{
		TrendVerdict:         "compounding",
		PipelineVerdict:      "reinforcing",
		ClosureVerdict:       "mining",
		ConcentrationVerdict: "distributed",
	}
	verdict := computeOverallVerdict(gs)
	if verdict != "compounding" {
		t.Errorf("expected 'compounding', got %q", verdict)
	}
}

func TestOverallVerdict_Decaying(t *testing.T) {
	gs := &types.GoldenSignals{
		TrendVerdict:         "decaying",
		PipelineVerdict:      "degrading",
		ClosureVerdict:       "hoarding",
		ConcentrationVerdict: "dormant",
	}
	verdict := computeOverallVerdict(gs)
	if verdict != "decaying" {
		t.Errorf("expected 'decaying', got %q", verdict)
	}
}

func TestOverallVerdict_Accumulating(t *testing.T) {
	gs := &types.GoldenSignals{
		TrendVerdict:         "compounding",
		PipelineVerdict:      "degrading",
		ClosureVerdict:       "mining",
		ConcentrationVerdict: "dormant",
	}
	verdict := computeOverallVerdict(gs)
	if verdict != "accumulating" {
		t.Errorf("expected 'accumulating', got %q", verdict)
	}
}

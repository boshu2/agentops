// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestSaveBaseline_CreatesFile(t *testing.T) {
	baseDir := t.TempDir()

	now := time.Now()
	metrics := &types.FlywheelMetrics{
		Timestamp:   now,
		PeriodStart: now.AddDate(0, 0, -7),
		PeriodEnd:   now,
		Delta:       types.DefaultDelta,
		Sigma:       0.5,
		Rho:         1.0,
		SigmaRho:    0.5,
		Velocity:    0.33,
		TierCounts:  map[string]int{"learning": 3},
	}

	path, err := saveBaseline(baseDir, metrics)
	if err != nil {
		t.Fatalf("saveBaseline failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("baseline file not created at %s", path)
	}

	// Verify path contains expected directory structure
	expectedDir := filepath.Join(baseDir, ".agents", "ao", "metrics")
	if dir := filepath.Dir(path); dir != expectedDir {
		t.Errorf("baseline dir = %q, want %q", dir, expectedDir)
	}

	// Verify filename format
	expectedFilename := "baseline-" + now.Format("2006-01-02") + ".json"
	if got := filepath.Base(path); got != expectedFilename {
		t.Errorf("baseline filename = %q, want %q", got, expectedFilename)
	}
}

func TestSaveBaseline_ValidJSON(t *testing.T) {
	baseDir := t.TempDir()

	now := time.Now()
	metrics := &types.FlywheelMetrics{
		Timestamp:           now,
		PeriodStart:         now.AddDate(0, 0, -7),
		PeriodEnd:           now,
		Delta:               types.DefaultDelta,
		Sigma:               0.75,
		Rho:                 2.0,
		SigmaRho:            1.5,
		Velocity:            1.33,
		AboveEscapeVelocity: true,
		TotalArtifacts:      10,
		TierCounts:          map[string]int{"learning": 5, "pattern": 3},
	}

	path, err := saveBaseline(baseDir, metrics)
	if err != nil {
		t.Fatalf("saveBaseline failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}

	var parsed types.FlywheelMetrics
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("baseline JSON parse failed: %v", err)
	}

	if parsed.Sigma != 0.75 {
		t.Errorf("parsed sigma = %f, want 0.75", parsed.Sigma)
	}
	if parsed.Rho != 2.0 {
		t.Errorf("parsed rho = %f, want 2.0", parsed.Rho)
	}
	if !parsed.AboveEscapeVelocity {
		t.Error("expected AboveEscapeVelocity = true")
	}
}

func TestSaveBaseline_FilePermissions(t *testing.T) {
	baseDir := t.TempDir()

	metrics := &types.FlywheelMetrics{
		Timestamp:  time.Now(),
		TierCounts: map[string]int{},
	}

	path, err := saveBaseline(baseDir, metrics)
	if err != nil {
		t.Fatalf("saveBaseline failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat baseline: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("baseline file mode = %o, want 600", got)
	}
}

func TestSaveBaseline_CreatesDirectories(t *testing.T) {
	baseDir := t.TempDir()
	metricsDir := filepath.Join(baseDir, ".agents", "ao", "metrics")

	// Directory should not exist yet
	if _, err := os.Stat(metricsDir); !os.IsNotExist(err) {
		t.Fatal("expected metrics dir to not exist yet")
	}

	metrics := &types.FlywheelMetrics{
		Timestamp:  time.Now(),
		TierCounts: map[string]int{},
	}

	if _, err := saveBaseline(baseDir, metrics); err != nil {
		t.Fatalf("saveBaseline failed: %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(metricsDir)
	if err != nil {
		t.Fatalf("metrics dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("metrics dir should be a directory")
	}
}

func TestRunMetricsBaseline_DryRun(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := runMetricsBaseline(nil, nil); err != nil {
		t.Fatalf("runMetricsBaseline dry-run: %v", err)
	}

	// Verify no baseline file was created
	metricsDir := filepath.Join(dir, ".agents", "ao", "metrics")
	if _, err := os.Stat(metricsDir); !os.IsNotExist(err) {
		t.Error("expected no metrics directory in dry-run mode")
	}
}

func TestRunMetricsBaseline_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldDays := metricsDays
	metricsDays = 7
	defer func() { metricsDays = oldDays }()

	if err := runMetricsBaseline(nil, nil); err != nil {
		t.Fatalf("runMetricsBaseline failed on empty dir: %v", err)
	}

	// Verify baseline file was created
	metricsDir := filepath.Join(dir, ".agents", "ao", "metrics")
	files, err := filepath.Glob(filepath.Join(metricsDir, "baseline-*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 baseline file, got %d", len(files))
	}
}

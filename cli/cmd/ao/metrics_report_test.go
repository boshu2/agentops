// practices: [dora-metrics, sre]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunMetricsReport_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldDays := metricsDays
	metricsDays = 7
	defer func() { metricsDays = oldDays }()

	// Should not error on empty directory
	if err := runMetricsReport(nil, nil); err != nil {
		t.Fatalf("runMetricsReport failed on empty dir: %v", err)
	}
}

func TestRunMetricsReport_WithArtifacts(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "test.md"), []byte("# Learning"), 0o644); err != nil {
		t.Fatal(err)
	}

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldDays := metricsDays
	metricsDays = 7
	defer func() { metricsDays = oldDays }()

	if err := runMetricsReport(nil, nil); err != nil {
		t.Fatalf("runMetricsReport failed: %v", err)
	}
}

func TestRunMetricsReport_CustomDays(t *testing.T) {
	dir := t.TempDir()

	testProjectDir = dir
	defer func() { testProjectDir = "" }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	for _, days := range []int{1, 7, 14, 30} {
		t.Run("days="+string(rune('0'+days)), func(t *testing.T) {
			oldDays := metricsDays
			metricsDays = days
			defer func() { metricsDays = oldDays }()

			if err := runMetricsReport(nil, nil); err != nil {
				t.Fatalf("runMetricsReport with %d days: %v", days, err)
			}
		})
	}
}

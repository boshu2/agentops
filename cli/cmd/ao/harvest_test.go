package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/harvest"
)

func TestHarvestCmd_Flags(t *testing.T) {
	flags := harvestCmd.Flags()

	tests := []struct {
		name     string
		flagName string
	}{
		{"roots flag exists", "roots"},
		{"output-dir flag exists", "output-dir"},
		{"promote-to flag exists", "promote-to"},
		{"min-confidence flag exists", "min-confidence"},
		{"include flag exists", "include"},
		{"quiet flag exists", "quiet"},
		{"max-file-size flag exists", "max-file-size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flags.Lookup(tt.flagName)
			if f == nil {
				t.Fatalf("flag %q not found on harvestCmd", tt.flagName)
			}
		})
	}

	// Check specific defaults.
	if f := flags.Lookup("output-dir"); f.DefValue != ".agents/harvest" {
		t.Errorf("output-dir default = %q, want %q", f.DefValue, ".agents/harvest")
	}
	if f := flags.Lookup("min-confidence"); f.DefValue != "0.5" {
		t.Errorf("min-confidence default = %q, want %q", f.DefValue, "0.5")
	}
	if f := flags.Lookup("include"); f.DefValue != "learnings,patterns,research" {
		t.Errorf("include default = %q, want %q", f.DefValue, "learnings,patterns,research")
	}
	if f := flags.Lookup("max-file-size"); f.DefValue != "1048576" {
		t.Errorf("max-file-size default = %q, want %q", f.DefValue, "1048576")
	}
	if f := flags.Lookup("quiet"); f.DefValue != "false" {
		t.Errorf("quiet default = %q, want %q", f.DefValue, "false")
	}

	// Verify roots and promote-to resolve to absolute paths (no tilde).
	home, _ := os.UserHomeDir()
	if f := flags.Lookup("roots"); f.DefValue != filepath.Join(home, "gt") {
		t.Errorf("roots default = %q, want %q", f.DefValue, filepath.Join(home, "gt"))
	}
	if f := flags.Lookup("promote-to"); f.DefValue != filepath.Join(home, ".agents", "learnings") {
		t.Errorf("promote-to default = %q, want %q", f.DefValue, filepath.Join(home, ".agents", "learnings"))
	}
}

func TestRunHarvest_DryRun(t *testing.T) {
	tmp := t.TempDir()

	// Create a rig with .agents/learnings containing a markdown file.
	rigDir := filepath.Join(tmp, "myproject", ".agents", "learnings")
	if err := os.MkdirAll(rigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: Test Learning\nconfidence: 0.8\n---\n\n# Test Learning\n\nSome content here.\n"
	if err := os.WriteFile(filepath.Join(rigDir, "2026-03-29-test.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set up output directory inside tmp.
	outputDir := filepath.Join(tmp, "harvest-output")
	promoteDir := filepath.Join(tmp, "promoted")

	// Save and restore global state.
	origRoots := harvestRootsFlag
	origOutput := harvestOutputDir
	origPromote := harvestPromoteTo
	origQuiet := harvestQuiet
	origDryRun := dryRun
	origMinConf := harvestMinConfidence
	origInclude := harvestInclude
	origMaxSize := harvestMaxFileSize
	t.Cleanup(func() {
		harvestRootsFlag = origRoots
		harvestOutputDir = origOutput
		harvestPromoteTo = origPromote
		harvestQuiet = origQuiet
		dryRun = origDryRun
		harvestMinConfidence = origMinConf
		harvestInclude = origInclude
		harvestMaxFileSize = origMaxSize
	})

	harvestRootsFlag = tmp
	harvestOutputDir = outputDir
	harvestPromoteTo = promoteDir
	harvestQuiet = true
	dryRun = true
	harvestMinConfidence = 0.5
	harvestInclude = "learnings,patterns,research"
	harvestMaxFileSize = 1048576

	if err := runHarvest(harvestCmd, nil); err != nil {
		t.Fatalf("runHarvest returned error: %v", err)
	}

	// Catalog should be written.
	latestPath := filepath.Join(outputDir, "latest.json")
	if _, err := os.Stat(latestPath); os.IsNotExist(err) {
		t.Fatalf("expected catalog at %s, not found", latestPath)
	}

	// Read and verify catalog contains artifacts.
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}
	var cat harvest.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("unmarshaling catalog: %v", err)
	}
	if len(cat.Artifacts) == 0 {
		t.Error("expected at least one artifact in catalog")
	}
	if cat.RigsScanned == 0 {
		t.Error("expected RigsScanned > 0")
	}

	// Promotion directory should NOT exist (dry run).
	if _, err := os.Stat(promoteDir); !os.IsNotExist(err) {
		t.Errorf("promote dir %s should not exist in dry-run mode", promoteDir)
	}
}

func TestRunHarvest_JSONOutput(t *testing.T) {
	tmp := t.TempDir()

	// Create a rig with a learning.
	rigDir := filepath.Join(tmp, "proj", ".agents", "learnings")
	if err := os.MkdirAll(rigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: JSON Test\nconfidence: 0.9\n---\n\n# JSON Test\n\nContent.\n"
	if err := os.WriteFile(filepath.Join(rigDir, "2026-03-29-json.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save and restore global state.
	origRoots := harvestRootsFlag
	origOutput := output
	origQuiet := harvestQuiet
	origDryRun := dryRun
	origMinConf := harvestMinConfidence
	origInclude := harvestInclude
	origMaxSize := harvestMaxFileSize
	origOutputDir := harvestOutputDir
	origPromote := harvestPromoteTo
	t.Cleanup(func() {
		harvestRootsFlag = origRoots
		output = origOutput
		harvestQuiet = origQuiet
		dryRun = origDryRun
		harvestMinConfidence = origMinConf
		harvestInclude = origInclude
		harvestMaxFileSize = origMaxSize
		harvestOutputDir = origOutputDir
		harvestPromoteTo = origPromote
	})

	harvestRootsFlag = tmp
	output = "json"
	harvestQuiet = true
	dryRun = true
	harvestMinConfidence = 0.5
	harvestInclude = "learnings,patterns,research"
	harvestMaxFileSize = 1048576
	harvestOutputDir = filepath.Join(tmp, "out")
	harvestPromoteTo = filepath.Join(tmp, "promoted")

	// Capture stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runHarvest(harvestCmd, nil)

	w.Close()
	os.Stdout = origStdout

	captured, _ := io.ReadAll(r)

	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}

	// Verify valid JSON.
	var cat harvest.Catalog
	if err := json.Unmarshal(captured, &cat); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot: %s", err, string(captured))
	}

	if len(cat.Artifacts) == 0 {
		t.Error("expected at least one artifact in JSON output")
	}

	if cat.RigsScanned == 0 {
		t.Error("expected RigsScanned > 0 in JSON output")
	}
}

// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestHarvestCmd_MaxPromotionsFlag_Wired guards that the --max-promotions
// flag is registered on `ao harvest`. soc-f2q4 wires the flag and the
// AO_MAX_PROMOTIONS env-var fallback into runHarvest's volume-gate site.
func TestHarvestCmd_MaxPromotionsFlag_Wired(t *testing.T) {
	flags := harvestCmd.Flags()
	f := flags.Lookup("max-promotions")
	if f == nil {
		t.Fatal("flag --max-promotions not found on harvestCmd")
	}
	if f.DefValue != "500" {
		t.Errorf("max-promotions default = %q, want %q", f.DefValue, "500")
	}
}

// TestHarvestVolumeGate_BelowThreshold_Silent runs `ao harvest` with a
// promotion count under the threshold and asserts no WARN line is written
// to stderr.
func TestHarvestVolumeGate_BelowThreshold_Silent(t *testing.T) {
	tmp := setupHarvestVolumeGateFixture(t, 3) // 3 distinct artifacts, threshold 500
	stderr, runErr := runHarvestCaptureStderr(t, tmp, harvestVolumeGateOverrides{
		maxPromotions: 500,
	})
	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}
	if strings.Contains(stderr, "exceeded threshold") {
		t.Fatalf("expected no WARN line below threshold, stderr=%q", stderr)
	}
}

// TestHarvestVolumeGate_AboveThreshold_WarnsButContinues runs `ao harvest`
// with a promotion count above the threshold and asserts:
//   - the WARN line shows up on stderr with the configured threshold,
//   - runHarvest returns nil (exit code unchanged: gate is advisory),
//   - all source files were promoted (gate does not block writes).
func TestHarvestVolumeGate_AboveThreshold_WarnsButContinues(t *testing.T) {
	const promotionCount = 6
	const threshold = 3
	tmp := setupHarvestVolumeGateFixture(t, promotionCount)

	stderr, runErr := runHarvestCaptureStderr(t, tmp, harvestVolumeGateOverrides{
		maxPromotions:    threshold,
		maxPromotionsSet: true,
	})
	if runErr != nil {
		t.Fatalf("runHarvest returned error (gate must NOT block): %v", runErr)
	}
	if !strings.Contains(stderr, "exceeded threshold 3") {
		t.Fatalf("expected WARN with threshold=3 in stderr, got=%q", stderr)
	}
	if !strings.Contains(stderr, "WARN:") {
		t.Fatalf("expected WARN: prefix in stderr, got=%q", stderr)
	}

	// All artifacts must still land on disk — gate is advisory, not a block.
	promoteDir := filepath.Join(tmp, "promoted", "learning")
	entries, err := os.ReadDir(promoteDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", promoteDir, err)
	}
	mdCount := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			mdCount++
		}
	}
	if mdCount != promotionCount {
		t.Fatalf("promoted %d files, want %d (gate is advisory; must not block writes)", mdCount, promotionCount)
	}
}

// TestHarvestVolumeGate_FlagOverride verifies the --max-promotions flag is
// honored end-to-end. Threshold=2 with 4 promotions → WARN.
func TestHarvestVolumeGate_FlagOverride(t *testing.T) {
	tmp := setupHarvestVolumeGateFixture(t, 4)
	stderr, runErr := runHarvestCaptureStderr(t, tmp, harvestVolumeGateOverrides{
		maxPromotions:    2,
		maxPromotionsSet: true,
	})
	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}
	if !strings.Contains(stderr, "exceeded threshold 2") {
		t.Fatalf("expected --max-promotions=2 to drive WARN, got stderr=%q", stderr)
	}
}

// TestHarvestVolumeGate_EnvOverride verifies AO_MAX_PROMOTIONS=N is honored
// when the flag is not set explicitly. Sets env=2, leaves flag at its
// default (500), and asserts the env-derived threshold drives the WARN.
func TestHarvestVolumeGate_EnvOverride(t *testing.T) {
	tmp := setupHarvestVolumeGateFixture(t, 4)
	t.Setenv("AO_MAX_PROMOTIONS", "2")
	stderr, runErr := runHarvestCaptureStderr(t, tmp, harvestVolumeGateOverrides{
		maxPromotions: 500, // default; env should win because flag is not Changed
	})
	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}
	if !strings.Contains(stderr, "exceeded threshold 2") {
		t.Fatalf("expected AO_MAX_PROMOTIONS=2 to drive WARN, got stderr=%q", stderr)
	}
}

// TestHarvestVolumeGate_FlagWinsOverEnv verifies that when both the flag
// and env-var are set, the flag wins. Env=10 (would suppress with 4
// promotions), flag=2 (would warn with 4 promotions) → WARN expected.
func TestHarvestVolumeGate_FlagWinsOverEnv(t *testing.T) {
	tmp := setupHarvestVolumeGateFixture(t, 4)
	t.Setenv("AO_MAX_PROMOTIONS", "10")
	stderr, runErr := runHarvestCaptureStderr(t, tmp, harvestVolumeGateOverrides{
		maxPromotions:    2,
		maxPromotionsSet: true,
	})
	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}
	if !strings.Contains(stderr, "exceeded threshold 2") {
		t.Fatalf("expected flag=2 to win over env=10, got stderr=%q", stderr)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type harvestVolumeGateOverrides struct {
	maxPromotions    int
	maxPromotionsSet bool // when true, mark the flag as Changed so it wins over env
}

// setupHarvestVolumeGateFixture creates a single rig with `count` distinct
// learning files, all above the default min-confidence threshold so they
// land in catalog.Promoted. Returns the tmp HOME root.
func setupHarvestVolumeGateFixture(t *testing.T, count int) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	rigDir := filepath.Join(tmp, "proj", ".agents", "learnings")
	if err := os.MkdirAll(rigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Distinct content per file so BuildCatalog does not dedup them.
	for i := 0; i < count; i++ {
		idx := strconv.Itoa(i)
		body := "---\ntitle: Distinct " + idx + "\nconfidence: 0.9\n---\n\n# Distinct " + idx + "\n\nUnique body number " + idx + ".\n"
		path := filepath.Join(rigDir, "2026-04-30-d"+idx+".md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return tmp
}

// runHarvestCaptureStderr drives runHarvest end-to-end against the fixture
// rooted at homeDir, captures stderr, and returns it along with any runErr.
// Flags are saved/restored automatically; max-promotions is set per overrides.
func runHarvestCaptureStderr(t *testing.T, homeDir string, ov harvestVolumeGateOverrides) (string, error) {
	t.Helper()

	origRoots := harvestRootsFlag
	origOutput := harvestOutputDir
	origPromote := harvestPromoteTo
	origQuiet := harvestQuiet
	origDryRun := dryRun
	origMinConf := harvestMinConfidence
	origInclude := harvestInclude
	origMaxSize := harvestMaxFileSize
	origMaxPromotions := harvestMaxPromotions
	t.Cleanup(func() {
		harvestRootsFlag = origRoots
		harvestOutputDir = origOutput
		harvestPromoteTo = origPromote
		harvestQuiet = origQuiet
		dryRun = origDryRun
		harvestMinConfidence = origMinConf
		harvestInclude = origInclude
		harvestMaxFileSize = origMaxSize
		harvestMaxPromotions = origMaxPromotions
		// Reset Changed state on the flag so subsequent tests start clean.
		if f := harvestCmd.Flags().Lookup("max-promotions"); f != nil {
			f.Changed = false
		}
	})

	harvestRootsFlag = homeDir
	harvestOutputDir = filepath.Join(homeDir, "out")
	harvestPromoteTo = filepath.Join(homeDir, "promoted")
	harvestQuiet = true
	dryRun = false
	harvestMinConfidence = 0.5
	harvestInclude = "learnings"
	harvestMaxFileSize = 1048576
	harvestMaxPromotions = ov.maxPromotions
	if f := harvestCmd.Flags().Lookup("max-promotions"); f != nil {
		f.Changed = ov.maxPromotionsSet
	}

	// Capture stderr.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	runErr := runHarvest(harvestCmd, nil)

	w.Close()
	os.Stderr = origStderr
	captured, _ := io.ReadAll(r)
	return string(captured), runErr
}

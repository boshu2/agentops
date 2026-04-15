package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

	// Verify roots and promote-to use empty defaults (resolved at runtime to avoid
	// embedding absolute home paths in generated docs).
	if f := flags.Lookup("roots"); f.DefValue != "" {
		t.Errorf("roots default = %q, want empty (resolved at runtime)", f.DefValue)
	}
	if f := flags.Lookup("promote-to"); f.DefValue != "" {
		t.Errorf("promote-to default = %q, want empty (resolved at runtime)", f.DefValue)
	}
}

func TestHarvestCSVListTrimsEntries(t *testing.T) {
	got := harvestCSVList(" learnings, patterns ,research ")
	want := []string{"learnings", "patterns", "research"}
	if len(got) != len(want) {
		t.Fatalf("harvestCSVList length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("harvestCSVList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunHarvest_DryRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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
	if got := cat.Summary.ArtifactsExtracted; got != len(cat.Artifacts) {
		t.Errorf("summary.artifacts_extracted = %d, want %d", got, len(cat.Artifacts))
	}
	if got := cat.Summary.PromotionCandidates; got != len(cat.Promoted) {
		t.Errorf("summary.promotion_candidates = %d, want %d", got, len(cat.Promoted))
	}
	if got := cat.Summary.WarningCount; got != len(cat.Warnings) {
		t.Errorf("summary.warning_count = %d, want %d", got, len(cat.Warnings))
	}
	if got := cat.TotalFiles; got != 1 {
		t.Errorf("total_files = %d, want 1 candidate file", got)
	}
	if !cat.DryRun {
		t.Error("expected dry_run=true in catalog")
	}

	// Promotion directory should NOT exist (dry run).
	if _, err := os.Stat(promoteDir); !os.IsNotExist(err) {
		t.Errorf("promote dir %s should not exist in dry-run mode", promoteDir)
	}
}

func TestRunHarvest_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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
	latestPath := filepath.Join(harvestOutputDir, "latest.json")
	if _, err := os.Stat(latestPath); err != nil {
		t.Fatalf("expected latest.json to be written in JSON mode: %v", err)
	}
}

func TestRunHarvest_JSONOutputPreservesSideEffects(t *testing.T) {
	fixture := setupHarvestJSONSideEffectsFixture(t)
	configureHarvestJSONSideEffectsFlags(t, fixture)
	captured := runHarvestWithCapturedStdout(t)
	cat := parseHarvestJSONCatalog(t, captured)

	assertHarvestJSONSideEffectCatalog(t, cat)
	assertHarvestJSONSideEffectPersistence(t, fixture)
	assertHarvestJSONSideEffectPromotion(t, fixture, cat)
}

type harvestJSONSideEffectsFixture struct {
	tmp        string
	sourceName string
	outputDir  string
	promoteTo  string
}

func setupHarvestJSONSideEffectsFixture(t *testing.T) harvestJSONSideEffectsFixture {
	t.Helper()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	rigDir := filepath.Join(tmp, "proj", ".agents", "learnings")
	if err := os.MkdirAll(rigDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sourceName := "2026-04-10-json-promote.md"
	valid := "---\ntitle: JSON Promote\nconfidence: 0.9\nmaturity: provisional\nutility: 0.8\n---\n\n# JSON Promote\n\nContent.\n"
	if err := os.WriteFile(filepath.Join(rigDir, sourceName), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := "---\nkey:\n\tvalue_with_tab: broken\n---\n"
	if err := os.WriteFile(filepath.Join(rigDir, "2026-04-10-bad.md"), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}

	return harvestJSONSideEffectsFixture{
		tmp:        tmp,
		sourceName: sourceName,
		outputDir:  filepath.Join(tmp, "out"),
		promoteTo:  filepath.Join(tmp, "promoted"),
	}
}

func configureHarvestJSONSideEffectsFlags(t *testing.T, fixture harvestJSONSideEffectsFixture) {
	t.Helper()

	origRoots := harvestRootsFlag
	origOutput := output
	origJsonFlag := jsonFlag
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
		jsonFlag = origJsonFlag
		harvestQuiet = origQuiet
		dryRun = origDryRun
		harvestMinConfidence = origMinConf
		harvestInclude = origInclude
		harvestMaxFileSize = origMaxSize
		harvestOutputDir = origOutputDir
		harvestPromoteTo = origPromote
	})

	harvestRootsFlag = fixture.tmp
	output = "json"
	jsonFlag = false
	harvestQuiet = true
	dryRun = false
	harvestMinConfidence = 0.5
	harvestInclude = "learnings"
	harvestMaxFileSize = 1048576
	harvestOutputDir = fixture.outputDir
	harvestPromoteTo = fixture.promoteTo
}

func runHarvestWithCapturedStdout(t *testing.T) []byte {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	runErr := runHarvest(harvestCmd, nil)

	w.Close()

	captured, _ := io.ReadAll(r)

	if runErr != nil {
		t.Fatalf("runHarvest returned error: %v", runErr)
	}

	return captured
}

func parseHarvestJSONCatalog(t *testing.T, captured []byte) harvest.Catalog {
	t.Helper()

	var cat harvest.Catalog
	if err := json.Unmarshal(captured, &cat); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot: %s", err, string(captured))
	}

	return cat
}

func assertHarvestJSONSideEffectCatalog(t *testing.T, cat harvest.Catalog) {
	t.Helper()

	if cat.DryRun {
		t.Fatal("JSON output should reflect non-dry-run execution")
	}
	if cat.PromotionCount != 1 {
		t.Fatalf("JSON output promotion_count = %d, want 1", cat.PromotionCount)
	}
	if cat.Summary.PromotionWrites != 1 {
		t.Fatalf("JSON output summary.promotion_writes = %d, want 1", cat.Summary.PromotionWrites)
	}
	if cat.Summary.WarningCount != 1 {
		t.Fatalf("JSON output summary.warning_count = %d, want 1", cat.Summary.WarningCount)
	}
	if len(cat.Warnings) != 1 || cat.Warnings[0].Stage != "parse_frontmatter" {
		t.Fatalf("JSON output warnings = %#v, want one parse_frontmatter warning", cat.Warnings)
	}
	if len(cat.Promoted) != 1 {
		t.Fatalf("JSON output promoted artifacts = %d, want 1", len(cat.Promoted))
	}
}

func assertHarvestJSONSideEffectPersistence(t *testing.T, fixture harvestJSONSideEffectsFixture) {
	t.Helper()

	latestPath := filepath.Join(fixture.outputDir, "latest.json")
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("expected latest.json to be written in JSON mode: %v", err)
	}
	var persisted harvest.Catalog
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("latest.json is not valid JSON: %v", err)
	}
	if persisted.PromotionCount != 1 || persisted.Summary.PromotionWrites != 1 {
		t.Fatalf("persisted promotion counts = %d/%d, want 1/1",
			persisted.PromotionCount, persisted.Summary.PromotionWrites)
	}
	if persisted.Summary.WarningCount != 1 {
		t.Fatalf("persisted summary.warning_count = %d, want 1", persisted.Summary.WarningCount)
	}
}

func assertHarvestJSONSideEffectPromotion(t *testing.T, fixture harvestJSONSideEffectsFixture, cat harvest.Catalog) {
	t.Helper()

	promotedPath := filepath.Join(fixture.promoteTo, "learning", cat.Promoted[0].SourceRig+"-"+fixture.sourceName)
	promoted, err := os.ReadFile(promotedPath)
	if err != nil {
		t.Fatalf("expected JSON mode to promote artifact %s: %v", promotedPath, err)
	}
	promotedText := string(promoted)
	if !strings.Contains(promotedText, `promoted_from: "proj-proj"`) {
		t.Fatalf("promoted artifact missing provenance header:\n%s", promotedText)
	}
	if !strings.Contains(promotedText, "# JSON Promote") {
		t.Fatalf("promoted artifact missing body:\n%s", promotedText)
	}
}

// ---------------------------------------------------------------------------
// duplicateArtifactCount
// ---------------------------------------------------------------------------

func TestDuplicateArtifactCount_NoDuplicates(t *testing.T) {
	cat := &harvest.Catalog{}
	got := duplicateArtifactCount(cat)
	if got != 0 {
		t.Errorf("duplicateArtifactCount(no dups) = %d, want 0", got)
	}
}

func TestDuplicateArtifactCount_WithDuplicates(t *testing.T) {
	cat := &harvest.Catalog{
		Duplicates: []harvest.DuplicateGroup{
			{Count: 3}, // 3-1 = 2 extra
			{Count: 2}, // 2-1 = 1 extra
			{Count: 1}, // 1-1 = 0 extra
		},
	}
	got := duplicateArtifactCount(cat)
	if got != 3 {
		t.Errorf("duplicateArtifactCount = %d, want 3", got)
	}
}

func TestRunHarvest_MalformedFileBecomesWarning(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	rigDir := filepath.Join(tmp, "myproject", ".agents", "learnings")
	if err := os.MkdirAll(rigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	valid := "---\ntitle: Valid Learning\nconfidence: 0.8\n---\n\n# Valid Learning\n\nThis artifact should still be harvested.\n"
	if err := os.WriteFile(filepath.Join(rigDir, "2026-04-10-valid.md"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := "---\nkey:\n\tvalue_with_tab: broken\n---\n"
	if err := os.WriteFile(filepath.Join(rigDir, "2026-04-10-bad.md"), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}

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
	harvestOutputDir = filepath.Join(tmp, "harvest-output")
	harvestPromoteTo = filepath.Join(tmp, "promoted")
	harvestQuiet = true
	dryRun = true
	harvestMinConfidence = 0.5
	harvestInclude = "learnings"
	harvestMaxFileSize = 1048576

	if err := runHarvest(harvestCmd, nil); err != nil {
		t.Fatalf("runHarvest returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(harvestOutputDir, "latest.json"))
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}
	var cat harvest.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("unmarshaling catalog: %v", err)
	}
	if len(cat.Artifacts) != 1 {
		t.Fatalf("expected 1 valid artifact, got %d", len(cat.Artifacts))
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %#v", cat.Warnings)
	}
	if cat.Summary.WarningCount != 1 {
		t.Fatalf("summary.warning_count = %d, want 1", cat.Summary.WarningCount)
	}
	if cat.TotalFiles != 2 {
		t.Fatalf("total_files = %d, want 2 candidate files", cat.TotalFiles)
	}
	if cat.Warnings[0].Stage != "parse_frontmatter" {
		t.Fatalf("warning stage = %q, want parse_frontmatter", cat.Warnings[0].Stage)
	}
}

// ---------------------------------------------------------------------------
// failIfDreamHoldsLock — pm-011 Dream vs. harvest concurrency guard
// ---------------------------------------------------------------------------

// writeTestLockFile creates .agents/overnight/run.lock under repoRoot with
// the given body. It returns the lock file path for follow-up mtime tweaks.
func writeTestLockFile(t *testing.T, repoRoot, body string) string {
	t.Helper()
	lockDir := filepath.Join(repoRoot, ".agents", "overnight")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	lockPath := filepath.Join(lockDir, "run.lock")
	if err := os.WriteFile(lockPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	return lockPath
}

func TestHarvest_RefusesDuringDreamRun(t *testing.T) {
	tmp := t.TempDir()
	// Use the current process PID so ProcessAlive returns true
	// deterministically without relying on any external process.
	writeTestLockFile(t, tmp, strconv.Itoa(os.Getpid())+"\n")

	err := failIfDreamHoldsLock(tmp)
	if err == nil {
		t.Fatalf("expected error when Dream holds a live lock, got nil")
	}
	if !strings.Contains(err.Error(), "Dream holds the overnight lock") {
		t.Fatalf("error message = %q, want substring %q", err.Error(), "Dream holds the overnight lock")
	}
}

func TestHarvest_ProceedsWhenNoLockFile(t *testing.T) {
	tmp := t.TempDir()

	if err := failIfDreamHoldsLock(tmp); err != nil {
		t.Fatalf("expected nil when no lock file exists, got %v", err)
	}
}

func TestHarvest_ProceedsWhenLockStale(t *testing.T) {
	tmp := t.TempDir()
	// Dead PID (way beyond the valid POSIX range on any normal
	// system) plus a backdated mtime so LockIsStale returns true.
	lockPath := writeTestLockFile(t, tmp, "999999999\n")

	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := failIfDreamHoldsLock(tmp); err != nil {
		t.Fatalf("expected nil for stale lock, got %v", err)
	}
}

func TestHarvest_ProceedsWhenLockPIDDead(t *testing.T) {
	tmp := t.TempDir()
	// Fresh mtime (default from WriteFile is now) so we bypass the
	// stale fast-path and exercise the explicit PID liveness check.
	writeTestLockFile(t, tmp, "999999999\n")

	if err := failIfDreamHoldsLock(tmp); err != nil {
		t.Fatalf("expected nil when lock PID is dead, got %v", err)
	}
}

func TestHarvest_LockFileCorrupted_ProceedsWithWarning(t *testing.T) {
	tmp := t.TempDir()
	// Garbage content — not a decimal PID. ReadLockPID should return
	// 0 and failIfDreamHoldsLock should proceed without blocking.
	writeTestLockFile(t, tmp, "this is not a pid file\n")

	if err := failIfDreamHoldsLock(tmp); err != nil {
		t.Fatalf("expected nil for corrupt lock file, got %v", err)
	}
}

func TestRunHarvest_PersistsDiscoveryWarnings(t *testing.T) {
	// Root bypasses filesystem permission checks, so the chmod(0)
	// trick below cannot trigger a permission-denied warning.
	if os.Getuid() == 0 {
		t.Skip("test requires non-root to enforce directory permissions")
	}

	tmp := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	validRigDir := filepath.Join(tmp, "goodproject", ".agents", "learnings")
	if err := os.MkdirAll(validRigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	valid := "---\ntitle: Valid Learning\nconfidence: 0.8\n---\n\n# Valid Learning\n\nThis artifact should still be harvested.\n"
	if err := os.WriteFile(filepath.Join(validRigDir, "2026-04-10-valid.md"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}

	badAgentsDir := filepath.Join(tmp, "badproject", ".agents")
	if err := os.MkdirAll(filepath.Join(badAgentsDir, "learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(badAgentsDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(badAgentsDir, 0o755)
	})

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
	harvestOutputDir = filepath.Join(tmp, "harvest-output")
	harvestPromoteTo = filepath.Join(tmp, "promoted")
	harvestQuiet = true
	dryRun = true
	harvestMinConfidence = 0.5
	harvestInclude = "learnings"
	harvestMaxFileSize = 1048576

	if err := runHarvest(harvestCmd, nil); err != nil {
		t.Fatalf("runHarvest returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(harvestOutputDir, "latest.json"))
	if err != nil {
		t.Fatalf("reading catalog: %v", err)
	}
	var cat harvest.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("unmarshaling catalog: %v", err)
	}
	if len(cat.Artifacts) != 1 {
		t.Fatalf("expected 1 harvested artifact, got %d", len(cat.Artifacts))
	}
	if cat.TotalFiles != 1 {
		t.Fatalf("total_files = %d, want 1 candidate file from the readable rig", cat.TotalFiles)
	}
	if cat.Summary.WarningCount != len(cat.Warnings) {
		t.Fatalf("summary.warning_count = %d, want %d", cat.Summary.WarningCount, len(cat.Warnings))
	}

	foundDiscoveryWarning := false
	for _, warning := range cat.Warnings {
		if strings.HasPrefix(warning.Stage, "discover_") {
			foundDiscoveryWarning = true
			if warning.Path == "" {
				t.Fatal("discovery warning should record the failing path")
			}
			break
		}
	}
	if !foundDiscoveryWarning {
		t.Fatalf("expected a persisted discovery warning, got %#v", cat.Warnings)
	}
}

package harvest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildCatalog_GroupsDuplicates(t *testing.T) {
	arts := []Artifact{
		{ID: "a1", ContentHash: "hash-dup", Confidence: 0.9, Date: "2026-03-01", Type: "learning"},
		{ID: "a2", ContentHash: "hash-dup", Confidence: 0.5, Date: "2026-03-02", Type: "learning"},
		{ID: "a3", ContentHash: "hash-unique", Confidence: 0.7, Date: "2026-03-01", Type: "pattern"},
	}

	cat := BuildCatalog(arts, 0.0)

	if len(cat.Duplicates) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(cat.Duplicates))
	}

	dg := cat.Duplicates[0]
	if dg.Hash != "hash-dup" {
		t.Errorf("expected duplicate hash %q, got %q", "hash-dup", dg.Hash)
	}
	if dg.Count != 2 {
		t.Errorf("expected count 2, got %d", dg.Count)
	}
	if dg.Kept != "a1" {
		t.Errorf("expected kept artifact %q (highest confidence), got %q", "a1", dg.Kept)
	}
	if len(dg.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts in group, got %d", len(dg.Artifacts))
	}
	if cat.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", cat.TotalFiles)
	}
	if cat.Summary.UniqueArtifacts != 2 {
		t.Errorf("expected summary.unique_artifacts=2, got %d", cat.Summary.UniqueArtifacts)
	}
	if cat.Summary.DuplicateGroups != 1 {
		t.Errorf("expected summary.duplicate_groups=1, got %d", cat.Summary.DuplicateGroups)
	}
	if cat.Summary.DuplicateExcess != 1 {
		t.Errorf("expected summary.duplicate_excess=1, got %d", cat.Summary.DuplicateExcess)
	}
}

func TestBuildCatalog_DuplicateTiebreakByDate(t *testing.T) {
	arts := []Artifact{
		{ID: "old", ContentHash: "hash-tie", Confidence: 0.8, Date: "2026-01-01", Type: "learning"},
		{ID: "new", ContentHash: "hash-tie", Confidence: 0.8, Date: "2026-03-01", Type: "learning"},
	}

	cat := BuildCatalog(arts, 0.0)

	if len(cat.Duplicates) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(cat.Duplicates))
	}
	if cat.Duplicates[0].Kept != "new" {
		t.Errorf("expected kept artifact %q (most recent date), got %q", "new", cat.Duplicates[0].Kept)
	}
}

func TestBuildCatalog_PromotionThreshold(t *testing.T) {
	arts := []Artifact{
		{ID: "low", ContentHash: "h1", Confidence: 0.2, Date: "2026-03-01", Type: "learning"},
		{ID: "mid", ContentHash: "h2", Confidence: 0.5, Date: "2026-03-01", Type: "pattern"},
		{ID: "high", ContentHash: "h3", Confidence: 0.8, Date: "2026-03-01", Type: "learning"},
	}

	cat := BuildCatalog(arts, 0.5)

	if len(cat.Promoted) != 2 {
		t.Fatalf("expected 2 promoted artifacts (>= 0.5), got %d", len(cat.Promoted))
	}
	if cat.Summary.PromotionCandidates != 2 {
		t.Fatalf("expected summary.promotion_candidates=2, got %d", cat.Summary.PromotionCandidates)
	}

	ids := map[string]bool{}
	for _, p := range cat.Promoted {
		ids[p.ID] = true
	}
	if !ids["mid"] {
		t.Error("expected 'mid' (confidence 0.5) to be promoted")
	}
	if !ids["high"] {
		t.Error("expected 'high' (confidence 0.8) to be promoted")
	}
	if ids["low"] {
		t.Error("'low' (confidence 0.2) should not be promoted")
	}
}

// TestBuildCatalog_TracksExcludedCandidates guards na-pmx1.10:
// below-threshold winners must be recorded (sorted by confidence desc) so
// the CLI can surface top-N near-misses and the exclusion count.
func TestBuildCatalog_TracksExcludedCandidates(t *testing.T) {
	arts := []Artifact{
		{ID: "far-low", Title: "far low", ContentHash: "h1", Confidence: 0.1, Date: "2026-03-01", Type: "learning"},
		{ID: "near-miss-a", Title: "near miss a", ContentHash: "h2", Confidence: 0.48, Date: "2026-03-02", Type: "pattern"},
		{ID: "near-miss-b", Title: "near miss b", ContentHash: "h3", Confidence: 0.45, Date: "2026-03-03", Type: "learning"},
		{ID: "promoted", Title: "promoted", ContentHash: "h4", Confidence: 0.7, Date: "2026-03-04", Type: "learning"},
	}

	cat := BuildCatalog(arts, 0.5)

	if len(cat.ExcludedCandidates) != 3 {
		t.Fatalf("expected 3 excluded, got %d", len(cat.ExcludedCandidates))
	}
	// First excluded must be the highest-confidence near-miss.
	if cat.ExcludedCandidates[0].ID != "near-miss-a" {
		t.Errorf("top excluded = %q, want near-miss-a", cat.ExcludedCandidates[0].ID)
	}
	// Ordering: 0.48 > 0.45 > 0.1
	if cat.ExcludedCandidates[1].ID != "near-miss-b" {
		t.Errorf("second excluded = %q, want near-miss-b", cat.ExcludedCandidates[1].ID)
	}
	if cat.ExcludedCandidates[2].ID != "far-low" {
		t.Errorf("third excluded = %q, want far-low", cat.ExcludedCandidates[2].ID)
	}

	top := cat.TopExcludedNearMiss(2)
	if len(top) != 2 {
		t.Fatalf("TopExcludedNearMiss(2) returned %d", len(top))
	}
	if top[0].ID != "near-miss-a" || top[1].ID != "near-miss-b" {
		t.Errorf("top-2 = %q,%q want near-miss-a,near-miss-b", top[0].ID, top[1].ID)
	}

	// n bigger than set returns everything available, not a panic.
	allTop := cat.TopExcludedNearMiss(100)
	if len(allTop) != 3 {
		t.Errorf("TopExcludedNearMiss(100) returned %d, want 3", len(allTop))
	}

	// n <= 0 returns nil.
	if got := cat.TopExcludedNearMiss(0); got != nil {
		t.Errorf("TopExcludedNearMiss(0) = %v, want nil", got)
	}
}

func TestPromote_CopiesWithProvenance(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create a source file with frontmatter including metadata the scoring pipeline needs.
	srcFile := filepath.Join(srcDir, "note.md")
	srcContent := "---\ntitle: Original\nconfidence: 0.9\nmaturity: provisional\nutility: 0.7\ntype: learning\ndate: 2026-04-02\n---\n\n# My Learning\n\nSome content here.\n"
	if err := os.WriteFile(srcFile, []byte(srcContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := &Catalog{
		Promoted: []Artifact{
			{
				ID:         "art-1",
				Type:       "learning",
				SourceRig:  "agentops-nami",
				SourcePath: srcFile,
				Confidence: 0.9,
			},
		},
	}

	count, err := Promote(cat, destDir, false)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 promoted, got %d", count)
	}

	destFile := filepath.Join(destDir, "learning", "agentops-nami-note.md")
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("promoted file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `promoted_from: "agentops-nami"`) {
		t.Error("missing promoted_from in provenance header")
	}
	if !strings.Contains(content, `original_path:`) {
		t.Error("missing original_path in provenance header")
	}
	if !strings.Contains(content, `harvest_confidence: 0.9`) {
		t.Error("missing harvest_confidence in provenance header")
	}
	if !strings.Contains(content, "# My Learning") {
		t.Error("missing body content in promoted file")
	}
	// Verify scoring-relevant metadata is preserved from original frontmatter.
	if !strings.Contains(content, "maturity: provisional") {
		t.Error("original maturity should be preserved in promoted file")
	}
	if !strings.Contains(content, "utility: 0.7") {
		t.Error("original utility should be preserved in promoted file")
	}
	if !strings.Contains(content, "type: learning") {
		t.Error("original type should be preserved in promoted file")
	}
	if !strings.Contains(content, "confidence: 0.9") {
		t.Error("original confidence should be preserved in promoted file")
	}
	// Non-carried fields (like title) should NOT appear — only scoring-relevant fields are kept.
	if strings.Contains(content, "title: Original") {
		t.Error("non-scoring fields like title should not be carried forward")
	}
}

func TestPromote_DryRunNoCopy(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "note.md")
	if err := os.WriteFile(srcFile, []byte("# Content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := &Catalog{
		Promoted: []Artifact{
			{
				ID:         "art-1",
				Type:       "learning",
				SourceRig:  "test-rig",
				SourcePath: srcFile,
				Confidence: 0.8,
			},
		},
	}

	count, err := Promote(cat, destDir, true)
	if err != nil {
		t.Fatalf("Promote dry run failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 in dry run, got %d", count)
	}

	destFile := filepath.Join(destDir, "learning", "test-rig-note.md")
	if _, err := os.Stat(destFile); err == nil {
		t.Error("dry run should not create files")
	}
}

func TestPromote_SkipsExisting(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "note.md")
	if err := os.WriteFile(srcFile, []byte("# Content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-create the destination.
	typeDir := filepath.Join(destDir, "learning")
	if err := os.MkdirAll(typeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	destFile := filepath.Join(typeDir, "test-rig-note.md")
	if err := os.WriteFile(destFile, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := &Catalog{
		Promoted: []Artifact{
			{
				ID:         "art-1",
				Type:       "learning",
				SourceRig:  "test-rig",
				SourcePath: srcFile,
				Confidence: 0.8,
			},
		},
	}

	count, err := Promote(cat, destDir, false)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 promoted (existing skipped), got %d", count)
	}
}

func TestWriteCatalog_DatedAndLatest(t *testing.T) {
	dir := t.TempDir()

	cat := &Catalog{
		Timestamp:   mustParseTime("2026-03-15T12:00:00Z"),
		RigsScanned: 3,
		TotalFiles:  10,
		Artifacts:   []Artifact{{ID: "a1", Type: "learning"}},
	}

	if err := WriteCatalog(dir, cat); err != nil {
		t.Fatalf("WriteCatalog failed: %v", err)
	}

	dated := filepath.Join(dir, "2026-03-15.json")
	latest := filepath.Join(dir, "latest.json")

	datedData, err := os.ReadFile(dated)
	if err != nil {
		t.Fatalf("dated file not found: %v", err)
	}
	latestData, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("latest.json not found: %v", err)
	}

	if string(datedData) != string(latestData) {
		t.Error("dated and latest files should have identical content")
	}

	// Verify valid JSON.
	var parsed Catalog
	if err := json.Unmarshal(datedData, &parsed); err != nil {
		t.Fatalf("dated file is not valid JSON: %v", err)
	}
	if parsed.RigsScanned != 3 {
		t.Errorf("expected RigsScanned=3, got %d", parsed.RigsScanned)
	}
	if parsed.Summary.ArtifactsExtracted != 1 {
		t.Errorf("expected summary.artifacts_extracted=1, got %d", parsed.Summary.ArtifactsExtracted)
	}
	if len(parsed.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(parsed.Artifacts))
	}
	if parsed.Artifacts[0].ID != "a1" {
		t.Errorf("expected artifact ID %q, got %q", "a1", parsed.Artifacts[0].ID)
	}
}

func mustParseTime(s string) time.Time {
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return parsed
}

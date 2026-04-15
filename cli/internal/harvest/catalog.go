package harvest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CatalogSchemaVersion identifies the JSON schema version for the Catalog
// output. Bump this when making breaking changes to the serialized shape so
// downstream consumers can detect incompatibilities.
//
// Version history:
//   - 2: ExcludedCandidates slimmed from []Artifact to []ExcludedArtifactMeta
//     (id, title, confidence only). Introduced SchemaVersion field.
const CatalogSchemaVersion = 2

// ExcludedArtifactMeta is the slim representation of a below-threshold
// candidate emitted into the catalog JSON. The full Artifact (with
// Frontmatter, SourcePath, ContentHash, etc.) balloons the catalog when
// the exclusion list grows past ~100, so JSON output carries only the
// fields operators actually inspect: ID, Title, and Confidence.
type ExcludedArtifactMeta struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
}

// toExcludedMeta projects an Artifact down to the slim JSON-facing
// representation used for Catalog.ExcludedCandidates.
func toExcludedMeta(a Artifact) ExcludedArtifactMeta {
	return ExcludedArtifactMeta{
		ID:         a.ID,
		Title:      a.Title,
		Confidence: a.Confidence,
	}
}

// Catalog holds the results of a cross-rig harvest.
type Catalog struct {
	SchemaVersion int              `json:"schema_version"`
	Timestamp     time.Time        `json:"timestamp"`
	RigsScanned   int              `json:"rigs_scanned"`
	TotalFiles    int              `json:"total_files"` // Candidate files seen across included dirs; BuildCatalog falls back to len(artifacts)
	Roots         []string         `json:"roots,omitempty"`
	IncludeDirs   []string         `json:"include_dirs,omitempty"`
	PromoteTo     string           `json:"promote_to,omitempty"`
	MinConfidence float64          `json:"min_confidence,omitempty"`
	DryRun        bool             `json:"dry_run,omitempty"`
	Rigs          []RigInfo        `json:"rigs,omitempty"`
	Warnings      []HarvestWarning `json:"warnings,omitempty"`
	Artifacts     []Artifact       `json:"artifacts"`
	Duplicates    []DuplicateGroup `json:"duplicates"`
	Promoted      []Artifact       `json:"promoted"`
	// ExcludedCandidates are winners (deduped artifacts) that did not meet
	// MinConfidence. Tracked so operators can see how much signal the
	// threshold is dropping and decide whether to tune it. Serialized as
	// a slim projection (id, title, confidence) to keep the JSON compact.
	ExcludedCandidates []ExcludedArtifactMeta `json:"excluded_candidates,omitempty"`
	// excludedFull retains the full Artifact values for internal use
	// (e.g., TopExcludedNearMiss). It is not serialized to JSON.
	excludedFull   []Artifact     `json:"-"`
	PromotionCount int            `json:"promotion_count,omitempty"`
	Summary        CatalogSummary `json:"summary"`
}

// DuplicateGroup represents artifacts with identical content across rigs.
type DuplicateGroup struct {
	Hash      string     `json:"hash"`
	Count     int        `json:"count"`
	Artifacts []Artifact `json:"artifacts"`
	Kept      string     `json:"kept"` // ID of the kept artifact
}

// CatalogSummary exposes the operator-facing counts that downstream skills and
// humans would otherwise have to reconstruct from the raw artifact lists.
type CatalogSummary struct {
	ArtifactsExtracted  int            `json:"artifacts_extracted"`
	UniqueArtifacts     int            `json:"unique_artifacts"`
	DuplicateGroups     int            `json:"duplicate_groups"`
	DuplicateArtifacts  int            `json:"duplicate_artifacts"`
	DuplicateExcess     int            `json:"duplicate_excess"`
	PromotionCandidates int            `json:"promotion_candidates"`
	PromotionWrites     int            `json:"promotion_writes"`
	WarningCount        int            `json:"warning_count"`
	ArtifactsByType     map[string]int `json:"artifacts_by_type,omitempty"`
}

// BuildCatalog groups artifacts by content hash, resolves duplicates by
// confidence, and identifies promotion candidates above minConfidence.
func BuildCatalog(artifacts []Artifact, minConfidence float64) *Catalog {
	cat := &Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Timestamp:     time.Now().UTC(),
		TotalFiles:    len(artifacts),
		Artifacts:     artifacts,
	}

	// Group by ContentHash.
	groups := make(map[string][]Artifact)
	for _, a := range artifacts {
		groups[a.ContentHash] = append(groups[a.ContentHash], a)
	}

	// Track winners (unique artifacts or duplicate winners).
	var winners []Artifact

	// Collect hashes in sorted order for deterministic output.
	hashes := make([]string, 0, len(groups))
	for h := range groups {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)

	for _, h := range hashes {
		arts := groups[h]
		if len(arts) == 1 {
			winners = append(winners, arts[0])
			continue
		}

		// Sort: highest confidence first, then most recent date, then alphabetical ID.
		sort.Slice(arts, func(i, j int) bool {
			if arts[i].Confidence != arts[j].Confidence {
				return arts[i].Confidence > arts[j].Confidence
			}
			if arts[i].Date != arts[j].Date {
				return arts[i].Date > arts[j].Date
			}
			return arts[i].ID < arts[j].ID
		})

		winner := arts[0]
		winners = append(winners, winner)

		cat.Duplicates = append(cat.Duplicates, DuplicateGroup{
			Hash:      h,
			Count:     len(arts),
			Artifacts: arts,
			Kept:      winner.ID,
		})
	}

	// Promote winners above threshold; track exclusions below it so the
	// operator can see what the threshold dropped. We keep full Artifact
	// values in excludedFull for internal use (TopExcludedNearMiss) and
	// emit a slim projection via ExcludedCandidates for JSON output.
	for _, w := range winners {
		if w.Confidence >= minConfidence {
			cat.Promoted = append(cat.Promoted, w)
		} else {
			cat.excludedFull = append(cat.excludedFull, w)
		}
	}
	// Sort exclusions by confidence descending so near-misses come first.
	sort.Slice(cat.excludedFull, func(i, j int) bool {
		return cat.excludedFull[i].Confidence > cat.excludedFull[j].Confidence
	})
	cat.ExcludedCandidates = make([]ExcludedArtifactMeta, 0, len(cat.excludedFull))
	for _, a := range cat.excludedFull {
		cat.ExcludedCandidates = append(cat.ExcludedCandidates, toExcludedMeta(a))
	}
	if len(cat.ExcludedCandidates) == 0 {
		// Preserve `omitempty` behavior: an empty slice still serializes
		// as `[]`, which differs from the previous nil-slice output.
		cat.ExcludedCandidates = nil
	}
	cat.refreshSummary()

	return cat
}

// TopExcludedNearMiss returns up to n excluded candidates with the highest
// confidence, so the operator can preview what is sitting just below the
// min-confidence threshold and decide whether to lower it.
//
// Returns full Artifact values for internal callers (CLI output uses
// Title/Type/Confidence). The JSON-serialized ExcludedCandidates field
// is slimmed separately.
func (c *Catalog) TopExcludedNearMiss(n int) []Artifact {
	if n <= 0 || len(c.excludedFull) == 0 {
		return nil
	}
	if n > len(c.excludedFull) {
		n = len(c.excludedFull)
	}
	out := make([]Artifact, n)
	copy(out, c.excludedFull[:n])
	return out
}

// Promote copies promoted artifacts to destDir with provenance headers.
// Returns the count of files promoted. If dryRun is true, counts but does
// not write any files.
func Promote(catalog *Catalog, destDir string, dryRun bool) (int, error) {
	count := 0

	for _, art := range catalog.Promoted {
		// Create type subdirectory (pm-003).
		typeDir := filepath.Join(destDir, art.Type)
		if !dryRun {
			if err := os.MkdirAll(typeDir, 0o755); err != nil {
				return count, fmt.Errorf("creating type dir %s: %w", typeDir, err)
			}
		}

		// Build destination filename: {source_rig}-{basename}.
		base := filepath.Base(art.SourcePath)
		destName := art.SourceRig + "-" + base
		destPath := filepath.Join(typeDir, destName)

		// Skip if destination already exists.
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		count++

		if dryRun {
			continue
		}

		// Read source file.
		data, err := os.ReadFile(art.SourcePath)
		if err != nil {
			return count, fmt.Errorf("reading source %s: %w", art.SourcePath, err)
		}

		// Merge original frontmatter with provenance fields.
		// Preserves maturity, utility, type, confidence from the source
		// while adding harvest provenance metadata.
		now := time.Now().UTC().Format("2006-01-02")
		origFM := extractFrontmatter(string(data))
		body := stripFrontmatter(string(data))

		// Start with provenance fields
		var headerLines []string
		headerLines = append(headerLines,
			fmt.Sprintf("promoted_from: %q", art.SourceRig),
			fmt.Sprintf("promoted_at: %q", now),
			fmt.Sprintf("original_path: %q", art.SourcePath),
			fmt.Sprintf("harvest_confidence: %g", art.Confidence),
		)

		// Carry forward original metadata fields that the scoring pipeline needs.
		// These are the fields that passesQualityGate and inject_scoring check.
		// When a field is missing from the source, add a default so harvested
		// files always have the minimum metadata for scoring.
		defaults := map[string]string{
			"type":     art.Type,
			"maturity": "provisional",
			"utility":  "0.5",
		}
		for _, key := range []string{"type", "maturity", "utility", "confidence", "source_bead", "source_phase", "date", "category", "id"} {
			if val, ok := origFM[key]; ok {
				headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, val))
			} else if def, ok := defaults[key]; ok {
				headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, def))
			}
		}

		header := "---\n" + strings.Join(headerLines, "\n") + "\n---\n\n"
		content := header + body

		if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
			return count, fmt.Errorf("writing promoted file %s: %w", destPath, err)
		}
	}

	return count, nil
}

// extractFrontmatter parses YAML frontmatter into a key-value map.
// Returns an empty map if no frontmatter is found.
func extractFrontmatter(content string) map[string]string {
	fm := make(map[string]string)
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return fm
	}
	first := strings.Index(trimmed, "---")
	rest := trimmed[first+3:]
	second := strings.Index(rest, "---")
	if second < 0 {
		return fm
	}
	block := rest[:second]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Remove surrounding quotes if present
			val = strings.Trim(val, "\"'")
			fm[key] = val
		}
	}
	return fm
}

// stripFrontmatter removes YAML frontmatter delimiters and content,
// returning only the body.
func stripFrontmatter(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return content
	}

	first := strings.Index(trimmed, "---")
	rest := trimmed[first+3:]
	second := strings.Index(rest, "---")
	if second < 0 {
		return content
	}

	return strings.TrimLeft(rest[second+3:], "\n")
}

// WriteCatalog writes the catalog as indented JSON to both a dated file
// and a latest.json symlink-free copy.
func WriteCatalog(dir string, cat *Catalog) error {
	if cat.SchemaVersion == 0 {
		cat.SchemaVersion = CatalogSchemaVersion
	}
	cat.refreshSummary()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating catalog dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling catalog: %w", err)
	}
	data = append(data, '\n')

	dated := filepath.Join(dir, cat.Timestamp.Format("2006-01-02")+".json")
	if err := os.WriteFile(dated, data, 0o644); err != nil {
		return fmt.Errorf("writing dated catalog %s: %w", dated, err)
	}

	latest := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(latest, data, 0o644); err != nil {
		return fmt.Errorf("writing latest catalog %s: %w", latest, err)
	}

	return nil
}

func (cat *Catalog) refreshSummary() {
	if cat == nil {
		return
	}

	byType := map[string]int{}
	for _, art := range cat.Artifacts {
		byType[art.Type]++
	}

	duplicateArtifacts := 0
	duplicateExcess := 0
	for _, group := range cat.Duplicates {
		duplicateArtifacts += group.Count
		if group.Count > 1 {
			duplicateExcess += group.Count - 1
		}
	}

	cat.Summary = CatalogSummary{
		ArtifactsExtracted:  len(cat.Artifacts),
		UniqueArtifacts:     len(cat.Artifacts) - duplicateExcess,
		DuplicateGroups:     len(cat.Duplicates),
		DuplicateArtifacts:  duplicateArtifacts,
		DuplicateExcess:     duplicateExcess,
		PromotionCandidates: len(cat.Promoted),
		PromotionWrites:     cat.PromotionCount,
		WarningCount:        len(cat.Warnings),
		ArtifactsByType:     byType,
	}
}

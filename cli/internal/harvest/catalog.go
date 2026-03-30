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

// Catalog holds the results of a cross-rig harvest.
type Catalog struct {
	Timestamp   time.Time        `json:"timestamp"`
	RigsScanned int              `json:"rigs_scanned"`
	TotalFiles  int              `json:"total_files"`
	Artifacts   []Artifact       `json:"artifacts"`
	Duplicates  []DuplicateGroup `json:"duplicates"`
	Promoted    []Artifact       `json:"promoted"`
}

// DuplicateGroup represents artifacts with identical content across rigs.
type DuplicateGroup struct {
	Hash      string     `json:"hash"`
	Count     int        `json:"count"`
	Artifacts []Artifact `json:"artifacts"`
	Kept      string     `json:"kept"` // ID of the kept artifact
}

// BuildCatalog groups artifacts by content hash, resolves duplicates by
// confidence, and identifies promotion candidates above minConfidence.
func BuildCatalog(artifacts []Artifact, minConfidence float64) *Catalog {
	cat := &Catalog{
		Timestamp:  time.Now().UTC(),
		TotalFiles: len(artifacts),
		Artifacts:  artifacts,
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

	// Promote winners above threshold.
	for _, w := range winners {
		if w.Confidence >= minConfidence {
			cat.Promoted = append(cat.Promoted, w)
		}
	}

	return cat
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

		// Strip original frontmatter, keep body only.
		body := stripFrontmatter(string(data))

		// Build provenance header.
		now := time.Now().UTC().Format("2006-01-02")
		header := fmt.Sprintf("---\npromoted_from: %q\npromoted_at: %q\noriginal_path: %q\nharvest_confidence: %g\n---\n\n",
			art.SourceRig, now, art.SourcePath, art.Confidence)

		content := header + body

		if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
			return count, fmt.Errorf("writing promoted file %s: %w", destPath, err)
		}
	}

	return count, nil
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

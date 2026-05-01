package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DedupGroup represents a set of duplicate learnings.
type DedupGroup struct {
	Hash  string   `json:"hash"`
	Count int      `json:"count"`
	Files []string `json:"files"`
}

// DedupResult is the output of the dedup scan.
type DedupResult struct {
	TotalFiles      int          `json:"total_files"`
	UniqueContent   int          `json:"unique_content"`
	DuplicateGroups int          `json:"duplicate_groups"`
	DuplicateFiles  int          `json:"duplicate_files"`
	Groups          []DedupGroup `json:"groups,omitempty"`
}

// DedupManifestArchived describes a single archived file in a merge manifest.
type DedupManifestArchived struct {
	Src     string  `json:"src"`
	Dst     string  `json:"dst"`
	Utility float64 `json:"utility"`
}

// DedupManifestGroup describes one duplicate group resolution.
type DedupManifestGroup struct {
	ContentHash string                  `json:"content_hash"`
	Kept        string                  `json:"kept"`
	KeptUtility float64                 `json:"kept_utility"`
	Archived    []DedupManifestArchived `json:"archived"`
}

// DedupManifestTotals summarizes a merge operation.
type DedupManifestTotals struct {
	Groups        int `json:"groups"`
	ArchivedFiles int `json:"archived_files"`
	KeptFiles     int `json:"kept_files"`
}

// DedupManifest is the JSON manifest written before any moves happen.
// It enables exact reversal of an `ao dedup --merge` operation.
type DedupManifest struct {
	Version       int                  `json:"version"`
	Timestamp     string               `json:"timestamp"`
	MergeStrategy string               `json:"merge_strategy"`
	Groups        []DedupManifestGroup `json:"groups"`
	Totals        DedupManifestTotals  `json:"totals"`
}

// CollectDedupFiles finds all .jsonl and .md files in learnings and patterns directories.
// Returns the combined file list. Returns an empty list if neither directory exists.
func CollectDedupFiles(cwd string) ([]string, error) {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	patternsDir := filepath.Join(cwd, ".agents", "patterns")

	learningsExists := true
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		learningsExists = false
	}
	patternsExists := true
	if _, err := os.Stat(patternsDir); os.IsNotExist(err) {
		patternsExists = false
	}

	if !learningsExists && !patternsExists {
		return nil, nil
	}

	files := make([]string, 0)
	for _, dir := range []string{learningsDir, patternsDir} {
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		files = append(files, jsonlFiles...)
		files = append(files, mdFiles...)
	}

	return files, nil
}

// GroupByContentHash groups files by their normalized content hash.
// Files whose body is empty are skipped.
func GroupByContentHash(files []string) map[string][]string {
	hashToFiles := make(map[string][]string)
	for _, f := range files {
		body := ExtractLearningBody(f)
		if body == "" {
			continue
		}
		hash := HashNormalizedContent(body)
		hashToFiles[hash] = append(hashToFiles[hash], f)
	}
	return hashToFiles
}

// BuildDedupManifest constructs the merge manifest from the duplicate groups.
// It does not touch the filesystem; pure data transformation suitable for
// preview-then-write flows. Groups with <=1 file are skipped (no duplicates).
// Group iteration order is sorted by content hash for stable manifests.
func BuildDedupManifest(hashToFiles map[string][]string, cwd, archiveDirRel string, now time.Time) DedupManifest {
	hashes := make([]string, 0, len(hashToFiles))
	for h, group := range hashToFiles {
		if len(group) > 1 {
			hashes = append(hashes, h)
		}
	}
	sort.Strings(hashes)

	manifest := DedupManifest{
		Version:       1,
		Timestamp:     now.UTC().Format(time.RFC3339),
		MergeStrategy: "highest-utility",
	}
	for _, h := range hashes {
		group := hashToFiles[h]
		kept, archived := PickHighestUtility(group)
		keptRel, _ := filepath.Rel(cwd, kept)
		mg := DedupManifestGroup{
			ContentHash: truncateHash(h),
			Kept:        keptRel,
			KeptUtility: ReadUtilityFromFile(kept),
		}
		for _, a := range archived {
			aRel, _ := filepath.Rel(cwd, a)
			dstRel := filepath.Join(archiveDirRel, filepath.Base(a))
			mg.Archived = append(mg.Archived, DedupManifestArchived{
				Src:     aRel,
				Dst:     dstRel,
				Utility: ReadUtilityFromFile(a),
			})
		}
		manifest.Groups = append(manifest.Groups, mg)
		manifest.Totals.Groups++
		manifest.Totals.KeptFiles++
		manifest.Totals.ArchivedFiles += len(archived)
	}
	return manifest
}

// CountArchiveCandidates returns the number of files that would be archived if
// the dedup groups were merged. Used for threshold-pause prompting before any
// destructive work.
func CountArchiveCandidates(hashToFiles map[string][]string) int {
	total := 0
	for _, group := range hashToFiles {
		if len(group) > 1 {
			total += len(group) - 1
		}
	}
	return total
}

// WriteDedupManifest atomically writes the manifest as pretty-printed JSON.
// It creates parent directories, writes to a temp file, and renames into place.
func WriteDedupManifest(manifest DedupManifest, manifestPath string) error {
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	tmp := manifestPath + ".tmp"
	if writeErr := os.WriteFile(tmp, data, 0o640); writeErr != nil {
		return fmt.Errorf("write manifest tmp: %w", writeErr)
	}
	if renameErr := os.Rename(tmp, manifestPath); renameErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename manifest: %w", renameErr)
	}
	return nil
}

// DedupManifestPath returns the canonical path for a manifest given a UTC timestamp.
// Format: <cwd>/.agents/archive/dedup/<UTC-timestamp>-manifest.json with colons
// stripped to keep the filename portable across filesystems (e.g., Windows).
func DedupManifestPath(cwd string, now time.Time) string {
	stamp := now.UTC().Format("20060102T150405Z")
	return filepath.Join(cwd, ".agents", "archive", "dedup", stamp+"-manifest.json")
}

// MergeDedupGroups resolves duplicate groups by keeping the highest-utility file
// and archiving the rest to .agents/archive/dedup/. In dry-run mode, it prints
// what would happen without moving any files.
//
// On non-dry-run runs, a JSON manifest is written BEFORE any file moves so the
// operation is fully reversible. The manifest path is printed and returned via
// the manifestPath out-of-band side effect (callers can compute it independently
// using DedupManifestPath if needed).
func MergeDedupGroups(hashToFiles map[string][]string, cwd string, dryRun bool) error {
	archiveDir := filepath.Join(cwd, ".agents", "archive", "dedup")
	archiveDirRel := filepath.Join(".agents", "archive", "dedup")

	if dryRun {
		fmt.Println("Merge (dry-run):")
		manifest := BuildDedupManifest(hashToFiles, cwd, archiveDirRel, time.Now())
		for _, g := range manifest.Groups {
			fmt.Printf("  Keep: %s (utility %.2f)\n", g.Kept, g.KeptUtility)
			for _, a := range g.Archived {
				fmt.Printf("  [dry-run] Would archive: %s -> %s\n", a.Src, a.Dst)
			}
		}
		return nil
	}

	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	now := time.Now()
	manifest := BuildDedupManifest(hashToFiles, cwd, archiveDirRel, now)
	manifestPath := DedupManifestPath(cwd, now)
	if err := WriteDedupManifest(manifest, manifestPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	manifestRel, _ := filepath.Rel(cwd, manifestPath)
	fmt.Printf("Manifest: %s\n", manifestRel)

	fmt.Println("Merge Results:")
	for _, g := range manifest.Groups {
		fmt.Printf("  Keep: %s (utility %.2f)\n", g.Kept, g.KeptUtility)
		for _, a := range g.Archived {
			src := filepath.Join(cwd, a.Src)
			dst := filepath.Join(cwd, a.Dst)
			if mvErr := os.Rename(src, dst); mvErr != nil {
				fmt.Fprintf(os.Stderr, "Error archiving %s: %v\n", a.Src, mvErr)
				continue
			}
			fmt.Printf("  Archived: %s -> %s\n", a.Src, a.Dst)
		}
	}
	return nil
}

// BuildDedupResult creates a DedupResult from the hash-to-files map,
// using relative paths for cleaner output.
func BuildDedupResult(hashToFiles map[string][]string, totalFiles int, cwd string) DedupResult {
	result := DedupResult{
		TotalFiles:    totalFiles,
		UniqueContent: len(hashToFiles),
	}

	for hash, group := range hashToFiles {
		if len(group) > 1 {
			result.DuplicateGroups++
			result.DuplicateFiles += len(group)
			relFiles := make([]string, len(group))
			for i, f := range group {
				rel, relErr := filepath.Rel(cwd, f)
				if relErr != nil {
					rel = f
				}
				relFiles[i] = rel
			}
			result.Groups = append(result.Groups, DedupGroup{
				Hash:  truncateHash(hash),
				Count: len(group),
				Files: relFiles,
			})
		}
	}

	return result
}

// truncateHash returns the first 12 chars of a hash, or the hash itself if shorter.
func truncateHash(h string) string {
	if len(h) >= 12 {
		return h[:12]
	}
	return h
}

// PickHighestUtility selects the file with the highest utility from a group.
// Returns the keeper and the list of files to archive.
func PickHighestUtility(files []string) (string, []string) {
	bestIdx := 0
	bestUtility := ReadUtilityFromFile(files[0])
	for i := 1; i < len(files); i++ {
		u := ReadUtilityFromFile(files[i])
		if u > bestUtility {
			bestUtility = u
			bestIdx = i
		}
	}
	kept := files[bestIdx]
	var archived []string
	for i, f := range files {
		if i != bestIdx {
			archived = append(archived, f)
		}
	}
	return kept, archived
}

// ReadUtilityFromFile reads the utility value from a file's metadata.
// For .md files, parses YAML frontmatter. For .jsonl files, parses the first JSON line.
// Returns 0.5 as default if utility is not found or cannot be parsed.
func ReadUtilityFromFile(path string) float64 {
	const defaultUtility = 0.5

	content, err := os.ReadFile(path)
	if err != nil {
		return defaultUtility
	}
	text := string(content)

	if strings.HasSuffix(path, ".jsonl") {
		return ReadUtilityFromJSONL(text, defaultUtility)
	}
	return ReadUtilityFromFrontmatter(text, defaultUtility)
}

// ReadUtilityFromFrontmatter parses YAML frontmatter for the utility field.
func ReadUtilityFromFrontmatter(text string, defaultVal float64) float64 {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return defaultVal
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return defaultVal
		}
		if strings.HasPrefix(line, "utility:") {
			valStr := strings.TrimSpace(strings.TrimPrefix(line, "utility:"))
			if u, parseErr := strconv.ParseFloat(valStr, 64); parseErr == nil {
				return u
			}
		}
	}
	return defaultVal
}

// ReadUtilityFromJSONL parses the first JSON line for the utility field.
func ReadUtilityFromJSONL(text string, defaultVal float64) float64 {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return defaultVal
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return defaultVal
	}
	switch v := data["utility"].(type) {
	case float64:
		return v
	case string:
		if u, parseErr := strconv.ParseFloat(v, 64); parseErr == nil {
			return u
		}
	}
	return defaultVal
}

// ExtractLearningBody extracts the body content from a learning file.
func ExtractLearningBody(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(content)

	if strings.HasSuffix(path, ".md") {
		return ExtractMarkdownBody(text)
	}
	return ExtractJSONLBody(text)
}

// ExtractMarkdownBody returns content after YAML frontmatter.
func ExtractMarkdownBody(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return text
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return text
}

// ExtractJSONLBody extracts title or content from a JSONL first line.
func ExtractJSONLBody(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return ""
	}
	if content, ok := data["content"].(string); ok && content != "" {
		return content
	}
	if title, ok := data["title"].(string); ok {
		return title
	}
	return ""
}

// HashNormalizedContent normalizes and hashes content for dedup comparison.
func HashNormalizedContent(body string) string {
	s := strings.ToLower(strings.TrimSpace(body))
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "---", "")
	s = strings.Join(strings.Fields(s), " ")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

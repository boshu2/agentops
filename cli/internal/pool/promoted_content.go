package pool

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CanonicalContentBody returns the semantic body used for promoted-content
// hashing. Pending captures and older Pool.writeArtifact output can both
// contain nested "## What We Learned" sections, so the last matching section
// is treated as the highest-signal body. Documents without that section fall
// back to the post-frontmatter text.
func CanonicalContentBody(markdown string) string {
	postFrontmatter := stripYAMLFrontmatter(markdown)
	if section, ok := extractLastSection(postFrontmatter, "## What We Learned"); ok {
		return section
	}
	return strings.TrimSpace(postFrontmatter)
}

// ExtractPromotedArtifactBody returns the canonical candidate body from a
// promoted artifact markdown document. Pool.writeArtifact stores candidate
// content under "## What We Learned"; older hand-authored artifacts fall back
// to the post-frontmatter document body so they still participate in live
// dedupe scans and reindex backfills.
func ExtractPromotedArtifactBody(markdown string) (string, bool) {
	body := CanonicalContentBody(markdown)
	if body == "" {
		return "", false
	}
	return body, true
}

// ExtractPromotedArtifactBodyFile reads path and extracts the canonical body
// with ExtractPromotedArtifactBody.
func ExtractPromotedArtifactBodyFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return ExtractPromotedArtifactBody(string(data))
}

// CollectPromotedArtifactFiles walks .agents/learnings and .agents/patterns
// under baseDir and returns all markdown artifacts in deterministic order.
func CollectPromotedArtifactFiles(baseDir string) ([]string, error) {
	dirs := []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	}
	var files []string
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files, nil
}

// CollectArchivedArtifactFiles returns markdown artifacts that were archived
// by AgentOps corpus cleanup. These bodies are no longer active promoted
// artifacts, but they are known corpus bodies and must not be recreated from
// stale pending or pool state.
func CollectArchivedArtifactFiles(baseDir string) ([]string, error) {
	patterns := []string{
		filepath.Join(baseDir, ".agents", "archive", "dedup", "*.md"),
		filepath.Join(baseDir, ".agents", "defrag", "*", "files", ".agents", "learnings", "*.md"),
		filepath.Join(baseDir, ".agents", "defrag", "*", "files", ".agents", "patterns", "*.md"),
	}
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files, nil
}

// LoadPromotedContentHashes live-scans surviving promoted artifacts and
// returns content_hash -> artifact_path. It is the canonical fallback when
// promoted-index.jsonl is missing or stale.
func LoadPromotedContentHashes(baseDir string) map[string]string {
	files, err := CollectPromotedArtifactFiles(baseDir)
	if err != nil {
		return map[string]string{}
	}
	return contentHashesForFiles(files)
}

// LoadArchivedContentHashes returns hashes for cleanup-archived knowledge
// artifacts. Use this to block recreation of seen-again bodies; do not use it
// for pool reindex because archived paths are not active promoted artifacts.
func LoadArchivedContentHashes(baseDir string) map[string]string {
	files, err := CollectArchivedArtifactFiles(baseDir)
	if err != nil {
		return map[string]string{}
	}
	return contentHashesForFiles(files)
}

// LoadKnownPromotedContentHashes combines active promoted artifacts and
// cleanup archives. Active artifacts win when the same hash exists in both.
func LoadKnownPromotedContentHashes(baseDir string) map[string]string {
	known := LoadArchivedContentHashes(baseDir)
	for hash, path := range LoadPromotedContentHashes(baseDir) {
		known[hash] = path
	}
	return known
}

func contentHashesForFiles(files []string) map[string]string {
	hashes := make(map[string]string)
	for _, f := range files {
		body, ok := ExtractPromotedArtifactBodyFile(f)
		if !ok {
			continue
		}
		hash := ContentHash(body)
		if hash == "" {
			continue
		}
		if _, exists := hashes[hash]; !exists {
			hashes[hash] = f
		}
	}
	return hashes
}

func stripYAMLFrontmatter(text string) string {
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

func extractLastSection(text, heading string) (string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
		}
	}
	if start == -1 {
		return "", false
	}
	end := len(lines)
	for j := start; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if strings.HasPrefix(trimmed, "## ") {
			end = j
			break
		}
	}
	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		return "", false
	}
	return body, true
}

// Package lifecycle contains pure functions for knowledge lifecycle operations:
// curation, deduplication, and defragmentation.
package lifecycle

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// ValidArtifactTypes enumerates the allowed artifact type values.
var ValidArtifactTypes = map[string]bool{
	"learning": true,
	"decision": true,
	"failure":  true,
	"pattern":  true,
}

// CurateArtifact represents a cataloged knowledge artifact.
type CurateArtifact struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Content       string `json:"content"`
	Date          string `json:"date"`
	SchemaVersion int    `json:"schema_version"`
	CuratedAt     string `json:"curated_at"`
	Path          string `json:"path"`
}

// CurateVerifyResult holds the output of a verify operation.
type CurateVerifyResult struct {
	Verified    bool     `json:"verified"`
	GatesPassed int      `json:"gates_passed"`
	GatesFailed int      `json:"gates_failed"`
	Regressions []string `json:"regressions"`
}

// CurateStatusResult holds the output of a status query.
type CurateStatusResult struct {
	Learnings     int    `json:"learnings"`
	Decisions     int    `json:"decisions"`
	Failures      int    `json:"failures"`
	Patterns      int    `json:"patterns"`
	Total         int    `json:"total"`
	LastCatalogAt string `json:"last_catalog_at,omitempty"`
	LastVerifyAt  string `json:"last_verify_at,omitempty"`
	PendingVerify int    `json:"pending_verify"`
}

// ParseFrontmatter extracts YAML frontmatter key-value pairs from a
// markdown document delimited by --- lines. Returns the frontmatter map and
// the body content below the closing delimiter.
//
// Parsing is delegated to wiki.FrontmatterCodec. On a miss (no frontmatter)
// the verbatim input is returned as the body, preserving the historical
// contract; on a parsed block the trimmed body is returned.
func ParseFrontmatter(data string) (map[string]any, string) {
	lines := strings.Split(data, "\n")
	if len(lines) < 3 {
		return map[string]any{}, data
	}
	doc := wiki.FrontmatterCodec{}.DecodeLines(lines)
	if !doc.HasFrontmatter && !documentHadDelimiters(lines) {
		return doc.Fields, data
	}
	return doc.Fields, doc.Body
}

// documentHadDelimiters reports whether lines open and close a well-formed
// --- frontmatter block, regardless of whether the YAML inside parsed. It
// distinguishes "no frontmatter" (return verbatim input) from "frontmatter
// present but invalid YAML" (return trimmed body).
func documentHadDelimiters(lines []string) bool {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return true
		}
	}
	return false
}

// FrontmatterString extracts a string value from a frontmatter map.
func FrontmatterString(fm map[string]any, key string) string {
	v, ok := fm[key]
	if !ok || v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case time.Time:
		return typed.UTC().Format("2006-01-02")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

// GenerateArtifactID creates a unique ID based on artifact type, date, and content hash.
func GenerateArtifactID(artifactType, date, content string) string {
	var prefix string
	switch artifactType {
	case "learning":
		prefix = "learn"
	case "decision":
		prefix = "decis"
	case "failure":
		prefix = "fail"
	case "pattern":
		prefix = "patt"
	}

	h := sha256.Sum256([]byte(content))
	shortHash := fmt.Sprintf("%x", h[:4])

	return fmt.Sprintf("%s-%s-%s", prefix, date, shortHash)
}

// ArtifactDir returns the target directory for the given artifact type.
func ArtifactDir(artifactType string) string {
	if artifactType == "pattern" {
		return ".agents/patterns"
	}
	return ".agents/learnings"
}

// ResolveCurateGoalsFile finds the first existing GOALS file.
func ResolveCurateGoalsFile() (string, error) {
	candidates := []string{"GOALS.md", "GOALS.yaml", "GOALS.yml"}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

// CountArtifactsInDir reads JSON artifacts from a directory and returns counts by type and the latest CuratedAt time.
func CountArtifactsInDir(dir string) (counts map[string]int, latest time.Time) {
	counts = make(map[string]int)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return counts, latest
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}
		var a CurateArtifact
		if json.Unmarshal(data, &a) != nil {
			continue
		}
		counts[a.Type]++
		if t, err := time.Parse(time.RFC3339, a.CuratedAt); err == nil {
			if t.After(latest) {
				latest = t
			}
		}
	}
	return counts, latest
}

// CountArtifactsSince counts artifacts in the given dirs with CuratedAt after the given time.
func CountArtifactsSince(learningsDir, patternsDir string, since time.Time) int {
	count := 0
	for _, dir := range []string{learningsDir, patternsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
			if readErr != nil {
				continue
			}
			var a CurateArtifact
			if json.Unmarshal(data, &a) != nil {
				continue
			}
			if t, err := time.Parse(time.RFC3339, a.CuratedAt); err == nil {
				if t.After(since) {
					count++
				}
			}
		}
	}
	return count
}

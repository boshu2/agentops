// practices: [ddd-bounded-context, wiki-knowledge-surface]
package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/adapters/vendorimage/codexruntime"
)

// Codex artifact/transcript helpers that the SPINE consumes. Extracted from
// codex.go so they remain in the default build even though the codex lifecycle
// COMMAND is archived behind //go:build legacy (ADR-0012, age-h4y3):
//   - collectRecentResearchArtifacts feeds ao's ranked-context bundle
//     (context_ranked_intel.go, context_relevance.go).
//   - findTranscriptBySessionID is used by ao's context surface (context.go).
// They depend only on stdlib + the codexruntime adapter + the untagged
// SectionResearch constant, so they are safe to keep in the spine.

// codexArtifactRef is a lightweight reference to a recent .agents artifact
// (research note, briefing) surfaced in the ranked-context bundle.
type codexArtifactRef struct {
	Title      string `json:"title"`
	Path       string `json:"path"`
	ModifiedAt string `json:"modified_at"`
}

func collectRecentResearchArtifacts(cwd, query string, limit int) []codexArtifactRef {
	return collectRecentCodexArtifacts(filepath.Join(cwd, ".agents", SectionResearch), query, limit)
}

func collectRecentCodexArtifacts(dir, query string, limit int) []codexArtifactRef {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type researchFile struct {
		path    string
		modTime time.Time
	}
	var files []researchFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, researchFile{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	slices.SortFunc(files, func(a, b researchFile) int {
		return b.modTime.Compare(a.modTime)
	})

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var artifacts []codexArtifactRef
	for _, file := range files {
		if queryLower != "" {
			baseLower := strings.ToLower(filepath.Base(file.path))
			if !strings.Contains(baseLower, queryLower) {
				data, err := os.ReadFile(file.path)
				if err != nil || !strings.Contains(strings.ToLower(string(data)), queryLower) {
					continue
				}
			}
		}
		artifacts = append(artifacts, codexArtifactRef{
			Title:      strings.TrimSuffix(filepath.Base(file.path), filepath.Ext(file.path)),
			Path:       file.path,
			ModifiedAt: file.modTime.UTC().Format(time.RFC3339),
		})
		if len(artifacts) >= limit {
			break
		}
	}
	return artifacts
}

func findTranscriptBySessionID(sessionID string) (string, error) {
	return codexruntime.FindTranscriptBySessionID(sessionID)
}

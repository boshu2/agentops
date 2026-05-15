package quality

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// CountFilesInDir counts .md, .jsonl, and .json files in a directory (non-recursive).
func CountFilesInDir(dir string) int {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0
	}
	count := 0
	mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	count += len(mdFiles)
	jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	count += len(jsonlFiles)
	jsonFiles, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	count += len(jsonFiles)
	return count
}

// ComputeHealthDelta computes the average age in days of active learnings.
func ComputeHealthDelta(baseDir string) float64 {
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		return 0
	}

	now := time.Now()
	var totalAge float64
	var count int

	_ = filepath.Walk(learningsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".jsonl") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		age := now.Sub(info.ModTime()).Hours() / 24.0
		totalAge += age
		count++
		return nil
	})

	if count == 0 {
		return 0
	}
	return totalAge / float64(count)
}

// CountConstraints counts constraint files in .agents/constraints/.
func CountConstraints(baseDir string) int {
	constraintsDir := filepath.Join(baseDir, ".agents", "constraints")
	if _, err := os.Stat(constraintsDir); os.IsNotExist(err) {
		return 0
	}
	count := 0
	mdFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.md"))
	count += len(mdFiles)
	yamlFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.yaml"))
	count += len(yamlFiles)
	jsonFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.json"))
	for _, f := range jsonFiles {
		if filepath.Base(f) != "index.json" {
			count++
		}
	}
	return count
}

// LastNSessions returns the last N unique session IDs from citations, ordered by recency.
func LastNSessions(citations []types.CitationEvent, n int) []string {
	sessionLatest := make(map[string]time.Time)
	for _, c := range citations {
		if c.SessionID == "" {
			continue
		}
		if t, ok := sessionLatest[c.SessionID]; !ok || c.CitedAt.After(t) {
			sessionLatest[c.SessionID] = c.CitedAt
		}
	}

	type sessionTime struct {
		id string
		t  time.Time
	}
	var sessions []sessionTime
	for id, t := range sessionLatest {
		sessions = append(sessions, sessionTime{id, t})
	}
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].t.After(sessions[i].t) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	limit := n
	if limit > len(sessions) {
		limit = len(sessions)
	}
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = sessions[i].id
	}
	return result
}

// CountUniqueSessions counts unique session IDs from citations.
func CountUniqueSessions(citations []types.CitationEvent) int {
	seen := make(map[string]bool)
	for _, c := range citations {
		if c.SessionID != "" {
			seen[c.SessionID] = true
		}
	}
	return len(seen)
}

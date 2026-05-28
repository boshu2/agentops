// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/search"
)

// Thin wrappers — canonical definitions in internal/search/sessions.go.
func collectSessionFiles(sessionsDir string) ([]string, error) {
	return search.CollectSessionFiles(sessionsDir)
}

// collectRecentSessions finds recent session summaries
func collectRecentSessions(cwd, query string, limit int) ([]session, error) {
	sessionsDir := filepath.Join(cwd, ".agents", "ao", SectionSessions)
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := collectSessionFiles(sessionsDir)
	if err != nil {
		return nil, err
	}

	sessions := make([]session, 0, len(files))
	queryLower := strings.ToLower(query)

	for _, file := range files {
		if len(sessions) >= limit {
			break
		}

		s, err := parseSessionFile(file)
		if err != nil || s.Summary == "" {
			continue
		}

		if query != "" && !strings.Contains(strings.ToLower(s.Summary), queryLower) {
			continue
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

func parseJSONLSessionSummary(path string) (string, error) {
	return search.ParseJSONLSessionSummary(path)
}
func parseMarkdownSessionSummary(path string) (string, error) {
	return search.ParseMarkdownSessionSummary(path)
}
func parseSessionFile(path string) (session, error) { return search.ParseSessionFile(path) }

package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	canonicalSessionPattern = regexp.MustCompile(`^session-\d{8}-\d{6}$`)
	timestampSessionPattern = regexp.MustCompile(`^\d{8}-\d{6}$`)
	uuidSessionPattern      = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
)

// canonicalSessionID normalizes session IDs to a stable canonical form.
// Determinism is required so injection, outcome, and feedback stages join reliably.
func canonicalSessionID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Sprintf("session-%s", time.Now().Format("20060102-150405"))
	}

	if canonicalSessionPattern.MatchString(trimmed) {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "session-") {
		return trimmed
	}
	if timestampSessionPattern.MatchString(trimmed) {
		return "session-" + trimmed
	}
	if uuidSessionPattern.MatchString(strings.ToLower(trimmed)) {
		// Deterministic UUID normalization (no timestamp-based randomness).
		return "session-uuid-" + strings.ToLower(trimmed)
	}
	return trimmed
}

// sessionIDAliases returns acceptable IDs for cross-version matching.
func sessionIDAliases(raw string) []string {
	aliases := make(map[string]struct{})
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		aliases[trimmed] = struct{}{}
	}

	canonical := canonicalSessionID(trimmed)
	if canonical != "" {
		aliases[canonical] = struct{}{}
	}

	if strings.HasPrefix(trimmed, "session-") {
		suffix := strings.TrimPrefix(trimmed, "session-")
		if timestampSessionPattern.MatchString(suffix) {
			aliases[suffix] = struct{}{}
		}
	}
	if timestampSessionPattern.MatchString(trimmed) {
		aliases["session-"+trimmed] = struct{}{}
	}

	result := make([]string, 0, len(aliases))
	for id := range aliases {
		result = append(result, id)
	}
	return result
}

// canonicalArtifactPath resolves an artifact path to a stable absolute form.
func canonicalArtifactPath(baseDir, artifactPath string) string {
	p := strings.TrimSpace(artifactPath)
	if p == "" {
		return ""
	}

	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		if strings.TrimSpace(baseDir) == "" {
			baseDir = "."
		}
		p = filepath.Join(baseDir, p)
	}

	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

func canonicalArtifactKey(baseDir, artifactPath string) string {
	return filepath.ToSlash(canonicalArtifactPath(baseDir, artifactPath))
}

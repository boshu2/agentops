package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CitationStatus string

const (
	CitationFresh   CitationStatus = "FRESH"
	CitationStale   CitationStatus = "STALE"
	CitationUnknown CitationStatus = "UNKNOWN"
)

type Citation struct {
	Kind     string         `json:"kind"`
	Raw      string         `json:"raw"`
	Status   CitationStatus `json:"status"`
	Reason   string         `json:"reason"`
	Resolved string         `json:"resolved"`
}

func verifyFileCitation(citation *Citation, cwd string) {
	path := citation.Raw
	if index := strings.LastIndex(path, ":"); index >= 0 {
		var line int
		if _, err := fmt.Sscanf(path[index+1:], "%d", &line); err == nil {
			path = path[:index]
		}
	}
	if _, err := os.Stat(filepath.Join(cwd, path)); err == nil {
		citation.Status, citation.Reason = CitationFresh, "file exists"
		return
	}
	if strings.Contains(path, "/") {
		citation.Status, citation.Reason = CitationStale, fmt.Sprintf("file %s not found", path)
		return
	}
	matches := findCitationFiles(cwd, path)
	switch len(matches) {
	case 0:
		citation.Status, citation.Reason = CitationStale, fmt.Sprintf("filename %q not found", path)
	case 1:
		citation.Status, citation.Reason, citation.Resolved = CitationFresh, "filename resolves uniquely", matches[0]
	default:
		citation.Status, citation.Reason = CitationUnknown, fmt.Sprintf("filename %q is ambiguous", path)
		citation.Resolved = strings.Join(matches[:min(3, len(matches))], ", ")
	}
}

func verifyFunctionCitation(citation *Citation, cwd string) {
	verifyTextCitation(citation, cwd, strings.TrimPrefix(citation.Raw, "func "), "function")
}

func verifySymbolCitation(citation *Citation, cwd string) {
	verifyTextCitation(citation, cwd, strings.Trim(citation.Raw, "`"), "symbol")
}

func verifyTextCitation(citation *Citation, cwd, needle, kind string) {
	if needle == "" {
		citation.Status, citation.Reason = CitationUnknown, "empty citation"
		return
	}
	var matches []string
	for _, root := range []string{"cli", "skills", "scripts", "docs"} {
		base := filepath.Join(cwd, root)
		_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if path != base && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if len(matches) >= 10 {
				return nil
			}
			data, readErr := os.ReadFile(path) // #nosec G122 -- path comes from this bounded repository walk
			if readErr == nil && strings.Contains(string(data), needle) {
				if relative, relErr := filepath.Rel(cwd, path); relErr == nil {
					matches = append(matches, relative)
				}
			}
			return nil
		})
	}
	if len(matches) == 0 {
		citation.Status, citation.Reason = CitationStale, fmt.Sprintf("%s %q not found", kind, needle)
		return
	}
	citation.Status = CitationFresh
	citation.Reason = fmt.Sprintf("%s found in %d file(s)", kind, len(matches))
	if len(matches) == 1 {
		citation.Resolved = matches[0]
	}
}

func findCitationFiles(cwd, name string) []string {
	var matches []string
	for _, root := range []string{"cli", "skills", "docs", "scripts"} {
		base := filepath.Join(cwd, root)
		_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if path != base && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Name() == name {
				if relative, relErr := filepath.Rel(cwd, path); relErr == nil {
					matches = append(matches, relative)
				}
			}
			return nil
		})
	}
	return matches
}

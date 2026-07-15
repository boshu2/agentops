package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

func walkKnowledgeFiles(dir string, extensions ...string) []string {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(extensions))
	for _, extension := range extensions {
		allowed[strings.ToLower(extension)] = struct{}{}
	}
	var files []string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		if _, ok := allowed[strings.ToLower(filepath.Ext(path))]; ok {
			files = append(files, path)
		}
		return nil
	})
	slices.Sort(files)
	return files
}

func templateCompletionValues() []string {
	values := append([]string(nil), validTemplateNames...)
	sort.Strings(values)
	return values
}

func extractLearningBody(path string) string {
	content, err := os.ReadFile(path) // #nosec G304 -- caller-selected local knowledge artifact
	if err != nil {
		return ""
	}
	text := string(content)
	if strings.HasSuffix(path, ".md") {
		lines := strings.Split(text, "\n")
		if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
			return text
		}
		for index := 1; index < len(lines); index++ {
			if strings.TrimSpace(lines[index]) == "---" {
				return strings.Join(lines[index+1:], "\n")
			}
		}
		return text
	}
	var value map[string]any
	first, _, _ := strings.Cut(text, "\n")
	if json.Unmarshal([]byte(first), &value) != nil {
		return ""
	}
	if content, ok := value["content"].(string); ok && content != "" {
		return content
	}
	title, _ := value["title"].(string)
	return title
}

func updateFrontMatterFields(lines []string, fields map[string]string) []string {
	result := append([]string(nil), lines...)
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		updated := false
		for index, line := range result {
			if strings.HasPrefix(line, key+":") {
				result[index] = fmt.Sprintf("%s: %s", key, fields[key])
				updated = true
				break
			}
		}
		if !updated {
			result = append(result, fmt.Sprintf("%s: %s", key, fields[key]))
		}
	}
	return result
}

func rebuildWithFrontMatter(frontmatter, body []string) string {
	return "---\n" + strings.Join(frontmatter, "\n") + "\n---\n" + strings.Join(body, "\n")
}

func expandFilePatterns(baseDir string, patterns []string) ([]string, error) {
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	var files []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(base, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			matches = []string{pattern}
		}
		for _, match := range matches {
			absolute, err := filepath.Abs(match)
			if err != nil {
				continue
			}
			relative, err := filepath.Rel(base, absolute)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("path %q is outside workspace", match)
			}
			info, err := os.Stat(absolute)
			if err != nil {
				continue
			}
			if info.IsDir() {
				entries, _ := os.ReadDir(absolute)
				for _, entry := range entries {
					if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".md") && !strings.HasSuffix(entry.Name(), ".jsonl")) {
						continue
					}
					path := filepath.Join(absolute, entry.Name())
					if !seen[path] {
						seen[path], files = true, append(files, path)
					}
				}
				continue
			}
			if !seen[absolute] {
				seen[absolute], files = true, append(files, absolute)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

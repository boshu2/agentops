package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// walkKnowledgeFiles returns matching artifact files under dir, including
// namespaced subdirectories used by global cross-repo knowledge stores.
func walkKnowledgeFiles(dir string, exts ...string) []string {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}

	allowed := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		allowed[strings.ToLower(ext)] = struct{}{}
	}

	var files []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
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

// practices: [fail-closed-safety, hexagonal-architecture]

package corpusscan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// scannedExts are the rendered-output extensions the directory walk inspects.
// The scanner is meant for RENDERED public text (markdown / json), not source.
var scannedExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".json":     true,
	".txt":      true,
	".html":     true,
}

// FileResult holds the scan outcome for a single file.
type FileResult struct {
	Path string `json:"path"`
	Hits []Hit  `json:"hits"`
	// Err is set when the file could not be read. Fail-closed: an unreadable
	// file is treated as UNSAFE (Clean() reports the whole report dirty).
	Err string `json:"error,omitempty"`
}

// Clean reports whether this file is publishable: no hits AND no read error.
func (r FileResult) Clean() bool { return len(r.Hits) == 0 && r.Err == "" }

// Report is the result of scanning a path (file or directory tree).
type Report struct {
	Root  string       `json:"root"`
	Files []FileResult `json:"files"`
}

// Clean reports whether every scanned file is publishable. Fail-closed: if any
// file has a hit OR could not be read, the report is NOT clean. An empty
// report (no files scanned) is reported as clean — the caller decides whether
// "nothing to scan" is acceptable; the scanner does not invent a violation.
func (r Report) Clean() bool {
	for _, f := range r.Files {
		if !f.Clean() {
			return false
		}
	}
	return true
}

// HitCount totals every hit across the report.
func (r Report) HitCount() int {
	n := 0
	for _, f := range r.Files {
		n += len(f.Hits)
	}
	return n
}

// ErrorCount totals files that could not be read (fail-closed failures).
func (r Report) ErrorCount() int {
	n := 0
	for _, f := range r.Files {
		if f.Err != "" {
			n++
		}
	}
	return n
}

// Scan inspects a file or directory tree at path for leak markers. For a
// directory it walks recursively, scanning only rendered-text extensions. For
// a single file it scans regardless of extension (the caller named it
// explicitly). The returned Report.Clean() is the fail-closed publish verdict.
func Scan(path string) (Report, error) {
	rep := Report{Root: path}
	info, err := os.Stat(path)
	if err != nil {
		return rep, fmt.Errorf("corpus scan: stat %s: %w", path, err)
	}

	if !info.IsDir() {
		rep.Files = append(rep.Files, scanFile(path))
		return rep, nil
	}

	walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Fail-closed: record the walk error against the path rather than
			// silently skipping it.
			rep.Files = append(rep.Files, FileResult{Path: p, Err: err.Error()})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !scannedExts[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		rep.Files = append(rep.Files, scanFile(p))
		return nil
	})
	if walkErr != nil {
		return rep, fmt.Errorf("corpus scan: walk %s: %w", path, walkErr)
	}

	sort.Slice(rep.Files, func(i, j int) bool { return rep.Files[i].Path < rep.Files[j].Path })
	return rep, nil
}

// scanFile reads one file and scans its text. A read error becomes a
// fail-closed FileResult.Err (the file is treated as unsafe). It never
// modifies the file.
func scanFile(path string) FileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileResult{Path: path, Err: err.Error()}
	}
	return FileResult{Path: path, Hits: ScanText(string(data))}
}

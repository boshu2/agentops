// practices: [fail-closed-safety, wiki-knowledge-surface]
package corpus

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ClassifyResult is the report from a ClassifyDir run.
type ClassifyResult struct {
	// Applied is true when changes were written to disk (false = dry run).
	Applied bool `json:"applied"`
	// Scanned counts learning records examined (meta docs excluded).
	Scanned int `json:"scanned"`
	// Changed counts records that were (or, in dry run, would be) annotated.
	Changed int `json:"changed"`
	// Skipped counts non-learning meta docs that were left alone.
	Skipped int `json:"skipped"`
	// ChangedFiles lists the relative paths that need / got annotation (sorted).
	ChangedFiles []string `json:"changed_files,omitempty"`
}

// ClassifyDir walks root for `.md` learning records and ensures each carries the
// two promote-gate frontmatter defaults (sensitivity, publishable). Meta/policy
// docs (CORPUS-POLICY.md, README.md, …) are skipped. With apply=false it only
// reports what would change; with apply=true it rewrites the changed files in
// place.
//
// It is malformed-tolerant by construction — AnnotateLearning never parses the
// YAML body — so a single junk record cannot abort the migration.
func ClassifyDir(root string, apply bool) (ClassifyResult, error) {
	res := ClassifyResult{Applied: apply}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip a nested .git dir defensively (the corpus is its own repo).
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		if !IsLearningFile(d.Name()) {
			res.Skipped++
			return nil
		}
		res.Scanned++
		raw, readErr := os.ReadFile(path) // #nosec G304 -- path from the corpus dir walk
		if readErr != nil {
			return readErr
		}
		out, changed := AnnotateLearning(string(raw))
		if !changed {
			return nil
		}
		res.Changed++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		res.ChangedFiles = append(res.ChangedFiles, rel)
		if apply {
			// Preserve the file mode; learnings are 0644.
			info, statErr := d.Info()
			mode := fs.FileMode(0o644)
			if statErr == nil {
				mode = info.Mode().Perm()
			}
			if writeErr := os.WriteFile(path, []byte(out), mode); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
	sort.Strings(res.ChangedFiles)
	return res, err
}

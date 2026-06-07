// practices: [wiki-knowledge-surface, design-by-contract]
package agentsurface

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Inventory is the catalog returned by the .agents surface commands.
type Inventory struct {
	Contract  string   `json:"contract"`
	Allowlist []string `json:"allowlist"`
	Skills    []string `json:"skills"`
}

// ParseAllowlist extracts the allowlist between the BEGIN/END markers in the
// contract doc. Inline `# comment` and blank lines are stripped. The result is
// sorted and de-duplicated.
func ParseAllowlist(content string) []string {
	const beginMarker = "<!-- BEGIN agents-write-surfaces-allowlist -->"
	const endMarker = "<!-- END agents-write-surfaces-allowlist -->"

	seen := make(map[string]bool)
	out := []string{}
	inside := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, beginMarker) {
			inside = true
			continue
		}
		if strings.Contains(line, endMarker) {
			inside = false
			continue
		}
		if !inside {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// DiscoverActiveSkills returns the names of skills/<name>/ entries that have a
// SKILL.md file. Result is sorted.
func DiscoverActiveSkills(skillsDir string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeprecatedCommands maps old namespace-qualified command references to their
// new flat replacements.
var DeprecatedCommands = map[string]string{
	"ao know forge":            "ao forge",
	"ao know inject":           "ao inject",
	"ao know search":           "ao search",
	"ao know lookup":           "ao lookup",
	"ao know trace":            "ao trace",
	"ao know store":            "ao store",
	"ao know index":            "ao index",
	"ao know temper":           "ao temper",
	"ao know feedback":         "ao feedback",
	"ao know migrate":          "ao migrate",
	"ao know batch-feedback":   "ao batch-feedback",
	"ao know session-outcome":  "ao session-outcome",
	"ao work rpi":              "ao rpi",
	"ao work ratchet":          "ao ratchet",
	"ao work goals":            "ao goals",
	"ao work session":          "ao session",
	"ao work feedback-loop":    "ao feedback-loop",
	"ao work context":          "ao context",
	"ao work task-sync":        "ao task-sync",
	"ao work task-feedback":    "ao task-feedback",
	"ao work task-status":      "ao task-status",
	"ao quality flywheel":      "ao flywheel",
	"ao quality pool":          "ao pool",
	"ao quality metrics":       "ao metrics",
	"ao quality gate":          "ao gate",
	"ao quality maturity":      "ao maturity",
	"ao quality constraint":    "ao constraint",
	"ao quality vibe-check":    "ao vibe-check",
	"ao quality badge":         "ao badge",
	"ao quality contradict":    "ao contradict",
	"ao quality dedup":         "ao dedup",
	"ao quality anti-patterns": "ao anti-patterns",
	"ao quality curate":        "ao curate",
	"ao settings config":       "ao config",
	"ao settings plans":        "ao plans",
	"ao settings hooks":        "ao hooks",
	"ao settings memory":       "ao memory",
	"ao settings notebook":     "ao notebook",
	"ao settings worktree":     "ao worktree",
	"ao start demo":            "ao demo",
	"ao start init":            "ao init",
	"ao start seed":            "ao seed",
	"ao start quick-start":     "ao quick-start",
}

// StaleReference records a single deprecated command reference found in a file.
type StaleReference struct {
	File       string `json:"file"`
	OldCommand string `json:"old_command"`
	NewCommand string `json:"new_command"`
}

// CheckStaleReferences scans hooks, skills, docs, and scripts for deprecated
// command references and reports them as warnings.
func CheckStaleReferences(globs []string) Check {
	var refs []StaleReference

	for _, pattern := range globs {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			found := ScanFileForDeprecatedCommands(f)
			refs = append(refs, found...)
		}
	}

	if len(refs) == 0 {
		return Check{
			Name:     "Stale References",
			Status:   "pass",
			Detail:   "No deprecated command references found",
			Required: false,
		}
	}

	seen := make(map[string]bool)
	for _, r := range refs {
		seen[r.OldCommand] = true
	}
	cmds := make([]string, 0, len(seen))
	for cmd := range seen {
		cmds = append(cmds, cmd)
	}

	detail := fmt.Sprintf("%d stale reference(s) in %d file(s)", len(refs), CountUniqueFiles(refs))
	if len(cmds) <= 3 {
		detail += fmt.Sprintf(" — update: %s", strings.Join(cmds, ", "))
	}

	return Check{
		Name:     "Stale References",
		Status:   "warn",
		Detail:   detail,
		Required: false,
	}
}

// ScanFileForDeprecatedCommands reads a file and checks each line for
// deprecated command patterns.
func ScanFileForDeprecatedCommands(path string) []StaleReference {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var refs []StaleReference
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if isRenameDocLine(line) {
			continue
		}
		for oldCmd, newCmd := range DeprecatedCommands {
			idx := strings.Index(line, oldCmd)
			if idx < 0 {
				continue
			}
			afterIdx := idx + len(oldCmd)
			if afterIdx < len(line) {
				ch := line[afterIdx]
				if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '-' {
					continue
				}
			}

			refs = append(refs, StaleReference{
				File:       path,
				OldCommand: oldCmd,
				NewCommand: newCmd,
			})
			break
		}
	}

	return refs
}

// isRenameDocLine reports whether a line is documenting a command rename
// (e.g. `ao old command` → `ao new command`). Such lines intentionally
// reference the deprecated command and should not count as stale usage.
func isRenameDocLine(line string) bool {
	return strings.Contains(line, "→") || strings.Contains(line, " -> ")
}

// CountUniqueFiles counts the number of distinct files in a slice of StaleReferences.
func CountUniqueFiles(refs []StaleReference) int {
	seen := make(map[string]bool)
	for _, r := range refs {
		seen[r.File] = true
	}
	return len(seen)
}

// CountHealFindings counts lines matching the heal.sh report format.
func CountHealFindings(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			count++
		}
	}
	if count == 0 {
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "finding(s) detected") {
				_, _ = fmt.Sscanf(strings.TrimSpace(line), "%d", &count)
				break
			}
		}
	}
	return count
}

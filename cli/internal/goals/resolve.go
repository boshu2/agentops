package goals

import "os"

// ResolveGoalsFile returns the goals file path to operate on. When explicit is
// non-empty it is returned unchanged; otherwise the working directory is probed
// for GOALS.md (v4) first and GOALS.yaml (v1-3) second, defaulting to GOALS.md
// for a brand-new project. This owns the filesystem probe the command module is
// forbidden to perform directly.
func ResolveGoalsFile(explicit string) string {
	if explicit != "" {
		return explicit
	}
	// Prefer GOALS.md (v4), fall back to GOALS.yaml.
	if info, err := os.Stat("GOALS.md"); err == nil && !info.IsDir() {
		return "GOALS.md"
	}
	if info, err := os.Stat("GOALS.yaml"); err == nil && !info.IsDir() {
		return "GOALS.yaml"
	}
	return "GOALS.md" // Default for new projects.
}

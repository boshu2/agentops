// Package harvest discovers and catalogs .agents/ directories across rigs.
package harvest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RigInfo describes a discovered .agents/ directory with provenance metadata.
type RigInfo struct {
	Path      string   `json:"path"`       // Absolute path to .agents/ directory
	Project   string   `json:"project"`    // e.g., "agentops"
	Crew      string   `json:"crew"`       // e.g., "nami"
	Rig       string   `json:"rig"`        // "{project}-{crew}" composite key
	FileCount int      `json:"file_count"` // Count of files in .agents/ (non-recursive, top-level entries)
	Subdirs   []string `json:"subdirs"`    // Names of subdirectories in .agents/
}

// WalkOptions configures the discovery walk.
type WalkOptions struct {
	Roots       []string // Base directories to scan (default: ~/gt/)
	MaxFileSize int64    // Skip files > this (default: 1MB = 1048576)
	SkipDirs    []string // Directory names to skip
	IncludeDirs []string // .agents/ subdirs to harvest (learnings, patterns, research)
}

// DefaultWalkOptions returns sensible defaults.
func DefaultWalkOptions() WalkOptions {
	home, _ := os.UserHomeDir()
	return WalkOptions{
		Roots:       []string{filepath.Join(home, "gt")},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive", ".tmp", "test-fixtures", "node_modules", "vendor", ".archive"},
		IncludeDirs: []string{"learnings", "patterns", "research"},
	}
}

// DiscoverRigs walks the configured roots and returns all discovered .agents/ directories.
func DiscoverRigs(opts WalkOptions) ([]RigInfo, error) {
	var rigs []RigInfo

	for _, root := range opts.Roots {
		// Nonexistent root is not an error -- return empty results.
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		found, err := walkRoot(root, opts)
		if err != nil {
			return nil, fmt.Errorf("walking root %s: %w", root, err)
		}
		rigs = append(rigs, found...)
	}

	// Include ~/.agents/ as global hub if it exists.
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".agents")
		if info, statErr := os.Stat(globalPath); statErr == nil && info.IsDir() {
			// Only add if not already discovered (e.g., if home was a root).
			if !containsPath(rigs, globalPath) {
				ri, inspectErr := inspectAgentsDir(globalPath, "global", "hub")
				if inspectErr == nil {
					rigs = append(rigs, ri)
				}
			}
		}
	}

	return rigs, nil
}

// walkRoot walks a single root directory for .agents/ directories.
func walkRoot(root string, opts WalkOptions) ([]RigInfo, error) {
	var rigs []RigInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission denied or other access error -- skip, continue.
			if os.IsPermission(err) {
				fmt.Fprintf(os.Stderr, "harvest: warning: permission denied: %s\n", path)
				return filepath.SkipDir
			}
			return err
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Skip configured directories.
		if isSkipDir(name, opts.SkipDirs) {
			return filepath.SkipDir
		}

		if name != ".agents" {
			return nil
		}

		// Check if any ancestor is a skip dir.
		if pathContainsSkipDir(path, opts.SkipDirs) {
			return filepath.SkipDir
		}

		project, crew := extractProvenance(root, path)
		ri, inspectErr := inspectAgentsDir(path, project, crew)
		if inspectErr != nil {
			fmt.Fprintf(os.Stderr, "harvest: warning: inspecting %s: %v\n", path, inspectErr)
			return filepath.SkipDir
		}

		rigs = append(rigs, ri)
		return filepath.SkipDir
	})

	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	return rigs, nil
}

// inspectAgentsDir builds a RigInfo from an .agents/ directory.
func inspectAgentsDir(agentsPath, project, crew string) (RigInfo, error) {
	entries, err := os.ReadDir(agentsPath)
	if err != nil {
		return RigInfo{}, fmt.Errorf("reading .agents dir: %w", err)
	}

	var subdirs []string
	fileCount := 0
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, e.Name())
		} else {
			fileCount++
		}
	}

	rig := project + "-" + crew

	return RigInfo{
		Path:      agentsPath,
		Project:   project,
		Crew:      crew,
		Rig:       rig,
		FileCount: fileCount,
		Subdirs:   subdirs,
	}, nil
}

// extractProvenance derives project and crew from the filesystem path.
// Pattern: {root}/{project}/crew/{crew}/.agents/ -> project, crew
// Pattern: {root}/{project}/.agents/ -> project, project
// Fallback: "unknown", "unknown"
func extractProvenance(root, agentsPath string) (string, string) {
	// Get relative path from root to .agents/.
	rel, err := filepath.Rel(root, agentsPath)
	if err != nil {
		return "unknown", "unknown"
	}

	parts := strings.Split(rel, string(filepath.Separator))
	// parts[-1] is always ".agents"

	// Pattern: {project}/crew/{crew}/.agents
	if len(parts) >= 4 && parts[len(parts)-3] == "crew" {
		project := parts[0]
		crew := parts[len(parts)-2]
		return project, crew
	}

	// Pattern: {project}/.agents
	if len(parts) >= 2 {
		project := parts[0]
		return project, project
	}

	return "unknown", "unknown"
}

// isSkipDir returns true if the directory name matches a skip entry.
func isSkipDir(name string, skipDirs []string) bool {
	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}
	return false
}

// pathContainsSkipDir checks if any path component is a skip directory.
func pathContainsSkipDir(path string, skipDirs []string) bool {
	for _, skip := range skipDirs {
		if strings.Contains(path, string(filepath.Separator)+skip+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// containsPath checks if any RigInfo in the slice has the given path.
func containsPath(rigs []RigInfo, path string) bool {
	for _, r := range rigs {
		if r.Path == path {
			return true
		}
	}
	return false
}

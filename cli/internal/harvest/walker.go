// Package harvest discovers and catalogs .agents/ directories across rigs.
package harvest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	// SkipGlobalHub, when true, disables the automatic ~/.agents/
	// include at the end of rig discovery. Used by private-lane
	// callers (Dream, corpus.Compute) that want strictly-scoped
	// local rig discovery.
	SkipGlobalHub bool
}

// DefaultWalkOptions returns sensible defaults.
func DefaultWalkOptions() WalkOptions {
	home, _ := os.UserHomeDir()
	return WalkOptions{
		Roots:       []string{filepath.Join(home, "gt")},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive", ".tmp", "test-fixtures", "node_modules", "vendor", ".archive", ".git"},
		IncludeDirs: []string{"learnings", "patterns", "research"},
	}
}

// DiscoverRigs walks the configured roots and returns all discovered .agents/ directories.
func DiscoverRigs(opts WalkOptions) ([]RigInfo, error) {
	rigs, _, err := DiscoverRigsWithWarnings(opts)
	return rigs, err
}

// DiscoverRigsWithWarnings walks the configured roots, returning discovered
// rigs plus non-fatal discovery warnings that callers can persist in the
// harvest catalog.
func DiscoverRigsWithWarnings(opts WalkOptions) ([]RigInfo, []HarvestWarning, error) {
	var rigs []RigInfo
	var warnings []HarvestWarning

	for _, root := range opts.Roots {
		// Nonexistent root is not an error -- return empty results.
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		found, rootWarnings, err := walkRoot(root, opts)
		if err != nil {
			return nil, warnings, fmt.Errorf("walking root %s: %w", root, err)
		}
		rigs = append(rigs, found...)
		warnings = append(warnings, rootWarnings...)
	}

	// Include ~/.agents/ as global hub if it exists, unless the caller
	// opted out (private-lane callers like Dream and corpus.Compute).
	if !opts.SkipGlobalHub {
		home, err := os.UserHomeDir()
		if err == nil {
			globalPath := filepath.Join(home, ".agents")
			if info, statErr := os.Stat(globalPath); statErr == nil && info.IsDir() {
				// Only add if not already discovered (e.g., if home was a root).
				if !containsPath(rigs, globalPath) {
					ri, inspectErr := inspectAgentsDir(globalPath, "global", "hub")
					if inspectErr == nil {
						rigs = append(rigs, ri)
					} else {
						warnings = append(warnings, newDiscoveryWarning(
							"discover_inspect",
							globalPath,
							fmt.Errorf("inspecting %s: %w", globalPath, inspectErr),
						))
					}
				}
			}
		}
	}

	return rigs, warnings, nil
}

// walkRoot walks a single root directory for .agents/ directories.
func walkRoot(root string, opts WalkOptions) ([]RigInfo, []HarvestWarning, error) {
	var rigs []RigInfo
	var warnings []HarvestWarning

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission denied or other access error -- skip, continue.
			if os.IsPermission(err) {
				warnings = append(warnings, newDiscoveryWarning(
					"discover_permission",
					path,
					fmt.Errorf("permission denied at %s: %w", path, err),
				))
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
			warnings = append(warnings, newDiscoveryWarning(
				"discover_inspect",
				path,
				fmt.Errorf("inspecting %s: %w", path, inspectErr),
			))
			return filepath.SkipDir
		}

		rigs = append(rigs, ri)
		return filepath.SkipDir
	})

	if err != nil {
		return nil, warnings, fmt.Errorf("walking %s: %w", root, err)
	}
	return rigs, warnings, nil
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

// extractProvenance derives project and crew. Resolution order:
//  1. .agents/.config file (project: foo, crew: bar)
//  2. Path patterns: {root}/{project}/crew/{crew}/.agents and {root}/{project}/.agents
//  3. Path-prefix fallback: e.g. /mnt/c/Users/X/.agents -> ("mnt-c", "x"),
//     /home/boful/.agents -> ("home-boful", "root").
//
// Returns "unknown", "unknown" only as a last resort when no signal is recoverable.
func extractProvenance(root, agentsPath string) (string, string) {
	if proj, crew, ok := readAgentsConfig(agentsPath); ok {
		return proj, crew
	}

	rel, err := filepath.Rel(root, agentsPath)
	if err == nil && rel != "." {
		parts := strings.Split(rel, string(filepath.Separator))

		// Pattern: {project}/crew/{crew}/.agents
		if len(parts) >= 4 && parts[len(parts)-3] == "crew" {
			return parts[0], parts[len(parts)-2]
		}

		// Pattern: {project}/.agents
		if len(parts) >= 2 && parts[0] != "" {
			return parts[0], parts[0]
		}
	}

	// Root IS the .agents dir (or path-rel returned ".") — derive from absolute path.
	return derivePathPrefixProvenance(agentsPath)
}

// readAgentsConfig reads project/crew from <agentsPath>/.config if present.
// Format is line-based "key: value" so the file stays trivial to author by hand.
// Lines beginning with "#" are comments. Returns ok=false when no .config exists
// or no usable project key was found.
func readAgentsConfig(agentsPath string) (string, string, bool) {
	data, err := os.ReadFile(filepath.Join(agentsPath, ".config"))
	if err != nil {
		return "", "", false
	}
	var project, crew string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch key {
		case "project":
			project = val
		case "crew":
			crew = val
		}
	}
	if project == "" {
		return "", "", false
	}
	if crew == "" {
		crew = project
	}
	return project, crew, true
}

// derivePathPrefixProvenance returns a sensible (project, crew) pair from the
// absolute path of an .agents/ directory. Used when neither .agents/.config
// nor the {root}/{project}/[crew/{crew}/].agents/ pattern resolves the rig.
//
// Conventions:
//
//	/mnt/<drive>/<...>/.agents          -> ("mnt-<drive>", "<...leaf>" or "root")
//	/home/<user>/.agents                -> ("home-<user>", "root")
//	/home/<user>/<a>/.../.agents        -> ("home-<user>", "<...leaf>")
//	/<top>/<...>/.agents                -> ("<top>", "<...leaf>" or "<top>")
var rigSanitizeRe = regexp.MustCompile(`[^a-z0-9-]`)

func derivePathPrefixProvenance(agentsPath string) (string, string) {
	abs, err := filepath.Abs(agentsPath)
	if err != nil {
		abs = agentsPath
	}
	abs = filepath.Clean(abs)

	parts := strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator))
	// Anchor on the parent of .agents so the rig describes the containing tree.
	if len(parts) > 0 && parts[len(parts)-1] == ".agents" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return "unknown", "unknown"
	}

	var project, crew string
	switch parts[0] {
	case "mnt":
		if len(parts) >= 2 {
			project = "mnt-" + parts[1]
			if len(parts) >= 3 {
				crew = parts[len(parts)-1]
			} else {
				crew = "root"
			}
		} else {
			project = "mnt"
			crew = "root"
		}
	case "home":
		if len(parts) >= 2 {
			project = "home-" + parts[1]
			if len(parts) >= 3 {
				crew = parts[len(parts)-1]
			} else {
				crew = "root"
			}
		} else {
			project = "home"
			crew = "root"
		}
	default:
		project = parts[0]
		if len(parts) >= 2 {
			crew = parts[len(parts)-1]
		} else {
			crew = parts[0]
		}
	}
	return sanitizeRigToken(project), sanitizeRigToken(crew)
}

func sanitizeRigToken(s string) string {
	s = strings.ToLower(s)
	s = rigSanitizeRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "unknown"
	}
	return s
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

func newDiscoveryWarning(stage, path string, err error) HarvestWarning {
	return HarvestWarning{
		Path:    path,
		Stage:   stage,
		Message: err.Error(),
	}
}

package ratchet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocationType identifies where an artifact was found.
type LocationType string

const (
	LocationCrew    LocationType = "crew"    // Current directory .agents/
	LocationRig     LocationType = "rig"     // Parent rig .agents/
	LocationTown    LocationType = "town"    // ~/gt/.agents/
	LocationPlugins LocationType = "plugins" // plugins/*/
)

// SearchOrder defines the priority of search locations.
// Most specific (crew) → most general (plugins).
var SearchOrder = []LocationType{
	LocationCrew,
	LocationRig,
	LocationTown,
	LocationPlugins,
}

// Locator provides multi-location artifact search.
type Locator struct {
	// startDir is the directory to start searching from.
	startDir string

	// home is the user's home directory.
	home string

	// townDir is the Gas Town root (~/gt).
	townDir string
}

// NewLocator creates a new artifact locator.
func NewLocator(startDir string) (*Locator, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		absStart = startDir
	}

	return &Locator{
		startDir: absStart,
		home:     home,
		townDir:  filepath.Join(home, "gt"),
	}, nil
}

// Find searches for artifacts matching the pattern across all locations.
// Returns matches in priority order (crew first, then rig, town, plugins).
func (l *Locator) Find(pattern string) (*FindResult, error) {
	result := &FindResult{
		Pattern:  pattern,
		Matches:  []FindMatch{},
		Warnings: []string{},
	}

	seen := make(map[string]FindMatch) // Track by basename for duplicate detection

	for priority, loc := range SearchOrder {
		paths, err := l.searchLocation(loc, pattern)
		if err != nil {
			continue // Skip locations that fail
		}

		for _, p := range paths {
			baseName := filepath.Base(p)
			match := FindMatch{
				Path:     p,
				Location: string(loc),
				Priority: priority,
			}

			// Check for duplicates
			if existing, ok := seen[baseName]; ok {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Duplicate '%s' found at %s (priority: %s) and %s (priority: %s)",
						baseName, existing.Path, existing.Location, p, loc))
			} else {
				seen[baseName] = match
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}

// FindFirst finds the first matching artifact (highest priority).
func (l *Locator) FindFirst(pattern string) (string, LocationType, error) {
	for _, loc := range SearchOrder {
		paths, err := l.searchLocation(loc, pattern)
		if err != nil {
			continue
		}
		if len(paths) > 0 {
			return paths[0], loc, nil
		}
	}
	return "", "", fmt.Errorf("artifact not found: %s", pattern)
}

// searchLocation searches a specific location for the pattern.
func (l *Locator) searchLocation(loc LocationType, pattern string) ([]string, error) {
	searchRoot, err := l.resolveSearchRoot(loc)
	if err != nil {
		return nil, err
	}

	// Check if search root exists
	if _, err := os.Stat(searchRoot); os.IsNotExist(err) {
		return nil, fmt.Errorf("location not found: %s", searchRoot)
	}

	return l.glob(searchRoot, pattern)
}

// resolveSearchRoot determines the filesystem path for a given location type.
func (l *Locator) resolveSearchRoot(loc LocationType) (string, error) {
	switch loc {
	case LocationCrew:
		return filepath.Join(l.startDir, ".agents"), nil
	case LocationRig:
		rigDir := l.findRigRoot()
		if rigDir == "" {
			return "", ErrNoRigRoot
		}
		return filepath.Join(rigDir, ".agents"), nil
	case LocationTown:
		return filepath.Join(l.townDir, ".agents"), nil
	case LocationPlugins:
		return l.resolvePluginsRoot(), nil
	default:
		return "", fmt.Errorf("unknown location: %s", loc)
	}
}

// resolvePluginsRoot finds the plugins directory, trying the current dir first
// then falling back to the rig root.
func (l *Locator) resolvePluginsRoot() string {
	path := filepath.Join(l.startDir, "plugins")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if rigDir := l.findRigRoot(); rigDir != "" {
			return filepath.Join(rigDir, "plugins")
		}
	}
	return path
}

// glob performs pattern matching in the search root.
func (l *Locator) glob(searchRoot, pattern string) ([]string, error) {
	// Handle different pattern types
	if filepath.IsAbs(pattern) {
		// Absolute path - just check if it exists
		if _, err := os.Stat(pattern); err == nil {
			return []string{pattern}, nil
		}
		return nil, nil
	}

	// Relative pattern - search in the root
	fullPattern := filepath.Join(searchRoot, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

// findRigRoot walks up looking for the rig root.
// A rig root is identified by having a .beads/ or crew/ directory.
func (l *Locator) findRigRoot() string {
	dir := l.startDir
	for {
		// Check for rig markers
		if l.isRigRoot(dir) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isRigRoot checks if a directory is a rig root.
func (l *Locator) isRigRoot(dir string) bool {
	// Check for common rig markers
	markers := []string{".beads", "crew", "polecats"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// LocationPaths returns the paths that would be searched for each location.
func (l *Locator) LocationPaths() map[LocationType]string {
	paths := make(map[LocationType]string)

	paths[LocationCrew] = filepath.Join(l.startDir, ".agents")

	if rigDir := l.findRigRoot(); rigDir != "" {
		paths[LocationRig] = filepath.Join(rigDir, ".agents")
	}

	paths[LocationTown] = filepath.Join(l.townDir, ".agents")

	pluginsDir := filepath.Join(l.startDir, "plugins")
	if _, err := os.Stat(pluginsDir); err == nil {
		paths[LocationPlugins] = pluginsDir
	} else if rigDir := l.findRigRoot(); rigDir != "" {
		paths[LocationPlugins] = filepath.Join(rigDir, "plugins")
	}

	return paths
}

// ResolveArtifactPath resolves an artifact reference to its full path.
// Supports formats like:
//   - "research/topic.md" - relative to .agents/
//   - ".agents/research/topic.md" - relative path with .agents/
//   - "/abs/path/to/file.md" - absolute path
//   - "*.md" - glob pattern
func (l *Locator) ResolveArtifactPath(ref string) (string, LocationType, error) {
	// Absolute path
	if filepath.IsAbs(ref) {
		if _, err := os.Stat(ref); err == nil {
			return ref, l.locationForPath(ref), nil
		}
		return "", "", fmt.Errorf("artifact not found: %s", ref)
	}

	// Contains glob pattern
	if strings.ContainsAny(ref, "*?[]") {
		return l.FindFirst(ref)
	}

	// Try as-is first (might include .agents/)
	ref = strings.TrimPrefix(ref, ".agents/")

	// Search in order
	return l.FindFirst(ref)
}

// locationForPath determines the location type for an absolute path.
func (l *Locator) locationForPath(path string) LocationType {
	if strings.HasPrefix(path, l.townDir) {
		if strings.Contains(path, "/crew/") {
			return LocationCrew
		}
		if !strings.HasPrefix(path, filepath.Join(l.townDir, ".agents")) {
			// Could be a rig
			return LocationRig
		}
		return LocationTown
	}

	if strings.Contains(path, "plugins/") {
		return LocationPlugins
	}

	// Default to crew for local paths
	return LocationCrew
}

// ArtifactExists checks if an artifact exists in any location.
func (l *Locator) ArtifactExists(pattern string) bool {
	_, _, err := l.FindFirst(pattern)
	return err == nil
}

// AgentsDir returns the .agents directory for the specified location.
func (l *Locator) AgentsDir(loc LocationType) (string, error) {
	paths := l.LocationPaths()
	if path, ok := paths[loc]; ok {
		return path, nil
	}
	return "", fmt.Errorf("location %s not available", loc)
}

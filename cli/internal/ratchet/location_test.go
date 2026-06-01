package ratchet

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchOrder(t *testing.T) {
	// Verify search order is crew → rig → town → plugins
	expected := []LocationType{
		LocationCrew,
		LocationRig,
		LocationTown,
		LocationPlugins,
	}

	if len(SearchOrder) != len(expected) {
		t.Errorf("SearchOrder has %d elements, want %d", len(SearchOrder), len(expected))
	}

	for i, loc := range expected {
		if SearchOrder[i] != loc {
			t.Errorf("SearchOrder[%d] = %s, want %s", i, SearchOrder[i], loc)
		}
	}
}

func TestNewLocator(t *testing.T) {
	tmpDir := t.TempDir()

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	if loc.startDir != tmpDir {
		t.Errorf("startDir = %s, want %s", loc.startDir, tmpDir)
	}

	if loc.home == "" {
		t.Error("home directory should not be empty")
	}

	if loc.townDir == "" {
		t.Error("townDir should not be empty")
	}
}

func TestLocator_FindFirst(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, ".agents", "research")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(agentsDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	// Should find the file
	path, locType, err := loc.FindFirst("research/test.md")
	if err != nil {
		t.Errorf("FindFirst failed: %v", err)
	}

	if path != testFile {
		t.Errorf("FindFirst path = %s, want %s", path, testFile)
	}

	if locType != LocationCrew {
		t.Errorf("FindFirst location = %s, want %s", locType, LocationCrew)
	}
}

func TestResolveSearchRoot_NoRigRoot(t *testing.T) {
	// Use a temp dir with no rig markers (.beads, crew, polecats).
	// resolveSearchRoot(LocationRig) should return ErrNoRigRoot.
	tmp := t.TempDir()
	loc, err := NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	_, err = loc.resolveSearchRoot(LocationRig)
	if err == nil {
		t.Fatal("expected error when no rig root exists")
	}
	if !errors.Is(err, ErrNoRigRoot) {
		t.Errorf("expected ErrNoRigRoot, got %v", err)
	}
}

func TestLocator_FindWithDuplicateWarning(t *testing.T) {
	// Create a temp directory structure simulating crew + rig
	tmpDir := t.TempDir()

	// Create "crew" level .agents
	crewAgents := filepath.Join(tmpDir, ".agents", "research")
	if err := os.MkdirAll(crewAgents, 0755); err != nil {
		t.Fatalf("Failed to create crew agents: %v", err)
	}
	crewFile := filepath.Join(crewAgents, "shared.md")
	if err := os.WriteFile(crewFile, []byte("# Crew"), 0644); err != nil {
		t.Fatalf("Failed to create crew file: %v", err)
	}

	// Create a rig marker so we can have rig-level .agents
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	result, err := loc.Find("research/*.md")
	if err != nil {
		t.Errorf("Find failed: %v", err)
	}

	if len(result.Matches) == 0 {
		t.Error("Find should return at least one match")
	}

	// Verify pattern is set
	if result.Pattern != "research/*.md" {
		t.Errorf("Pattern = %s, want %s", result.Pattern, "research/*.md")
	}
}

func TestLocator_ArtifactExists(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(agentsDir, "exists.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	// File exists
	if !loc.ArtifactExists("exists.md") {
		t.Error("ArtifactExists should return true for existing file")
	}

	// File doesn't exist
	if loc.ArtifactExists("nonexistent.md") {
		t.Error("ArtifactExists should return false for non-existing file")
	}
}

func TestLocator_ResolveArtifactPath(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, ".agents", "research")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	testFile := filepath.Join(agentsDir, "topic.md")
	if err := os.WriteFile(testFile, []byte("# Topic"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{"relative path", "research/topic.md", false},
		{"with .agents/ prefix", ".agents/research/topic.md", false},
		{"absolute path", testFile, false},
		{"glob pattern", "research/*.md", false},
		{"nonexistent", "nonexistent.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, locType, err := loc.ResolveArtifactPath(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveArtifactPath(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if path == "" {
					t.Errorf("ResolveArtifactPath(%q) returned empty path", tt.ref)
				}
				if locType == "" {
					t.Errorf("ResolveArtifactPath(%q) returned empty location", tt.ref)
				}
			}
		})
	}
}

func TestLocator_GetLocationPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a rig marker
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Create plugins dir
	if err := os.MkdirAll(filepath.Join(tmpDir, "plugins"), 0755); err != nil {
		t.Fatalf("Failed to create plugins dir: %v", err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}

	paths := loc.LocationPaths()

	// Should have crew path
	if _, ok := paths[LocationCrew]; !ok {
		t.Error("GetLocationPaths should include crew")
	}

	// Should have plugins path (since we created it)
	if _, ok := paths[LocationPlugins]; !ok {
		t.Error("GetLocationPaths should include plugins")
	}

	// Should have town path
	if _, ok := paths[LocationTown]; !ok {
		t.Error("GetLocationPaths should include town")
	}
}

func TestLocationTypeValues(t *testing.T) {
	// Verify location type string values
	if LocationCrew != "crew" {
		t.Errorf("LocationCrew = %s, want crew", LocationCrew)
	}
	if LocationRig != "rig" {
		t.Errorf("LocationRig = %s, want rig", LocationRig)
	}
	if LocationTown != "town" {
		t.Errorf("LocationTown = %s, want town", LocationTown)
	}
	if LocationPlugins != "plugins" {
		t.Errorf("LocationPlugins = %s, want plugins", LocationPlugins)
	}
}

func TestGetAgentsDir(t *testing.T) {
	loc, err := NewLocator(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Valid location should return a path
	crewDir, err := loc.AgentsDir(LocationCrew)
	if err != nil {
		t.Fatalf("GetAgentsDir(crew) error: %v", err)
	}
	if crewDir == "" {
		t.Error("expected non-empty crew agents dir")
	}

	// Invalid location should return error
	_, err = loc.AgentsDir(LocationType("invalid"))
	if err == nil {
		t.Error("expected error for invalid location type")
	}
}

func TestLocationForPath(t *testing.T) {
	tmpDir := t.TempDir()
	loc := &Locator{
		startDir: tmpDir,
		home:     filepath.Dir(tmpDir),
		townDir:  filepath.Join(filepath.Dir(tmpDir), "gt"),
	}

	tests := []struct {
		name string
		path string
		want LocationType
	}{
		{"crew path under town with /crew/", filepath.Join(loc.townDir, "crew", "test", "file.md"), LocationCrew},
		{"town .agents path", filepath.Join(loc.townDir, ".agents", "learnings", "test.md"), LocationTown},
		{"rig path under town", filepath.Join(loc.townDir, "myrig", "file.md"), LocationRig},
		{"plugins path", "/some/plugins/myplugin/file.md", LocationPlugins},
		{"default to crew", "/random/path/file.md", LocationCrew},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loc.locationForPath(tt.path)
			if got != tt.want {
				t.Errorf("locationForPath(%q) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}

func TestGlobAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	loc := &Locator{startDir: tmpDir}

	// Create a file
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Absolute path that exists
	results, err := loc.glob(tmpDir, testFile)
	if err != nil {
		t.Fatalf("glob absolute path: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for existing absolute path, got %d", len(results))
	}

	// Absolute path that doesn't exist
	results, err = loc.glob(tmpDir, "/nonexistent/file.md")
	if err != nil {
		t.Fatalf("glob nonexistent: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent absolute path, got %d", len(results))
	}
}

func TestResolveArtifactPath(t *testing.T) {
	tmpDir := t.TempDir()
	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a test artifact
	agentsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(agentsDir, "test-learning.md")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Resolve by pattern
	resolved, locType, err := loc.ResolveArtifactPath("learnings/test-learning.md")
	if err != nil {
		t.Fatalf("ResolveArtifactPath: %v", err)
	}
	if resolved == "" {
		t.Error("expected resolved path")
	}
	if locType == "" {
		t.Error("expected location type")
	}

	// Resolve nonexistent
	_, _, err = loc.ResolveArtifactPath("nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent artifact")
	}
}

func TestResolveArtifactPath_AbsoluteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = loc.ResolveArtifactPath("/nonexistent/absolute/path.md")
	if err == nil {
		t.Error("expected error for nonexistent absolute path")
	}
}

func TestSearchLocation_UnknownLocationType(t *testing.T) {
	tmpDir := t.TempDir()
	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = loc.searchLocation(LocationType("bogus"), "*.md")
	if err == nil {
		t.Error("expected error for unknown location type")
	}
}

func TestFindFirst_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .agents so crew search doesn't fail entirely
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = loc.FindFirst("nonexistent-pattern-*.md")
	if err == nil {
		t.Error("expected error when no artifact matches pattern")
	}
}

func TestNewLocator_UserHomeDirError(t *testing.T) {
	t.Setenv("HOME", "")

	loc, err := NewLocator(t.TempDir())
	if err == nil {
		t.Fatal("expected error when HOME is unset")
	}
	if loc != nil {
		t.Error("expected nil locator on error")
	}
	if !strings.Contains(err.Error(), "get home directory") {
		t.Errorf("expected 'get home directory' in error, got: %v", err)
	}
}

func TestGetLocationPaths_NoPluginsNoRig(t *testing.T) {
	// No .beads, no crew, no plugins dir
	tmpDir := t.TempDir()
	loc, err := NewLocator(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	paths := loc.LocationPaths()
	// Should always have crew and town
	if _, ok := paths[LocationCrew]; !ok {
		t.Error("expected crew path")
	}
	if _, ok := paths[LocationTown]; !ok {
		t.Error("expected town path")
	}
}

func TestNewLocator_RelativeDirResolvesAbsolute(t *testing.T) {
	loc, err := NewLocator("relative/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("NewLocator failed: %v", err)
	}
	if !filepath.IsAbs(loc.startDir) {
		t.Fatalf("expected absolute startDir, got: %s", loc.startDir)
	}
}

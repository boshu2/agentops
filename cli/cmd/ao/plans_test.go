package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"strings"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

func TestComputePlanChecksum(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "plans_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	content := "# Plan Content\n\nThis is test content."
	path := filepath.Join(tmpDir, "test-plan.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test checksum computation
	t.Run("valid file", func(t *testing.T) {
		checksum, err := computePlanChecksum(path)
		if err != nil {
			t.Errorf("computePlanChecksum() error = %v", err)
		}
		if len(checksum) != 16 { // 8 bytes = 16 hex chars
			t.Errorf("checksum length = %d, want 16", len(checksum))
		}
	})

	// Test same content = same checksum
	t.Run("deterministic", func(t *testing.T) {
		cs1, _ := computePlanChecksum(path)
		cs2, _ := computePlanChecksum(path)
		if cs1 != cs2 {
			t.Errorf("checksums differ for same file: %s vs %s", cs1, cs2)
		}
	})

	// Test nonexistent file
	t.Run("nonexistent file", func(t *testing.T) {
		_, err := computePlanChecksum(filepath.Join(tmpDir, "nonexistent.md"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestCreatePlanEntry(t *testing.T) {
	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	entry := createPlanEntry(
		"/path/to/plan.md",
		modTime,
		"/project/path",
		"my-plan",
		"ol-123",
		"abc123",
	)

	if entry.Path != "/path/to/plan.md" {
		t.Errorf("Path = %q, want %q", entry.Path, "/path/to/plan.md")
	}
	if entry.CreatedAt != modTime {
		t.Errorf("CreatedAt = %v, want %v", entry.CreatedAt, modTime)
	}
	if entry.ProjectPath != "/project/path" {
		t.Errorf("ProjectPath = %q, want %q", entry.ProjectPath, "/project/path")
	}
	if entry.PlanName != "my-plan" {
		t.Errorf("PlanName = %q, want %q", entry.PlanName, "my-plan")
	}
	if entry.BeadsID != "ol-123" {
		t.Errorf("BeadsID = %q, want %q", entry.BeadsID, "ol-123")
	}
	if entry.Checksum != "abc123" {
		t.Errorf("Checksum = %q, want %q", entry.Checksum, "abc123")
	}
	if entry.Status != types.PlanStatusActive {
		t.Errorf("Status = %v, want %v", entry.Status, types.PlanStatusActive)
	}
}

func TestBuildBeadsIDIndex(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{Path: "/a.md", BeadsID: "ol-001"},
		{Path: "/b.md", BeadsID: "ol-002"},
		{Path: "/c.md", BeadsID: ""}, // No beads ID
		{Path: "/d.md", BeadsID: "ol-003"},
	}

	index := buildBeadsIDIndex(entries)

	if len(index) != 3 {
		t.Errorf("index length = %d, want 3", len(index))
	}
	if index["ol-001"] != 0 {
		t.Errorf("index[ol-001] = %d, want 0", index["ol-001"])
	}
	if index["ol-002"] != 1 {
		t.Errorf("index[ol-002] = %d, want 1", index["ol-002"])
	}
	if index["ol-003"] != 3 {
		t.Errorf("index[ol-003] = %d, want 3", index["ol-003"])
	}
	if _, ok := index[""]; ok {
		t.Error("empty beads ID should not be indexed")
	}
}

func TestSyncEpicStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      types.PlanStatus
		beadsStatus string
		wantChanged bool
		wantStatus  types.PlanStatus
	}{
		{
			name:        "active to completed",
			status:      types.PlanStatusActive,
			beadsStatus: "closed",
			wantChanged: true,
			wantStatus:  types.PlanStatusCompleted,
		},
		{
			name:        "completed to active",
			status:      types.PlanStatusCompleted,
			beadsStatus: "open",
			wantChanged: true,
			wantStatus:  types.PlanStatusActive,
		},
		{
			name:        "no change active",
			status:      types.PlanStatusActive,
			beadsStatus: "open",
			wantChanged: false,
			wantStatus:  types.PlanStatusActive,
		},
		{
			name:        "no change completed",
			status:      types.PlanStatusCompleted,
			beadsStatus: "closed",
			wantChanged: false,
			wantStatus:  types.PlanStatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []types.PlanManifestEntry{
				{Status: tt.status},
			}
			changed := syncEpicStatus(entries, 0, tt.beadsStatus)
			if changed != tt.wantChanged {
				t.Errorf("syncEpicStatus() changed = %v, want %v", changed, tt.wantChanged)
			}
			if entries[0].Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", entries[0].Status, tt.wantStatus)
			}
		})
	}
}

func TestCountUnlinkedEntries(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "plan-1", BeadsID: "ol-001"},
		{PlanName: "plan-2", BeadsID: ""},
		{PlanName: "plan-3", BeadsID: "ol-003"},
		{PlanName: "plan-4", BeadsID: ""},
	}

	// Note: countUnlinkedEntries also calls VerbosePrintf, which is fine in tests
	count := countUnlinkedEntries(entries)
	if count != 2 {
		t.Errorf("countUnlinkedEntries() = %d, want 2", count)
	}
}

func TestAppendManifestEntry(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.jsonl")

	entry := types.PlanManifestEntry{
		Path:     "/plan1.md",
		PlanName: "plan1",
		Status:   types.PlanStatusActive,
	}

	t.Run("creates new file", func(t *testing.T) {
		if err := appendManifestEntry(manifestPath, entry); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, _ := os.ReadFile(manifestPath)
		if len(content) == 0 {
			t.Error("expected non-empty file")
		}
	})

	t.Run("appends to existing", func(t *testing.T) {
		entry2 := types.PlanManifestEntry{
			Path:     "/plan2.md",
			PlanName: "plan2",
			Status:   types.PlanStatusActive,
		}
		if err := appendManifestEntry(manifestPath, entry2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, err := loadManifest(manifestPath)
		if err != nil {
			t.Fatalf("loadManifest error: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries after append, got %d", len(entries))
		}
	})
}

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := loadManifest(filepath.Join(tmpDir, "nope.jsonl"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		emptyPath := filepath.Join(tmpDir, "empty.jsonl")
		if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		entries, err := loadManifest(emptyPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("got %d entries from empty file, want 0", len(entries))
		}
	})

	t.Run("skips invalid lines", func(t *testing.T) {
		mixedPath := filepath.Join(tmpDir, "mixed.jsonl")
		entry := types.PlanManifestEntry{Path: "/valid.md", PlanName: "valid"}
		line, _ := json.Marshal(entry)
		content := string(line) + "\nnot json\n" + string(line) + "\n"
		if err := os.WriteFile(mixedPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		entries, err := loadManifest(mixedPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("got %d entries, want 2 (invalid line skipped)", len(entries))
		}
	})
}

func TestSaveManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "save.jsonl")

	entries := []types.PlanManifestEntry{
		{Path: "/a.md", PlanName: "a", Status: types.PlanStatusActive},
		{Path: "/b.md", PlanName: "b", Status: types.PlanStatusCompleted},
	}

	if err := saveManifest(manifestPath, entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify round-trip
	loaded, err := loadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadManifest error: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("got %d entries after save/load, want 2", len(loaded))
	}
	if loaded[0].PlanName != "a" {
		t.Errorf("first entry name = %q, want %q", loaded[0].PlanName, "a")
	}
}

func TestBuildBeadsStatusIndex(t *testing.T) {
	epics := []beadsEpic{
		{ID: "ol-001", Status: "open"},
		{ID: "ol-002", Status: "closed"},
		{ID: "ol-003", Status: "open"},
	}

	index := buildBeadsStatusIndex(epics)

	if len(index) != 3 {
		t.Errorf("index length = %d, want 3", len(index))
	}
	if index["ol-001"] != "open" {
		t.Errorf("index[ol-001] = %q, want %q", index["ol-001"], "open")
	}
	if index["ol-002"] != "closed" {
		t.Errorf("index[ol-002] = %q, want %q", index["ol-002"], "closed")
	}
}

func TestDetectStatusDrifts(t *testing.T) {
	byBeadsID := map[string]*types.PlanManifestEntry{
		"ol-001": {PlanName: "plan-1", BeadsID: "ol-001", Status: types.PlanStatusActive},
		"ol-002": {PlanName: "plan-2", BeadsID: "ol-002", Status: types.PlanStatusCompleted},
		"ol-003": {PlanName: "plan-3", BeadsID: "ol-003", Status: types.PlanStatusActive},
	}

	beadsIndex := map[string]string{
		"ol-001": "open", // matches
		"ol-002": "open", // mismatch: manifest=completed, beads=open
		// ol-003 missing from beads
	}

	drifts := detectStatusDrifts(byBeadsID, beadsIndex)

	// Should find 2 drifts: status_mismatch for ol-002, missing_beads for ol-003
	if len(drifts) != 2 {
		t.Errorf("detectStatusDrifts() found %d drifts, want 2", len(drifts))
	}

	// Check for expected drift types
	foundMismatch := false
	foundMissing := false
	for _, d := range drifts {
		if d.Type == "status_mismatch" && d.BeadsID == "ol-002" {
			foundMismatch = true
		}
		if d.Type == "missing_beads" && d.BeadsID == "ol-003" {
			foundMissing = true
		}
	}
	if !foundMismatch {
		t.Error("expected to find status_mismatch for ol-002")
	}
	if !foundMissing {
		t.Error("expected to find missing_beads for ol-003")
	}
}

func TestDetectOrphanedEntries(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "linked-1", BeadsID: "ol-001"},
		{PlanName: "orphan-1", BeadsID: ""},
		{PlanName: "linked-2", BeadsID: "ol-002"},
		{PlanName: "orphan-2", BeadsID: ""},
	}

	drifts := detectOrphanedEntries(entries)

	if len(drifts) != 2 {
		t.Errorf("detectOrphanedEntries() found %d drifts, want 2", len(drifts))
	}

	for _, d := range drifts {
		if d.Type != "orphaned" {
			t.Errorf("drift type = %q, want 'orphaned'", d.Type)
		}
	}
}

// --- getManifestPath ---

func TestGetManifestPath(t *testing.T) {
	t.Run("returns path with .agents dir in cwd", func(t *testing.T) {
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		path, err := getManifestPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, filepath.Join(".agents", "plans", "manifest.jsonl")) {
			t.Errorf("path = %q, want suffix .agents/plans/manifest.jsonl", path)
		}
	})

	t.Run("returns default path when no .agents exists", func(t *testing.T) {
		dir := t.TempDir()

		t.Chdir(dir)

		path, err := getManifestPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should fall back to cwd/.agents/plans/manifest.jsonl
		if !strings.Contains(path, "manifest.jsonl") {
			t.Errorf("path = %q, expected manifest.jsonl", path)
		}
	})

	t.Run("finds .agents in parent with rig markers", func(t *testing.T) {
		dir := t.TempDir()
		// Create rig marker in parent dir
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0755); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(dir, "sub", "deep")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(subDir)

		path, err := getManifestPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should find .agents at the rig root (parent of sub/deep)
		if !strings.Contains(path, dir) {
			t.Errorf("path = %q, expected to be under %q", path, dir)
		}
	})
}

// --- findAgentsDir ---

func TestFindAgentsDir(t *testing.T) {
	t.Run("finds existing .agents dir", func(t *testing.T) {
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(dir, "a", "b")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(subDir)
		if got != agentsDir {
			t.Errorf("findAgentsDir(%q) = %q, want %q", subDir, got, agentsDir)
		}
	})

	t.Run("finds dir by rig marker .beads", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0755); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(dir, "child")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(subDir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("findAgentsDir(%q) = %q, want %q", subDir, got, want)
		}
	})

	t.Run("finds dir by rig marker crew", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "crew"), 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(dir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("findAgentsDir(%q) = %q, want %q", dir, got, want)
		}
	})

	t.Run("finds dir by rig marker polecats", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "polecats"), 0755); err != nil {
			t.Fatal(err)
		}

		got := findAgentsDir(dir)
		want := filepath.Join(dir, ".agents")
		if got != want {
			t.Errorf("findAgentsDir(%q) = %q, want %q", dir, got, want)
		}
	})

	t.Run("returns empty for root dir without markers", func(t *testing.T) {
		dir := t.TempDir()
		got := findAgentsDir(dir)
		if got != "" {
			t.Errorf("findAgentsDir(%q) = %q, want empty", dir, got)
		}
	})
}

// --- detectProjectPath ---

func TestDetectProjectPath(t *testing.T) {
	t.Run("detects from .claude/plans content with Project line", func(t *testing.T) {
		dir := t.TempDir()
		planDir := filepath.Join(dir, ".claude", "plans")
		if err := os.MkdirAll(planDir, 0755); err != nil {
			t.Fatal(err)
		}
		planPath := filepath.Join(planDir, "my-plan.md")
		content := "# Plan\nProject: /home/user/myproject\nSteps:\n1. Do stuff"
		if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := detectProjectPath(planPath)
		if got != "/home/user/myproject" {
			t.Errorf("detectProjectPath() = %q, want %q", got, "/home/user/myproject")
		}
	})

	t.Run("detects from Working directory line", func(t *testing.T) {
		dir := t.TempDir()
		planDir := filepath.Join(dir, ".claude", "plans")
		if err := os.MkdirAll(planDir, 0755); err != nil {
			t.Fatal(err)
		}
		planPath := filepath.Join(planDir, "work-plan.md")
		content := "# Plan\nWorking directory: /opt/project\nDo things"
		if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := detectProjectPath(planPath)
		if got != "/opt/project" {
			t.Errorf("detectProjectPath() = %q, want %q", got, "/opt/project")
		}
	})

	t.Run("returns cwd for non-.claude/plans path", func(t *testing.T) {
		dir := t.TempDir()
		planPath := filepath.Join(dir, "local-plan.md")
		if err := os.WriteFile(planPath, []byte("# Plan"), 0644); err != nil {
			t.Fatal(err)
		}

		got := detectProjectPath(planPath)
		// Should return os.Getwd() since plan is not in .claude/plans/
		if got == "" {
			t.Error("expected non-empty project path")
		}
	})

	t.Run("returns empty for unreadable .claude/plans file", func(t *testing.T) {
		got := detectProjectPath("/nonexistent/.claude/plans/no-such-plan.md")
		if got != "" {
			t.Errorf("expected empty for unreadable file, got %q", got)
		}
	})

	t.Run("falls through to cwd for .claude/plans file without project line", func(t *testing.T) {
		dir := t.TempDir()
		planDir := filepath.Join(dir, ".claude", "plans")
		if err := os.MkdirAll(planDir, 0755); err != nil {
			t.Fatal(err)
		}
		planPath := filepath.Join(planDir, "no-project.md")
		if err := os.WriteFile(planPath, []byte("# Plan\nNo project info here"), 0644); err != nil {
			t.Fatal(err)
		}

		got := detectProjectPath(planPath)
		// No "Project:" or "Working directory:" lines, falls through to cwd
		if got == "" {
			t.Error("expected non-empty (cwd fallback)")
		}
	})
}

// --- resolveProjectPath ---

func TestResolveProjectPath(t *testing.T) {
	t.Run("explicit overrides detection", func(t *testing.T) {
		got := resolveProjectPath("/explicit/path", "/some/.claude/plans/plan.md")
		if got != "/explicit/path" {
			t.Errorf("got %q, want /explicit/path", got)
		}
	})

	t.Run("empty explicit falls through to detection", func(t *testing.T) {
		dir := t.TempDir()
		planPath := filepath.Join(dir, "plan.md")
		if err := os.WriteFile(planPath, []byte("# Plan"), 0644); err != nil {
			t.Fatal(err)
		}
		got := resolveProjectPath("", planPath)
		// detectProjectPath should return cwd since not in .claude/plans/
		if got == "" {
			t.Error("expected non-empty from detection")
		}
	})
}

// --- resolvePlanName ---

func TestResolvePlanName(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		planPath string
		want     string
	}{
		{name: "explicit name", explicit: "my-custom-name", planPath: "/a/b/plan.md", want: "my-custom-name"},
		{name: "derived from filename", explicit: "", planPath: "/a/b/peaceful-stirring-tome.md", want: "peaceful-stirring-tome"},
		{name: "derived strips extension", explicit: "", planPath: "/a/b/plan.txt", want: "plan"},
		{name: "no extension", explicit: "", planPath: "/a/b/planfile", want: "planfile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePlanName(tt.explicit, tt.planPath)
			if got != tt.want {
				t.Errorf("resolvePlanName(%q, %q) = %q, want %q", tt.explicit, tt.planPath, got, tt.want)
			}
		})
	}
}

// --- buildRegisterEntry ---

func TestBuildRegisterEntry(t *testing.T) {
	t.Run("builds entry for valid file", func(t *testing.T) {
		dir := t.TempDir()
		planPath := filepath.Join(dir, "test-plan.md")
		if err := os.WriteFile(planPath, []byte("# Test Plan Content"), 0644); err != nil {
			t.Fatal(err)
		}

		entry, err := buildRegisterEntry(planPath, "/project", "my-plan", "ol-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Path != planPath {
			t.Errorf("Path = %q, want %q", entry.Path, planPath)
		}
		if entry.ProjectPath != "/project" {
			t.Errorf("ProjectPath = %q, want /project", entry.ProjectPath)
		}
		if entry.PlanName != "my-plan" {
			t.Errorf("PlanName = %q, want my-plan", entry.PlanName)
		}
		if entry.BeadsID != "ol-123" {
			t.Errorf("BeadsID = %q, want ol-123", entry.BeadsID)
		}
		if entry.Checksum == "" {
			t.Error("expected non-empty checksum")
		}
		if entry.Status != types.PlanStatusActive {
			t.Errorf("Status = %v, want active", entry.Status)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := buildRegisterEntry("/nonexistent/plan.md", "", "", "")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "plan not found") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- loadOrCreateManifest ---

func TestLoadOrCreateManifest(t *testing.T) {
	t.Run("creates manifest dir when not exists", func(t *testing.T) {
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		manifestPath, entries, err := loadOrCreateManifest()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(manifestPath, "manifest.jsonl") {
			t.Errorf("manifestPath = %q, want suffix manifest.jsonl", manifestPath)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries for new manifest, got %d", len(entries))
		}
		// Verify directory was created
		if _, err := os.Stat(filepath.Dir(manifestPath)); err != nil {
			t.Errorf("manifest dir should have been created: %v", err)
		}
	})

	t.Run("loads existing manifest", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := types.PlanManifestEntry{Path: "/existing.md", PlanName: "existing", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		_, entries, err := loadOrCreateManifest()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
	})
}

// --- upsertManifestEntry ---

func TestUpsertManifestEntry(t *testing.T) {
	t.Run("updates existing entry", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "manifest.jsonl")

		existing := []types.PlanManifestEntry{
			{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive},
			{Path: "/b.md", PlanName: "plan-b", Status: types.PlanStatusActive},
		}
		newEntry := types.PlanManifestEntry{Path: "/a.md", PlanName: "plan-a-updated", Status: types.PlanStatusCompleted}

		updated, err := upsertManifestEntry(manifestPath, existing, newEntry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Error("expected true for existing path update")
		}

		// Verify saved correctly
		loaded, err := loadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load error: %v", err)
		}
		if len(loaded) != 2 {
			t.Errorf("expected 2 entries, got %d", len(loaded))
		}
		if loaded[0].PlanName != "plan-a-updated" {
			t.Errorf("expected updated name, got %q", loaded[0].PlanName)
		}
	})

	t.Run("appends new entry", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "manifest.jsonl")

		existing := []types.PlanManifestEntry{
			{Path: "/a.md", PlanName: "plan-a"},
		}
		newEntry := types.PlanManifestEntry{Path: "/c.md", PlanName: "plan-c", Status: types.PlanStatusActive}

		updated, err := upsertManifestEntry(manifestPath, existing, newEntry)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated {
			t.Error("expected false for new entry")
		}

		loaded, err := loadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load error: %v", err)
		}
		if len(loaded) != 1 {
			t.Errorf("expected 1 appended entry, got %d", len(loaded))
		}
	})
}

// --- filterPlans ---

func TestFilterPlans(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{Path: "/a.md", PlanName: "plan-a", ProjectPath: "/proj1", Status: types.PlanStatusActive},
		{Path: "/b.md", PlanName: "plan-b", ProjectPath: "/proj2", Status: types.PlanStatusCompleted},
		{Path: "/c.md", PlanName: "plan-c", ProjectPath: "/proj1/sub", Status: types.PlanStatusActive},
	}

	t.Run("no filters returns all", func(t *testing.T) {
		got := filterPlans(entries, "", "")
		if len(got) != 3 {
			t.Errorf("expected 3, got %d", len(got))
		}
	})

	t.Run("filter by project substring", func(t *testing.T) {
		got := filterPlans(entries, "proj1", "")
		if len(got) != 2 {
			t.Errorf("expected 2 (proj1 and proj1/sub), got %d", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		got := filterPlans(entries, "", "completed")
		if len(got) != 1 {
			t.Errorf("expected 1 completed, got %d", len(got))
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		got := filterPlans(entries, "proj1", "active")
		if len(got) != 2 {
			t.Errorf("expected 2, got %d", len(got))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		got := filterPlans(entries, "nonexistent", "")
		if len(got) != 0 {
			t.Errorf("expected 0, got %d", len(got))
		}
	})
}

// --- applyPlanUpdates ---

func TestApplyPlanUpdates(t *testing.T) {
	t.Run("updates status and beadsID for matching path", func(t *testing.T) {
		entries := []types.PlanManifestEntry{
			{Path: "/a.md", Status: types.PlanStatusActive, BeadsID: ""},
		}
		found := applyPlanUpdates(entries, "/a.md", "completed", "ol-99")
		if !found {
			t.Fatal("expected true for matching path")
		}
		if entries[0].Status != "completed" {
			t.Errorf("Status = %q, want completed", entries[0].Status)
		}
		if entries[0].BeadsID != "ol-99" {
			t.Errorf("BeadsID = %q, want ol-99", entries[0].BeadsID)
		}
		if entries[0].UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}
	})

	t.Run("returns false for non-matching path", func(t *testing.T) {
		entries := []types.PlanManifestEntry{
			{Path: "/a.md", Status: types.PlanStatusActive},
		}
		found := applyPlanUpdates(entries, "/nonexistent.md", "completed", "")
		if found {
			t.Error("expected false for non-matching path")
		}
	})

	t.Run("only updates provided fields", func(t *testing.T) {
		entries := []types.PlanManifestEntry{
			{Path: "/a.md", Status: types.PlanStatusActive, BeadsID: "existing-id"},
		}
		found := applyPlanUpdates(entries, "/a.md", "completed", "")
		if !found {
			t.Fatal("expected true")
		}
		if entries[0].BeadsID != "existing-id" {
			t.Errorf("BeadsID should not change, got %q", entries[0].BeadsID)
		}
	})
}

// --- printPlanEntry ---

func TestPrintPlanEntry(t *testing.T) {
	t.Run("prints active plan", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName: "my-plan",
			Status:   types.PlanStatusActive,
			BeadsID:  "ol-123",
		}
		// Just verify it doesn't panic — output goes to stdout
		printPlanEntry(entry, false)
	})

	t.Run("prints completed plan", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName: "done-plan",
			Status:   types.PlanStatusCompleted,
		}
		printPlanEntry(entry, false)
	})

	t.Run("prints unknown status as string", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName: "weird-plan",
			Status:   types.PlanStatus("custom-status"),
		}
		printPlanEntry(entry, false)
	})

	t.Run("verbose mode prints extra details", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName:    "verbose-plan",
			Path:        "/path/to/plan.md",
			ProjectPath: "/project",
			Status:      types.PlanStatusActive,
			CreatedAt:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		}
		printPlanEntry(entry, true)
	})
}

// --- printRegistrationSummary ---

func TestPrintRegistrationSummary(t *testing.T) {
	t.Run("prints basic summary", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName: "new-plan",
		}
		printRegistrationSummary(entry) // should not panic
	})

	t.Run("prints summary with beads and project", func(t *testing.T) {
		entry := types.PlanManifestEntry{
			PlanName:    "full-plan",
			BeadsID:     "ol-456",
			ProjectPath: "/my/project",
		}
		printRegistrationSummary(entry) // should not panic
	})
}

// --- printSyncSummary ---

func TestPrintSyncSummary(t *testing.T) {
	t.Run("no drift", func(t *testing.T) {
		printSyncSummary(3, 0) // should not panic
	})

	t.Run("with drift", func(t *testing.T) {
		printSyncSummary(1, 2) // should not panic, prints hint
	})
}

// --- printDrifts ---

func TestPrintDrifts(t *testing.T) {
	drifts := []driftEntry{
		{Type: "status_mismatch", PlanName: "plan-1", BeadsID: "ol-1", Manifest: "active", Beads: "closed"},
		{Type: "orphaned", PlanName: "plan-2"},
		{Type: "missing_beads", PlanName: "plan-3", BeadsID: "ol-3"},
	}
	out, _ := captureStdout(t, func() error { printDrifts(drifts); return nil })
	for _, want := range []string{"plan-1", "mismatch", "plan-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// --- syncEpicsToManifest ---

func TestSyncEpicsToManifest(t *testing.T) {
	t.Run("syncs matching epics", func(t *testing.T) {
		entries := []types.PlanManifestEntry{
			{Path: "/a.md", BeadsID: "ol-1", Status: types.PlanStatusActive},
			{Path: "/b.md", BeadsID: "ol-2", Status: types.PlanStatusActive},
		}
		epics := []beadsEpic{
			{ID: "ol-1", Status: "closed"},
			{ID: "ol-2", Status: "open"},
		}
		byBeadsID := buildBeadsIDIndex(entries)

		synced := syncEpicsToManifest(entries, epics, byBeadsID)
		if synced != 1 {
			t.Errorf("expected 1 synced, got %d", synced)
		}
		if entries[0].Status != types.PlanStatusCompleted {
			t.Errorf("entry[0] status = %v, want completed", entries[0].Status)
		}
	})

	t.Run("no matches returns 0", func(t *testing.T) {
		entries := []types.PlanManifestEntry{
			{Path: "/a.md", BeadsID: "ol-1", Status: types.PlanStatusActive},
		}
		epics := []beadsEpic{
			{ID: "ol-99", Status: "closed"},
		}
		byBeadsID := buildBeadsIDIndex(entries)

		synced := syncEpicsToManifest(entries, epics, byBeadsID)
		if synced != 0 {
			t.Errorf("expected 0, got %d", synced)
		}
	})
}

// --- runPlansRegister ---

func TestRunPlansRegister(t *testing.T) {
	oldDryRun := dryRun
	oldPlanProjectPath := planProjectPath
	oldPlanBeadsID := planBeadsID
	oldPlanName := planName
	defer func() {
		dryRun = oldDryRun
		planProjectPath = oldPlanProjectPath
		planBeadsID = oldPlanBeadsID
		planName = oldPlanName
	}()

	t.Run("dry-run mode with existing file", func(t *testing.T) {
		dir := t.TempDir()
		planPath := filepath.Join(dir, "my-plan.md")
		if err := os.WriteFile(planPath, []byte("# Plan"), 0644); err != nil {
			t.Fatal(err)
		}
		dryRun = true

		cmd := &cobra.Command{}
		err := runPlansRegister(cmd, []string{planPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("dry-run mode with missing file returns error", func(t *testing.T) {
		dryRun = true
		cmd := &cobra.Command{}
		err := runPlansRegister(cmd, []string{"/nonexistent/plan.md"})
		if err == nil {
			t.Fatal("expected error for missing file in dry-run")
		}
	})

	t.Run("registers new plan", func(t *testing.T) {
		dir := t.TempDir()
		dryRun = false
		planProjectPath = "/test-project"
		planBeadsID = "ol-test"
		planName = "test-plan"

		// Create plan file
		planPath := filepath.Join(dir, "register-plan.md")
		if err := os.WriteFile(planPath, []byte("# Register Test Plan"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create .agents dir so getManifestPath works
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansRegister(cmd, []string{planPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify manifest was written
		manifestPath := filepath.Join(dir, ".agents", "plans", "manifest.jsonl")
		entries, err := loadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load manifest: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].PlanName != "test-plan" {
			t.Errorf("PlanName = %q, want test-plan", entries[0].PlanName)
		}
	})

	t.Run("updates existing plan on re-register", func(t *testing.T) {
		dir := t.TempDir()
		dryRun = false
		planProjectPath = "/test-project"
		planBeadsID = "ol-re"
		planName = "re-plan"

		planPath := filepath.Join(dir, "re-register.md")
		if err := os.WriteFile(planPath, []byte("# Re-Register Plan"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		// Register first time
		if err := runPlansRegister(cmd, []string{planPath}); err != nil {
			t.Fatalf("first register: %v", err)
		}

		// Register again (should update)
		planName = "re-plan-updated"
		if err := runPlansRegister(cmd, []string{planPath}); err != nil {
			t.Fatalf("second register: %v", err)
		}

		manifestPath := filepath.Join(dir, ".agents", "plans", "manifest.jsonl")
		entries, err := loadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load manifest: %v", err)
		}
		// Should still be 1 entry (updated, not duplicated)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry after update, got %d", len(entries))
		}
	})
}

// --- runPlansList ---

func TestRunPlansList(t *testing.T) {
	oldProjectPath := planProjectPath
	oldStatus := planStatus
	oldVerbose := verbose
	defer func() {
		planProjectPath = oldProjectPath
		planStatus = oldStatus
		verbose = oldVerbose
	}()

	t.Run("no manifest prints message", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		planProjectPath = ""
		planStatus = ""
		cmd := &cobra.Command{}
		err := runPlansList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("lists plans from manifest", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		entries := []types.PlanManifestEntry{
			{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive, ProjectPath: "/proj"},
			{Path: "/b.md", PlanName: "plan-b", Status: types.PlanStatusCompleted, ProjectPath: "/proj"},
		}
		for _, e := range entries {
			data, _ := json.Marshal(e)
			f, _ := os.OpenFile(filepath.Join(plansDir, "manifest.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			_, _ = f.WriteString(string(data) + "\n")
			_ = f.Close()
		}

		t.Chdir(dir)

		planProjectPath = ""
		planStatus = ""
		verbose = false
		cmd := &cobra.Command{}
		err := runPlansList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("filter returns no match", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := types.PlanManifestEntry{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		planProjectPath = ""
		planStatus = "nonexistent-status"
		cmd := &cobra.Command{}
		err := runPlansList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// --- runPlansSearch ---

func TestRunPlansSearch(t *testing.T) {
	t.Run("no manifest prints message", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansSearch(cmd, []string{"anything"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("finds matching plans", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		entries := []types.PlanManifestEntry{
			{Path: "/auth-plan.md", PlanName: "auth-migration", BeadsID: "ol-auth", Status: types.PlanStatusActive},
			{Path: "/data-plan.md", PlanName: "data-cleanup", Status: types.PlanStatusActive},
		}
		var content strings.Builder
		for _, e := range entries {
			data, _ := json.Marshal(e)
			content.WriteString(string(data) + "\n")
		}
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), []byte(content.String()), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansSearch(cmd, []string{"auth"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no matches prints message", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := types.PlanManifestEntry{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansSearch(cmd, []string{"zzzznonexistent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// --- runPlansUpdate ---

func TestRunPlansUpdate(t *testing.T) {
	oldDryRun := dryRun
	oldPlanStatus := planStatus
	oldPlanBeadsID := planBeadsID
	defer func() {
		dryRun = oldDryRun
		planStatus = oldPlanStatus
		planBeadsID = oldPlanBeadsID
	}()

	t.Run("dry-run mode", func(t *testing.T) {
		dryRun = true
		cmd := &cobra.Command{}
		err := runPlansUpdate(cmd, []string{"some-plan.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("updates plan status", func(t *testing.T) {
		dir := t.TempDir()
		dryRun = false
		planStatus = "completed"
		planBeadsID = "ol-upd"

		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		planPath := filepath.Join(dir, "update-plan.md")
		if err := os.WriteFile(planPath, []byte("# Plan"), 0644); err != nil {
			t.Fatal(err)
		}

		absPath, _ := filepath.Abs(planPath)
		entry := types.PlanManifestEntry{Path: absPath, PlanName: "update-plan", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansUpdate(cmd, []string{planPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify update
		entries, _ := loadManifest(filepath.Join(plansDir, "manifest.jsonl"))
		if len(entries) == 0 {
			t.Fatal("expected at least 1 entry")
		}
		if string(entries[0].Status) != "completed" {
			t.Errorf("status = %v, want completed", entries[0].Status)
		}
		if entries[0].BeadsID != "ol-upd" {
			t.Errorf("beadsID = %q, want ol-upd", entries[0].BeadsID)
		}
	})

	t.Run("returns error for non-existent plan", func(t *testing.T) {
		dir := t.TempDir()
		dryRun = false
		planStatus = "completed"
		planBeadsID = ""

		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := types.PlanManifestEntry{Path: "/some-other.md", PlanName: "other", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		err := runPlansUpdate(cmd, []string{"/nonexistent/plan.md"})
		if err == nil {
			t.Fatal("expected error for plan not in manifest")
		}
		if !strings.Contains(err.Error(), "plan not found in manifest") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- runPlansSync ---

func TestRunPlansSync(t *testing.T) {
	oldDryRun := dryRun
	defer func() { dryRun = oldDryRun }()

	t.Run("dry-run mode", func(t *testing.T) {
		dryRun = true
		cmd := &cobra.Command{}
		err := runPlansSync(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("sync with no beads available", func(t *testing.T) {
		dir := t.TempDir()
		dryRun = false

		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := types.PlanManifestEntry{Path: "/a.md", PlanName: "plan-a", Status: types.PlanStatusActive}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(filepath.Join(plansDir, "manifest.jsonl"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		t.Chdir(dir)

		cmd := &cobra.Command{}
		// This will fail to query beads (bd not available) but should handle gracefully
		err := runPlansSync(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// buildBeadsStatusIndex, detectStatusDrifts, detectOrphanedEntries, saveManifest
// tests are declared earlier in this file (lines 276-400). Duplicates removed.

// TestPlansCommand_ThreeCaseFallback exercises the atom-3 / soc-fmgx 3-case
// decision tree on `ao plans sync`:
//
//	case A: --via-daemon=false           → direct path, file-locked
//	case B: --via-daemon=true, daemon-up → daemon-routed (HTTP POST + wait)
//	case C: --via-daemon=true, daemon-down → fallback to direct path + warning
func TestPlansCommand_ThreeCaseFallback(t *testing.T) {
	prevDryRun := dryRun
	prevVia := plansSyncViaDaemon
	prevURL := daemonURL
	prevParamsFn := plansDaemonParamsFn
	t.Cleanup(func() {
		dryRun = prevDryRun
		plansSyncViaDaemon = prevVia
		daemonURL = prevURL
		plansDaemonParamsFn = prevParamsFn
	})
	dryRun = false

	t.Run("flag-off → direct path takes file lock and returns ok", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		plansSyncViaDaemon = false
		daemonURL = "http://127.0.0.1:1" // would fail if accidentally used
		err := runPlansSync(&cobra.Command{}, nil)
		if err != nil {
			t.Fatalf("flag-off run: %v", err)
		}
		// File lock should have been created and released; the .lock file
		// remains as a sentinel (cheap, not a problem).
		if _, statErr := os.Stat(filepath.Join(plansDir, "manifest.jsonl.lock")); statErr != nil {
			t.Fatalf("manifest.jsonl.lock should exist after flag-off direct path: %v", statErr)
		}
	})

	t.Run("flag-on AND daemon-down → falls back to direct path with warning", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		plansSyncViaDaemon = true
		// Use an unreachable port to force the fetchDaemonStatus health
		// probe to fail with errDaemonUnavailable.
		daemonURL = "http://127.0.0.1:1"
		err := runPlansSync(&cobra.Command{}, nil)
		if err != nil {
			t.Fatalf("flag-on+daemon-down should fall through to direct path, got: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(plansDir, "manifest.jsonl.lock")); statErr != nil {
			t.Fatalf("manifest.jsonl.lock should exist after fallback: %v", statErr)
		}
	})

	t.Run("flag-on AND daemon-up → submits plans.projection job and prints manifest path", func(t *testing.T) {
		dir := t.TempDir()
		plansDir := filepath.Join(dir, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		plansSyncViaDaemon = true

		// Inject deterministic params so the daemon spec is well-formed.
		plansDaemonParamsFn = func() (string, string, string) {
			return "test-project", "soc", filepath.Join(dir, ".agents", "plans")
		}

		// Spin up a fake daemon: /v1/status reports ready + (after submission)
		// the submitted job in completed state with manifest artifacts.
		var (
			mu          sync.Mutex
			submittedID string
		)
		artifactPath := filepath.Join(dir, ".agents", "plans", "manifest.jsonl")
		mux := http.NewServeMux()
		statusHandler := func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			jobs := []daemonpkg.QueueJobState{}
			if submittedID != "" {
				jobs = append(jobs, daemonpkg.QueueJobState{
					JobID:   submittedID,
					JobType: daemonpkg.JobTypePlansProjection,
					Status:  daemonpkg.JobStatusCompleted,
					Artifacts: map[string]string{
						"manifest_jsonl": artifactPath,
						"manifest_count": "0",
					},
				})
			}
			_ = json.NewEncoder(w).Encode(daemonpkg.ReadOnlyStatusResponse{
				Ready: true,
				Queue: daemonpkg.QueueSnapshot{Jobs: jobs},
			})
		}
		mux.HandleFunc("/v1/status", statusHandler)
		mux.HandleFunc("/status", statusHandler)
		jobsHandler := func(w http.ResponseWriter, r *http.Request) {
			var req daemonpkg.SubmitJobRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			submittedID = "job-plans-test"
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(daemonpkg.SubmitJobResponse{
				Accepted: true,
				JobID:    "job-plans-test",
				Status:   daemonpkg.JobStatusCompleted,
			})
		}
		mux.HandleFunc("/v1/jobs", jobsHandler)
		mux.HandleFunc("/jobs", jobsHandler)
		fake := httptest.NewServer(mux)
		defer fake.Close()
		daemonURL = fake.URL

		var stdout strings.Builder
		cmd := &cobra.Command{}
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		err := runPlansSync(cmd, nil)
		if err != nil {
			t.Fatalf("flag-on+daemon-up: %v", err)
		}
		if submittedID == "" {
			t.Fatalf("daemon never received a job submission (stdout=%q)", stdout.String())
		}
		if !strings.Contains(stdout.String(), "Daemon synced plans.projection") {
			t.Fatalf("stdout missing daemon-synced banner: %q", stdout.String())
		}
		if !strings.Contains(stdout.String(), submittedID) {
			t.Fatalf("stdout missing job id %q: %q", submittedID, stdout.String())
		}
	})
}

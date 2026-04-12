package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// readLearningData — md files
// ---------------------------------------------------------------------------

func TestMaturity_readLearningData_Markdown(t *testing.T) {
	tmp := t.TempDir()

	t.Run("reads frontmatter fields from md", func(t *testing.T) {
		assertReadLearningDataMarkdownFrontmatter(t, tmp)
	})

	t.Run("returns false for md with no parseable fields", func(t *testing.T) {
		assertReadLearningDataPlainMarkdownIgnored(t, tmp)
	})

	t.Run("handles numeric strings as floats", func(t *testing.T) {
		assertReadLearningDataNumericStrings(t, tmp)
	})
}

func assertReadLearningDataMarkdownFrontmatter(t *testing.T, tmp string) {
	t.Helper()
	path := filepath.Join(tmp, "learn.md")
	content := "---\nutility: 0.7500\nconfidence: 0.6000\nmaturity: candidate\n---\n# Content\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	data, ok := readLearningData(path)
	if !ok {
		t.Fatal("expected ok=true for valid md with frontmatter")
	}

	utility, ok := data["utility"].(float64)
	if !ok || utility != 0.75 {
		t.Errorf("utility = %v, want 0.75", data["utility"])
	}

	maturity, ok := data["maturity"].(string)
	if !ok || maturity != "candidate" {
		t.Errorf("maturity = %v, want 'candidate'", data["maturity"])
	}
}

func assertReadLearningDataPlainMarkdownIgnored(t *testing.T, tmp string) {
	t.Helper()
	path := filepath.Join(tmp, "plain.md")
	content := "# Just a heading\nSome content\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := readLearningData(path)
	if ok {
		t.Error("expected ok=false for md file with no relevant frontmatter fields")
	}
}

func assertReadLearningDataNumericStrings(t *testing.T, tmp string) {
	t.Helper()
	path := filepath.Join(tmp, "numeric.md")
	content := "---\nutility: 0.4200\nreward_count: 5\n---\n# X\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	data, ok := readLearningData(path)
	if !ok {
		t.Fatal("expected ok=true")
	}

	// reward_count should parse as float.
	if rc, ok := data["reward_count"].(float64); !ok || rc != 5 {
		t.Errorf("reward_count = %v, want 5.0", data["reward_count"])
	}
}

func TestMaturity_readLearningData_JSONL(t *testing.T) {
	tmp := t.TempDir()

	t.Run("delegates to JSONL reader", func(t *testing.T) {
		path := filepath.Join(tmp, "learn.jsonl")
		firstLine := `{"id":"L-1","utility":0.8,"maturity":"established"}`
		if err := os.WriteFile(path, []byte(firstLine+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		data, ok := readLearningData(path)
		if !ok {
			t.Fatal("expected ok=true for valid JSONL")
		}

		if data["id"] != "L-1" {
			t.Errorf("id = %v, want 'L-1'", data["id"])
		}
	})
}

func TestMaturity_readLearningData_MissingFile(t *testing.T) {
	_, ok := readLearningData("/nonexistent/file.md")
	if ok {
		t.Error("expected ok=false for missing file")
	}
}

// ---------------------------------------------------------------------------
// runMaturityMigrateMd
// ---------------------------------------------------------------------------

func TestMaturity_runMaturityMigrateMd_NoBareMarkdown(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Already has utility - should NOT be migrated
	content := "---\nutility: 0.8000\nmaturity: established\n---\n# Already good\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "good.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed: %v", err)
	}

	// Verify the file was NOT modified
	data, _ := os.ReadFile(filepath.Join(learningsDir, "good.md"))
	if !strings.Contains(string(data), "0.8000") {
		t.Error("file with existing utility should not be modified")
	}
}

func TestMaturity_runMaturityMigrateMd_InjectsFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Has frontmatter but no utility
	content := "---\ntitle: Test\n---\n# Content\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "needsmigration.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(learningsDir, "needsmigration.md"))
	text := string(data)
	if !strings.Contains(text, "utility:") {
		t.Error("expected utility field to be injected")
	}
	if !strings.Contains(text, "maturity: provisional") {
		t.Error("expected maturity field to be injected")
	}
}

func TestMaturity_runMaturityMigrateMd_PrependsFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No frontmatter at all
	content := "# Plain markdown\nSome content here\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "bare.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(learningsDir, "bare.md"))
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Error("expected frontmatter to be prepended")
	}
	if !strings.Contains(text, "utility:") {
		t.Error("expected utility in prepended frontmatter")
	}
	if !strings.Contains(text, "# Plain markdown") {
		t.Error("expected original content to be preserved")
	}
}

func TestMaturity_runMaturityMigrateMd_SkipsJSONL(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// JSONL file should be skipped
	jsonlContent := `{"id":"L-1","utility":0.5}` + "\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "test.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// .md file to verify counting
	mdContent := "# No frontmatter\nContent\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "needs.md"), []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed: %v", err)
	}

	// JSONL should be unchanged
	jsonlData, _ := os.ReadFile(filepath.Join(learningsDir, "test.jsonl"))
	if string(jsonlData) != jsonlContent {
		t.Error("JSONL file should not be modified")
	}
}

// ---------------------------------------------------------------------------
// runMaturityRecalibrate
// ---------------------------------------------------------------------------

func TestMaturity_runMaturityRecalibrate_DryRun(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsonlContent := `{"id":"L-1","utility":0.3,"confidence":0.5}` + "\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "test.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	err := runMaturityRecalibrate(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityRecalibrate dry-run failed: %v", err)
	}

	// Verify file was NOT modified in dry-run
	data, _ := os.ReadFile(filepath.Join(learningsDir, "test.jsonl"))
	if !strings.Contains(string(data), "0.3") {
		t.Error("file should not be modified in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// readLearningData dispatching
// ---------------------------------------------------------------------------

func TestMaturity_readLearningData_DispatchesByExtension(t *testing.T) {
	tmp := t.TempDir()

	// Create a JSONL file
	jsonlPath := filepath.Join(tmp, "test.jsonl")
	jsonlData := map[string]any{"id": "L-1", "utility": 0.75}
	b, _ := json.Marshal(jsonlData)
	if err := os.WriteFile(jsonlPath, append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an MD file
	mdPath := filepath.Join(tmp, "test.md")
	mdContent := "---\nutility: 0.6000\nmaturity: candidate\n---\n# X\n"
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	// JSONL dispatch
	data, ok := readLearningData(jsonlPath)
	if !ok {
		t.Fatal("expected ok=true for JSONL")
	}
	if data["id"] != "L-1" {
		t.Errorf("JSONL id = %v, want L-1", data["id"])
	}

	// MD dispatch
	data, ok = readLearningData(mdPath)
	if !ok {
		t.Fatal("expected ok=true for MD")
	}
	if v, ok := data["maturity"].(string); !ok || v != "candidate" {
		t.Errorf("MD maturity = %v, want candidate", data["maturity"])
	}
}

// ---------------------------------------------------------------------------
// runMaturityMigrateMd edge cases
// ---------------------------------------------------------------------------

func TestMaturity_runMaturityMigrateMd_MalformedFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Opening --- but no closing ---
	content := "---\ntitle: Bad\nno closing delimiter\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "malformed.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not panic or error
	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed on malformed: %v", err)
	}
}

func TestMaturity_runMaturityMigrateMd_EmptyLearningsDir(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := runMaturityMigrateMd(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityMigrateMd failed on empty: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runMaturityRecalibrate — actual recalibration
// ---------------------------------------------------------------------------

func TestMaturity_runMaturityRecalibrate_ResetsUtility(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	data := map[string]any{
		"id":         "L-1",
		"utility":    0.1,
		"confidence": 0.2,
		"maturity":   "provisional",
	}
	b, _ := json.Marshal(data)
	if err := os.WriteFile(filepath.Join(learningsDir, "low.jsonl"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	err := runMaturityRecalibrate(learningsDir)
	if err != nil {
		t.Fatalf("runMaturityRecalibrate failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(learningsDir, "low.jsonl"))
	lines := strings.Split(string(content), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("expected non-empty first line after recalibration")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("unmarshal recalibrated: %v", err)
	}

	// Utility should be reset to InitialUtility (0.5)
	utility, ok := result["utility"].(float64)
	if !ok {
		t.Fatal("utility not found in result")
	}
	if utility < types.InitialUtility-0.01 || utility > types.InitialUtility+0.01 {
		t.Errorf("utility = %.4f, want ~%.4f", utility, types.InitialUtility)
	}
}

// ---------------------------------------------------------------------------
// expiryCategory struct behavior
// ---------------------------------------------------------------------------

func TestMaturity_expiryCategory_AccumulatesCorrectly(t *testing.T) {
	cats := expiryCategory{}
	cats.active = append(cats.active, "a.md", "b.md")
	cats.neverExpiring = append(cats.neverExpiring, "c.md")
	cats.newlyExpired = append(cats.newlyExpired, "d.md")
	cats.alreadyArchived = append(cats.alreadyArchived, "e.md")

	total := len(cats.active) + len(cats.neverExpiring) + len(cats.newlyExpired) + len(cats.alreadyArchived)
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
}

// ---------------------------------------------------------------------------
// evictionCandidate JSON serialization
// ---------------------------------------------------------------------------

func TestMaturity_evictionCandidate_JSONSerialization(t *testing.T) {
	c := evictionCandidate{
		Path:       "/path/to/file.jsonl",
		Name:       "file.jsonl",
		Utility:    0.15,
		Confidence: 0.1,
		Maturity:   "provisional",
		LastCited:  "never",
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded evictionCandidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != "file.jsonl" {
		t.Errorf("Name = %q, want %q", decoded.Name, "file.jsonl")
	}
	if decoded.Utility != 0.15 {
		t.Errorf("Utility = %f, want 0.15", decoded.Utility)
	}
	if decoded.LastCited != "never" {
		t.Errorf("LastCited = %q, want %q", decoded.LastCited, "never")
	}
}

// ---------------------------------------------------------------------------
// parseFrontmatterFields
// ---------------------------------------------------------------------------

func TestMaturity_parseFrontmatterFields(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		fields     []string
		wantFields map[string]string
	}{
		{
			name:       "basic frontmatter",
			content:    "---\ntitle: My Learning\nvalid_until: 2099-12-31\n---\n# Body\n",
			fields:     []string{"title", "valid_until"},
			wantFields: map[string]string{"title": "My Learning", "valid_until": "2099-12-31"},
		},
		{
			name:       "quoted values stripped",
			content:    "---\ntitle: \"Quoted Title\"\nstatus: 'single'\n---\n",
			fields:     []string{"title", "status"},
			wantFields: map[string]string{"title": "Quoted Title", "status": "single"},
		},
		{
			name:       "field not present",
			content:    "---\ntitle: Test\n---\n",
			fields:     []string{"missing_field"},
			wantFields: map[string]string{},
		},
		{
			name:       "no frontmatter",
			content:    "# Just a heading\nSome content\n",
			fields:     []string{"title"},
			wantFields: map[string]string{},
		},
		{
			name:       "empty file",
			content:    "",
			fields:     []string{"title"},
			wantFields: map[string]string{},
		},
		{
			name:       "only opening delimiter",
			content:    "---\ntitle: Test\n",
			fields:     []string{"title"},
			wantFields: map[string]string{"title": "Test"},
		},
		{
			name:       "expiry_status field",
			content:    "---\nexpiry_status: archived\nvalid_until: 2020-01-01\n---\n",
			fields:     []string{"expiry_status", "valid_until"},
			wantFields: map[string]string{"expiry_status": "archived", "valid_until": "2020-01-01"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "test.md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := parseFrontmatterFields(path, tt.fields...)
			if err != nil {
				t.Fatalf("parseFrontmatterFields failed: %v", err)
			}

			for k, want := range tt.wantFields {
				if got[k] != want {
					t.Errorf("field %q = %q, want %q", k, got[k], want)
				}
			}
			// Check no extra fields returned
			for k := range got {
				if _, ok := tt.wantFields[k]; !ok {
					t.Errorf("unexpected field %q = %q", k, got[k])
				}
			}
		})
	}
}

func TestMaturity_parseFrontmatterFields_fileNotFound(t *testing.T) {
	_, err := parseFrontmatterFields("/nonexistent/path.md", "title")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ---------------------------------------------------------------------------
// classifyExpiryEntry
// ---------------------------------------------------------------------------

func TestMaturity_classifyExpiryEntry(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Active: future valid_until
	writeTestFrontmatter(t, learningsDir, "active.md", "valid_until: 2099-12-31")
	// Expired: past valid_until
	writeTestFrontmatter(t, learningsDir, "expired.md", "valid_until: 2020-01-01")
	// No expiry
	writeTestFrontmatter(t, learningsDir, "no-expiry.md", "title: Test")
	// Already archived
	writeTestFrontmatter(t, learningsDir, "archived.md", "expiry_status: archived\nvalid_until: 2020-01-01")
	// Malformed date
	writeTestFrontmatter(t, learningsDir, "bad-date.md", "valid_until: not-a-date")
	// RFC3339 date format
	writeTestFrontmatter(t, learningsDir, "rfc3339.md", "valid_until: 2099-12-31T23:59:59Z")

	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatal(err)
	}

	cats := expiryCategory{}
	for _, entry := range entries {
		if entry.IsDir() || !hasExtension(entry.Name(), ".md") {
			continue
		}
		classifyExpiryEntry(entry, learningsDir, &cats)
	}

	if len(cats.active) != 2 { // active.md + rfc3339.md
		t.Errorf("active count = %d, want 2 (got %v)", len(cats.active), cats.active)
	}
	if len(cats.newlyExpired) != 1 { // expired.md
		t.Errorf("newlyExpired count = %d, want 1 (got %v)", len(cats.newlyExpired), cats.newlyExpired)
	}
	if len(cats.alreadyArchived) != 1 { // archived.md
		t.Errorf("alreadyArchived count = %d, want 1 (got %v)", len(cats.alreadyArchived), cats.alreadyArchived)
	}
	if len(cats.neverExpiring) != 2 { // no-expiry.md + bad-date.md
		t.Errorf("neverExpiring count = %d, want 2 (got %v)", len(cats.neverExpiring), cats.neverExpiring)
	}
}

func hasExtension(name, ext string) bool {
	return len(name) > len(ext) && name[len(name)-len(ext):] == ext
}

func writeTestFrontmatter(t *testing.T, dir, name, frontmatter string) {
	t.Helper()
	content := "---\n" + frontmatter + "\n---\n# Content\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// isEvictionEligible
// ---------------------------------------------------------------------------

func TestMaturity_isEvictionEligible(t *testing.T) {
	tests := []struct {
		name       string
		utility    float64
		confidence float64
		maturity   string
		want       bool
	}{
		{"established never eligible", 0.1, 0.1, "established", false},
		{"high utility not eligible", 0.5, 0.1, "provisional", false},
		{"high confidence not eligible", 0.1, 0.5, "provisional", false},
		{"all criteria met", 0.1, 0.1, "provisional", true},
		{"boundary utility 0.3", 0.3, 0.1, "provisional", false},
		{"boundary confidence 0.3", 0.1, 0.3, "provisional", false},
		{"candidate eligible", 0.1, 0.1, "candidate", true},
		{"anti-pattern eligible", 0.1, 0.1, "anti-pattern", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEvictionEligible(tt.utility, tt.confidence, tt.maturity)
			if got != tt.want {
				t.Errorf("isEvictionEligible(%f, %f, %q) = %v, want %v",
					tt.utility, tt.confidence, tt.maturity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// evictionCitationStatus
// ---------------------------------------------------------------------------

func TestMaturity_evictionCitationStatus(t *testing.T) {
	now := time.Now()
	cutoff := now.AddDate(0, 0, -90)

	tests := []struct {
		name      string
		file      string
		lastCited map[string]time.Time
		wantStr   string
		wantOK    bool
	}{
		{
			name:      "never cited",
			file:      "/path/to/file",
			lastCited: map[string]time.Time{},
			wantStr:   "never",
			wantOK:    true,
		},
		{
			name:      "cited before cutoff",
			file:      "/path/to/file",
			lastCited: map[string]time.Time{"/path/to/file": now.AddDate(0, 0, -120)},
			wantStr:   now.AddDate(0, 0, -120).Format("2006-01-02"),
			wantOK:    true,
		},
		{
			name:      "cited after cutoff - not eligible",
			file:      "/path/to/file",
			lastCited: map[string]time.Time{"/path/to/file": now.AddDate(0, 0, -30)},
			wantStr:   "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, gotOK := evictionCitationStatus(tt.file, tt.lastCited, cutoff)
			if gotOK != tt.wantOK {
				t.Errorf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotStr != tt.wantStr {
				t.Errorf("str = %q, want %q", gotStr, tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readLearningJSONLData
// ---------------------------------------------------------------------------

func TestReadLearningJSONLData(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantOK  bool
	}{
		{"valid JSONL", `{"id":"L-1","utility":0.5}` + "\n", true},
		{"empty file", "", false},
		{"only whitespace", "   \n", false},
		{"invalid JSON", "not json\n", false},
		{"multi-line uses first", `{"id":"L-1"}` + "\n" + `{"id":"L-2"}` + "\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".jsonl")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			data, ok := readLearningJSONLData(path)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && data == nil {
				t.Error("expected non-nil data when ok=true")
			}
		})
	}
}

func TestMaturity_readLearningJSONLData_missingFile(t *testing.T) {
	_, ok := readLearningJSONLData("/nonexistent/file.jsonl")
	if ok {
		t.Error("expected ok=false for missing file")
	}
}

// ---------------------------------------------------------------------------
// archiveExpiredLearnings
// ---------------------------------------------------------------------------

func TestArchiveExpiredLearnings(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	for _, name := range []string{"expired1.md", "expired2.md"} {
		if err := os.WriteFile(filepath.Join(learningsDir, name), []byte("# Test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	expired := []string{"expired1.md", "expired2.md"}
	err := archiveExpiredLearnings(tmp, learningsDir, expired)
	if err != nil {
		t.Fatalf("archiveExpiredLearnings failed: %v", err)
	}

	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")
	for _, name := range expired {
		if _, err := os.Stat(filepath.Join(archiveDir, name)); os.IsNotExist(err) {
			t.Errorf("expected %s in archive dir", name)
		}
		if _, err := os.Stat(filepath.Join(learningsDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed from learnings dir", name)
		}
	}
}

func TestMaturity_archiveExpiredLearnings_dryRun(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "test.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	err := archiveExpiredLearnings(tmp, learningsDir, []string{"test.md"})
	if err != nil {
		t.Fatalf("archiveExpiredLearnings dry-run failed: %v", err)
	}

	// File should still exist
	if _, err := os.Stat(filepath.Join(learningsDir, "test.md")); os.IsNotExist(err) {
		t.Error("file should not be moved in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// archiveEvictionCandidates
// ---------------------------------------------------------------------------

func TestArchiveEvictionCandidates(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(learningsDir, "evict-me.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L-1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	candidates := []evictionCandidate{
		{Path: path, Name: "evict-me.jsonl"},
	}
	err := archiveEvictionCandidates(tmp, candidates)
	if err != nil {
		t.Fatalf("archiveEvictionCandidates failed: %v", err)
	}

	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")
	if _, err := os.Stat(filepath.Join(archiveDir, "evict-me.jsonl")); os.IsNotExist(err) {
		t.Error("expected file in archive dir")
	}
}

func TestMaturity_archiveEvictionCandidates_dryRun(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(learningsDir, "keep.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L-1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	candidates := []evictionCandidate{
		{Path: path, Name: "keep.jsonl"},
	}
	err := archiveEvictionCandidates(tmp, candidates)
	if err != nil {
		t.Fatalf("archiveEvictionCandidates dry-run failed: %v", err)
	}

	// File should still exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should not be moved in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// collectEvictionCandidates
// ---------------------------------------------------------------------------

func TestCollectEvictionCandidates(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Eligible: low utility, low confidence, provisional
	createTestLearningJSONL(t, learningsDir, "eligible.jsonl", map[string]any{
		"id": "L-eligible", "utility": 0.1, "confidence": 0.1, "maturity": "provisional",
	})
	// Not eligible: established
	createTestLearningJSONL(t, learningsDir, "established.jsonl", map[string]any{
		"id": "L-established", "utility": 0.1, "confidence": 0.1, "maturity": "established",
	})
	// Not eligible: high utility
	createTestLearningJSONL(t, learningsDir, "high-utility.jsonl", map[string]any{
		"id": "L-high", "utility": 0.8, "confidence": 0.1, "maturity": "provisional",
	})

	files, _ := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
	cutoff := time.Now().AddDate(0, 0, -90)
	lastCited := make(map[string]time.Time) // no citations

	candidates := collectEvictionCandidates(tmp, files, lastCited, cutoff)
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
	if len(candidates) > 0 && candidates[0].Name != "eligible.jsonl" {
		t.Errorf("expected eligible.jsonl, got %s", candidates[0].Name)
	}
}

// ---------------------------------------------------------------------------
// reportEvictionCandidates
// ---------------------------------------------------------------------------

func TestMaturity_reportEvictionCandidates_noCandidates(t *testing.T) {
	shouldArchive, err := reportEvictionCandidates([]string{"a.jsonl", "b.jsonl"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldArchive {
		t.Error("expected shouldArchive=false for no candidates")
	}
}

func TestMaturity_reportEvictionCandidates_withCandidates(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	candidates := []evictionCandidate{
		{Name: "test.jsonl", Utility: 0.1, Confidence: 0.1, Maturity: "provisional", LastCited: "never"},
	}
	shouldArchive, err := reportEvictionCandidates([]string{"test.jsonl"}, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldArchive {
		t.Error("expected shouldArchive=true when candidates exist (text mode)")
	}
}

func TestMaturity_reportEvictionCandidates_jsonOutput(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	candidates := []evictionCandidate{
		{Name: "test.jsonl", Utility: 0.1, Confidence: 0.1, Maturity: "provisional", LastCited: "never"},
	}
	shouldArchive, err := reportEvictionCandidates([]string{"test.jsonl"}, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldArchive {
		t.Error("expected shouldArchive=false in JSON mode (JSON output handles it)")
	}
}

func TestNormalizeLearningMarkdownMetadata_AddsDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "legacy.md")
	if err := os.WriteFile(path, []byte("# Legacy Learning\n\nThis legacy learning keeps enough body text to survive curation.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := normalizeLearningMetadata(path, false)
	if err != nil {
		t.Fatalf("normalizeLearningMetadata: %v", err)
	}
	if !changed {
		t.Fatal("expected markdown normalization to report a change")
	}

	fields, err := parseFrontmatterFields(path, "utility", "maturity", "confidence", "reward_count", "helpful_count", "harmful_count")
	if err != nil {
		t.Fatalf("parseFrontmatterFields: %v", err)
	}
	for key, want := range defaultLearningMetadataFields() {
		if got := fields[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestNormalizeLearningJSONLMetadata_AddsDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "legacy.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"L-legacy","title":"Legacy"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := normalizeLearningMetadata(path, false)
	if err != nil {
		t.Fatalf("normalizeLearningMetadata: %v", err)
	}
	if !changed {
		t.Fatal("expected JSONL normalization to report a change")
	}

	data, ok := readLearningJSONLData(path)
	if !ok {
		t.Fatal("expected normalized JSONL data to be readable")
	}
	if got := floatValueFromData(data, "utility", 0); got != types.InitialUtility {
		t.Errorf("utility = %.2f, want %.2f", got, types.InitialUtility)
	}
	if got := nonEmptyStringFromData(data, "maturity", ""); got != "provisional" {
		t.Errorf("maturity = %q, want provisional", got)
	}
}

func TestShouldArchiveUncitedLearning(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "old.md")
	if err := os.WriteFile(path, []byte("# Old Learning\n\nThis learning has enough content to exercise uncited archival logic.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -90)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().AddDate(0, 0, -60)
	if !shouldArchiveUncitedLearning(tmp, path, "provisional", old, map[string]time.Time{}, cutoff) {
		t.Fatal("expected uncited provisional learning older than cutoff to archive")
	}
	if shouldArchiveUncitedLearning(tmp, path, "established", old, map[string]time.Time{}, cutoff) {
		t.Fatal("established learning should not archive via uncited policy")
	}
	if shouldArchiveUncitedLearning(tmp, path, "candidate", old, map[string]time.Time{
		canonicalArtifactPath(tmp, path): time.Now().AddDate(0, 0, -1),
	}, cutoff) {
		t.Fatal("recently cited learning should not archive")
	}
}

func TestIsLowSignalLearningBody(t *testing.T) {
	if !isLowSignalLearningBody("didn't work because the goals agent rewrote it.") {
		t.Fatal("expected short fragment to be low signal")
	}
	if isLowSignalLearningBody("The session-end cleanup should normalize metadata before archiving stale learnings. It also needs citation-aware retention so uncited fragments do not accumulate forever.") {
		t.Fatal("expected substantive body to survive low-signal check")
	}
}

func TestBuildCurationCandidate_LowSignalAndUncited(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(learningsDir, "legacy.md")
	if err := os.WriteFile(path, []byte("didn't work because the goals agent rewrote it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -90)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	oldUncitedDays := maturityUncitedDays
	dryRun = false
	maturityUncitedDays = 60
	defer func() {
		dryRun = oldDryRun
		maturityUncitedDays = oldUncitedDays
	}()

	candidate, ok, normalized, err := buildCurationCandidate(tmp, path, map[string]time.Time{}, time.Now().AddDate(0, 0, -60))
	if err != nil {
		t.Fatalf("buildCurationCandidate: %v", err)
	}
	if !ok {
		t.Fatal("expected curation candidate")
	}
	if !normalized {
		t.Fatal("expected candidate build to normalize missing metadata")
	}
	if got := strings.Join(candidate.Reasons, ","); got != "low-signal-body,uncited-60d" {
		t.Fatalf("reasons = %q, want %q", got, "low-signal-body,uncited-60d")
	}
}

func TestArchiveCurationCandidates(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(learningsDir, "archive-me.md")
	if err := os.WriteFile(path, []byte("# Archive Me\n\nThis learning is ready to move.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	err := archiveCurationCandidates(tmp, []curationCandidate{{Path: path, Name: "archive-me.md"}})
	if err != nil {
		t.Fatalf("archiveCurationCandidates: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "archive", "learnings", "archive-me.md")); os.IsNotExist(err) {
		t.Fatal("expected curated learning in archive")
	}
}

// ---------------------------------------------------------------------------
// displayMaturityDistribution (smoke test, output only)
// ---------------------------------------------------------------------------

func TestDisplayMaturityDistribution(t *testing.T) {
	dist := &ratchet.MaturityDistribution{
		Provisional: 3,
		Candidate:   2,
		Established: 1,
		AntiPattern: 0,
		Total:       6,
	}
	out, err := captureStdout(t, func() error {
		displayMaturityDistribution(dist)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Provisional") {
		t.Errorf("expected output to contain 'Provisional', got: %s", out)
	}
	if !strings.Contains(out, "Provisional:  3") {
		t.Errorf("expected output to contain 'Provisional:  3', got: %s", out)
	}
	if !strings.Contains(out, "Total:        6") {
		t.Errorf("expected output to contain 'Total:        6', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// displayMaturityResult (smoke test, output only)
// ---------------------------------------------------------------------------

func TestDisplayMaturityResult(t *testing.T) {
	tests := []struct {
		name    string
		result  *ratchet.MaturityTransitionResult
		applied bool
	}{
		{
			name: "no transition",
			result: &ratchet.MaturityTransitionResult{
				LearningID:   "L-1",
				OldMaturity:  "provisional",
				Transitioned: false,
				Utility:      0.5,
				Confidence:   0.6,
				RewardCount:  3,
				HelpfulCount: 2,
				HarmfulCount: 1,
				Reason:       "not enough feedback",
			},
			applied: false,
		},
		{
			name: "transition applied",
			result: &ratchet.MaturityTransitionResult{
				LearningID:   "L-2",
				OldMaturity:  "provisional",
				NewMaturity:  "candidate",
				Transitioned: true,
				Utility:      0.8,
				Confidence:   0.7,
				RewardCount:  5,
				HelpfulCount: 4,
				HarmfulCount: 1,
				Reason:       "sufficient positive feedback",
			},
			applied: true,
		},
		{
			name: "transition not yet applied",
			result: &ratchet.MaturityTransitionResult{
				LearningID:   "L-3",
				OldMaturity:  "candidate",
				NewMaturity:  "established",
				Transitioned: true,
				Utility:      0.9,
				Confidence:   0.8,
				RewardCount:  10,
				HelpfulCount: 8,
				HarmfulCount: 2,
				Reason:       "proven value",
			},
			applied: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error {
				displayMaturityResult(tt.result, tt.applied)
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.result.LearningID) {
				t.Errorf("expected output to contain learning ID %q, got: %s", tt.result.LearningID, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// displayPendingTransitions
// ---------------------------------------------------------------------------

func TestMaturity_displayPendingTransitions_text(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	results := []*ratchet.MaturityTransitionResult{
		{LearningID: "L-1", OldMaturity: "provisional", NewMaturity: "candidate", Transitioned: true},
	}
	err := displayPendingTransitions(results)
	if err != nil {
		t.Fatalf("displayPendingTransitions text failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// evictionCandidate struct
// ---------------------------------------------------------------------------

func TestMaturity_evictionCandidateFields(t *testing.T) {
	c := evictionCandidate{
		Path:       "/path/to/file.jsonl",
		Name:       "file.jsonl",
		Utility:    0.15,
		Confidence: 0.1,
		Maturity:   "provisional",
		LastCited:  "never",
	}
	if c.Path == "" || c.Name == "" || c.Maturity == "" {
		t.Error("evictionCandidate fields should be set")
	}
}

// ---------------------------------------------------------------------------
// expiryCategory struct
// ---------------------------------------------------------------------------

func TestMaturity_expiryCategoryEmpty(t *testing.T) {
	cats := expiryCategory{}
	if len(cats.active) != 0 || len(cats.neverExpiring) != 0 ||
		len(cats.newlyExpired) != 0 || len(cats.alreadyArchived) != 0 {
		t.Error("empty expiryCategory should have all nil slices")
	}
}

// ===========================================================================
// Merged from maturity_deep_test.go
// ===========================================================================

// writeMaturityLearningJSONL writes a JSONL learning file with the given metadata.
func writeMaturityLearningJSONL(t *testing.T, dir, name string, data map[string]any) string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal learning data: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write learning file: %v", err)
	}
	return path
}

// setupMaturityDir creates a temp dir with .agents/learnings/ structure.
func setupMaturityDir(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	return tmp, learningsDir
}

// makeTransitionResults creates a slice of MaturityTransitionResult for testing.
func makeTransitionResults(ids ...string) []*ratchet.MaturityTransitionResult {
	var results []*ratchet.MaturityTransitionResult
	for _, id := range ids {
		results = append(results, &ratchet.MaturityTransitionResult{
			LearningID:   id,
			OldMaturity:  "provisional",
			NewMaturity:  "anti-pattern",
			Transitioned: true,
			Utility:      0.1,
			HarmfulCount: 10,
			RewardCount:  10,
			Reason:       "test",
		})
	}
	return results
}

// --- runMaturitySingle tests ---

func TestMaturity_runMaturitySingle_dryRun(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)

	writeMaturityLearningJSONL(t, learningsDir, "L001.jsonl", map[string]any{
		"id":       "L001",
		"maturity": "provisional",
		"utility":  0.5,
	})

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	err := runMaturitySingle(tmp, "L001")
	if err != nil {
		t.Fatalf("runMaturitySingle dry-run: %v", err)
	}
	// Verify learning file was not modified in dry-run mode
	if _, statErr := os.Stat(filepath.Join(learningsDir, "L001.jsonl")); statErr != nil {
		t.Errorf("learning file missing after dry-run: %v", statErr)
	}
}

func TestMaturity_runMaturitySingle_notFound(t *testing.T) {
	tmp, _ := setupMaturityDir(t)

	err := runMaturitySingle(tmp, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for nonexistent learning")
	}
	if !strings.Contains(err.Error(), "find learning") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaturity_runMaturitySingle_checkNoTransition(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)

	writeMaturityLearningJSONL(t, learningsDir, "L002.jsonl", map[string]any{
		"id":            "L002",
		"maturity":      "provisional",
		"utility":       0.5,
		"confidence":    0.5,
		"reward_count":  1,
		"helpful_count": 0,
		"harmful_count": 0,
	})

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	captureJSONStdout(t, func() {
		err := runMaturitySingle(tmp, "L002")
		if err != nil {
			t.Fatalf("runMaturitySingle: %v", err)
		}
	})
}

// --- checkOrApplyMaturity tests ---

func TestMaturity_checkOrApplyMaturity_checkMode(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	path := writeMaturityLearningJSONL(t, learningsDir, "L003.jsonl", map[string]any{
		"id":            "L003",
		"maturity":      "provisional",
		"utility":       0.5,
		"reward_count":  1,
		"helpful_count": 1,
		"harmful_count": 0,
	})

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	result, err := checkOrApplyMaturity(path)
	if err != nil {
		t.Fatalf("checkOrApplyMaturity: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LearningID != "L003" {
		t.Fatalf("expected learning ID L003, got %q", result.LearningID)
	}
}

func TestMaturity_checkOrApplyMaturity_applyMode(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	path := writeMaturityLearningJSONL(t, learningsDir, "L004.jsonl", map[string]any{
		"id":            "L004",
		"maturity":      "provisional",
		"utility":       0.8,
		"confidence":    0.9,
		"reward_count":  5,
		"helpful_count": 4,
		"harmful_count": 0,
	})

	oldApply := maturityApply
	maturityApply = true
	defer func() { maturityApply = oldApply }()

	result, err := checkOrApplyMaturity(path)
	if err != nil {
		t.Fatalf("checkOrApplyMaturity apply: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- outputSingleMaturityResult tests ---

func TestMaturity_outputSingleMaturityResult_json(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	result := &ratchet.MaturityTransitionResult{
		LearningID:   "L005",
		OldMaturity:  "provisional",
		NewMaturity:  "provisional",
		Transitioned: false,
		Utility:      0.5,
		Confidence:   0.5,
		HelpfulCount: 1,
		HarmfulCount: 0,
		RewardCount:  1,
		Reason:       "no transition",
	}

	got := captureJSONStdout(t, func() {
		err := outputSingleMaturityResult(result)
		if err != nil {
			t.Fatalf("outputSingleMaturityResult json: %v", err)
		}
	})
	if !strings.Contains(got, "L005") {
		t.Fatalf("expected JSON output to contain L005, got: %s", got)
	}
}

func TestMaturity_outputSingleMaturityResult_table(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	result := &ratchet.MaturityTransitionResult{
		LearningID:   "L006",
		OldMaturity:  "provisional",
		NewMaturity:  "candidate",
		Transitioned: true,
		Utility:      0.8,
		Confidence:   0.9,
		HelpfulCount: 4,
		HarmfulCount: 0,
		RewardCount:  5,
		Reason:       "met threshold",
	}

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	captureJSONStdout(t, func() {
		err := outputSingleMaturityResult(result)
		if err != nil {
			t.Fatalf("outputSingleMaturityResult table: %v", err)
		}
	})
}

// --- runMaturity (Cobra RunE) tests ---

func TestMaturity_runMaturity_noLearningsDir(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	cmd := &cobra.Command{}
	err := runMaturity(cmd, []string{})
	if err != nil {
		t.Fatalf("expected nil error when no learnings dir, got: %v", err)
	}
	// Verify no learnings dir was created as side effect
	if _, statErr := os.Stat(filepath.Join(tmp, ".agents", "learnings")); statErr == nil {
		t.Error("learnings dir was unexpectedly created when it should not exist")
	}
}

func TestMaturity_runMaturity_noArgsNoScan(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	chdirTo(t, tmp)

	oldScan := maturityScan
	maturityScan = false
	defer func() { maturityScan = oldScan }()

	oldExpire := maturityExpire
	maturityExpire = false
	defer func() { maturityExpire = oldExpire }()

	oldEvict := maturityEvict
	maturityEvict = false
	defer func() { maturityEvict = oldEvict }()

	cmd := &cobra.Command{}
	err := runMaturity(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when no args and no --scan")
	}
	if !strings.Contains(err.Error(), "must provide learning-id or use --scan") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaturity_runMaturity_scanMode(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	chdirTo(t, tmp)

	writeMaturityLearningJSONL(t, learningsDir, "scan-test.jsonl", map[string]any{
		"id":            "scan-test",
		"maturity":      "provisional",
		"utility":       0.5,
		"reward_count":  1,
		"helpful_count": 1,
		"harmful_count": 0,
	})

	oldScan := maturityScan
	maturityScan = true
	defer func() { maturityScan = oldScan }()

	oldExpire := maturityExpire
	maturityExpire = false
	defer func() { maturityExpire = oldExpire }()

	oldEvict := maturityEvict
	maturityEvict = false
	defer func() { maturityEvict = oldEvict }()

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runMaturity(cmd, []string{})
		if err != nil {
			t.Fatalf("runMaturity scan mode: %v", err)
		}
	})
}

func TestMaturity_runMaturity_withLearningID(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	chdirTo(t, tmp)

	writeMaturityLearningJSONL(t, learningsDir, "L010.jsonl", map[string]any{
		"id":            "L010",
		"maturity":      "provisional",
		"utility":       0.5,
		"reward_count":  1,
		"helpful_count": 1,
		"harmful_count": 0,
	})

	oldScan := maturityScan
	maturityScan = false
	defer func() { maturityScan = oldScan }()

	oldExpire := maturityExpire
	maturityExpire = false
	defer func() { maturityExpire = oldExpire }()

	oldEvict := maturityEvict
	maturityEvict = false
	defer func() { maturityEvict = oldEvict }()

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runMaturity(cmd, []string{"L010"})
		if err != nil {
			t.Fatalf("runMaturity with ID: %v", err)
		}
	})
}

// --- applyScannedTransitions tests ---

func TestMaturity_applyScannedTransitions_noResults(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	out := captureJSONStdout(t, func() {
		applyScannedTransitions(learningsDir, nil)
	})
	// With nil results, should produce no transition output
	if strings.Contains(out, "transitioned") {
		t.Errorf("expected no transition output for nil results, got: %s", out)
	}
}

func TestMaturity_applyScannedTransitions_missingFile(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	results := makeTransitionResults("missing-learning")

	out := captureJSONStdout(t, func() {
		applyScannedTransitions(learningsDir, results)
	})
	// Missing file should not crash, just skip
	_ = out // function completed without panic
	if _, err := os.Stat(filepath.Join(learningsDir, "missing-learning.jsonl")); err == nil {
		t.Error("expected missing-learning.jsonl to not exist")
	}
}

func TestMaturity_applyScannedTransitions_withValidFile(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	// Create a learning that matches the transition result
	writeMaturityLearningJSONL(t, learningsDir, "apply-target.jsonl", map[string]any{
		"id":            "apply-target",
		"maturity":      "provisional",
		"utility":       0.1,
		"confidence":    0.1,
		"reward_count":  10,
		"helpful_count": 0,
		"harmful_count": 10,
	})

	results := makeTransitionResults("apply-target")

	captureJSONStdout(t, func() {
		applyScannedTransitions(learningsDir, results)
	})
	// Verify the learning file still exists after applying transitions
	if _, err := os.Stat(filepath.Join(learningsDir, "apply-target.jsonl")); err != nil {
		t.Errorf("learning file missing after apply: %v", err)
	}
}

func TestMaturity_runMaturityScan_noTransitions(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	writeMaturityLearningJSONL(t, learningsDir, "stable.jsonl", map[string]any{
		"id":            "stable",
		"maturity":      "provisional",
		"utility":       0.5,
		"reward_count":  1,
		"helpful_count": 0,
		"harmful_count": 0,
	})

	captureJSONStdout(t, func() {
		err := runMaturityScan(learningsDir)
		if err != nil {
			t.Fatalf("runMaturityScan no transitions: %v", err)
		}
	})
}

func TestMaturity_runMaturityScan_withTransitions(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	oldApply := maturityApply
	maturityApply = false
	defer func() { maturityApply = oldApply }()

	writeMaturityLearningJSONL(t, learningsDir, "ready.jsonl", map[string]any{
		"id":            "ready",
		"maturity":      "provisional",
		"utility":       0.8,
		"confidence":    0.9,
		"reward_count":  5,
		"helpful_count": 4,
		"harmful_count": 0,
	})

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	captureJSONStdout(t, func() {
		err := runMaturityScan(learningsDir)
		if err != nil {
			t.Fatalf("runMaturityScan with transitions: %v", err)
		}
	})
}

func TestMaturity_runMaturityScan_withApply(t *testing.T) {
	_, learningsDir := setupMaturityDir(t)

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	oldApply := maturityApply
	maturityApply = true
	defer func() { maturityApply = oldApply }()

	writeMaturityLearningJSONL(t, learningsDir, "apply-me.jsonl", map[string]any{
		"id":            "apply-me",
		"maturity":      "provisional",
		"utility":       0.8,
		"confidence":    0.9,
		"reward_count":  5,
		"helpful_count": 4,
		"harmful_count": 0,
	})

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	captureJSONStdout(t, func() {
		err := runMaturityScan(learningsDir)
		if err != nil {
			t.Fatalf("runMaturityScan with apply: %v", err)
		}
	})
}

// --- runAntiPatterns tests ---

func TestMaturity_runAntiPatterns_noLearningsDir(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runAntiPatterns(cmd, nil)
		if err != nil {
			t.Fatalf("expected nil for missing learnings dir, got: %v", err)
		}
	})
}

func TestMaturity_runAntiPatterns_noAntiPatterns(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	chdirTo(t, tmp)

	writeMaturityLearningJSONL(t, learningsDir, "normal.jsonl", map[string]any{
		"id":            "normal",
		"maturity":      "provisional",
		"utility":       0.8,
		"reward_count":  2,
		"helpful_count": 2,
		"harmful_count": 0,
	})

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runAntiPatterns(cmd, nil)
		if err != nil {
			t.Fatalf("runAntiPatterns with no anti-patterns: %v", err)
		}
	})
}

func TestMaturity_runAntiPatterns_tableOutput(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	chdirTo(t, tmp)

	writeMaturityLearningJSONL(t, learningsDir, "bad.jsonl", map[string]any{
		"id":            "bad",
		"maturity":      "anti-pattern",
		"utility":       0.1,
		"confidence":    0.1,
		"reward_count":  10,
		"helpful_count": 0,
		"harmful_count": 10,
	})

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runAntiPatterns(cmd, nil)
		if err != nil {
			t.Fatalf("runAntiPatterns table: %v", err)
		}
	})
}

func TestMaturity_runAntiPatterns_jsonOutput(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	chdirTo(t, tmp)

	writeMaturityLearningJSONL(t, learningsDir, "bad-json.jsonl", map[string]any{
		"id":            "bad-json",
		"maturity":      "anti-pattern",
		"utility":       0.1,
		"confidence":    0.1,
		"reward_count":  10,
		"helpful_count": 0,
		"harmful_count": 10,
	})

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	captureJSONStdout(t, func() {
		cmd := &cobra.Command{}
		err := runAntiPatterns(cmd, nil)
		if err != nil {
			t.Fatalf("runAntiPatterns JSON: %v", err)
		}
	})
}

func TestMaturity_buildCitationMap_noCitationsFile(t *testing.T) {
	result := buildCitationMap(t.TempDir())
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestMaturity_runMaturityEvict_noLearningsDir(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	got := captureJSONStdout(t, func() {
		if err := runMaturityEvict(maturityCmd); err != nil {
			t.Fatalf("no dir: %v", err)
		}
	})
	if !strings.Contains(got, "No learnings directory") {
		t.Errorf("expected no-dir, got: %s", got)
	}
}

func TestMaturity_runMaturityEvict_withCandidate(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(learningsDir, 0o755)
	chdirTo(t, tmp)
	writeMaturityLearningJSONL(t, learningsDir, "ev.jsonl", map[string]any{
		"id": "ev", "maturity": "provisional", "utility": 0.1, "confidence": 0.1,
		"reward_count": 1, "helpful_count": 0, "harmful_count": 1,
	})
	oldEvict := maturityEvict
	oldArchive := maturityArchive
	maturityEvict = true
	maturityArchive = true
	defer func() { maturityEvict = oldEvict; maturityArchive = oldArchive }()
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()
	captureJSONStdout(t, func() {
		if err := runMaturityEvict(maturityCmd); err != nil {
			t.Fatalf("evict: %v", err)
		}
	})
}

func TestMaturity_runMaturityEvict_jsonOutput(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(learningsDir, 0o755)
	chdirTo(t, tmp)
	writeMaturityLearningJSONL(t, learningsDir, "ej.jsonl", map[string]any{
		"id": "ej", "maturity": "provisional", "utility": 0.1, "confidence": 0.1,
		"reward_count": 1, "helpful_count": 0, "harmful_count": 1,
	})
	oldEvict := maturityEvict
	oldArchive := maturityArchive
	maturityEvict = true
	maturityArchive = false
	defer func() { maturityEvict = oldEvict; maturityArchive = oldArchive }()
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()
	captureJSONStdout(t, func() {
		if err := runMaturityEvict(maturityCmd); err != nil {
			t.Fatalf("json: %v", err)
		}
	})
}

func TestMaturity_runMaturityScanAll_dryRun2(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()
	got := captureJSONStdout(t, func() {
		if err := runMaturityScanAll(filepath.Join(tmp, ".agents", "learnings"), filepath.Join(tmp, ".agents", "patterns")); err != nil {
			t.Fatalf("dry-run: %v", err)
		}
	})
	if !strings.Contains(got, "[dry-run]") {
		t.Errorf("expected dry-run, got: %s", got)
	}
}

func TestMaturity_runMaturityScanAll_noDirs2(t *testing.T) {
	tmp := t.TempDir()
	got := captureJSONStdout(t, func() {
		if err := runMaturityScanAll(filepath.Join(tmp, "x"), filepath.Join(tmp, "y")); err != nil {
			t.Fatalf("no dirs: %v", err)
		}
	})
	if !strings.Contains(got, "No learnings or patterns") {
		t.Errorf("expected msg, got: %s", got)
	}
}

func TestMaturity_runMaturityScanAll_bothDirsApply(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	os.MkdirAll(patternsDir, 0o755)
	writeMaturityLearningJSONL(t, learningsDir, "rd.jsonl", map[string]any{
		"id": "rd", "maturity": "provisional", "utility": 0.7, "confidence": 0.6,
		"reward_count": 5, "helpful_count": 5, "harmful_count": 0,
	})
	writeMaturityLearningJSONL(t, patternsDir, "pp.jsonl", map[string]any{
		"id": "pp", "maturity": "established", "utility": 0.8, "confidence": 0.8,
		"reward_count": 10, "helpful_count": 10, "harmful_count": 0,
	})
	oldApply := maturityApply
	maturityApply = true
	defer func() { maturityApply = oldApply }()
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()
	captureJSONStdout(t, func() {
		if err := runMaturityScanAll(learningsDir, patternsDir); err != nil {
			t.Fatalf("apply: %v", err)
		}
	})
}

func TestMaturity_runMaturityScanAll_noTransitions2(t *testing.T) {
	tmp, learningsDir := setupMaturityDir(t)
	writeMaturityLearningJSONL(t, learningsDir, "st.jsonl", map[string]any{
		"id": "st", "maturity": "established", "utility": 0.9, "confidence": 0.9,
		"reward_count": 20, "helpful_count": 20, "harmful_count": 0,
	})
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()
	got := captureJSONStdout(t, func() {
		if err := runMaturityScanAll(learningsDir, filepath.Join(tmp, ".agents", "patterns")); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	if !strings.Contains(got, "No pending maturity transitions") {
		t.Errorf("expected no-transitions, got: %s", got)
	}
}

func TestMaturity_displayPendingTransitions_jsonOut(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()
	results := []*ratchet.MaturityTransitionResult{{
		LearningID: "L099", OldMaturity: "provisional", NewMaturity: "candidate",
		Transitioned: true, Utility: 0.7, Confidence: 0.6,
		RewardCount: 5, HelpfulCount: 5, HarmfulCount: 0, Reason: "test",
	}}
	got := captureJSONStdout(t, func() {
		if err := displayPendingTransitions(results); err != nil {
			t.Fatalf("json: %v", err)
		}
	})
	if !strings.Contains(got, "L099") {
		t.Errorf("expected L099, got: %s", got)
	}
}

func TestMaturity_runMaturity_evictBranch(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	chdirTo(t, tmp)
	oldEvict := maturityEvict
	maturityEvict = true
	defer func() { maturityEvict = oldEvict }()
	captureJSONStdout(t, func() {
		if err := runMaturity(maturityCmd, nil); err != nil {
			t.Fatalf("evict: %v", err)
		}
	})
}

func TestMaturity_runMaturity_expireBranch(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	chdirTo(t, tmp)
	oldExpire := maturityExpire
	maturityExpire = true
	defer func() { maturityExpire = oldExpire }()
	captureJSONStdout(t, func() {
		if err := runMaturity(maturityCmd, nil); err != nil {
			t.Fatalf("expire: %v", err)
		}
	})
}

func TestMaturity_runMaturity_migrateMdBranch(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	chdirTo(t, tmp)
	oldMigrate := maturityMigrateMd
	maturityMigrateMd = true
	defer func() { maturityMigrateMd = oldMigrate }()
	captureJSONStdout(t, func() {
		if err := runMaturity(maturityCmd, nil); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	})
}

func TestMaturity_runMaturity_recalibrateBranch(t *testing.T) {
	tmp, _ := setupMaturityDir(t)
	chdirTo(t, tmp)
	oldRecal := maturityRecalibrate
	maturityRecalibrate = true
	defer func() { maturityRecalibrate = oldRecal }()
	captureJSONStdout(t, func() {
		if err := runMaturity(maturityCmd, nil); err != nil {
			t.Fatalf("recalibrate: %v", err)
		}
	})
}

func TestMaturity_runMaturityExpire_archivesExpiredFile(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(learningsDir, 0o755)
	chdirTo(t, tmp)
	os.WriteFile(filepath.Join(learningsDir, "old.md"), []byte("---\ntitle: Old\nvalid_until: 2020-01-01\n---\nExpired.\n"), 0o644)
	oldExpire := maturityExpire
	oldArchive := maturityArchive
	maturityExpire = true
	maturityArchive = true
	defer func() { maturityExpire = oldExpire; maturityArchive = oldArchive }()
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()
	captureJSONStdout(t, func() {
		if err := runMaturityExpire(maturityCmd); err != nil {
			t.Fatalf("archive: %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(learningsDir, "old.md")); !os.IsNotExist(err) {
		t.Error("expired file still exists")
	}
}

func TestMaturity_runMaturityExpire_mixedCategories(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(learningsDir, 0o755)
	chdirTo(t, tmp)
	os.WriteFile(filepath.Join(learningsDir, "active.md"), []byte("---\nvalid_until: 2099-01-01\n---\nB\n"), 0o644)
	os.WriteFile(filepath.Join(learningsDir, "eternal.md"), []byte("---\ntitle: E\n---\nB\n"), 0o644)
	os.WriteFile(filepath.Join(learningsDir, "archived.md"), []byte("---\nexpiry_status: archived\n---\nB\n"), 0o644)
	os.WriteFile(filepath.Join(learningsDir, "expired.md"), []byte("---\nvalid_until: 2020-01-01\n---\nB\n"), 0o644)
	oldExpire := maturityExpire
	oldArchive := maturityArchive
	maturityExpire = true
	maturityArchive = false
	defer func() { maturityExpire = oldExpire; maturityArchive = oldArchive }()
	got := captureJSONStdout(t, func() {
		if err := runMaturityExpire(maturityCmd); err != nil {
			t.Fatalf("mixed: %v", err)
		}
	})
	if !strings.Contains(got, "Active:           1") {
		t.Errorf("expected 1 active, got: %s", got)
	}
}

func TestMaturity_classifyExpiryEntry_rfc3339Date(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "rfc.md"), []byte("---\nvalid_until: 2020-01-01T00:00:00Z\n---\nB\n"), 0o644)
	info, _ := os.ReadDir(tmp)
	cats := expiryCategory{}
	classifyExpiryEntry(info[0], tmp, &cats)
	if len(cats.newlyExpired) != 1 {
		t.Errorf("expected 1 newlyExpired, got %d", len(cats.newlyExpired))
	}
}

func TestMaturity_classifyExpiryEntry_badDateFormat(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "bad.md"), []byte("---\nvalid_until: not-a-date\n---\nB\n"), 0o644)
	info, _ := os.ReadDir(tmp)
	cats := expiryCategory{}
	classifyExpiryEntry(info[0], tmp, &cats)
	if len(cats.neverExpiring) != 1 {
		t.Errorf("expected 1 neverExpiring, got %d", len(cats.neverExpiring))
	}
}

func TestMaturity_classifyExpiryEntry_futureDate(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "future.md"), []byte("---\nvalid_until: 2099-12-31\n---\nB\n"), 0o644)
	info, _ := os.ReadDir(tmp)
	cats := expiryCategory{}
	classifyExpiryEntry(info[0], tmp, &cats)
	if len(cats.active) != 1 {
		t.Errorf("expected 1 active, got %d", len(cats.active))
	}
}

func TestMaturity_classifyExpiryEntry_alreadyArchived(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "arch.md"), []byte("---\nexpiry_status: archived\n---\nB\n"), 0o644)
	info, _ := os.ReadDir(tmp)
	cats := expiryCategory{}
	classifyExpiryEntry(info[0], tmp, &cats)
	if len(cats.alreadyArchived) != 1 {
		t.Errorf("expected 1 alreadyArchived, got %d", len(cats.alreadyArchived))
	}
}

func TestMaturity_floatValueFromData_stringType(t *testing.T) {
	data := map[string]any{"utility": "not-a-float"}
	if got := floatValueFromData(data, "utility", 0.5); got != 0.5 {
		t.Errorf("string type = %f, want 0.5", got)
	}
}

func TestMaturity_nonEmptyStringFromData_emptyString(t *testing.T) {
	data := map[string]any{"maturity": ""}
	if got := nonEmptyStringFromData(data, "maturity", "default"); got != "default" {
		t.Errorf("empty = %q, want default", got)
	}
}

func TestMaturity_nonEmptyStringFromData_intType(t *testing.T) {
	data := map[string]any{"maturity": 42}
	if got := nonEmptyStringFromData(data, "maturity", "default"); got != "default" {
		t.Errorf("int = %q, want default", got)
	}
}

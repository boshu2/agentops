package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMineWorkItemID_HotspotUsesFileFunc(t *testing.T) {
	item := mineWorkItemEmit{
		Type:  "refactor",
		Title: "Reduce complexity: foo in bar.go (CC=20)",
		File:  "bar.go",
		Func:  "foo",
	}
	id := mineWorkItemID(item)
	if len(id) != 16 {
		t.Errorf("ID length = %d, want 16", len(id))
	}

	// Same File+Func with different title/severity should produce same ID
	item2 := mineWorkItemEmit{
		Type:     "refactor",
		Title:    "Different title",
		Severity: "low",
		File:     "bar.go",
		Func:     "foo",
	}
	id2 := mineWorkItemID(item2)
	if id != id2 {
		t.Errorf("ID changed when title/severity changed: %s vs %s", id, id2)
	}
}

func TestMineWorkItemID_NonHotspotUsesTitle(t *testing.T) {
	item := mineWorkItemEmit{
		Type:  "research",
		Title: "Orphaned research: old-topic.md",
	}
	id := mineWorkItemID(item)
	if len(id) != 16 {
		t.Errorf("ID length = %d, want 16", len(id))
	}

	// Different title should produce different ID
	item2 := mineWorkItemEmit{
		Type:  "research",
		Title: "Orphaned research: other-topic.md",
	}
	id2 := mineWorkItemID(item2)
	if id == id2 {
		t.Errorf("different titles produced same ID: %s", id)
	}
}

func TestMineWorkItemID_DifferentTypesProduceDifferentIDs(t *testing.T) {
	item1 := mineWorkItemEmit{Type: "refactor", Title: "same"}
	item2 := mineWorkItemEmit{Type: "research", Title: "same"}
	if mineWorkItemID(item1) == mineWorkItemID(item2) {
		t.Error("different types with same title should produce different IDs")
	}
}

func TestPrintMineSummary_GitOnly(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{
		Git: &GitFindings{
			CommitCount:      42,
			TopCoChangeFiles: []string{"a.go", "b.go"},
			RecurringFixes:   []string{"fix: typo"},
		},
	}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Mine complete.") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "42 commits") {
		t.Errorf("missing commit count in: %s", out)
	}
	if !strings.Contains(out, "2 co-change files") {
		t.Errorf("missing co-change files in: %s", out)
	}
	if !strings.Contains(out, "1 fix patterns") {
		t.Errorf("missing fix patterns in: %s", out)
	}
}

func TestPrintMineSummary_AgentsOnly(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{
		Agents: &AgentsFindings{
			TotalResearch:    10,
			OrphanedResearch: []string{"old.md"},
		},
	}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "10 research files") {
		t.Errorf("missing research count in: %s", out)
	}
	if !strings.Contains(out, "1 orphaned") {
		t.Errorf("missing orphaned count in: %s", out)
	}
}

func TestPrintMineSummary_CodeSkipped(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{
		Code: &CodeFindings{Skipped: true},
	}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "skipped (gocyclo not installed)") {
		t.Errorf("missing skipped message in: %s", out)
	}
}

func TestPrintMineSummary_CodeHotspots(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "foo", Complexity: 20, RecentEdits: 5},
				{File: "b.go", Func: "bar", Complexity: 15, RecentEdits: 3},
			},
		},
	}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "2 hotspots") {
		t.Errorf("missing hotspot count in: %s", out)
	}
}

func TestPrintMineSummary_Events(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{
		Events: &EventsFindings{
			RunsScanned: 5,
			TotalEvents: 100,
			ErrorEvents: []EventErrorSummary{{RunID: "r1", Message: "boom"}},
		},
	}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "5 runs scanned") {
		t.Errorf("missing runs scanned in: %s", out)
	}
	if !strings.Contains(out, "100 total events") {
		t.Errorf("missing total events in: %s", out)
	}
	if !strings.Contains(out, "1 errors") {
		t.Errorf("missing error count in: %s", out)
	}
}

func TestPrintMineSummary_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	r := &MineReport{}
	printMineSummary(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Mine complete.") {
		t.Error("missing header for empty report")
	}
	// Should not contain section-specific output
	if strings.Contains(out, "git:") || strings.Contains(out, "agents:") || strings.Contains(out, "code:") {
		t.Errorf("empty report should not have section output: %s", out)
	}
}

func TestPrintMineDryRun(t *testing.T) {
	var buf bytes.Buffer
	err := printMineDryRun(&buf, []string{"git", "agents"}, 24*7*3600*1e9) // 7d in ns... actually it's time.Duration
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("missing dry-run marker in: %s", out)
	}
	if !strings.Contains(out, "git, agents") {
		t.Errorf("missing sources in: %s", out)
	}
	if !strings.Contains(out, "No files will be written.") {
		t.Errorf("missing no-write message in: %s", out)
	}
}

// TestMineCommandJSON_ExactlyOnePayloadOnStdout is a regression guard for
// the ag-co7 double-write bug: `ao mine --json` must emit the JSON payload
// exactly once on stdout (no trailing duplicate, no prefix text). Piped
// consumers (`jq`, scripts, tests) require a single parseable document.
func TestMineCommandJSON_ExactlyOnePayloadOnStdout(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	chdirTo(t, dir)
	outputDir := filepath.Join(dir, "mine-output")

	out, err := executeCommand("mine", "--json", "--sources", "agents", "--output-dir", outputDir)
	if err != nil {
		t.Fatalf("mine --json: %v\noutput:\n%s", err, out)
	}

	// Must parse as a single JSON document with no trailing bytes.
	dec := json.NewDecoder(strings.NewReader(out))
	var first MineReport
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("first decode failed (stdout not a single JSON doc): %v\noutput:\n%s", err, out)
	}
	// Any further successful decode means the payload was written twice.
	var second MineReport
	if err := dec.Decode(&second); err == nil {
		t.Fatalf("stdout contained a second JSON payload (double-write regression):\n%s", out)
	}
	// There should also be no non-whitespace trailing bytes after the first doc.
	trailing := strings.TrimSpace(out[dec.InputOffset():])
	if trailing != "" {
		t.Fatalf("unexpected trailing bytes after single JSON payload: %q", trailing)
	}
	// And no human-readable summary leaked in.
	if strings.Contains(out, "Mine complete.") {
		t.Fatalf("human summary leaked onto stdout:\n%s", out)
	}
}

// TestMineCommandJSON_DryRunEmitsSingleJSONDocument guards the dry-run path:
// `ao mine --json --dry-run` should emit a single parseable JSON payload,
// not a human-readable text block, and definitely not both.
func TestMineCommandJSON_DryRunEmitsSingleJSONDocument(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	chdirTo(t, dir)

	out, err := executeCommand("mine", "--json", "--dry-run", "--sources", "git")
	if err != nil {
		t.Fatalf("mine --json --dry-run: %v\noutput:\n%s", err, out)
	}

	if strings.Contains(out, "[dry-run] ao mine") {
		t.Fatalf("dry-run text leaked onto --json stdout:\n%s", out)
	}

	dec := json.NewDecoder(strings.NewReader(out))
	var payload struct {
		DryRun    bool     `json:"dry_run"`
		Sources   []string `json:"sources"`
		OutputDir string   `json:"output_dir"`
	}
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("dry-run --json stdout is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if !payload.DryRun {
		t.Errorf("dry_run flag = false, want true")
	}
	if len(payload.Sources) != 1 || payload.Sources[0] != "git" {
		t.Errorf("sources = %v, want [git]", payload.Sources)
	}
	// Reject any second payload on stdout.
	var extra map[string]any
	if err := dec.Decode(&extra); err == nil {
		t.Fatalf("dry-run --json stdout contained a second payload:\n%s", out)
	}
}

func TestMineCommandJSONSuppressesHumanSummary(t *testing.T) {
	dir := t.TempDir()
	chdirTo(t, dir)
	outputDir := filepath.Join(dir, "mine-output")

	out, err := executeCommand("mine", "--json", "--sources", "agents", "--output-dir", outputDir)
	if err != nil {
		t.Fatalf("mine --json: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(out, "Mine complete.") {
		t.Fatalf("json stdout included human summary:\n%s", out)
	}

	var report MineReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("mine --json stdout is not a single JSON document: %v\noutput:\n%s", err, out)
	}
	if len(report.Sources) != 1 || report.Sources[0] != "agents" {
		t.Fatalf("sources = %v, want [agents]", report.Sources)
	}
	if report.Agents == nil {
		t.Fatal("agents findings missing from report")
	}

	if _, err := os.Stat(filepath.Join(outputDir, "latest.json")); err != nil {
		t.Fatalf("latest mine report missing: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(outputDir, "????-??-??-??.json"))
	if err != nil {
		t.Fatalf("glob dated mine report: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("dated mine reports = %d, want 1 (%v)", len(matches), matches)
	}
}

func TestCollectMineWorkItems_Empty(t *testing.T) {
	r := &MineReport{}
	items := collectMineWorkItems(r)
	if len(items) != 0 {
		t.Errorf("items = %d, want 0 for empty report", len(items))
	}
}

func TestCollectMineWorkItems_CodeHotspots(t *testing.T) {
	r := &MineReport{
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "big", Complexity: 25, RecentEdits: 10},
			},
		},
	}
	items := collectMineWorkItems(r)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Type != "refactor" {
		t.Errorf("Type = %q, want %q", items[0].Type, "refactor")
	}
	if items[0].Severity != "high" {
		t.Errorf("Severity = %q, want %q", items[0].Severity, "high")
	}
	if items[0].File != "a.go" {
		t.Errorf("File = %q, want %q", items[0].File, "a.go")
	}
	if items[0].ID == "" {
		t.Error("ID should be set")
	}
}

func TestCollectMineWorkItems_AgentsOrphanedResearch(t *testing.T) {
	r := &MineReport{
		Agents: &AgentsFindings{
			OrphanedResearch: []string{"stale-topic.md"},
		},
	}
	items := collectMineWorkItems(r)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Severity != "medium" {
		t.Errorf("Severity = %q, want %q", items[0].Severity, "medium")
	}
}

func TestReadDirContent(t *testing.T) {
	tmp := t.TempDir()
	// Create .md files and a non-.md file
	os.WriteFile(filepath.Join(tmp, "one.md"), []byte("content one"), 0o644)
	os.WriteFile(filepath.Join(tmp, "two.md"), []byte("content two"), 0o644)
	os.WriteFile(filepath.Join(tmp, "skip.txt"), []byte("ignored"), 0o644)
	os.Mkdir(filepath.Join(tmp, "subdir"), 0o755)

	contents, err := readDirContent(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("contents = %d entries, want 2", len(contents))
	}
	if contents["one.md"] != "content one" {
		t.Errorf("one.md = %q, want %q", contents["one.md"], "content one")
	}
	if contents["two.md"] != "content two" {
		t.Errorf("two.md = %q, want %q", contents["two.md"], "content two")
	}
}

func TestReadDirContent_NonexistentDir(t *testing.T) {
	_, err := readDirContent("/nonexistent-dir-12345")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLoadExistingMineIDs_NonexistentFile(t *testing.T) {
	ids, err := loadExistingMineIDs("/nonexistent-file-12345.jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ids = %d, want 0 for nonexistent file", len(ids))
	}
}

func TestLoadExistingMineIDs_WithEntries(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")

	entry := map[string]interface{}{
		"source_epic": "compile-mine",
		"consumed":    false,
		"items": []map[string]string{
			{"id": "abc123", "title": "test"},
			{"id": "def456", "title": "test2"},
		},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0o644)

	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ids = %d, want 2", len(ids))
	}
	if !ids["abc123"] {
		t.Error("missing id abc123")
	}
	if !ids["def456"] {
		t.Error("missing id def456")
	}
}

func TestLoadExistingMineIDs_ConsumedEntriesIgnored(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")

	entry := map[string]interface{}{
		"source_epic": "compile-mine",
		"consumed":    true,
		"items":       []map[string]string{{"id": "consumed1"}},
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(path, append(data, '\n'), 0o644)

	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("consumed entries should be ignored, got %d ids", len(ids))
	}
}

func TestLoadExistingMineIDs_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ids = %d, want 0 for empty file", len(ids))
	}
}

func TestWriteMineWorkItems(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "work.jsonl")

	items := []mineWorkItemEmit{
		{ID: "id1", Title: "item one", Type: "refactor", Severity: "high"},
		{ID: "id2", Title: "item two", Type: "research", Severity: "medium"},
	}

	err := writeMineWorkItems(path, items, "2026-03-11T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}

	// Verify first line structure
	var entry struct {
		SourceEpic  string             `json:"source_epic"`
		Timestamp   string             `json:"timestamp"`
		Items       []mineWorkItemEmit `json:"items"`
		Consumed    bool               `json:"consumed"`
		ClaimStatus string             `json:"claim_status"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if entry.SourceEpic != "compile-mine" {
		t.Errorf("source_epic = %q, want %q", entry.SourceEpic, "compile-mine")
	}
	if entry.Consumed {
		t.Error("consumed should be false")
	}
	if entry.ClaimStatus != "available" {
		t.Errorf("claim_status = %q, want %q", entry.ClaimStatus, "available")
	}
	if len(entry.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(entry.Items))
	}
	if entry.Items[0].ID != "id1" {
		t.Errorf("items[0].ID = %q, want %q", entry.Items[0].ID, "id1")
	}
}

func TestWriteMineWorkItems_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}

	tmp := t.TempDir()
	readOnlyDir := filepath.Join(tmp, "readonly")
	os.Mkdir(readOnlyDir, 0o555)
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0o755) })

	path := filepath.Join(readOnlyDir, "work.jsonl")
	items := []mineWorkItemEmit{{ID: "id1", Title: "test"}}
	err := writeMineWorkItems(path, items, "2026-03-11T00:00:00Z")
	if err == nil {
		t.Error("expected error writing to read-only directory")
	}
}

func TestEmitMineWorkItems_EmptyReport(t *testing.T) {
	tmp := t.TempDir()
	r := &MineReport{}
	err := emitMineWorkItems(tmp, r)
	if err != nil {
		t.Fatalf("unexpected error for empty report: %v", err)
	}
	// No file should be created
	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	if _, err := os.Stat(path); err == nil {
		t.Error("next-work.jsonl should not be created for empty report")
	}
}

func TestEmitMineWorkItems_WithHotspots(t *testing.T) {
	tmp := t.TempDir()
	r := &MineReport{
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "bigFunc", Complexity: 30, RecentEdits: 5},
			},
		},
	}
	err := emitMineWorkItems(tmp, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading next-work.jsonl: %v", err)
	}
	if !strings.Contains(string(data), "bigFunc") {
		t.Errorf("expected bigFunc in output, got: %s", string(data))
	}
}

func TestEmitMineWorkItems_DeduplicatesExisting(t *testing.T) {
	tmp := t.TempDir()
	r := &MineReport{
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "bigFunc", Complexity: 30, RecentEdits: 5},
			},
		},
	}

	// First emit
	err := emitMineWorkItems(tmp, r)
	if err != nil {
		t.Fatalf("first emit: %v", err)
	}

	// Second emit — same items should be deduplicated
	err = emitMineWorkItems(tmp, r)
	if err != nil {
		t.Fatalf("second emit: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Should still be 1 line — dedup prevents re-emit
	if len(lines) != 1 {
		t.Errorf("lines = %d, want 1 (dedup should prevent re-emit)", len(lines))
	}
}

func TestMineAgentsDir_NoResearchDir(t *testing.T) {
	dir := t.TempDir()
	findings, err := mineAgentsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 0 {
		t.Errorf("TotalResearch = %d, want 0", findings.TotalResearch)
	}
	if len(findings.OrphanedResearch) != 0 {
		t.Errorf("OrphanedResearch = %d, want 0", len(findings.OrphanedResearch))
	}
}

func TestMineAgentsDir_AllReferenced(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create research file
	if err := os.WriteFile(filepath.Join(researchDir, "2026-01-01-auth.md"), []byte("# Auth Research"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create learning that references the research
	if err := os.WriteFile(filepath.Join(learningsDir, "2026-01-02-auth-learning.md"),
		[]byte("Based on 2026-01-01-auth.md findings."), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := mineAgentsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 1 {
		t.Errorf("TotalResearch = %d, want 1", findings.TotalResearch)
	}
	if len(findings.OrphanedResearch) != 0 {
		t.Errorf("OrphanedResearch = %v, want empty (research is referenced)", findings.OrphanedResearch)
	}
}

func TestMineAgentsDir_OrphanedResearch(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create research file with no referencing learning
	if err := os.WriteFile(filepath.Join(researchDir, "2026-01-01-orphan.md"), []byte("# Orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := mineAgentsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 1 {
		t.Errorf("TotalResearch = %d, want 1", findings.TotalResearch)
	}
	if len(findings.OrphanedResearch) != 1 {
		t.Errorf("OrphanedResearch len = %d, want 1", len(findings.OrphanedResearch))
	}
	if len(findings.OrphanedResearch) > 0 && findings.OrphanedResearch[0] != "2026-01-01-orphan.md" {
		t.Errorf("OrphanedResearch[0] = %q, want %q", findings.OrphanedResearch[0], "2026-01-01-orphan.md")
	}
}

// ---------------------------------------------------------------------------
// parseMineWindow
// ---------------------------------------------------------------------------

func TestParseMineWindow(t *testing.T) {
	tests := []struct {
		input   string
		want    int64 // in hours for comparison
		wantErr bool
	}{
		{"7d", 168, false},
		{"1d", 24, false},
		{"24h", 24, false},
		{"2h", 2, false},
		{"30m", 0, false}, // 30 minutes = 0 full hours
		{"", 0, true},
		{"0d", 0, true},
		{"-1h", 0, true},
		{"abc", 0, true},
		{"10x", 0, true},
		{"d", 0, true},    // no number
		{"3.5d", 0, true}, // non-integer
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMineWindow(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseMineWindow(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMineWindow(%q) error = %v", tt.input, err)
			}
			gotHours := int64(got.Hours())
			if gotHours != tt.want {
				t.Errorf("parseMineWindow(%q) = %d hours, want %d", tt.input, gotHours, tt.want)
			}
		})
	}
}

func TestParseMineWindow_MinutesPrecision(t *testing.T) {
	got, err := parseMineWindow("30m")
	if err != nil {
		t.Fatalf("parseMineWindow(\"30m\") error = %v", err)
	}
	if got.Minutes() != 30 {
		t.Errorf("parseMineWindow(\"30m\") = %v minutes, want 30", got.Minutes())
	}
}

// ---------------------------------------------------------------------------
// splitSources
// ---------------------------------------------------------------------------

func TestSplitSources(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"git", []string{"git"}, false},
		{"git,agents", []string{"git", "agents"}, false},
		{"git, agents, code", []string{"git", "agents", "code"}, false},
		{"events", []string{"events"}, false},
		{"git,code,agents,events", []string{"git", "code", "agents", "events"}, false},
		{"", nil, true},
		{",,,", nil, true},
		{"invalid", nil, true},
		{"git,invalid", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := splitSources(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("splitSources(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitSources(%q) error = %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitSources(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitSources(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// countRecentEdits
// ---------------------------------------------------------------------------

func TestCountRecentEdits_InGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("git operations in short mode")
	}
	dir := initGitHistoryFixtureRepo(t, []gitCommitFixture{
		{Path: "main.go", Content: "package main\n", Message: "add main", Timestamp: time.Now().Add(-1 * time.Hour)},
		{Path: "main.go", Content: "package main\n// v2\n", Message: "update main", Timestamp: time.Now().Add(-30 * time.Minute)},
	})

	count := countRecentEdits(dir, "main.go", 24*time.Hour)
	if count != 2 {
		t.Errorf("countRecentEdits = %d, want 2", count)
	}
}

func TestCountRecentEdits_NoEditsInWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("git operations in short mode")
	}
	dir := initGitHistoryFixtureRepo(t, []gitCommitFixture{
		{Path: "old.go", Content: "package old\n", Message: "ancient commit", Timestamp: time.Now().Add(-365 * 24 * time.Hour)},
	})

	count := countRecentEdits(dir, "old.go", 1*time.Hour)
	if count != 0 {
		t.Errorf("countRecentEdits = %d, want 0 (commit outside window)", count)
	}
}

func TestCountRecentEdits_NonexistentFile(t *testing.T) {
	if testing.Short() {
		t.Skip("git operations in short mode")
	}
	dir := initTestRepo(t)
	count := countRecentEdits(dir, "nonexistent.go", 24*time.Hour)
	if count != 0 {
		t.Errorf("countRecentEdits = %d, want 0 for nonexistent file", count)
	}
}

func TestMineAgentsDir_EmptyResearchDir(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	findings, err := mineAgentsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 0 {
		t.Errorf("TotalResearch = %d, want 0", findings.TotalResearch)
	}
}

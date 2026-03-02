package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseMineWindow_ValidDurations(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"26h", 26 * time.Hour},
		{"7d", 168 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h", 1 * time.Hour},
		{"1d", 24 * time.Hour},
		{"5m", 5 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMineWindow(tt.input)
			if err != nil {
				t.Fatalf("parseMineWindow(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseMineWindow(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMineWindow_Invalid(t *testing.T) {
	tests := []string{
		"foo",
		"",
		"0h",
		"-1d",
		"abc",
		"7x",
		"d",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseMineWindow(input)
			if err == nil {
				t.Errorf("parseMineWindow(%q) expected error, got nil", input)
			}
		})
	}
}

func TestSplitSources_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"git,agents,code", []string{"git", "agents", "code"}},
		{"git,agents", []string{"git", "agents"}},
		{"code", []string{"code"}},
		{"git", []string{"git"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := splitSources(tt.input)
			if err != nil {
				t.Fatalf("splitSources(%q) error: %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitSources(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("splitSources(%q)[%d] = %q, want %q", tt.input, i, g, tt.want[i])
				}
			}
		})
	}
}

func TestSplitSources_UnknownSource(t *testing.T) {
	tests := []string{
		"git,fake",
		"unknown",
		"git,agents,xyz",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := splitSources(input)
			if err == nil {
				t.Errorf("splitSources(%q) expected error, got nil", input)
			}
		})
	}
}

func TestMineAgentsDir_OrphanDetection(t *testing.T) {
	tmp := t.TempDir()

	// Create .agents/research/ with two files
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "topic-a.md"), []byte("# Topic A\nSome research."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "topic-b.md"), []byte("# Topic B\nMore research."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create .agents/learnings/ with a file that references topic-a.md only
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "learning-1.md"), []byte("# Learning\nBased on topic-a.md research."), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := mineAgentsDir(tmp)
	if err != nil {
		t.Fatalf("mineAgentsDir error: %v", err)
	}

	if findings.TotalResearch != 2 {
		t.Errorf("TotalResearch = %d, want 2", findings.TotalResearch)
	}

	if len(findings.OrphanedResearch) != 1 {
		t.Fatalf("OrphanedResearch count = %d, want 1", len(findings.OrphanedResearch))
	}
	if findings.OrphanedResearch[0] != "topic-b.md" {
		t.Errorf("OrphanedResearch[0] = %q, want %q", findings.OrphanedResearch[0], "topic-b.md")
	}
}

func TestMineAgentsDir_NoResearchDir(t *testing.T) {
	tmp := t.TempDir()
	findings, err := mineAgentsDir(tmp)
	if err != nil {
		t.Fatalf("mineAgentsDir error: %v", err)
	}
	if findings.TotalResearch != 0 {
		t.Errorf("TotalResearch = %d, want 0", findings.TotalResearch)
	}
	if len(findings.OrphanedResearch) != 0 {
		t.Errorf("OrphanedResearch count = %d, want 0", len(findings.OrphanedResearch))
	}
}

func TestMineGitLog_NoGit(t *testing.T) {
	tmp := t.TempDir()
	findings, err := mineGitLog(tmp, 26*time.Hour)
	// No git repo → error propagated from git log failure
	if err == nil {
		t.Fatal("expected error from mineGitLog in non-git directory, got nil")
	}
	if findings != nil {
		t.Errorf("expected nil findings on error, got %+v", findings)
	}
}

func TestWriteMineReport_CreatesLatest(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "mine-output")

	report := &MineReport{
		Timestamp:    time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC),
		SinceSeconds: 93600,
		Sources:      []string{"git"},
		Git: &GitFindings{
			CommitCount: 5,
		},
	}

	if err := writeMineReport(outDir, report); err != nil {
		t.Fatalf("writeMineReport error: %v", err)
	}

	// Check dated file exists
	datedPath := filepath.Join(outDir, "2026-03-01-14.json")
	if _, err := os.Stat(datedPath); os.IsNotExist(err) {
		t.Errorf("dated file not created: %s", datedPath)
	}

	// Check latest.json exists
	latestPath := filepath.Join(outDir, "latest.json")
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("read latest.json: %v", err)
	}

	var decoded MineReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal latest.json: %v", err)
	}
	if decoded.Git.CommitCount != 5 {
		t.Errorf("decoded CommitCount = %d, want 5", decoded.Git.CommitCount)
	}
	if decoded.SinceSeconds != 93600 {
		t.Errorf("decoded SinceSeconds = %d, want 93600", decoded.SinceSeconds)
	}
}

func TestRunMine_DryRun(t *testing.T) {
	// Save and restore global state
	oldDryRun := dryRun
	oldSources := mineSourcesFlag
	oldSince := mineSince
	oldOutput := mineOutputDir
	defer func() {
		dryRun = oldDryRun
		mineSourcesFlag = oldSources
		mineSince = oldSince
		mineOutputDir = oldOutput
	}()

	tmp := t.TempDir()
	dryRun = true
	mineSourcesFlag = "git,agents"
	mineSince = "26h"
	mineOutputDir = filepath.Join(tmp, "mine-output")

	var buf bytes.Buffer
	cmd := mineCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := runMine(cmd, nil); err != nil {
		t.Fatalf("runMine dry-run error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected dry-run output, got empty string")
	}

	// Verify no files were written
	if _, err := os.Stat(filepath.Join(tmp, "mine-output")); !os.IsNotExist(err) {
		t.Error("dry-run should not create output directory")
	}
}

func TestRunMine_JSONOutputIsPureJSON(t *testing.T) {
	// Save and restore global state
	oldDryRun := dryRun
	oldSources := mineSourcesFlag
	oldSince := mineSince
	oldOutputDir := mineOutputDir
	oldQuiet := mineQuiet
	oldEmit := mineEmitWorkItems
	oldOutput := output
	defer func() {
		dryRun = oldDryRun
		mineSourcesFlag = oldSources
		mineSince = oldSince
		mineOutputDir = oldOutputDir
		mineQuiet = oldQuiet
		mineEmitWorkItems = oldEmit
		output = oldOutput
	}()

	tmp := t.TempDir()
	chdirTo(t, tmp)

	dryRun = false
	mineSourcesFlag = "agents"
	mineSince = "26h"
	mineOutputDir = filepath.Join(tmp, "mine-output")
	mineQuiet = false
	mineEmitWorkItems = false
	output = "json"

	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd := mineCmd
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	if err := runMine(cmd, nil); err != nil {
		t.Fatalf("runMine json output error: %v (stderr=%s)", err, errBuf.String())
	}

	raw := out.String()
	if strings.Contains(raw, "Mine complete.") {
		t.Fatalf("json output contains human summary text: %q", raw)
	}

	var report MineReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("runMine output should be valid JSON: %v\noutput: %s", err, raw)
	}

	if report.Agents == nil {
		t.Fatal("expected agents findings in JSON output")
	}
}

func TestPrintMineDryRun(t *testing.T) {
	var buf bytes.Buffer
	sources := []string{"git", "agents"}
	window := 26 * time.Hour

	if err := printMineDryRun(&buf, sources, window); err != nil {
		t.Fatalf("printMineDryRun error: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Error("expected output, got empty string")
	}
	if !bytes.Contains([]byte(out), []byte("dry-run")) {
		t.Error("expected output to contain 'dry-run'")
	}
	if !bytes.Contains([]byte(out), []byte("git")) {
		t.Error("expected output to contain 'git'")
	}
}

func TestReadDirContent_MissingDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := readDirContent(filepath.Join(tmp, "nonexistent"))
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// emitMineWorkItems
// ---------------------------------------------------------------------------

func makeTestMineReport(orphans []string, hotspots []ComplexityHotspot) *MineReport {
	r := &MineReport{
		Timestamp:    time.Now().UTC(),
		SinceSeconds: 93600,
		Sources:      []string{"agents", "code"},
	}
	if orphans != nil {
		r.Agents = &AgentsFindings{
			TotalResearch:    len(orphans),
			OrphanedResearch: orphans,
		}
	}
	if hotspots != nil {
		r.Code = &CodeFindings{Hotspots: hotspots}
	}
	return r
}

func TestEmitMineWorkItems_EmptyFindings(t *testing.T) {
	tmp := t.TempDir()
	r := makeTestMineReport(nil, nil)
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No next-work.jsonl should be created
	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no next-work.jsonl for empty findings")
	}
}

func TestEmitMineWorkItems_OrphanSeverityMedium(t *testing.T) {
	tmp := t.TempDir()
	r := makeTestMineReport([]string{"old-research.md"}, nil)
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("emit error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read next-work.jsonl: %v", err)
	}

	var entry struct {
		SourceEpic string `json:"source_epic"`
		Items      []struct {
			Severity string `json:"severity"`
			Type     string `json:"type"`
			Source   string `json:"source"`
			Title    string `json:"title"`
		} `json:"items"`
		Consumed bool `json:"consumed"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.SourceEpic != "athena-mine" {
		t.Errorf("source_epic = %q, want athena-mine", entry.SourceEpic)
	}
	if entry.Consumed {
		t.Error("emitted entry should not be consumed")
	}
	if len(entry.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(entry.Items))
	}
	item := entry.Items[0]
	if item.Severity != "medium" {
		t.Errorf("orphan severity = %q, want medium", item.Severity)
	}
	if item.Type != "knowledge-gap" {
		t.Errorf("orphan type = %q, want knowledge-gap", item.Type)
	}
	if item.Source != "athena-mine" {
		t.Errorf("item source = %q, want athena-mine", item.Source)
	}
}

func TestEmitMineWorkItems_HotspotSeverityHigh(t *testing.T) {
	tmp := t.TempDir()
	hotspot := ComplexityHotspot{File: "notebook.go", Func: "buildLastSessionSection", Complexity: 24, RecentEdits: 5}
	r := makeTestMineReport(nil, []ComplexityHotspot{hotspot})
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("emit error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, _ := os.ReadFile(path)
	var entry struct {
		Items []struct {
			Severity string `json:"severity"`
			Type     string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entry.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(entry.Items))
	}
	if entry.Items[0].Severity != "high" {
		t.Errorf("hotspot severity = %q, want high", entry.Items[0].Severity)
	}
	if entry.Items[0].Type != "refactor" {
		t.Errorf("hotspot type = %q, want refactor", entry.Items[0].Type)
	}
}

func TestEmitMineWorkItems_DeduplicatesUnconsumedEntry(t *testing.T) {
	tmp := t.TempDir()
	r := makeTestMineReport([]string{"orphan.md"}, nil)

	// First emit
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("first emit error: %v", err)
	}
	// Second emit — should be a no-op due to dedup
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("second emit error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, _ := os.ReadFile(path)
	lines := 0
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("expected 1 line after dedup, got %d", lines)
	}
}

func TestEmitMineWorkItems_EmitsAfterConsumedEntry(t *testing.T) {
	tmp := t.TempDir()
	// Pre-populate with a consumed athena-mine entry
	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	consumed := `{"source_epic":"athena-mine","timestamp":"2026-01-01T00:00:00Z","items":[],"consumed":true,"consumed_by":null,"consumed_at":null}` + "\n"
	if err := os.WriteFile(path, []byte(consumed), 0o640); err != nil {
		t.Fatal(err)
	}

	r := makeTestMineReport([]string{"new-orphan.md"}, nil)
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("emit error: %v", err)
	}

	data, _ := os.ReadFile(path)
	lines := 0
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("expected 2 lines (consumed + new), got %d", lines)
	}
}

func TestEmitMineWorkItems_HotspotsBeforeOrphans(t *testing.T) {
	tmp := t.TempDir()
	hotspot := ComplexityHotspot{File: "foo.go", Func: "heavy", Complexity: 20, RecentEdits: 2}
	r := makeTestMineReport([]string{"orphan.md"}, []ComplexityHotspot{hotspot})
	if err := emitMineWorkItems(tmp, r); err != nil {
		t.Fatalf("emit error: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (one per item), got %d", len(lines))
	}

	// Parse each line
	type entryItem struct {
		Items []struct {
			Severity string `json:"severity"`
		} `json:"items"`
	}
	var first, second entryItem
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].Severity != "high" {
		t.Errorf("first line should be hotspot (high), got %v", first.Items)
	}
	if len(second.Items) != 1 || second.Items[0].Severity != "medium" {
		t.Errorf("second line should be orphan (medium), got %v", second.Items)
	}
}

func TestMineWorkItemID_Deterministic(t *testing.T) {
	// Hotspot item with File+Func: ID should be based on File+Func, not Title
	item := mineWorkItemEmit{
		Title: "Reduce complexity: foo in bar.go (CC=20)",
		Type:  "refactor",
		File:  "bar.go",
		Func:  "foo",
	}
	id1 := mineWorkItemID(item)
	id2 := mineWorkItemID(item)
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %q and %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("expected 16-char ID, got %d chars: %q", len(id1), id1)
	}

	// Non-hotspot item (no File/Func): ID should be based on Title
	orphanItem := mineWorkItemEmit{
		Title: "Rescue orphan: old-research.md",
		Type:  "knowledge-gap",
	}
	oid1 := mineWorkItemID(orphanItem)
	oid2 := mineWorkItemID(orphanItem)
	if oid1 != oid2 {
		t.Errorf("expected deterministic IDs for orphan, got %q and %q", oid1, oid2)
	}
	if len(oid1) != 16 {
		t.Errorf("expected 16-char ID for orphan, got %d chars: %q", len(oid1), oid1)
	}
}

func TestMineWorkItemID_DifferentInputs(t *testing.T) {
	a := mineWorkItemEmit{Title: "Item A", Type: "refactor"}
	b := mineWorkItemEmit{Title: "Item B", Type: "refactor"}
	if mineWorkItemID(a) == mineWorkItemID(b) {
		t.Error("different items should produce different IDs")
	}
}

func TestEmitMineWorkItems_ItemLevelDedup(t *testing.T) {
	tmp := t.TempDir()

	// First emit: orphan A and hotspot B
	r1 := makeTestMineReport(
		[]string{"orphan-a.md"},
		[]ComplexityHotspot{{File: "foo.go", Func: "bar", Complexity: 20, RecentEdits: 3}},
	)
	if err := emitMineWorkItems(tmp, r1); err != nil {
		t.Fatalf("first emit: %v", err)
	}

	path := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data1, _ := os.ReadFile(path)
	lines1 := countNonEmptyLines(data1)
	if lines1 != 2 {
		t.Fatalf("after first emit: expected 2 lines, got %d", lines1)
	}

	// Second emit: same orphan A + new orphan C — only C should be added
	r2 := makeTestMineReport([]string{"orphan-a.md", "orphan-c.md"}, nil)
	if err := emitMineWorkItems(tmp, r2); err != nil {
		t.Fatalf("second emit: %v", err)
	}

	data2, _ := os.ReadFile(path)
	lines2 := countNonEmptyLines(data2)
	if lines2 != 3 {
		t.Fatalf("after second emit: expected 3 lines (2 original + 1 new), got %d", lines2)
	}
}

func TestWriteMineReport_EmptyDir(t *testing.T) {
	// writeMineReport with empty dir should return an explicit error,
	// NOT silently write to "." (current working directory).
	// filepath.Clean("") returns "." so without an explicit guard,
	// MkdirAll(filepath.Clean("")) would succeed and write to CWD.
	err := writeMineReport("", &MineReport{})
	if err == nil {
		t.Fatal("expected error for empty output dir, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("expected 'must not be empty' in error, got: %v", err)
	}
}

func TestCountRecentEdits_UsesWindowParam(t *testing.T) {
	// Verify the window parameter actually reaches git log --since.
	// A zero-duration window should count zero edits (--since=0 seconds ago = epoch, but
	// in a fresh temp dir with no git repo, countRecentEdits returns 0 on error).
	// With a large window in a real git repo, it should find edits.
	// We test indirectly: the function signature accepts window and doesn't panic.
	tmp := t.TempDir()

	// No git repo → returns 0 regardless of window (git fails silently)
	edits := countRecentEdits(tmp, "nonexistent.go", 26*time.Hour)
	if edits != 0 {
		t.Errorf("expected 0 edits in non-git dir, got %d", edits)
	}

	// Verify different windows don't panic
	edits2 := countRecentEdits(tmp, "nonexistent.go", 7*24*time.Hour)
	if edits2 != 0 {
		t.Errorf("expected 0 edits with 7d window in non-git dir, got %d", edits2)
	}
}

func TestMineCodeComplexity_AcceptsWindow(t *testing.T) {
	// Verify mineCodeComplexity accepts the window param without error.
	// In a temp dir with no Go code, it should return skipped or empty findings.
	tmp := t.TempDir()
	findings, err := mineCodeComplexity(tmp, 26*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Either skipped (no gocyclo) or empty hotspots (no Go files)
	if !findings.Skipped && len(findings.Hotspots) != 0 {
		t.Errorf("expected skipped or empty hotspots, got %d", len(findings.Hotspots))
	}
}

func countNonEmptyLines(data []byte) int {
	count := 0
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// mineGitLog error includes stderr context (Finding 1)
// ---------------------------------------------------------------------------

func TestMineGitLog_ErrorIncludesStderr(t *testing.T) {
	tmp := t.TempDir()
	_, err := mineGitLog(tmp, 26*time.Hour)
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "git log") {
		t.Errorf("error should contain 'git log' prefix, got: %v", err)
	}
	if !strings.Contains(errMsg, "fatal") && !strings.Contains(errMsg, "not a git repository") {
		t.Errorf("error should include git stderr for debugging, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadExistingMineIDs error handling
// ---------------------------------------------------------------------------

func TestLoadExistingMineIDs_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")
	content := `{"source_epic":"athena-mine","consumed":false,"items":[{"id":"abc123"}]}` + "\n" +
		`{this is not valid json` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("invalid lines should be skipped, got error: %v", err)
	}
	if !ids["abc123"] {
		t.Error("valid line's ID should be present")
	}
}

func TestLoadExistingMineIDs_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("empty file should return nil error, got: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty map, got %d entries", len(ids))
	}
}

func TestLoadExistingMineIDs_SkipsEmptyIDs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")
	content := `{"source_epic":"athena-mine","consumed":false,"items":[{"id":""},{"id":"valid123"}]}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ids, err := loadExistingMineIDs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 ID (empty should be skipped), got %d", len(ids))
	}
	if !ids["valid123"] {
		t.Error("expected 'valid123' to be present")
	}
}

func TestLoadExistingMineIDs_ErrorMessageNotDoubleWrapped(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission denied as root")
	}
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rpiDir, "next-work.jsonl")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	r := makeTestMineReport([]string{"orphan.md"}, nil)
	err := emitMineWorkItems(tmp, r)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	errMsg := err.Error()
	if strings.Count(errMsg, "mine IDs") > 1 {
		t.Errorf("error message is double-wrapped: %v", err)
	}
}

func TestLoadExistingMineIDs_NotExist(t *testing.T) {
	ids, err := loadExistingMineIDs(filepath.Join(t.TempDir(), "nonexistent.jsonl"))
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty map, got %d entries", len(ids))
	}
}

func TestLoadExistingMineIDs_Unreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	u, err := user.Current()
	if err == nil && u.Uid == "0" {
		t.Skip("running as root — permission test would not fail")
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "next-work.jsonl")
	if err := os.WriteFile(path, []byte(`{"source_epic":"athena-mine"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	ids, err := loadExistingMineIDs(path)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
	if ids != nil {
		t.Errorf("expected nil map on error, got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// Stable dedup IDs
// ---------------------------------------------------------------------------

func TestMineWorkItemID_StableAcrossCCChanges(t *testing.T) {
	itemCC20 := mineWorkItemEmit{
		Title: "Reduce complexity: heavy in foo.go (CC=20)",
		Type:  "refactor",
		File:  "foo.go",
		Func:  "heavy",
	}
	itemCC25 := mineWorkItemEmit{
		Title: "Reduce complexity: heavy in foo.go (CC=25)",
		Type:  "refactor",
		File:  "foo.go",
		Func:  "heavy",
	}
	id20 := mineWorkItemID(itemCC20)
	id25 := mineWorkItemID(itemCC25)
	if id20 != id25 {
		t.Errorf("same File+Func with different CC should produce same ID, got %q vs %q", id20, id25)
	}
}

func TestCollectMineWorkItems_HotspotPopulatesFileFunc(t *testing.T) {
	hotspot := ComplexityHotspot{File: "src/main.go", Func: "runServer", Complexity: 18, RecentEdits: 4}
	r := makeTestMineReport(nil, []ComplexityHotspot{hotspot})
	items := collectMineWorkItems(r)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].File != "src/main.go" {
		t.Errorf("File = %q, want %q", items[0].File, "src/main.go")
	}
	if items[0].Func != "runServer" {
		t.Errorf("Func = %q, want %q", items[0].Func, "runServer")
	}
}

func TestCollectMineWorkItems_OrphanDoesNotSetFileFunc(t *testing.T) {
	r := makeTestMineReport([]string{"stale-research.md"}, nil)
	items := collectMineWorkItems(r)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].File != "" {
		t.Errorf("orphan File = %q, want empty", items[0].File)
	}
	if items[0].Func != "" {
		t.Errorf("orphan Func = %q, want empty", items[0].Func)
	}
}

func TestMineWorkItemID_PartialFileFunc_FallsBackToTitle(t *testing.T) {
	fileOnly := mineWorkItemEmit{Title: "Some title", Type: "refactor", File: "foo.go"}
	titleOnly := mineWorkItemEmit{Title: "Some title", Type: "refactor"}
	if mineWorkItemID(fileOnly) != mineWorkItemID(titleOnly) {
		t.Error("File-only item should fall back to Title-based ID")
	}

	funcOnly := mineWorkItemEmit{Title: "Some title", Type: "refactor", Func: "bar"}
	if mineWorkItemID(funcOnly) != mineWorkItemID(titleOnly) {
		t.Error("Func-only item should fall back to Title-based ID")
	}
}

func TestMineWorkItemID_NoCollisionOnFieldBoundaries(t *testing.T) {
	a := mineWorkItemEmit{Type: "ab", Title: "cd"}
	b := mineWorkItemEmit{Type: "a", Title: "bcd"}
	idA := mineWorkItemID(a)
	idB := mineWorkItemID(b)
	if idA == idB {
		t.Errorf("field boundary shift should produce different IDs, both got %q", idA)
	}
}

func TestMineWorkItemID_NoCollisionOnFileFuncBoundary(t *testing.T) {
	a := mineWorkItemEmit{Type: "refactor", File: "src/ab", Func: "cd"}
	b := mineWorkItemEmit{Type: "refactor", File: "src/a", Func: "bcd"}
	if mineWorkItemID(a) == mineWorkItemID(b) {
		t.Error("File/Func boundary shift should produce different IDs")
	}
}

func TestMineWorkItemID_AlgorithmV2_BreaksBackcompat(t *testing.T) {
	// Document: ID algorithm changed in v2 (PR #64).
	// Old algorithm: sha256(Title + Type)[:16]
	// New algorithm: sha256(Type + \0 + File + \0 + Func)[:16] for hotspots,
	//                sha256(Type + \0 + Title)[:16] for others.
	// Existing next-work.jsonl entries will be re-emitted once after upgrade.
	// This is accepted as a one-time cost for stable dedup going forward.

	item := mineWorkItemEmit{
		Title: "Reduce complexity: heavy in foo.go (CC=20)",
		Type:  "refactor",
		File:  "foo.go",
		Func:  "heavy",
	}

	// Compute what the OLD algorithm would produce
	oldH := sha256.New()
	oldH.Write([]byte(item.Title))
	oldH.Write([]byte(item.Type))
	oldID := hex.EncodeToString(oldH.Sum(nil))[:16]

	newID := mineWorkItemID(item)

	if oldID == newID {
		t.Fatal("new algorithm should produce different IDs than old — if this fails, the algorithm didn't actually change")
	}
	t.Logf("Old ID: %s, New ID: %s — one-time dedup miss accepted", oldID, newID)

	// Orphan items also changed: old sha256(Title+Type) vs new sha256(Type+\0+Title)
	orphan := mineWorkItemEmit{
		Title: "Rescue orphan: old-research.md",
		Type:  "knowledge-gap",
	}
	oldOrphanH := sha256.New()
	oldOrphanH.Write([]byte(orphan.Title))
	oldOrphanH.Write([]byte(orphan.Type))
	oldOrphanID := hex.EncodeToString(oldOrphanH.Sum(nil))[:16]

	newOrphanID := mineWorkItemID(orphan)

	if oldOrphanID == newOrphanID {
		t.Fatal("orphan IDs should also differ after algorithm change")
	}
	t.Logf("Orphan Old ID: %s, New ID: %s — one-time dedup miss accepted", oldOrphanID, newOrphanID)
}

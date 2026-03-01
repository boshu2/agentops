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
	if err != nil {
		t.Fatalf("mineGitLog error: %v", err)
	}
	// No git repo → empty findings, no error
	if findings.CommitCount != 0 {
		t.Errorf("CommitCount = %d, want 0", findings.CommitCount)
	}
	if len(findings.TopCoChangeFiles) != 0 {
		t.Errorf("TopCoChangeFiles count = %d, want 0", len(findings.TopCoChangeFiles))
	}
	if len(findings.RecurringFixes) != 0 {
		t.Errorf("RecurringFixes count = %d, want 0", len(findings.RecurringFixes))
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
	item := mineWorkItemEmit{Title: "Reduce complexity: foo in bar.go (CC=20)", Type: "refactor"}
	id1 := mineWorkItemID(item)
	id2 := mineWorkItemID(item)
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %q and %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("expected 16-char ID, got %d chars: %q", len(id1), id1)
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

func countNonEmptyLines(data []byte) int {
	count := 0
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			count++
		}
	}
	return count
}

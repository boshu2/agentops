package overnight

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFinding drops a minimal but well-formed finding markdown file at
// <cwd>/.agents/findings/<name>.
func writeFinding(t *testing.T, cwd, name, title, summary string) {
	t.Helper()
	dir := filepath.Join(cwd, ".agents", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir findings: %v", err)
	}
	body := `---
id: "` + strings.TrimSuffix(name, ".md") + `"
type: "finding"
title: "` + title + `"
severity: "significant"
status: "active"
---
# Finding: ` + title + `

## Summary
` + summary + `

## Pattern
Something happened.
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write finding %s: %v", name, err)
	}
}

// readNextWorkLines returns the raw JSONL lines written to next-work.jsonl
// under cwd, ignoring blank lines.
func readNextWorkLines(t *testing.T, cwd string) []string {
	t.Helper()
	path := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read next-work.jsonl: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func TestRouteFindings_MissingFindingsDir_SoftFail(t *testing.T) {
	cwd := t.TempDir()
	routed, degraded, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if routed != 0 {
		t.Fatalf("expected routed=0, got %d", routed)
	}
	foundMarker := false
	for _, d := range degraded {
		if d == "no findings dir" {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Fatalf("expected degraded to contain 'no findings dir', got %v", degraded)
	}
}

func TestRouteFindings_FirstCall_RoutesAllUnseen(t *testing.T) {
	cwd := t.TempDir()
	writeFinding(t, cwd, "f-2026-03-22-001.md", "Stale artifact dir", "Audits can cite stale artifact directories.")
	writeFinding(t, cwd, "f-2026-03-22-002.md", "Release tag drift", "Release tags drift from committed changelog.")
	writeFinding(t, cwd, "f-2026-03-22-003.md", "Pre-push bypass", "Pre-push hook is bypassed by env var.")

	routed, _, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("RouteFindings: %v", err)
	}
	if routed != 3 {
		t.Fatalf("expected routed=3, got %d", routed)
	}

	lines := readNextWorkLines(t, cwd)
	if len(lines) != 1 {
		t.Fatalf("expected 1 appended line, got %d", len(lines))
	}

	var parsed struct {
		SourceEpic string          `json:"source_epic"`
		Items      []routedFinding `json:"items"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if parsed.SourceEpic != "dream-findings-router" {
		t.Fatalf("expected source_epic=dream-findings-router, got %q", parsed.SourceEpic)
	}
	if len(parsed.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(parsed.Items))
	}
	wantIDs := map[string]bool{
		"f-2026-03-22-001": false,
		"f-2026-03-22-002": false,
		"f-2026-03-22-003": false,
	}
	for _, it := range parsed.Items {
		if _, ok := wantIDs[it.ID]; !ok {
			t.Fatalf("unexpected id %s", it.ID)
		}
		wantIDs[it.ID] = true
		if it.Type != "tech-debt" {
			t.Fatalf("expected type=tech-debt, got %s", it.Type)
		}
		if it.Source != "council-finding" {
			t.Fatalf("expected source=council-finding, got %s", it.Source)
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing id %s", id)
		}
	}
}

func TestRouteFindings_SecondCall_IsNoOp(t *testing.T) {
	cwd := t.TempDir()
	writeFinding(t, cwd, "f-2026-04-03-001.md", "Title", "Summary body.")

	routed, _, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if routed != 1 {
		t.Fatalf("first call expected 1, got %d", routed)
	}

	routed2, _, err2 := RouteFindings(cwd)
	if err2 != nil {
		t.Fatalf("second call: %v", err2)
	}
	if routed2 != 0 {
		t.Fatalf("expected second call routed=0, got %d", routed2)
	}

	lines := readNextWorkLines(t, cwd)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after second call (dedup), got %d", len(lines))
	}
}

func TestRouteFindings_DedupsAgainstExistingEntry(t *testing.T) {
	cwd := t.TempDir()
	rpiDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	preExisting := `{"source_epic":"legacy","timestamp":"2026-03-22T00:00:00Z","items":[{"id":"f-2026-03-22-001","title":"pre-seeded","type":"finding","severity":"medium"}],"consumed":true,"claim_status":"consumed","claimed_by":null,"claimed_at":null}` + "\n"
	if err := os.WriteFile(filepath.Join(rpiDir, "next-work.jsonl"), []byte(preExisting), 0o644); err != nil {
		t.Fatalf("write seed next-work: %v", err)
	}

	writeFinding(t, cwd, "f-2026-03-22-001.md", "Same id", "Body.")

	routed, _, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("RouteFindings: %v", err)
	}
	if routed != 0 {
		t.Fatalf("expected 0 routed (deduped), got %d", routed)
	}

	lines := readNextWorkLines(t, cwd)
	if len(lines) != 1 {
		t.Fatalf("expected only the pre-seeded line, got %d", len(lines))
	}
}

func TestRouteFindings_IgnoresRegistryJSON(t *testing.T) {
	cwd := t.TempDir()
	findingsDir := filepath.Join(cwd, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(findingsDir, "registry.abc123.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	routed, _, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("RouteFindings: %v", err)
	}
	if routed != 0 {
		t.Fatalf("expected routed=0, got %d", routed)
	}
	// next-work.jsonl should not exist since nothing was routed.
	if _, statErr := os.Stat(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")); !os.IsNotExist(statErr) {
		t.Fatalf("expected next-work.jsonl absent, stat=%v", statErr)
	}
}

func TestRouteFindings_IgnoresReadmeMD(t *testing.T) {
	cwd := t.TempDir()
	findingsDir := filepath.Join(cwd, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(findingsDir, "README.md"), []byte("# Findings"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	routed, _, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("RouteFindings: %v", err)
	}
	if routed != 0 {
		t.Fatalf("expected routed=0, got %d", routed)
	}
}

func TestRouteFindings_HandlesMalformedFinding_Degraded(t *testing.T) {
	cwd := t.TempDir()
	dir := filepath.Join(cwd, ".agents", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No body, no summary, no heading — just frontmatter.
	empty := `---
id: "f-2026-04-04-001"
title: ""
severity: ""
---
`
	if err := os.WriteFile(filepath.Join(dir, "f-2026-04-04-001.md"), []byte(empty), 0o644); err != nil {
		t.Fatalf("write malformed: %v", err)
	}

	routed, degraded, err := RouteFindings(cwd)
	if err != nil {
		t.Fatalf("RouteFindings: %v", err)
	}
	if routed != 1 {
		t.Fatalf("expected malformed finding to still route (degraded=1), got routed=%d", routed)
	}
	foundMarker := false
	for _, d := range degraded {
		if strings.Contains(d, "empty body") {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Fatalf("expected 'empty body' degraded marker, got %v", degraded)
	}
}

func TestFindingID_ParsesWellFormed(t *testing.T) {
	got := findingID("f-2026-04-04-002.md")
	if got != "f-2026-04-04-002" {
		t.Fatalf("expected f-2026-04-04-002, got %q", got)
	}
}

func TestFindingID_RejectsMalformed(t *testing.T) {
	cases := []string{
		"skills-audit.md",
		"f-2026-04-4-2.md",
		"f-2026-04-04-2.md",
		"registry.abc.json",
		"README.md",
		"f-2026-04-04-002.txt",
	}
	for _, name := range cases {
		if got := findingID(name); got != "" {
			t.Fatalf("expected empty id for %q, got %q", name, got)
		}
	}
}

func TestLoadNextWorkIDs_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.jsonl")
	ids, err := loadNextWorkIDs(path)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if ids == nil {
		t.Fatalf("expected non-nil empty set")
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty set, got %d entries", len(ids))
	}
}

func TestLoadNextWorkIDs_MultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	content := `{"source_epic":"a","items":[{"id":"f-2026-03-22-001"},{"id":"f-2026-03-22-002"}]}
{"source_epic":"b","items":[{"id":"f-2026-03-22-002"},{"id":"f-2026-03-22-003"}]}
{"source_epic":"c","items":[{"title":"no-id-here"}]}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ids, err := loadNextWorkIDs(path)
	if err != nil {
		t.Fatalf("loadNextWorkIDs: %v", err)
	}
	want := []string{"f-2026-03-22-001", "f-2026-03-22-002", "f-2026-03-22-003"}
	if len(ids) != len(want) {
		t.Fatalf("expected %d ids, got %d (%v)", len(want), len(ids), ids)
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("missing id %s in %v", id, ids)
		}
	}
}

func TestFindingFilenameRe_RejectsCalendarInvalid(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"f-2026-04-09-001.md", "f-2026-04-09-001"}, // valid
		{"f-2026-02-29-001.md", ""},                 // 2026 not leap year
		{"f-9999-99-99-001.md", ""},                 // calendar-invalid
		{"f-2026-13-01-001.md", ""},                 // month > 12
		{"f-2026-04-31-001.md", ""},                 // April has 30 days
	}
	for _, tc := range cases {
		got := findingID(tc.name)
		if got != tc.want {
			t.Errorf("findingID(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

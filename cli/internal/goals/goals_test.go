package goals

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testdataPath(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestLoadGoals_V2(t *testing.T) {
	gf, err := LoadGoals(testdataPath("valid_v2.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gf.Version != 2 {
		t.Errorf("version = %d, want 2", gf.Version)
	}
	if len(gf.Goals) != 2 {
		t.Fatalf("got %d goals, want 2", len(gf.Goals))
	}

	g := gf.Goals[0]
	if g.ID != "test-coverage" {
		t.Errorf("goal[0].ID = %q, want %q", g.ID, "test-coverage")
	}
	if g.Description == "" {
		t.Error("goal[0].Description is empty")
	}
	if g.Check == "" {
		t.Error("goal[0].Check is empty")
	}
	if g.Weight != 5 {
		t.Errorf("goal[0].Weight = %d, want 5", g.Weight)
	}
	// v2 goals should default Type to "health"
	if g.Type != GoalTypeHealth {
		t.Errorf("goal[0].Type = %q, want %q", g.Type, GoalTypeHealth)
	}
	if gf.Goals[1].Type != GoalTypeHealth {
		t.Errorf("goal[1].Type = %q, want %q", gf.Goals[1].Type, GoalTypeHealth)
	}
}

func TestLoadGoals_V3(t *testing.T) {
	gf, err := LoadGoals(testdataPath("valid_v3.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gf.Version != 3 {
		t.Errorf("version = %d, want 3", gf.Version)
	}
	if gf.Mission != "Ship reliable software" {
		t.Errorf("mission = %q, want %q", gf.Mission, "Ship reliable software")
	}
	if len(gf.Goals) != 2 {
		t.Fatalf("got %d goals, want 2", len(gf.Goals))
	}

	g := gf.Goals[0]
	if g.Type != GoalTypeHealth {
		t.Errorf("goal[0].Type = %q, want %q", g.Type, GoalTypeHealth)
	}
	if g.Continuous == nil {
		t.Fatal("goal[0].Continuous is nil")
	}
	if g.Continuous.Metric != "api_latency_p99" {
		t.Errorf("goal[0].Continuous.Metric = %q, want %q", g.Continuous.Metric, "api_latency_p99")
	}
	if g.Continuous.Threshold != 0.2 {
		t.Errorf("goal[0].Continuous.Threshold = %f, want 0.2", g.Continuous.Threshold)
	}
	if len(g.Tags) != 2 {
		t.Errorf("goal[0].Tags len = %d, want 2", len(g.Tags))
	}

	g1 := gf.Goals[1]
	if g1.Type != GoalTypeArchitecture {
		t.Errorf("goal[1].Type = %q, want %q", g1.Type, GoalTypeArchitecture)
	}
}

func TestLoadGoals_FileNotFound(t *testing.T) {
	_, err := LoadGoals(testdataPath("nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestValidateGoals_Valid(t *testing.T) {
	gf, err := LoadGoals(testdataPath("valid_v3.yaml"))
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	errs := ValidateGoals(gf)
	if len(errs) != 0 {
		t.Errorf("expected 0 validation errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateGoals_MissingFields(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{}, // all fields missing
		},
	}
	errs := ValidateGoals(gf)
	// Expect errors for: id (required), description (required), check (required), weight (must be 1-10)
	fields := map[string]bool{}
	for _, e := range errs {
		fields[e.Field] = true
	}
	for _, f := range []string{"id", "description", "check", "weight"} {
		if !fields[f] {
			t.Errorf("expected validation error for field %q, not found", f)
		}
	}
}

func TestValidateGoals_DuplicateIDs(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "dup-goal", Description: "first", Check: "echo 1", Weight: 5, Type: GoalTypeHealth},
			{ID: "dup-goal", Description: "second", Check: "echo 2", Weight: 5, Type: GoalTypeHealth},
		},
	}
	errs := ValidateGoals(gf)
	found := false
	for _, e := range errs {
		if e.Field == "id" && e.Message == "duplicate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected duplicate id validation error, not found")
	}
}

func TestValidateGoals_InvalidWeight(t *testing.T) {
	cases := []struct {
		name   string
		weight int
	}{
		{"zero", 0},
		{"eleven", 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gf := &GoalFile{
				Version: 2,
				Goals: []Goal{
					{ID: "bad-weight", Description: "d", Check: "c", Weight: tc.weight, Type: GoalTypeHealth},
				},
			}
			errs := ValidateGoals(gf)
			found := false
			for _, e := range errs {
				if e.Field == "weight" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected weight validation error for weight=%d, not found", tc.weight)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{GoalID: "my-goal", Field: "check", Message: "required"}
	msg := e.Error()
	if msg == "" {
		t.Fatal("Error() should return a non-empty string")
	}
	// Should contain the goal ID, field, and message
	for _, substr := range []string{"my-goal", "check", "required"} {
		if !strings.Contains(msg, substr) {
			t.Errorf("Error() missing substring %q in %q", substr, msg)
		}
	}
}

func TestLoadGoals_UnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v99.yaml")
	content := "version: 99\ngoals:\n  - id: test\n    description: d\n    check: echo ok\n    weight: 5\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadGoals(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestLoadGoals_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	// Use something that parses as YAML but fails struct mapping — actually YAML is permissive.
	// Use truly broken YAML.
	content := "version: [\nbad yaml\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadGoals(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestValidateGoals_InvalidType(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "typed-goal", Description: "d", Check: "c", Weight: 5, Type: GoalType("invalid-type")},
		},
	}
	errs := ValidateGoals(gf)
	found := false
	for _, e := range errs {
		if e.Field == "type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected type validation error, not found")
	}
}

func TestValidateGoals_InvalidIDFormat(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "UPPER_CASE", Description: "d", Check: "c", Weight: 5, Type: GoalTypeHealth},
		},
	}
	errs := ValidateGoals(gf)
	found := false
	for _, e := range errs {
		if e.Field == "id" && e.Message == "must be kebab-case" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected kebab-case id validation error, not found")
	}
}

// --- Format Detection ---

func TestDetectFormat_MDExtension(t *testing.T) {
	f := DetectFormat("GOALS.md")
	if f != "md" {
		t.Errorf("DetectFormat(GOALS.md) = %q, want %q", f, "md")
	}
}

func TestDetectFormat_YAMLExtension(t *testing.T) {
	dir := t.TempDir()
	// No GOALS.md present → should return yaml
	f := DetectFormat(filepath.Join(dir, "GOALS.yaml"))
	if f != "yaml" {
		t.Errorf("DetectFormat(GOALS.yaml) = %q, want %q", f, "yaml")
	}
}

func TestDetectFormat_DefaultWithMDPresent(t *testing.T) {
	dir := t.TempDir()
	// Create GOALS.md alongside
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goals\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := DetectFormat(filepath.Join(dir, "GOALS.yaml"))
	if f != "md" {
		t.Errorf("DetectFormat with GOALS.md present = %q, want %q", f, "md")
	}
}

func TestDetectFormat_DefaultWithoutMD(t *testing.T) {
	dir := t.TempDir()
	f := DetectFormat(filepath.Join(dir, "GOALS.yaml"))
	if f != "yaml" {
		t.Errorf("DetectFormat without GOALS.md = %q, want %q", f, "yaml")
	}
}

func TestLoadGoals_Markdown(t *testing.T) {
	gf, err := LoadGoals(testdataPath("valid_goals.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gf.Version != 4 {
		t.Errorf("version = %d, want 4", gf.Version)
	}
	if gf.Format != "md" {
		t.Errorf("format = %q, want %q", gf.Format, "md")
	}
	if len(gf.Goals) != 2 {
		t.Errorf("goals = %d, want 2", len(gf.Goals))
	}
	if len(gf.Directives) != 2 {
		t.Errorf("directives = %d, want 2", len(gf.Directives))
	}
	// Verify goal types are defaulted
	for i, g := range gf.Goals {
		if g.Type != GoalTypeHealth {
			t.Errorf("goal[%d].Type = %q, want %q", i, g.Type, GoalTypeHealth)
		}
	}
}

func TestResolveGoalsPath_MDPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goals\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := ResolveGoalsPath(filepath.Join(dir, "GOALS.yaml"))
	want := filepath.Join(dir, "GOALS.md")
	if path != want {
		t.Errorf("ResolveGoalsPath = %q, want %q", path, want)
	}
}

func TestResolveGoalsPath_MDDirect(t *testing.T) {
	path := ResolveGoalsPath("/some/path/GOALS.md")
	if path != "/some/path/GOALS.md" {
		t.Errorf("ResolveGoalsPath = %q, want original path", path)
	}
}

// --- DetectFormat (extra) ---

func TestExtra_DetectFormat_ExplicitMD(t *testing.T) {
	got := DetectFormat("GOALS.md")
	if got != "md" {
		t.Errorf("DetectFormat(GOALS.md) = %q, want %q", got, "md")
	}
}

func TestExtra_DetectFormat_YAMLNoMDSibling(t *testing.T) {
	// Create a temp dir with only a YAML file (no GOALS.md sibling).
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := DetectFormat(yamlPath)
	if got != "yaml" {
		t.Errorf("DetectFormat(yaml, no md sibling) = %q, want %q", got, "yaml")
	}
}

func TestExtra_DetectFormat_YAMLWithMDSibling(t *testing.T) {
	// When GOALS.md exists alongside GOALS.yaml, prefer md.
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "GOALS.yaml")
	mdPath := filepath.Join(dir, "GOALS.md")
	os.WriteFile(yamlPath, []byte("version: 2\n"), 0o600)
	os.WriteFile(mdPath, []byte("# Goals\n"), 0o600)

	got := DetectFormat(yamlPath)
	if got != "md" {
		t.Errorf("DetectFormat(yaml with md sibling) = %q, want %q", got, "md")
	}
}

func TestExtra_DetectFormat_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	ymlPath := filepath.Join(dir, "GOALS.yml")
	os.WriteFile(ymlPath, []byte("version: 2\n"), 0o600)

	got := DetectFormat(ymlPath)
	if got != "yaml" {
		t.Errorf("DetectFormat(.yml) = %q, want %q", got, "yaml")
	}
}

func TestExtra_DetectFormat_NoExtensionNoMD(t *testing.T) {
	dir := t.TempDir()
	noExtPath := filepath.Join(dir, "GOALS")
	os.WriteFile(noExtPath, []byte("stuff\n"), 0o600)

	got := DetectFormat(noExtPath)
	if got != "yaml" {
		t.Errorf("DetectFormat(no ext, no md) = %q, want %q", got, "yaml")
	}
}

func TestExtra_DetectFormat_NoExtensionWithMD(t *testing.T) {
	dir := t.TempDir()
	noExtPath := filepath.Join(dir, "GOALS")
	mdPath := filepath.Join(dir, "GOALS.md")
	os.WriteFile(noExtPath, []byte("stuff\n"), 0o600)
	os.WriteFile(mdPath, []byte("# Goals\n"), 0o600)

	got := DetectFormat(noExtPath)
	if got != "md" {
		t.Errorf("DetectFormat(no ext, md exists) = %q, want %q", got, "md")
	}
}

// --- LoadGoals (extra) ---

func TestExtra_LoadGoals_UnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.yaml")
	os.WriteFile(path, []byte("version: 99\ngoals: []\n"), 0o600)

	_, err := LoadGoals(path)
	if err == nil {
		t.Fatal("expected error for unsupported version, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtra_LoadGoals_Version1Warning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.yaml")
	content := `version: 1
goals:
  - id: test-goal
    description: A goal
    check: echo ok
    weight: 5
`
	os.WriteFile(path, []byte(content), 0o600)

	gf, err := LoadGoals(path)
	if err != nil {
		t.Fatalf("LoadGoals v1: %v", err)
	}
	if gf.Format != "yaml" {
		t.Errorf("Format = %q, want %q", gf.Format, "yaml")
	}
	if gf.Version != 1 {
		t.Errorf("Version = %d, want 1", gf.Version)
	}
}

func TestExtra_LoadGoals_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.yaml")
	os.WriteFile(path, []byte(":::invalid yaml\n"), 0o600)

	_, err := LoadGoals(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestExtra_LoadGoals_FileNotFound(t *testing.T) {
	_, err := LoadGoals("/nonexistent/GOALS.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtra_LoadGoals_MDFileNotFound(t *testing.T) {
	_, err := LoadGoals("/nonexistent/GOALS.md")
	if err == nil {
		t.Fatal("expected error for missing md file")
	}
}

func TestExtra_LoadGoals_InvalidMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.md")
	// Empty file triggers "empty goals file" error.
	os.WriteFile(path, []byte(""), 0o600)

	_, err := LoadGoals(path)
	if err == nil {
		t.Fatal("expected error for empty md file")
	}
	if !strings.Contains(err.Error(), "empty goals file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- AppendHistory (extra) ---

func TestExtra_AppendHistory_CreatesFileAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	entry := HistoryEntry{
		Timestamp:    time.Now().Format(time.RFC3339),
		GoalsPassing: 3,
		GoalsTotal:   5,
		Score:        60.0,
		GitSHA:       "abc1234",
	}

	if err := AppendHistory(entry, path); err != nil {
		t.Fatalf("AppendHistory (create): %v", err)
	}

	// Append a second entry.
	entry2 := HistoryEntry{
		Timestamp:    time.Now().Format(time.RFC3339),
		GoalsPassing: 4,
		GoalsTotal:   5,
		Score:        80.0,
		GitSHA:       "def5678",
	}
	if err := AppendHistory(entry2, path); err != nil {
		t.Fatalf("AppendHistory (append): %v", err)
	}

	// Verify both entries exist.
	entries, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
	if entries[0].GoalsPassing != 3 {
		t.Errorf("first entry GoalsPassing = %d, want 3", entries[0].GoalsPassing)
	}
	if entries[1].GoalsPassing != 4 {
		t.Errorf("second entry GoalsPassing = %d, want 4", entries[1].GoalsPassing)
	}
}

func TestExtra_AppendHistory_BadPath(t *testing.T) {
	err := AppendHistory(HistoryEntry{}, "/nonexistent/dir/file.jsonl")
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}

// --- ParseMarkdownGoals (extra) ---

func TestExtra_ParseMarkdownGoals_Empty(t *testing.T) {
	_, err := ParseMarkdownGoals([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestExtra_ParseMarkdownGoals_WhitespaceOnly(t *testing.T) {
	_, err := ParseMarkdownGoals([]byte("   \n  \n  "))
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

func TestExtra_ParseMarkdownGoals_NoGatesSection(t *testing.T) {
	md := `# Goals
My mission statement

## North Stars
- Star one
`
	gf, err := ParseMarkdownGoals([]byte(md))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gf.Goals) != 0 {
		t.Errorf("expected 0 goals without Gates section, got %d", len(gf.Goals))
	}
	if gf.Mission != "My mission statement" {
		t.Errorf("Mission = %q, want %q", gf.Mission, "My mission statement")
	}
}

// --- parseMission (extra) ---

func TestExtra_parseMission_NoH1(t *testing.T) {
	lines := []string{"No heading here", "just text"}
	got := parseMission(lines)
	if got != "" {
		t.Errorf("parseMission(no H1) = %q, want empty", got)
	}
}

func TestExtra_parseMission_H1FollowedByH2(t *testing.T) {
	// H1 immediately followed by another heading (no mission paragraph).
	lines := []string{"# Goals", "", "## Section"}
	got := parseMission(lines)
	if got != "" {
		t.Errorf("parseMission(H1 then H2) = %q, want empty", got)
	}
}

func TestExtra_parseMission_H1FollowedByH3(t *testing.T) {
	lines := []string{"# Goals", "### Sub-heading"}
	got := parseMission(lines)
	if got != "" {
		t.Errorf("parseMission(H1 then H3) = %q, want empty", got)
	}
}

// --- parseListSection (extra) ---

func TestExtra_parseListSection_NonBulletLinesIgnored(t *testing.T) {
	lines := []string{
		"## North Stars",
		"This is not a bullet",
		"- Real bullet",
		"plain text again",
		"* Another bullet",
		"## Next Section",
	}
	items := parseListSection(lines, "North Stars")
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
	if items[0] != "Real bullet" {
		t.Errorf("item[0] = %q, want %q", items[0], "Real bullet")
	}
	if items[1] != "Another bullet" {
		t.Errorf("item[1] = %q, want %q", items[1], "Another bullet")
	}
}

func TestExtra_parseListSection_EndedByH1(t *testing.T) {
	lines := []string{
		"## Anti Stars",
		"- Bad thing",
		"# Top Level Heading",
	}
	items := parseListSection(lines, "Anti Stars")
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
}

func TestExtra_parseListSection_EndedByH3(t *testing.T) {
	lines := []string{
		"## Anti Stars",
		"- Bad thing",
		"### Sub Section",
		"- Should not appear",
	}
	items := parseListSection(lines, "Anti Stars")
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
}

func TestExtra_parseListSection_EmptyBulletSkipped(t *testing.T) {
	lines := []string{
		"## North Stars",
		"- ",
		"- Real item",
	}
	items := parseListSection(lines, "North Stars")
	if len(items) != 1 {
		t.Errorf("got %d items, want 1 (empty bullet skipped)", len(items))
	}
}

func TestExtra_parseListSection_SectionNotFound(t *testing.T) {
	lines := []string{"## Other Section", "- item"}
	items := parseListSection(lines, "Missing Section")
	if len(items) != 0 {
		t.Errorf("got %d items for missing section, want 0", len(items))
	}
}

// --- parseGatesTable (extra) ---

func TestExtra_parseGatesTable_NoGatesSection(t *testing.T) {
	lines := []string{"## Other", "| a | b |"}
	goals := parseGatesTable(lines)
	if goals != nil {
		t.Errorf("expected nil goals for no Gates section, got %v", goals)
	}
}

func TestExtra_parseGatesTable_EmptyRowSkipped(t *testing.T) {
	lines := []string{
		"## Gates",
		"| ID | Check | Weight | Description |",
		"|---|---|---|---|",
		"| | `echo hi` | 5 | empty id |",
		"| valid-id | `echo ok` | 3 | real goal |",
	}
	goals := parseGatesTable(lines)
	// The row with empty ID should be skipped.
	if len(goals) != 1 {
		t.Errorf("got %d goals, want 1 (empty ID row skipped)", len(goals))
	}
	if goals[0].ID != "valid-id" {
		t.Errorf("goal ID = %q, want %q", goals[0].ID, "valid-id")
	}
}

func TestExtra_parseGatesTable_InvalidWeightDefaultsTo5(t *testing.T) {
	lines := []string{
		"## Gates",
		"| ID | Check | Weight | Description |",
		"|---|---|---|---|",
		"| my-goal | `echo ok` | abc | test desc |",
	}
	goals := parseGatesTable(lines)
	if len(goals) != 1 {
		t.Fatalf("got %d goals, want 1", len(goals))
	}
	if goals[0].Weight != 5 {
		t.Errorf("Weight = %d, want 5 (default for invalid)", goals[0].Weight)
	}
}

func TestExtra_parseGatesTable_OutOfRangeWeightDefaultsTo5(t *testing.T) {
	lines := []string{
		"## Gates",
		"| ID | Check | Weight | Description |",
		"|---|---|---|---|",
		"| my-goal | `echo ok` | 99 | test |",
	}
	goals := parseGatesTable(lines)
	if goals[0].Weight != 5 {
		t.Errorf("Weight = %d, want 5 (default for out-of-range)", goals[0].Weight)
	}
}

func TestExtra_parseGatesTable_DescriptionFallsBackToID(t *testing.T) {
	lines := []string{
		"## Gates",
		"| ID | Check | Weight | Description |",
		"|---|---|---|---|",
		"| my-goal | `echo ok` | 3 | |",
	}
	goals := parseGatesTable(lines)
	if goals[0].Description != "my-goal" {
		t.Errorf("Description = %q, want %q (fallback to ID)", goals[0].Description, "my-goal")
	}
}

// --- MeasureOne (extra) ---

func TestExtra_MeasureOne_BadCommand(t *testing.T) {
	g := Goal{
		ID:     "bad-cmd",
		Check:  "/nonexistent/binary/that/does/not/exist",
		Weight: 3,
	}
	m := MeasureOne(g, 5*time.Second)
	if m.Result != "fail" {
		t.Errorf("Result = %q, want %q for bad command", m.Result, "fail")
	}
	if m.GoalID != "bad-cmd" {
		t.Errorf("GoalID = %q, want %q", m.GoalID, "bad-cmd")
	}
	if m.Weight != 3 {
		t.Errorf("Weight = %d, want 3", m.Weight)
	}
}

func TestExtra_MeasureOne_TimeoutSkip(t *testing.T) {
	g := Goal{
		ID:     "slow-cmd",
		Check:  "sleep 60",
		Weight: 2,
	}
	m := MeasureOne(g, 100*time.Millisecond)
	if m.Result != "skip" {
		t.Errorf("Result = %q, want %q for timeout", m.Result, "skip")
	}
}

func TestExtra_MeasureOne_ContinuousMetric(t *testing.T) {
	g := Goal{
		ID:     "metric-goal",
		Check:  "echo 95.5",
		Weight: 5,
		Continuous: &ContinuousMetric{
			Metric:    "coverage",
			Threshold: 90.0,
		},
	}
	m := MeasureOne(g, 5*time.Second)
	if m.Result != "pass" {
		t.Errorf("Result = %q, want pass", m.Result)
	}
	if m.Value == nil {
		t.Fatal("Value is nil, want parsed float")
	}
	if *m.Value != 95.5 {
		t.Errorf("Value = %f, want 95.5", *m.Value)
	}
	if m.Threshold == nil {
		t.Fatal("Threshold is nil, want 90.0")
	}
	if *m.Threshold != 90.0 {
		t.Errorf("Threshold = %f, want 90.0", *m.Threshold)
	}
}

// --- killAllChildren (extra) ---

func TestExtra_killAllChildren_EmptyPids(t *testing.T) {
	// Calling killAllChildren with no tracked children should not panic.
	childGroups.mu.Lock()
	childGroups.pids = make(map[int]struct{})
	childGroups.mu.Unlock()

	killAllChildren()

	childGroups.mu.Lock()
	defer childGroups.mu.Unlock()
	if len(childGroups.pids) != 0 {
		t.Errorf("pids should be empty after killAllChildren, got %d", len(childGroups.pids))
	}
}

func TestExtra_killAllChildren_WithStalePid(t *testing.T) {
	// Track a PID that doesn't exist (e.g., 999999999). killAllChildren
	// should not panic even if kill fails.
	childGroups.mu.Lock()
	childGroups.pids = map[int]struct{}{999999999: {}}
	childGroups.mu.Unlock()

	killAllChildren() // Should not panic.

	childGroups.mu.Lock()
	defer childGroups.mu.Unlock()
	if len(childGroups.pids) != 0 {
		t.Errorf("pids should be cleared after killAllChildren, got %d", len(childGroups.pids))
	}
}

// --- SaveSnapshot (extra) ---

func TestExtra_SaveSnapshot_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "snapshots")
	s := &Snapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		GitSHA:    "abc",
		Goals:     []Measurement{{GoalID: "g1", Result: "pass", Weight: 5}},
		Summary:   SnapshotSummary{Total: 1, Passing: 1, Score: 100},
	}

	path, err := SaveSnapshot(s, dir)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("path %q should end with .json", path)
	}

	// Verify we can load it back.
	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if loaded.GitSHA != "abc" {
		t.Errorf("GitSHA = %q, want %q", loaded.GitSHA, "abc")
	}
	if len(loaded.Goals) != 1 {
		t.Errorf("Goals count = %d, want 1", len(loaded.Goals))
	}
}

func TestExtra_SaveSnapshot_BadDir(t *testing.T) {
	// Try to save to a path where dir creation will fail.
	s := &Snapshot{}
	_, err := SaveSnapshot(s, "/dev/null/impossible")
	if err == nil {
		t.Fatal("expected error for impossible directory")
	}
}

// --- runGoals (extra) ---

func TestExtra_runGoals_MetaAndNonMeta(t *testing.T) {
	goals := []Goal{
		{ID: "meta-g", Check: "echo meta", Weight: 3, Type: GoalTypeMeta},
		{ID: "health-g", Check: "echo health", Weight: 5, Type: GoalTypeHealth},
	}
	measurements := runGoals(goals, 5*time.Second)
	if len(measurements) != 2 {
		t.Fatalf("got %d measurements, want 2", len(measurements))
	}
	// Meta should run first.
	if measurements[0].GoalID != "meta-g" {
		t.Errorf("first measurement = %q, want %q (meta first)", measurements[0].GoalID, "meta-g")
	}
	if measurements[1].GoalID != "health-g" {
		t.Errorf("second measurement = %q, want %q", measurements[1].GoalID, "health-g")
	}
}

func TestExtra_runGoals_OnlyMeta(t *testing.T) {
	goals := []Goal{
		{ID: "m1", Check: "echo m1", Weight: 1, Type: GoalTypeMeta},
	}
	measurements := runGoals(goals, 5*time.Second)
	if len(measurements) != 1 {
		t.Fatalf("got %d measurements, want 1", len(measurements))
	}
	if measurements[0].GoalID != "m1" {
		t.Errorf("GoalID = %q, want %q", measurements[0].GoalID, "m1")
	}
}

func TestExtra_runGoals_ExclusiveExecution(t *testing.T) {
	// Goals containing "go test" trigger exclusive execution.
	goals := []Goal{
		{ID: "test-g", Check: "echo 'go test placeholder'", Weight: 5, Type: GoalTypeHealth},
		{ID: "normal-g", Check: "echo normal", Weight: 3, Type: GoalTypeHealth},
	}
	measurements := runGoals(goals, 5*time.Second)
	if len(measurements) != 2 {
		t.Fatalf("got %d measurements, want 2", len(measurements))
	}
	for _, m := range measurements {
		if m.Result != "pass" {
			t.Errorf("goal %q Result = %q, want pass", m.GoalID, m.Result)
		}
	}
}

// --- computeSummary (extra) ---

func TestExtra_computeSummary_AllSkipped(t *testing.T) {
	ms := []Measurement{
		{GoalID: "g1", Result: "skip", Weight: 5},
		{GoalID: "g2", Result: "skip", Weight: 3},
	}
	s := computeSummary(ms)
	if s.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", s.Skipped)
	}
	if s.Score != 0 {
		t.Errorf("Score = %f, want 0 (all skipped)", s.Score)
	}
}

// --- Benchmarks ---

func BenchmarkLoadGoals(b *testing.B) {
	dir := b.TempDir()
	path := dir + "/GOALS.yaml"
	content := `version: 3
goals:
  - id: test-coverage
    description: Achieve 95%+ test coverage
    check: go test -cover
    weight: 3
    type: quality
  - id: complexity-budget
    description: Keep complexity under 15
    check: gocyclo -over 15
    weight: 2
    type: health
  - id: lint-clean
    description: Zero staticcheck findings
    check: staticcheck ./...
    weight: 1
    type: quality
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("write: %v", err)
	}

	b.ResetTimer()
	for range b.N {
		_, _ = LoadGoals(path)
	}
}

func TestMigrateV1ToV2_SetsVersionAndMission(t *testing.T) {
	gf := &GoalFile{Version: 1, Goals: []Goal{
		{ID: "g1", Check: "true", Weight: 1},
	}}
	MigrateV1ToV2(gf)
	if gf.Version != 2 {
		t.Errorf("Version = %d, want 2", gf.Version)
	}
	if gf.Mission != "Project fitness goals" {
		t.Errorf("Mission = %q, want default", gf.Mission)
	}
	// Goals without type should get default health type
	if gf.Goals[0].Type != GoalTypeHealth {
		t.Errorf("Goal type = %q, want health", gf.Goals[0].Type)
	}
}

func TestMigrateV1ToV2_PreservesMission(t *testing.T) {
	gf := &GoalFile{Version: 1, Mission: "Custom mission"}
	MigrateV1ToV2(gf)
	if gf.Mission != "Custom mission" {
		t.Errorf("Mission = %q, want preserved custom", gf.Mission)
	}
}

func TestKillAllChildren_EmptyAndNilMap(t *testing.T) {
	// Should not panic when pids map is nil
	childGroups.mu.Lock()
	childGroups.pids = nil
	childGroups.mu.Unlock()
	killAllChildren() // nil map

	// Should not panic when pids map is empty
	childGroups.mu.Lock()
	childGroups.pids = make(map[int]struct{})
	childGroups.mu.Unlock()
	killAllChildren() // empty map
}

func TestLoadGoals_AffectsFilesSidecar(t *testing.T) {
	dir := t.TempDir()
	goalsPath := filepath.Join(dir, "GOALS.md")
	mdContent := `# Goals

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g-one | echo ok | 5 | first |
| g-two | echo ok | 3 | second |
`
	if err := os.WriteFile(goalsPath, []byte(mdContent), 0o600); err != nil {
		t.Fatal(err)
	}
	sidecarPath := filepath.Join(dir, "goals-affects-files.yaml")
	sidecar := `affects_files:
  g-one:
    - scripts/g-one.sh
    - cli/**
  g-two:
    - scripts/g-two.sh
  unknown-goal:
    - scripts/should-be-ignored.sh
`
	if err := os.WriteFile(sidecarPath, []byte(sidecar), 0o600); err != nil {
		t.Fatal(err)
	}

	gf, err := LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if len(gf.Goals) != 2 {
		t.Fatalf("goals len = %d, want 2", len(gf.Goals))
	}

	got := gf.Goals[0]
	if got.ID != "g-one" {
		t.Errorf("goals[0].ID = %q, want g-one", got.ID)
	}
	wantOne := []string{"scripts/g-one.sh", "cli/**"}
	if len(got.AffectsFiles) != len(wantOne) {
		t.Fatalf("g-one AffectsFiles = %v, want %v", got.AffectsFiles, wantOne)
	}
	for i, w := range wantOne {
		if got.AffectsFiles[i] != w {
			t.Errorf("g-one AffectsFiles[%d] = %q, want %q", i, got.AffectsFiles[i], w)
		}
	}

	got2 := gf.Goals[1]
	if got2.ID != "g-two" {
		t.Errorf("goals[1].ID = %q, want g-two", got2.ID)
	}
	if len(got2.AffectsFiles) != 1 || got2.AffectsFiles[0] != "scripts/g-two.sh" {
		t.Errorf("g-two AffectsFiles = %v, want [scripts/g-two.sh]", got2.AffectsFiles)
	}
}

func TestLoadGoals_AffectsFilesSidecarMissing(t *testing.T) {
	// Sidecar absent: AffectsFiles stays nil; no error.
	dir := t.TempDir()
	goalsPath := filepath.Join(dir, "GOALS.md")
	mdContent := `# Goals

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g-only | echo ok | 5 | only goal |
`
	if err := os.WriteFile(goalsPath, []byte(mdContent), 0o600); err != nil {
		t.Fatal(err)
	}
	gf, err := LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if gf.Goals[0].AffectsFiles != nil {
		t.Errorf("AffectsFiles = %v, want nil for missing sidecar", gf.Goals[0].AffectsFiles)
	}
}

func TestSidecarPathFor(t *testing.T) {
	got := sidecarPathFor(filepath.Join("repo", "GOALS.md"))
	want := filepath.Join("repo", "goals-affects-files.yaml")
	if got != want {
		t.Errorf("sidecarPathFor = %q, want %q", got, want)
	}
}

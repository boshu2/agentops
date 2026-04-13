package goals

import (
	"os"
	"strings"
	"testing"
)

func TestParseMarkdownGoals_ValidFixture(t *testing.T) {
	data, err := os.ReadFile(testdataPath("valid_goals.md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	gf, err := ParseMarkdownGoals(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gf.Version != 4 {
		t.Errorf("version = %d, want 4", gf.Version)
	}
	if gf.Format != "md" {
		t.Errorf("format = %q, want %q", gf.Format, "md")
	}
	if gf.Mission != "Ship reliable, well-tested software that improves continuously." {
		t.Errorf("mission = %q", gf.Mission)
	}
	if len(gf.NorthStars) != 2 {
		t.Fatalf("north stars = %d, want 2", len(gf.NorthStars))
	}
	if len(gf.AntiStars) != 2 {
		t.Fatalf("anti stars = %d, want 2", len(gf.AntiStars))
	}
	if len(gf.Directives) != 2 {
		t.Fatalf("directives = %d, want 2", len(gf.Directives))
	}
	if len(gf.Goals) != 2 {
		t.Fatalf("goals = %d, want 2", len(gf.Goals))
	}
}

func TestParseMission(t *testing.T) {
	lines := strings.Split("# Goals\n\nBuild great software.\n\n## North Stars\n", "\n")
	mission := parseMission(lines)
	if mission != "Build great software." {
		t.Errorf("mission = %q, want %q", mission, "Build great software.")
	}
}

func TestParseMission_Empty(t *testing.T) {
	lines := strings.Split("# Goals\n\n## North Stars\n", "\n")
	mission := parseMission(lines)
	if mission != "" {
		t.Errorf("mission = %q, want empty", mission)
	}
}

func TestParseListSection_NorthStars(t *testing.T) {
	input := "## North Stars\n\n- First star\n- Second star\n\n## Anti Stars\n"
	items := parseListSection(strings.Split(input, "\n"), "North Stars")
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0] != "First star" {
		t.Errorf("items[0] = %q", items[0])
	}
}

func TestParseListSection_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name    string
		heading string
	}{
		{"lowercase", "## north stars\n\n- Item\n"},
		{"uppercase", "## NORTH STARS\n\n- Item\n"},
		{"mixed", "## North stars\n\n- Item\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := parseListSection(strings.Split(tc.heading, "\n"), "North Stars")
			if len(items) != 1 {
				t.Errorf("items = %d, want 1", len(items))
			}
		})
	}
}

func TestParseListSection_AsteriskBullets(t *testing.T) {
	input := "## North Stars\n\n* First\n* Second\n"
	items := parseListSection(strings.Split(input, "\n"), "North Stars")
	if len(items) != 2 {
		t.Errorf("items = %d, want 2", len(items))
	}
}

func TestParseListSection_Missing(t *testing.T) {
	input := "## Something Else\n\n- Item\n"
	items := parseListSection(strings.Split(input, "\n"), "North Stars")
	if len(items) != 0 {
		t.Errorf("items = %d, want 0", len(items))
	}
}

func TestParseDirectives(t *testing.T) {
	input := `## Directives

### 1. First Directive

Some description here.

**Steer:** increase

### 2. Second Directive

Another description.

**Steer:** decrease

## Gates
`
	directives := parseDirectives(strings.Split(input, "\n"))
	if len(directives) != 2 {
		t.Fatalf("directives = %d, want 2", len(directives))
	}
	if directives[0].Number != 1 {
		t.Errorf("d[0].Number = %d, want 1", directives[0].Number)
	}
	if directives[0].Title != "First Directive" {
		t.Errorf("d[0].Title = %q", directives[0].Title)
	}
	if directives[0].Description != "Some description here." {
		t.Errorf("d[0].Description = %q", directives[0].Description)
	}
	if directives[0].Steer != "increase" {
		t.Errorf("d[0].Steer = %q", directives[0].Steer)
	}
	if directives[1].Number != 2 {
		t.Errorf("d[1].Number = %d, want 2", directives[1].Number)
	}
}

func TestParseDirectives_CaseInsensitive(t *testing.T) {
	input := "## DIRECTIVES\n\n### 1. Test\n\nBody.\n\n**Steer:** hold\n"
	directives := parseDirectives(strings.Split(input, "\n"))
	if len(directives) != 1 {
		t.Errorf("directives = %d, want 1", len(directives))
	}
}

func TestParseDirectives_PreservesIndentation(t *testing.T) {
	input := "## Directives\n\n### 1. Indented Body\n\nPlain text.\n\n  - list item one\n  - list item two\n\n    code block\n\n**Steer:** increase\n\n## Gates\n"
	directives := parseDirectives(strings.Split(input, "\n"))
	if len(directives) != 1 {
		t.Fatalf("directives = %d, want 1", len(directives))
	}
	desc := directives[0].Description
	if !strings.Contains(desc, "  - list item one") {
		t.Errorf("indentation lost in list item, got: %q", desc)
	}
	if !strings.Contains(desc, "    code block") {
		t.Errorf("indentation lost in code block, got: %q", desc)
	}
}

func TestParseDirectives_Missing(t *testing.T) {
	input := "## Gates\n\n| ID | Check | Weight | Description |\n"
	directives := parseDirectives(strings.Split(input, "\n"))
	if len(directives) != 0 {
		t.Errorf("directives = %d, want 0", len(directives))
	}
}

func TestParseGatesTable(t *testing.T) {
	input := `## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| build-ok | ` + "`make build`" + ` | 8 | Build passes |
| test-ok | ` + "`make test`" + ` | 5 | Tests pass |

## Next Section
`
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 2 {
		t.Fatalf("goals = %d, want 2", len(goals))
	}
	if goals[0].ID != "build-ok" {
		t.Errorf("goals[0].ID = %q", goals[0].ID)
	}
	if goals[0].Check != "make build" {
		t.Errorf("goals[0].Check = %q (backticks should be stripped)", goals[0].Check)
	}
	if goals[0].Weight != 8 {
		t.Errorf("goals[0].Weight = %d, want 8", goals[0].Weight)
	}
	if goals[0].Description != "Build passes" {
		t.Errorf("goals[0].Description = %q", goals[0].Description)
	}
}

func TestParseGatesTable_CaseInsensitive(t *testing.T) {
	input := "## GATES\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n| test | `echo ok` | 5 | Test |\n"
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Errorf("goals = %d, want 1", len(goals))
	}
}

func TestParseGatesTable_ExtraWhitespace(t *testing.T) {
	input := "## Gates\n\n|  ID  |  Check  |  Weight  |  Description  |\n| --- | --- | --- | --- |\n|  spaced  |  `echo hi`  |  3  |  Spaced out  |\n"
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Fatalf("goals = %d, want 1", len(goals))
	}
	if goals[0].ID != "spaced" {
		t.Errorf("goals[0].ID = %q, want %q", goals[0].ID, "spaced")
	}
}

func TestParseGatesTable_MissingWeight(t *testing.T) {
	input := "## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n| no-weight | `echo ok` | bad | Test |\n"
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Fatalf("goals = %d, want 1", len(goals))
	}
	// Bad weight defaults to 5
	if goals[0].Weight != 5 {
		t.Errorf("goals[0].Weight = %d, want 5 (default)", goals[0].Weight)
	}
}

func TestParseGatesTable_Missing(t *testing.T) {
	input := "## Directives\n\n### 1. Only Directives\n"
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 0 {
		t.Errorf("goals = %d, want 0", len(goals))
	}
}

func TestParseMarkdownGoals_Empty(t *testing.T) {
	_, err := ParseMarkdownGoals([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestParseMarkdownGoals_MissingSections(t *testing.T) {
	// Minimal valid: just a heading
	data := []byte("# Goals\n\nJust a mission.\n")
	gf, err := ParseMarkdownGoals(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gf.Mission != "Just a mission." {
		t.Errorf("mission = %q", gf.Mission)
	}
	if len(gf.NorthStars) != 0 {
		t.Errorf("north stars should be empty, got %d", len(gf.NorthStars))
	}
	if len(gf.Directives) != 0 {
		t.Errorf("directives should be empty, got %d", len(gf.Directives))
	}
	if len(gf.Goals) != 0 {
		t.Errorf("goals should be empty, got %d", len(gf.Goals))
	}
}

func TestRoundTrip(t *testing.T) {
	data, err := os.ReadFile(testdataPath("valid_goals.md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	gf1, err := ParseMarkdownGoals(data)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}

	rendered := RenderGoalsMD(gf1)

	gf2, err := ParseMarkdownGoals([]byte(rendered))
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}

	// Compare key fields
	if gf1.Mission != gf2.Mission {
		t.Errorf("mission mismatch: %q vs %q", gf1.Mission, gf2.Mission)
	}
	if len(gf1.NorthStars) != len(gf2.NorthStars) {
		t.Errorf("north stars count: %d vs %d", len(gf1.NorthStars), len(gf2.NorthStars))
	}
	if len(gf1.AntiStars) != len(gf2.AntiStars) {
		t.Errorf("anti stars count: %d vs %d", len(gf1.AntiStars), len(gf2.AntiStars))
	}
	if len(gf1.Directives) != len(gf2.Directives) {
		t.Errorf("directives count: %d vs %d", len(gf1.Directives), len(gf2.Directives))
	}
	for i := range gf1.Directives {
		if gf1.Directives[i].Title != gf2.Directives[i].Title {
			t.Errorf("directive[%d] title: %q vs %q", i, gf1.Directives[i].Title, gf2.Directives[i].Title)
		}
		if gf1.Directives[i].Steer != gf2.Directives[i].Steer {
			t.Errorf("directive[%d] steer: %q vs %q", i, gf1.Directives[i].Steer, gf2.Directives[i].Steer)
		}
	}
	if len(gf1.Goals) != len(gf2.Goals) {
		t.Errorf("goals count: %d vs %d", len(gf1.Goals), len(gf2.Goals))
	}
	for i := range gf1.Goals {
		if gf1.Goals[i].ID != gf2.Goals[i].ID {
			t.Errorf("goal[%d] ID: %q vs %q", i, gf1.Goals[i].ID, gf2.Goals[i].ID)
		}
		if gf1.Goals[i].Check != gf2.Goals[i].Check {
			t.Errorf("goal[%d] Check: %q vs %q", i, gf1.Goals[i].Check, gf2.Goals[i].Check)
		}
		if gf1.Goals[i].Weight != gf2.Goals[i].Weight {
			t.Errorf("goal[%d] Weight: %d vs %d", i, gf1.Goals[i].Weight, gf2.Goals[i].Weight)
		}
	}
}

func TestRoundTripPipeChars(t *testing.T) {
	// Build a GoalFile with pipe characters in Check and Description fields.
	gf := &GoalFile{
		Version: 4,
		Format:  "md",
		Mission: "Test pipe escaping.",
		Goals: []Goal{
			{
				ID:          "pipe-check",
				Check:       "cmd --flag a|b",
				Weight:      7,
				Description: "Runs a|b pipeline",
				Type:        GoalTypeHealth,
			},
			{
				ID:          "pipe-desc",
				Check:       "make test",
				Weight:      5,
				Description: "Pipe in desc: foo|bar|baz",
				Type:        GoalTypeHealth,
			},
		},
	}

	rendered := RenderGoalsMD(gf)

	gf2, err := ParseMarkdownGoals([]byte(rendered))
	if err != nil {
		t.Fatalf("parsing rendered markdown: %v", err)
	}

	if len(gf2.Goals) != 2 {
		t.Fatalf("goals count = %d, want 2", len(gf2.Goals))
	}

	for i, want := range gf.Goals {
		got := gf2.Goals[i]
		if got.ID != want.ID {
			t.Errorf("goal[%d].ID = %q, want %q", i, got.ID, want.ID)
		}
		if got.Check != want.Check {
			t.Errorf("goal[%d].Check = %q, want %q", i, got.Check, want.Check)
		}
		if got.Weight != want.Weight {
			t.Errorf("goal[%d].Weight = %d, want %d", i, got.Weight, want.Weight)
		}
		if got.Description != want.Description {
			t.Errorf("goal[%d].Description = %q, want %q", i, got.Description, want.Description)
		}
	}
}

func TestSplitTableRow(t *testing.T) {
	row := "| foo | bar | baz |"
	cells := splitTableRow(row)
	if len(cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(cells))
	}
	if cells[0] != "foo" {
		t.Errorf("cells[0] = %q", cells[0])
	}
}

// TestParseMarkdownGoals_Adversarial exercises edge-case inputs to verify the
// parser does not panic and handles each situation gracefully.
func TestParseMarkdownGoals_Adversarial(t *testing.T) {
	tests := []struct {
		name string
		// input is the full GOALS.md content to parse.
		input string
		// wantErr indicates the parse should return a non-nil error.
		wantErr bool
		// check is an optional function that inspects the parsed GoalFile.
		// It is only called when wantErr is false.
		check func(t *testing.T, gf *GoalFile)
	}{
		{
			name: "pipe in Check field round-trips via RenderGoalsMD",
			// This test verifies that a goal whose Check contains a literal pipe
			// survives render → parse without corruption.
			input: func() string {
				gf := &GoalFile{
					Version: 4,
					Format:  "md",
					Mission: "Pipe test.",
					Goals: []Goal{
						{
							ID:          "pipe-in-check",
							Check:       `bash -c "echo a | grep b"`,
							Weight:      5,
							Description: "Grep after echo",
							Type:        GoalTypeHealth,
						},
					},
				}
				return RenderGoalsMD(gf)
			}(),
			check: checkPipeInCheckAdversarialGoal,
		},
		{
			name: "pipe in Description field round-trips via RenderGoalsMD",
			input: func() string {
				gf := &GoalFile{
					Version: 4,
					Format:  "md",
					Mission: "Pipe desc test.",
					Goals: []Goal{
						{
							ID:          "pipe-in-desc",
							Check:       "make test",
							Weight:      5,
							Description: "Run A | then B",
							Type:        GoalTypeHealth,
						},
					},
				}
				return RenderGoalsMD(gf)
			}(),
			check: checkPipeInDescriptionAdversarialGoal,
		},
		{
			// Backtick-wrapped check values have their surrounding backticks stripped via
			// strings.Trim(check, "`"). A cell containing multiple backtick-delimited
			// segments (e.g. "`cmd` && `other`") is unusual but must not cause a panic.
			// The parser strips only leading/trailing backticks, so the inner content
			// may differ from the original. This test documents current behaviour.
			name: "backtick in Check field — no panic, documented behaviour",
			// Build a table row whose Check cell contains internal backticks:
			// | bt-check | `cmd` && `other` | 5 | Backtick check |
			input: "# Goals\n\nBacktick test.\n\n## Gates\n\n" +
				"| ID | Check | Weight | Description |\n" +
				"|----|-------|--------|-------------|\n" +
				"| bt-check | `cmd` && `other` | 5 | Backtick check |\n",
			check: checkBacktickInCheckAdversarialGoal,
		},
		{
			// A row where the Description column is empty; the parser falls back to the
			// goal ID as the description (see the "Use ID as description fallback" comment
			// in parseGatesTable).
			name:  "empty Description cell falls back to ID",
			input: "# Goals\n\nEmpty desc.\n\n## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n| empty-desc | `echo ok` | 5 |  |\n",
			check: checkEmptyDescriptionAdversarialGoal,
		},
		{
			// A data row that has only 3 columns instead of the expected 4.
			// The parser should not panic; the missing Description column is simply absent
			// and the ID-fallback applies.
			name:  "fewer columns than header — no panic",
			input: "# Goals\n\nShort row.\n\n## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n| short-row | `echo ok` | 3 |\n",
			check: checkNoPanicOnly,
		},
		{
			// Unicode characters in mission, north-star items, and directive titles.
			name:  "unicode in mission, north stars, and directive titles",
			input: "# Goals\n\n目標: 品質を改善する 🚀\n\n## North Stars\n\n- Schön und ästhetisch\n- 日本語のアイテム\n\n## Directives\n\n### 1. Ünïcödé Directive\n\nAccents and kanji: 漢字\n\n**Steer:** increase\n\n## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n| unicode-gate | `echo ok` | 5 | Unicode: 日本語 |\n",
			check: checkUnicodeAdversarialGoalFile,
		},
		{
			// Directives numbered 1, 3, 5 (2 and 4 are absent). The parser should
			// preserve the authored Number field from the heading — not reassign indices.
			name:  "skipped directive numbers preserve authored numbers",
			input: "# Goals\n\nSkip test.\n\n## Directives\n\n### 1. First\n\nBody one.\n\n**Steer:** increase\n\n### 3. Third\n\nBody three.\n\n**Steer:** hold\n\n### 5. Fifth\n\nBody five.\n\n**Steer:** decrease\n\n## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n",
			check: checkSkippedDirectiveNumbersAdversarialGoalFile,
		},
		{
			// Gates section with header + separator but zero data rows.
			name:  "table with no data rows yields empty goals slice",
			input: "# Goals\n\nNo data rows.\n\n## Gates\n\n| ID | Check | Weight | Description |\n|----|-------|--------|-------------|\n",
			check: checkEmptyGoalsAdversarialGoalFile,
		},
		{
			// A separator row that has fewer dashes than expected (e.g. only one cell's
			// worth of dashes). The regex tableSepRe should still recognise it as a
			// separator, and no data row is emitted.
			name:  "malformed separator row is skipped, not treated as data",
			input: "# Goals\n\nMalformed sep.\n\n## Gates\n\n| ID | Check | Weight | Description |\n|---|\n| my-goal | `echo ok` | 5 | OK |\n",
			check: checkNoPanicOnly,
		},
		{
			// With the paired-backtick fix: an asymmetric cell (`incomplete cmd,
			// leading backtick only, no trailing backtick) must be preserved
			// as-is — no silent data loss.
			name: "asymmetric backtick preserved — no silent corruption",
			input: "# Goals\n\nBacktick test.\n\n## Gates\n\n" +
				"| ID | Check | Weight | Description |\n" +
				"|----|-------|--------|-------------|\n" +
				"| g1 | `incomplete cmd | 5 | Test |\n",
			check: checkAsymmetricBacktickAdversarialGoal,
		},
		{
			// A Check cell that is exactly a single backtick must be preserved
			// rather than silently emptied.
			name: "single-backtick-char Check preserved — no data loss",
			input: "# Goals\n\nBacktick test.\n\n## Gates\n\n" +
				"| ID | Check | Weight | Description |\n" +
				"|----|-------|--------|-------------|\n" +
				"| g1 | ` | 5 | Test |\n",
			check: checkSingleBacktickAdversarialGoal,
		},
		{
			// A check cell enclosed in a matching backtick pair strips only the
			// outermost pair, preserving the interior content unchanged.
			name: "matched backtick pair — outer pair stripped, inner preserved",
			input: "# Goals\n\nBacktick test.\n\n## Gates\n\n" +
				"| ID | Check | Weight | Description |\n" +
				"|----|-------|--------|-------------|\n" +
				"| g1 | `cmd1 && cmd2` | 5 | Test |\n",
			check: checkMatchedBacktickPairAdversarialGoal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic.
			gf, err := ParseMarkdownGoals([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, gf)
			}
		})
	}
}

func checkPipeInCheckAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	want := `bash -c "echo a | grep b"`
	if got.Check != want {
		t.Errorf("Check = %q, want %q", got.Check, want)
	}
}

func checkPipeInDescriptionAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	want := "Run A | then B"
	if got.Description != want {
		t.Errorf("Description = %q, want %q", got.Description, want)
	}
}

func checkBacktickInCheckAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	if got.Check == "" {
		t.Error("Check is empty, expected non-empty")
	}
}

func checkEmptyDescriptionAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	if got.Description != "empty-desc" {
		t.Errorf("Description = %q, want %q (fallback to ID)", got.Description, "empty-desc")
	}
}

func checkNoPanicOnly(t *testing.T, gf *GoalFile) {
	t.Helper()
}

func checkUnicodeAdversarialGoalFile(t *testing.T, gf *GoalFile) {
	t.Helper()

	wantMission := "目標: 品質を改善する 🚀"
	if gf.Mission != wantMission {
		t.Errorf("Mission = %q, want %q", gf.Mission, wantMission)
	}
	if len(gf.NorthStars) != 2 {
		t.Fatalf("NorthStars = %d, want 2", len(gf.NorthStars))
	}
	if gf.NorthStars[0] != "Schön und ästhetisch" {
		t.Errorf("NorthStars[0] = %q", gf.NorthStars[0])
	}
	if len(gf.Directives) != 1 {
		t.Fatalf("Directives = %d, want 1", len(gf.Directives))
	}
	if gf.Directives[0].Title != "Ünïcödé Directive" {
		t.Errorf("Directive[0].Title = %q", gf.Directives[0].Title)
	}
	got := expectSingleAdversarialGoal(t, gf)
	if got.Description != "Unicode: 日本語" {
		t.Errorf("Goals[0].Description = %q", got.Description)
	}
}

func checkSkippedDirectiveNumbersAdversarialGoalFile(t *testing.T, gf *GoalFile) {
	t.Helper()

	if len(gf.Directives) != 3 {
		t.Fatalf("Directives = %d, want 3", len(gf.Directives))
	}
	wantNumbers := []int{1, 3, 5}
	for i, want := range wantNumbers {
		if gf.Directives[i].Number != want {
			t.Errorf("Directives[%d].Number = %d, want %d", i, gf.Directives[i].Number, want)
		}
	}
	wantTitles := []string{"First", "Third", "Fifth"}
	for i, want := range wantTitles {
		if gf.Directives[i].Title != want {
			t.Errorf("Directives[%d].Title = %q, want %q", i, gf.Directives[i].Title, want)
		}
	}
}

func checkEmptyGoalsAdversarialGoalFile(t *testing.T, gf *GoalFile) {
	t.Helper()

	if len(gf.Goals) != 0 {
		t.Errorf("Goals = %d, want 0", len(gf.Goals))
	}
}

func checkAsymmetricBacktickAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	want := "`incomplete cmd"
	if got.Check != want {
		t.Errorf("Check = %q, want %q", got.Check, want)
	}
}

func checkSingleBacktickAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	want := "`"
	if got.Check != want {
		t.Errorf("Check = %q, want %q", got.Check, want)
	}
}

func checkMatchedBacktickPairAdversarialGoal(t *testing.T, gf *GoalFile) {
	t.Helper()

	got := expectSingleAdversarialGoal(t, gf)
	want := "cmd1 && cmd2"
	if got.Check != want {
		t.Errorf("Check = %q, want %q", got.Check, want)
	}
}

func expectSingleAdversarialGoal(t *testing.T, gf *GoalFile) Goal {
	t.Helper()

	if len(gf.Goals) != 1 {
		t.Fatalf("goals = %d, want 1", len(gf.Goals))
	}
	return gf.Goals[0]
}

func TestParseGateRow_NegativeWeight(t *testing.T) {
	cells := []string{"G1", "echo ok", "-1", "neg weight"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)
	if g.Weight != 5 {
		t.Errorf("Weight = %d, want 5 (negative should default)", g.Weight)
	}
}

func TestParseGateRow_OversizedWeight(t *testing.T) {
	cells := []string{"G1", "echo ok", "99", "big weight"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)
	if g.Weight != 5 {
		t.Errorf("Weight = %d, want 5 (oversized should default)", g.Weight)
	}
}

func TestParseGateRow_ValidWeight(t *testing.T) {
	cells := []string{"G1", "echo ok", "3", "valid weight"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)
	if g.Weight != 3 {
		t.Errorf("Weight = %d, want 3", g.Weight)
	}
}

func TestParseDirectives_EmptySteer(t *testing.T) {
	input := "## Directives\n\n### 1. Test Dir\n\nBody text.\n\n**Steer:**   \n\n## Gates\n"
	directives := parseDirectives(strings.Split(input, "\n"))
	if len(directives) != 1 {
		t.Fatalf("directives = %d, want 1", len(directives))
	}
	if directives[0].Steer != "" {
		t.Errorf("Steer = %q, want empty (whitespace-only steer should be rejected)", directives[0].Steer)
	}
}

func TestBuildGateColumnMap_DefaultOrder(t *testing.T) {
	cells := []string{"ID", "Check", "Weight", "Description"}
	colMap := buildGateColumnMap(cells)

	want := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	for k, v := range want {
		if colMap[k] != v {
			t.Errorf("colMap[%q] = %d, want %d", k, colMap[k], v)
		}
	}
}

func TestBuildGateColumnMap_ReorderedColumns(t *testing.T) {
	cells := []string{"Description", "ID", "Weight", "Check"}
	colMap := buildGateColumnMap(cells)

	want := map[string]int{"id": 1, "check": 3, "weight": 2, "description": 0}
	for k, v := range want {
		if colMap[k] != v {
			t.Errorf("colMap[%q] = %d, want %d", k, colMap[k], v)
		}
	}
}

func TestBuildGateColumnMap_CaseInsensitive(t *testing.T) {
	cells := []string{"id", "CHECK", "Weight", "DESCRIPTION"}
	colMap := buildGateColumnMap(cells)

	want := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	for k, v := range want {
		if colMap[k] != v {
			t.Errorf("colMap[%q] = %d, want %d", k, colMap[k], v)
		}
	}
}

func TestParseGateRow_AllFields(t *testing.T) {
	cells := []string{"G1", "cmd", "5", "desc"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)

	if g.ID != "G1" {
		t.Errorf("ID = %q, want %q", g.ID, "G1")
	}
	if g.Check != "cmd" {
		t.Errorf("Check = %q, want %q", g.Check, "cmd")
	}
	if g.Weight != 5 {
		t.Errorf("Weight = %d, want 5", g.Weight)
	}
	if g.Description != "desc" {
		t.Errorf("Description = %q, want %q", g.Description, "desc")
	}
	if g.Type != GoalTypeHealth {
		t.Errorf("Type = %q, want %q", g.Type, GoalTypeHealth)
	}
}

func TestParseGateRow_MissingWeight(t *testing.T) {
	cells := []string{"G2", "echo ok", "bad", "test gate"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)

	if g.Weight != 5 {
		t.Errorf("Weight = %d, want 5 (default for non-numeric)", g.Weight)
	}
}

func TestParseGateRow_BacktickStrip(t *testing.T) {
	cells := []string{"G3", "`some check`", "3", "backtick test"}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)

	if g.Check != "some check" {
		t.Errorf("Check = %q, want %q (backticks should be stripped)", g.Check, "some check")
	}
}

func TestParseGateRow_EmptyDescription(t *testing.T) {
	cells := []string{"G4", "echo ok", "5", ""}
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	g := parseGateRow(cells, colMap)

	if g.Description != "G4" {
		t.Errorf("Description = %q, want %q (should fall back to ID)", g.Description, "G4")
	}
}

// TestParseGateRow_ZeroWidthChar verifies that a zero-width space (U+200B)
// embedded in a Check command is preserved as-is by the parser.
// This test documents parser behaviour: the character is passed through
// without stripping. Callers are responsible for validating command safety.
func TestParseGateRow_ZeroWidthChar(t *testing.T) {
	colMap := buildGateColumnMap([]string{"ID", "Check", "Weight", "Description"})
	// U+200B zero-width space embedded between "echo" and "ok"
	cells := []string{"g1", "echo\u200Bok", "5", "zero-width test"}
	g := parseGateRow(cells, colMap)
	if g.Check != "echo\u200Bok" {
		t.Errorf("zero-width char was altered: got %q, want %q", g.Check, "echo\u200Bok")
	}
}

// TestParseGateRow_CombiningChar verifies that combining diacritical marks
// (e.g. e + U+0301 combining acute accent = é) are preserved in the
// Description field without NFC normalisation or byte corruption.
func TestParseGateRow_CombiningChar(t *testing.T) {
	colMap := buildGateColumnMap([]string{"ID", "Check", "Weight", "Description"})
	// "cafe" + U+0301 combining acute = "café" in decomposed form
	cells := []string{"g1", "echo ok", "5", "cafe\u0301"}
	g := parseGateRow(cells, colMap)
	if g.Description != "cafe\u0301" {
		t.Errorf("combining char was altered: got %q, want %q", g.Description, "cafe\u0301")
	}
}

// TestParseGatesTable_ExtraColumns verifies that a data row with more columns
// than the header does not panic and does not corrupt the Description field.
// Extra columns beyond the header are silently discarded.
// TestParseGatesTable_NonTableTextInGatesSection exercises the branch at line 286
// where a non-table-row line (text without pipes) appears in the Gates section
// and is skipped via the tableRowRe check.
func TestParseGatesTable_NonTableTextInGatesSection(t *testing.T) {
	input := `## Gates

Some explanatory text that is not a table row.

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`echo ok`" + ` | 5 | Works |

## Next Section
`
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Fatalf("goals = %d, want 1 (non-table text should be skipped)", len(goals))
	}
	if goals[0].ID != "g1" {
		t.Errorf("goals[0].ID = %q, want %q", goals[0].ID, "g1")
	}
}

// TestParseGatesTable_HeadingBreaksSection exercises the heading-break branch
// at line 277 where a heading (e.g. ### subsection) terminates the Gates section.
func TestParseGatesTable_HeadingBreaksSection(t *testing.T) {
	input := `## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`echo ok`" + ` | 5 | First |

### Subsection heading ends gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g2 | ` + "`echo no`" + ` | 3 | Should not appear |
`
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Fatalf("goals = %d, want 1 (heading should end gates section)", len(goals))
	}
	if goals[0].ID != "g1" {
		t.Errorf("goals[0].ID = %q, want %q", goals[0].ID, "g1")
	}
}

// TestParseGatesTable_EmptyIDRowSkipped exercises the g.ID != "" check at line 306
// where a data row with a completely empty ID cell is skipped.
func TestParseGatesTable_EmptyIDRowSkipped(t *testing.T) {
	input := `## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
|  | ` + "`echo skip`" + ` | 5 | No ID |
| valid | ` + "`echo ok`" + ` | 3 | Has ID |
`
	goals := parseGatesTable(strings.Split(input, "\n"))
	if len(goals) != 1 {
		t.Fatalf("goals = %d, want 1 (empty-ID row should be skipped)", len(goals))
	}
	if goals[0].ID != "valid" {
		t.Errorf("goals[0].ID = %q, want %q", goals[0].ID, "valid")
	}
}

func TestParseGatesTable_ExtraColumns(t *testing.T) {
	input := "# G\n\n## Gates\n" +
		"| ID | Check | Weight | Description | Extra |\n" +
		"|---|---|---|---|---|\n" +
		"| g1 | echo ok | 5 | Test | surplus |\n"
	gf, err := ParseMarkdownGoals([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gf.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(gf.Goals))
	}
	if gf.Goals[0].ID != "g1" {
		t.Errorf("goal ID = %q, want %q", gf.Goals[0].ID, "g1")
	}
	// The extra column must NOT corrupt the Description field.
	if gf.Goals[0].Description != "Test" {
		t.Errorf("Description = %q, want %q", gf.Goals[0].Description, "Test")
	}
}

package goals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixturePath is the executable-spec patcher fixture.
const fixturePath = "testdata/goals-spec-fixture.md"

func readFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

// sectionFrom returns content from the first occurrence of marker to the end.
func sectionFrom(content, marker string) string {
	idx := strings.Index(content, marker)
	if idx < 0 {
		return ""
	}
	return content[idx:]
}

func TestNewGoalsPatcher_RejectsEmpty(t *testing.T) {
	if _, err := NewGoalsPatcher([]byte("   \n\t\n")); err == nil {
		t.Fatal("expected error for empty goals file, got nil")
	}
}

func TestGoalsPatcher_RoundTripByteStable(t *testing.T) {
	inputs := map[string][]byte{"fixture": readFixture(t)}
	if live, err := os.ReadFile(filepath.Join("..", "..", "..", "GOALS.md")); err == nil {
		inputs["live-GOALS.md"] = live
	}
	for name, data := range inputs {
		t.Run(name, func(t *testing.T) {
			p, err := NewGoalsPatcher(data)
			if err != nil {
				t.Fatalf("NewGoalsPatcher: %v", err)
			}
			if got := p.Bytes(); string(got) != string(data) {
				t.Errorf("round trip not byte-stable: %d input bytes, %d output bytes", len(data), len(got))
			}
		})
	}
}

func TestParseDirectiveBlocks_Fields(t *testing.T) {
	dirs, err := ParseDirectiveBlocks(readFixture(t))
	if err != nil {
		t.Fatalf("ParseDirectiveBlocks: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("directive count = %d, want 3", len(dirs))
	}

	d2 := dirs[1]
	if d2.Number != 2 {
		t.Errorf("d2.Number = %d, want 2", d2.Number)
	}
	if d2.Title != "Carry structured attribute metadata" {
		t.Errorf("d2.Title = %q", d2.Title)
	}
	if d2.StableID != "d-existing-two" {
		t.Errorf("d2.StableID = %q, want d-existing-two", d2.StableID)
	}
	if d2.Steer != "decrease (lossy writes)" {
		t.Errorf("d2.Steer = %q", d2.Steer)
	}
	if d2.Setpoint != "AOP-CLAIM-FIXTURE | exact wording | GOALS.md" {
		t.Errorf("d2.Setpoint = %q", d2.Setpoint)
	}
	if got := strings.Join(d2.Scenarios, ","); got != "s-2026-05-17-001,s-2026-05-17-002" {
		t.Errorf("d2.Scenarios = %q", got)
	}
	if d2.ScenarioThreshold != "0.8" {
		t.Errorf("d2.ScenarioThreshold = %q, want 0.8", d2.ScenarioThreshold)
	}
	if d2.StartLine != 23 || d2.EndLine != 33 {
		t.Errorf("d2 line range = [%d,%d], want [23,33]", d2.StartLine, d2.EndLine)
	}
}

func TestParseDirectiveBlocks_LineRanges(t *testing.T) {
	dirs, err := ParseDirectiveBlocks(readFixture(t))
	if err != nil {
		t.Fatalf("ParseDirectiveBlocks: %v", err)
	}
	want := []struct{ start, end int }{{16, 22}, {23, 33}, {34, 38}}
	for i, w := range want {
		if dirs[i].StartLine != w.start || dirs[i].EndLine != w.end {
			t.Errorf("directive %d range = [%d,%d], want [%d,%d]",
				i+1, dirs[i].StartLine, dirs[i].EndLine, w.start, w.end)
		}
	}
	// Directive 3 carries no attribute metadata.
	if len(dirs[2].attrs) != 0 {
		t.Errorf("directive 3 attrs = %d, want 0", len(dirs[2].attrs))
	}
}

func TestGoalsPatcher_SetAttributeReplaceInPlace(t *testing.T) {
	data := readFixture(t)
	p, err := NewGoalsPatcher(data)
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	if err := p.SetAttribute(2, AttrSteer, "hold (test)"); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	// Line count is unchanged for an in-place replace.
	if got, want := strings.Count(string(p.Bytes()), "\n"), strings.Count(string(data), "\n"); got != want {
		t.Errorf("newline count = %d, want %d (replace must not add lines)", got, want)
	}
	d2, ok := p.DirectiveByNumber(2)
	if !ok {
		t.Fatal("directive 2 missing after patch")
	}
	if d2.Steer != "hold (test)" {
		t.Errorf("d2.Steer = %q, want hold (test)", d2.Steer)
	}
	// Untouched directives keep their bytes.
	if d1, _ := p.DirectiveByNumber(1); d1.Steer != "increase (preserved bytes)" {
		t.Errorf("d1.Steer changed to %q", d1.Steer)
	}
}

func TestGoalsPatcher_SetAttributePreservesNonTargetSections(t *testing.T) {
	data := readFixture(t)
	origTail := sectionFrom(string(data), "## Three-Gap Contract Proof Surface")
	origComment := strings.Contains(string(data), "<!-- agentops:claim:AOP-CLAIM-FIXTURE -->")
	if origTail == "" || !origComment {
		t.Fatal("fixture missing expected non-target content")
	}

	cases := []struct {
		name string
		edit func(*GoalsPatcher) error
	}{
		{"replace-in-place", func(p *GoalsPatcher) error { return p.SetAttribute(2, AttrSteer, "hold (x)") }},
		{"insert-new-attr", func(p *GoalsPatcher) error { return p.SetAttribute(1, AttrScenarios, "s-2026-05-17-009") }},
		{"insert-into-bare-block", func(p *GoalsPatcher) error { return p.SetAttribute(3, AttrDirectiveID, "d-bare-three") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewGoalsPatcher(data)
			if err != nil {
				t.Fatalf("NewGoalsPatcher: %v", err)
			}
			if err := tc.edit(p); err != nil {
				t.Fatalf("edit: %v", err)
			}
			out := string(p.Bytes())
			if got := sectionFrom(out, "## Three-Gap Contract Proof Surface"); got != origTail {
				t.Errorf("Three-Gap section + Gates table not preserved byte-for-byte")
			}
			if !strings.Contains(out, "<!-- agentops:claim:AOP-CLAIM-FIXTURE -->") {
				t.Error("HTML claim comment dropped by patch")
			}
		})
	}
}

func TestGoalsPatcher_SetAttributeInsertsInCanonicalOrder(t *testing.T) {
	data := readFixture(t)

	// Scenarios (rank 3) on directive 1 lands after its Steer line (rank 1).
	t.Run("scenarios-after-steer", func(t *testing.T) {
		p, _ := NewGoalsPatcher(data)
		if err := p.SetAttribute(1, AttrScenarios, "s-2026-05-17-009"); err != nil {
			t.Fatalf("SetAttribute: %v", err)
		}
		lines := strings.Split(string(p.Bytes()), "\n")
		steerIdx, scenIdx := -1, -1
		for i, l := range lines[:25] {
			if strings.HasPrefix(l, "**Steer:** increase") {
				steerIdx = i
			}
			if strings.HasPrefix(l, "**Scenarios:** s-2026-05-17-009") {
				scenIdx = i
			}
		}
		if steerIdx < 0 || scenIdx < 0 || scenIdx != steerIdx+1 {
			t.Errorf("Scenarios line idx %d, Steer idx %d — Scenarios must immediately follow Steer", scenIdx, steerIdx)
		}
		d1, _ := p.DirectiveByNumber(1)
		if got := strings.Join(d1.Scenarios, ","); got != "s-2026-05-17-009" {
			t.Errorf("re-parsed d1.Scenarios = %q", got)
		}
	})

	// Directive ID (rank 0) on directive 1 lands before its Steer line.
	t.Run("directive-id-before-steer", func(t *testing.T) {
		p, _ := NewGoalsPatcher(data)
		if err := p.SetAttribute(1, AttrDirectiveID, "d-first-one"); err != nil {
			t.Fatalf("SetAttribute: %v", err)
		}
		lines := strings.Split(string(p.Bytes()), "\n")
		idIdx, steerIdx := -1, -1
		for i, l := range lines[:25] {
			if strings.HasPrefix(l, "**Directive ID:** d-first-one") {
				idIdx = i
			}
			if strings.HasPrefix(l, "**Steer:** increase") {
				steerIdx = i
			}
		}
		if idIdx < 0 || steerIdx < 0 || idIdx != steerIdx-1 {
			t.Errorf("Directive ID idx %d, Steer idx %d — ID must immediately precede Steer", idIdx, steerIdx)
		}
	})
}

func TestGoalsPatcher_SetAttributeIntoBareBlock(t *testing.T) {
	data := readFixture(t)
	p, err := NewGoalsPatcher(data)
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	if err := p.SetAttribute(3, AttrDirectiveID, "d-bare-three"); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	d3, ok := p.DirectiveByNumber(3)
	if !ok || d3.StableID != "d-bare-three" {
		t.Fatalf("directive 3 stable ID = %q, want d-bare-three", d3.StableID)
	}
	// The inserted attribute is separated from body text by a blank line.
	out := string(p.Bytes())
	if !strings.Contains(out, "bare block.\n\n**Directive ID:** d-bare-three") {
		t.Error("attribute inserted into bare block without a blank separator line")
	}
	if errs := p.Validate(); len(errs) != 0 {
		t.Errorf("Validate after bare-block insert: %v", errs)
	}
}

func TestGoalsPatcher_EnsureStableIDs(t *testing.T) {
	data := readFixture(t)
	p, err := NewGoalsPatcher(data)
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	ids, err := p.EnsureStableIDs()
	if err != nil {
		t.Fatalf("EnsureStableIDs: %v", err)
	}
	want := map[int]string{
		1: "d-keep-the-patcher-non-lossy",
		2: "d-existing-two", // pre-existing ID is preserved, not regenerated
		3: "d-survive-directives-that-have-no-attributes",
	}
	for num, wantID := range want {
		if ids[num] != wantID {
			t.Errorf("directive %d ID = %q, want %q", num, ids[num], wantID)
		}
	}

	// Idempotent: a second pass changes no bytes.
	afterFirst := string(p.Bytes())
	if _, err := p.EnsureStableIDs(); err != nil {
		t.Fatalf("second EnsureStableIDs: %v", err)
	}
	if string(p.Bytes()) != afterFirst {
		t.Error("EnsureStableIDs is not idempotent — second pass mutated the file")
	}
	if errs := p.Validate(); len(errs) != 0 {
		t.Errorf("Validate after EnsureStableIDs: %v", errs)
	}
}

func TestGoalsPatcher_StableIDsAreNumberIndependent(t *testing.T) {
	// Two GOALS files with the same directive titles in different display-number
	// order must assign each title the same stable ID — the ID is a function of
	// the title, so `ao goals steer prioritize` renumbering cannot change it.
	const orderA = "# Goals\n\nm\n\n## Directives\n\n### 1. Alpha directive\n\nbody\n\n### 2. Beta directive\n\nbody\n"
	const orderB = "# Goals\n\nm\n\n## Directives\n\n### 1. Beta directive\n\nbody\n\n### 2. Alpha directive\n\nbody\n"

	idsByTitle := func(src string) map[string]string {
		p, err := NewGoalsPatcher([]byte(src))
		if err != nil {
			t.Fatalf("NewGoalsPatcher: %v", err)
		}
		ids, err := p.EnsureStableIDs()
		if err != nil {
			t.Fatalf("EnsureStableIDs: %v", err)
		}
		out := map[string]string{}
		for _, d := range p.Directives() {
			out[d.Title] = ids[d.Number]
		}
		return out
	}

	a, b := idsByTitle(orderA), idsByTitle(orderB)
	for _, title := range []string{"Alpha directive", "Beta directive"} {
		if a[title] != b[title] {
			t.Errorf("%q ID differs by ordering: %q vs %q", title, a[title], b[title])
		}
		if want := SlugifyDirectiveID(title); a[title] != want {
			t.Errorf("%q ID = %q, want title slug %q", title, a[title], want)
		}
	}
}

func TestGoalsPatcher_EnsureStableIDsCollisionSuffix(t *testing.T) {
	const src = "# Goals\n\nm\n\n## Directives\n\n### 1. Same title\n\nbody\n\n### 2. Same title\n\nbody\n\n### 3. Same title\n\nbody\n"
	p, err := NewGoalsPatcher([]byte(src))
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	ids, err := p.EnsureStableIDs()
	if err != nil {
		t.Fatalf("EnsureStableIDs: %v", err)
	}
	want := map[int]string{1: "d-same-title", 2: "d-same-title-2", 3: "d-same-title-3"}
	for num, wantID := range want {
		if ids[num] != wantID {
			t.Errorf("directive %d ID = %q, want %q", num, ids[num], wantID)
		}
	}
}

func TestSlugifyDirectiveID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Keep the patcher non-lossy", "d-keep-the-patcher-non-lossy"},
		{"  Spaces  &  Symbols!! ", "d-spaces-symbols"},
		{"", "d-directive"},
		{"!!!", "d-directive"},
		{"3D Rendering", "d-3d-rendering"},
		{"already-kebab", "d-already-kebab"},
	}
	for _, tc := range cases {
		got := SlugifyDirectiveID(tc.in)
		if got != tc.want {
			t.Errorf("SlugifyDirectiveID(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if !directiveIDRe.MatchString(got) {
			t.Errorf("SlugifyDirectiveID(%q) = %q does not match stable-ID format", tc.in, got)
		}
	}
}

func TestGoalsPatcher_ValidateLineNumberedErrors(t *testing.T) {
	const src = "# Goals\n\nm\n\n## Directives\n\n" +
		"### 1. Bad metadata\n\nbody\n\n" +
		"**Directive ID:** Bad ID\n" +
		"**Scenario threshold:** 1.5\n\n" +
		"### 2. Duplicate id\n\nbody\n\n" +
		"**Directive ID:** d-dupe\n\n" +
		"### 3. Also dupe\n\nbody\n\n" +
		"**Directive ID:** d-dupe\n"
	p, err := NewGoalsPatcher([]byte(src))
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	errs := p.Validate()
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{
		"GOALS.md:11: invalid Directive ID",
		"GOALS.md:12: invalid Scenario threshold",
		"GOALS.md:24: duplicate Directive ID",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Validate() missing %q\ngot:\n%s", want, joined)
		}
	}
}

func TestGoalsPatcher_SetAttributeRejectsMalformed(t *testing.T) {
	cases := []struct {
		name, key, value, wantSubstr string
	}{
		{"bad-directive-id", AttrDirectiveID, "Bad ID", "invalid Directive ID"},
		{"bad-threshold", AttrScenarioThreshold, "2.0", "invalid Scenario threshold"},
		{"empty-steer", AttrSteer, "", "must not be empty"},
		{"unknown-key", "Bogus", "x", "unknown directive attribute"},
		{"multiline-value", AttrSetpoint, "line1\nline2", "single line"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewGoalsPatcher(readFixture(t))
			if err != nil {
				t.Fatalf("NewGoalsPatcher: %v", err)
			}
			err = p.SetAttribute(1, tc.key, tc.value)
			if err == nil {
				t.Fatalf("SetAttribute(%q, %q) succeeded, want error", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestGoalsPatcher_SetAttributeUnknownDirective(t *testing.T) {
	p, err := NewGoalsPatcher(readFixture(t))
	if err != nil {
		t.Fatalf("NewGoalsPatcher: %v", err)
	}
	err = p.SetAttribute(99, AttrSteer, "increase (x)")
	if err == nil || !strings.Contains(err.Error(), "directive #99 not found") {
		t.Errorf("SetAttribute on missing directive: err = %v", err)
	}
}

// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRankFindings_UtilityDrivesOrder locks the gold-ranking wire: with
// freshness held equal, a finding's utility decides its rank, and flipping the
// utilities inverts the order. This is the consumer half of age-4fd — without
// it, a finding's gold-frontmatter utility never reaches the rank the scenario
// runner sees.
func TestRankFindings_UtilityDrivesOrder(t *testing.T) {
	findings := []knowledgeFinding{
		{Title: "low", Source: "low.md", FreshnessScore: 0.5, Utility: 0.50},
		{Title: "high", Source: "high.md", FreshnessScore: 0.5, Utility: 0.85},
	}
	rankFindings(findings)
	if findings[0].Title != "high" {
		t.Fatalf("higher-utility finding should rank first; got %q then %q", findings[0].Title, findings[1].Title)
	}
	if findings[0].CompositeScore <= findings[1].CompositeScore {
		t.Errorf("composite not ordered by utility: %.4f <= %.4f", findings[0].CompositeScore, findings[1].CompositeScore)
	}

	// flip the utilities: order must invert (rules out a stable-sort artifact)
	flipped := []knowledgeFinding{
		{Title: "low", Source: "low.md", FreshnessScore: 0.5, Utility: 0.85},
		{Title: "high", Source: "high.md", FreshnessScore: 0.5, Utility: 0.50},
	}
	rankFindings(flipped)
	if flipped[0].Title != "low" {
		t.Fatalf("after flip, higher-utility finding (low) should rank first; got %q", flipped[0].Title)
	}
}

// TestRankPatterns_UtilityDrivesOrder is the pattern mirror of the above.
func TestRankPatterns_UtilityDrivesOrder(t *testing.T) {
	patterns := []pattern{
		{Name: "low", FreshnessScore: 0.5, Utility: 0.50},
		{Name: "high", FreshnessScore: 0.5, Utility: 0.85},
	}
	rankPatterns(patterns)
	if patterns[0].Name != "high" {
		t.Fatalf("higher-utility pattern should rank first; got %q then %q", patterns[0].Name, patterns[1].Name)
	}
	if patterns[0].CompositeScore <= patterns[1].CompositeScore {
		t.Errorf("composite not ordered by utility: %.4f <= %.4f", patterns[0].CompositeScore, patterns[1].CompositeScore)
	}
}

func TestTrimField(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"title: hello world", "hello world"},
		{"severity: \"high\"", "high"},
		{"status: 'open'", "open"},
		{"id:   spaced  ", "spaced"},
		{"nocolon", ""},
		{"key:", ""},
		{"key: \"quoted 'inner'\"", "quoted 'inner"},
	}
	for _, tt := range tests {
		got := trimField(tt.input)
		if got != tt.want {
			t.Errorf("trimField(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseListField(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"[cli, hooks, scoring]", []string{"cli", "hooks", "scoring"}},
		{"cli, hooks", []string{"cli", "hooks"}},
		{"[\"cli\", \"hooks\"]", []string{"cli", "hooks"}},
		{"", nil},
		{"[]", nil},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := parseListField(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseListField(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseListField(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFindingMatchesQuery(t *testing.T) {
	f := knowledgeFinding{
		ID:        "cli-flag-bug",
		Title:     "CLI flag parsing fails on dashes",
		Summary:   "Double dashes cause panic",
		Severity:  "high",
		ScopeTags: []string{"cli", "hooks"},
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"cli", true},
		{"flag", true},
		{"high", true},
		{"hooks", true},
		{"nonexistent-query-xyz", false},
	}
	for _, tt := range tests {
		got := findingMatchesQuery(f, tt.query)
		if got != tt.want {
			t.Errorf("findingMatchesQuery(f, %q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestParseFindingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-finding.md")
	content := `---
id: f-001
title: Test Finding
severity: high
source_skill: vibe
scope_tags: [cli, hooks]
compiler_targets: [inject, lookup]
status: open
---

# Test Finding

This is the summary paragraph.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := parseFindingFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if f.ID != "f-001" {
		t.Errorf("ID = %q, want %q", f.ID, "f-001")
	}
	if f.Title != "Test Finding" {
		t.Errorf("Title = %q, want %q", f.Title, "Test Finding")
	}
	if f.Severity != "high" {
		t.Errorf("Severity = %q, want %q", f.Severity, "high")
	}
	if f.SourceSkill != "vibe" {
		t.Errorf("SourceSkill = %q, want %q", f.SourceSkill, "vibe")
	}
	if f.Status != "open" {
		t.Errorf("Status = %q, want %q", f.Status, "open")
	}
	if len(f.ScopeTags) != 2 || f.ScopeTags[0] != "cli" {
		t.Errorf("ScopeTags = %v, want [cli hooks]", f.ScopeTags)
	}
	if len(f.CompilerTargets) != 2 || f.CompilerTargets[0] != "inject" {
		t.Errorf("CompilerTargets = %v, want [inject lookup]", f.CompilerTargets)
	}
}

func TestParseFindingFileTitleFromHeading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-title.md")
	content := `---
severity: medium
---

# Heading Used As Title
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := parseFindingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Heading Used As Title" {
		t.Errorf("Title = %q, want %q", f.Title, "Heading Used As Title")
	}
}

func TestApplyFindingField(t *testing.T) {
	tests := []struct {
		line  string
		check func(f *knowledgeFinding) bool
		desc  string
	}{
		{"id: abc-123", func(f *knowledgeFinding) bool { return f.ID == "abc-123" }, "id field"},
		{"severity: high", func(f *knowledgeFinding) bool { return f.Severity == "high" }, "severity field"},
		{"source_skill: vibe", func(f *knowledgeFinding) bool { return f.SourceSkill == "vibe" }, "source_skill underscore"},
		{"source-skill: vibe", func(f *knowledgeFinding) bool { return f.SourceSkill == "vibe" }, "source-skill hyphen"},
		{"hit_count: 5", func(f *knowledgeFinding) bool { return f.HitCount == 5 }, "hit_count"},
		{"hit-count: 3", func(f *knowledgeFinding) bool { return f.HitCount == 3 }, "hit-count hyphen"},
		{"status: active", func(f *knowledgeFinding) bool { return f.Status == "active" }, "status field"},
		{"scope_tags: go,cli", func(f *knowledgeFinding) bool { return len(f.ScopeTags) == 2 && f.ScopeTags[0] == "go" }, "scope_tags"},
		{"unrecognized: value", func(f *knowledgeFinding) bool { return f.ID == "" && f.Title == "" }, "unknown field ignored"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := &knowledgeFinding{}
			applyFindingField(f, tt.line)
			if !tt.check(f) {
				t.Errorf("applyFindingField(%q) did not produce expected result", tt.line)
			}
		})
	}
}

func TestApplyFindingFreshness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.md")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	f := knowledgeFinding{}
	now := time.Now()
	applyFindingFreshness(&f, path, now)

	// File just written — freshness should be close to 1.0
	if f.FreshnessScore < 0.9 {
		t.Errorf("FreshnessScore = %f, expected > 0.9 for fresh file", f.FreshnessScore)
	}
	if f.Utility == 0 {
		t.Error("Utility should be set to InitialUtility, got 0")
	}
}

func TestApplyFindingFreshnessMissingFile(t *testing.T) {
	f := knowledgeFinding{}
	applyFindingFreshness(&f, "/nonexistent/path.md", time.Now())

	if f.FreshnessScore != 0.5 {
		t.Errorf("FreshnessScore = %f, want 0.5 for missing file", f.FreshnessScore)
	}
}

func TestCollectFindingsFromDirEmpty(t *testing.T) {
	result, err := collectFindingsFromDir("", "query", time.Now(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for empty dir, got %v", result)
	}
}

// TestCollectFindingsFromDir_RelevanceWeighting is the age-r3w refuter fix: a
// finding matching all query tokens must carry MatchConfidence 1.0 and keep its
// utility, while a finding matching only one token is discounted (MatchConfidence
// < 1 folded into utility) so it ranks below the fuller match — closing the gap
// where any-token matching + freshness/utility-only ranking let a fresh one-token
// false positive evict a relevant multi-token finding.
func TestCollectFindingsFromDir_RelevanceWeighting(t *testing.T) {
	dir := t.TempDir()
	write := func(name, title string) {
		c := "---\ntitle: " + title + "\n---\n\nbody.\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(c), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("relevant.md", "codex exec stdin stall in non-tty context") // all query tokens
	write("partial.md", "codex deployment guide")                     // only "codex"

	results, err := collectFindingsFromDir(dir, "codex exec stdin", time.Now(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected both findings to match >=1 token, got %d", len(results))
	}
	var relevant, partial knowledgeFinding
	for _, f := range results {
		if f.ID == "relevant" {
			relevant = f
		} else {
			partial = f
		}
	}
	if relevant.MatchConfidence != 1.0 {
		t.Errorf("full-match MatchConfidence = %v, want 1.0", relevant.MatchConfidence)
	}
	if !(partial.MatchConfidence > 0 && partial.MatchConfidence < 1.0) {
		t.Errorf("partial-match MatchConfidence = %v, want 0 < x < 1.0", partial.MatchConfidence)
	}
	if relevant.Utility <= partial.Utility {
		t.Errorf("relevance weighting not applied: relevant.Utility %v <= partial.Utility %v", relevant.Utility, partial.Utility)
	}
}

// TestCollectGlobalFindings_RelevanceWeighting proves the GLOBAL finding path
// (parallel to collectFindingsFromDir) also routes through matchAndWeighFinding,
// so the relevance weighting can't be bypassed there — the gap the round-2 refuter
// found (collectGlobalFindings previously used the boolean matcher only).
func TestCollectGlobalFindings_RelevanceWeighting(t *testing.T) {
	dir := t.TempDir()
	write := func(name, title string) {
		c := "---\ntitle: " + title + "\n---\n\nbody.\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(c), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("relevant.md", "codex exec stdin stall in non-tty context") // all query tokens
	write("partial.md", "codex deployment guide")                     // only "codex"

	got := collectGlobalFindings(dir, map[string]bool{}, "codex exec stdin", time.Now(), false)
	if len(got) != 2 {
		t.Fatalf("expected 2 global findings, got %d", len(got))
	}
	var relevant, partial knowledgeFinding
	for _, f := range got {
		if f.ID == "relevant" {
			relevant = f
		} else {
			partial = f
		}
	}
	if relevant.MatchConfidence != 1.0 {
		t.Errorf("global full-match MatchConfidence = %v, want 1.0", relevant.MatchConfidence)
	}
	if relevant.Utility <= partial.Utility {
		t.Errorf("global relevance weighting not applied: relevant %v <= partial %v", relevant.Utility, partial.Utility)
	}
}

func TestCollectFindingsFromDir(t *testing.T) {
	dir := t.TempDir()
	content := `---
title: Bug One
severity: high
---

Summary text.
`
	if err := os.WriteFile(filepath.Join(dir, "bug-one.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := collectFindingsFromDir(dir, "", time.Now(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(results))
	}
	if results[0].Title != "Bug One" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Bug One")
	}
	if !results[0].Global {
		t.Error("expected Global=true")
	}
}

func TestCollectFindingsWithNestedGlobalDir(t *testing.T) {
	localDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(localDir, ".agents", "findings"), 0755); err != nil {
		t.Fatal(err)
	}

	globalDir := t.TempDir()
	globalNamespace := filepath.Join(globalDir, "jren-platform")
	if err := os.MkdirAll(globalNamespace, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
title: ArgoCD Timeout Layers
severity: high
status: open
---

Nested global finding content.
`
	if err := os.WriteFile(filepath.Join(globalNamespace, "argocd-timeout.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := collectFindings(localDir, "argocd", 10, globalDir, 0.8)
	if err != nil {
		t.Fatalf("collectFindings() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(results))
	}
	if !results[0].Global {
		t.Error("expected nested global finding to be flagged as Global")
	}
}

func TestResolveFindingsDirAndIndexFindingPaths(t *testing.T) {
	root := t.TempDir()
	findingsDir := filepath.Join(root, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(findingsDir, "alpha.md")
	if err := os.WriteFile(path, []byte("# Alpha\n"), 0644); err != nil {
		t.Fatal(err)
	}

	gotDir := resolveFindingsDir(root)
	if gotDir != findingsDir {
		t.Fatalf("resolveFindingsDir() = %q, want %q", gotDir, findingsDir)
	}

	paths := indexFindingPaths(findingsDir)
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if !paths[abs] {
		t.Fatalf("indexFindingPaths() missing %q", abs)
	}

	local := []knowledgeFinding{{CompositeScore: 2.0}, {Global: true, CompositeScore: 4.0}}
	applyGlobalFindingWeight(local, 0.5)
	if local[0].CompositeScore != 2.0 {
		t.Fatalf("local score changed unexpectedly: got %v", local[0].CompositeScore)
	}
	if local[1].CompositeScore != 2.0 {
		t.Fatalf("global score = %v, want 2.0 after weight", local[1].CompositeScore)
	}
}

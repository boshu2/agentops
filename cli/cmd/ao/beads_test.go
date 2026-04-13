// Tests for `ao beads` subcommands — focused on pure-function logic
// (parsing, extraction, verification wiring) rather than end-to-end bd
// invocations. Tests that need bd override execBD/bdAvailable with fakes.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- beadSlugify ----------

func TestBeadSlugify_BasicKebab(t *testing.T) {
	got := beadSlugify("Fix the auth middleware bug", 40)
	want := "fix-the-auth-middleware-bug"
	if got != want {
		t.Fatalf("beadSlugify = %q, want %q", got, want)
	}
}

func TestBeadSlugify_CollapsesPunctuation(t *testing.T) {
	got := beadSlugify("M8 (C1 Option A): staging-tree refactor!", 40)
	want := "m8-c1-option-a-staging-tree-refactor"
	if got != want {
		t.Fatalf("beadSlugify = %q, want %q", got, want)
	}
}

func TestBeadSlugify_TruncatesAtMax(t *testing.T) {
	got := beadSlugify("This is a very long bead title that goes on and on", 20)
	if len(got) > 20 {
		t.Fatalf("beadSlugify len = %d, want <= 20 (got %q)", len(got), got)
	}
	if strings.HasSuffix(got, "-") {
		t.Fatalf("beadSlugify should not end with dash after truncation: %q", got)
	}
}

func TestBeadSlugify_EmptyInputReturnsUntitled(t *testing.T) {
	got := beadSlugify("", 40)
	if got != "untitled" {
		t.Fatalf("beadSlugify(empty) = %q, want %q", got, "untitled")
	}
}

func TestBeadSlugify_PunctuationOnlyReturnsUntitled(t *testing.T) {
	got := beadSlugify("!!! ??? ...", 40)
	if got != "untitled" {
		t.Fatalf("beadSlugify(punct-only) = %q, want %q", got, "untitled")
	}
}

// ---------- parseBDShow ----------

func TestParseBDShow_CanonicalFormat(t *testing.T) {
	raw := `○ na-h61 · M8 (C1 Option A): fitness.go staging-tree refactor   [● P2 · OPEN]
Owner: Boden Fuller · Type: task
Created: 2026-04-11 · Updated: 2026-04-11

DESCRIPTION
This is the bead description. It references cli/cmd/ao/fitness.go and
mentions func collectLearnings across 8 production callers.
[rerun: b4]`
	parsed, err := parseBDShow(raw)
	if err != nil {
		t.Fatalf("parseBDShow: %v", err)
	}
	if parsed.ID != "na-h61" {
		t.Errorf("ID = %q, want na-h61", parsed.ID)
	}
	if !strings.Contains(parsed.Title, "Option A") {
		t.Errorf("Title = %q, want to contain 'Option A'", parsed.Title)
	}
	if !strings.Contains(parsed.Status, "OPEN") {
		t.Errorf("Status = %q, want to contain 'OPEN'", parsed.Status)
	}
	if !strings.Contains(parsed.Description, "fitness.go") {
		t.Errorf("Description missing expected content: %q", parsed.Description)
	}
	if strings.Contains(parsed.Description, "[rerun:") {
		t.Errorf("Description should strip [rerun:] suffix, got: %q", parsed.Description)
	}
}

func TestParseBDShow_EmptyErrors(t *testing.T) {
	_, err := parseBDShow("")
	if err == nil {
		t.Fatal("parseBDShow(empty) should error")
	}
}

func TestParseBDShow_ClosedBeadCapturesCloseReason(t *testing.T) {
	raw := `✓ na-h61 · M8 — dedicated full-complexity session   [● P2 · CLOSED]
Owner: Boden Fuller · Type: task
Created: 2026-04-11 · Updated: 2026-04-11
Close reason: Closed by 7e4e34ad (feat(overnight)). The actual fix was a sequencing bug in loop.go:381.

DESCRIPTION
[rerun: b57]`
	parsed, err := parseBDShow(raw)
	if err != nil {
		t.Fatalf("parseBDShow: %v", err)
	}
	if parsed.ID != "na-h61" {
		t.Errorf("ID = %q, want na-h61", parsed.ID)
	}
	if !strings.Contains(parsed.Status, "CLOSED") {
		t.Errorf("Status = %q, want to contain CLOSED", parsed.Status)
	}
	if parsed.CloseReason == "" {
		t.Fatalf("expected CloseReason to be captured")
	}
	if !strings.Contains(parsed.CloseReason, "sequencing bug") {
		t.Errorf("CloseReason missing content: %q", parsed.CloseReason)
	}
	// Body() should prefer CloseReason for closed beads.
	body := parsed.Body()
	if !strings.Contains(body, "sequencing bug") {
		t.Errorf("Body() should return close reason for closed bead; got %q", body)
	}
}

func TestBdShowParsed_BodyPrefersCloseReason(t *testing.T) {
	p := &bdShowParsed{
		Description: "original description",
		CloseReason: "close reason body",
	}
	if got := p.Body(); got != "close reason body" {
		t.Errorf("Body() = %q, want %q", got, "close reason body")
	}
}

func TestBdShowParsed_BodyFallsBackToDescription(t *testing.T) {
	p := &bdShowParsed{Description: "original description"}
	if got := p.Body(); got != "original description" {
		t.Errorf("Body() = %q, want %q", got, "original description")
	}
}

// ---------- extractCitations ----------

func TestExtractCitations_FindsFileWithLine(t *testing.T) {
	desc := "See cli/internal/overnight/loop.go:444 for the halt logic."
	cites := extractCitations(desc)
	found := false
	for _, c := range cites {
		if c.Kind == "file" && strings.Contains(c.Raw, "loop.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file citation for loop.go; got %+v", cites)
	}
}

func TestExtractCitations_FindsFunctionReference(t *testing.T) {
	desc := "The bug is in func collectLearnings in inject_learnings.go."
	cites := extractCitations(desc)
	found := false
	for _, c := range cites {
		if c.Kind == "function" && strings.Contains(c.Raw, "collectLearnings") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected function citation for collectLearnings; got %+v", cites)
	}
}

func TestExtractCitations_FindsBacktickedSymbol(t *testing.T) {
	desc := "Check `SomeExportedThing` and also `anotherSymbol`."
	cites := extractCitations(desc)
	count := 0
	for _, c := range cites {
		if c.Kind == "symbol" {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("expected >= 2 symbol citations; got %d in %+v", count, cites)
	}
}

func TestExtractCitations_DeduplicatesAcrossKinds(t *testing.T) {
	desc := "file.go and file.go again"
	cites := extractCitations(desc)
	fileCount := 0
	for _, c := range cites {
		if c.Kind == "file" && c.Raw == "file.go" {
			fileCount++
		}
	}
	if fileCount != 1 {
		t.Fatalf("expected 1 file citation (deduped); got %d", fileCount)
	}
}

// ---------- verifyCitationInPlace (integration with real fs) ----------

func TestVerifyFileCitation_Fresh(t *testing.T) {
	dir := t.TempDir()
	// Create a file that should be "fresh".
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Citation{Kind: "file", Raw: "real.go"}
	verifyCitationInPlace(&c, dir)
	if c.Status != CitationFresh {
		t.Fatalf("status = %q, want FRESH (reason: %s)", c.Status, c.Reason)
	}
}

func TestVerifyFileCitation_Stale(t *testing.T) {
	dir := t.TempDir()
	c := Citation{Kind: "file", Raw: "nonexistent.go"}
	verifyCitationInPlace(&c, dir)
	if c.Status != CitationStale {
		t.Fatalf("status = %q, want STALE", c.Status)
	}
}

func TestVerifyFileCitation_StripsLineSuffix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Citation{Kind: "file", Raw: "real.go:42"}
	verifyCitationInPlace(&c, dir)
	if c.Status != CitationFresh {
		t.Fatalf("status with line suffix = %q, want FRESH (reason: %s)", c.Status, c.Reason)
	}
}

// ---------- isClosedStatus ----------

func TestIsClosedStatus_RecognisesCommonSpellings(t *testing.T) {
	cases := map[string]bool{
		"CLOSED":           true,
		"closed":           true,
		"DONE":             true,
		"RESOLVED":         true,
		" closed  ":        true,
		"● P2 · CLOSED":    true, // real bd status field shape
		"CLOSED P1 task":   true, // substring match at start
		"resolved in 2026": true, // substring anywhere
		"OPEN":             false,
		"in-progress":      false,
		"":                 false,
		"disclosed secret": true, // known false positive; substring match is the trade-off
	}
	for in, want := range cases {
		if got := isClosedStatus(in); got != want {
			t.Errorf("isClosedStatus(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestListBeadIDs_ParsesFlatAndTreeOutput(t *testing.T) {
	origBD, origAvail := execBD, bdAvailable
	defer func() { execBD, bdAvailable = origBD, origAvail }()
	bdAvailable = func() bool { return true }
	execBD = func(args ...string) ([]byte, error) {
		return []byte(`✓ na-0g5 ● P1 task Integrate behavioral discipline
✓ na-348 ● P1 epic Flywheel provenance-first live corpus control loop
├── ✓ na-348.1 ● P1 task Retro quick-capture provenance defaults
└── ✓ na-348.2 ● P1 task Nightly live retrieval proof loop
○ na-h61 · Open bead · [● P2 · OPEN]
not a bead line — should be skipped
`), nil
	}
	ids, err := listBeadIDs("all")
	if err != nil {
		t.Fatalf("listBeadIDs: %v", err)
	}
	want := []string{"na-0g5", "na-348", "na-348.1", "na-348.2", "na-h61"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

// ---------- bdAvailable degradation ----------

func TestVerifyBead_DegradesWhenBDAbsent(t *testing.T) {
	origAvail := bdAvailable
	defer func() { bdAvailable = origAvail }()
	bdAvailable = func() bool { return false }

	report, err := verifyBead("na-nothing")
	if err != nil {
		t.Fatalf("verifyBead should not error when bd absent: %v", err)
	}
	if report.BDAvailable {
		t.Fatalf("BDAvailable should be false")
	}
	if report.StaleCount != 0 {
		t.Fatalf("StaleCount should be 0 when bd absent, got %d", report.StaleCount)
	}
}

func TestAuditBeads_DegradesWhenBDAbsent(t *testing.T) {
	origAvail := bdAvailable
	defer func() { bdAvailable = origAvail }()
	bdAvailable = func() bool { return false }

	report, err := auditBeads(false)
	if err != nil {
		t.Fatalf("auditBeads should not error when bd absent: %v", err)
	}
	if report.BDAvailable {
		t.Fatalf("BDAvailable should be false")
	}
	if report.Error == "" {
		t.Fatalf("expected missing-bd error message")
	}
}

func TestAuditBeads_ClassifiesStaleAndConsolidatable(t *testing.T) {
	origBD, origAvail := execBD, bdAvailable
	origGit, origPattern := execGitLog, repoPatternExists
	defer func() {
		execBD, bdAvailable = origBD, origAvail
		execGitLog, repoPatternExists = origGit, origPattern
	}()

	bdAvailable = func() bool { return true }
	execBD = func(args ...string) ([]byte, error) {
		if len(args) >= 4 && args[0] == "list" && args[3] == "--json" {
			switch args[2] {
			case "open":
				return []byte(`[
{"id":"na-audit1","title":"Audit docs","description":"Update skills/swarm/SKILL.md and ` + "`MissingSymbolOne`" + `","created_at":"2026-04-01T00:00:00Z"},
{"id":"na-audit2","title":"Audit runtime","description":"Update skills/swarm/SKILL.md and ` + "`MissingSymbolTwo`" + `","created_at":"2026-04-01T00:00:00Z"}
]`), nil
			case "in_progress":
				return []byte(`[]`), nil
			}
		}
		return []byte(`[]`), nil
	}
	execGitLog = func(args ...string) (string, error) {
		return "", nil
	}
	repoPatternExists = func(pattern string) bool {
		return false
	}

	report, err := auditBeads(false)
	if err != nil {
		t.Fatalf("auditBeads: %v", err)
	}
	if report.Summary.Total != 2 {
		t.Fatalf("Total = %d, want 2", report.Summary.Total)
	}
	if report.Summary.LikelyStale != 2 {
		t.Fatalf("LikelyStale = %d, want 2", report.Summary.LikelyStale)
	}
	if report.Summary.Consolidatable != 2 {
		t.Fatalf("Consolidatable = %d, want 2", report.Summary.Consolidatable)
	}
	if len(report.Consolidatable) != 1 || report.Consolidatable[0].File != "skills/swarm/SKILL.md" {
		t.Fatalf("unexpected consolidation report: %+v", report.Consolidatable)
	}
}

func TestPatternExistsInRepoSearchesScopedRoots(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	if err := os.MkdirAll(filepath.Join("cli", "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir cli/cmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join("cli", "cmd", "sample.go"), []byte("package main\nconst auditToken = \"needle-root\"\n"), 0o644); err != nil {
		t.Fatalf("write searchable file: %v", err)
	}
	if !patternExistsInRepo("needle-root") {
		t.Fatalf("patternExistsInRepo should find literal patterns under scoped roots")
	}

	if err := os.MkdirAll(filepath.Join("scripts", "testdata"), 0o755); err != nil {
		t.Fatalf("mkdir scripts/testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join("scripts", "testdata", "ignored.sh"), []byte("ignored-needle\n"), 0o644); err != nil {
		t.Fatalf("write ignored testdata file: %v", err)
	}
	if patternExistsInRepo("ignored-needle") {
		t.Fatalf("patternExistsInRepo should skip ignored traversal directories")
	}
}

func TestClusterBeads_DegradesWhenBDAbsent(t *testing.T) {
	origAvail := bdAvailable
	defer func() { bdAvailable = origAvail }()
	bdAvailable = func() bool { return false }

	report, err := clusterBeads(false)
	if err != nil {
		t.Fatalf("clusterBeads should not error when bd absent: %v", err)
	}
	if report.BDAvailable {
		t.Fatalf("BDAvailable should be false")
	}
	if report.Error == "" {
		t.Fatalf("expected missing-bd error message")
	}
}

func TestClusterBeadRecords_GroupsSharedPathAndPrefersEpic(t *testing.T) {
	records := []beadRecord{
		{
			ID:          "na-epic",
			Title:       "Swarm cluster epic",
			Description: "Update skills/swarm/SKILL.md",
			IssueType:   "epic",
			Labels:      []string{"skill:swarm"},
		},
		{
			ID:          "na-task",
			Title:       "Swarm cluster task",
			Description: "Update skills/swarm/SKILL.md",
			IssueType:   "task",
			Labels:      []string{"skill:swarm"},
		},
		{
			ID:          "na-other",
			Title:       "Release note cleanup",
			Description: "Update docs/CHANGELOG.md",
			IssueType:   "task",
		},
	}

	clusters, unclustered := clusterBeadRecords(records)
	if len(clusters) != 1 {
		t.Fatalf("clusters = %+v, want exactly one", clusters)
	}
	if clusters[0].Representative != "na-epic" {
		t.Fatalf("Representative = %q, want na-epic", clusters[0].Representative)
	}
	if len(clusters[0].Beads) != 2 {
		t.Fatalf("cluster bead count = %d, want 2", len(clusters[0].Beads))
	}
	if len(unclustered) != 1 || unclustered[0].ID != "na-other" {
		t.Fatalf("unclustered = %+v, want na-other", unclustered)
	}
}

// ---------- verifyBead with execBD fake (end-to-end smoke) ----------

func TestVerifyBead_ParsesAndClassifiesCitations(t *testing.T) {
	origBD, origAvail := execBD, bdAvailable
	defer func() { execBD, bdAvailable = origBD, origAvail }()

	bdAvailable = func() bool { return true }
	execBD = func(args ...string) ([]byte, error) {
		return []byte(`○ na-fake · Fake bead for testing   [● P2 · OPEN]
Owner: Test · Type: task

DESCRIPTION
This bead cites cli/cmd/ao/beads.go (which exists) and also
cli/cmd/ao/does-not-exist-at-all.go (which does not).
`), nil
	}

	// Run from the repo cli/ dir so the real file actually exists.
	// Fallback: use os.Getwd and set cwd to repo root.
	report, err := verifyBead("na-fake")
	if err != nil {
		t.Fatalf("verifyBead: %v", err)
	}
	if report.TotalCount == 0 {
		t.Fatalf("expected citations extracted, got 0")
	}
	// We don't assert exact FRESH/STALE counts because the test cwd is
	// arbitrary — just prove the wiring works end-to-end.
	if report.TotalCount != report.FreshCount+report.StaleCount {
		t.Fatalf("citation counts inconsistent: total=%d fresh=%d stale=%d",
			report.TotalCount, report.FreshCount, report.StaleCount)
	}
}

// ---------- renderLearningBody ----------

func TestRenderLearningBody_IncludesFrontmatterAndBody(t *testing.T) {
	fm := LearningFrontmatter{
		Title:      "Test Bead",
		BeadID:     "na-test",
		Source:     "bd-close",
		Date:       "2026-04-11",
		Maturity:   "provisional",
		Provenance: "test provenance",
		Tags:       []string{"tag-a", "tag-b"},
	}
	parsed := &bdShowParsed{
		Description: "The closure reason goes here.\nMultiple lines allowed.",
	}
	got := renderLearningBody(fm, parsed)

	wantFragments := []string{
		"---",
		"bead_id: na-test",
		"source: bd-close",
		"maturity: provisional",
		"tag-a",
		"tag-b",
		"# Test Bead",
		"## Closure reason",
		"The closure reason goes here.",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(got, frag) {
			t.Errorf("renderLearningBody missing fragment %q in:\n%s", frag, got)
		}
	}
}

package doctor

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// refsTestAliases returns two stable alias rows (alias, canonical) from the
// live workspaceCanonicalAliases table. Tests consume the table as data — the
// canonical names are looked up, never hardcoded — but they do require these
// two long-standing rows to exist, and fail loudly if the registry drops them.
func refsTestAliases(t *testing.T) (skillAlias, skillCanonical, cliAlias, cliCanonical string) {
	t.Helper()
	skillAlias, cliAlias = "handoffs", "retros"
	skillCanonical, ok := workspaceCanonicalAliases[skillAlias]
	if !ok {
		t.Fatalf("alias registry no longer contains %q; update refsTestAliases", skillAlias)
	}
	cliCanonical, ok = workspaceCanonicalAliases[cliAlias]
	if !ok {
		t.Fatalf("alias registry no longer contains %q; update refsTestAliases", cliAlias)
	}
	return skillAlias, skillCanonical, cliAlias, cliCanonical
}

// writeRefsFile writes content to path, creating parent directories.
func writeRefsFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// refsFixtureRepo builds a fixture repo reproducing the 2026-07-18
// writer-matrix audit classes: contracted dirs (skill-referenced,
// cli-referenced, alias-referenced), an active orphan, a dead orphan, an
// empty orphan, a dir referenced only from a _test.go file (which must not
// count), exempt dirs (ao, stale, retry-chain, dot-leading), and drift
// references in a SKILL.md and a non-test .go file.
func refsFixtureRepo(t *testing.T) (env *DetectEnv, repo string) {
	t.Helper()
	// Pin the GC TTL to the 14-day default regardless of the outer env.
	t.Setenv(workspaceTTLEnvVar, "")
	skillAlias, skillCanonical, cliAlias, _ := refsTestAliases(t)
	repo = t.TempDir()
	agents := filepath.Join(repo, ".agents")

	// Source corpus. The SKILL.md references a contracted dir and one alias
	// (twice, one occurrence sentence-terminated to exercise dot-trimming).
	writeRefsFile(t, filepath.Join(repo, "skills", "demo", "SKILL.md"),
		"# demo\n"+
			"Writes .agents/contracted/ for evidence.\n"+
			"Legacy path: .agents/"+skillAlias+"/log.md\n"+
			"See also .agents/"+skillAlias+".\n")
	writeRefsFile(t, filepath.Join(repo, "cli", "cmd", "tool.go"),
		"package cmd\n"+
			"// writes .agents/cli-owned/receipts\n"+
			"const legacyDir = \".agents/"+cliAlias+"\"\n"+
			"// canonical sibling must not re-match the shorter alias: .agents/"+cliAlias+"pective\n"+
			"const legacyAgain = \".agents/"+cliAlias+"/x.md\"\n")
	// A _test.go reference must NOT count as a writer contract.
	writeRefsFile(t, filepath.Join(repo, "cli", "cmd", "tool_test.go"),
		"package cmd\n// test-only: .agents/testonly\n")
	// The alias registry's defining file is exempt from drift findings even
	// when it contains an alias reference.
	writeRefsFile(t, filepath.Join(repo, "cli", "internal", "doctor", "fix_workspace.go"),
		"package doctor\n// registry doc: .agents/"+skillAlias+" -> .agents/"+skillCanonical+"\n")
	// testdata is never walked.
	writeRefsFile(t, filepath.Join(repo, "cli", "testdata", "fixture.go"),
		"package fixture\n// .agents/"+cliAlias+"\n")

	// Live workspace dirs.
	writeRefsFile(t, filepath.Join(agents, "contracted", "a.md"), "contracted")
	writeRefsFile(t, filepath.Join(agents, "cli-owned", "b.md"), "cli-owned")
	// Referenced only via the alias spelling: canonicalization keeps it contracted.
	writeRefsFile(t, filepath.Join(agents, skillCanonical, "c.md"), "family")
	writeRefsFile(t, filepath.Join(agents, "active-orphan", "note.md"), "fresh")
	writeRefsFile(t, filepath.Join(agents, "dead-orphan", "old.md"), "stale")
	old := time.Now().Add(-30 * day)
	if err := os.Chtimes(filepath.Join(agents, "dead-orphan", "old.md"), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	for _, dir := range []string{"empty-orphan", "ao", "junk.stale-20260101T000000Z", "job-retry2", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(agents, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writeRefsFile(t, filepath.Join(agents, "testonly", "t.md"), "test-only ref")

	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

func TestWorkspaceUndeclaredWriter_FixtureMatrix(t *testing.T) {
	env, _ := refsFixtureRepo(t)
	findings, err := workspaceUndeclaredWriterDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// Exactly the four orphans, in deterministic name-sorted order, each with
	// its audit classification in the title.
	wantTitles := []string{
		"workspace undeclared writer: .agents/active-orphan (ACTIVE-ORPHAN)",
		"workspace undeclared writer: .agents/dead-orphan (DEAD-ORPHAN)",
		"workspace undeclared writer: .agents/empty-orphan (DEAD-ORPHAN)",
		"workspace undeclared writer: .agents/testonly (ACTIVE-ORPHAN)",
	}
	if len(findings) != len(wantTitles) {
		t.Fatalf("findings = %d, want %d: %+v", len(findings), len(wantTitles), findings)
	}
	for i, f := range findings {
		if f.Title != wantTitles[i] {
			t.Errorf("finding %d title = %q, want %q", i, f.Title, wantTitles[i])
		}
		if f.ID != fmWorkspaceUndeclaredWriterID || f.Subsystem != "workspace" || f.Severity != "P3" {
			t.Errorf("finding %d contract: id=%q subsystem=%q severity=%q", i, f.ID, f.Subsystem, f.Severity)
		}
		if f.Remediation.AutoFixable {
			t.Errorf("finding %d marked auto-fixable; detector is report-only", i)
		}
	}
	if findings[0].Evidence.File != filepath.Join(".agents", "active-orphan") {
		t.Errorf("finding 0 evidence file = %q", findings[0].Evidence.File)
	}
	if want := "1 transitive regular file(s)"; !strings.Contains(findings[0].Evidence.Query, want) {
		t.Errorf("finding 0 query = %q, want it to contain %q", findings[0].Evidence.Query, want)
	}
	if want := "newest write 30d ago"; !strings.Contains(findings[1].Evidence.Query, want) {
		t.Errorf("dead-orphan query = %q, want it to contain %q", findings[1].Evidence.Query, want)
	}
	if want := "no regular files"; !strings.Contains(findings[2].Evidence.Query, want) {
		t.Errorf("empty-orphan query = %q, want it to contain %q", findings[2].Evidence.Query, want)
	}
}

func TestWorkspaceDriftRef_FixtureMatrix(t *testing.T) {
	env, _ := refsFixtureRepo(t)
	skillAlias, skillCanonical, cliAlias, cliCanonical := refsTestAliases(t)
	findings, err := workspaceDriftRefDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// One finding per (file, alias) pair, file-then-alias sorted. The exempt
	// registry file and the testdata fixture must produce none.
	wantTitles := []string{
		fmt.Sprintf("workspace drift reference: cli/cmd/tool.go references .agents/%s (canonical: .agents/%s)", cliAlias, cliCanonical),
		fmt.Sprintf("workspace drift reference: skills/demo/SKILL.md references .agents/%s (canonical: .agents/%s)", skillAlias, skillCanonical),
	}
	if len(findings) != len(wantTitles) {
		t.Fatalf("findings = %d, want %d: %+v", len(findings), len(wantTitles), findings)
	}
	for i, f := range findings {
		if f.Title != wantTitles[i] {
			t.Errorf("finding %d title = %q, want %q", i, f.Title, wantTitles[i])
		}
		if f.ID != fmWorkspaceDriftRefID || f.Subsystem != "workspace" || f.Severity != "P2" {
			t.Errorf("finding %d contract: id=%q subsystem=%q severity=%q", i, f.ID, f.Subsystem, f.Severity)
		}
		if f.Remediation.AutoFixable {
			t.Errorf("finding %d marked auto-fixable; detector is report-only", i)
		}
	}
	// tool.go: alias on lines 3 and 5; the longer segment on line 4 must not
	// re-match the shorter alias.
	if got, want := findings[0].Evidence.Lines, []int{3, 5}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("cli finding lines = %v, want %v", got, want)
	}
	if want := fmt.Sprintf("2 occurrence(s) of .agents/%s", cliAlias); !strings.Contains(findings[0].Evidence.Query, want) {
		t.Errorf("cli finding query = %q, want it to contain %q", findings[0].Evidence.Query, want)
	}
	// SKILL.md: lines 3 and 4 — the sentence-terminated occurrence
	// (".agents/<alias>.") must still count via dot-trimming.
	if got, want := findings[1].Evidence.Lines, []int{3, 4}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("skill finding lines = %v, want %v", got, want)
	}
}

// The canonical spelling itself must never be flagged as a drift reference,
// and an alias that is a strict prefix of a canonical name must not match
// inside the longer segment.
func TestWorkspaceDriftRef_CanonicalAndPrefixSafe(t *testing.T) {
	skillAlias, skillCanonical, _, _ := refsTestAliases(t)
	repo := t.TempDir()
	writeRefsFile(t, filepath.Join(repo, "cli", "clean.go"),
		"package clean\n// canonical: .agents/"+skillCanonical+"\n// unrelated: .agents/"+skillAlias+"-archive\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceDriftRefDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("canonical/longer-segment refs produced %d findings: %+v", len(findings), findings)
	}
}

// An oversized corpus file is skipped entirely: its alias reference neither
// produces a drift finding nor contracts a directory.
func TestWorkspaceRefs_OversizeFileSkipped(t *testing.T) {
	t.Setenv(workspaceTTLEnvVar, "")
	_, _, cliAlias, _ := refsTestAliases(t)
	repo := t.TempDir()
	big := "// .agents/" + cliAlias + "\n// .agents/bigref\n" + strings.Repeat("x", workspaceRefMaxFileBytes)
	writeRefsFile(t, filepath.Join(repo, "cli", "big.go"), big)
	writeRefsFile(t, filepath.Join(repo, ".agents", "bigref", "f.md"), "fresh")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}

	drift, err := workspaceDriftRefDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("drift Detect: %v", err)
	}
	if len(drift) != 0 {
		t.Fatalf("oversize file produced %d drift findings: %+v", len(drift), drift)
	}
	orphans, err := workspaceUndeclaredWriterDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("orphan Detect: %v", err)
	}
	if len(orphans) != 1 || !strings.Contains(orphans[0].Title, ".agents/bigref (ACTIVE-ORPHAN)") {
		t.Fatalf("orphan findings = %+v, want exactly the unreferenced bigref dir", orphans)
	}
}

// With no source corpus at all (no skills/ and no cli/ — or only symlinked
// roots, which are never walked), both detectors stay silent: there is no
// writer-contract surface to join against.
func TestWorkspaceRefs_NoCorpusInert(t *testing.T) {
	t.Setenv(workspaceTTLEnvVar, "")
	_, _, cliAlias, _ := refsTestAliases(t)
	repo := t.TempDir()
	writeRefsFile(t, filepath.Join(repo, ".agents", "orphan", "o.md"), "unreferenced")
	// A symlinked corpus root must count as absent, not walked.
	outside := t.TempDir()
	writeRefsFile(t, filepath.Join(outside, "x.go"), "package x\n// .agents/"+cliAlias+"\n")
	if err := os.Symlink(outside, filepath.Join(repo, "cli")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}

	orphans, err := workspaceUndeclaredWriterDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("orphan Detect: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("no-corpus repo produced %d orphan findings: %+v", len(orphans), orphans)
	}
	drift, err := workspaceDriftRefDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("drift Detect: %v", err)
	}
	if len(drift) != 0 {
		t.Fatalf("no-corpus repo produced %d drift findings: %+v", len(drift), drift)
	}
}

// fm-ws-drift-ref needs no .agents tree: the drifted source reference is what
// recreates the drift, so it is flagged even before any directory exists.
// fm-ws-undeclared-writer, by contrast, is silent without a workspace root.
func TestWorkspaceRefs_NoAgentsDir(t *testing.T) {
	_, _, cliAlias, cliCanonical := refsTestAliases(t)
	repo := t.TempDir()
	writeRefsFile(t, filepath.Join(repo, "cli", "w.go"), "package w\n// .agents/"+cliAlias+"\n")
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}

	drift, err := workspaceDriftRefDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("drift Detect: %v", err)
	}
	if len(drift) != 1 {
		t.Fatalf("drift findings = %d, want 1: %+v", len(drift), drift)
	}
	want := fmt.Sprintf("workspace drift reference: cli/w.go references .agents/%s (canonical: .agents/%s)", cliAlias, cliCanonical)
	if drift[0].Title != want {
		t.Errorf("drift title = %q, want %q", drift[0].Title, want)
	}
	orphans, err := workspaceUndeclaredWriterDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("orphan Detect: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("missing .agents produced %d orphan findings: %+v", len(orphans), orphans)
	}
}

func TestWorkspaceDriftRefExemptFile_Rows(t *testing.T) {
	cases := []struct {
		rel    string
		exempt bool
	}{
		{"cli/internal/doctor/fix_workspace.go", true}, // alias registry definer
		{"docs/agents-dir-hygiene.md", true},           // documents the aliases
		{"docs/adr/0016-state-tiers.md", true},         // immutable decision records
		{".agents/audits/matrix.md", true},             // workspace content, not source
		{"cli/testdata/fixture.go", true},              // fixture corpus
		{"skills/demo/archive/old.md", true},           // archived path element
		{"skills/_fixtures/pkg/SKILL.md", true},        // fixture corpus
		{"cli/internal/doctor/fix_workspace_drift.go", false},
		{"cli/cmd/tool.go", false},
		{"skills/demo/SKILL.md", false},
		{"docs/agents-dir-hygiene.md.bak", false}, // exact match only
	}
	for _, tc := range cases {
		if got := workspaceDriftRefExemptFile(tc.rel); got != tc.exempt {
			t.Errorf("workspaceDriftRefExemptFile(%q) = %t, want %t", tc.rel, got, tc.exempt)
		}
	}
}

// refsTreeDigest hashes every path, file size, and content under root so a
// before/after comparison proves the detectors mutated nothing.
func refsTreeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	entries := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		line := rel
		if d.Type().IsRegular() {
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			line += fmt.Sprintf(":%d:%x", len(b), sha256.Sum256(b))
		}
		paths = append(paths, rel)
		entries[rel] = line
		return nil
	})
	if err != nil {
		t.Fatalf("digest walk: %v", err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintln(h, entries[p])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Report-only contract: neither ID has a registered fixer, and a detect pass
// leaves the repository tree byte-identical.
func TestWorkspaceRefs_ReportOnlyNoMutation(t *testing.T) {
	env, repo := refsFixtureRepo(t)
	for _, id := range []string{fmWorkspaceUndeclaredWriterID, fmWorkspaceDriftRefID} {
		if fx := FixerByID(id); fx != nil {
			t.Errorf("FixerByID(%q) = %T, want nil (report-only)", id, fx)
		}
	}
	before := refsTreeDigest(t, repo)
	if _, err := (workspaceUndeclaredWriterDetector{}).Detect(env); err != nil {
		t.Fatalf("orphan Detect: %v", err)
	}
	if _, err := (workspaceDriftRefDetector{}).Detect(env); err != nil {
		t.Fatalf("drift Detect: %v", err)
	}
	if after := refsTreeDigest(t, repo); after != before {
		t.Error("detect pass mutated the repository tree")
	}
}

func TestWorkspaceRefs_Registered(t *testing.T) {
	byID := map[string]Detector{}
	for _, d := range Detectors() {
		byID[d.ID()] = d
	}
	cases := []struct {
		id       string
		severity string
	}{
		{fmWorkspaceUndeclaredWriterID, "P3"},
		{fmWorkspaceDriftRefID, "P2"},
	}
	for _, tc := range cases {
		det, ok := byID[tc.id]
		if !ok {
			t.Fatalf("detector %s not registered", tc.id)
		}
		if det.Subsystem() != "workspace" || det.Severity() != tc.severity {
			t.Errorf("%s contract: subsystem=%q severity=%q, want workspace/%s",
				tc.id, det.Subsystem(), det.Severity(), tc.severity)
		}
		if det.QuickPath() {
			t.Errorf("%s is QuickPath; the corpus walk is not a <200ms fast-path detector", tc.id)
		}
		if det.OnlineRequired() {
			t.Errorf("%s claims OnlineRequired", tc.id)
		}
		if det.EstimatedCostMS() < 100 {
			t.Errorf("%s EstimatedCostMS = %d; a full corpus walk under 100ms is not honest", tc.id, det.EstimatedCostMS())
		}
		if FixerByID(tc.id) != nil {
			t.Errorf("%s has a registered fixer, want report-only", tc.id)
		}
	}
}

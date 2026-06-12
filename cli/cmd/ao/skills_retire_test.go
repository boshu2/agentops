package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixture yaml mirrors the REAL docs/contracts/skill-dispositions.yaml shape
// (fixture-fidelity rule): load-bearing comments, a `historical:` mapping ABOVE
// the `dispositions:` key, and flat aligned `- skill:` rows.
const retireFixtureYAMLHeader = `# Skill dispositions — canonical per-skill DDD/disposition data
#
# Source of truth for the Full Skill Map. Do not hand-edit the generated
# table — edit this yaml and regenerate.
#
# Schema per row:
#   skill           skill slug (matches skills/<slug>/SKILL.md name)
#   domain          "BC<n> <Name>" — must match docs/contracts/bounded-contexts.yaml
#   hexagonal_role  domain | driving-adapter | driven-adapter | supporting | generic
#   disposition     keep | update | refactor | merge-review | cut-review
#   rationale       one-line justification for the disposition

# Historical terminal-state rows — skills whose dirs are ALREADY ABSENT from
# the repo when their ledger row was written. Rows are NEVER removed.
#
# Schema per row:
#   state        terminal state: merged-into | cut
#   merged-into  target skill slug (when state is merged-into)
#   date         date the terminal state was reached
#   rationale    one-line historical justification
historical:
  old-skill:
    state:        merged-into
    merged-into:  validate
    date:         2026-06-07
    rationale:    "Already absent from repo; historical row added with no removal precondition."
`

const retireFixtureYAMLAlphaRow = `  - skill:          alpha
    domain:         "BC4 Factory"
    hexagonal_role: supporting
    disposition:    merge-review
    rationale:      "Fold into beta"
`

const retireFixtureYAMLBetaRow = `  - skill:          beta
    domain:         "BC4 Factory"
    hexagonal_role: supporting
    disposition:    keep
    rationale:      "Target skill"
`

func retireFixtureYAML() string {
	return retireFixtureYAMLHeader + "\ndispositions:\n" + retireFixtureYAMLAlphaRow + retireFixtureYAMLBetaRow
}

const retireFixtureCritical = `# Critical skills require human-supervised edits.
#
# ao skills edit seal refuses these slugs unless the caller passes
# --allow-critical.

evolve
rpi
`

func retireHistoricalEntry(slug, state, target, date, rationale string) string {
	entry := "  " + slug + ":\n" +
		"    state:        " + state + "\n"
	if target != "" {
		entry += "    merged-into:  " + target + "\n"
	}
	entry += "    date:         " + date + "\n" +
		"    rationale:    \"" + rationale + "\"\n"
	return entry
}

func setupSkillsRetireRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeSkillEditFile(t, repo, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: alpha skill\nconsumes: []\ncontext_rel: []\n---\nalpha body\n")
	writeSkillEditFile(t, repo, "skills/alpha/references/extra.md", "alpha extra\n")
	writeSkillEditFile(t, repo, "skills/beta/SKILL.md", "---\nname: beta\ndescription: beta skill\nconsumes: []\ncontext_rel: []\n---\nbeta body\n")
	writeSkillEditFile(t, repo, "skills-codex/alpha/SKILL.md", "---\nname: alpha\n---\nalpha codex twin\n")
	writeSkillEditFile(t, repo, "skills-codex/beta/SKILL.md", "---\nname: beta\n---\nbeta codex twin\n")
	writeSkillEditFile(t, repo, "docs/contracts/skill-dispositions.yaml", retireFixtureYAML())
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", retireFixtureCritical)
	writeSkillEditFile(t, repo, "docs/SKILLS.md", "# Skills\n\n- beta — the target skill\n")
	return repo
}

func readRetireFile(t *testing.T, repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func retireDirExists(repo, rel string) bool {
	info, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel)))
	return err == nil && info.IsDir()
}

func stubRetireRegen(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	orig := skillsRetireRunScript
	skillsRetireRunScript = func(repoRoot, script string) error {
		calls = append(calls, script)
		return nil
	}
	t.Cleanup(func() { skillsRetireRunScript = orig })
	return &calls
}

// Scenario 1: retire alpha --into beta removes both trees, moves the ledger
// row to historical with merged-into + date, and leaves the rest of the yaml
// byte-identical outside the moved row.
func TestSkillsRetireIntoRemovesTreesAndFlipsLedger(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	calls := stubRetireRegen(t)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen")
	if err != nil {
		t.Fatalf("ao skills retire: %v\n%s", err, out)
	}
	if retireDirExists(repo, "skills/alpha") {
		t.Fatal("expected skills/alpha to be removed")
	}
	if retireDirExists(repo, "skills-codex/alpha") {
		t.Fatal("expected skills-codex/alpha to be removed")
	}
	if !retireDirExists(repo, "skills/beta") || !retireDirExists(repo, "skills-codex/beta") {
		t.Fatal("expected target beta trees to be untouched")
	}
	if len(*calls) != 0 {
		t.Fatalf("expected --no-regen to skip regen scripts, got %v", *calls)
	}

	today := time.Now().Format("2006-01-02")
	want := retireFixtureYAMLHeader +
		retireHistoricalEntry("alpha", "merged-into", "beta", today,
			"Retired via ao skills retire: merged into beta.") +
		"\ndispositions:\n" + retireFixtureYAMLBetaRow
	got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")
	if got != want {
		t.Fatalf("ledger flip not byte-exact.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if !strings.Contains(out, "skills/alpha") || !strings.Contains(out, "skills-codex/alpha") {
		t.Fatalf("expected report to name removed trees, got:\n%s", out)
	}
}

// Scenario 2: --dry-run reports every planned operation and mutates nothing.
func TestSkillsRetireDryRunMutatesNothing(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	calls := stubRetireRegen(t)
	yamlBefore := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--dry-run")
	if err != nil {
		t.Fatalf("ao skills retire --dry-run: %v\n%s", err, out)
	}
	if !retireDirExists(repo, "skills/alpha") || !retireDirExists(repo, "skills-codex/alpha") {
		t.Fatal("dry-run must not remove any tree")
	}
	if got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml"); got != yamlBefore {
		t.Fatalf("dry-run must not touch the ledger.\n--- got ---\n%s", got)
	}
	if got := readRetireFile(t, repo, "docs/contracts/critical-skills.txt"); got != retireFixtureCritical {
		t.Fatalf("dry-run must not touch critical-skills.txt, got:\n%s", got)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run must not run regen scripts, got %v", *calls)
	}
	for _, planned := range []string{
		"DRY-RUN",
		"skills/alpha",
		"skills-codex/alpha",
		"skill-dispositions.yaml",
		"merged-into: beta",
		"sync-skill-counts.sh",
		"generate-skill-domain-map.sh",
		"generate-registry.sh",
		"generate-context-map.sh",
		"regen-codex-hashes.sh",
	} {
		if !strings.Contains(out, planned) {
			t.Fatalf("dry-run report missing %q:\n%s", planned, out)
		}
	}
}

// Scenario 3: a critical slug without --allow-critical refuses and changes nothing.
func TestSkillsRetireRefusesCriticalWithoutOverride(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", retireFixtureCritical+"alpha\n")
	yamlBefore := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen")
	if err == nil || !strings.Contains(err.Error(), "critical") {
		t.Fatalf("expected critical-skill refusal, got err=%v out=%s", err, out)
	}
	if !retireDirExists(repo, "skills/alpha") {
		t.Fatal("refusal must not remove skills/alpha")
	}
	if got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml"); got != yamlBefore {
		t.Fatal("refusal must not touch the ledger")
	}
}

// Critical slug WITH --allow-critical proceeds and drops the critical-skills.txt line.
func TestSkillsRetireAllowCriticalRemovesCriticalLine(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "docs/contracts/critical-skills.txt", retireFixtureCritical+"alpha\n")

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen", "--allow-critical")
	if err != nil {
		t.Fatalf("ao skills retire --allow-critical: %v\n%s", err, out)
	}
	got := readRetireFile(t, repo, "docs/contracts/critical-skills.txt")
	if got != retireFixtureCritical {
		t.Fatalf("expected only the alpha line removed from critical-skills.txt, got:\n%s", got)
	}
}

// Scenario 4: --into pointing at a skill dir that does not exist refuses
// before any mutation.
func TestSkillsRetireRefusesMissingIntoTarget(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	yamlBefore := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")

	out, err := executeCommand("skills", "retire", "alpha", "--into", "nonexistent", "--no-regen")
	if err == nil || !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected missing --into target refusal, got err=%v out=%s", err, out)
	}
	if !retireDirExists(repo, "skills/alpha") || !retireDirExists(repo, "skills-codex/alpha") {
		t.Fatal("refusal must not remove any tree")
	}
	if got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml"); got != yamlBefore {
		t.Fatal("refusal must not touch the ledger")
	}
}

// Scenario 5: another skill holding a context_rel edge to the slug is reported
// (file + edge) and the command exits non-zero.
func TestSkillsRetireRippleReportsFrontmatterEdge(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "skills/gamma/SKILL.md",
		"---\nname: gamma\nconsumes: []\ncontext_rel:\n- kind: supplier-to\n  with: alpha\n---\ngamma body\n")

	out, err := executeCommand("skills", "retire", "alpha", "--no-regen")
	if err == nil {
		t.Fatalf("expected non-zero exit on unresolved refs, got success:\n%s", out)
	}
	if !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("expected unresolved-refs error, got: %v", err)
	}
	if !strings.Contains(out, filepath.ToSlash(filepath.Join("skills", "gamma", "SKILL.md"))) {
		t.Fatalf("expected ripple report to name gamma's SKILL.md:\n%s", out)
	}
	if !strings.Contains(out, "with: alpha") {
		t.Fatalf("expected ripple report to show the context_rel edge:\n%s", out)
	}
	// Mutations still land (gate reports, operator resolves): trees gone, ledger flipped to cut.
	if retireDirExists(repo, "skills/alpha") {
		t.Fatal("expected skills/alpha removed even with unresolved refs")
	}
	yaml := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")
	if !strings.Contains(yaml, "state:        cut") {
		t.Fatalf("expected cut historical row without --into, got:\n%s", yaml)
	}
}

// Scenario 6: --json emits a machine-readable report.
func TestSkillsRetireJSONReport(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	writeSkillEditFile(t, repo, "skills/gamma/SKILL.md",
		"---\nname: gamma\nconsumes:\n- alpha\ncontext_rel: []\n---\ngamma body\n")

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen", "--json")
	if err == nil {
		t.Fatalf("expected non-zero exit on unresolved refs, got success:\n%s", out)
	}
	var report struct {
		Slug         string   `json:"slug"`
		Into         string   `json:"into"`
		DryRun       bool     `json:"dry_run"`
		Operations   []string `json:"operations"`
		RemovedPaths []string `json:"removed_paths"`
		Ledger       struct {
			DispositionsRowRemoved bool   `json:"dispositions_row_removed"`
			HistoricalState        string `json:"historical_state"`
			HistoricalDate         string `json:"historical_date"`
			CriticalLineRemoved    bool   `json:"critical_line_removed"`
		} `json:"ledger"`
		Regen          []string `json:"regen"`
		UnresolvedRefs []struct {
			Scan  string `json:"scan"`
			File  string `json:"file"`
			Line  int    `json:"line"`
			Match string `json:"match"`
		} `json:"unresolved_refs"`
	}
	// The captured output may carry cobra's trailing "Error: ..." line after
	// the JSON document; decode just the first JSON value.
	if jsonErr := json.NewDecoder(strings.NewReader(out)).Decode(&report); jsonErr != nil {
		t.Fatalf("expected machine-readable JSON, got %v:\n%s", jsonErr, out)
	}
	if report.Slug != "alpha" || report.Into != "beta" || report.DryRun {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if len(report.RemovedPaths) != 2 ||
		report.RemovedPaths[0] != "skills/alpha" || report.RemovedPaths[1] != "skills-codex/alpha" {
		t.Fatalf("unexpected removed_paths: %v", report.RemovedPaths)
	}
	if !report.Ledger.DispositionsRowRemoved || report.Ledger.HistoricalState != "merged-into" {
		t.Fatalf("unexpected ledger report: %+v", report.Ledger)
	}
	if report.Ledger.HistoricalDate != time.Now().Format("2006-01-02") {
		t.Fatalf("unexpected historical date: %q", report.Ledger.HistoricalDate)
	}
	if len(report.Regen) != 0 {
		t.Fatalf("expected empty regen list with --no-regen, got %v", report.Regen)
	}
	if len(report.UnresolvedRefs) != 1 {
		t.Fatalf("expected exactly one unresolved ref, got %+v", report.UnresolvedRefs)
	}
	ref := report.UnresolvedRefs[0]
	if ref.Scan != "skill-frontmatter" || ref.File != "skills/gamma/SKILL.md" || ref.Line != 4 {
		t.Fatalf("unexpected unresolved ref: %+v", ref)
	}
	if len(report.Operations) == 0 {
		t.Fatalf("expected operations list, got %+v", report)
	}
}

// Scenario 7: phantom slug (ledger row but no dir) gets a clear error.
func TestSkillsRetirePhantomSlugError(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)

	out, err := executeCommand("skills", "retire", "ghost", "--no-regen")
	if err == nil {
		t.Fatalf("expected phantom-slug error, got success:\n%s", out)
	}
	if !strings.Contains(err.Error(), "ledger-only") || !strings.Contains(err.Error(), "skills/ghost") {
		t.Fatalf("expected ledger-only phantom error naming skills/ghost, got: %v", err)
	}
}

// BC5 embedded carve-out: using-agentops is out of scope for this command.
func TestSkillsRetireRefusesUsingAgentops(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)

	_, err := executeCommand("skills", "retire", "using-agentops", "--no-regen")
	if err == nil || !strings.Contains(err.Error(), "using-agentops") {
		t.Fatalf("expected using-agentops carve-out refusal, got: %v", err)
	}
}

// Scenario 8a: with regen enabled the five scripts run in order.
func TestSkillsRetireRegenInvokesScriptsInOrder(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	calls := stubRetireRegen(t)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta")
	if err != nil {
		t.Fatalf("ao skills retire with regen: %v\n%s", err, out)
	}
	want := []string{
		"sync-skill-counts.sh",
		"generate-skill-domain-map.sh",
		"generate-registry.sh",
		"generate-context-map.sh",
		"regen-codex-hashes.sh",
	}
	if len(*calls) != len(want) {
		t.Fatalf("expected %d regen invocations, got %v", len(want), *calls)
	}
	for i, script := range want {
		if (*calls)[i] != script {
			t.Fatalf("regen order mismatch at %d: got %v, want %v", i, *calls, want)
		}
	}
}

// Scenario 8b: --no-regen invokes none of the scripts.
func TestSkillsRetireNoRegenSkipsScripts(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	calls := stubRetireRegen(t)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen")
	if err != nil {
		t.Fatalf("ao skills retire --no-regen: %v\n%s", err, out)
	}
	if len(*calls) != 0 {
		t.Fatalf("expected no regen invocations with --no-regen, got %v", *calls)
	}
}

// ag-s3ja7 finding 1: an indented comment line directly after the retired row
// is NOT part of the row block — it must survive byte-identically.
func TestSkillsRetirePreservesCommentAfterRow(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	commentLine := "  # COMMENT-AFTER-ROW: load-bearing note about the beta row below\n"
	writeSkillEditFile(t, repo, "docs/contracts/skill-dispositions.yaml",
		retireFixtureYAMLHeader+"\ndispositions:\n"+retireFixtureYAMLAlphaRow+commentLine+retireFixtureYAMLBetaRow)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen")
	if err != nil {
		t.Fatalf("ao skills retire: %v\n%s", err, out)
	}
	today := time.Now().Format("2006-01-02")
	want := retireFixtureYAMLHeader +
		retireHistoricalEntry("alpha", "merged-into", "beta", today,
			"Retired via ao skills retire: merged into beta.") +
		"\ndispositions:\n" + commentLine + retireFixtureYAMLBetaRow
	got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")
	if !strings.Contains(got, "# COMMENT-AFTER-ROW") {
		t.Fatalf("indented comment after the retired row was eaten:\n%s", got)
	}
	if got != want {
		t.Fatalf("ledger flip not byte-exact around the comment.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// ag-s3ja7 finding 2: --into gets the same slug-format validation as the
// retire slug, plus a self-merge refusal — all BEFORE any mutation.
func TestSkillsRetireRefusesInvalidIntoTarget(t *testing.T) {
	cases := []struct {
		name string
		into string
	}{
		{"path-traversal", "../scripts"},
		{"nested-path", "zz/sub"},
		{"self-merge", "alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := setupSkillsRetireRepo(t)
			t.Chdir(repo)
			// Make the bad targets resolve to real dirs so only the new
			// validation (not isDir) can refuse them.
			writeSkillEditFile(t, repo, "scripts/placeholder.sh", "#!/bin/bash\n")
			writeSkillEditFile(t, repo, "skills/zz/sub/SKILL.md", "---\nname: sub\n---\nbody\n")
			yamlBefore := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")

			out, err := executeCommand("skills", "retire", "alpha", "--into", tc.into, "--no-regen")
			if err == nil {
				t.Fatalf("expected --into %q refusal, got success:\n%s", tc.into, out)
			}
			if !strings.Contains(err.Error(), "--into") {
				t.Fatalf("expected refusal to name --into, got: %v", err)
			}
			if !retireDirExists(repo, "skills/alpha") || !retireDirExists(repo, "skills-codex/alpha") {
				t.Fatalf("--into %q refusal must not remove any tree", tc.into)
			}
			if got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml"); got != yamlBefore {
				t.Fatalf("--into %q refusal must not touch the ledger", tc.into)
			}
		})
	}
}

// ag-s3ja7 finding 3: retiring the LAST dispositions row must preserve the
// file's trailing newline.
func TestSkillsRetireLastRowKeepsTrailingNewline(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	// alpha is the last row in the file; the file ends with exactly one \n.
	writeSkillEditFile(t, repo, "docs/contracts/skill-dispositions.yaml",
		retireFixtureYAMLHeader+"\ndispositions:\n"+retireFixtureYAMLBetaRow+retireFixtureYAMLAlphaRow)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--no-regen")
	if err != nil {
		t.Fatalf("ao skills retire: %v\n%s", err, out)
	}
	got := readRetireFile(t, repo, "docs/contracts/skill-dispositions.yaml")
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("trailing newline dropped on last-row retire:\n%q", got[len(got)-40:])
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Fatalf("expected exactly one trailing newline, got more:\n%q", got[len(got)-40:])
	}
	today := time.Now().Format("2006-01-02")
	want := retireFixtureYAMLHeader +
		retireHistoricalEntry("alpha", "merged-into", "beta", today,
			"Retired via ao skills retire: merged into beta.") +
		"\ndispositions:\n" + retireFixtureYAMLBetaRow
	if got != want {
		t.Fatalf("last-row retire not byte-exact.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// stubRetireRegenFailAt makes the regen runner fail on the nth script (1-based).
func stubRetireRegenFailAt(t *testing.T, n int) *[]string {
	t.Helper()
	var calls []string
	orig := skillsRetireRunScript
	skillsRetireRunScript = func(repoRoot, script string) error {
		calls = append(calls, script)
		if len(calls) == n {
			return fmt.Errorf("regen %s: exit status 1", script)
		}
		return nil
	}
	t.Cleanup(func() { skillsRetireRunScript = orig })
	return &calls
}

// ag-s3ja7 finding 4: a regen failure after trees+ledger mutated must still
// emit the operations report (text) alongside the error.
func TestSkillsRetireRegenFailureStillEmitsReport(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	calls := stubRetireRegenFailAt(t, 3)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta")
	if err == nil {
		t.Fatalf("expected regen failure error, got success:\n%s", out)
	}
	if !strings.Contains(err.Error(), "generate-registry.sh") {
		t.Fatalf("expected error naming the failed script, got: %v", err)
	}
	if len(*calls) != 3 {
		t.Fatalf("expected regen to stop at the failing script, got %v", *calls)
	}
	// The mutations already landed — the operator must see them.
	for _, op := range []string{
		"removed skills/alpha",
		"removed skills-codex/alpha",
		"skill-dispositions.yaml",
		"ran scripts/sync-skill-counts.sh",
		"ran scripts/generate-skill-domain-map.sh",
	} {
		if !strings.Contains(out, op) {
			t.Fatalf("regen-failure report missing %q:\n%s", op, out)
		}
	}
}

// ag-s3ja7 finding 4 (--json): the partial report is emitted as JSON too.
func TestSkillsRetireRegenFailureStillEmitsJSONReport(t *testing.T) {
	repo := setupSkillsRetireRepo(t)
	t.Chdir(repo)
	stubRetireRegenFailAt(t, 3)

	out, err := executeCommand("skills", "retire", "alpha", "--into", "beta", "--json")
	if err == nil {
		t.Fatalf("expected regen failure error, got success:\n%s", out)
	}
	var report struct {
		Slug       string   `json:"slug"`
		Operations []string `json:"operations"`
		Regen      []string `json:"regen"`
	}
	if jsonErr := json.NewDecoder(strings.NewReader(out)).Decode(&report); jsonErr != nil {
		t.Fatalf("expected JSON report alongside regen failure, got %v:\n%s", jsonErr, out)
	}
	if report.Slug != "alpha" {
		t.Fatalf("unexpected report slug: %+v", report)
	}
	if len(report.Operations) == 0 {
		t.Fatalf("expected operations in the partial report, got %+v", report)
	}
	if len(report.Regen) != 2 {
		t.Fatalf("expected the two completed regen scripts recorded, got %v", report.Regen)
	}
}

// ag-s3ja7 finding 5: the Long help documents exit-code semantics and the
// --dry-run exits-0 behavior.
func TestSkillsRetireHelpDocumentsExitSemantics(t *testing.T) {
	out, err := executeCommand("skills", "retire", "--help")
	if err != nil {
		t.Fatalf("ao skills retire --help: %v\n%s", err, out)
	}
	for _, want := range []string{"Exit codes", "--dry-run", "unresolved"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help text missing %q:\n%s", want, out)
		}
	}
}

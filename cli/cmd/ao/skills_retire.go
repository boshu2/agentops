package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	skillsRetireInto          string
	skillsRetireDryRun        bool
	skillsRetireAllowCritical bool
	skillsRetireNoRegen       bool
	skillsRetireJSON          bool
)

// skillsRetireRegenScripts are the deterministic regen surfaces a retire
// invalidates, run in this exact order (counts -> domain map -> registry ->
// context map -> codex hashes).
var skillsRetireRegenScripts = []string{
	"sync-skill-counts.sh",
	"generate-skill-domain-map.sh",
	"generate-registry.sh",
	"generate-context-map.sh",
	"regen-codex-hashes.sh",
}

// skillsRetireRunScript is an injectable seam so tests can assert the regen
// invocation list without shelling out.
var skillsRetireRunScript = func(repoRoot, script string) error {
	cmd := exec.Command("bash", filepath.Join(repoRoot, "scripts", script))
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("regen %s: %w\n%s", script, err, strings.TrimSpace(string(out)))
	}
	return nil
}

type skillsRetireOptions struct {
	RepoRoot      string
	Slug          string
	Into          string
	DryRun        bool
	AllowCritical bool
	NoRegen       bool
}

type skillsRetireLedgerReport struct {
	DispositionsRowRemoved bool   `json:"dispositions_row_removed"`
	HistoricalState        string `json:"historical_state"`
	HistoricalDate         string `json:"historical_date"`
	CriticalLineRemoved    bool   `json:"critical_line_removed"`
}

type skillsRetireRef struct {
	Scan  string `json:"scan"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Match string `json:"match"`
}

type skillsRetireReport struct {
	Slug           string                   `json:"slug"`
	Into           string                   `json:"into,omitempty"`
	DryRun         bool                     `json:"dry_run"`
	Operations     []string                 `json:"operations"`
	RemovedPaths   []string                 `json:"removed_paths"`
	Ledger         skillsRetireLedgerReport `json:"ledger"`
	Regen          []string                 `json:"regen"`
	UnresolvedRefs []skillsRetireRef        `json:"unresolved_refs"`
}

var skillsRetireCmd = &cobra.Command{
	Use:   "retire <slug>",
	Short: "Retire a skill: remove its trees, flip the dispositions ledger, regen, and report ripples",
	Long: `One deterministic retire operation for a skill.

Five phases: validate, remove trees (skills/, skills-codex/,
skills-codex-overrides/, images/*/skills/), flip
docs/contracts/skill-dispositions.yaml (active row -> historical, non-lossy
text edit), run the regen scripts, then scan-and-report remaining references
(exit non-zero while any remain). No git commands run — the operator lands
the change; every mutation is git-recoverable.`,
	Args: cobra.ExactArgs(1),
	RunE: runSkillsRetire,
}

func init() {
	skillsCmd.AddCommand(skillsRetireCmd)
	skillsRetireCmd.Flags().StringVar(&skillsRetireInto, "into", "", "Target skill the retiree merges into (historical state merged-into; default: cut)")
	skillsRetireCmd.Flags().BoolVar(&skillsRetireDryRun, "dry-run", false, "Report every planned operation without mutating anything")
	skillsRetireCmd.Flags().BoolVar(&skillsRetireAllowCritical, "allow-critical", false, "Allow retiring a slug listed in docs/contracts/critical-skills.txt")
	skillsRetireCmd.Flags().BoolVar(&skillsRetireNoRegen, "no-regen", false, "Skip the regen scripts after the ledger flip")
	skillsRetireCmd.Flags().BoolVar(&skillsRetireJSON, "json", false, "Emit a machine-readable JSON report")
}

func runSkillsRetire(cmd *cobra.Command, args []string) error {
	repoRoot, err := resolveRepoRootForSkills()
	if err != nil {
		return err
	}
	opts := skillsRetireOptions{
		RepoRoot:      repoRoot,
		Slug:          strings.TrimSpace(args[0]),
		Into:          strings.TrimSpace(skillsRetireInto),
		DryRun:        skillsRetireDryRun,
		AllowCritical: skillsRetireAllowCritical,
		NoRegen:       skillsRetireNoRegen,
	}
	cmd.SilenceUsage = true
	report, err := retireSkill(opts)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if skillsRetireJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		printSkillsRetireReport(out, report)
	}
	if !report.DryRun && len(report.UnresolvedRefs) > 0 {
		return fmt.Errorf("%d unresolved reference(s) to %q remain; resolve them and re-run the scan", len(report.UnresolvedRefs), report.Slug)
	}
	return nil
}

// retireSkill executes the five retire phases. Unresolved references are
// reported, not returned as an error — the caller decides the exit semantics.
func retireSkill(opts skillsRetireOptions) (*skillsRetireReport, error) {
	critical, err := validateSkillsRetire(opts)
	if err != nil {
		return nil, err
	}
	report := &skillsRetireReport{
		Slug:           opts.Slug,
		Into:           opts.Into,
		DryRun:         opts.DryRun,
		Operations:     []string{},
		RemovedPaths:   []string{},
		Regen:          []string{},
		UnresolvedRefs: []skillsRetireRef{},
	}

	// Phase 2: remove trees (whichever exist).
	trees := skillsRetireTrees(opts.RepoRoot, opts.Slug)
	if err := retireRemoveTrees(opts, trees, report); err != nil {
		return nil, err
	}
	// Phase 3: ledger flip (non-lossy, text-targeted), ordered with the dir
	// removal in the same run, BEFORE regen — the domain-map validator
	// trip-wires on row-active-but-dir-missing.
	if err := retireFlipLedgers(opts, critical[opts.Slug], report); err != nil {
		return nil, err
	}
	// Phase 4: regen.
	if err := retireRunRegen(opts, report); err != nil {
		return nil, err
	}
	// Phase 5: ripple scan-and-report (read-only; runs in dry-run too).
	refs, err := scanSkillsRetireRipples(opts.RepoRoot, opts.Slug, trees)
	if err != nil {
		return nil, err
	}
	report.UnresolvedRefs = refs
	return report, nil
}

// validateSkillsRetire runs every phase-1 refusal before any mutation and
// returns the critical-skill set for the later ledger phase.
func validateSkillsRetire(opts skillsRetireOptions) (map[string]bool, error) {
	slug := opts.Slug
	if slug == "" {
		return nil, fmt.Errorf("skill slug is required")
	}
	if strings.Contains(slug, "/") || strings.Contains(slug, string(filepath.Separator)) || slug == "." || slug == ".." {
		return nil, fmt.Errorf("invalid skill slug %q", slug)
	}
	if slug == "using-agentops" {
		return nil, fmt.Errorf("refusing to retire %q: the BC5 embedded carve-out is out of scope for this command", slug)
	}
	if !isDir(filepath.Join(opts.RepoRoot, "skills", slug)) {
		return nil, fmt.Errorf("skill not found on disk: skills/%s — phantom/ledger-only slug; fix docs/contracts/skill-dispositions.yaml directly, there is nothing to retire", slug)
	}
	if opts.Into != "" && !isDir(filepath.Join(opts.RepoRoot, "skills", opts.Into)) {
		return nil, fmt.Errorf("--into target not found: skills/%s does not exist", opts.Into)
	}
	critical, err := loadCriticalSkills(opts.RepoRoot, "")
	if err != nil {
		return nil, err
	}
	if critical[slug] && !opts.AllowCritical {
		return nil, fmt.Errorf("critical skill %q refuses unattended retire; rerun with --allow-critical only for human-supervised retirement", slug)
	}
	return critical, nil
}

// retireRemoveTrees removes (or plans) every existing tree for the slug.
func retireRemoveTrees(opts skillsRetireOptions, trees []string, report *skillsRetireReport) error {
	for _, rel := range trees {
		if opts.DryRun {
			report.Operations = append(report.Operations, "would remove "+rel)
		} else {
			if err := os.RemoveAll(filepath.Join(opts.RepoRoot, filepath.FromSlash(rel))); err != nil {
				return fmt.Errorf("remove %s: %w", rel, err)
			}
			report.Operations = append(report.Operations, "removed "+rel)
		}
		report.RemovedPaths = append(report.RemovedPaths, rel)
	}
	return nil
}

// retireFlipLedgers flips skill-dispositions.yaml and, for critical slugs,
// drops the critical-skills.txt line.
func retireFlipLedgers(opts skillsRetireOptions, isCritical bool, report *skillsRetireReport) error {
	state := "cut"
	if opts.Into != "" {
		state = "merged-into"
	}
	date := time.Now().Format("2006-01-02")
	report.Ledger.HistoricalState = state
	report.Ledger.HistoricalDate = date
	dispositionsRel := "docs/contracts/skill-dispositions.yaml"
	dispositionsPath := filepath.Join(opts.RepoRoot, filepath.FromSlash(dispositionsRel))
	flipped, rowRemoved, err := flipDispositionsLedger(dispositionsPath, opts.Slug, opts.Into, state, date)
	if err != nil {
		return err
	}
	report.Ledger.DispositionsRowRemoved = rowRemoved
	ledgerOp := fmt.Sprintf("%s: dispositions row removed=%t; historical row appended (%s, %s)", dispositionsRel, rowRemoved, historicalStateLabel(state, opts.Into), date)
	if opts.DryRun {
		report.Operations = append(report.Operations, "would update "+ledgerOp)
	} else {
		if err := os.WriteFile(dispositionsPath, []byte(flipped), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dispositionsRel, err)
		}
		report.Operations = append(report.Operations, "updated "+ledgerOp)
	}
	if !isCritical {
		return nil
	}
	criticalRel := "docs/contracts/critical-skills.txt"
	criticalPath := filepath.Join(opts.RepoRoot, filepath.FromSlash(criticalRel))
	trimmed, removed, err := removeCriticalSkillLine(criticalPath, opts.Slug)
	if err != nil || !removed {
		return err
	}
	report.Ledger.CriticalLineRemoved = true
	if opts.DryRun {
		report.Operations = append(report.Operations, "would remove "+opts.Slug+" from "+criticalRel)
		return nil
	}
	if err := os.WriteFile(criticalPath, []byte(trimmed), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", criticalRel, err)
	}
	report.Operations = append(report.Operations, "removed "+opts.Slug+" from "+criticalRel)
	return nil
}

// retireRunRegen runs (or plans) the regen scripts in their fixed order.
func retireRunRegen(opts skillsRetireOptions, report *skillsRetireReport) error {
	if opts.NoRegen {
		report.Operations = append(report.Operations, "regen skipped (--no-regen)")
		return nil
	}
	for _, script := range skillsRetireRegenScripts {
		if opts.DryRun {
			report.Operations = append(report.Operations, "would run scripts/"+script)
			continue
		}
		if err := skillsRetireRunScript(opts.RepoRoot, script); err != nil {
			return err
		}
		report.Regen = append(report.Regen, script)
		report.Operations = append(report.Operations, "ran scripts/"+script)
	}
	return nil
}

func historicalStateLabel(state, into string) string {
	if state == "merged-into" {
		return "merged-into: " + into
	}
	return state
}

// skillsRetireTrees lists the slug's directory trees that exist, in
// deterministic order.
func skillsRetireTrees(repoRoot, slug string) []string {
	var trees []string
	for _, root := range []string{"skills", "skills-codex", "skills-codex-overrides"} {
		if isDir(filepath.Join(repoRoot, root, slug)) {
			trees = append(trees, root+"/"+slug)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(repoRoot, "images", "*", "skills", slug))
	for _, match := range matches {
		if isDir(match) {
			if rel, err := filepath.Rel(repoRoot, match); err == nil {
				trees = append(trees, filepath.ToSlash(rel))
			}
		}
	}
	return trees
}

var skillsRetireRowRe = regexp.MustCompile(`^\s*-\s*skill:\s+(\S+)\s*$`)

// flipDispositionsLedger removes the slug's active dispositions row and
// appends a historical terminal-state row, as a text-targeted edit that
// preserves every other byte (the file's comments are load-bearing — never a
// YAML round-trip).
func flipDispositionsLedger(path, slug, into, state, date string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read skill dispositions ledger: %w", err)
	}
	lines := strings.Split(string(data), "\n")

	// Remove the active dispositions row block: from its `- skill:` line up
	// to (exclusive) the next row or the next top-level key.
	rowRemoved := false
	var kept []string
	for i := 0; i < len(lines); i++ {
		m := skillsRetireRowRe.FindStringSubmatch(lines[i])
		if m == nil || m[1] != slug {
			kept = append(kept, lines[i])
			continue
		}
		rowRemoved = true
		for i++; i < len(lines); i++ {
			if skillsRetireRowRe.MatchString(lines[i]) || (lines[i] != "" && !strings.HasPrefix(lines[i], " ")) {
				i--
				break
			}
		}
	}
	lines = kept

	// Append the historical row at the end of the `historical:` mapping,
	// which sits ABOVE the `dispositions:` key by contract.
	dispositionsIdx := -1
	historicalIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "dispositions:") && dispositionsIdx == -1 {
			dispositionsIdx = i
		}
		if strings.HasPrefix(line, "historical:") && historicalIdx == -1 {
			historicalIdx = i
		}
	}
	if dispositionsIdx == -1 {
		return "", false, fmt.Errorf("skill dispositions ledger %s has no dispositions: section", path)
	}
	rationale := "Retired via ao skills retire: cut."
	if state == "merged-into" {
		rationale = fmt.Sprintf("Retired via ao skills retire: merged into %s.", into)
	}
	entry := []string{"  " + slug + ":", "    state:        " + state}
	if state == "merged-into" {
		entry = append(entry, "    merged-into:  "+into)
	}
	entry = append(entry,
		"    date:         "+date,
		"    rationale:    \""+rationale+"\"",
	)
	insertAt := dispositionsIdx
	if historicalIdx == -1 || historicalIdx > dispositionsIdx {
		// No historical section above dispositions: create one.
		entry = append([]string{"historical:"}, append(entry, "")...)
	} else {
		// Walk back past blank lines separating historical from dispositions.
		for insertAt > historicalIdx+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
	}
	updated := make([]string, 0, len(lines)+len(entry))
	updated = append(updated, lines[:insertAt]...)
	updated = append(updated, entry...)
	updated = append(updated, lines[insertAt:]...)
	return strings.Join(updated, "\n"), rowRemoved, nil
}

// removeCriticalSkillLine drops the slug's line from critical-skills.txt,
// preserving comments and every other line byte-for-byte.
func removeCriticalSkillLine(path, slug string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read critical skills policy: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	var kept []string
	removed := false
	for _, line := range lines {
		entry := line
		if i := strings.Index(entry, "#"); i >= 0 {
			entry = entry[:i]
		}
		if strings.TrimSpace(entry) == slug {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), removed, nil
}

// skillsRetireScanClass is one ripple-scan table row: a name, the paths it
// covers, and a file-content matcher.
type skillsRetireScanClass struct {
	name  string
	paths func(repoRoot string) []string
	match func(relPath string, data []byte, slug string) []skillsRetireRef
}

var skillsRetireScanClasses = []skillsRetireScanClass{
	{
		name: "skill-frontmatter",
		paths: func(repoRoot string) []string {
			matches, _ := filepath.Glob(filepath.Join(repoRoot, "skills", "*", "SKILL.md"))
			return matches
		},
		match: matchSkillFrontmatterEdges,
	},
	{
		name: "docs-skills-md",
		paths: func(repoRoot string) []string {
			return []string{filepath.Join(repoRoot, "docs", "SKILLS.md")}
		},
		match: matchSlugWordLines,
	},
	{
		name: "cli-test-literals",
		paths: func(repoRoot string) []string {
			return globRecursive(filepath.Join(repoRoot, "cli"), func(path string) bool {
				return strings.HasSuffix(path, "_test.go")
			})
		},
		match: func(relPath string, data []byte, slug string) []skillsRetireRef {
			return matchLiteralLines(relPath, data, "skills/"+slug)
		},
	},
	{
		name: "eval-fixtures",
		paths: func(repoRoot string) []string {
			return globRecursive(filepath.Join(repoRoot, "evals"), func(string) bool { return true })
		},
		match: matchSlugWordLines,
	},
	{
		name: "claude-link-allowlists",
		paths: func(repoRoot string) []string {
			matches, _ := filepath.Glob(filepath.Join(repoRoot, ".claude", "*allowlist*"))
			return matches
		},
		match: matchSlugWordLines,
	},
}

// scanSkillsRetireRipples runs every scan class and reports remaining
// references to the slug, excluding the removed trees themselves (relevant in
// dry-run, where they still exist on disk).
func scanSkillsRetireRipples(repoRoot, slug string, removedTrees []string) ([]skillsRetireRef, error) {
	excluded := func(rel string) bool {
		for _, tree := range removedTrees {
			if rel == tree || strings.HasPrefix(rel, tree+"/") {
				return true
			}
		}
		return false
	}
	refs := []skillsRetireRef{}
	for _, class := range skillsRetireScanClasses {
		for _, path := range class.paths(repoRoot) {
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if excluded(rel) {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("scan %s: %w", rel, err)
			}
			for _, ref := range class.match(rel, data, slug) {
				ref.Scan = class.name
				refs = append(refs, ref)
			}
		}
	}
	return refs, nil
}

// matchSkillFrontmatterEdges reports lines inside a SKILL.md frontmatter
// block where a context_rel or consumes entry names the slug.
func matchSkillFrontmatterEdges(relPath string, data []byte, slug string) []skillsRetireRef {
	var refs []skillsRetireRef
	inFrontmatter := false
	frontmatterSeen := false
	currentKey := ""
	keyRe := regexp.MustCompile(`^([A-Za-z][\w-]*):`)
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "---" {
			if !frontmatterSeen {
				frontmatterSeen = true
				inFrontmatter = true
				continue
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		if m := keyRe.FindStringSubmatch(line); m != nil {
			currentKey = m[1]
		}
		if currentKey != "context_rel" && currentKey != "consumes" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "with: "+slug || trimmed == "- "+slug ||
			(strings.HasSuffix(trimmed, " "+slug) && (strings.HasPrefix(trimmed, "with:") || strings.HasPrefix(trimmed, "-"))) {
			refs = append(refs, skillsRetireRef{File: relPath, Line: i + 1, Match: trimmed})
		}
	}
	return refs
}

// matchSlugWordLines reports lines containing the slug as a standalone token
// (word-boundary aware so "alpha" never matches "alpha-beta").
func matchSlugWordLines(relPath string, data []byte, slug string) []skillsRetireRef {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9-])` + regexp.QuoteMeta(slug) + `($|[^A-Za-z0-9-])`)
	var refs []skillsRetireRef
	for i, line := range strings.Split(string(data), "\n") {
		if re.MatchString(line) {
			refs = append(refs, skillsRetireRef{File: relPath, Line: i + 1, Match: strings.TrimSpace(line)})
		}
	}
	return refs
}

// matchLiteralLines reports lines containing the literal needle.
func matchLiteralLines(relPath string, data []byte, needle string) []skillsRetireRef {
	var refs []skillsRetireRef
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, needle) {
			refs = append(refs, skillsRetireRef{File: relPath, Line: i + 1, Match: strings.TrimSpace(line)})
		}
	}
	return refs
}

// globRecursive lists regular files under root matching keep. Missing roots
// yield an empty list.
func globRecursive(root string, keep func(path string) bool) []string {
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // missing roots and racy deletes are not scan failures
		}
		if keep(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

func printSkillsRetireReport(out io.Writer, report *skillsRetireReport) {
	header := fmt.Sprintf("retire %s (%s)", report.Slug, historicalStateLabel(historicalStateOf(report), report.Into))
	if report.DryRun {
		header = "DRY-RUN: " + header
	}
	fmt.Fprintln(out, header)
	for _, op := range report.Operations {
		prefix := ""
		if report.DryRun {
			prefix = "DRY-RUN: "
		}
		fmt.Fprintf(out, "%s%s\n", prefix, op)
	}
	if len(report.UnresolvedRefs) == 0 {
		fmt.Fprintln(out, "unresolved refs: none")
		return
	}
	fmt.Fprintf(out, "unresolved refs (%d):\n", len(report.UnresolvedRefs))
	for _, ref := range report.UnresolvedRefs {
		fmt.Fprintf(out, "  [%s] %s:%d: %s\n", ref.Scan, ref.File, ref.Line, ref.Match)
	}
}

func historicalStateOf(report *skillsRetireReport) string {
	return report.Ledger.HistoricalState
}

// Package ladder implements the five-step "next-work" decision ladder for
// /evolve. The ladder traverses ready beads and adjacent context to recommend
// what the agent should claim next; when the ladder is exhausted the agent
// is expected to call `ao evolve blocked` (see soc-g34d) rather than halt.
//
// Each step is a small function with a stable signature so callers can run
// them individually for testing. Step ownership:
//
//	step1_shape_filter      → filter operator-shape beads from `bd ready`
//	step2_grep_siblings     → enrich rationale with sibling-pattern grep matches
//	step3_primitive_test    → apply the 3-question Primitive Test
//	step4_cross_hop_pickup  → traverse in-progress beads' discovered-from chains
//	step5_bug_fallback      → final fallback: smallest-surface bug from `bd ready`
//
// The package shells out to `bd ready --json` and `bd show <id> --json` via
// the BeadRunner interface; tests inject fakes to avoid depending on a real
// bd installation. See cli/cmd/ao/evolve_next_work.go for the CLI wiring.
package ladder

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Bead is the trimmed projection of `bd ready --json` / `bd show --json` that
// the ladder needs. Additional fields from the source JSON are ignored.
type Bead struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        string   `json:"issue_type"`
	Status      string   `json:"status"`
	Labels      []string `json:"labels"`
	// Dependencies is the raw dependency list as returned by bd.
	Dependencies []Dependency `json:"dependencies"`
}

// Dependency mirrors the bd `discovered-from` / `blocks` shape.
type Dependency struct {
	Type     string `json:"type"`
	IssueID  string `json:"issue_id"`
	Relation string `json:"relation"`
}

// Recommendation is the result of running the ladder.
type Recommendation struct {
	RecommendedBead   string   `json:"recommended_bead"`
	Rationale         string   `json:"rationale"`
	Alternatives      []string `json:"alternatives"`
	LadderStepMatched int      `json:"ladder_step_matched"`
}

// BeadRunner abstracts the bd CLI for testability. Production callers pass
// ExecBeadRunner; tests pass a fake.
type BeadRunner interface {
	Ready(ctx context.Context) ([]Bead, error)
	ReadyByType(ctx context.Context, issueType string) ([]Bead, error)
	Show(ctx context.Context, id string) (Bead, error)
	InProgress(ctx context.Context) ([]Bead, error)
}

// GrepRunner abstracts the read-only filesystem grep used by step 2. Tests
// inject a fake to assert call shapes without touching the repo.
type GrepRunner interface {
	Grep(ctx context.Context, pattern string, roots []string) ([]string, error)
}

// Config captures the operator-tunable behavior of the ladder.
type Config struct {
	IncludeOperatorShape bool
	RepoRoot             string
}

// operatorShapeLabels marks beads that are scaffolding/orchestration work the
// agent should not claim unless --include-operator-shape is set.
var operatorShapeLabels = map[string]struct{}{
	"operator-shape":   {},
	"meta-runtime":     {},
	"human-coordinate": {},
}

// Run executes the ladder in order and returns the first non-empty result.
// On full exhaustion (step 5 also returns nothing) the recommendation is the
// "ladder exhausted" sentinel.
func Run(ctx context.Context, br BeadRunner, gr GrepRunner, cfg Config) (Recommendation, error) {
	if br == nil {
		return Recommendation{}, fmt.Errorf("ladder: nil BeadRunner")
	}

	// Step 1: shape filter.
	candidate, alts, err := Step1ShapeFilter(ctx, br, cfg)
	if err != nil {
		return Recommendation{}, fmt.Errorf("step1: %w", err)
	}
	if candidate.ID != "" {
		// Step 2: sibling-pattern grep enrichment.
		patterns := siblingPatterns(candidate)
		var grepHits []string
		if len(patterns) > 0 && gr != nil {
			grepHits = Step2GrepSiblings(ctx, gr, cfg.RepoRoot, patterns)
		}
		// Step 3: Primitive Test.
		passes, failureSummary := Step3PrimitiveTest(candidate)
		if !passes {
			rec := Recommendation{
				RecommendedBead:   candidate.ID,
				Rationale:         "scout-mode: " + candidate.ID + " needs decomposition; primitive test failed (" + failureSummary + ")",
				Alternatives:      alts,
				LadderStepMatched: 3,
			}
			return rec, nil
		}
		rationale := fmt.Sprintf("shape-compatible ready bead at step 1")
		if len(grepHits) > 0 {
			rationale += "; sibling refs: " + strings.Join(grepHits, ", ")
		}
		return Recommendation{
			RecommendedBead:   candidate.ID,
			Rationale:         rationale,
			Alternatives:      alts,
			LadderStepMatched: 1,
		}, nil
	}

	// Step 4: cross-hop pickup.
	siblingCand, siblingAlts, err := Step4CrossHopPickup(ctx, br)
	if err != nil {
		return Recommendation{}, fmt.Errorf("step4: %w", err)
	}
	if siblingCand.ID != "" {
		return Recommendation{
			RecommendedBead:   siblingCand.ID,
			Rationale:         "cross-hop pickup from in-progress bead's discovered-from chain",
			Alternatives:      siblingAlts,
			LadderStepMatched: 4,
		}, nil
	}

	// Step 5: bug fallback.
	bug, bugAlts, err := Step5BugFallback(ctx, br)
	if err != nil {
		return Recommendation{}, fmt.Errorf("step5: %w", err)
	}
	if bug.ID != "" {
		return Recommendation{
			RecommendedBead:   bug.ID,
			Rationale:         "bug-fallback: smallest surface-area bug from bd ready",
			Alternatives:      bugAlts,
			LadderStepMatched: 5,
		}, nil
	}

	return Recommendation{
		RecommendedBead:   "",
		Rationale:         "ladder exhausted; agent should call 'ao evolve blocked' instead of halting",
		Alternatives:      nil,
		LadderStepMatched: 0,
	}, nil
}

// Step1ShapeFilter returns the first ready bead whose labels do not match the
// operator-shape skip set (unless cfg.IncludeOperatorShape is true). The
// remaining ready beads (up to 5) are returned as alternatives.
func Step1ShapeFilter(ctx context.Context, br BeadRunner, cfg Config) (Bead, []string, error) {
	beads, err := br.Ready(ctx)
	if err != nil {
		return Bead{}, nil, err
	}
	var kept []Bead
	for _, b := range beads {
		if !cfg.IncludeOperatorShape && hasOperatorShapeLabel(b) {
			continue
		}
		kept = append(kept, b)
	}
	if len(kept) == 0 {
		return Bead{}, nil, nil
	}
	first := kept[0]
	alts := make([]string, 0, len(kept)-1)
	for i := 1; i < len(kept) && i < 6; i++ {
		alts = append(alts, kept[i].ID)
	}
	return first, alts, nil
}

// Step2GrepSiblings runs the supplied grep against the repo's skills/ and cli/
// roots for each pattern, returning up to 3 distinct file paths (file:line).
// Pure read-only enrichment — never causes the ladder to skip a step.
func Step2GrepSiblings(ctx context.Context, gr GrepRunner, repoRoot string, patterns []string) []string {
	roots := []string{
		filepath.Join(repoRoot, "skills"),
		filepath.Join(repoRoot, "cli"),
	}
	seen := map[string]struct{}{}
	var hits []string
	for _, p := range patterns {
		matches, err := gr.Grep(ctx, p, roots)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if _, dup := seen[m]; dup {
				continue
			}
			seen[m] = struct{}{}
			hits = append(hits, m)
			if len(hits) >= 3 {
				return hits
			}
		}
	}
	return hits
}

// Step3PrimitiveTest applies the 3-question Primitive Test to a candidate
// bead. Returns (passes, summary). passes is true iff at most one question
// answers "no"; summary is a short string describing which questions failed
// when passes is false.
func Step3PrimitiveTest(b Bead) (bool, string) {
	q1Names := primitiveQ1NamesFiles(b)
	q2Observable := primitiveQ2HasObservableAcceptance(b)
	q3Sibling := primitiveQ3CitesSibling(b)

	misses := []string{}
	if !q1Names {
		misses = append(misses, "Q1 names-files")
	}
	if !q2Observable {
		misses = append(misses, "Q2 observable-acceptance")
	}
	if !q3Sibling {
		misses = append(misses, "Q3 sibling-cited")
	}
	if len(misses) >= 2 {
		return false, strings.Join(misses, ", ")
	}
	return true, ""
}

// Step4CrossHopPickup walks the in-progress beads' discovered-from chains for
// sibling ready beads. Returns the first match or the zero Bead.
func Step4CrossHopPickup(ctx context.Context, br BeadRunner) (Bead, []string, error) {
	inProgress, err := br.InProgress(ctx)
	if err != nil {
		// in-progress lookup is best-effort; fall through silently.
		return Bead{}, nil, nil
	}
	seen := map[string]struct{}{}
	var candidates []Bead
	for _, ip := range inProgress {
		full, err := br.Show(ctx, ip.ID)
		if err != nil {
			continue
		}
		for _, dep := range full.Dependencies {
			if dep.Relation != "discovered-from" && dep.Type != "discovered-from" {
				continue
			}
			id := dep.IssueID
			if id == "" || id == ip.ID {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			sib, err := br.Show(ctx, id)
			if err != nil || sib.Status != "ready" {
				continue
			}
			candidates = append(candidates, sib)
		}
	}
	if len(candidates) == 0 {
		return Bead{}, nil, nil
	}
	alts := make([]string, 0, len(candidates)-1)
	for i := 1; i < len(candidates) && i < 6; i++ {
		alts = append(alts, candidates[i].ID)
	}
	return candidates[0], alts, nil
}

// Step5BugFallback returns the ready bug with the smallest surface area
// (heuristic: distinct file paths mentioned in description).
func Step5BugFallback(ctx context.Context, br BeadRunner) (Bead, []string, error) {
	bugs, err := br.ReadyByType(ctx, "bug")
	if err != nil {
		return Bead{}, nil, err
	}
	if len(bugs) == 0 {
		return Bead{}, nil, nil
	}
	sort.SliceStable(bugs, func(i, j int) bool {
		return surfaceArea(bugs[i]) < surfaceArea(bugs[j])
	})
	alts := make([]string, 0, len(bugs)-1)
	for i := 1; i < len(bugs) && i < 6; i++ {
		alts = append(alts, bugs[i].ID)
	}
	return bugs[0], alts, nil
}

// siblingPatterns returns the trigger phrases the bead's text contains. The
// returned slice contains the exact substrings to grep for.
func siblingPatterns(b Bead) []string {
	corpus := strings.ToLower(b.Title + "\n" + b.Description)
	triggers := []string{
		"wiring",
		"with_x builder",
		"hop c shape",
		"sibling pattern",
	}
	var out []string
	for _, t := range triggers {
		if strings.Contains(corpus, t) {
			out = append(out, t)
		}
	}
	return out
}

func hasOperatorShapeLabel(b Bead) bool {
	for _, l := range b.Labels {
		if _, ok := operatorShapeLabels[strings.ToLower(l)]; ok {
			return true
		}
	}
	return false
}

// primitiveQ1NamesFiles answers "does the bead description name files?". A
// "file" is a token that looks like a path with an extension and at least one
// slash, OR a known top-level dir name like cli/, skills/, scripts/, docs/.
var filePathRegex = regexp.MustCompile(`(?m)\b[A-Za-z0-9_./-]+\.(?:go|md|json|yaml|yml|sh|py|ts|tsx|js|feature|bats)\b`)

func primitiveQ1NamesFiles(b Bead) bool {
	return filePathRegex.MatchString(b.Description)
}

// primitiveQ2HasObservableAcceptance answers "does it have observable
// acceptance?" — looks for testable assertion vocabulary or a Scenarios block.
func primitiveQ2HasObservableAcceptance(b Bead) bool {
	corpus := strings.ToLower(b.Description)
	markers := []string{
		"## scenarios",
		"## acceptance",
		"acceptance:",
		"when ",
		"then ",
		"assert ",
		"asserts ",
		"asserted",
		"observable",
		"verifies that",
		"verify that",
		"check that",
		"exit 1",
		"exit 0",
		"returns ",
		"returns:",
	}
	for _, m := range markers {
		if strings.Contains(corpus, m) {
			return true
		}
	}
	return false
}

// primitiveQ3CitesSibling answers "does it cite a sibling or predecessor
// pattern?" — looks for explicit pattern-citation vocabulary.
func primitiveQ3CitesSibling(b Bead) bool {
	corpus := strings.ToLower(b.Title + "\n" + b.Description)
	markers := []string{
		"sibling",
		"predecessor",
		"follows ",
		"mirrors ",
		"as in ",
		"pattern from",
		"see soc-",
		"see also",
		"based on soc-",
	}
	for _, m := range markers {
		if strings.Contains(corpus, m) {
			return true
		}
	}
	return false
}

// surfaceArea counts distinct file-path-ish tokens in the description.
func surfaceArea(b Bead) int {
	matches := filePathRegex.FindAllString(b.Description, -1)
	seen := map[string]struct{}{}
	for _, m := range matches {
		seen[m] = struct{}{}
	}
	return len(seen)
}

// ExecBeadRunner is the production BeadRunner that shells out to `bd`.
type ExecBeadRunner struct {
	// BinaryPath optionally pins the bd binary; empty resolves via $PATH.
	BinaryPath string
}

// Ready returns all ready beads.
func (e ExecBeadRunner) Ready(ctx context.Context) ([]Bead, error) {
	return e.runReady(ctx, nil)
}

// ReadyByType returns ready beads filtered to a single issue type.
func (e ExecBeadRunner) ReadyByType(ctx context.Context, issueType string) ([]Bead, error) {
	return e.runReady(ctx, []string{"--type=" + issueType})
}

func (e ExecBeadRunner) runReady(ctx context.Context, extra []string) ([]Bead, error) {
	args := append([]string{"ready", "--json"}, extra...)
	bin := e.BinaryPath
	if bin == "" {
		bin = "bd"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}
	return decodeBeadList(out)
}

// Show returns the full Bead detail for id.
func (e ExecBeadRunner) Show(ctx context.Context, id string) (Bead, error) {
	bin := e.BinaryPath
	if bin == "" {
		bin = "bd"
	}
	cmd := exec.CommandContext(ctx, bin, "show", id, "--json")
	out, err := cmd.Output()
	if err != nil {
		return Bead{}, fmt.Errorf("bd show %s: %w", id, err)
	}
	var b Bead
	if err := json.Unmarshal(out, &b); err != nil {
		return Bead{}, fmt.Errorf("decode bd show %s: %w", id, err)
	}
	return b, nil
}

// InProgress returns beads whose status is "in_progress".
func (e ExecBeadRunner) InProgress(ctx context.Context) ([]Bead, error) {
	bin := e.BinaryPath
	if bin == "" {
		bin = "bd"
	}
	cmd := exec.CommandContext(ctx, bin, "list", "--status=in_progress", "--json")
	out, err := cmd.Output()
	if err != nil {
		// Some bd versions use different status names; treat missing as empty.
		return nil, nil
	}
	return decodeBeadList(out)
}

func decodeBeadList(out []byte) ([]Bead, error) {
	out = trimByteOrderMark(out)
	if len(out) == 0 {
		return nil, nil
	}
	// bd ready --json sometimes emits a wrapper {"issues":[...]} and sometimes a
	// raw [...] depending on subcommand. Try both.
	var direct []Bead
	if err := json.Unmarshal(out, &direct); err == nil {
		return direct, nil
	}
	var wrap struct {
		Issues []Bead `json:"issues"`
		Items  []Bead `json:"items"`
		Data   []Bead `json:"data"`
	}
	if err := json.Unmarshal(out, &wrap); err != nil {
		return nil, fmt.Errorf("decode bd json: %w", err)
	}
	switch {
	case len(wrap.Issues) > 0:
		return wrap.Issues, nil
	case len(wrap.Items) > 0:
		return wrap.Items, nil
	case len(wrap.Data) > 0:
		return wrap.Data, nil
	}
	return nil, nil
}

func trimByteOrderMark(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// ExecGrepRunner shells out to `grep -rn -l`-style ripgrep equivalent (using
// plain grep for portability). Returns up to 3 file:line hits per pattern.
type ExecGrepRunner struct{}

// Grep runs grep -rn against the given roots for pattern and returns up to
// 3 file:line strings.
func (ExecGrepRunner) Grep(ctx context.Context, pattern string, roots []string) ([]string, error) {
	args := []string{"-rn", "--max-count=3", pattern}
	args = append(args, roots...)
	cmd := exec.CommandContext(ctx, "grep", args...)
	out, _ := cmd.Output() // non-zero exit when no matches; ignore.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var hits []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// "file:line:content" → keep "file:line".
		parts := strings.SplitN(l, ":", 3)
		if len(parts) < 2 {
			continue
		}
		hits = append(hits, parts[0]+":"+parts[1])
		if len(hits) >= 3 {
			break
		}
	}
	return hits, nil
}

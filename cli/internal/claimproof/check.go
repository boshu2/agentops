// Package claimproof reports whether changed public claims have citable
// evidence. It is intentionally read-only: promotion and binding stay in the
// existing claim registry and ClaimEvidencePort surfaces.
package claimproof

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/claimpolicy"
)

const defaultBase = "origin/main"

var markerRe = regexp.MustCompile(`agentops:claim:(AOP-CLAIM-[A-Z0-9-]+)`)

// GitRunner runs a git subcommand in repoRoot and returns stdout.
type GitRunner func(ctx context.Context, repoRoot string, args ...string) (string, error)

// Options configure a claim proof-card check.
type Options struct {
	RepoRoot    string
	Base        string
	ChangedOnly bool
	RunGit      GitRunner
}

// Report is the structured output for ao claim check.
type Report struct {
	Summary Summary `json:"summary" yaml:"summary"`
	Cards   []Card  `json:"cards" yaml:"cards"`
}

// Summary gives operator-scale context for the proof-card list.
type Summary struct {
	Mode            string         `json:"mode" yaml:"mode"`
	Base            string         `json:"base,omitempty" yaml:"base,omitempty"`
	ChangedSurfaces int            `json:"changed_surfaces" yaml:"changed_surfaces"`
	Claims          int            `json:"claims" yaml:"claims"`
	Verdicts        map[string]int `json:"verdicts" yaml:"verdicts"`
}

// Card is the per-claim, per-surface proof card.
type Card struct {
	ClaimID       string     `json:"claim_id" yaml:"claim_id"`
	Surface       string     `json:"surface" yaml:"surface"`
	Tier          string     `json:"tier" yaml:"tier"`
	CitationOK    bool       `json:"citation_ok" yaml:"citation_ok"`
	CiteAllowed   []string   `json:"cite_allowed,omitempty" yaml:"cite_allowed,omitempty"`
	RegistryFound bool       `json:"registry_found" yaml:"registry_found"`
	Evidence      []Evidence `json:"evidence" yaml:"evidence"`
	EvalBinding   string     `json:"eval_binding,omitempty" yaml:"eval_binding,omitempty"`
	Verdict       string     `json:"verdict" yaml:"verdict"`
	NextAction    string     `json:"next_action" yaml:"next_action"`
}

// Evidence describes one registry evidence path.
type Evidence struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type registryFile struct {
	Version string                  `yaml:"version"`
	Tiers   map[string]registryTier `yaml:"tiers"`
	Claims  map[string]registryRow  `yaml:"claims"`
}

type registryTier struct {
	CiteAllowed []string `yaml:"cite_allowed"`
}

type registryRow struct {
	Tier        string   `yaml:"tier"`
	Surfaces    []string `yaml:"surfaces"`
	Owner       string   `yaml:"owner"`
	Evidence    []string `yaml:"evidence"`
	EvalBinding string   `yaml:"eval_binding"`
	Notes       string   `yaml:"notes"`
}

type markerHit struct {
	ClaimID string
	Surface string
}

// Check returns proof cards for changed claim markers.
func Check(ctx context.Context, opts Options) (Report, error) {
	if opts.RepoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Report{}, fmt.Errorf("claim proof: get working directory: %w", err)
		}
		opts.RepoRoot = cwd
	}
	opts.RepoRoot = filepath.Clean(opts.RepoRoot)
	if opts.Base == "" {
		opts.Base = defaultBase
	}
	if !opts.ChangedOnly {
		return Report{}, errors.New("claim proof: only --changed mode is implemented")
	}

	reg, err := loadRegistry(opts.RepoRoot)
	if err != nil {
		return Report{}, err
	}

	changed, err := changedPaths(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	hits, err := markerHits(opts.RepoRoot, changed)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Summary: Summary{
			Mode:     "changed",
			Base:     opts.Base,
			Verdicts: map[string]int{},
		},
		Cards: []Card{},
	}
	surfaces := map[string]bool{}
	for _, hit := range hits {
		surfaces[hit.Surface] = true
		row, found := reg.Claims[hit.ClaimID]
		card := buildCard(ctx, opts, hit, row, found, reg.Tiers)
		report.Cards = append(report.Cards, card)
		report.Summary.Verdicts[card.Verdict]++
	}
	report.Summary.ChangedSurfaces = len(surfaces)
	report.Summary.Claims = len(report.Cards)
	return report, nil
}

func loadRegistry(repoRoot string) (registryFile, error) {
	path := filepath.Join(repoRoot, "docs", "contracts", "claim-registry.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return registryFile{}, fmt.Errorf("claim proof: read claim registry: %w", err)
	}
	var reg registryFile
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return registryFile{}, fmt.Errorf("claim proof: parse claim registry: %w", err)
	}
	if reg.Claims == nil {
		reg.Claims = map[string]registryRow{}
	}
	return reg, nil
}

func changedPaths(ctx context.Context, opts Options) ([]string, error) {
	run := opts.RunGit
	if run == nil {
		run = realGit
	}

	var out []string
	for _, args := range [][]string{
		{"diff", "--name-only", opts.Base + "...HEAD"},
		{"diff", "--name-only", "HEAD"},
		{"ls-files", "--others", "--exclude-standard"},
	} {
		s, err := run(ctx, opts.RepoRoot, args...)
		if err != nil {
			return nil, fmt.Errorf("claim proof: git %s: %w", strings.Join(args, " "), err)
		}
		out = append(out, splitLines(s)...)
	}
	return dedupe(out), nil
}

func markerHits(repoRoot string, paths []string) ([]markerHit, error) {
	seen := map[string]bool{}
	var hits []markerHit
	for _, rel := range paths {
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" || shouldSkipPath(rel) || !looksLikeClaimHost(rel) {
			continue
		}
		ids, err := extractClaimIDs(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, id := range ids {
			key := rel + "\x00" + id
			if seen[key] {
				continue
			}
			seen[key] = true
			hits = append(hits, markerHit{ClaimID: id, Surface: rel})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Surface == hits[j].Surface {
			return hits[i].ClaimID < hits[j].ClaimID
		}
		return hits[i].Surface < hits[j].Surface
	})
	return hits, nil
}

func shouldSkipPath(path string) bool {
	return strings.HasPrefix(path, ".agents/") ||
		strings.HasPrefix(path, "_beads/") ||
		strings.HasSuffix(path, "_test.go") ||
		strings.Contains(path, "/testdata/") ||
		path == "docs/contracts/claim-registry.yaml" ||
		path == "docs/contracts/claim-eval-promote.md"
}

func looksLikeClaimHost(path string) bool {
	switch filepath.Ext(path) {
	case ".md", ".yaml", ".yml", ".go":
		return true
	default:
		return false
	}
}

func extractClaimIDs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Read-only handle: a Close error cannot lose data, so it is safe to drop.
	defer func() { _ = f.Close() }()

	seen := map[string]bool{}
	var ids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		for _, match := range markerRe.FindAllStringSubmatch(scanner.Text(), -1) {
			id := match[1]
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, scanner.Err()
}

func buildCard(ctx context.Context, opts Options, hit markerHit, row registryRow, found bool, tiers map[string]registryTier) Card {
	card := Card{
		ClaimID:       hit.ClaimID,
		Surface:       hit.Surface,
		RegistryFound: found,
	}
	if !found {
		card.Tier = "MISSING"
		card.Verdict = "missing_registry"
		card.NextAction = "run scripts/regen-claim-registry.sh, then curate tier and evidence"
		return card
	}

	card.Tier = claimpolicy.NormalizeTier(row.Tier)
	if tier, ok := tiers[card.Tier]; ok {
		card.CiteAllowed = append([]string(nil), tier.CiteAllowed...)
		card.CitationOK = claimpolicy.SurfaceAllowed(card.CiteAllowed, hit.Surface)
	}
	card.EvalBinding = strings.TrimSpace(row.EvalBinding)
	for _, path := range row.Evidence {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		card.Evidence = append(card.Evidence, classifyEvidence(ctx, opts, path))
	}
	card.Verdict, card.NextAction = verdict(card)
	return card
}

func classifyEvidence(ctx context.Context, opts Options, path string) Evidence {
	e := Evidence{Path: filepath.ToSlash(path)}
	if strings.HasPrefix(e.Path, ".agents/") {
		e.Status = "not_citable"
		e.Reason = ".agents is gitignored; export this evidence to a tracked path before citing it"
		return e
	}
	abs := filepath.Join(opts.RepoRoot, filepath.FromSlash(e.Path))
	if _, err := os.Stat(abs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			e.Status = "missing"
			e.Reason = "registry evidence path does not exist in the working tree"
			return e
		}
		e.Status = "unreadable"
		e.Reason = err.Error()
		return e
	}
	if isTracked(ctx, opts, e.Path) {
		e.Status = "tracked"
		return e
	}
	e.Status = "untracked"
	e.Reason = "evidence exists but is not tracked by git"
	return e
}

func isTracked(ctx context.Context, opts Options, path string) bool {
	run := opts.RunGit
	if run == nil {
		run = realGit
	}
	_, err := run(ctx, opts.RepoRoot, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

func verdict(card Card) (string, string) {
	if !card.RegistryFound {
		return "missing_registry", "run scripts/regen-claim-registry.sh, then curate tier and evidence"
	}
	if !card.CitationOK {
		return "citation_ceiling", "promote the claim tier with tracked evidence or move this marker to an allowed surface before citing it here"
	}
	if len(card.Evidence) == 0 {
		return "unproven", "add tracked evidence or bind an exported artifact before strengthening this claim"
	}

	allNotCitable := true
	allTracked := true
	for _, e := range card.Evidence {
		if e.Status != "not_citable" {
			allNotCitable = false
		}
		if e.Status != "tracked" {
			allTracked = false
		}
	}
	if allNotCitable {
		return "not_citable", "export .agents evidence to docs/evidence/ with scripts/export-evidence.sh, then update the registry evidence path"
	}
	if !allTracked {
		return "weak", "replace missing, untracked, or .agents-only evidence with tracked evidence before citing this claim"
	}
	if strings.EqualFold(card.Tier, "UNPROVEN") {
		return "weak", "tracked evidence exists; run the relevant validation and curate the claim tier if it passes"
	}
	return "supported", "run scripts/regen-claim-registry.sh --check and ao gate check --fast --scope head before landing"
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		v = filepath.ToSlash(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func realGit(ctx context.Context, repoRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

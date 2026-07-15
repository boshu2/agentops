// Package domainsignal computes a read-only three-signal domain observation.
// The signals answer different questions about a reviewed change:
//
//   - intent_domain        — where was the work intended?
//   - changed_file_domains — where did the CODE actually change? (path->BC heuristic)
//   - escape_domain        — where did a reviewer place a finding?
//
// A mismatch between intent and the changed files (the code moved outside the
// intended bounded context) is itself evidence — it is PRESERVED, never collapsed.
// The path->BC mapping is an ADVISORY heuristic sourced from the prose ownership in
// docs/architecture/component-map.md (the 6 bounded contexts); it is best-effort,
// not authoritative, so an unclassified path simply contributes no signal.
package domainsignal

import (
	"slices"
	"sort"
	"strings"
)

// Bounded-context tags. These match the vocabulary beads use for intent_domain
// (agent_context.intent_domain, e.g. "BC2 Validation"), so the two BC-vocabulary
// signals (intent + changed) are directly comparable.
const (
	BC1Corpus        = "BC1 Corpus"
	BC2Validation    = "BC2 Validation"
	BC3Loop          = "BC3 Loop"
	BC4Factory       = "BC4 Factory"
	BC5Runtime       = "BC5 Runtime"
	BC6Orchestration = "BC6 Orchestration"
)

// rule maps a path PREFIX to a bounded context. Matching is PREFIX-ONLY by
// design: substring matching over paths has an unbounded false-positive tail (a
// bare "gate" misclassifies "aggregate"/"navigate"/"delegate"; even scoped to
// scripts/ it catches "scripts/aggregate-data.sh") — and for a MISMATCH signal a
// false classification is worse than a missing one (it manufactures a cross-domain
// alarm that isn't real). So this advisory heuristic favors PRECISION over recall:
// a path is classified only when its prefix is unambiguous; anything else
// contributes no signal. This intentionally uses prefix-only matching.
type rule struct {
	prefix string
	bc     string
}

// rules is the ordered path->BC table. Sourced from component-map.md ownership.
// Specific package prefixes precede the broad cli/cmd/ao adapter prefix so a
// validation package is not swallowed by the runtime adapter.
var rules = []rule{
	// BC2 Validation — deterministic gates, safety, and evidence.
	{prefix: "cli/internal/gates/", bc: BC2Validation},
	{prefix: "cli/internal/safety/", bc: BC2Validation},
	{prefix: "cli/internal/domainsignal/", bc: BC2Validation},
	{prefix: "cli/internal/provenancegraph/", bc: BC2Validation},
	{prefix: "tests/scripts/", bc: BC2Validation},
	// BC1 Corpus — .agents, wiki, retrieval, capture, citation.
	{prefix: ".agents/", bc: BC1Corpus},
	{prefix: "cli/internal/corpus/", bc: BC1Corpus},
	{prefix: "cli/internal/wiki/", bc: BC1Corpus},
	{prefix: "cli/internal/search/", bc: BC1Corpus},
	// BC3 Loop — historical tracker artifacts only.
	{prefix: "_beads/", bc: BC3Loop},
	// BC4 Factory — skill/workflow admission, skill quality.
	{prefix: "skills/", bc: BC4Factory},
	{prefix: "skills-codex/", bc: BC4Factory},
	{prefix: "cli/internal/skills", bc: BC4Factory},
	// BC6 Orchestration — substrate dispatch, swarm, agent messaging.
	{prefix: "cli/internal/orchestrat", bc: BC6Orchestration},
	{prefix: "cli/internal/swarm/", bc: BC6Orchestration},
	// BC5 Runtime — ao CLI driving adapters, git/workspace/session, harness.
	// Broad; LAST so specific internal packages above win first.
	{prefix: "cli/internal/runtimecmd/", bc: BC5Runtime},
	{prefix: "cli/internal/harness", bc: BC5Runtime},
	{prefix: "cli/cmd/ao/", bc: BC5Runtime},
}

// ClassifyPathToBC returns the bounded-context tag a path belongs to, or "" when
// no prefix rule matches (an unclassified path contributes no signal — it is not
// forced into a bucket). Deterministic: first matching prefix wins.
func ClassifyPathToBC(path string) string {
	p := strings.TrimPrefix(path, "./")
	for _, r := range rules {
		if strings.HasPrefix(p, r.prefix) {
			return r.bc
		}
	}
	return ""
}

// ChangedFilesDomains returns the DISTINCT bounded contexts the given changed
// files touch, in canonical BC order (BC1..BC6). It returns the full SET, not a
// single dominant BC: collapsing to one would destroy the cross-domain spread
// that the three-signal record exists to preserve. Unclassified paths are dropped.
func ChangedFilesDomains(paths []string) []string {
	seen := map[string]bool{}
	for _, p := range paths {
		if bc := ClassifyPathToBC(p); bc != "" {
			seen[bc] = true
		}
	}
	out := make([]string, 0, len(seen))
	for bc := range seen {
		out = append(out, bc)
	}
	sort.Strings(out) // "BC1".."BC6" sort canonically by their numeric prefix
	return out
}

// Record is a three-signal domain observation.
type Record struct {
	IntentDomain       string   `json:"intent_domain,omitempty"`
	ChangedFileDomains []string `json:"changed_file_domains,omitempty"`
	EscapeDomain       string   `json:"escape_domain,omitempty"`
	// Mismatch is true when the work crossed bounded contexts: the (non-empty)
	// intent_domain is not among the changed-file domains — the code changed
	// outside the intended BC. This is the cross-domain evidence to preserve.
	Mismatch bool `json:"mismatch"`
}

// Build assembles the three-signal record. changedFiles are repo-relative paths
// (for example, from a caller-supplied manifest); intent and escape are declared
// labels. Mismatch is computed over the BC-vocabulary signals (intent vs changed
// files); escape_domain is recorded raw (the reviewer's
// free-text choice, which may not be a BC) but is not forced into the comparison.
func Build(intent string, changedFiles []string, escape string) Record {
	changed := ChangedFilesDomains(changedFiles)
	return Record{
		IntentDomain:       intent,
		ChangedFileDomains: changed,
		EscapeDomain:       escape,
		Mismatch:           intent != "" && len(changed) > 0 && !slices.Contains(changed, intent),
	}
}

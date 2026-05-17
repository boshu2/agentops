// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/domainslice"
)

// domainSliceEvidence is the persisted, audit-friendly snapshot of a domain-slice
// manifest recorded into phased state when `ao rpi phased --domain` runs.
//
// It is distinct from the in-package domainslice manifest type (which is
// unexported): this struct is what lands in phased-state.json so `ao rpi status`
// and downstream evidence consumers can see which domain bounded the run and
// what its read fence / owned IDs were, without re-reading docs/domains/.
type domainSliceEvidence struct {
	Domain           string   `json:"domain"`
	Version          string   `json:"version"`
	BoundedContext   string   `json:"bounded_context"`
	ManifestPath     string   `json:"manifest_path"`
	DirectiveIDs     []string `json:"directive_ids,omitempty"`
	ScenarioIDs      []string `json:"scenario_ids,omitempty"`
	ContextRoots     []string `json:"context_roots,omitempty"`
	AllowedReadGlobs []string `json:"allowed_read_globs,omitempty"`
	DeniedReadGlobs  []string `json:"denied_read_globs,omitempty"`
	Owner            string   `json:"owner,omitempty"`
}

// domainsDirName is the tracked directory holding domain-slice manifests.
// Manifests live at docs/domains/<name>/manifest.yaml (ADR-0004 Decision B).
const domainsDirName = "docs/domains"

// domainManifestPath returns the repo-relative path of a domain's manifest.
func domainManifestPath(domain string) string {
	return filepath.Join(domainsDirName, domain, "manifest.yaml")
}

// listAvailableDomains returns the sorted set of domain names that have a
// manifest.yaml under docs/domains/ within cwd. Directories without a manifest
// (e.g. a bare README) are skipped.
func listAvailableDomains(cwd string) []string {
	entries, err := os.ReadDir(filepath.Join(cwd, domainsDirName))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(cwd, domainsDirName, e.Name(), "manifest.yaml")
		if _, statErr := os.Stat(manifest); statErr == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// unknownDomainError builds the actionable error returned when a caller passes
// --domain <name> but no manifest exists. It names the valid domains found and
// the scaffold command (bead F3.4) so the operator can recover.
func unknownDomainError(cwd, domain string) error {
	valid := listAvailableDomains(cwd)
	var validList string
	if len(valid) == 0 {
		validList = "(none found under docs/domains/)"
	} else {
		validList = strings.Join(valid, ", ")
	}
	return fmt.Errorf(
		"unknown domain %q: no manifest at %s\nvalid domains: %s\nto create it, run: ao rpi phased --scaffold-domain %s",
		domain, domainManifestPath(domain), validList, domain)
}

// resolveDomainEvidence loads the domain-slice manifest for opts.Domain and
// returns a persistable evidence snapshot. When opts.Domain is empty it returns
// (nil, nil) so the unscoped phased path is entirely unchanged. An unknown
// domain produces an actionable error naming the valid domains and the
// scaffold command.
func resolveDomainEvidence(cwd string, opts phasedEngineOptions) (*domainSliceEvidence, error) {
	domain := strings.TrimSpace(opts.Domain)
	if domain == "" {
		return nil, nil
	}
	rel := domainManifestPath(domain)
	abs := filepath.Join(cwd, rel)
	if _, err := os.Stat(abs); err != nil {
		return nil, unknownDomainError(cwd, domain)
	}
	manifest, err := domainslice.Load(abs)
	if err != nil {
		return nil, fmt.Errorf("load domain %q: %w", domain, err)
	}
	return &domainSliceEvidence{
		Domain:           manifest.Domain,
		Version:          manifest.Version,
		BoundedContext:   strings.TrimSpace(manifest.BoundedContext),
		ManifestPath:     rel,
		DirectiveIDs:     manifest.DirectiveIDs,
		ScenarioIDs:      manifest.ScenarioIDs,
		ContextRoots:     manifest.ContextRoots,
		AllowedReadGlobs: manifest.AllowedReadGlobs,
		DeniedReadGlobs:  manifest.DeniedReadGlobs,
		Owner:            manifest.Owner,
	}, nil
}

// applyDomainScopeToState resolves the domain manifest (if --domain was set)
// and records it on state for persistence and evidence. Unscoped runs are a
// no-op so existing `ao rpi phased <goal>` behavior is unchanged.
func applyDomainScopeToState(cwd string, opts phasedEngineOptions, state *phasedState) error {
	evidence, err := resolveDomainEvidence(cwd, opts)
	if err != nil {
		return err
	}
	if evidence == nil {
		return nil
	}
	state.Domain = evidence.Domain
	state.DomainManifest = evidence
	fmt.Printf("Domain scope: %s (%s)\n", evidence.Domain, evidence.ManifestPath)
	return nil
}

// renderDomainBoundariesBlock returns a prompt section describing the domain
// slice's boundaries: bounded context, owned directives/scenarios, context
// roots, and the read fence. It is prepended to each phase prompt when the run
// is domain-scoped. Returns "" for unscoped runs so the prompt is unchanged.
func renderDomainBoundariesBlock(state *phasedState) string {
	if state == nil || state.DomainManifest == nil {
		return ""
	}
	d := state.DomainManifest
	var b strings.Builder
	b.WriteString("## Domain scope\n\n")
	fmt.Fprintf(&b, "This RPI run is scoped to the **%s** domain slice (%s).\n", d.Domain, d.ManifestPath)
	if d.BoundedContext != "" {
		fmt.Fprintf(&b, "Bounded context: %s\n", d.BoundedContext)
	}
	b.WriteString("\n")
	writeDomainList(&b, "Owned directives", d.DirectiveIDs)
	writeDomainList(&b, "Owned scenarios", d.ScenarioIDs)
	writeDomainList(&b, "Context roots (implementation surface)", d.ContextRoots)
	writeDomainList(&b, "Allowed read globs (read fence — allow)", d.AllowedReadGlobs)
	writeDomainList(&b, "Denied read globs (read fence — deny; takes precedence)", d.DeniedReadGlobs)
	b.WriteString("Stay inside this domain slice: do not edit files outside the context roots, ")
	b.WriteString("and do not read files matching the denied globs. Scope workers, validators, ")
	b.WriteString("and scenario checks to this domain's owned directives and scenarios.\n")
	return b.String()
}

// writeDomainList appends a labeled bullet list to b. Empty lists are skipped.
func writeDomainList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n")
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

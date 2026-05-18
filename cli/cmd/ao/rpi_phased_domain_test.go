// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// domainManifestYAML is a minimal-but-complete domain-slice manifest body that
// validates against schemas/domain-slice-manifest.v1.schema.json and decodes
// via domainslice.Load. Tests write it under docs/domains/<name>/manifest.yaml.
const domainManifestYAML = `schema_version: 1
domain: payments
version: 1.0.0
bounded_context: Owns the billing subscription lifecycle and Stripe webhook ingestion.
directive_ids:
  - d-billing-webhooks
  - d-billing-dunning
scenario_ids:
  - s-2026-05-17-010
  - s-2026-05-17-011
context_roots:
  - cli/cmd/ao/billing.go
  - cli/internal/billing/
allowed_read_globs:
  - cli/cmd/ao/billing*.go
  - cli/internal/billing/**
denied_read_globs:
  - .agents/holdout/**
  - cli/internal/search/**
validation_commands:
  - label: build
    command: "cd cli && go build ./cmd/ao/..."
owner: maintainers
`

// writeDomainManifest creates docs/domains/<name>/manifest.yaml under root with body.
func writeDomainManifest(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "domains", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// TestRPIPhasedDomain_ResolveEvidence verifies that --domain loads the manifest
// and produces an evidence snapshot with the manifest's boundaries.
func TestRPIPhasedDomain_ResolveEvidence(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)

	evidence, err := resolveDomainEvidence(root, phasedEngineOptions{Domain: "payments"})
	if err != nil {
		t.Fatalf("resolveDomainEvidence: %v", err)
	}
	if evidence == nil {
		t.Fatal("expected non-nil evidence for --domain payments")
	}
	if evidence.Domain != "payments" {
		t.Errorf("Domain = %q, want %q", evidence.Domain, "payments")
	}
	if evidence.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", evidence.Version, "1.0.0")
	}
	if evidence.ManifestPath != filepath.Join("docs", "domains", "payments", "manifest.yaml") {
		t.Errorf("ManifestPath = %q, unexpected", evidence.ManifestPath)
	}
	wantDirectives := []string{"d-billing-webhooks", "d-billing-dunning"}
	if strings.Join(evidence.DirectiveIDs, ",") != strings.Join(wantDirectives, ",") {
		t.Errorf("DirectiveIDs = %v, want %v", evidence.DirectiveIDs, wantDirectives)
	}
	wantScenarios := []string{"s-2026-05-17-010", "s-2026-05-17-011"}
	if strings.Join(evidence.ScenarioIDs, ",") != strings.Join(wantScenarios, ",") {
		t.Errorf("ScenarioIDs = %v, want %v", evidence.ScenarioIDs, wantScenarios)
	}
	if strings.Join(evidence.DeniedReadGlobs, ",") != ".agents/holdout/**,cli/internal/search/**" {
		t.Errorf("DeniedReadGlobs = %v, unexpected", evidence.DeniedReadGlobs)
	}
}

// TestRPIPhasedDomain_NoDomainIsNoop verifies that an unset --domain leaves
// resolution a no-op so existing `ao rpi phased <goal>` behavior is unchanged.
func TestRPIPhasedDomain_NoDomainIsNoop(t *testing.T) {
	evidence, err := resolveDomainEvidence(t.TempDir(), phasedEngineOptions{Domain: ""})
	if err != nil {
		t.Fatalf("resolveDomainEvidence with empty domain: %v", err)
	}
	if evidence != nil {
		t.Errorf("expected nil evidence for unscoped run, got %+v", evidence)
	}
}

// TestRPIPhasedDomain_RecordedInState verifies applyDomainScopeToState records
// the domain name and manifest snapshot into phased state for persistence.
func TestRPIPhasedDomain_RecordedInState(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)

	state := &phasedState{Goal: "add dunning emails"}
	if err := applyDomainScopeToState(root, phasedEngineOptions{Domain: "payments"}, state); err != nil {
		t.Fatalf("applyDomainScopeToState: %v", err)
	}
	if state.Domain != "payments" {
		t.Errorf("state.Domain = %q, want %q", state.Domain, "payments")
	}
	if state.DomainManifest == nil {
		t.Fatal("state.DomainManifest is nil; expected recorded evidence")
	}
	if state.DomainManifest.BoundedContext != "Owns the billing subscription lifecycle and Stripe webhook ingestion." {
		t.Errorf("recorded BoundedContext = %q, unexpected", state.DomainManifest.BoundedContext)
	}

	// Unscoped run: state must be untouched.
	plain := &phasedState{Goal: "add caching"}
	if err := applyDomainScopeToState(root, phasedEngineOptions{Domain: ""}, plain); err != nil {
		t.Fatalf("applyDomainScopeToState unscoped: %v", err)
	}
	if plain.Domain != "" || plain.DomainManifest != nil {
		t.Errorf("unscoped run mutated state: Domain=%q DomainManifest=%v", plain.Domain, plain.DomainManifest)
	}
}

// TestRPIPhasedDomain_UnknownDomainError verifies an unknown --domain produces
// an actionable error naming the valid domains and the scaffold command.
func TestRPIPhasedDomain_UnknownDomainError(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)
	writeDomainManifest(t, root, "auth", strings.Replace(domainManifestYAML, "domain: payments", "domain: auth", 1))

	_, err := resolveDomainEvidence(root, phasedEngineOptions{Domain: "shipping"})
	if err == nil {
		t.Fatal("expected error for unknown domain, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		`unknown domain "shipping"`,
		"docs/domains/shipping/manifest.yaml",
		"valid domains: auth, payments",
		"ao rpi phased --scaffold-domain shipping",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("unknown-domain error missing %q\ngot: %s", want, msg)
		}
	}
}

// TestRPIPhasedDomain_UnknownDomainNoManifests verifies the error reads clearly
// when docs/domains/ has no manifests at all.
func TestRPIPhasedDomain_UnknownDomainNoManifests(t *testing.T) {
	_, err := resolveDomainEvidence(t.TempDir(), phasedEngineOptions{Domain: "ghost"})
	if err == nil {
		t.Fatal("expected error for unknown domain, got nil")
	}
	if !strings.Contains(err.Error(), "(none found under docs/domains/)") {
		t.Errorf("expected no-manifests hint, got: %s", err.Error())
	}
}

// TestRPIPhasedDomain_PromptIncludesBoundaries verifies that a domain-scoped
// run injects the domain boundaries into every phase prompt.
func TestRPIPhasedDomain_PromptIncludesBoundaries(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)

	state := &phasedState{
		Goal:     "wire stripe webhooks",
		EpicID:   "ag-5k2",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
		Opts:     defaultPhasedEngineOptions(),
	}
	if err := applyDomainScopeToState(root, phasedEngineOptions{Domain: "payments"}, state); err != nil {
		t.Fatalf("applyDomainScopeToState: %v", err)
	}

	for _, phaseNum := range []int{1, 2, 3} {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase(%d): %v", phaseNum, err)
		}
		for _, want := range []string{
			"## Domain scope",
			"scoped to the **payments** domain slice",
			"Owns the billing subscription lifecycle",
			"d-billing-webhooks",
			"s-2026-05-17-010",
			"cli/internal/billing/",
			"Denied read globs",
			"cli/internal/search/**",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("phase %d prompt missing %q\nprompt:\n%s", phaseNum, want, prompt)
			}
		}
	}
}

// TestRPIPhasedDomain_PromptUnchangedWithoutDomain verifies the no-domain path
// renders no domain-scope block — existing behavior is unchanged.
func TestRPIPhasedDomain_PromptUnchangedWithoutDomain(t *testing.T) {
	state := &phasedState{
		Goal:     "add caching layer",
		EpicID:   "ag-5k2",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
		Opts:     defaultPhasedEngineOptions(),
	}
	for _, phaseNum := range []int{1, 2, 3} {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase(%d): %v", phaseNum, err)
		}
		if strings.Contains(prompt, "## Domain scope") {
			t.Errorf("phase %d prompt unexpectedly contains domain-scope block for unscoped run", phaseNum)
		}
	}
}

// TestRPIPhasedDomain_RetryPromptIncludesBoundaries verifies the read fence is
// carried into retry prompts too.
func TestRPIPhasedDomain_RetryPromptIncludesBoundaries(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)

	state := &phasedState{
		Goal:     "wire stripe webhooks",
		EpicID:   "ag-5k2",
		Opts:     phasedEngineOptions{MaxRetries: 3},
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}
	if err := applyDomainScopeToState(root, phasedEngineOptions{Domain: "payments"}, state); err != nil {
		t.Fatalf("applyDomainScopeToState: %v", err)
	}

	retryCtx := &retryContext{Attempt: 1, Verdict: "FAIL", Findings: []finding{}}
	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("buildRetryPrompt: %v", err)
	}
	if !strings.Contains(prompt, "## Domain scope") {
		t.Errorf("retry prompt missing domain-scope block\nprompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "d-billing-webhooks") {
		t.Errorf("retry prompt missing owned directive id\nprompt:\n%s", prompt)
	}
}

// TestRPIPhasedDomain_ListAvailableDomains verifies listing only returns
// directories that actually contain a manifest.yaml.
func TestRPIPhasedDomain_ListAvailableDomains(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)
	writeDomainManifest(t, root, "auth", strings.Replace(domainManifestYAML, "domain: payments", "domain: auth", 1))
	// A directory with no manifest must be skipped.
	if err := os.MkdirAll(filepath.Join(root, "docs", "domains", "empty-dir"), 0o755); err != nil {
		t.Fatalf("mkdir empty-dir: %v", err)
	}

	got := listAvailableDomains(root)
	if strings.Join(got, ",") != "auth,payments" {
		t.Errorf("listAvailableDomains = %v, want [auth payments]", got)
	}
}

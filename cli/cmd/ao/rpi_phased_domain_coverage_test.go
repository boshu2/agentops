// practices: [agile-manifesto, dora-metrics]
// Package main — F3.T1 regression-coverage gap fill (bead soc-58nt.3.6).
//
// This file covers functions that were not directly exercised by the primary
// test files (rpi_phased_domain_test.go, rpi_phased_domain_audit_test.go,
// rpi_phased_domain_enforce_test.go, rpi_phased_domain_scaffold_test.go).
// It does NOT duplicate any existing assertion.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── normalizeAuditPath ────────────────────────────────────────────────────────

// TestNormalizeAuditPath verifies path canonicalisation: leading "./" is
// stripped, forward-slashes are preserved, and whitespace is trimmed.
// These are the invariants that make glob matching stable across platforms.
func TestNormalizeAuditPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain relative path", "cli/cmd/ao/billing.go", "cli/cmd/ao/billing.go"},
		{"dot-slash prefix", "./cli/cmd/ao/billing.go", "cli/cmd/ao/billing.go"},
		{"double dot-slash not stripped", "../cli/billing.go", "../cli/billing.go"},
		{"no-op when already clean", "GOALS.md", "GOALS.md"},
		{"leading whitespace trimmed", "  cli/x.go", "cli/x.go"},
		{"trailing whitespace trimmed", "cli/x.go  ", "cli/x.go"},
		{"whitespace + dot-slash", "  ./cli/x.go  ", "cli/x.go"},
	}
	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeAuditPath(tc.in)
			if got != tc.want {
				t.Errorf("normalizeAuditPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ── collectPhaseArtifactRefs ──────────────────────────────────────────────────

// TestCollectPhaseArtifactRefs_MultiPhase verifies that artifacts from multiple
// phase-result.json files are collected into a single flat candidate list with
// the correct phase number and evidence-source label on each entry.
func TestCollectPhaseArtifactRefs_MultiPhase(t *testing.T) {
	stateDir := t.TempDir()

	writePhaseResult := func(phase int, artifacts map[string]string) {
		t.Helper()
		pr := phaseResult{
			SchemaVersion: 1, Phase: phase, PhaseName: "phase", Status: "complete",
			Artifacts: artifacts,
		}
		data, err := json.MarshalIndent(pr, "", "  ")
		if err != nil {
			t.Fatalf("marshal phase %d result: %v", phase, err)
		}
		path := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, phase))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write phase %d result: %v", phase, err)
		}
	}

	writePhaseResult(1, map[string]string{
		"loader": "cli/internal/goals/loader.go",
	})
	writePhaseResult(2, map[string]string{
		"impl":  "cli/internal/billing/stripe.go",
		"stray": "./cli/internal/search/learnings.go", // dot-slash must be normalized
	})
	// Phase 3 result is intentionally absent — collectPhaseArtifactRefs must not panic.

	refs := collectPhaseArtifactRefs(stateDir)

	if len(refs) != 3 {
		t.Fatalf("expected 3 artifact candidates (2 from phase 2 + 1 from phase 1), got %d: %+v", len(refs), refs)
	}

	// Check phase numbers and evidence-source labels are set correctly.
	phaseSet := map[int]bool{}
	for _, r := range refs {
		phaseSet[r.Phase] = true
		wantSrc := fmt.Sprintf("phase-%d-result.json:artifacts", r.Phase)
		if r.EvidenceSource != wantSrc {
			t.Errorf("ref %q: EvidenceSource = %q, want %q", r.Path, r.EvidenceSource, wantSrc)
		}
	}
	if !phaseSet[1] || !phaseSet[2] {
		t.Errorf("expected refs from phases 1 and 2, got phase set %v", phaseSet)
	}

	// Verify dot-slash normalization was applied.
	for _, r := range refs {
		if strings.HasPrefix(r.Path, "./") {
			t.Errorf("path %q was not normalized (still has dot-slash prefix)", r.Path)
		}
	}
}

// TestCollectPhaseArtifactRefs_EmptyDir verifies no panic or error when the
// state directory has no phase-result.json files at all.
func TestCollectPhaseArtifactRefs_EmptyDir(t *testing.T) {
	refs := collectPhaseArtifactRefs(t.TempDir())
	if len(refs) != 0 {
		t.Errorf("expected empty refs for empty state dir, got %+v", refs)
	}
}

// TestCollectPhaseArtifactRefs_MalformedPhaseResult verifies that a malformed
// phase-result.json is skipped silently without affecting other phases.
func TestCollectPhaseArtifactRefs_MalformedPhaseResult(t *testing.T) {
	stateDir := t.TempDir()

	// Write a malformed phase-1 result.
	bad := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 1))
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write bad phase result: %v", err)
	}
	// Write a valid phase-2 result.
	good := phaseResult{
		SchemaVersion: 1, Phase: 2, PhaseName: "impl", Status: "complete",
		Artifacts: map[string]string{"a": "cli/x.go"},
	}
	data, _ := json.MarshalIndent(good, "", "  ")
	if err := os.WriteFile(filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 2)), data, 0o600); err != nil {
		t.Fatalf("write good phase result: %v", err)
	}

	refs := collectPhaseArtifactRefs(stateDir)
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from valid phase 2, got %d: %+v", len(refs), refs)
	}
	if len(refs) > 0 && refs[0].Phase != 2 {
		t.Errorf("expected phase 2 ref, got phase %d", refs[0].Phase)
	}
}

// ── renderDomainBoundariesBlock ────────────────────────────────────────────────

// TestRenderDomainBoundariesBlock_NilState verifies that nil state returns an
// empty string (unscoped run path is unchanged).
func TestRenderDomainBoundariesBlock_NilState(t *testing.T) {
	t.Parallel()
	got := renderDomainBoundariesBlock(nil)
	if got != "" {
		t.Errorf("renderDomainBoundariesBlock(nil) = %q, want empty string", got)
	}
}

// TestRenderDomainBoundariesBlock_NoManifest verifies that a state with no
// DomainManifest returns an empty string.
func TestRenderDomainBoundariesBlock_NoManifest(t *testing.T) {
	t.Parallel()
	got := renderDomainBoundariesBlock(&phasedState{Goal: "build X"})
	if got != "" {
		t.Errorf("renderDomainBoundariesBlock(no manifest) = %q, want empty string", got)
	}
}

// TestRenderDomainBoundariesBlock_AllFields verifies that all key sections are
// present in the rendered block when the manifest has every field populated.
func TestRenderDomainBoundariesBlock_AllFields(t *testing.T) {
	t.Parallel()
	state := &phasedState{
		DomainManifest: &domainSliceEvidence{
			Domain:           "payments",
			ManifestPath:     "docs/domains/payments/manifest.yaml",
			BoundedContext:   "Owns billing and Stripe webhook ingestion.",
			DirectiveIDs:     []string{"d-billing-webhooks"},
			ScenarioIDs:      []string{"s-2026-05-17-010"},
			ContextRoots:     []string{"cli/internal/billing/"},
			AllowedReadGlobs: []string{"cli/internal/billing/**"},
			DeniedReadGlobs:  []string{".agents/holdout/**"},
		},
	}
	got := renderDomainBoundariesBlock(state)

	for _, want := range []string{
		"## Domain scope",
		"scoped to the **payments** domain slice",
		"docs/domains/payments/manifest.yaml",
		"Owns billing and Stripe webhook ingestion.",
		"Owned directives:",
		"d-billing-webhooks",
		"Owned scenarios:",
		"s-2026-05-17-010",
		"Context roots",
		"cli/internal/billing/",
		"Allowed read globs",
		"cli/internal/billing/**",
		"Denied read globs",
		".agents/holdout/**",
		"Stay inside this domain slice",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderDomainBoundariesBlock missing %q\nblock:\n%s", want, got)
		}
	}
}

// TestRenderDomainBoundariesBlock_EmptyOptionalLists verifies that empty
// optional lists (scenario_ids, allowed_read_globs) produce no empty bullet
// sections — the rendered block stays clean.
func TestRenderDomainBoundariesBlock_EmptyOptionalLists(t *testing.T) {
	t.Parallel()
	state := &phasedState{
		DomainManifest: &domainSliceEvidence{
			Domain:       "minimal",
			ManifestPath: "docs/domains/minimal/manifest.yaml",
			ContextRoots: []string{"cli/internal/minimal/"},
			// ScenarioIDs and AllowedReadGlobs are intentionally empty.
		},
	}
	got := renderDomainBoundariesBlock(state)
	if strings.Contains(got, "Owned scenarios:") {
		t.Errorf("expected no 'Owned scenarios:' section for empty list, got block:\n%s", got)
	}
	if strings.Contains(got, "Allowed read globs") {
		t.Errorf("expected no 'Allowed read globs' section for empty list, got block:\n%s", got)
	}
	// Domain header must always appear.
	if !strings.Contains(got, "## Domain scope") {
		t.Errorf("domain scope header missing from block:\n%s", got)
	}
}

// ── domainManifestPath ────────────────────────────────────────────────────────

// TestDomainManifestPath verifies the canonical relative path formula
// docs/domains/<name>/manifest.yaml for several domain names.
func TestDomainManifestPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		domain string
		want   string
	}{
		{"payments", filepath.Join("docs", "domains", "payments", "manifest.yaml")},
		{"auth", filepath.Join("docs", "domains", "auth", "manifest.yaml")},
		{"a", filepath.Join("docs", "domains", "a", "manifest.yaml")},
	}
	for _, tc := range cases {

		t.Run(tc.domain, func(t *testing.T) {
			t.Parallel()
			got := domainManifestPath(tc.domain)
			if got != tc.want {
				t.Errorf("domainManifestPath(%q) = %q, want %q", tc.domain, got, tc.want)
			}
		})
	}
}

// ── reportDomainScopeAudit — stdout shape ─────────────────────────────────────

// TestReportDomainScopeAudit_EnforcedWithRefs verifies the stdout report for
// an `enforced` run that observed a denied-glob reference names the gate-fail
// status, the out-of-domain path, and the enforcement mode.
func TestReportDomainScopeAudit_EnforcedWithRefs(t *testing.T) {
	t.Parallel()
	audit := &domainScopeAudit{
		Domain:            "payments",
		Enforcement:       "enforced",
		EnforcementReason: "read-interception substrate active",
		GateFailed:        true,
		OutOfDomainRefs: []outOfDomainRef{
			{
				Path:           "cli/internal/search/learnings.go",
				Phase:          2,
				EvidenceSource: "phase-2-result.json:artifacts",
				Reason:         "matched denied glob",
				MatchedGlob:    "cli/internal/search/**",
			},
		},
	}
	out, err := captureStdout(t, func() error {
		reportDomainScopeAudit(audit, ".agents/rpi/domain-scope-audit.json")
		return nil
	})
	if err != nil {
		t.Fatalf("reportDomainScopeAudit: %v", err)
	}
	for _, want := range []string{
		"payments",
		"enforced",
		"read-interception substrate active",
		"out-of-domain reference",
		"cli/internal/search/learnings.go",
		"phase 2",
		"Slice gate FAILED",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("enforced report missing %q\noutput:\n%s", want, out)
		}
	}
}

// TestReportDomainScopeAudit_UnavailableWarns verifies the stdout report for an
// `unavailable` run includes the "NOT enforced" warning so operators are not
// misled into thinking enforcement is active.
func TestReportDomainScopeAudit_UnavailableWarns(t *testing.T) {
	t.Parallel()
	audit := &domainScopeAudit{
		Domain:            "payments",
		Enforcement:       "unavailable",
		EnforcementReason: "gc runtime is opaque",
		GateFailed:        false,
		OutOfDomainRefs:   []outOfDomainRef{},
	}
	out, err := captureStdout(t, func() error {
		reportDomainScopeAudit(audit, ".agents/rpi/domain-scope-audit.json")
		return nil
	})
	if err != nil {
		t.Fatalf("reportDomainScopeAudit: %v", err)
	}
	for _, want := range []string{
		"unavailable",
		"NOT enforced",
		"No out-of-domain references",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("unavailable report missing %q\noutput:\n%s", want, out)
		}
	}
}

// TestReportDomainScopeAudit_AuditedNoRefs verifies the stdout report for an
// `audited` run with no out-of-domain refs uses the clean "no references" line
// and does NOT emit a gate-fail message.
func TestReportDomainScopeAudit_AuditedNoRefs(t *testing.T) {
	t.Parallel()
	audit := &domainScopeAudit{
		Domain:          "payments",
		Enforcement:     "audited",
		GateFailed:      false,
		OutOfDomainRefs: []outOfDomainRef{},
	}
	out, err := captureStdout(t, func() error {
		reportDomainScopeAudit(audit, ".agents/rpi/domain-scope-audit.json")
		return nil
	})
	if err != nil {
		t.Fatalf("reportDomainScopeAudit: %v", err)
	}
	if !strings.Contains(out, "No out-of-domain references") {
		t.Errorf("clean audited report must say 'No out-of-domain references', got:\n%s", out)
	}
	if strings.Contains(out, "Slice gate FAILED") {
		t.Errorf("audited run must not emit gate-fail message, got:\n%s", out)
	}
}

// ── no-domain baseline: existing behavior unchanged ───────────────────────────

// TestRPIPhasedDomain_RenderBoundariesNoopUnscoped verifies that an unscoped
// state produces an empty domain block, so the prompt is bit-for-bit unchanged
// vs pre-F3 behavior. This is the "existing no-domain behavior unchanged"
// acceptance criterion.
func TestRPIPhasedDomain_RenderBoundariesNoopUnscoped(t *testing.T) {
	t.Parallel()
	for _, state := range []*phasedState{
		nil,
		{Goal: "add caching"},
		{Goal: "add caching", DomainManifest: nil},
	} {
		block := renderDomainBoundariesBlock(state)
		if block != "" {
			t.Errorf("renderDomainBoundariesBlock on unscoped state returned non-empty block: %q", block)
		}
	}
}

// ── relative-path resolution from manifest dir ────────────────────────────────

// TestResolveDomainEvidence_RelativeContextRootsPreserved verifies that
// context_roots written as repo-relative paths in the manifest are preserved
// verbatim in the evidence snapshot. The manifest loader does NOT resolve them
// against the manifest's directory — they are repo-relative by contract.
func TestResolveDomainEvidence_RelativeContextRootsPreserved(t *testing.T) {
	root := t.TempDir()
	// The manifest declares context_roots as repo-relative paths.
	manifest := `schema_version: 1
domain: billing
version: 0.1.0
bounded_context: Owns billing lifecycle.
directive_ids:
  - d-billing-core
scenario_ids: []
context_roots:
  - cli/internal/billing/
  - cli/cmd/ao/billing.go
allowed_read_globs:
  - cli/internal/billing/**
denied_read_globs:
  - .agents/holdout/**
validation_commands:
  - label: build
    command: "cd cli && go build ./cmd/ao/..."
owner: team-billing
`
	writeDomainManifest(t, root, "billing", manifest)

	evidence, err := resolveDomainEvidence(root, phasedEngineOptions{Domain: "billing"})
	if err != nil {
		t.Fatalf("resolveDomainEvidence: %v", err)
	}
	if evidence == nil {
		t.Fatal("expected non-nil evidence")
	}

	// The context_roots must be preserved exactly as declared in the manifest
	// (repo-relative, not expanded to absolute paths).
	wantRoots := []string{"cli/internal/billing/", "cli/cmd/ao/billing.go"}
	if len(evidence.ContextRoots) != len(wantRoots) {
		t.Fatalf("ContextRoots length = %d, want %d; got %v", len(evidence.ContextRoots), len(wantRoots), evidence.ContextRoots)
	}
	for i, want := range wantRoots {
		if evidence.ContextRoots[i] != want {
			t.Errorf("ContextRoots[%d] = %q, want %q", i, evidence.ContextRoots[i], want)
		}
	}
	// Absolute paths must NOT appear in the evidence — that would break glob matching.
	for _, root := range evidence.ContextRoots {
		if filepath.IsAbs(root) {
			t.Errorf("ContextRoots entry %q is absolute; must be repo-relative", root)
		}
	}
}

// TestResolveDomainEvidence_DomainNameWhitespaceTrimmed verifies that
// --domain flag values with surrounding whitespace resolve the same manifest
// as the trimmed name (the flag parser may pass raw input).
func TestResolveDomainEvidence_DomainNameWhitespaceTrimmed(t *testing.T) {
	root := t.TempDir()
	writeDomainManifest(t, root, "payments", domainManifestYAML)

	// opts.Domain with leading/trailing spaces must resolve the manifest.
	evidence, err := resolveDomainEvidence(root, phasedEngineOptions{Domain: "  payments  "})
	if err != nil {
		t.Fatalf("resolveDomainEvidence with whitespace-padded domain: %v", err)
	}
	if evidence == nil {
		t.Fatal("expected non-nil evidence for whitespace-padded domain flag")
	}
	if evidence.Domain != "payments" {
		t.Errorf("Domain = %q, want %q", evidence.Domain, "payments")
	}
}

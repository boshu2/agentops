// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"testing"
)

// TestRPIPhasedDomainEnforce_ResolveEnforcement is the core table test: under
// the hookless model each runtime resolves to exactly one of the two honest
// modes — `audited` for observable runtimes, `unavailable` for opaque ones.
func TestRPIPhasedDomainEnforce_ResolveEnforcement(t *testing.T) {
	cases := []struct {
		name        string
		runtimeMode string
		wantMode    domainEnforcementMode
	}{
		{"gc runtime is always unavailable", "gc", domainEnforcementUnavailable},
		{"direct runtime is audited", "direct", domainEnforcementAudited},
		{"stream runtime is audited", "stream", domainEnforcementAudited},
		{"tmux runtime is audited", "tmux", domainEnforcementAudited},
		{"auto runtime is audited", "auto", domainEnforcementAudited},
		{"empty runtime normalizes to auto, audited", "", domainEnforcementAudited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &phasedState{
				DomainManifest: auditTestEvidence(),
				Opts:           phasedEngineOptions{RuntimeMode: tc.runtimeMode},
			}
			decision := resolveDomainEnforcement(t.TempDir(), state)
			if decision.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q (reason: %s)", decision.Mode, tc.wantMode, decision.Reason)
			}
			if decision.Reason == "" {
				t.Error("decision.Reason must never be empty")
			}
			// The hookless resolver never names a HookSource.
			if decision.HookSource != "" {
				t.Errorf("hookless decision must not name a HookSource, got %q", decision.HookSource)
			}
			// The hookless resolver never claims enforced.
			if decision.Mode == domainEnforcementEnforced {
				t.Error("hookless resolver must never return enforced mode")
			}
		})
	}
}

// TestRPIPhasedDomainEnforce_GateFailed verifies the slice gate hard-fails only
// under the (now backward-compat-only) enforced mode when a denied-glob
// reference is observed.
func TestRPIPhasedDomainEnforce_GateFailed(t *testing.T) {
	deniedRef := []outOfDomainRef{
		{Path: "cli/internal/search/x.go", Reason: "matched denied glob", MatchedGlob: "cli/internal/search/**"},
	}
	allowFenceMiss := []outOfDomainRef{
		{Path: "cli/internal/rpi/x.go", Reason: "not in allowed read fence"},
	}
	cases := []struct {
		name string
		mode domainEnforcementMode
		refs []outOfDomainRef
		want bool
	}{
		{"enforced + denied-glob ref hard-fails", domainEnforcementEnforced, deniedRef, true},
		{"enforced + allow-fence-miss does not hard-fail", domainEnforcementEnforced, allowFenceMiss, false},
		{"enforced + no refs does not fail", domainEnforcementEnforced, nil, false},
		{"audited + denied-glob ref does not fail", domainEnforcementAudited, deniedRef, false},
		{"unavailable + denied-glob ref does not fail", domainEnforcementUnavailable, deniedRef, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gateFailedFromEnforcement(tc.mode, tc.refs)
			if got != tc.want {
				t.Errorf("gateFailedFromEnforcement(%q) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

// TestRPIPhasedDomainEnforce_AuditDistinguishesModes verifies the persisted JSON
// evidence carries each resolvable enforcement mode verbatim. Under the hookless
// model the resolver yields `audited` for observable runtimes and `unavailable`
// for opaque (Gas City) runtimes; neither hard-fails the gate.
func TestRPIPhasedDomainEnforce_AuditDistinguishesModes(t *testing.T) {
	evidence := auditTestEvidence()
	deniedRef := []outOfDomainRef{
		{Path: "cli/internal/search/learnings.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"},
	}

	// --- audited: observable runtime, visible-evidence only ---
	auditedState := &phasedState{
		DomainManifest: evidence,
		Opts:           phasedEngineOptions{RuntimeMode: "direct"},
	}
	auditedDecision := resolveDomainEnforcement(t.TempDir(), auditedState)
	audited := buildDomainScopeAuditWithEnforcement(
		"run-audited", evidence, deniedRef, []string{"phase-result.json artifacts"}, auditedDecision)
	if audited.Enforcement != "audited" {
		t.Fatalf("audited run Enforcement = %q, want audited", audited.Enforcement)
	}
	if audited.GateFailed {
		t.Error("audited run must NOT hard-fail the gate")
	}
	if audited.EnforcementHookSource != "" {
		t.Errorf("hookless audited run must not record an EnforcementHookSource, got %q", audited.EnforcementHookSource)
	}

	// --- unavailable: opaque gc path warns, no enforcement claim ---
	gcState := &phasedState{
		DomainManifest: evidence,
		Opts:           phasedEngineOptions{RuntimeMode: "gc"},
	}
	gcDecision := resolveDomainEnforcement(t.TempDir(), gcState)
	unavailable := buildDomainScopeAuditWithEnforcement(
		"run-gc", evidence, deniedRef, []string{"phase-result.json artifacts"}, gcDecision)
	if unavailable.Enforcement != "unavailable" {
		t.Fatalf("gc run Enforcement = %q, want unavailable", unavailable.Enforcement)
	}
	if unavailable.GateFailed {
		t.Error("unavailable run must NOT hard-fail the gate — no enforcement claim is made")
	}
	if unavailable.EnforcementReason == "" {
		t.Error("unavailable run must explain why enforcement is unavailable")
	}

	// The two resolvable modes must be distinct in the JSON evidence.
	if audited.Enforcement == unavailable.Enforcement {
		t.Errorf("audited and unavailable must be distinct, both = %q", audited.Enforcement)
	}
	if audited.Note == unavailable.Note {
		t.Error("enforcement Note must be distinct per mode")
	}
}

// TestRPIPhasedDomainEnforce_EvidenceRoundTrips verifies the enforcement fields
// survive a JSON marshal/unmarshal cycle so downstream consumers see them. The
// `enforced` mode + HookSource field remain in the schema for backward-compatible
// readers even though the hookless resolver never produces them.
func TestRPIPhasedDomainEnforce_EvidenceRoundTrips(t *testing.T) {
	evidence := auditTestEvidence()
	decision := domainEnforcementDecision{
		Mode:        domainEnforcementEnforced,
		Reason:      "legacy enforced decision (backward-compat schema)",
		HookSource:  "legacy",
		RuntimeMode: "tmux",
	}
	deniedRef := []outOfDomainRef{
		{Path: "cli/internal/search/x.go", Reason: "matched denied glob", Phase: 3, EvidenceSource: "phase-3-result.json:artifacts"},
	}
	audit := buildDomainScopeAuditWithEnforcement("run-rt", evidence, deniedRef, nil, decision)

	raw, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("marshal audit: %v", err)
	}
	var reread domainScopeAudit
	if err := json.Unmarshal(raw, &reread); err != nil {
		t.Fatalf("unmarshal audit: %v", err)
	}
	if reread.Enforcement != "enforced" {
		t.Errorf("reread Enforcement = %q, want enforced", reread.Enforcement)
	}
	if reread.EnforcementHookSource != "legacy" {
		t.Errorf("reread EnforcementHookSource = %q, want legacy", reread.EnforcementHookSource)
	}
	if reread.RuntimeMode != "tmux" {
		t.Errorf("reread RuntimeMode = %q, want tmux", reread.RuntimeMode)
	}
	if !reread.GateFailed {
		t.Error("reread GateFailed = false, want true (enforced + denied-glob ref)")
	}
	if reread.EnforcementReason != "legacy enforced decision (backward-compat schema)" {
		t.Errorf("reread EnforcementReason = %q, unexpected", reread.EnforcementReason)
	}
}

// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readHookManifestJSON is a minimal hooks.json declaring a PreToolUse Read hook
// that can block — a real read-observing substrate.
const readHookManifestJSON = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Read",
        "hooks": [{"type": "command", "command": "holdout-isolation-gate.sh"}]
      }
    ]
  }
}`

// noReadHookManifestJSON declares only a Bash PreToolUse hook — nothing that
// can observe a file read.
const noReadHookManifestJSON = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "dangerous-git-guard.sh"}]
      }
    ]
  }
}`

// writeHooksManifest writes a hooks.json fixture under <dir>/hooks/ and returns
// the directory so it can be used as a fake repo cwd.
func writeHooksManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}
	return dir
}

// pinHookPathsToRepo restricts hook detection to the given cwd's repo manifest
// only, so tests never observe the host's installed hooks.
func pinHookPathsToRepo(t *testing.T) {
	t.Helper()
	restore := hooksManifestPathsFn
	hooksManifestPathsFn = func(cwd string) []string {
		return []string{filepath.Join(cwd, "hooks", "hooks.json")}
	}
	t.Cleanup(func() { hooksManifestPathsFn = restore })
}

// TestRPIPhasedDomainEnforce_MatcherCoversReadObservation verifies which
// PreToolUse matcher strings count as read-observing.
func TestRPIPhasedDomainEnforce_MatcherCoversReadObservation(t *testing.T) {
	cases := []struct {
		matcher string
		want    bool
	}{
		{"Read", true},
		{"Glob", true},
		{"Grep", true},
		{"Bash", false},
		{"Edit|Write|Bash", false},
		{"Edit|Read|Bash", true},
		{"Write", false},
		{"", false},
		{"  Read  ", true},
	}
	for _, tc := range cases {
		got := matcherCoversReadObservation(tc.matcher)
		if got != tc.want {
			t.Errorf("matcherCoversReadObservation(%q) = %v, want %v", tc.matcher, got, tc.want)
		}
	}
}

// TestRPIPhasedDomainEnforce_DetectReadObservingHook verifies the manifest scan
// finds a read-observing hook only when one is actually declared.
func TestRPIPhasedDomainEnforce_DetectReadObservingHook(t *testing.T) {
	pinHookPathsToRepo(t)

	withHook := writeHooksManifest(t, readHookManifestJSON)
	if src := detectReadObservingHook(withHook); src == "" {
		t.Error("expected a read-observing hook to be detected, got none")
	}

	withoutHook := writeHooksManifest(t, noReadHookManifestJSON)
	if src := detectReadObservingHook(withoutHook); src != "" {
		t.Errorf("expected no read-observing hook (Bash-only manifest), got %q", src)
	}

	// Missing manifest dir → no hook, no error.
	if src := detectReadObservingHook(t.TempDir()); src != "" {
		t.Errorf("expected no hook for absent manifest, got %q", src)
	}

	// Malformed manifest → skipped, no hook, no panic.
	bad := writeHooksManifest(t, "{not json")
	if src := detectReadObservingHook(bad); src != "" {
		t.Errorf("expected no hook for malformed manifest, got %q", src)
	}
}

// TestRPIPhasedDomainEnforce_ResolveEnforcement is the core table test: each
// runtime/hook combination must resolve to exactly one of the three honest
// enforcement modes.
func TestRPIPhasedDomainEnforce_ResolveEnforcement(t *testing.T) {
	pinHookPathsToRepo(t)

	cases := []struct {
		name        string
		runtimeMode string
		manifest    string // "" = no hooks/ dir at all
		wantMode    domainEnforcementMode
	}{
		{"gc runtime is always unavailable", "gc", readHookManifestJSON, domainEnforcementUnavailable},
		{"gc unavailable even with no hook", "gc", "", domainEnforcementUnavailable},
		{"direct + read hook is enforced", "direct", readHookManifestJSON, domainEnforcementEnforced},
		{"stream + read hook is enforced", "stream", readHookManifestJSON, domainEnforcementEnforced},
		{"tmux + read hook is enforced", "tmux", readHookManifestJSON, domainEnforcementEnforced},
		{"auto + read hook is enforced", "auto", readHookManifestJSON, domainEnforcementEnforced},
		{"direct + no read hook is audited", "direct", noReadHookManifestJSON, domainEnforcementAudited},
		{"direct + no hooks dir is audited", "direct", "", domainEnforcementAudited},
		{"empty runtime normalizes to auto, audited without hook", "", noReadHookManifestJSON, domainEnforcementAudited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cwd string
			if tc.manifest == "" {
				cwd = t.TempDir()
			} else {
				cwd = writeHooksManifest(t, tc.manifest)
			}
			state := &phasedState{
				DomainManifest: auditTestEvidence(),
				Opts:           phasedEngineOptions{RuntimeMode: tc.runtimeMode},
			}
			decision := resolveDomainEnforcement(cwd, state)
			if decision.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q (reason: %s)", decision.Mode, tc.wantMode, decision.Reason)
			}
			if decision.Reason == "" {
				t.Error("decision.Reason must never be empty")
			}
			if tc.wantMode == domainEnforcementEnforced && decision.HookSource == "" {
				t.Error("enforced decision must name the HookSource")
			}
			if tc.wantMode != domainEnforcementEnforced && decision.HookSource != "" {
				t.Errorf("non-enforced decision must not name a HookSource, got %q", decision.HookSource)
			}
		})
	}
}

// TestRPIPhasedDomainEnforce_GateFailed verifies the slice gate hard-fails only
// under enforced mode when a denied-glob reference is observed.
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

// TestRPIPhasedDomainEnforce_AuditDistinguishesThreeModes verifies the persisted
// JSON evidence carries each of the three enforcement modes verbatim, with
// distinct reasons and notes — a hook-capable path that observed a denied-glob
// read hard-fails the gate; an opaque path reports `unavailable` without any
// enforcement claim.
func TestRPIPhasedDomainEnforce_AuditDistinguishesThreeModes(t *testing.T) {
	pinHookPathsToRepo(t)
	evidence := auditTestEvidence()
	deniedRef := []outOfDomainRef{
		{Path: "cli/internal/search/learnings.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"},
	}

	// --- enforced: hook-capable path blocks a denied-glob access ---
	enforcedRepo := writeHooksManifest(t, readHookManifestJSON)
	enforcedState := &phasedState{
		DomainManifest: evidence,
		Opts:           phasedEngineOptions{RuntimeMode: "direct"},
	}
	enforcedDecision := resolveDomainEnforcement(enforcedRepo, enforcedState)
	enforced := buildDomainScopeAuditWithEnforcement(
		"run-enforced", evidence, deniedRef, []string{"phase-result.json artifacts"}, enforcedDecision)
	if enforced.Enforcement != "enforced" {
		t.Fatalf("enforced run Enforcement = %q, want enforced", enforced.Enforcement)
	}
	if !enforced.GateFailed {
		t.Error("enforced run with denied-glob ref must set GateFailed=true")
	}
	if enforced.EnforcementHookSource == "" {
		t.Error("enforced run must record EnforcementHookSource")
	}
	if enforced.RuntimeMode != "direct" {
		t.Errorf("enforced RuntimeMode = %q, want direct", enforced.RuntimeMode)
	}

	// --- unavailable: non-hook (opaque gc) path warns, no enforcement claim ---
	gcState := &phasedState{
		DomainManifest: evidence,
		Opts:           phasedEngineOptions{RuntimeMode: "gc"},
	}
	gcDecision := resolveDomainEnforcement(enforcedRepo, gcState) // hook present, but gc is opaque
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

	// --- audited: hook-capable runtime, no read hook → visible-evidence only ---
	auditedRepo := writeHooksManifest(t, noReadHookManifestJSON)
	auditedState := &phasedState{
		DomainManifest: evidence,
		Opts:           phasedEngineOptions{RuntimeMode: "stream"},
	}
	auditedDecision := resolveDomainEnforcement(auditedRepo, auditedState)
	audited := buildDomainScopeAuditWithEnforcement(
		"run-audited", evidence, deniedRef, []string{"phase-result.json artifacts"}, auditedDecision)
	if audited.Enforcement != "audited" {
		t.Fatalf("audited run Enforcement = %q, want audited", audited.Enforcement)
	}
	if audited.GateFailed {
		t.Error("audited run must NOT hard-fail the gate")
	}

	// All three modes must be mutually distinct in the JSON evidence.
	modes := map[string]bool{
		enforced.Enforcement:    true,
		unavailable.Enforcement: true,
		audited.Enforcement:     true,
	}
	if len(modes) != 3 {
		t.Errorf("expected 3 distinct enforcement modes in evidence, got %v", modes)
	}
	// Notes must be mode-specific so a reader knows the guarantee strength.
	if enforced.Note == audited.Note || enforced.Note == unavailable.Note || audited.Note == unavailable.Note {
		t.Error("enforcement Note must be distinct per mode")
	}
}

// TestRPIPhasedDomainEnforce_EvidenceRoundTrips verifies the enforcement fields
// survive a JSON marshal/unmarshal cycle so downstream consumers see them.
func TestRPIPhasedDomainEnforce_EvidenceRoundTrips(t *testing.T) {
	evidence := auditTestEvidence()
	decision := domainEnforcementDecision{
		Mode:        domainEnforcementEnforced,
		Reason:      "read hook installed",
		HookSource:  "hooks/hooks.json",
		RuntimeMode: "tmux",
	}
	deniedRef := []outOfDomainRef{
		{Path: "cli/internal/search/x.go", Phase: 3, EvidenceSource: "phase-3-result.json:artifacts"},
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
	if reread.EnforcementHookSource != "hooks/hooks.json" {
		t.Errorf("reread EnforcementHookSource = %q, want hooks/hooks.json", reread.EnforcementHookSource)
	}
	if reread.RuntimeMode != "tmux" {
		t.Errorf("reread RuntimeMode = %q, want tmux", reread.RuntimeMode)
	}
	if !reread.GateFailed {
		t.Error("reread GateFailed = false, want true (enforced + denied-glob ref)")
	}
	if reread.EnforcementReason != "read hook installed" {
		t.Errorf("reread EnforcementReason = %q, unexpected", reread.EnforcementReason)
	}
}

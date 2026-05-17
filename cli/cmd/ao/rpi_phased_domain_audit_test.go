// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRPIPhasedDomainAudit_MatchGitignoreGlob verifies the gitignore-style glob
// matcher used to classify out-of-domain references.
func TestRPIPhasedDomainAudit_MatchGitignoreGlob(t *testing.T) {
	cases := []struct {
		name string
		glob string
		path string
		want bool
	}{
		{"recursive double-star", "cli/internal/search/**", "cli/internal/search/learnings.go", true},
		{"double-star no match other dir", "cli/internal/search/**", "cli/internal/goals/x.go", false},
		{"trailing slash dir prefix", "cli/internal/billing/", "cli/internal/billing/stripe.go", true},
		{"trailing slash exact dir", "cli/internal/billing/", "cli/internal/billing", true},
		{"single star segment", "cli/cmd/ao/billing*.go", "cli/cmd/ao/billing.go", true},
		{"single star no match", "cli/cmd/ao/billing*.go", "cli/cmd/ao/goals.go", false},
		{"plain prefix match", "GOALS.md", "GOALS.md", true},
		{"holdout deny", ".agents/holdout/**", ".agents/holdout/scenario-3.json", true},
		{"empty glob", "", "any/path.go", false},
		{"empty path", "cli/**", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchGitignoreGlob(tc.glob, tc.path)
			if got != tc.want {
				t.Errorf("matchGitignoreGlob(%q, %q) = %v, want %v", tc.glob, tc.path, got, tc.want)
			}
		})
	}
}

// TestRPIPhasedDomainAudit_ClassifyDomainRef verifies deny precedence and the
// allow-fence semantics.
func TestRPIPhasedDomainAudit_ClassifyDomainRef(t *testing.T) {
	allowed := []string{"cli/cmd/ao/billing*.go", "cli/internal/billing/**"}
	denied := []string{".agents/holdout/**", "cli/internal/search/**"}

	cases := []struct {
		name       string
		path       string
		wantOut    bool
		wantReason string
		wantGlob   string
	}{
		{"in allowed fence", "cli/internal/billing/stripe.go", false, "", ""},
		{"matches denied glob", "cli/internal/search/learnings.go", true, "matched denied glob", "cli/internal/search/**"},
		{"not in allowed fence", "cli/internal/rpi/phase.go", true, "not in allowed read fence", ""},
		{"deny beats allow", ".agents/holdout/x.json", true, "matched denied glob", ".agents/holdout/**"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, reason, glob := classifyDomainRef(tc.path, allowed, denied)
			if out != tc.wantOut {
				t.Errorf("outOfDomain = %v, want %v", out, tc.wantOut)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if glob != tc.wantGlob {
				t.Errorf("matchedGlob = %q, want %q", glob, tc.wantGlob)
			}
		})
	}
}

// TestRPIPhasedDomainAudit_ClassifyEmptyAllowList verifies that with no allow
// list, only denied globs flag a path (least-privilege off → allow-by-default).
func TestRPIPhasedDomainAudit_ClassifyEmptyAllowList(t *testing.T) {
	out, _, _ := classifyDomainRef("any/unrelated/file.go", nil, []string{"secret/**"})
	if out {
		t.Error("expected in-domain when allow list is empty and path is not denied")
	}
	out2, reason, glob := classifyDomainRef("secret/key.go", nil, []string{"secret/**"})
	if !out2 || reason != "matched denied glob" || glob != "secret/**" {
		t.Errorf("expected denied hit, got out=%v reason=%q glob=%q", out2, reason, glob)
	}
}

// auditTestEvidence returns a domainSliceEvidence with a typical read fence.
func auditTestEvidence() *domainSliceEvidence {
	return &domainSliceEvidence{
		Domain:       "payments",
		ManifestPath: filepath.Join("docs", "domains", "payments", "manifest.yaml"),
		ContextRoots: []string{"cli/internal/billing/"},
		AllowedReadGlobs: []string{
			"cli/cmd/ao/billing*.go",
			"cli/internal/billing/**",
		},
		DeniedReadGlobs: []string{
			".agents/holdout/**",
			"cli/internal/search/**",
		},
	}
}

// TestRPIPhasedDomainAudit_ScanEvidence verifies out-of-domain detection,
// dedup, and ordering.
func TestRPIPhasedDomainAudit_ScanEvidence(t *testing.T) {
	evidence := auditTestEvidence()
	candidates := []outOfDomainRef{
		{Path: "cli/internal/billing/stripe.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"},
		{Path: "cli/internal/search/learnings.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"},
		{Path: "cli/internal/search/learnings.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"}, // dup
		{Path: "cli/internal/rpi/phase.go", Phase: 1, EvidenceSource: "phase-1-result.json:artifacts"},
	}
	refs := scanEvidenceForOutOfDomain(evidence, candidates)
	if len(refs) != 2 {
		t.Fatalf("expected 2 out-of-domain refs (dedup applied), got %d: %+v", len(refs), refs)
	}
	// Sorted by phase then path: phase-1 rpi first.
	if refs[0].Path != "cli/internal/rpi/phase.go" || refs[0].Phase != 1 {
		t.Errorf("refs[0] = %+v, want rpi phase-1 first", refs[0])
	}
	if refs[0].Reason != "not in allowed read fence" {
		t.Errorf("refs[0].Reason = %q, want allow-fence miss", refs[0].Reason)
	}
	if refs[1].Path != "cli/internal/search/learnings.go" {
		t.Errorf("refs[1].Path = %q, want search file", refs[1].Path)
	}
	if refs[1].Reason != "matched denied glob" || refs[1].MatchedGlob != "cli/internal/search/**" {
		t.Errorf("refs[1] deny info = %q/%q, unexpected", refs[1].Reason, refs[1].MatchedGlob)
	}
}

// TestRPIPhasedDomainAudit_BuildArtifact verifies the audit struct shape and
// that buildDomainScopeAudit (no enforcement decision supplied) defaults to the
// conservative "audited" mode.
func TestRPIPhasedDomainAudit_BuildArtifact(t *testing.T) {
	evidence := auditTestEvidence()
	candidates := []outOfDomainRef{
		{Path: "cli/internal/search/learnings.go", Phase: 2, EvidenceSource: "phase-2-result.json:artifacts"},
	}
	audit := buildDomainScopeAudit("run123", evidence, candidates, []string{"phase-result.json artifacts"})

	if audit.Enforcement != "audited" {
		t.Errorf("Enforcement = %q, want %q (default mode)", audit.Enforcement, "audited")
	}
	if audit.GateFailed {
		t.Error("default-audited build must not set GateFailed")
	}
	if audit.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", audit.SchemaVersion)
	}
	if audit.Domain != "payments" {
		t.Errorf("Domain = %q, want payments", audit.Domain)
	}
	if audit.RunID != "run123" {
		t.Errorf("RunID = %q, want run123", audit.RunID)
	}
	if len(audit.OutOfDomainRefs) != 1 {
		t.Fatalf("expected 1 out-of-domain ref, got %d", len(audit.OutOfDomainRefs))
	}
	ref := audit.OutOfDomainRefs[0]
	if ref.Path != "cli/internal/search/learnings.go" || ref.Phase != 2 {
		t.Errorf("ref = %+v, unexpected path/phase", ref)
	}
	if ref.EvidenceSource != "phase-2-result.json:artifacts" {
		t.Errorf("ref.EvidenceSource = %q, unexpected", ref.EvidenceSource)
	}
	if !strings.Contains(audit.Note, "does NOT") {
		t.Errorf("audit.Note must disclaim hard enforcement, got: %q", audit.Note)
	}
}

// TestRPIPhasedDomainAudit_WriteAndReread verifies the artifact is written to
// disk and round-trips through JSON.
func TestRPIPhasedDomainAudit_WriteAndReread(t *testing.T) {
	cwd := t.TempDir()
	evidence := auditTestEvidence()
	audit := buildDomainScopeAudit("", evidence, nil, []string{"phase-result.json artifacts"})

	path, err := writeDomainScopeAudit(cwd, audit)
	if err != nil {
		t.Fatalf("writeDomainScopeAudit: %v", err)
	}
	// No run ID → flat path under .agents/rpi/.
	want := filepath.Join(cwd, ".agents", "rpi", domainScopeAuditFile)
	if path != want {
		t.Errorf("audit path = %q, want %q", path, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit artifact: %v", err)
	}
	var reread domainScopeAudit
	if err := json.Unmarshal(raw, &reread); err != nil {
		t.Fatalf("unmarshal audit: %v", err)
	}
	if reread.Enforcement != "audited" {
		t.Errorf("reread Enforcement = %q, want audited", reread.Enforcement)
	}
	if reread.Domain != "payments" {
		t.Errorf("reread Domain = %q, want payments", reread.Domain)
	}
	if reread.OutOfDomainRefs == nil {
		t.Error("OutOfDomainRefs must serialize as [] not null")
	}
}

// TestRPIPhasedDomainAudit_RecordEndToEnd verifies recordDomainScopeAudit reads
// phase-result.json artifacts as visible evidence and reports out-of-domain
// references with path/phase/evidence-source.
func TestRPIPhasedDomainAudit_RecordEndToEnd(t *testing.T) {
	cwd := t.TempDir()
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	// Phase 2 wrote an out-of-domain artifact (search package — denied).
	pr := phaseResult{
		SchemaVersion: 1, Phase: 2, PhaseName: "implementation", Status: "complete",
		Artifacts: map[string]string{
			"impl":  "cli/internal/billing/stripe.go",   // in domain
			"stray": "cli/internal/search/learnings.go", // out of domain (denied)
		},
	}
	data, _ := json.MarshalIndent(pr, "", "  ")
	if err := os.WriteFile(filepath.Join(stateDir, "phase-2-result.json"), data, 0o600); err != nil {
		t.Fatalf("write phase result: %v", err)
	}

	// Pin hook detection to an empty fixture set so this test is deterministic
	// regardless of the host's installed hooks → stays `audited`.
	restore := hooksManifestPathsFn
	hooksManifestPathsFn = func(string) []string { return nil }
	defer func() { hooksManifestPathsFn = restore }()

	state := &phasedState{Goal: "wire webhooks", DomainManifest: auditTestEvidence()}
	recordDomainScopeAudit(cwd, state)

	auditPath := filepath.Join(stateDir, domainScopeAuditFile)
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit artifact not written: %v", err)
	}
	var audit domainScopeAudit
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatalf("unmarshal audit: %v", err)
	}
	if audit.Enforcement != "audited" {
		t.Errorf("Enforcement = %q, want audited", audit.Enforcement)
	}
	if len(audit.OutOfDomainRefs) != 1 {
		t.Fatalf("expected 1 out-of-domain ref from phase artifacts, got %d: %+v", len(audit.OutOfDomainRefs), audit.OutOfDomainRefs)
	}
	ref := audit.OutOfDomainRefs[0]
	if ref.Path != "cli/internal/search/learnings.go" {
		t.Errorf("ref.Path = %q, want search file", ref.Path)
	}
	if ref.Phase != 2 {
		t.Errorf("ref.Phase = %d, want 2", ref.Phase)
	}
	if ref.EvidenceSource != "phase-2-result.json:artifacts" {
		t.Errorf("ref.EvidenceSource = %q, unexpected", ref.EvidenceSource)
	}
}

// TestRPIPhasedDomainAudit_UnscopedRunIsNoop verifies recordDomainScopeAudit
// does nothing for an unscoped run (no --domain).
func TestRPIPhasedDomainAudit_UnscopedRunIsNoop(t *testing.T) {
	cwd := t.TempDir()
	state := &phasedState{Goal: "plain run"} // no DomainManifest
	recordDomainScopeAudit(cwd, state)

	if _, err := os.Stat(filepath.Join(cwd, ".agents", "rpi", domainScopeAuditFile)); !os.IsNotExist(err) {
		t.Errorf("unscoped run must not write a domain-scope audit artifact")
	}
}

// TestRPIPhasedDomainAudit_RunRegistryPath verifies the audit lands in the run
// registry directory when a run ID is present.
func TestRPIPhasedDomainAudit_RunRegistryPath(t *testing.T) {
	cwd := t.TempDir()
	got := domainScopeAuditPath(cwd, "abc123def456")
	want := filepath.Join(rpiRunRegistryDir(cwd, "abc123def456"), domainScopeAuditFile)
	if got != want {
		t.Errorf("domainScopeAuditPath with run ID = %q, want %q", got, want)
	}
}

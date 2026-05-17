// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// domainScopeAuditFile is the per-run filename for the domain-scope audit artifact.
// It lands under .agents/rpi/runs/<run-id>/ (or the flat .agents/rpi/ path for
// runs without a run ID), alongside phase results.
const domainScopeAuditFile = "domain-scope-audit.json"

// domainScopeAuditEnforcement is the default enforcement value the audit emits
// when no runtime enforcement decision is supplied (e.g. legacy callers). It
// is the conservative `audited` mode: visible-evidence scanning only, no hard
// enforcement claim. Runtime-resolved enforcement (`enforced` / `audited` /
// `unavailable`) is computed by resolveDomainEnforcement (bead soc-58nt.3.9)
// and threaded through buildDomainScopeAuditWithEnforcement.
const domainScopeAuditEnforcement = "audited"

// outOfDomainRef is a single out-of-domain reference detected in visible
// evidence (a command output line, a generated-file manifest entry, etc.).
type outOfDomainRef struct {
	// Path is the repo-relative file path that fell outside the domain fence.
	Path string `json:"path"`
	// Phase is the RPI phase number whose evidence surfaced the reference
	// (0 when the reference came from non-phase evidence such as run setup).
	Phase int `json:"phase"`
	// EvidenceSource names where the reference was observed (e.g.
	// "phase-2-result.json:artifacts", "validation-command-output").
	EvidenceSource string `json:"evidence_source"`
	// Reason explains why the path is out of domain: "matched denied glob" or
	// "not in allowed read fence".
	Reason string `json:"reason"`
	// MatchedGlob is the denied glob that matched, when Reason is a deny hit.
	MatchedGlob string `json:"matched_glob,omitempty"`
}

// domainScopeAudit is the persisted, audit-friendly artifact recording the
// declared domain scope for a run plus any out-of-domain references that were
// visible in command outputs and generated artifacts.
//
// Enforcement is always "audited" — see domainScopeAuditEnforcement. This
// artifact is least-privilege *provisioning evidence*, not a runtime sandbox.
type domainScopeAudit struct {
	SchemaVersion int    `json:"schema_version"`
	RunID         string `json:"run_id,omitempty"`
	Domain        string `json:"domain"`
	ManifestPath  string `json:"manifest_path"`
	// Enforcement is the runtime-resolved enforcement mode: one of
	// "enforced" (a read-observing PreToolUse hook intercepts denied-glob
	// reads), "audited" (visible-evidence scan only), or "unavailable" (the
	// runtime cannot observe reads — e.g. opaque Gas City sessions). The
	// three modes are honest by construction: `enforced` is only claimed when
	// a real interception substrate exists. See bead soc-58nt.3.9.
	Enforcement string `json:"enforcement"`
	// EnforcementReason explains why Enforcement resolved to its value, so the
	// artifact is self-documenting (e.g. why a run is `unavailable`).
	EnforcementReason string `json:"enforcement_reason,omitempty"`
	// EnforcementHookSource names the hooks manifest that supplied the
	// read-observing PreToolUse hook, populated only when Enforcement is
	// "enforced".
	EnforcementHookSource string `json:"enforcement_hook_source,omitempty"`
	// RuntimeMode is the normalized phased runtime mode the enforcement
	// decision was made for (direct|stream|tmux|gc|auto).
	RuntimeMode string `json:"runtime_mode,omitempty"`
	// GateFailed is true when Enforcement is "enforced" and a denied-glob
	// reference still surfaced in visible evidence — the read fence was
	// crossed despite a live interception hook, so the slice gate hard-fails.
	GateFailed bool `json:"gate_failed"`
	// AllowedReadGlobs / DeniedReadGlobs are the declared read fence, copied
	// from the domain-slice manifest so the audit is self-contained.
	AllowedReadGlobs []string `json:"allowed_read_globs,omitempty"`
	DeniedReadGlobs  []string `json:"denied_read_globs,omitempty"`
	ContextRoots     []string `json:"context_roots,omitempty"`
	// EvidenceSources lists which evidence surfaces were scanned for this run.
	EvidenceSources []string `json:"evidence_sources"`
	// OutOfDomainRefs holds every out-of-domain reference found in visible
	// evidence. Empty slice means the scan found no out-of-domain references.
	OutOfDomainRefs []outOfDomainRef `json:"out_of_domain_refs"`
	// Note is a human-readable disclaimer that this is an audit, not hard
	// enforcement.
	Note string `json:"note"`
}

// matchGitignoreGlob reports whether path matches a single gitignore-style
// glob. It supports the `**` recursive wildcard and a trailing `/` (directory
// prefix). Matching is purely lexical — no filesystem access.
func matchGitignoreGlob(glob, path string) bool {
	glob = strings.TrimSpace(glob)
	path = strings.TrimSpace(path)
	if glob == "" || path == "" {
		return false
	}
	// A trailing slash means "this directory and everything under it".
	if strings.HasSuffix(glob, "/") {
		return path == strings.TrimSuffix(glob, "/") ||
			strings.HasPrefix(path, glob)
	}
	return globRegexMatch(glob, path)
}

// globRegexMatch converts a gitignore-style glob (with `**`) to a regexp-free
// matcher by splitting on `**` and matching each segment with path.Match-style
// single-level semantics. Decomposed out of matchGitignoreGlob to keep
// cyclomatic complexity low.
func globRegexMatch(glob, path string) bool {
	if !strings.Contains(glob, "**") {
		return singleLevelMatch(glob, path)
	}
	parts := strings.Split(glob, "**")
	return matchGlobParts(parts, path)
}

// matchGlobParts walks the `**`-split glob segments greedily against path.
func matchGlobParts(parts []string, path string) bool {
	pos := 0
	for i, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		idx := indexOfSegment(path[pos:], part, i == 0)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}

// indexOfSegment finds part within s; when anchored it must match at the start.
func indexOfSegment(s, part string, anchored bool) int {
	if anchored {
		if strings.HasPrefix(s, part) {
			return 0
		}
		return -1
	}
	return strings.Index(s, part)
}

// singleLevelMatch matches a glob without `**` against path, treating `*` as
// "any run of non-separator characters" via filepath.Match per path segment.
func singleLevelMatch(glob, path string) bool {
	if ok, err := filepath.Match(glob, path); err == nil && ok {
		return true
	}
	// Allow a glob with no wildcard to match a path prefix (directory ref).
	if !strings.ContainsAny(glob, "*?[") {
		return path == glob || strings.HasPrefix(path, strings.TrimSuffix(glob, "/")+"/")
	}
	return false
}

// classifyDomainRef decides whether path is out of domain given the read fence.
// Deny globs take precedence over allow globs. A path is out of domain when it
// matches a denied glob, or when an allow list exists and the path matches none
// of it. Returns (outOfDomain, reason, matchedGlob).
func classifyDomainRef(path string, allowed, denied []string) (bool, string, string) {
	for _, g := range denied {
		if matchGitignoreGlob(g, path) {
			return true, "matched denied glob", g
		}
	}
	if len(allowed) == 0 {
		return false, "", ""
	}
	for _, g := range allowed {
		if matchGitignoreGlob(g, path) {
			return false, "", ""
		}
	}
	return true, "not in allowed read fence", ""
}

// scanEvidenceForOutOfDomain inspects a set of (path, phase, source) candidate
// references and returns those that fall outside the domain read fence. The
// candidates are deduplicated by path+source so a repeated reference is
// reported once.
func scanEvidenceForOutOfDomain(evidence *domainSliceEvidence, candidates []outOfDomainRef) []outOfDomainRef {
	if evidence == nil {
		return nil
	}
	var refs []outOfDomainRef
	seen := map[string]bool{}
	for _, c := range candidates {
		key := c.Path + "|" + c.EvidenceSource
		if seen[key] {
			continue
		}
		seen[key] = true
		outOfDomain, reason, glob := classifyDomainRef(c.Path, evidence.AllowedReadGlobs, evidence.DeniedReadGlobs)
		if !outOfDomain {
			continue
		}
		refs = append(refs, outOfDomainRef{
			Path: c.Path, Phase: c.Phase, EvidenceSource: c.EvidenceSource,
			Reason: reason, MatchedGlob: glob,
		})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Phase != refs[j].Phase {
			return refs[i].Phase < refs[j].Phase
		}
		return refs[i].Path < refs[j].Path
	})
	return refs
}

// collectPhaseArtifactRefs reads the phase-result.json artifacts already
// written under stateDir and returns the file paths they reference as audit
// candidates. This is "visible evidence": generated-file manifests recorded by
// each phase. Missing or malformed result files are skipped silently — the
// audit is best-effort over whatever evidence exists.
func collectPhaseArtifactRefs(stateDir string) []outOfDomainRef {
	var candidates []outOfDomainRef
	for phase := 1; phase <= len(phases); phase++ {
		path := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, phase))
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var pr phaseResult
		if json.Unmarshal(raw, &pr) != nil {
			continue
		}
		for _, artifactPath := range pr.Artifacts {
			candidates = append(candidates, outOfDomainRef{
				Path:           normalizeAuditPath(artifactPath),
				Phase:          pr.Phase,
				EvidenceSource: fmt.Sprintf("phase-%d-result.json:artifacts", pr.Phase),
			})
		}
	}
	return candidates
}

// normalizeAuditPath canonicalises a referenced path to a repo-relative,
// forward-slash form so glob matching is stable across platforms.
func normalizeAuditPath(p string) string {
	p = strings.TrimSpace(filepath.ToSlash(p))
	return strings.TrimPrefix(p, "./")
}

// buildDomainScopeAudit assembles the audit artifact for a run from the domain
// evidence and the candidate references gathered from visible evidence. It
// records the conservative default `audited` enforcement mode; callers with a
// resolved runtime enforcement decision should use
// buildDomainScopeAuditWithEnforcement instead.
func buildDomainScopeAudit(runID string, evidence *domainSliceEvidence, candidates []outOfDomainRef, evidenceSources []string) *domainScopeAudit {
	return buildDomainScopeAuditWithEnforcement(runID, evidence, candidates, evidenceSources,
		domainEnforcementDecision{
			Mode:   domainEnforcementAudited,
			Reason: "no runtime enforcement decision supplied; defaulting to visible-evidence audit",
		})
}

// buildDomainScopeAuditWithEnforcement assembles the audit artifact and stamps
// it with a runtime-resolved enforcement decision. The decision controls the
// Enforcement / EnforcementReason / RuntimeMode fields, the GateFailed flag,
// and the human-readable Note — so the persisted JSON honestly distinguishes
// `enforced`, `audited`, and `unavailable` runs.
func buildDomainScopeAuditWithEnforcement(runID string, evidence *domainSliceEvidence, candidates []outOfDomainRef, evidenceSources []string, decision domainEnforcementDecision) *domainScopeAudit {
	refs := scanEvidenceForOutOfDomain(evidence, candidates)
	if refs == nil {
		refs = []outOfDomainRef{}
	}
	if evidenceSources == nil {
		evidenceSources = []string{}
	}
	return &domainScopeAudit{
		SchemaVersion:         1,
		RunID:                 runID,
		Domain:                evidence.Domain,
		ManifestPath:          evidence.ManifestPath,
		Enforcement:           string(decision.Mode),
		EnforcementReason:     decision.Reason,
		EnforcementHookSource: decision.HookSource,
		RuntimeMode:           decision.RuntimeMode,
		GateFailed:            gateFailedFromEnforcement(decision.Mode, refs),
		AllowedReadGlobs:      evidence.AllowedReadGlobs,
		DeniedReadGlobs:       evidence.DeniedReadGlobs,
		ContextRoots:          evidence.ContextRoots,
		EvidenceSources:       evidenceSources,
		OutOfDomainRefs:       refs,
		Note:                  domainEnforcementNote(decision.Mode),
	}
}

// domainEnforcementNote returns the human-readable disclaimer matching an
// enforcement mode, so a reader of the artifact understands exactly how strong
// the fence guarantee is for this run.
func domainEnforcementNote(mode domainEnforcementMode) string {
	switch mode {
	case domainEnforcementEnforced:
		return "Enforced: a read-observing PreToolUse hook intercepts denied-glob " +
			"reads/writes at the substrate level. A denied-glob reference visible " +
			"in this run's evidence hard-fails the slice gate (gate_failed=true)."
	case domainEnforcementUnavailable:
		return "Unavailable: the runtime cannot observe agent file reads (e.g. " +
			"opaque Gas City sessions), so the read fence is recorded but NOT " +
			"enforced. No enforcement claim is made for this run."
	default:
		return "Audit only: no read-observing hook is installed, so this records " +
			"the declared read fence and reports references visible in command " +
			"outputs and generated artifacts. It does NOT hard-enforce the fence."
	}
}

// domainScopeAuditPath returns the path of the audit artifact for a run.
// It uses the run registry directory when a run ID is present, falling back to
// the flat .agents/rpi/ path otherwise.
func domainScopeAuditPath(cwd, runID string) string {
	if strings.TrimSpace(runID) != "" {
		return filepath.Join(rpiRunRegistryDir(cwd, runID), domainScopeAuditFile)
	}
	return filepath.Join(cwd, ".agents", "rpi", domainScopeAuditFile)
}

// writeDomainScopeAudit serialises the audit artifact atomically (write .tmp,
// rename). Returns the final path so callers can report it.
func writeDomainScopeAudit(cwd string, audit *domainScopeAudit) (string, error) {
	finalPath := domainScopeAuditPath(cwd, audit.RunID)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return "", fmt.Errorf("create audit directory: %w", err)
	}
	data, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal domain-scope audit: %w", err)
	}
	data = append(data, '\n')
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write domain-scope audit tmp: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("rename domain-scope audit: %w", err)
	}
	return finalPath, nil
}

// recordDomainScopeAudit produces and persists the domain-scope audit artifact
// for a domain-scoped run. It is a no-op for unscoped runs (no --domain), so
// existing `ao rpi phased <goal>` behavior is unchanged. Out-of-domain
// references found in visible evidence are reported to stdout; the artifact
// always records enforcement: audited.
func recordDomainScopeAudit(cwd string, state *phasedState) {
	if state == nil || state.DomainManifest == nil {
		return
	}
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	candidates := collectPhaseArtifactRefs(stateDir)
	evidenceSources := []string{"phase-result.json artifacts"}
	decision := resolveDomainEnforcement(cwd, state)
	audit := buildDomainScopeAuditWithEnforcement(
		state.RunID, state.DomainManifest, candidates, evidenceSources, decision)

	path, err := writeDomainScopeAudit(cwd, audit)
	if err != nil {
		VerbosePrintf("Warning: could not write domain-scope audit: %v\n", err)
		return
	}
	reportDomainScopeAudit(audit, path)
}

// reportDomainScopeAudit prints a concise summary of the audit to stdout,
// naming any out-of-domain references with path/phase/evidence-source.
func reportDomainScopeAudit(audit *domainScopeAudit, path string) {
	fmt.Printf("Domain-scope audit (%s, enforcement: %s): %s\n",
		audit.Domain, audit.Enforcement, path)
	if audit.EnforcementReason != "" {
		fmt.Printf("  Enforcement: %s\n", audit.EnforcementReason)
	}
	if audit.Enforcement == string(domainEnforcementUnavailable) {
		fmt.Println("  Warning: read fence is NOT enforced for this run — no enforcement claim is made.")
	}
	if len(audit.OutOfDomainRefs) == 0 {
		fmt.Println("  No out-of-domain references in visible evidence.")
		return
	}
	fmt.Printf("  %d out-of-domain reference(s) in visible evidence:\n", len(audit.OutOfDomainRefs))
	for _, ref := range audit.OutOfDomainRefs {
		fmt.Printf("  - %s (phase %d, source: %s) — %s\n",
			ref.Path, ref.Phase, ref.EvidenceSource, ref.Reason)
	}
	if audit.GateFailed {
		fmt.Println("  Slice gate FAILED: a denied-glob access was observed under enforced mode.")
	}
}

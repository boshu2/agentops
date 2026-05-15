// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// FactoryAdmissionDecision is the verdict returned for one admission
// probe. Admitted=true means the work is cleared to enter the
// factory; Reasons explains why (always populated, even on PASS, so
// callers can audit the decision trail). The shape mirrors
// daemon.FactoryAdmissionDecision but is narrow on purpose — the
// daemon side carries additional scheduling/queue fields that aren't
// part of the BC4 contract.
type FactoryAdmissionDecision struct {
	Admitted bool
	Reasons  []string
}

// FactoryRepoEvidence is the repo-state slice the admission decision
// reads. Dirty=true blocks admission per the standard rules; HeadSHA
// is checked against the work order's base_sha; TrackedAgents lists
// any policy-violating tracked-.agents/ files.
type FactoryRepoEvidence struct {
	HeadSHA       string
	Dirty         bool
	TrackedAgents []string
}

// FactoryPREvidence is the open-PR-blocker slice. Known=false means
// the provider couldn't gather the evidence (treated as
// OPEN_PR_EVIDENCE_UNKNOWN per the admission rules). Blockers names
// the conflicting PR numbers/titles.
type FactoryPREvidence struct {
	Known    bool
	Blockers []string
}

// FactoryCIEvidence is the main-branch CI baseline slice. Known=false
// → MAIN_CI_UNKNOWN; Green=false (with Known=true) → MAIN_CI_RED.
type FactoryCIEvidence struct {
	Known bool
	Green bool
}

// FactoryAdmissionPort is the BC4 Factory admission read-side.
// Callers — the factory-admission executor in daemon/, future
// factory-claim-ledger consumers, and the soc-2klg.3 auto-promote
// pipeline — depend on this port so admission evidence can be
// gathered via a swappable provider (real-repo, in-memory test
// double, or future cross-repo aggregator) without coupling to the
// daemon package's concrete types.
//
// Contract:
//
//   - Each Probe* method MUST return a struct with Known set so the
//     decision logic can distinguish "we know it's OK" from
//     "we don't know yet".
//   - ProbeRepoState always returns a usable struct (HeadSHA may be
//     empty on a missing repo, which the decision logic treats as
//     REPO_HEAD_UNKNOWN).
//   - ProbeOpenPRBlockers takes a list of file globs being touched
//     by the candidate work order; the provider returns conflicts
//     with currently-open PRs.
//   - ProbeMainCIBaseline returns the main-branch CI verdict; the
//     decision logic treats Known=false as MAIN_CI_UNKNOWN.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// The shape mirrors daemon.FactoryAdmissionEvidenceProvider but lives
// at the BC4 port boundary so the daemon's concrete types stay an
// implementation detail. Production adapter (soc-2klg.1.b future
// cycle) wraps daemon.LocalFactoryAdmissionEvidenceProvider.
//
// See docs/contracts/ubiquitous-language.md (BC4 row) for the
// canonical Factory context surface. soc-2klg epic; this is port
// scaffold .1 of 4 children.
type FactoryAdmissionPort interface {
	ProbeRepoState(ctx context.Context) (FactoryRepoEvidence, error)
	ProbeOpenPRBlockers(ctx context.Context, touched []string) (FactoryPREvidence, error)
	ProbeMainCIBaseline(ctx context.Context) (FactoryCIEvidence, error)
}

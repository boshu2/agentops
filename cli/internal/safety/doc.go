// Package safety documents the AgentOps runtime threat model and hosts the
// remaining Go-native contract validators (see sandbox.go).
//
// # Status after the hookless migration (IMPORTANT — read before trusting this)
//
// AgentOps 3.0 removed all runtime hooks (commit e431339c4, 2026-05-24). The
// PreToolUse/Stop shell guards that once enforced most of the threats below are
// GONE. Enforcement did not disappear — it was RELOCATED into Go (in other
// packages) or DOWNGRADED to an advisory skill. This package no longer wires any
// guard itself; nothing imports it for enforcement. Treat the threat model below
// as a map of WHERE each control now lives, not as a claim that this package
// enforces it. (Recon 2026-07-02 audit A6 corrected the prior version of this
// doc, which described the deleted hooks as if they were active.)
//
// # Threat model → where enforcement lives now
//
// T1 - Command Injection: the hook-era binary allowlist (go/pytest/npm/make) and
// shell-metacharacter block lived in the deleted hooks/task-validation-gate.sh
// and are GONE. The CLI's remaining shell-exec surfaces (canon verifier,
// eval engine, codex task packets) run operator- or agent-authored commands by
// design and are guarded by operator-sourcing invariants + array-based exec
// (no fmt.Sprintf into "sh -c"), NOT by a runtime allowlist. See the
// AGENTOPS_CANON_VERIFIER_CMD trust-boundary comment in cli/internal/canon.
//
// T2 - Path Traversal: STILL ENFORCED, in Go, in the packages that own the
// paths — cli/internal/pool (validateCandidateID) and cli/internal/ratchet
// (ValidateArtifactPath), plus symlink-resolving confinement in
// cli/internal/worktree and cli/internal/resolver. This is the one control that
// survived the migration intact.
//
// T3 - Destructive Git Operations: the blocking hooks/dangerous-git-guard.sh is
// GONE. This is now ADVISORY, provided by the `dcg` skill rather than a runtime
// block. Additionally, root.go's SanitizeGitProcessEnv unsets GIT_DIR/
// GIT_WORK_TREE/GIT_COMMON_DIR on every invocation, and the pawl review path
// hardens git against config code-exec (--no-ext-diff, core.fsmonitor=).
//
// T4 - Worker Privilege Escalation: the blocking hooks/git-worker-guard.sh is
// GONE. Worker "no commit/push" discipline is now a convention enforced by the
// orchestration substrate (NTM/Agent Mail lane ownership), not a runtime guard.
//
// T5 - Unvalidated Code Push: RELOCATED and still enforced. The push gate is now
// the local Go release gate (ao gate check, cli/internal/gates) plus the
// commit-bound cross-family pawl verdict (docs/contracts/pawls.md,
// scripts/check-pawl-pre-push.sh). A blocking gate check that cannot produce a
// verdict now fails closed (recon 2026-07-02 W1: UNKNOWN on a blocking check is
// no longer a silent pass). NOTE: this control is only active when the pre-push
// hook is actually wired — a hijacked core.hooksPath silently bypasses it
// (audit A2; scripts/install-pre-push-gate.sh now detects and refuses that).
//
// T6 - Runaway Autonomous Loops: kill switches remain as .agents/rpi/KILL and
// the deterministic MemRL policy escalation (cli/internal/types/memrl_policy.go).
// The hook-era env kill switches (AGENTOPS_HOOKS_DISABLED and per-hook variants)
// no longer gate anything, because there are no hooks to disable.
//
// T7 - Policy Bypass and Retry Abuse: ENFORCED in Go by the MemRL policy contract
// (cli/internal/types/memrl_policy.go) and the SPC governor
// (cli/internal/governor: error-budget burn-rate ship-vs-harden, exit 3 = stop).
//
// T8 - Malicious Repository Sourcing: the specific hook-sourcing hardening is
// moot (no hooks). The live analog is the pawl RCE guard (cli/cmd/ao/pawl.go):
// in-repo review scripts run only when the ao binary physically resides in the
// resolved checkout; otherwise embedded scripts run with a sanitized PATH and
// neutralized BASH_ENV/GIT_EXTERNAL_DIFF.
//
// T9 - Team Lifecycle Violations: the blocking hooks/stop-team-guard.sh is GONE.
// The Go validators for it live in sandbox.go (ValidateMessageSize,
// ValidateTeamLifecycle) and are TESTED but currently UNWIRED — no production
// caller invokes them today. They are kept as a ready contract for a future
// native team-orchestration lane; do not describe them as active enforcement.
//
// # Design principles (post-hookless)
//
// Fail-closed for validation, fail-open for missing infra: the release gate and
// pawl fail closed (no verdict = not done); best-effort telemetry and advisory
// surfaces fail open so an incomplete toolchain never blocks legitimate work.
//
// Enforcement lives with the owner of the resource, not in this package: path
// confinement in pool/ratchet, push authority in gates/pawl, loop bounds in the
// MemRL policy + governor. This package is the shared threat-model reference plus
// the parked sandbox validators, not a runtime enforcement point.
package safety

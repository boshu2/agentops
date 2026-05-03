---
id: research-2026-05-03-repo-operating-loop-hardening
type: research
date: 2026-05-03
backend: inline-codex
goal: "Plan hardening for validation mutation, Codex hook install parity, .agents policy, worktree cleanliness, release audit scoping, and bd/Dolt boundedness."
---

# Research: Repo Operating Loop Hardening

## Summary

The six requested improvements are real and interdependent. AgentOps already has
many of the right pieces: `scripts/check-agents-hash-snapshot.sh`,
`scripts/check-agents-write-surfaces.sh`, `docs/contracts/agents-write-surfaces.md`,
`scripts/check-worktree-disposition.sh`, release audit validators, Codex plugin
install tests, and a bd server-mode closeout runbook. The problem is that these
pieces protect different state scopes, assume different mutation defaults, and
do not yet express one machine-readable operating policy.

The highest-leverage plan is not a single large refactor. It is a sequenced
operating-loop epic:

1. define repo-local `.agents` lifecycle policy,
2. make validation no-mutation by default,
3. fix installed Codex hook helper layout and force-push parsing,
4. classify canonical-root dirty state mechanically,
5. scope release audit artifact validation to relevant artifacts,
6. bound bd/Dolt commands with portable timeouts,
7. carry validation lane metadata in the repo execution profile.

## Product Context Applied

`PRODUCT.md` frames AgentOps as operational discipline for indeterministic
workers. The current failure mode cuts directly against that promise: validation
and closeout can mutate the memory corpus or fail because another local run
left artifacts behind. The Quality-First Maintainer persona benefits most from
turning those implicit behaviors into boring, repeatable contracts.

## Prior Knowledge Applied

- `.agents/patterns/2026-05-01-state-path-resolver.md` applies strongly: new
  hooks, helpers, and commands that write `.agents` state should use the
  canonical state-path resolver instead of ad hoc paths.
- `.agents/research/2026-04-29-ci-tests-hooks-noise-audit.md` applies: local
  gates need behavioral fixtures, not structural grep-only checks.
- `.agents/plans/2026-05-02-ai-native-zero-trust-release-process.md` overlaps
  with release evidence, but the stale local artifact problem is narrower and
  should not wait for the full zero-trust release epic.
- `soc-eh1z` already captured PR closeout toil. Most children are closed; this
  plan extends that lesson to repo-wide validation, hook install, and tracker
  boundedness.

## Key Files

| File | Evidence |
| --- | --- |
| `scripts/check-agents-hash-snapshot.sh` | Protects selected subtrees under `~/.agents`; it does not by itself prove repo-local `.agents` is stable. |
| `docs/contracts/agents-write-surfaces.md` | Catalogues `.agents` write surfaces, but lifecycle classifications are not yet enforceable enough for no-mutation validation. |
| `scripts/check-agents-write-surfaces.sh` | Useful extension point for enforcing `.agents` surface policy. |
| `scripts/pre-push-gate.sh` | Has isolated HOME helper for evals and a user-hub hash gate, but release audit and write-surface checks are still mixed into one fast gate. |
| `scripts/ci-local-release.sh` | Always creates `.agents/releases/local-ci/<run_id>` and snapshots `~/.agents`, so release artifact production and validation read-only behavior are not separated. |
| `scripts/release-smoke-test.sh` | On `origin/main`, smoke commands still need explicit no-citation/no-mutation discipline for read-only validation. |
| `hooks/dangerous-git-guard.sh` | Sources `../lib/hook-helpers.sh` from the hook directory and uses regex force-push detection. |
| `scripts/install-codex-plugin.sh` | Copies hook scripts to `hooks/` and copies `lib/*.sh` into that same directory, while installed hooks expect `../lib/hook-helpers.sh`. |
| `tests/scripts/test-codex-plugin-install.sh` | Tests plugin cache/config install, but does not assert installed hooks can source helpers from the cache layout. |
| `tests/hooks/hook-stdin-contracts.bats` | Covers `dangerous-git-guard.sh` force push, force-with-lease, malformed JSON, and kill switch behavior; it lacks branch-name false-positive cases. |
| `scripts/check-worktree-disposition.sh` | Already ignores some generated `.agents` and `wiki` dirt for canonical root checks; it needs clearer dirty-path classification. |
| `scripts/validate-release-audit-artifacts.sh` | Skips missing old artifact dirs, but still validates the latest audit even when its local `.agents/releases/local-ci` artifact dir is absent. |
| `docs/runbooks/bd-server-mode-closeout.md` | Correctly treats no Dolt remote as non-fatal, but does not cover commands that emit JSON and then hang. |
| `docs/contracts/repo-execution-profile.json` | Lists validation commands but not whether each lane is read-only, artifact-producing, isolated, or release-only. |

## Live Tracker Finding

During this discovery, `bd create` for `soc-0qz1.3` emitted valid JSON and then
remained alive until `gtimeout` killed it with exit `124`. A plain
`bd dolt remote list` also printed "No remotes configured." and then stayed
alive until the process was killed; the same command completed when wrapped in
`gtimeout 8s`. The issue persisted, which means repo automation should preserve
stdout from timed-out bd commands and distinguish "write probably completed but
process hung" from "write failed".

## Gap Analysis

1. **Mutation scope is split.** User-level `~/.agents` has a hash gate, while
   repo-local `.agents` relies on partial ignore and surface checks.
2. **Validation lanes are not typed.** Fast validation, release validation, and
   artifact-producing release evidence share scripts without a common policy
   describing read-only versus intentional write behavior.
3. **Installed hook layout is under-tested.** Source-tree hook tests pass even
   when installed Codex cache layout is missing `../lib`.
4. **Dangerous git parsing is too string-based.** Force push detection should
   parse command tokens enough to avoid blocking safe ref names containing `-f`.
5. **Canonical root dirt lacks ownership classification.** The gate can report
   dirty root state, but operators still have to manually decide whether each
   path is generated, policy, user edit, or unknown.
6. **Release audit artifacts are local but treated as portable.** Historical
   audit docs can reference `.agents/releases/local-ci` directories that exist
   only in the worktree that produced them.
7. **bd/Dolt closeout assumes commands terminate.** Server-mode no-remote is
   documented; hung-after-output behavior is not.

## Test Levels

Required: L0, L1, L2.

Recommended: L3 for installed Codex plugin smoke and end-to-end closeout flows.

Rationale: this plan touches shell gates, hook execution, install packaging,
Git/worktree behavior, release artifact resolution, docs contracts, and tracker
commands. Unit and fixture tests are necessary, but plugin-cache execution and
real closeout flows need system-level proof before release.

## Quality Validation

Coverage checked: startup docs, product contract, existing `.agents` plans and
research, `ao search`, `ao lookup`, write-surface contract, hash snapshot gate,
pre-push gate, local release gate, release smoke, Codex plugin installer, Codex
hook manifest, dangerous git hook tests, worktree disposition tests, release
artifact validator, bd server-mode runbook, and current open beads.

Depth ratings:

| Area | Depth | Notes |
| --- | ---: | --- |
| Validation mutation surfaces | 3/4 | Main scripts and prior branch work are identified; exact implementation can reuse existing branch `codex/soc-ff2p-rpi-release-audit`. |
| Codex hook install parity | 3/4 | Layout mismatch is concrete and testable. |
| `.agents` policy | 2/4 | Contract exists, but implementation needs policy decisions. |
| Worktree cleanliness | 2/4 | Gate behavior is clear; desired classification taxonomy needs design. |
| Release audit artifact scoping | 3/4 | Failure condition and validator branch are concrete. |
| bd/Dolt boundedness | 2/4 | Hang reproduced, but root cause is outside this repo unless bd source is vendored elsewhere. |

Critical assumption: repo-local `.agents` should remain a mix of force-tracked
durable knowledge and ignored runtime state. The plan should not flatten that
into "track all" or "ignore all"; it should classify each surface explicitly.

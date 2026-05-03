---
id: plan-2026-05-03-repo-operating-loop-hardening
type: plan
date: 2026-05-03
epic: soc-0qz1
source: ".agents/research/2026-05-03-repo-operating-loop-hardening.md"
---

# Plan: Repo Operating Loop Hardening

## Context

This plan covers the six repo-improvement themes from the 2026-05-03 objective:

- validation must be hermetic and non-mutating by default;
- installed Codex hooks must match source-tree assumptions;
- repo-local `.agents` tracking and ignore policy must be explicit;
- canonical root cleanliness must be mechanically classified;
- release artifact validation must not fail on stale local historical artifacts;
- bd/Dolt closeout commands must be bounded for automation.

Existing related work stays in place. `soc-ff2p.4` is the immediate dirty-root
cleanup blocker. `soc-owed` is the larger zero-trust release process epic.
`soc-eh1z` captured earlier PR closeout toil. New epic `soc-0qz1` is the
operating-loop hardening plan that connects the shared causes across those
threads.

## Files To Modify

| File | Change |
| --- | --- |
| `docs/contracts/agents-write-surfaces.md` | Add lifecycle classifications and allowed writer policy. |
| `scripts/check-agents-write-surfaces.sh` | Enforce missing/unknown `.agents` surface classifications. |
| `tests/scripts/check-agents-write-surfaces.bats` | Cover classification drift and new surface additions. |
| `docs/contracts/repo-execution-profile.md` | Define validation lane metadata semantics. |
| `docs/contracts/repo-execution-profile.json` | Classify repo validation commands. |
| `schemas/repo-execution-profile.schema.json` | Add lane metadata schema if the schema currently omits it. |
| `scripts/pre-push-gate.sh` | Consume scoped release audit checks and no-mutation validation lanes. |
| `scripts/ci-local-release.sh` | Separate read-only validation from intentional release artifact production. |
| `scripts/release-smoke-test.sh` | Preserve no-citation/no-mutation behavior for read-only smoke runs. |
| `scripts/check-agents-hash-snapshot.sh` | Extend or complement hashing for repo-local tracked knowledge surfaces. |
| `hooks/dangerous-git-guard.sh` | Fix helper sourcing under installed plugin layout and force-push token parsing. |
| `scripts/install-codex-plugin.sh` | Preserve hook helper layout in the plugin cache. |
| `tests/scripts/test-codex-plugin-install.sh` | Assert installed hooks can source helpers and execute from cache. |
| `tests/hooks/hook-stdin-contracts.bats` | Add safe branch/ref false-positive cases for dangerous git guard. |
| `scripts/check-worktree-disposition.sh` | Classify canonical-root dirty paths by owner/action. |
| `tests/scripts/test-worktree-disposition.sh` | Cover mixed generated, tracked policy, and unknown dirty root state. |
| `scripts/validate-release-audit-artifacts.sh` | Add explicit changed/target/all validation modes. |
| `tests/scripts/release-artifacts.bats` | Cover missing historical/latest artifact dirs and strict target release mode. |
| `docs/runbooks/bd-server-mode-closeout.md` | Add timeout-safe command behavior and hung-after-output handling. |

## Boundaries

Always:

- Keep release artifact production possible; only make read-only validation the
  default where release artifacts are not the objective.
- Preserve force-tracked durable `.agents` knowledge that is intentionally part
  of the repo.
- Test installed Codex plugin behavior, not only source-tree hooks.
- Treat `bd dolt push` without a remote and `bd command timed out` as distinct
  outcomes.

Never:

- Flatten `.agents` policy into "ignore everything" or "track everything".
- Make official release validation lax because local historical artifacts are
  missing.
- Block normal Git pushes because a branch or ref name contains `-f`.
- Require a clean canonical root by deleting or reverting user edits.

Ask first:

- Whether repo-local release artifact directories should ever be force-tracked
  for official release evidence, or whether audits should point to external
  durable artifacts instead.

## Issues

### `soc-0qz1.2` - Define repo-local .agents mutability policy and write-surface contract

Ownership: `docs/contracts/agents-write-surfaces.md`,
`scripts/check-agents-write-surfaces.sh`,
`tests/scripts/check-agents-write-surfaces.bats`.

Acceptance:

- Each top-level repo `.agents` surface has a lifecycle classification.
- Allowed writers are documented and enforceable.
- New unclassified surfaces fail fixtures or the write-surface gate.

Validation:

- `bats tests/scripts/check-agents-write-surfaces.bats`
- `bash scripts/check-agents-write-surfaces.sh`

Test levels: L0, L1.

### `soc-0qz1.8` - Codify repo execution profile no-mutation validation lanes

Ownership: `docs/contracts/repo-execution-profile.md`,
`docs/contracts/repo-execution-profile.json`,
`schemas/repo-execution-profile.schema.json`, RPI packet generation.

Acceptance:

- Validation commands include metadata such as `read_only`, `writes_artifacts`,
  `isolated_agents_home`, `release_only`, and mutation escape hatches.
- RPI execution packets carry validation lane metadata.
- Contract compatibility and schema tests pass.

Validation:

- `bash scripts/check-contract-compatibility.sh`
- `jq empty docs/contracts/repo-execution-profile.json`

Test levels: L0, L1.

### `soc-0qz1.4` - Fix Codex plugin hook packaging and dangerous-git parsing

Ownership: `hooks/dangerous-git-guard.sh`,
`scripts/install-codex-plugin.sh`, `tests/scripts/test-codex-plugin-install.sh`,
`tests/hooks/hook-stdin-contracts.bats`.

Acceptance:

- Installed plugin cache preserves `../lib/hook-helpers.sh` for hooks or hooks
  fail open with structured diagnostics when helpers are missing.
- `git push origin branch-with-f` and similar safe ref names are allowed.
- `git push -f` and `git push --force` are blocked.
- `git push --force-with-lease` remains allowed.

Validation:

- `bash tests/scripts/test-codex-plugin-install.sh`
- `bats tests/hooks/hook-stdin-contracts.bats`
- `bash scripts/validate-hook-preflight.sh`

Test levels: L1, L2, L3 for installed-cache smoke.

### `soc-0qz1.6` - Scope release audit artifact validation to relevant artifacts

Ownership: `scripts/validate-release-audit-artifacts.sh`,
`scripts/pre-push-gate.sh`, `tests/scripts/release-artifacts.bats`.

Acceptance:

- Validator has explicit changed/pre-push, target release, and strict-all modes.
- Local historical artifact dirs missing from older worktrees do not block
  unrelated pushes.
- Official release mode remains strict for the target release.

Validation:

- `bats tests/scripts/release-artifacts.bats`
- `scripts/pre-push-gate.sh --fast --scope worktree`

Test levels: L1, L2.

### `soc-0qz1.3` - Make validation hermetic and non-mutating by default

Ownership: `scripts/pre-push-gate.sh`, `scripts/ci-local-release.sh`,
`scripts/release-smoke-test.sh`, `scripts/check-agents-hash-snapshot.sh`,
validation fixtures.

Acceptance:

- Fast validation leaves tracked `.agents` knowledge hashes stable by default.
- Release artifact production is explicit and logged.
- Existing branch `codex/soc-ff2p-rpi-release-audit` is integrated or explicitly
  superseded.

Validation:

- `scripts/pre-push-gate.sh --fast --scope worktree`
- targeted no-mutation fixtures added by the implementation slice

Test levels: L1, L2.

### `soc-0qz1.5` - Classify canonical root cleanliness and worktree disposition blockers

Ownership: `scripts/check-worktree-disposition.sh`,
`tests/scripts/test-worktree-disposition.sh`, closeout docs/runbooks.

Acceptance:

- Dirty canonical-root paths are classified as generated/ignored,
  tracked-policy, user/operator edit, or unknown.
- `.gitignore` and tracked policy changes are never hidden as generated churn.
- `soc-ff2p.4` can be closed or narrowed after the classifier lands.

Validation:

- `bash tests/scripts/test-worktree-disposition.sh`
- `bash scripts/check-worktree-disposition.sh`

Test levels: L1, L2.

### `soc-0qz1.7` - Bound bd and Dolt closeout command behavior

Ownership: `docs/runbooks/bd-server-mode-closeout.md`, agent closeout docs,
any repo wrappers or validators that call `bd`.

Acceptance:

- bd read/write/Dolt commands in closeout guidance are wrapped with portable
  `gtimeout`/`timeout` behavior.
- Hung-after-output results preserve stdout and re-read created issue IDs when
  possible.
- No-remote and timeout outcomes are documented as distinct states.

Validation:

- `bash scripts/validate-bd-closeout-contract.sh`
- manual reproduction note for timed-out-after-JSON command behavior

Test levels: L0, L1.

## Execution Order

Wave 1:

- `soc-0qz1.2`
- `soc-0qz1.8`

Wave 2:

- `soc-0qz1.4`
- `soc-0qz1.6`

Wave 3:

- `soc-0qz1.3`
- `soc-0qz1.5`
- `soc-0qz1.7`

## File Dependency Matrix

| File | Issues | Serialization |
| --- | --- | --- |
| `docs/contracts/agents-write-surfaces.md` | `.2`, `.3`, `.5` | Policy first, then mutation and root-disposition consumers. |
| `docs/contracts/repo-execution-profile.json` | `.8`, `.3` | Lane metadata before broad validation changes. |
| `scripts/pre-push-gate.sh` | `.6`, `.3` | Scope release audit check before broad no-mutation changes. |
| `scripts/ci-local-release.sh` | `.3` | Single writer. |
| `hooks/dangerous-git-guard.sh` | `.4` | Single writer. |
| `scripts/install-codex-plugin.sh` | `.4` | Single writer. |
| `scripts/check-worktree-disposition.sh` | `.5` | After `.agents` policy vocabulary is stable. |
| `scripts/validate-release-audit-artifacts.sh` | `.6` | Independent narrow bug fix. |
| `docs/runbooks/bd-server-mode-closeout.md` | `.7` | Independent docs plus wrapper guidance. |

## Ready First Slice

Recommended first implementation issue: `soc-0qz1.4`.

Reason: it is concrete, high-confidence, user-visible, and independent of the
broader `.agents` policy vocabulary. It should be small enough for one focused
implementation and will immediately remove the installed hook mismatch that can
block normal push workflows.

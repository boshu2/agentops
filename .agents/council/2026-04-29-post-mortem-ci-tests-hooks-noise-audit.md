---
id: post-mortem-2026-04-29-ci-tests-hooks-noise-audit
type: post-mortem
date: 2026-04-29
epic: agentops-dtg
branch: rpi/agentops-dtg-ci-noise-audit
head: 8a18bfa9
mode: inline-council
---
# Post-Mortem: CI Tests and Hooks Noise Audit

> RPI streak: unavailable | Sessions: unavailable | Last verdict: WARN

## Council Verdict: WARN

The implementation satisfies the discovery plan and closes all five child beads.
The WARN is not about shipped behavior; it is about environmental closeout
readiness. The broad fast release gate passed every functional check but still
failed on worktree disposition because the canonical root has unrelated active
Olympus extraction work.

## Retrospective Perspectives

| Perspective | Verdict | Notes |
|---|---|---|
| Plan compliance | PASS | All five child issues closed. The only material scope additions were discovered during validation: release-audit scoping, virtualenv scan excludes, and headless inventory retry hardening. Each has direct proof coverage. |
| Tech debt | WARN | `scripts/check-worktree-disposition.sh` still blocks broad local release when canonical-root state is dirty outside the task branch. This is intentional policy pressure, but it prevents a fully green release-gate transcript for this branch. |
| Learnings | PASS | Behavioral fixtures caught and prevented the exact drift classes identified in research: orphan workflow jobs, inventory drift, manifest-derived hooks, release-artifact scope, and live runtime inventory flake. |

## Scope Delta

- Planned `docs/contracts/validation-surface-inventory.md` was optional; the
  delivered contract is `scripts/validation-surface-inventory.json` plus
  `scripts/validate-surface-inventory.sh`.
- Added release-audit artifact scoping after `pre-push-gate.sh --fast --scope
  worktree` found a false blocker from the latest release's missing local
  `.agents/releases/local-ci` artifact directory.
- Added `.venv`, `.venv-docs`, and `venv` excludes after `ci-local-release.sh
  --fast` found third-party virtualenv secret-pattern false positives.
- Raised headless runtime inventory retries from 2 to 3 after live Claude
  inventory returned two partial skill lists in a row.

## Closure Integrity

Executable audit:

```bash
bash /home/boful/.agents/skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dtg
```

Result: PASS. Five children checked, five passed, zero warnings, zero failures.
All five resolved as `grace-window` evidence because the beads were closed before
the final commit was created and pushed.

## Metadata Verification

- Broken-link scan for changed Markdown files: PASS.
- Plan-file extraction produced expected warnings for optional or illustrative
  references: `docs/contracts/validation-surface-inventory.md`,
  `tests/hooks/*.bats`, the historical `check-cmdao-coverage-floor.sh` stub, and
  bare/example references. These are not delivered-artifact failures.
- Changed-file proof exists on branch `origin/rpi/agentops-dtg-ci-noise-audit`
  for every planned implementation surface that was selected.

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---|---|---|
| Code | PASS | CI parity, hook preflight, surface inventory, release-audit, headless-runtime, pre-push, and scan-exclude changes are present on the branch. |
| Documentation | PASS | `AGENTS.md`, `docs/CI-CD.md`, and `docs/TESTING.md` now describe the validation surface inventory and fast-gate semantics. |
| Examples | PASS | BATS fixtures exercise the new expected behavior instead of only grepping implementation structure. |
| Proof | WARN | Targeted proof is green. Broad fast release proof passed all functional checks but failed worktree disposition due unrelated canonical-root state. |

## Proof Commands

Post-mortem reran these in a detached branch worktree:

```bash
bash scripts/validate-ci-policy-parity.sh
bash scripts/validate-surface-inventory.sh
bash tests/scripts/test-headless-runtime-skills.sh
bats tests/scripts/pre-push-gate.bats tests/scripts/release-artifacts.bats tests/scripts/ci-local-release.bats
bash tests/docs/validate-doc-release.sh
```

Results:

- CI policy parity: PASS, 32 jobs, 5 non-blocking.
- Surface inventory: PASS, 32 CI jobs inventoried.
- Headless runtime mock suite: 8 PASS, 0 FAIL.
- BATS local-gate/release-artifact suite: 58 ok.
- Doc-release gate: PASS, 1592 links checked, 0 broken.

Implementation-time proof also included:

- `bash scripts/pre-push-gate.sh --fast --scope upstream`: PASS.
- Isolated `scripts/ci-local-release.sh --fast --jobs 4`: one failure only,
  worktree disposition; all functional checks, headless runtime, release smoke,
  and release binary validation passed.

## Prediction Accuracy

| Prediction | Score | Outcome |
|---|---|---|
| Inventory can become new noise unless paired with a consuming validator | HIT | Inventory landed with `validate-surface-inventory.sh` and drift fixtures. |
| Hook preflight expansion can over-block without allowlist/report-first behavior | HIT | Implementation used documented exceptions and manifest-derived checks instead of blocking all utility scripts. |
| `ci-local-release.sh --fast` belongs in the final validation wave, not early waves | HIT | It was run only during final validation and exposed unrelated disposition state after targeted gates had already passed. |

## Test Pyramid Assessment

| Issue | Planned | Actual | Gaps | Action |
|---|---|---|---|---|
| agentops-dtg.1 | L0/L2 parity script fixtures | `test-ci-policy-parity.sh`, `validate-ci-policy-parity.sh` | None | None |
| agentops-dtg.2 | L0/L1/L2 inventory and local-gate checks | `validate-surface-inventory.sh`, shell fixtures, BATS local-gate fixtures | None | None |
| agentops-dtg.3 | L0/L1 hook contract checks | manifest-derived preflight, orphan hook shell/BATS tests | None | None |
| agentops-dtg.4 | L1 behavioral local-gate fixtures | pre-push and local-release BATS behavior tests | None | None |
| agentops-dtg.5 | L2/L3 closeout gates | targeted gates, pre-push upstream, isolated fast release gate | External disposition caveat only | Track outside this epic |

## Learnings

1. Tooling-only changes should not trigger validators that depend on another
   session's local artifact directories. Scope those validators to changed
   durable inputs and use fixture suites for tooling behavior.
2. Live runtime inventory checks can return repeated partial outputs. A small
   retry budget plus mocked failure fixtures reduces advisory noise without
   hiding real missing-skill failures.
3. For committed branches, `pre-push-gate.sh --fast --scope upstream` is the
   meaningful push-readiness scope. `--scope worktree` is useful before commit,
   but can legitimately skip everything after the worktree is clean.

## Follow-Up

No new dtg-specific follow-up bead is needed. The remaining blocker is existing
worktree-disposition state outside this epic, already represented by open
worktree/disposition work and the active Olympus extraction line.

## Flywheel

Flywheel stable -- no new `agentops-dtg` follow-up items harvested.

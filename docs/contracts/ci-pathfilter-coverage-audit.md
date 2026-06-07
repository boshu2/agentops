# CI Path-Filter / Gate-Target Coverage Audit

> **Bead:** ag-g9ex · **Bounded context:** BC2-Validation · **Derived from:** `.agents/retro/2026-05-31-bead-crank-postmortem.md` §4(#2), §7(#5).

## The invariant being audited

**A CI gate that reads a specific set of files MUST be triggered by a change-filter that covers the union of those files.**

When it isn't, an edit to a guarded file *skips* the gate that guards it. The `dorny/paths-filter` step in the `changes` job computes per-area booleans (`go`, `docs`, `contracts`, `goals`, …); every downstream job/step gates on those booleans. Because `summary` (a required branch-protection check) passes as long as the *triggered* gates pass, a skipped gate produces **green-but-incomplete** — the regression reaches `main` silently and surfaces only on a later, unrelated PR whose filter happens to re-trigger the gate.

This is exactly the #634/#638 failure: the docs-thinning commit `bbfd278e` collapsed the source-of-truth-precedence section of `AGENTS.md` to a pointer, breaking the security-suite redteam prompt-injection-precedence canary. That commit's diff touched only `AGENTS.md`, which the `contracts` filter did **not** cover at the time, so the `contracts-sync` canary never ran on its merge. It was caught reactively on PR #631 and fixed in #634 (`d609fecb`) + #638 (`29075031`, which extended `contracts` to a superset of the redteam-pack target globs and added the `goals` filter).

## The path-filters (as of this audit)

Defined in `.github/workflows/validate.yml`, `changes.steps[id=filter].with.filters`:

| Filter | Globs |
|---|---|
| `go` | `cli/**`, `go.mod`, `go.sum`, `tests/windows/**` |
| `skills` | `skills/**`, `skills-codex/**`, `skills-codex-overrides/**`, `tests/skills/**` |
| `hooks` | `lib/**`, `cli/embedded/**` |
| `docs` | `docs/**`, `README.md`, `CHANGELOG.md`, `PRODUCT.md`, `SKILL-TIERS.md` |
| `eval` | `evals/**`, `cli/internal/eval/**`, `cli/cmd/ao/eval*`, `schemas/eval-*` |
| `codex` | `skills-codex/**`, `skills-codex-overrides/**` |
| `shell` | `**/*.sh`, `scripts/**` |
| `bats` | `**/*.bats` |
| `ci` | `.github/**` |
| `contracts` | `schemas/**`, `docs/contracts/**`, `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/CI-CD.md`, `docs/strategic-direction.md`, `docs/standards/shell-script-standards.md`, `skills/security/**` |
| `goals` | `GOALS.md`, `spec/scenarios/**`, `docs/adr/ADR-0003*` |
| `learning` | `.agents/learnings/**` |
| `markdown` | `**/*.md` |

`ci` is the universal escape hatch: any `.github/**` change (which includes `validate.yml` itself) sets every job's effective trigger because the workflow is the thing being changed.

## Findings table

For each gate that reads specific files, the **trigger** column is the union of filter booleans in the gate step's (or job's) `if:`; **COVERED** means every file the gate reads matches at least one of those filters.

| Gate (job → step) | Files it reads | Trigger filters | Verdict |
|---|---|---|---|
| `contracts-sync` → Run official AgentOps contract canaries | redteam-pack target globs: `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/CI-CD.md`, `docs/strategic-direction.md`, `docs/standards/shell-script-standards.md`, `skills/security/SKILL.md` | `ci, contracts, go, skills` | COVERED (closed by #638 — `contracts` is now a superset; guarded by `tests/scripts/test-pathfilter-gate-coverage.bats`) |
| `doctrine-proof` → F1/F2 executable-spec link e2e | `GOALS.md`, `spec/scenarios/**`, `.feature` files | `ci, docs, go, goals` | COVERED (`goals` added by #638) |
| `doctrine-proof` → Scenario→test linkage | `skills/*/references/*.feature`, `spec/scenarios/**` | `ci, goals, shell, skills` | COVERED |
| `doctrine-proof` → Validate AGENTS.md tiered-split contract | `AGENTS.md`, `AGENTS-WORKFLOW.md`, `AGENTS-CI.md`, `AGENTS-CODEX.md`, `AGENTS-RUNTIME.md` | `ci, docs, shell` | **GAP → FIXED** (this PR) |
| `doctrine-proof` → Verify all scripts/skills/hooks are wired (wiring-closure) | `scripts/*.sh`, `GOALS.md`, `GOALS.yaml`, `.github/workflows/`, `tests/`, `skills/SKILL-TIERS.md` | `ci, go, hooks, shell` | **GAP → FIXED** (this PR) |
| `doctrine-proof` → Validate sovereignty-proof citations | `docs/sovereignty-proof/**` + every cited `file:line` (repo-wide) | `ci, contracts, docs, go, hooks, shell, skills` | COVERED (broad trigger; `docs/**` covers the proof dir) |
| `doctrine-proof` → finding-registry contract | `.agents/findings/registry.jsonl`, `docs/contracts/finding-registry.{md,schema.json}` | `ci, contracts` | COVERED (`contracts` covers `docs/contracts/**`; `.agents/findings/` is runtime state, not a PR-editable contract source) |
| `doctrine-proof` → three-gap super-gates | `.agents/{council,defrag,overnight}/**` | `ci, docs, go, hooks, skills` | COVERED (operates on derived runtime state, not PR-editable source) |
| `codex-parity` (job) → all codex validators | `skills-codex/**`, `skills-codex-overrides/**` | `ci, codex, skills` (job-level) | COVERED |
| `contracts-sync` → registry / SKU / context-map / domain-map drift | generated artifacts + `skills/**` | `ci, contracts, …, skills` | COVERED (`skills` covers SKILL.md edits that mutate the artifacts) |
| `contracts-sync` → CI policy parity (golden) | `.github/workflows/validate.yml`, `docs/contracts/ci-jobs.yaml` | `ci, contracts, docs, shell` | COVERED (`ci` covers the workflow; `contracts` covers the yaml) |
| `security` → security toolchain gate | `cli/**`, `**/*.sh` | `ci, go, shell` | COVERED |
| `correctness` → bats / go / smoke / integration | `cli/**`, `tests/**`, `**/*.sh`, `**/*.bats` | `ci, go, hooks, shell, skills, docs, bats` | COVERED |

### Non-findings (verified, not gaps)

- `check-corpus-freshness.sh` references `GOALS.md` only in a comment, not as a read — its actual input is the operator-side corpus snapshot dir (skipped on CI). No coverage requirement.
- `check-provenance-orphans.sh` contains the literal string `GOALS.md` inside a test-fixture JSON record (`evidence` field), not a file read.
- `check-memrl-health.sh` / `validate-next-work-contract-parity.sh` read `.agents/**` runtime ledgers and `docs/contracts/**` / `skills/**` sources that are all covered by `contracts`/`skills` when PR-editable.

## Gaps fixed in this PR

### Gap 1 — AGENTS tiered-split siblings uncovered (the direct #634 class)

`scripts/validate-agents-split.sh` validates `AGENTS.md` **and** the four siblings `AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md` (line-count cap, existence, bidirectional links). The gate step triggers on `docs || ci || shell`. `AGENTS.md` is covered by the `contracts` filter (post-#638) but `contracts` was not in the trigger, and the four `AGENTS-*.md` siblings are covered by **no** filter except `markdown` (which no gate consumes). So an edit to `AGENTS-WORKFLOW.md` alone — e.g. a future docs-thinning that breaks the bidirectional link or blows the size cap — would skip the very gate that enforces the split. This is the identical mechanism that disabled the security canary in `bbfd278e`.

**Fix:** add the four `AGENTS-*.md` siblings to the `contracts` filter (they are operator-contract source, sibling to `AGENTS.md` which is already there), and add `|| needs.changes.outputs.contracts == 'true'` to the agents-split gate step's trigger.

### Gap 2 — wiring-closure not re-triggered by GOALS.md de-wire

`scripts/check-wiring-closure.sh` asserts every `scripts/check-*.sh` is referenced somewhere in `GOALS.md`, `GOALS.yaml`, `.github/workflows/`, `tests/`, or `scripts/`. The gate triggers on `go || hooks || shell || ci`. The scripts it scans are covered by `shell`, but `GOALS.md`/`GOALS.yaml` (a primary reference source) are covered only by `goals`. A GOALS.md-only edit that removes the last reference to a gate (de-wiring it) would skip wiring-closure.

**Fix:** add `|| needs.changes.outputs.goals == 'true'` to the wiring-closure gate step's trigger so a GOALS.md/spec edit re-runs the wiring check.

## Governance: `--admin` self-merge of self-authored PRs (§7 #4)

The post-mortem §4(#2) found that `--admin` squash-merges composed with the `claude-review` usage-limit soft-fail leave **no live review gate** on self-authored autonomous PRs — the orchestrator merged ~30 of its own PRs where the only required automation gate could skip itself and `--admin` overrode the rest, never applying the `author != validator` invariant it was shipping.

A clean CI gate for this is **out of scope** here: GitHub Actions cannot reliably observe whether a merge used `--admin` from inside the `pull_request` workflow (the override happens at the REST merge call, not in a workflow-visible event), and `summary` already encodes the required-check set. Wiring a half-working gate would be over-engineering against the bead's explicit "do NOT over-engineer" guidance.

**Policy (documented here; mechanical enforcement deferred to a follow-up bead):**

1. An `--admin` squash-merge of a **self-authored** PR (PR author == the merging operator/agent) SHOULD carry a second automation signal before merge: either (a) a green `claude-code-review` verdict that did NOT soft-fail on usage-limit, or (b) an explicit human spot-check recorded as a provenance ledger event (`claude-code-review` verdicts are first-class ledger events per AGENTS.md → Provenance).
2. When neither is available (e.g. weekly Claude limit exhausted during an autonomous run), the merge is permitted but the orchestrator MUST record the gap as bead/provenance evidence and flag the bead for a post-hoc human review pass, so the gap is auditable rather than silent.
3. The autonomous loop's post-mortem checkpoint (the sibling crank/evolve skill gate, ag-7gmv) is the human-in-the-loop counterweight: ≥5 self-merged PRs in one session forces the checkpoint.

**Follow-up:** a mechanical signal (e.g. a `merge`-event workflow that records merge-method + author parity into the provenance ledger, or a branch-protection rule requiring `claude-code-review` to be a non-soft-fail success) is filed as a follow-up bead rather than shipped half-built here.

## Maintaining this invariant

The drift-blocking test `tests/scripts/test-pathfilter-gate-coverage.bats` asserts the redteam-pack ⊂ `contracts`, the `goals` filter wiring, **and** (added in this PR) that the AGENTS tiered-split gate is triggered by a filter covering all five `AGENTS*.md` files. When you add a gate that reads a new file, extend that bats suite with the gate→files→filter assertion rather than relying on a later PR to surface the gap.

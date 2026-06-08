---
name: ship-loop
description: 'Bot-paired fast-lane cycle for coherent-arc internal PRs (one closable bead or small-epic slice): claim → test → impl → pre-push → push → squash auto-merge → close. Use when shipping a fast-lane internal PR, landing a small fix, or closing a single harvested bead.'
practices:
- continuous-delivery
- xp
- tdd
- bdd
- pragmatic-programmer
hexagonal_role: driving-adapter
consumes:
- beads
- rpi
- post-mortem
produces:
- git-changes
- merged-prs
context_rel:
- kind: customer-of
  with: rpi
- kind: customer-of
  with: post-mortem
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: execution
  dependencies:
  - beads
  - rpi
  stability: experimental
  triggers:
  - ship loop
  - ship this
  - fast lane PR
  - land a small fix
  - bot-paired PR
  - close a harvested item
output_contract: merged PR on origin/main + closed bead
---

# /ship-loop — Bot-paired fast lane PR cycle

> **Lane choice:** use this skill for **coherent-arc internal PRs**.
> Pick one closable bead, or one small-epic slice of one surface, with paired tests.
> The PR is the *atomic-revert unit*: bundle scenarios that ship or revert together.
> Split scenarios with independent rollback.
> For fork-based OSS contributions, use the `/pr-*` family.
> For large epics or multi-wave work, use `/crank`.
> See `CLAUDE.md ## Workflow` for the canonical unit-of-PR rule.

Capture of the discipline that landed 8/9 internal PRs in the 2026-05-18 session.
Median time-to-merge was 19.5 minutes.
Five named failure modes were found; four closed mechanically.
The rationale lives in the 2026-05-18 workflow synthesis note under `docs/learnings`.

## Overview / When to Use

Run this skill at the START of each PR you intend to ship to your own `main` branch. The skill enforces the cycle as a sequence; each step has a clear done-state and gate.

**Pair partner:** `claude-review` (GitHub App workflow at `.github/workflows/claude.yml`) auto-fires on `pull_request: opened/synchronize`. No `@claude` mention is required. Operator does the edits; bot does the review check.

## The 9-step cycle

1. **Claim.** `bd ready` → pick the highest-severity unblocked item, OR read `.agents/rpi/next-work.jsonl` for harvested follow-ups. **`bd update <id> --claim`** atomically.
2. **Branch off fresh main.** `git checkout main && git pull --rebase`. Then `git checkout -b <type>/<slug>-<bead-id>`. NEVER stack off a sibling branch; auto-merge handles serialization via update-branch.
3. **Write the FIRST FAILING TEST.** BDD scenario (Gherkin) for behavior; unit test for invariants. The test must fail for the *right reason* (asserting expected behavior, not just "doesn't crash"). See [references/test-shape.md](references/test-shape.md).
4. **Minimal implementation.** Smallest code change that makes the test green. Resist scope creep. Refer to the project's standards (`.claude/rules/{go,python}.md`).
5. **`scripts/ship.sh`** (recommended).
   It detects inventory-touching changes and runs the regen sweep preemptively.
   That is the mechanical fix for anti-pattern #1.
   CI is the sole authoritative push gate.
   The old local mirror was retired because drift cost dominated per-push wait.
   For per-tool sanity before push, run only what your diff touches:
   `cd cli && make test`, targeted Bats tests, or codex hash regeneration.
   If unchanged-from-base content blocks CI, file an atomic side-quest PR first.
   For gate, validator, or CI changes, capture the target output line in the PR body as `Evidence:`.
   The evidence-claim CI job verifies that line against workflow logs.
6. **Commit with conventional-commit scope.** `feat(<scope>):`, `fix(<scope>):`, `docs(<scope>):`. Body explains the failure mode the test reproduces and how the fix removes it.
7. **Push + `gh pr create`.** Body cites the bead, the validation results, and links to the learning anchor in the script body (NOT a `.agents/learnings/` file existence — that breaks in CI's fresh clone).
8. **`gh pr merge <num> --squash --auto`.** Immediately. The bot fires `claude-review` automatically on PR open. When all required checks pass, merge fires without operator action.
9. **Close the bead.** `bd close <id> --reason "Merged via PR #<num>"`. The coherent-arc rule should keep concurrent PR count low (typically 1-2); when a large-epic split puts multiple PRs in flight against the same main, serialize them by waiting for each predecessor to merge, then call `gh api repos/<owner>/<repo>/pulls/<num>/update-branch -X PUT` on each BEHIND successor.

## Gate sequence (what each enforces)

| Gate | Enforces |
|---|---|
| Per-tool local checks (optional) | `cd cli && make test` for Go; `bats tests/scripts/<file>.bats` for shell; `scripts/regen-codex-hashes.sh` for codex parity. Run only what your diff touches. |
| `claude-review` (auto on PR open) | Reviewer pair — the bot half |
| `.github/workflows/validate.yml` | **Sole authoritative push gate** (soc-g2r9, PR #357). 60+ job suite on PR head: cli-docs-parity, embedded-sync, skill-frontmatter, registry-check, security-toolchain, validate-pr-evidence-claims (AP#7), plus the F-mode closures |
| `gh pr merge --squash --auto` | Auto-merge when all required checks pass |
| Manual update-branch fallback | Chain N PRs by enabling auto-merge, waiting for each predecessor, then calling `gh api repos/<owner>/<repo>/pulls/<num>/update-branch -X PUT` on BEHIND successors (closes F3) |

## Failure-mode mapping

- **F1:** Run ShellCheck after shell edits.
- **F2:** Split unrelated blockers into their own PRs.
- **F3:** Update BEHIND successor branches by API.
- **F4:** Keep review-bot docs aligned with workflow reality.
- **F5:** Expire stale evolve stop files.
- **meta:** Assert durable text, not local files.

## Anti-patterns

Read [references/anti-patterns.md](references/anti-patterns.md) for the full list with examples. Headline anti-patterns:

1. **Running `--fast` pre-push on an inventory-touching PR** — new skill, contract, or schema → use FULL gate; `--fast` skips ~15 inventory validators
2. **Bundling pre-existing fixes** — file each as its own atomic PR
3. **Keeping copied variables after a rewrite** — after a script rewrite, the first self-check is "are all variable declarations used?"
4. **Asserting local-only state in CI tests** — grep the reference, don't check the file
5. **Branches off out-of-date main** — `git checkout main && git pull --rebase` at branch creation
6. **Skipping the failing-test-first step** — adding a test after the fix gives false confidence

## Session scope (sister rule to coherent-arc)

Coherent-arc governs the *shape* of a single PR; session-scope governs the *count* of consecutive PRs in an autonomous session.

- **Default: 2-4 PRs per autonomous session.** Both arcs ship cleanly and merge.
- **≥5 PRs in flight or merged in one session triggers a mandatory post-mortem before continuing.** Diminishing returns and reactive-PR spirals (PR-fixes-fallout-from-prior-PR) are the dominant failure mode in the back-half of long sessions.
- **Post-mortem shape (1-2 sentences each):** Which PRs were planned vs reactive? How many self-corrections? Was the marginal PR discovery or churn?

**Derivation:** the 2026-05-19 cron-loop session shipped 6 PRs with 3 self-corrections.
PRs #5-#6 fixed fallout from PRs #1-#3.
Visible reactivity began by PR #5.
The cron loop kept nudging "keep going" without surfacing the post-mortem signal.
Mechanical enforcement is the mandatory `/evolve` post-mortem checkpoint.
That checkpoint reads the count from `scripts/session-pr-scope.sh`.
The old pre-creation Bash hook was removed in the 3.0 hookless teardown.
Re-author that check as an opt-in hook via the hooks-authoring skill.

## Pair mechanics (claude-review)

- `claude-review` fires automatically on `pull_request: opened` and `synchronize`. No `@claude` mention required.
- If `claude-review` is `IN_PROGRESS`, wait — don't poke. The bot does NOT respond to its own comments (anti-loop protection).
- If `claude-review` is silent after PR open, the workflow may need permission upgrades (see `docs/contracts/claude-bot-delegation.md` Gotchas 1-4) — surface to operator, do not retry.
- If you hit the self-revert loop (PR #270 case — bot reverting its own forward-port of `claude.yml`), rebase the branch locally onto fresh main and force-push.

## Examples

**Closing a harvested next-work item:**

```
1. /post-mortem ran; .agents/rpi/next-work.jsonl has an unclaimed "medium" item
2. /ship-loop picks the item: branch fix/<slug>-<bead> off main
3. Write the failing test that proves the failure mode exists
4. Add the minimal fix
5. Pre-push --fast → green
6. Push → gh pr create → gh pr merge --squash --auto
7. claude-review auto-runs; validate.yml runs; auto-merge fires
8. bd close <id>
```

**Shipping a chain of PRs:**

```
1-9. Run the cycle for each PR (off main, not stacked)
10. After all PRs are open with auto-merge enabled, wait for the predecessor PR to merge.
11. If a successor becomes BEHIND, run `gh api repos/<owner>/<repo>/pulls/<num>/update-branch -X PUT`; repeat until the chain is merged.
```

See [references/examples.md](references/examples.md) for full walkthroughs.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Auto-merge stalls | `claude-review` IN_PROGRESS or branch BEHIND | Wait for review; if BEHIND, `gh api repos/<o>/<r>/pulls/<n>/update-branch -X PUT` |
| `claude-review` never fires | Workflow lacks trigger or perms | Check `.github/workflows/claude.yml` `on:` block and permissions; may require `workflows: write` upgrade |
| Pre-push --fast blocks on unchanged content | Pre-existing F2-class blocker | File the fix as an atomic side-quest PR first; rebase your branch onto the side-quest's merge |
| Self-revert loop on a stale branch | Bot reverting its own forward-port | Rebase locally onto fresh main; force-push with `--force-with-lease` |
| Test asserts local file in CI | `.agents/` is gitignored | Change to `grep -q '<slug>' "$SCRIPT"` (reference assertion, not file existence) |

## See Also

- [pr-implement](../pr-implement/SKILL.md) — fork-based OSS contribution (different tier; different use case)
- [crank](../crank/SKILL.md) — multi-wave epic execution
- [rpi](../rpi/SKILL.md) — full lifecycle orchestrator (ship-loop is the per-PR mechanics inside RPI's implementation phase)
- [post-mortem](../post-mortem/SKILL.md) — harvests next-work items that ship-loop consumes
- [beads](../beads/SKILL.md) — task tracker that drives the claim step

## References

- [references/anti-patterns.md](references/anti-patterns.md)
- [references/examples.md](references/examples.md)
- [references/gh-merge-chain.md](references/gh-merge-chain.md)
- [references/test-shape.md](references/test-shape.md)
- Durable rationale: the 2026-05-18 workflow synthesis note under `docs/learnings`.

## Reference Documents

- [references/ship-loop.feature](references/ship-loop.feature) — Executable spec: claim→test→impl→push→squash-merge→close, one coherent arc, gated merge + bead close (soc-qk4b)

# /ship-loop walkthroughs

Three concrete walkthroughs from the 2026-05-18 session showing the cycle in action.

## Example 1: harvested-item closeout (cycle 200, PR #326)

**Trigger:** `/post-mortem` of the merge-arc produced a harvested item in `.agents/rpi/next-work.jsonl`: "Run shellcheck on staged *.sh in pre-push gate unconditionally" (medium severity, closes F1).

```bash
# 1. Claim
bd update soc-xkk2 --claim

# 2. Branch off fresh main
git checkout main && git pull --rebase
git checkout -b fix/shellcheck-unconditional-soc-xkk2

# 3. First failing test
# Wrote tests/scripts/pre-push-shellcheck-unconditional.bats with 6 structural assertions:
# - "step 30 NOT wrapped in `if needs_check shell`"
# - "step 30 consults `git diff --cached`"
# - "shellcheck -S warning severity preserved"
# - etc.
bats tests/scripts/pre-push-shellcheck-unconditional.bats  # → fails

# 4. Minimal implementation
# Edited scripts/pre-push-gate.sh step 30: dropped the needs_check gate,
# collected .sh from staged + worktree + all_changed, shellcheck the union.
bats tests/scripts/pre-push-shellcheck-unconditional.bats  # → 6/6 PASS

# 5. Pre-push --fast
bash scripts/pre-push-gate.sh --fast  # → passed

# 6. Commit
git add -A
git commit -m "fix(pre-push-gate): shellcheck staged *.sh unconditionally (soc-xkk2)"

# 7. Push + PR
git push -u origin fix/shellcheck-unconditional-soc-xkk2
gh pr create --title "fix(pre-push-gate): ..." --body "..."

# 8. Auto-merge
gh pr merge 326 --squash --auto

# 9. Close bead
# (bd close after the PR auto-merges — happens in the next cycle's status sweep)
```

**Total wall-clock: ~22 minutes. claude-review auto-fired on PR open; no @claude mention.**

## Example 2: chain of 3 in flight (cycles 201-203)

After /post-mortem, 3 harvested items each became a PR:
- PR #327 (claude-bot-delegation Gotcha 4 doc fix)
- PR #328 (evolve KILL/STOP auto-expire)
- PR #329 (gh-merge-chain helper)

```bash
# Cycle each through the 9-step /ship-loop (off main, not stacked).
# After all 3 are open with auto-merge enabled:

scripts/gh-merge-chain.sh --poll-interval 30 326 328 329

# Helper:
# - enables auto-merge on each (idempotent)
# - polls every 30s
# - when any PR merges, calls update-branch on the rest
# - exits 0 when all 3 merged
```

**Operator attention during phase 2: zero.** Helper runs to completion in ~20 min wall-clock with bot review + auto-merge handling the rest.

## Example 3: a self-inflicted regression (PR #325)

Cycle 199 was the cleanup for an F1-class regression that landed in cycle 198 (PR #322 left dead `REPO_ROOT`):

```bash
# When pre-push BLOCKED on shellcheck for a docs-only branch
# (because of the unrelated SC2034 in validate-ci-policy-parity.sh
#  from the just-merged PR #322):

# Option A: file the side-quest fix in a separate atomic PR first
#   (the "anti-pattern-avoiding" path; see references/anti-patterns.md)

# Option B: include the side-quest inline in the current PR
#   (the path actually taken — bundles fixes, but unblocks the current
#    branch immediately; ate the cost this time)

# Both are valid for one-line script fixes; the rule is to NAME it in
# the commit body so the discipline stays honest.
```

PR #325 commit body included:
> "Two micro-fixes bundled — shellcheck regression from just-merged #322 was blocking pre-push on the docs change."

The full F1 mechanical closure (PR #326) followed in the next cycle. That's how the discipline tightens: F1 ate 90 minutes of cascade in the doctor-engine arc (#281 → #282-#286); after the post-mortem named it, the next instance (#322) cost only 25 minutes (#325 + #326).

## When the cycle doesn't fit

If during step 3 you realize the failing test can't be expressed as a single scenario — STOP. The work is slow-lane shaped. Options:

1. Decompose into multiple beads, each its own /ship-loop cycle
2. Use `/crank` if the work is an epic with waves
3. Surface to operator for explicit lane choice

Don't force a multi-scenario change through ship-loop. That's how the doctor-engine cascade (#281 → 5 cleanup PRs) happens.

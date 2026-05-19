# Using gh-merge-chain.sh

`scripts/gh-merge-chain.sh` (added in PR #329, `soc-a5y5`) automates the F3 case: `gh pr merge --squash --auto` does NOT auto-rebase when main moves underneath an N-PR chain. Without the helper, the operator must call `gh api repos/.../pulls/<n>/update-branch -X PUT` once per merge for each successor.

## Usage

```bash
scripts/gh-merge-chain.sh 321 322 323 324 325 326
scripts/gh-merge-chain.sh --dry-run 321 322 323
scripts/gh-merge-chain.sh --poll-interval 30 --max-wait 1800 321 322 323
scripts/gh-merge-chain.sh --merge-method merge 321 322
```

## What it does

**Phase 1 (idempotent):** Enable auto-merge on every PR with `gh pr merge <n> --squash --auto`. No-op if already set.

**Phase 2 (polling loop):** Poll each PR's state every `POLL_INTERVAL` (default 20s):
- `MERGED` → log it, remove from remaining list
- `OPEN` → keep tracking
- `CLOSED` → exit 1 (treated as failure)

When any PR transitions MERGED, call `gh api repos/<o>/<r>/pulls/<n>/update-branch -X PUT` on all remaining successors. They go from BEHIND to up-to-date, fresh CI runs, auto-merge fires when checks pass.

Loop until all PRs merge OR `MAX_WAIT` (default 3600s) hits.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All PRs merged |
| 1 | A PR closed without merge OR fatal error |
| 2 | Usage error OR --max-wait reached with PRs still unmerged |

## When to invoke

- After running /ship-loop multiple times in a session and you have a chain of ≥3 PRs in flight, all targeting `main`
- After /post-mortem produces a batch of harvested items that each become a PR
- When you've manually opened a chain and want fire-and-forget completion

## When NOT to invoke

- Single PR — just `gh pr merge --squash --auto` directly
- PRs targeting different base branches
- PRs that need human review (the helper only handles BEHIND state, not review approval)

## Operational notes

- The helper polls via `gh pr view`, which uses your local GitHub auth. Token must have `repo` scope.
- `claude-review` must be auto-firing on the repo for `--auto` to work; see `docs/contracts/claude-bot-delegation.md`.
- Tests live at `tests/scripts/gh-merge-chain.bats` (7 tests, structural-property assertions).

# age-58o — push-to-main pawl gate wired

**Bead:** age-58o  
**Date:** 2026-06-17  
**Mechanism:** `scripts/check-pawl-pre-push.sh` invoked from `scripts/hooks/pre-push.local` after `ao gate check --fast` passes.

## What changed

- Cross-family pawl verdict check now runs on **push-to-main** (not only PR merge via `reconcile-pr.sh`).
- Pre-push hook stdin supplies `refs/heads/main` + landing SHA; script calls `pawl-verdict.sh check <bead> 0 --head <sha>`.
- Bead id extracted from commit message `(age-xxx)` / `(ag-xxx)` trailer.
- `#trivial` chore commits waived; `AGENTOPS_PREPUSH_SKIP_PAWL=1` escape hatch.

## Proof

```bash
bats tests/scripts/check-pawl-pre-push.bats
```

## Live path

```text
git push origin main
  → .git/hooks/pre-push.local
  → ao gate check --fast
  → check-pawl-pre-push.sh (stdin)
  → pawl-verdict.sh check (pr=0)
```

# Worktree Disposition Audit — 2026-06-16 (Wave D)

> **Status:** dry-run only — no `--apply`, no force deletes. Human sign-off required before any removal.

## Summary (gc-stale-worktrees dry-run)

Command: `bash scripts/gc-stale-worktrees.sh --json` (ref: `origin/main`, apply: false)

```json
{"ref":"origin/main","apply":false,"removed":27,"kept":140,"skipped":30}
```

| Bucket | Count | Meaning |
|--------|------:|---------|
| Would remove | 27 | Merged-branch worktrees eligible for gc |
| Would keep | 140 | Active or not yet merge-eligible |
| Skipped | 30 | Dirty, detached HEAD, or reservation blockers |

## Skip reasons observed (sample)

| Path | Reason |
|------|--------|
| `agentops-worktrees/postmerge-840-281d46d34` | detached HEAD |
| `agentops-worktrees/z1s5-v2` | uncommitted changes |
| `agentops-wt/wt-ag-ez7y6` | uncommitted changes |
| `agentops/wt-ag-s43tg-gate-rehearsal` | detached HEAD |
| `agentops/wt-ledger` | uncommitted changes |

## Disposition rules (pre-registered)

| Rule | Action |
|------|--------|
| Kill | Any delete-candidate with dirty status or active Agent Mail reservation |
| Gate | Per-candidate JSON lines in this audit — not aggregate counts alone |
| Redirect | Export-only; **no `--apply` without human ACK** |

## Next steps (operator)

1. Review dirty/detached skip list — assign owners or quarantine beads.
2. Re-run `bash scripts/gc-stale-worktrees.sh` after owner ACK for clean merged lanes.
3. Only then: `bash scripts/gc-stale-worktrees.sh --apply` (blocked in CI by design).

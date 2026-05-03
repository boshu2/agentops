# Release Worktree Disposition - 2026-05-03

Related beads: `soc-0qz1.5`, `soc-ff2p.4`

## Gate Categories

`bash scripts/check-worktree-disposition.sh` separates canonical-root dirt into:

- generated/ignored paths: runtime or local synthesis churn such as `.agents/*`
  and `wiki/*` that does not block by itself;
- generated/gate-managed paths: generated files that should be refreshed with
  their matching generator or restored;
- tracked-policy paths: repository policy and gate files, including `.gitignore`,
  `docs/contracts/*`, runbooks, and release/worktree gate scripts;
- user/operator edits: tracked work that needs an intentional commit, preserve
  branch, or revert;
- unknown paths: untracked files that need an explicit preserve, commit, ignore,
  or delete decision.

## Preserve, Commit, Or Defer

Use this decision order before release tagging:

1. Commit tracked-policy changes only when they are intentional release policy.
2. Commit user/operator edits only when they belong to the current release branch.
3. Preserve unfinished branch work on `codex/preserve-*` and add
   `docs/preserved-refs.tsv` owner and retirement-rule entries.
4. Defer unrelated unknown files by moving them out of the canonical root or
   deleting them after confirming they are disposable.
5. Restore or regenerate generated/gate-managed files so the worktree reflects
   the source-of-truth generator.

`.gitignore` changes are never hidden as generated churn. They are policy edits
and need the same intentional review as gate or contract changes.

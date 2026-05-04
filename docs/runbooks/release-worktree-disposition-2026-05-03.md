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

## Current Release Snapshot

### Current Gate Result

`bash scripts/check-worktree-disposition.sh` still fails from the task worktree
because the canonical root at `/Users/bo/dev/agentops` has uncommitted changes.

Blocking non-ignored path:

- `.gitignore` adds:
  ```gitignore
  # AgentOps session artifacts
  .agents/
  ```

Disposition: **operator decision required**. This edit changes repository ignore
policy and should be committed intentionally or reverted intentionally before
release tagging.

Runtime/state paths observed in the canonical root:

- `.agents/learnings/2026-04-07-v2.35.0-release-postmortem.md`
- `.agents/learnings/2026-04-19-orchestrator-compression-anti-pattern.md`
- `.agents/learnings/2026-04-27-ci-contract-scanners-need-syntax-parity.md`
- `.agents/learnings/2026-04-30-postmortem-task-queue-closure.md`
- `.agents/learnings/2026-04-30-pr-queue-stack-drain.md`
- `.agents/patterns/pre-tag-ci-validation.md`

Disposition: **runtime churn**. The current worktree-disposition gate ignores
`.agents/*`, but these tracked files should still be reviewed before a release
tag if they are intended to ship as knowledge-corpus updates.

### Release Rule

Do not tag v2.40.0 while this gate fails. Either:

1. commit the intentional `.gitignore` policy change and any intended `.agents`
   corpus updates;
2. revert or defer the dirty paths outside the release branch; or
3. record an explicit release-manager waiver that names each remaining dirty
   path and explains why the release may proceed.

Until one of those happens, `soc-ff2p.4` remains open.

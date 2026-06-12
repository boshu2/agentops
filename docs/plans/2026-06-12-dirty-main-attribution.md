# Dirty-main attribution + seams-epic reconciliation decision (W0.1)

owner: aug/* augmentation lane (live codex panes in ~/dev/agentops: sessions `agentops--codex-plan-approval`, `agentops--codex-plan-fable-approval`, `ao`; dirty-file mtimes 2026-06-12 07:16-07:28)
decision: subsume
am-thread: agent-mail unavailable for thread creation at decision time (no active reservations registry returned); this record + docs/plans/seams-drain-deltas-2026-06-12.md are the handoff surface (record relocated from .agents/council/ — gitignored)
bead: ag-xwjlc (seams epic)

## Evidence snapshot (2026-06-12, ~09:00 ET)

- `wt-ag-pj51` (chore/skill-trim-retire-52, commit 5e4f7e58a) is **already merged
  into main** (`git merge-base --is-ancestor` confirms). The "unmerged 405-file
  retirement" risk from planning is retired.
- main's recent history is `aug/*` merge commits (aug/validate, aug/status,
  aug/rpi, aug/review, aug/refactor ...) — an augmentation wave in progress.
- The working tree on host main is dirty with a large in-flight pass:
  PRODUCT.md, docs/SKILLS.md, docs/contracts/* (context-map, dispositions,
  critical-skills), .github/workflows/validate.yml, skills/SKILL-TIERS.md, many
  skills/*/SKILL.md (including uncommitted DELETIONS of vibe/ratchet/brainstorm/
  design/scenario/... — at committed HEAD those skills still exist), full
  skills-codex/ + images/gemini/ regen surfaces. `git status --porcelain`
  snapshot captured at decision time (~600 entries; see session evidence
  ~/.claude/usage-data/wave-evidence/).

## Decision

The active lane owns ALL shared/regen surfaces (single-writer per
docs/plans/skill-prune-phase2.md). The seams epic therefore:

1. **Subsumes its routing-edge drain (W2.1) into the lane**: deltas filed in
   `docs/plans/seams-drain-deltas-2026-06-12.md` instead of edited here.
2. Lands its own work on branch `chore/ag-seams-wave013` touching ONLY
   lane-free files (verified per-file against the dirty set): four new skill
   dirs, beads-br, automation-shape-routing, tests/scripts/*, one new script,
   this record, the delta doc.
3. **Holds the branch off main** until the lane's pass commits — pushing
   un-regen'd new skills now would redline context-map drift into the lane's
   fix-forward path. Merge step (post-lane): merge branch → run full regen set →
   pre-push gate → push.

## Rollback

Branch deletion (`git branch -D chore/ag-seams-wave013` + worktree remove);
host surfaces have their own snapshots (symlink list, cache purge list) in
~/.claude/usage-data/wave-evidence/.

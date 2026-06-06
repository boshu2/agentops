---
name: cc-worktree-isolation
description: |
  Isolate parallel agent writes so concurrent workers never clobber each other's files. Each writer operates in its own git worktree (separate working dir + branch); manual `git worktree` is the Codex-native path since Claude's EnterWorktree/isolation:worktree tools are unavailable here.

  Triggers: "worktree isolation", "parallel workers collide", "isolation: worktree", "sparse worktree checkout", "two agents editing the same file".

  Use when: spawning parallel workers that write to the same repo; a run produced merge garbage / lost edits; you need per-worker write isolation instead of ad hoc choreography. Not for: single-agent sequential work, or two tasks that MUST edit the same file (combine into one worker).
---

See the sibling `../SKILL.md` for the full methodology, constraints, quality rubric, examples, and troubleshooting. This file is the Codex-portable frontmatter; `prompt.md` carries the Codex execution profile.

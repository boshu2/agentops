---
name: cc-worktree-isolation
description: "Isolate parallel agent writes so concurrent workers never clobber each other's files. Each writer operates in its own git worktree (separate working dir + branch); manual `git worktree` is the Codex-native path since Claude's EnterWorktree/isolation:worktree tools are unavailable here."
---

See the sibling `../SKILL.md` for the full methodology, constraints, quality rubric, examples, and troubleshooting. This file is the Codex-portable frontmatter; `prompt.md` carries the Codex execution profile.

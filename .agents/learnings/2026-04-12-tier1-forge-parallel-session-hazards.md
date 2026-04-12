---
id: learning-2026-04-12-parallel-worktree-isolation
type: learning
date: 2026-04-12
category: process
confidence: high
maturity: provisional
utility: 0.8
---

# Learning: Parallel sessions in a shared worktree destroy untracked files

## What We Learned

When two Claude Code sessions operate in the same git worktree concurrently, any `git checkout` by one session silently removes untracked files that belong to the other session's feature branch. This happened 3 times during the Tier 1 forge implementation: the parallel "overnight" session kept flipping HEAD back to main, wiping all W4 files (redactor, forge_tier1) that hadn't been committed yet. The fix was creating a `git worktree add` at `/tmp/nami-tier1-w4` — an isolated copy where the parallel session couldn't touch files.

Key failure pattern: Write file → parallel session `git checkout main` → file gone from disk → test fails with "undefined" → rewrite file → repeat. Each rewrite costs ~5 minutes and context window budget.

## Why It Matters

Multi-session work is Bo's default operating mode (39% of messages during overlaps per CLAUDE.md). Any multi-wave implementation touching a shared worktree will hit this. The worktree solution is zero-cost and fully isolates branches.

## Rule

When implementing multi-wave work in a repo where parallel sessions are active: create a `git worktree add /tmp/<project>-<task> <branch>` immediately at session start. Do all writes in the worktree. Merge back to main worktree only at the end.

## Source

Tier 1 forge implementation (W0-W4+W7), 2026-04-12. Files rewritten 3x for redactor.go alone.

---

# Learning: Commit untracked files immediately after writing

## What We Learned

Even within a worktree, untracked files are vulnerable to `git clean`, `git checkout`, or other destructive operations by parallel processes. The safe pattern is: write file → `git add` → commit (even as WIP) immediately. This protects the file in git's object store regardless of what happens to the working tree.

During the Tier 1 forge work, the pattern of "write all files, then test, then commit" left a window where parallel session branch switches wiped files. The fix was tighter write→add→commit loops.

## Why It Matters

Reduces rework from file loss. A WIP commit can always be squashed later.

## Source

Same session. W1 commit survived because it was committed before the first branch flip; W4 files were lost 3x because they were untracked during the test phase.

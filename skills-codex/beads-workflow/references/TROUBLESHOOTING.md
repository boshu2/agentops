# Beads Troubleshooting — Reference

## Table of Contents
- [Worktree Errors](#worktree-errors)
- [Sync Issues](#sync-issues)
- [br vs bd Migration](#br-vs-bd-migration)
- [Common Problems](#common-problems)

---

## Worktree Errors

### The Problem

```
Error pulling from sync branch: failed to create worktree: exit status 128
fatal: 'main' is already checked out at '/path/to/repo'
```

Beads uses git worktrees for sync operations. If your `sync.branch` points to your current branch, git cannot create a worktree for it.

### The Fix

Create a dedicated sync branch that you never check out directly:

```bash
git branch beads-sync main
git push -u origin beads-sync
br config set sync.branch beads-sync
```

### For New Projects

Always configure a dedicated sync branch during setup:

```bash
br init
git branch beads-sync main
git push -u origin beads-sync
br config set sync.branch beads-sync
```

---

## Sync Issues

### Sync Commands

| Command | What it does |
|:--------|:-------------|
| `br sync --flush-only` | Export to JSONL only (no git operations) |
| `br sync` | Full sync: export, commit, push |

### Check Sync Branch Configuration

```bash
br config get sync.branch          # Should NOT be your current branch
git branch                         # Verify sync branch exists
git ls-remote --heads origin beads-sync  # Verify on remote
```

---

## br vs bd Migration

The Rust CLI (`br`) replaced the Go CLI (`bd`). Command mapping:

| Old (bd) | New (br) | Notes |
|:---------|:---------|:------|
| `bd init` | `br init` | Same |
| `bd create "Title"` | `br create "Title"` | Same |
| `bd sync` | `br sync` | Same |
| `bd sync --flush-only` | `br sync --flush-only` | Same |
| `bd list` | `br list` | Same |
| `bd config` | `br config` | Same |

### If you have both installed

Make sure `br` is in your PATH and takes precedence:

```bash
which br    # Should be beads_rust
br --version
```

---

## Common Problems

| Problem | Diagnosis | Fix |
|:--------|:----------|:----|
| Worktree errors | `br config get sync.branch` returns current branch | Create dedicated sync branch |
| Sync branch missing | `git ls-remote --heads origin beads-sync` empty | `git push -u origin beads-sync` |
| br not found | `which br` returns nothing | Install beads_rust, add to PATH |
| Cycles in graph | `br dep cycles` not empty | Review dependencies, remove cycle |
| bv shows nothing | No beads created | Run `br list` to verify |

### Health Check

```bash
br config list        # All settings
br dep cycles         # Must be empty
bv --robot-insights   # Check graph health
```

---

## Configuration Reference

```bash
br config list                              # Show all settings
br config get sync.branch                   # Check current sync branch
br config set sync.branch beads-sync        # Set sync branch
br config set issue_prefix myproject        # Set issue ID prefix
```

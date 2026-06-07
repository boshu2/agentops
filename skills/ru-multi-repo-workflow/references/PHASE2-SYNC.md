# Phase 2: Smart Sync — Deep Reference

> **Goal:** Ensure all repos have the canonical version—hyper-vigilant about not losing work.

## Contents

| Section | Jump |
|---------|------|
| Philosophy | [→](#the-philosophy) |
| Step 1: Check State | [→](#step-1-check-current-state) |
| Step 2: Sync | [→](#step-2-sync-all-repos) |
| Step 3: Divergence | [→](#step-3-handle-divergence) |
| Canonical Version | [→](#determining-canonical-version) |
| Resolution Actions | [→](#resolution-actions) |
| Surfacing Conflicts | [→](#surfacing-substantive-conflicts) |
| Safety Checks | [→](#safety-checks) |
| Scenarios | [→](#common-scenarios) |
| Validation | [→](#validation) |

---

## The Philosophy

**"Canonical" ≠ "Latest"**

The most recent commit isn't always the right one. A local experiment might be more valuable than a remote cleanup. A remote fix might supersede local work-in-progress.

**The insight:** You must manually diff and understand changes relative to each project's purpose. This cannot be mechanically determined.

---

## Step 1: Check Current State

```bash
cd /data/projects
ru status
```

### Reading `ru status` Output

| State | Meaning | Action |
|-------|---------|--------|
| `clean` | Local = Remote | Nothing to do |
| `ahead N` | Local has N unpushed commits | Consider pushing |
| `behind N` | Remote has N commits to pull | Pull (usually safe) |
| `diverged` | Both have unique commits | **JUDGMENT REQUIRED** |
| `dirty` | Uncommitted changes | Go back to Phase 1 |

---

## Step 2: Sync All Repos

```bash
# Parallel sync (4 jobs)
ru sync -j4

# Sequential (safer, easier to read output)
ru sync

# Dry run (see what would happen)
ru sync --dry-run
```

### Reading Sync Output

**Watch for these signals:**

| Output | Meaning | Action |
|--------|---------|--------|
| `Fast-forward` | Clean pull, no conflicts | ✅ Good |
| `Already up to date` | Nothing to do | ✅ Good |
| `CONFLICT` | Merge conflict | **STOP, investigate** |
| `diverged` | Both sides changed | **STOP, investigate** |
| `rejected` | Push failed | Check why |
| `error:` | Something broke | Investigate |

---

## Step 3: Handle Divergence

When local and remote have diverged:

```bash
cd /data/projects/REPO_NAME

# See the divergence
git log --oneline --graph HEAD origin/main --all

# See what's different
git diff HEAD origin/main

# See commit history comparison
git log --oneline HEAD...origin/main
```

### The Decision Tree

```
Local and remote differ?
│
├── Local has uncommitted changes
│   └─ STOP → Go back to Phase 1
│
├── Local ahead, remote behind (local has commits remote doesn't)
│   ├── Local changes are intentional work → Push
│   ├── Local changes are experiments → Decide: push or discard
│   └── Local changes are junk → Reset to remote (CAREFUL!)
│
├── Remote ahead, local behind (remote has commits local doesn't)
│   └─ Pull (usually safe)
│
├── Diverged (both have unique commits)
│   ├── Changes are independent (different files) → Rebase/merge
│   ├── Changes conflict (same files, different intent) → **SURFACE TO USER**
│   └── One side clearly supersedes the other → Reset the other
│
└── Same → Nothing to do
```

---

## Determining Canonical Version

**This is the judgment call.** Ask yourself:

1. **Which version represents the intended state of the project?**
2. **Which version would I want if I started fresh tomorrow?**
3. **Is any work being lost that I'd regret?**

### Investigation Commands

```bash
# Compare commits
git log --oneline HEAD...origin/main

# See actual differences
git diff HEAD origin/main

# See what local has that remote doesn't
git log --oneline origin/main..HEAD

# See what remote has that local doesn't
git log --oneline HEAD..origin/main

# See file-level summary
git diff --stat HEAD origin/main
```

### Common Patterns

| Situation | Canonical Is | Action |
|-----------|--------------|--------|
| Local has finished feature, remote is behind | Local | `git push` |
| Remote has merged PR, local is behind | Remote | `git pull` |
| Local has WIP, remote has polish | Usually remote | Pull, redo WIP |
| Both have valuable changes | Both | Merge carefully |
| Local is stale experiment | Remote | Reset local |

---

## Resolution Actions

### When Local is Canonical

```bash
# Just push
git push

# If rejected due to non-fast-forward
git push --force-with-lease  # Safer than --force
```

### When Remote is Canonical

```bash
# Standard pull
git pull

# Pull with rebase (cleaner history)
git pull --rebase

# If local has junk, hard reset
git fetch origin
git reset --hard origin/main  # DESTRUCTIVE
```

### When Both Have Value (Merge)

```bash
# Attempt merge
git merge origin/main

# If conflicts occur:
# 1. Edit conflicted files
# 2. Stage resolved files
git add resolved_file.py
# 3. Complete merge
git commit

# Then push the merge
git push
```

### When Both Have Value (Rebase)

```bash
# Rebase local onto remote
git rebase origin/main

# If conflicts:
# 1. Resolve conflicts
# 2. Stage resolved files
git add resolved_file.py
# 3. Continue rebase
git rebase --continue

# Then push (may need force)
git push --force-with-lease
```

---

## Surfacing Substantive Conflicts

**When to surface to user:**

- Conflicting changes to the same logic
- Different architectural approaches
- One version has features the other removed
- Unclear which represents "correct" behavior

**Format:**

```
🚨 CONFLICT REQUIRING JUDGMENT: repo_name

Local (abc123 - 3 days ago):
  - Refactored auth to use JWT
  - Added refresh token support

Remote (def456 - 1 day ago):
  - Refactored auth to use session cookies
  - Simplified token handling

These are incompatible approaches to the same problem.

Options:
1. Keep local (JWT approach, has refresh tokens)
2. Keep remote (session approach, simpler)
3. Merge manually (combine features?)

Which direction aligns with project goals?
```

---

## Safety Checks

### Before Any Destructive Action

```bash
# Create backup branch
git branch backup-$(date +%Y%m%d)

# Verify you can recover
git log backup-$(date +%Y%m%d) --oneline -3
```

### After Resolution

```bash
# Verify state
git status
git log --oneline -5

# Ensure tests pass (if applicable)
[project test command]

# Verify nothing unexpected changed
git diff HEAD~1 --stat
```

---

## Common Scenarios

### Scenario: Accidental Divergence

You pushed, someone else pushed, now diverged.

```bash
# Fetch latest
git fetch origin

# Rebase your work on top
git rebase origin/main

# Push (force needed after rebase)
git push --force-with-lease
```

### Scenario: Local Experiments to Discard

You tried something, it didn't work, remote is the truth.

```bash
# Save experiment just in case
git branch experiment-backup

# Reset to remote
git fetch origin
git reset --hard origin/main
```

### Scenario: Merge Conflict in Auto-Generated Files

Lock files, generated code, etc.

```bash
# Usually: accept remote version
git checkout --theirs package-lock.json
git add package-lock.json

# Or: regenerate
rm package-lock.json
npm install
git add package-lock.json
```

---

## Validation

After syncing all repos:

```bash
# Verify all clean
ru status | grep -v "clean"

# Should show nothing (all repos clean and synced)

# Double-check specific repo
cd /data/projects/REPO
git status
git log --oneline -3
```

---

## Quick Reference

```bash
# The flow
cd /data/projects
ru sync -j4                           # Sync all
# For each problem repo:
cd /data/projects/REPO
git log --oneline HEAD...origin/main  # See divergence
git diff HEAD origin/main             # See changes
# Decide canonical, then:
git pull --rebase                     # If remote canonical
git push                              # If local canonical
git merge origin/main                 # If both have value
```

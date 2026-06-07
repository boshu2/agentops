# Phase 1: Smart Commits — Deep Reference

> **Goal:** Commit all dirty repos with proper understanding and detailed messages.

## Contents

| Section | Jump |
|---------|------|
| Philosophy | [→](#the-philosophy) |
| Discovery | [→](#discovery-find-dirty-repos) |
| Step 1: Understand | [→](#step-1-understand-the-project) |
| Step 2: Review | [→](#step-2-review-changes) |
| Step 3: Group | [→](#step-3-identify-logical-groupings) |
| Step 4: Commit | [→](#step-4-stage-and-commit) |
| Step 5: Skip | [→](#step-5-what-not-to-commit) |
| Step 6: Push | [→](#step-6-push) |
| Scenarios | [→](#common-scenarios) |
| Validation | [→](#validation) |

---

## The Philosophy

You can't write good commit messages without understanding the project. A random file dump with "misc changes" is worse than no commit at all.

**The insight:** Reading AGENTS.md and README.md first takes 2 minutes. It saves 20 minutes of confusion later when you're trying to remember what these changes were for.

---

## Discovery: Find Dirty Repos

```bash
# JSON output for scripting
ru status --json 2>/dev/null | jq '[.repos[] | select(.dirty == true) | .name]'

# Human-readable
ru status | grep -E "^\s+M|^\?\?"

# Count dirty repos
ru status --json 2>/dev/null | jq '[.repos[] | select(.dirty)] | length'
```

### Understanding `ru status` Output

| Symbol | Meaning | Action |
|--------|---------|--------|
| `M` | Modified, tracked | Review changes, commit |
| `??` | Untracked | Decide: commit or ignore |
| `A` | Added to index | Already staged |
| `D` | Deleted | Commit the deletion |
| `R` | Renamed | Commit with context |

---

## Step 1: Understand the Project

**ALWAYS do this BEFORE looking at changes.**

```bash
cd /data/projects/REPO_NAME

# 1. Read project documentation
cat AGENTS.md README.md

# 2. If still unclear, investigate code structure
tree -L 2 src/
head -50 src/main.* src/lib.*

# 3. For complex projects, use code investigation agent
# (This is a judgment call - simple scripts don't need it)
```

### What to Look For in AGENTS.md/README.md

| Look For | Why It Matters |
|----------|----------------|
| Project purpose | Informs commit message context |
| Architecture | Helps group related changes |
| Naming conventions | Match existing style |
| Build/test commands | Know what to verify |
| Active development areas | Understand change significance |

---

## Step 2: Review Changes

```bash
# See all changes
git status

# See actual diffs
git diff

# See staged changes separately
git diff --cached

# See changes in specific file
git diff path/to/file
```

### Analyzing Changes

```bash
# Files changed with stats
git diff --stat

# Just filenames
git diff --name-only

# Group by directory
git diff --name-only | xargs -I{} dirname {} | sort -u
```

---

## Step 3: Identify Logical Groupings

**Changes that belong together:**

| Pattern | Example | Why Together |
|---------|---------|--------------|
| Feature + tests | `auth.py` + `test_auth.py` | Complete unit of work |
| Config cluster | `.env.example` + `config.py` | Same concern |
| Refactor scope | All files touching one module | Atomic change |
| Bug fix | Fix + regression test | Proves it's fixed |

**Changes that DON'T belong together:**

| Anti-Pattern | Why Bad |
|--------------|---------|
| Random files same day | No logical connection |
| Feature + unrelated cleanup | Muddies history |
| Multiple unrelated features | Can't cherry-pick/revert |

### Grouping Heuristic

```
Ask: "If I had to revert this commit, would reverting ALL these files make sense?"

Yes → Good grouping
No  → Split into multiple commits
```

---

## Step 4: Stage and Commit

```bash
# Stage specific files
git add path/to/related/files

# Stage hunks interactively (when file has mixed changes)
git add -p path/to/file

# Commit with detailed message
git commit -m "$(cat <<'EOF'
Descriptive title explaining the WHY (50 chars max)

- Detailed explanation of what changed
- Why these changes belong together
- Context for future readers
- Any caveats or known limitations

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

### Commit Message Structure

```
[Type]: [What] for [Why]          ← Title (imperative mood)

- [Specific change 1]              ← Body (bullet points)
- [Specific change 2]
- [Rationale if non-obvious]

[Breaking changes if any]          ← Footer

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Type Prefixes (Optional but Helpful)

| Type | Use For |
|------|---------|
| `feat:` | New functionality |
| `fix:` | Bug fixes |
| `refactor:` | Code restructure, no behavior change |
| `docs:` | Documentation only |
| `test:` | Test additions/fixes |
| `chore:` | Maintenance, deps, config |

---

## Step 5: What NOT to Commit

### Ephemeral Files (Always Skip)

```gitignore
# Logs
*.log
*.tmp
*.swp
*.bak
*~

# Build artifacts
node_modules/
target/
dist/
build/
__pycache__/
*.pyc
.cache/

# IDE/Editor
.idea/
.vscode/
*.sublime-*

# OS junk
.DS_Store
Thumbs.db
desktop.ini

# Secrets (CRITICAL)
.env
.env.local
*.key
*.pem
credentials.*
secrets.*
```

### Decision Matrix

| File Type | Commit? | Rationale |
|-----------|---------|-----------|
| Source code | ✅ Yes | Core deliverable |
| Tests | ✅ Yes | Proves correctness |
| Docs | ✅ Yes | User-facing |
| Config templates | ✅ Yes | `.env.example` helps others |
| Actual secrets | ❌ NEVER | Security |
| Build output | ❌ No | Reproducible |
| Editor config | ⚠️ Maybe | Only if team standard |
| Lock files | ✅ Usually | Reproducible builds |

---

## Step 6: Push

```bash
# Standard push
git push

# If upstream not set
git push -u origin main

# If rejected (remote has changes)
# → Go to Phase 2 (Smart Sync)
```

---

## Common Scenarios

### Scenario: Mixed Changes in One File

```bash
# Stage only specific hunks
git add -p file.py
# Answer: y/n/s(plit)/e(dit) for each hunk

# Commit the staged portion
git commit -m "First logical change"

# Stage and commit the rest
git add file.py
git commit -m "Second logical change"
```

### Scenario: Forgot to Include a File

```bash
# If not pushed yet
git add forgotten_file.py
git commit --amend --no-edit

# If already pushed, make new commit
git add forgotten_file.py
git commit -m "Add missing file from previous commit"
```

### Scenario: Committed Something Wrong

```bash
# Undo last commit, keep changes staged
git reset --soft HEAD~1

# Undo last commit, keep changes unstaged
git reset HEAD~1

# DANGEROUS: Discard changes entirely
git reset --hard HEAD~1
```

---

## Validation

Before pushing, verify:

```bash
# Check what will be pushed
git log origin/main..HEAD --oneline

# Verify no secrets
git diff origin/main..HEAD | grep -iE "(password|secret|key|token)"

# Run tests if applicable
[project-specific test command]
```

---

## Quick Reference

```bash
# The flow
ru status --json | jq '.repos[] | select(.dirty)'  # Find dirty
cd /data/projects/REPO                              # Enter repo
cat AGENTS.md README.md                             # Understand
git status && git diff                              # Review
git add FILES                                       # Stage logical group
git commit -m "MSG"                                 # Commit
git push                                            # Push
```

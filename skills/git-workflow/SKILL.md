---
name: git-workflow
description: Execute safe, reproducible Git workflows with MANDATORY semantic commits (feat/fix/docs), auto-injected templates, and git hooks integration
tags: [git, workflow, semantic-commits, hooks, automation]
---

# Git Workflow Skill

Execute safe, reproducible Git workflows with **MANDATORY semantic commit enforcement**, auto-injected commit templates via prepare-commit-msg hook, and institutional memory capture via post-commit hooks.

## When to Use

Use this skill proactively when:
- **Committing any changes** - Enforces semantic prefix (feat:, fix:, docs:, etc.)
- **Creating feature branches** - Ensures branch names align with commit types
- **Validating commit messages** - Checks format before pushing
- **Understanding git hooks** - Shows what happens automatically
- **Staging files safely** - Prevents secrets from being committed
- **Team collaboration** - Ensures consistent conventions across all contributors

## Core Principle: Semantic Commits Are MANDATORY

**Every commit MUST start with a semantic type prefix.** This is not optional.

**Required format:** `<type>(<scope>): <subject>`

**Valid types:** feat, fix, docs, refactor, test, chore, ci, perf, style, revert

**Examples:**
```
feat(monitoring): add Prometheus metrics exporter
fix(nginx): correct worker process count
docs(readme): update installation instructions
```

**Why mandatory:**
- Git becomes institutional memory (searchable, categorized history)
- Auto-generates release notes and changelogs
- Enables automation (CI/CD triggers based on type)
- Pattern recognition for AI agents
- Compliance and audit trails

**Full specification:** See `references/commit-conventions.md` (loaded JIT when needed)

## Git Hooks Architecture

This repository uses **3 automated hooks** that run during commit lifecycle:

### 1. pre-commit Hook (Before Commit)
**Purpose:** Keeps `.codex/agents-index.yaml` synchronized with `.claude/agents/*.md`
**Triggers:** Only when staging changes to `.claude/agents/`
**What it does:** Regenerates agent index, stages updated manifest
**Can fail:** Yes - fix errors, rerun `make codex-agents-index`, retry commit
**Location:** `tools/scripts/git-hooks/pre-commit` (symlinked to `.git/hooks/pre-commit`)

### 2. prepare-commit-msg Hook (Before Message Editing)
**Purpose:** Injects 4-section AgentOps template into commit message
**Triggers:** Every commit (unless already present)
**What it does:** Adds Context/Solution/Learning/Impact template
**Can fail:** No - always exits 0
**Location:** `tools/scripts/git-hooks/prepare-commit-msg`

**Template injected:**
```
Context: [problem/request that triggered this]
Solution: [approach taken]
Learning: [reusable insight gained]
Impact: [what improved]
```

### 3. post-commit Hook (After Commit)
**Purpose:** Captures session to codex, updates plan status, links commits, auto-completes todos
**Triggers:** After every successful commit
**What it does:** Runs 5 background tasks:
  - `capture_session.py` - Extracts session info, appends to codex
  - `update_plan_status.py` - Updates plan phase tracking
  - `link_commit.py` - Links commits to plans/issues
  - `sync_plan_docs.py` - Two-way sync between plans and commits
  - `todo_auto_completer.py` - Auto-marks todos complete based on commit
**Can fail:** No - always exits 0, runs async
**Logs:** `.git/hooks/capture-session.log`, `link-commit.log`, etc.
**Location:** `tools/scripts/git-hooks/post-commit`

**Hook installation:**
```bash
make install-hooks  # Creates symlinks from .git/hooks/ to tools/scripts/git-hooks/
```

## Standard Git Workflows

### 1. Safe Commit Workflow (MOST COMMON)

**Steps:**
1. **Validate first:** `make quick` (5s YAML check) and `make ci-all` (30s full suite)
2. **Stage files:** `git add .` (or specific files, never .env or secrets)
3. **Review staged:** `git diff --cached` to verify what will be committed
4. **Commit:** `git commit` (template auto-injected by prepare-commit-msg hook)
5. **Fill template:** Add semantic prefix + Context/Solution/Learning/Impact sections
6. **Validate format:** Use `scripts/validate_commit_message.sh` if uncertain
7. **Push:** `git push` (triggers ArgoCD sync)

**Template structure (auto-injected):**
```
feat(scope): brief description under 72 chars

Context: Why this work was needed
Solution: How it was implemented
Learning: Reusable insights for future work
Impact: Value delivered (metrics, capabilities, quality)
```

**Example filled template:**
```
feat(monitoring): add Prometheus metrics exporter

Context: Applications needed observability for production debugging
Solution: Added /metrics endpoint with custom metrics, configured scrape interval
Learning: Prometheus requires explicit metric registration before scraping
Impact: Enabled monitoring across 8 production sites, reduced MTTR by 40%
```

### 2. Feature Branch Workflow

**Branch naming (MUST align with commit types):**
```
feat/<description>       # New feature → commits use feat:
fix/<description>        # Bug fix → commits use fix:
docs/<description>       # Documentation → commits use docs:
refactor/<description>   # Code cleanup → commits use refactor:
test/<description>       # Testing → commits use test:
chore/<description>      # Maintenance → commits use chore:
```

**Examples:**
```bash
git checkout -b feat/prometheus-metrics    # Commits: feat(monitoring): ...
git checkout -b fix/nginx-worker-count     # Commits: fix(nginx): ...
git checkout -b docs/api-integration       # Commits: docs(api): ...
```

**Complete workflow:**
1. Create branch: `git checkout -b feat/my-feature`
2. Make changes, validate with `make quick`
3. Commit with semantic prefix: `git commit` (template auto-injected)
4. Push with tracking: `git push -u origin feat/my-feature`
5. Create PR linking to plan/issue
6. Review, merge → post-commit hooks capture to codex automatically

**Branch-Commit alignment rule:**
- Branch `feat/monitoring` → All commits MUST start with `feat(monitoring):`
- Branch `fix/nginx-config` → All commits MUST start with `fix(nginx):`
- Branch `docs/api-guide` → All commits MUST start with `docs(api):`

### 3. Safe Staging

**Never stage:**
- `.env` files (secrets)
- `config.env` (unless intentionally committing config)
- `pki/`, `ipi/auth/`, `ipi/tls/` (certificates/keys)
- Generated files (`apps/values.yaml`, `terraform.tfstate`)

**Always verify before commit:**
```bash
git diff --cached        # Review what will be committed
git status              # Check nothing unintended
```

## Semantic Commit Validation (CRITICAL)

### Automated Validation Script

Use `scripts/validate_commit_message.sh` to check commit format:

```bash
# Validate last commit
./scripts/validate_commit_message.sh HEAD

# Validate specific commit
./scripts/validate_commit_message.sh abc123

# Validate before committing (dry-run)
./scripts/validate_commit_message.sh --file .git/COMMIT_EDITMSG
```

### Validation Checklist

**Subject Line (REQUIRED):**
- ✅ Starts with valid type: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`, `style`, `revert`
- ✅ Has optional scope in parentheses: `feat(monitoring):`
- ✅ Has colon and space after type/scope: `feat(scope): `
- ✅ Subject in lowercase (not capitalized): `add metrics` not `Add metrics`
- ✅ No period at end: `add metrics` not `add metrics.`
- ✅ Under 72 characters total
- ✅ Imperative mood: `add feature` not `added feature` or `adds feature`

**Body Sections (REQUIRED for AgentOps):**
- ✅ Has `Context:` section explaining WHY
- ✅ Has `Solution:` section explaining WHAT
- ✅ Has `Learning:` section with reusable insight
- ✅ Has `Impact:` section with value metrics

**Common Mistakes:**
```
❌ feat: Add monitoring          # Capitalized after colon
❌ add monitoring                # Missing type prefix
❌ feat add monitoring           # Missing colon
❌ Feat(monitoring): add metrics # Capitalized type
❌ feat(monitoring): add metrics. # Period at end
✅ feat(monitoring): add metrics # CORRECT
```

### Quick Type Selection Decision Tree

**Ask yourself: What did I change?**
- Added new functionality? → `feat:`
- Fixed a bug? → `fix:`
- Changed documentation only? → `docs:`
- Cleaned up code without changing behavior? → `refactor:`
- Added/fixed tests? → `test:`
- Updated dependencies or tooling? → `chore:`
- Modified CI/CD pipelines? → `ci:`
- Improved performance? → `perf:`
- Fixed code style/linting? → `style:`

**Still unsure?** See `references/commit-conventions.md` for complete decision tree with examples

## Integration with Other Skills

Works with:
- **manifest-validation**: Validate before staging
- **config-rendering**: Harmonize before commit
- **testing**: Run tests before push

## Advanced Patterns

### Stash & Reapply
For context switches without committing:
```bash
git stash                    # Save work
# ... switch context ...
git stash pop               # Reapply work
```

### Rebase Before Push
Keep history clean:
```bash
git fetch origin
git rebase origin/main      # Rebase instead of merge
```

### Squash Commits
Combine related commits:
```bash
git rebase -i HEAD~3        # Interactive rebase last 3 commits
```

## Bundled Resources

### scripts/validate_commit_message.sh
Validates commit message format against semantic commit conventions. Run before pushing to catch format errors early.

### references/commit-conventions.md
Complete semantic commit specification with decision trees, examples, and integration patterns. Load JIT when you need deep guidance on commit types or format.

### references/git-hooks-readme.md
Documentation for all 3 git hooks (pre-commit, prepare-commit-msg, post-commit) including installation, troubleshooting, and testing.

## Quick Reference Card

**MANDATORY Format:**
```
<type>(<scope>): <subject>

Context: <why>
Solution: <what>
Learning: <insight>
Impact: <value>
```

**Valid Types:** feat, fix, docs, refactor, test, chore, ci, perf, style, revert

**Validation:** `./scripts/validate_commit_message.sh HEAD`

**Hooks:** 3 automatic (pre-commit, prepare-commit-msg, post-commit)

**Install Hooks:** `make install-hooks`

## Related Documentation

**In this skill:**
- `references/commit-conventions.md` - Complete specification (10k tokens)
- `references/git-hooks-readme.md` - Hook documentation (2k tokens)
- `scripts/validate_commit_message.sh` - Validation script (executable)

**In repository:**
- [git-workflow-guide.md](../../docs/how-to/guides/git-workflow-guide.md) - Complete workflow
- [Git safety rules](../../CLAUDE.md#-critical-safety-rules)
- [Session capture](../../docs/reference/codex-ops-notebook.md)

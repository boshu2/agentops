---
name: pr-research
description: >
  Systematic upstream codebase exploration for open source contribution.
  Outputs research artifact with contribution guidelines, PR patterns,
  maintainer expectations, and open issues. Triggers: "pr research",
  "upstream research", "contribution research", "open source prep".
version: 1.0.0
author: "AI Platform Team"
license: "MIT"
context: fork
allowed-tools: "Read,Write,Bash,Grep,Glob,Task,WebFetch"
skills:
  - beads
---

# PR Research Skill

Systematic exploration of upstream repositories before contributing. Produces
structured findings in `~/gt/.agents/<rig>/research/`.

## Overview

Research an external codebase to understand how to contribute effectively.
This is the FIRST step before planning or implementing an open source contribution.

**When to Use**:
- Before contributing to an external repository
- Starting a new open source contribution
- Evaluating whether to contribute to a project
- Understanding a project's contribution culture

**When NOT to Use**:
- Researching your own codebase (use `/research`)
- Already familiar with the project's guidelines
- Trivial documentation-only changes

---

## Workflow

```
-1. Prior Work Check    -> BLOCKING: Check for existing issues/PRs on this topic
0.  CONTRIBUTING.md     -> MANDATORY: Find and read contribution guidelines (BLOCKING)
1.  Repository Setup    -> Clone/identify upstream repo
2.  Guidelines Analysis -> Templates, CODE_OF_CONDUCT, additional docs
3.  PR Archaeology      -> Analyze merged PRs, commit patterns
4.  Maintainer Research -> Response patterns, review expectations
5.  Issue Discovery     -> Find contribution opportunities
6.  Output              -> Write research document
7.  Confirm             -> Verify file, inform user
```

---

## Phase -1: Prior Work Check (BLOCKING)

**CRITICAL**: Before ANY research, check if someone is already working on this.

### -1.1 Search for Existing Issues

```bash
# Search for open issues on this topic
gh issue list -R <owner/repo> --state open --search "<topic keywords>" --limit 20

# Search all issues (including closed) for prior work
gh issue list -R <owner/repo> --state all --search "<topic keywords>" --limit 10
```

### -1.2 Search for Existing PRs

```bash
# Search for open PRs that might address this
gh pr list -R <owner/repo> --state open --search "<topic keywords>" --limit 20

# Check for recently merged PRs (might already be fixed)
gh pr list -R <owner/repo> --state merged --search "<topic keywords>" --limit 10
```

### -1.3 Prior Work Checklist

| Check | Finding | Action |
|-------|---------|--------|
| Open issue exists | Issue #N covers this | Link to it, don't create duplicate |
| Open PR exists | PR #M implementing this | Don't duplicate work |
| Recently merged PR | PR #K fixed this | Verify fix, no work needed |
| Closed issue (won't fix) | Issue #J rejected | Understand why before proceeding |
| No prior work found | Clean slate | Proceed to Phase 0 |

### -1.4 If Prior Work Found

**DO NOT PROCEED** without addressing:

1. **Open issue exists**: Comment on it, don't create new one
2. **Open PR exists**: Coordinate with author or wait
3. **Recently merged**: Pull latest, verify fix, research complete
4. **Rejected previously**: Understand rejection before investing time

```bash
# Example: Check if issue still needs work
gh issue view <number> -R <owner/repo> --json state,assignees,labels,comments

# Example: Check PR status
gh pr view <number> -R <owner/repo> --json state,author,reviews
```

**Only proceed to Phase 0 if no conflicting work exists.**

---

## Phase 0: CONTRIBUTING.md Discovery (BLOCKING)

**CRITICAL**: This is the FIRST step. Do not proceed without finding contribution guidelines.

### 0.1 Find CONTRIBUTING.md

```bash
# Check all common locations
cat CONTRIBUTING.md 2>/dev/null && echo "---FOUND: CONTRIBUTING.md---"
cat .github/CONTRIBUTING.md 2>/dev/null && echo "---FOUND: .github/CONTRIBUTING.md---"
cat docs/CONTRIBUTING.md 2>/dev/null && echo "---FOUND: docs/CONTRIBUTING.md---"
cat CONTRIBUTORS.md 2>/dev/null && echo "---FOUND: CONTRIBUTORS.md---"

# Check README for contribution section
grep -i "contribut" README.md | head -10
```

### 0.2 Extract Key Requirements

From CONTRIBUTING.md, extract and document:

| Requirement | Where to Find | Example |
|-------------|---------------|---------|
| **Commit format** | "Commit messages" section | Conventional commits required |
| **PR process** | "Pull Requests" section | Must link issue, need 2 approvals |
| **Testing requirements** | "Testing" section | Must add tests, CI must pass |
| **Code style** | "Style" section | Run `make lint`, follow Go conventions |
| **CLA/DCO** | "Legal" or "License" section | Sign-off required |
| **Communication** | "Getting Started" section | Discuss in issue before large PRs |

### 0.3 CONTRIBUTING.md Checklist

| Check | Status | Notes |
|-------|--------|-------|
| File found | REQUIRED | At least one location must have guidelines |
| Commit format specified | Extract | e.g., "feat(scope): description" |
| PR template exists | Check | `.github/PULL_REQUEST_TEMPLATE.md` |
| Testing requirements | Extract | Coverage threshold, test types |
| Review process | Extract | Required approvals, CODEOWNERS |
| CLA/DCO required | Extract | Legal requirements |

### 0.4 No CONTRIBUTING.md Found

If no contribution guidelines exist:

1. **Check wiki**: Some projects use GitHub wiki for guidelines
2. **Check recent PRs**: Infer conventions from merged PRs
3. **Document absence**: Note this in research output as a risk
4. **Proceed with caution**: Higher risk of rejection

```bash
# Fallback: Check PR comments for guidelines
gh pr list --state merged --limit 5 --json body | jq -r '.[].body' | head -100
```

**WARNING**: Projects without CONTRIBUTING.md may have undocumented expectations.
Flag this as HIGH RISK in research output.

---

## Phase 1: Repository Setup

**Determine the upstream repository**:

```bash
# If in a fork, find upstream
git remote -v | grep upstream
git remote -v | grep origin

# If cloning fresh
git clone <upstream-url>
cd <repo>
```

**Identify rig for output**:
- If contributing FROM a rig's context, use that rig
- If general OSS work, use `_oss` subfolder
- Create directory: `mkdir -p ~/gt/.agents/<rig>/research/`

---

## Phase 2: Guidelines Analysis

**CRITICAL**: Always check these first.

### 2.1 Contribution Documentation

```bash
# Check for contribution docs
cat CONTRIBUTING.md 2>/dev/null || echo "No CONTRIBUTING.md"
cat CONTRIBUTORS.md 2>/dev/null || echo "No CONTRIBUTORS.md"
cat .github/CONTRIBUTING.md 2>/dev/null || echo "No .github/CONTRIBUTING.md"
cat CODE_OF_CONDUCT.md 2>/dev/null || echo "No CODE_OF_CONDUCT.md"

# Check for PR/issue templates
ls -la .github/ 2>/dev/null
cat .github/PULL_REQUEST_TEMPLATE.md 2>/dev/null
cat .github/ISSUE_TEMPLATE/*.md 2>/dev/null | head -100
```

### 2.2 Guidelines Checklist

| Document | Key Information |
|----------|-----------------|
| `CONTRIBUTING.md` | Contribution workflow, required steps |
| `CODE_OF_CONDUCT.md` | Community standards, enforcement |
| `PR_TEMPLATE.md` | Required sections for PRs |
| `ISSUE_TEMPLATE/` | How to report bugs/request features |

### 2.3 Development Setup

```bash
# Check for setup instructions
grep -i "setup\|install\|getting started" README.md | head -20

# Check for Makefile targets
cat Makefile 2>/dev/null | grep -E "^[a-z].*:" | head -20

# Check for package manager configs
ls package.json go.mod requirements.txt pyproject.toml Cargo.toml 2>/dev/null
```

---

## Phase 3: PR Archaeology

**CRITICAL**: Understand what successful PRs look like.

### 3.1 Recent Merged PRs

```bash
# List recent merged PRs
gh pr list --state merged --limit 20

# Analyze PR patterns
gh pr list --state merged --limit 10 --json title,body,additions,deletions,files | \
  jq -r '.[] | "[\(.additions)+/\(.deletions)-] \(.title)"'

# View exemplary PRs
gh pr view <number> --json title,body,files
```

### 3.2 Commit Conventions

```bash
# Recent commit style
git log --oneline -30 | head -20

# Check for conventional commits
git log --oneline -30 | grep -E "^[a-f0-9]+ (feat|fix|docs|refactor|test|chore)(\(.*\))?:"

# Commit message lengths
git log --format="%s" -20 | awk '{print length, $0}' | sort -rn | head -5
```

### 3.3 PR Size Analysis

| Size | Files | Lines | Likelihood |
|------|-------|-------|------------|
| **Small** | 1-3 | <100 | High acceptance |
| **Medium** | 4-10 | 100-500 | Moderate |
| **Large** | 10+ | 500+ | Needs discussion first |

---

## Phase 4: Maintainer Research

### 4.1 Core Contributors

```bash
# Top contributors
git shortlog -sn --all | head -10

# Recent active contributors
git shortlog -sn --since="6 months ago" | head -10

# Primary reviewers
gh pr list --state merged --limit 20 --json reviews | \
  jq -r '.[].reviews[].author.login' | sort | uniq -c | sort -rn | head -5
```

### 4.2 Response Patterns

```bash
# Check PR review time (rough estimate)
gh pr list --state merged --limit 10 --json createdAt,mergedAt | \
  jq -r '.[] | "\(.createdAt) -> \(.mergedAt)"'

# Open PR age (how long do PRs sit?)
gh pr list --state open --json createdAt,title | \
  jq -r '.[] | "\(.createdAt): \(.title)"'
```

### 4.3 Maintainer Expectations Checklist

- [ ] Commit message format (conventional? imperative?)
- [ ] PR body requirements (template? sections?)
- [ ] Test requirements (coverage threshold? new tests?)
- [ ] Documentation requirements (inline? README update?)
- [ ] CI requirements (all checks must pass?)
- [ ] Review process (CODEOWNERS? required approvals?)

---

## Phase 5: Issue Discovery

### 5.1 Good First Issues

```bash
# Find beginner-friendly issues
gh issue list --label "good first issue" --state open
gh issue list --label "help wanted" --state open
gh issue list --label "contributions welcome" --state open

# Recent issues (active project indicator)
gh issue list --state open --limit 20
```

### 5.2 Issue Analysis

```bash
# Issue categories
gh issue list --state open --json labels | \
  jq -r '.[].labels[].name' | sort | uniq -c | sort -rn | head -10

# Issues with no assignee
gh issue list --state open --json assignees,title,number | \
  jq -r '.[] | select(.assignees | length == 0) | "#\(.number): \(.title)"' | head -10
```

### 5.3 Contribution Opportunities

| Priority | Type | Description |
|----------|------|-------------|
| **High** | Good first issue | Explicitly marked for newcomers |
| **High** | Bug with reproduction | Clear problem, testable fix |
| **Medium** | Help wanted | Maintainers need assistance |
| **Medium** | Documentation | Often overlooked, high acceptance |
| **Low** | Feature request | May need discussion first |

---

## Phase 6: Output

Write to `~/gt/.agents/<rig>/research/YYYY-MM-DD-pr-{repo-slug}.md`

### Output Template

```markdown
---
date: YYYY-MM-DD
type: PR-Research
upstream: owner/repo
url: https://github.com/owner/repo
tags: [research, oss, contribution]
status: COMPLETE
---

# PR Research: {repo-name}

## Executive Summary

{2-3 sentences: project health, contribution friendliness, recommended approach}

## Repository Overview

| Attribute | Value |
|-----------|-------|
| **Stars** | N |
| **Open Issues** | N |
| **Open PRs** | N |
| **Last Commit** | YYYY-MM-DD |
| **Primary Language** | lang |
| **License** | license |

## Contribution Guidelines

### Documentation

| Document | Status | Key Requirements |
|----------|--------|------------------|
| CONTRIBUTING.md | Present/Missing | {summary} |
| CODE_OF_CONDUCT.md | Present/Missing | {type} |
| PR Template | Present/Missing | {required sections} |

### Development Setup

{Setup steps extracted from docs}

## PR Patterns

### Successful PR Characteristics

- **Average size**: X files, Y lines
- **Commit style**: {conventional/imperative/etc}
- **PR body**: {requirements}
- **Review time**: ~X days

### Example PRs

| PR | Type | Size | Notes |
|----|------|------|-------|
| #N | feat | S/M/L | {what made it successful} |

## Maintainer Expectations

### Core Contributors

| Contributor | Role | Activity |
|-------------|------|----------|
| @user | maintainer | active |

### Review Process

- Required approvals: N
- CODEOWNERS: Present/Missing
- CI requirements: {list}

## Contribution Opportunities

### Recommended Issues

| Issue | Type | Difficulty | Notes |
|-------|------|------------|-------|
| #N | bug/feat | easy/medium | {why suitable} |

### Avoided Areas

| Area | Reason |
|------|--------|
| {component} | Under active development |

## Risks & Considerations

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Slow review | {H/M/L} | {approach} |
| Scope creep | {H/M/L} | Small PRs |

## Recommendation

{Recommended contribution approach}

## Next Steps

→ `/pr-plan .agents/research/YYYY-MM-DD-pr-{repo}.md`
```

---

## Phase 7: Confirm

```bash
ls -la ~/gt/.agents/<rig>/research/
```

Tell user:
```
PR Research output: ~/gt/.agents/<rig>/research/YYYY-MM-DD-pr-{repo}.md
Next: /pr-plan ~/gt/.agents/<rig>/research/YYYY-MM-DD-pr-{repo}.md
```

---

## Anti-Patterns

| DON'T | DO INSTEAD |
|-------|------------|
| Skip guidelines check | Always read CONTRIBUTING.md first |
| Ignore PR patterns | Study successful merged PRs |
| Target active areas | Find stable code to contribute to |
| Start with large PRs | Begin with small, focused changes |
| Assume conventions | Check commit message style |
| Ignore issue labels | Look for "good first issue" |
| **Assume push access** | **Always verify with `gh repo view --json viewerPermission`** |

---

## Push Access Verification

**CRITICAL**: Never assume the user has push access to external repositories.

### Check Access Level

```bash
# Verify actual permissions
gh repo view <owner/repo> --json viewerPermission --jq '.viewerPermission'
```

| Permission | Meaning | Workflow |
|------------|---------|----------|
| `ADMIN` | Owner/admin | Can push directly |
| `WRITE` | Collaborator | Can push directly |
| `READ` | Read-only | **Must fork and PR** |
| `NONE` | No access | **Must fork and PR** |

### Default Assumption

**Unless `viewerPermission` is `ADMIN` or `WRITE`, assume fork-based workflow is required.**

Do NOT tell users they have push access based on:
- Being "crew" or having a local clone
- The repo being in their workspace
- Any assumption about their role

**Always verify programmatically.**

---

## Workflow Integration

```
/pr-research <repo> -> /pr-plan <research> -> implement -> /pr-prep
```

---

## References

- **PR Preparation**: `~/.claude/skills/pr-prep/SKILL.md`
- **Output Template**: See "Output Template" section above (lines 359-457)

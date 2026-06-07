# Prompts for Multi-Repo Workflow

> **Copy-paste prompts for each phase.** These are the exact instructions to give Claude for each workflow stage.

---

## Contents

| Phase | Jump |
|-------|------|
| Phase 1: Smart Commits | [→](#phase-1-smart-commits) |
| Phase 2: Smart Sync | [→](#phase-2-smart-sync) |
| Phase 3: Issue/PR Review | [→](#phase-3-issuepr-review) |
| Phase 4: Release & Monitor | [→](#phase-4-release--monitor) |
| Full Workflow | [→](#full-workflow-all-phases) |

---

## Phase 1: Smart Commits

### THE EXACT PROMPT

```
First read ALL of the AGENTS.md file and README.md file super carefully and
understand ALL of both! Then use your code investigation agent mode to fully
understand the code, technical architecture and purpose of the project.

Now, based on your knowledge of the project, commit all changed files in a
series of logically connected groupings with super detailed commit messages
for each and then push. Take your time to do it right. Don't edit the code
at all. Don't commit obviously ephemeral files. Use ultrathink.
```

### When to Use

- You have uncommitted changes across one or more repos
- `ru status` shows dirty repos
- You need thoughtful commit messages that explain WHY, not just WHAT

### Key Points

1. **Understand first** — 2 minutes reading AGENTS.md saves 20 minutes of confusion
2. **Logical groupings** — Related changes go together (feature + tests, config cluster, etc.)
3. **Don't edit** — This is about committing existing work, not making changes
4. **Skip ephemeral** — No logs, build artifacts, `.env`, IDE files

### Expected Behavior

The agent will:
1. Read and internalize project documentation
2. Review all uncommitted changes
3. Group related changes logically
4. Write detailed commit messages explaining the "why"
5. Commit each group separately
6. Push to remote

---

## Phase 2: Smart Sync

### THE EXACT PROMPT

```
First cd to /data/projects and run `ru` then `ru sync`. Carefully study every
single line of output.

Help me ensure all repos are up to date but be SUPER CAREFUL and hyper cautious
about potentially losing ANY useful work (code, documentation, beads tasks, etc)
in BOTH local repos and remote repos on GitHub.

In every case, I want the one "best" canonical version. Usually that's the latest
version, but NOT always—this cannot be mechanically determined. You must manually
diff between versions and understand the changes relative to the purpose and
structure of each specific project.
```

### When to Use

- After Phase 1 (all dirty repos committed)
- Before Phase 3 (need to be synced to review issues accurately)
- When `ru status` shows repos that are ahead/behind/diverged

### Key Points

1. **Canonical ≠ Latest** — The most recent commit isn't always right
2. **Manual diff required** — Understand changes relative to project purpose
3. **Surface conflicts** — Don't resolve substantive conflicts without user input
4. **Preserve work** — Never lose useful code, docs, or tasks from either side

### Expected Behavior

The agent will:
1. Run `ru sync` and analyze output
2. For each diverged repo:
   - Examine local and remote commits
   - Diff the actual changes
   - Determine which version is canonical (or if merge needed)
3. Surface any substantive conflicts for user judgment
4. Resolve mechanical conflicts autonomously
5. Push canonical versions

### SURFACE FORMAT for Conflicts

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

## Phase 3: Issue/PR Review

### THE EXACT PROMPT

```
Read AGENTS.md so it's fresh in your mind.

Use ru to go over all repos and review gh issues and PRs. Don't bother with
issues/PRs from before 2025.

CONTRIBUTION POLICY: We don't allow PRs or outside contributions. I'll have
Claude review submissions via gh and independently decide whether and how to
address them. Bug reports are welcome.

Use gh to review all open issues and PRs. Do your own totally separate and
independent verification. Don't trust user reports or suggested fixes—use
them as inspiration only. Everything must come from your own investigation,
official docs, actual code, and empirical evidence.

Many issues are likely stale—check dates and subsequent commits. Use ultrathink.

After reviewing and taking actions (fixes, features), respond on my behalf via gh.

NEVER merge PRs. You can look at them for good ideas but check with me first
before integrating even ideas—could be scope creep or wrong direction.
```

### When to Use

- After Phase 2 (repos are synced and up to date)
- When you need to process GitHub issues and PRs across repos
- `ru review --dry-run` shows open items

### Key Points

1. **Independent verification** — Never trust user reports blindly
2. **2025+ filter** — Ignore stale issues from before 2025
3. **No PR merges** — NEVER merge PRs; mine them for ideas only
4. **Scope awareness** — Surface complex features for approval; decline scope creep
5. **Respond via gh** — Use `gh issue comment`, `gh issue close`, etc.

### Expected Behavior

The agent will:
1. Discover open issues/PRs across all repos
2. Filter to 2025+ items
3. For each item:
   - Read the issue/PR
   - Verify claims independently (check code, reproduce bugs)
   - Determine appropriate action
4. Fix confirmed bugs and implement fitting features
5. Close stale issues with explanation
6. Surface complex decisions for user judgment
7. Respond on GitHub via `gh` commands

### Decision Matrix

| Type | Verified? | Action |
|------|-----------|--------|
| Bug, confirmed, unfixed | ✓ | Fix it, close: `gh issue close N -c "Fixed in SHA"` |
| Bug, already fixed | ✓ | Close: `gh issue close N -c "Fixed in SHA, please update"` |
| Bug, can't reproduce | ? | Ask: `gh issue comment N -b "Need: steps, OS, error output"` |
| Bug, pre-2025 | ✗ | Close: `gh issue close N -c "Closing as stale"` |
| Feature, simple, fits | ✓ | Implement, close |
| Feature, complex | ? | **SURFACE TO USER** |
| Feature, scope creep | ✗ | Decline: `gh issue close N -c "Would add scope I want to avoid"` |
| PR, any | — | **NEVER MERGE**, mine for ideas, close with explanation |

### SURFACE FORMAT for Scope Decisions

```
🤔 FEATURE REQUEST REQUIRING JUDGMENT: owner/repo#42

**Request:** "[Title from issue]"

**User's Problem:** [What they're trying to accomplish]

**Analysis:**
- Pros: [Benefits]
- Cons: [Drawbacks, maintenance burden]
- Scope Risk: [What it might lead to]

**Implementation Options:**
1. Full implementation: [What it would take]
2. Simplified version: [Reduced scope alternative]
3. Decline: [Polite response]

**My recommendation:** [Option X] because [reasoning]

What would you like me to do?
```

### Response Templates

**Bug Fixed:**
```bash
gh issue close NUMBER -R owner/repo -c "Fixed in abc123.

The issue was [brief explanation]. [Any additional context about the fix or usage notes.]"
```

**Already Fixed:**
```bash
gh issue close NUMBER -R owner/repo -c "This was fixed in abc123 (from [date]).

Please update to the latest version. If you still see the issue after updating, reopen with details."
```

**Cannot Reproduce:**
```bash
gh issue comment NUMBER -R owner/repo -b "I tried to reproduce this but couldn't. Could you provide:

- Exact steps to reproduce
- OS and version
- Full error output (if any)
- Version you're using

Happy to investigate further with more details."
```

**Stale Issue:**
```bash
gh issue close NUMBER -R owner/repo -c "Closing as stale—the codebase has changed significantly since this was filed.

If this is still relevant with current versions, please open a new issue with fresh reproduction steps."
```

**Feature Declined (Scope):**
```bash
gh issue close NUMBER -R owner/repo -c "Thanks for the suggestion! After consideration, this would add scope I'm trying to avoid for this project.

Feel free to fork if you need this functionality. [Optional: brief explanation of why it doesn't fit.]"
```

**PR Closed (Policy):**
```bash
gh pr close NUMBER -R owner/repo -c "Thanks for the contribution! Per project policy, I don't merge outside PRs, but I reviewed your approach.

[One of:]
- I've implemented a similar fix in abc123
- Noted the idea—may address differently later
- Current behavior is intentional because [reason]

Appreciate you taking the time!"
```

---

## Phase 4: Release & Monitor

### THE EXACT PROMPT

```
For projects that have GH Actions and releases, monitor the CI runs after pushing.

Watch each run until complete. If any fail:
1. Read the failure logs with `gh run view RUN_ID --log-failed`
2. Diagnose the root cause
3. Fix the issue (don't skip or ignore)
4. Commit the fix with a clear message
5. Push and watch again
6. Repeat until green

Never leave a repo with failing CI. Iterate until all runs pass.

Keep a detailed TODO list of:
- Which repos have CI
- Which runs are pending/passing/failing
- What fixes you've attempted

Surface persistent failures (3+ attempts) for judgment.
```

### When to Use

- After Phase 1 (commits pushed)
- When repos have `.github/workflows/` directory
- When you need to ensure CI is green before moving on

### Key Points

1. **Don't leave broken** — A push isn't done until CI is green
2. **Iterate** — Diagnose → fix → push → watch → repeat
3. **Track progress** — Use TODO list to avoid losing track across repos
4. **Surface persistent failures** — After 3 attempts, ask for judgment
5. **Skip ephemeral** — Never commit logs, build artifacts, secrets

### Expected Behavior

The agent will:
1. Detect which repos have CI (`.github/workflows/`)
2. After pushing, run `gh run watch` to monitor
3. If failure:
   - Read logs with `gh run view RUN_ID --log-failed`
   - Diagnose the issue
   - Make a focused fix
   - Commit and push
   - Watch again
4. Repeat until green
5. Surface persistent failures for human judgment

### Ephemeral File Filter

**NEVER commit these:**

| Category | Patterns |
|----------|----------|
| Transient | `*.log`, `*.tmp`, `*.swp`, `*~` |
| Build artifacts | `target/`, `node_modules/`, `dist/`, `__pycache__/` |
| **SECRETS** | `.env`, `*.key`, `*.pem`, `credentials.*` |
| IDE/Editor | `.idea/`, `.vscode/`, `*.sublime-*` |
| OS junk | `.DS_Store`, `Thumbs.db` |

### SURFACE FORMAT for Persistent Failures

```
🔴 CI FAILING AFTER 3+ ATTEMPTS: owner/repo — Run #12345

**Failure:** [test name or step]
**Error:** [key error message]

**Attempted fixes:**
1. [What you tried] — [result]
2. [What you tried] — [result]
3. [What you tried] — [result]

**Analysis:**
- [ ] Flaky test? (passes locally, fails in CI)
- [ ] Environment issue? (CI has different setup)
- [ ] Real regression? (code change broke something)

**Logs:** `gh run view 12345 --log-failed`
**Workflow file:** `.github/workflows/ci.yml`

Need guidance on how to proceed.
```

### Common CI Fix Patterns

**Formatting:**
```bash
cargo fmt && git add . && git commit -m "style: auto-format" && git push
```

**Lock file sync:**
```bash
cargo update && git add Cargo.lock && git commit -m "chore: update lock" && git push
```

**Test fix:**
```bash
# After fixing the test
git add . && git commit -m "fix(test): resolve flaky auth test" && git push
```

---

## Full Workflow (All Phases)

### THE EXACT PROMPT

For a complete multi-repo maintenance session:

```
Let's do a full multi-repo maintenance workflow:

PHASE 1: First, find all dirty repos with `ru status` and commit them. Read
AGENTS.md/README.md for each before committing. Group related changes logically
with detailed commit messages. Push when done. Don't commit ephemeral files
(logs, build artifacts, .env, IDE config).

PHASE 2: Then run `ru sync` and carefully ensure all repos have the canonical
version. Be hyper-cautious about losing work. Surface any substantive conflicts
for my judgment—don't resolve them autonomously.

PHASE 3: Review all 2025+ issues and PRs. Do independent verification—don't
trust user reports blindly. Fix confirmed bugs, implement fitting features,
close stale items. Surface complex scope decisions for my approval. NEVER merge
PRs. Respond on GitHub via gh.

PHASE 4: For repos with GH Actions, monitor CI runs after pushing. If any fail,
iterate: diagnose → fix → push → watch until green. Never leave broken CI.
Surface persistent failures (3+ attempts) for judgment.

Use ultrathink. Take your time. Surface anything that needs my judgment.
Keep a detailed TODO list to track progress across repos.
```

### When to Use

- Regular maintenance sessions
- Starting fresh after being away from repos
- When you want a complete sweep across all managed repos

---

## Quick Reference

```bash
# Before starting
ru doctor && gh auth status

# Phase 1: Commit
ru status --json | jq '.repos[] | select(.dirty)'
cd /data/projects/REPO && cat AGENTS.md README.md
git status && git diff

# Phase 2: Sync
ru sync -j4
git log --oneline HEAD...origin/main
git diff HEAD origin/main

# Phase 3: Review
ru review --dry-run
gh issue view NUMBER -R owner/repo
gh pr diff NUMBER -R owner/repo
gh issue close NUMBER -R owner/repo -c "REASON"

# Phase 4: Release & Monitor
ls .github/workflows/*.yml 2>/dev/null         # Has CI?
gh run list --limit 5                           # Recent runs
gh run watch                                    # Monitor live
gh run view RUN_ID --log-failed                 # Diagnose failure
# fix → commit → push → gh run watch            # Iterate until green
```

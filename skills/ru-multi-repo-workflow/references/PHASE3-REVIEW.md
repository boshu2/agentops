# Phase 3: Issue/PR Review — Deep Reference

> **Goal:** Review all GitHub issues/PRs with independent verification, respond on behalf of user, surface judgment calls.

## Contents

| Section | Jump |
|---------|------|
| Philosophy | [→](#the-philosophy) |
| Step 1: Discover | [→](#step-1-discover-open-items) |
| Step 2: Review | [→](#step-2-review-each-item) |
| Step 3: Verify | [→](#step-3-independent-verification) |
| Step 4: Decide | [→](#step-4-determine-action) |
| Step 5: Act | [→](#step-5-take-action) |
| Step 6: Respond | [→](#step-6-respond-via-gh) |
| Surfacing Calls | [→](#surfacing-judgment-calls) |
| Mining PRs | [→](#mining-prs-for-ideas) |
| Batch Processing | [→](#batch-processing) |
| Validation | [→](#validation) |

---

## The Philosophy

**Never trust user reports blindly.**

User-submitted issues are hints, not facts. Suggested fixes may be wrong, incomplete, or introduce new bugs. Your job is independent verification—use reports as inspiration, but everything must come from your own investigation.

**Contribution policy:** No outside PRs are merged. Ever. But PRs can contain good ideas worth implementing yourself.

---

## Step 1: Discover Open Items

```bash
# Overview of all repos
ru review --dry-run

# JSON for scripting
ru review --dry-run --json 2>/dev/null | jq '.summary'

# Filter to 2025+ (ignore stale issues)
ru review --dry-run --json 2>/dev/null | \
  jq '[.items[] | select(.created_at >= "2025-01-01")]'

# Count by repo
ru review --dry-run --json 2>/dev/null | \
  jq 'group_by(.repo) | map({repo: .[0].repo, count: length})'
```

---

## Step 2: Review Each Item

### For Issues

```bash
# View full issue
gh issue view NUMBER -R owner/repo

# View with comments
gh issue view NUMBER -R owner/repo --comments

# JSON for parsing
gh issue view NUMBER -R owner/repo --json title,body,comments,createdAt,author
```

### For Pull Requests

```bash
# View PR summary
gh pr view NUMBER -R owner/repo

# See the actual changes (CRITICAL)
gh pr diff NUMBER -R owner/repo

# View with review comments
gh pr view NUMBER -R owner/repo --comments
```

---

## Step 3: Independent Verification

**DO NOT:**
- Trust that the bug exists as described
- Trust that the suggested fix is correct
- Trust that the reproduction steps work
- Assume the issue is still relevant

**DO:**
- Reproduce the bug yourself
- Check if it's already fixed in recent commits
- Read the actual code
- Verify against official documentation
- Test empirically

### Verification Workflow

```bash
cd /data/projects/REPO_NAME

# 1. Check if commits since issue might have fixed it
git log --oneline --since="ISSUE_DATE" | head -20

# 2. Search codebase for relevant code
grep -r "relevant_function" src/

# 3. Read the actual implementation
cat src/relevant_file.py

# 4. If claiming API issue, check docs
# (Use WebFetch for official documentation)

# 5. If reproducible bug, try to reproduce
# (Project-specific commands)
```

### Verification Questions

| Claim Type | Verify By |
|------------|-----------|
| "X crashes" | Reproduce the crash |
| "X doesn't work" | Define "work", test actual behavior |
| "X should do Y" | Check docs, intended behavior |
| "X is slow" | Benchmark, compare to baseline |
| "X has security issue" | Analyze code path, test exploit |

---

## Step 4: Determine Action

### Decision Matrix

| Type | Verified? | Relevant? | Action |
|------|-----------|-----------|--------|
| Bug | Confirmed, unfixed | Yes | Fix it, close with commit ref |
| Bug | Confirmed, already fixed | N/A | Close, cite commit |
| Bug | Cannot reproduce | Maybe | Request more details |
| Bug | Pre-2025 | No | Close as stale |
| Feature | Simple, fits scope | Yes | Implement, close |
| Feature | Complex | Maybe | **SURFACE TO USER** |
| Feature | Scope creep | No | Decline politely |
| PR | Any | N/A | **NEVER MERGE**, mine for ideas |

### Scope Creep Indicators

| Signal | Why It's Scope Creep |
|--------|---------------------|
| "Also add X while you're at it" | Bundled unrelated request |
| "Support format Y too" | Opens floodgate for formats |
| Requires new dependencies | Maintenance burden |
| Changes core architecture | Risk outweighs benefit |
| "Make it configurable" | Complexity for edge cases |

---

## Step 5: Take Action

### Fixing Bugs

```bash
# 1. Create fix
# (Edit files as needed)

# 2. Test fix
[project test command]

# 3. Commit with issue reference
git commit -m "Fix: [description]

Closes #NUMBER

Co-Authored-By: Claude <noreply@anthropic.com>"

# 4. Push
git push
```

### Implementing Features

```bash
# 1. Implement feature
# (Edit files as needed)

# 2. Add tests if applicable
# (Edit test files)

# 3. Commit
git commit -m "Add: [feature description]

Closes #NUMBER

Co-Authored-By: Claude <noreply@anthropic.com>"

# 4. Push
git push
```

---

## Step 6: Respond via gh

### Response Templates

#### Bug Fixed

```bash
gh issue close NUMBER -R owner/repo -c "Fixed in abc123.

The issue was [brief explanation]. [Any additional context about the fix or usage notes.]"
```

#### Bug Already Fixed

```bash
gh issue close NUMBER -R owner/repo -c "This was fixed in abc123 (from [date]).

Please update to the latest version. If you still see the issue after updating, reopen with details."
```

#### Cannot Reproduce

```bash
gh issue comment NUMBER -R owner/repo -b "I tried to reproduce this but couldn't. Could you provide:

- Exact steps to reproduce
- OS and version
- Full error output (if any)
- Version you're using

Happy to investigate further with more details."
```

#### Stale Issue (Pre-2025)

```bash
gh issue close NUMBER -R owner/repo -c "Closing as stale—the codebase has changed significantly since this was filed.

If this is still relevant with current versions, please open a new issue with fresh reproduction steps."
```

#### Feature Implemented

```bash
gh issue close NUMBER -R owner/repo -c "Implemented in abc123.

[Brief description of what was added and how to use it.]"
```

#### Feature Declined (Scope)

```bash
gh issue close NUMBER -R owner/repo -c "Thanks for the suggestion! After consideration, this would add scope I'm trying to avoid for this project.

Feel free to fork if you need this functionality. [Optional: brief explanation of why it doesn't fit.]"
```

#### Feature Needs Discussion

```bash
gh issue comment NUMBER -R owner/repo -b "Interesting idea. A few questions before I consider this:

1. [Question about use case]
2. [Question about expected behavior]
3. [Question about edge cases]

This would help me understand if/how to implement it."
```

#### PR Closed (Policy)

```bash
gh pr close NUMBER -R owner/repo -c "Thanks for the contribution! Per project policy, I don't merge outside PRs, but I reviewed your approach.

[One of:]
- I've implemented a similar fix in abc123
- Noted the idea—may address differently later
- Current behavior is intentional because [reason]

Appreciate you taking the time!"
```

---

## Surfacing Judgment Calls

### When to Surface

| Situation | Surface? | Why |
|-----------|----------|-----|
| Simple bug, clear fix | No | Just fix it |
| Feature fits, low effort | No | Just do it |
| Complex feature request | **Yes** | Scope implications |
| Interesting PR ideas | **Yes** | Need approval to integrate |
| Ambiguous intended behavior | **Yes** | Could break things |
| Security-related | **Yes** | High stakes |

### Surface Format

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

---

## Mining PRs for Ideas

PRs are **never merged**, but may contain valuable insights.

```bash
# See what they changed
gh pr diff NUMBER -R owner/repo

# Read their approach
gh pr view NUMBER -R owner/repo --comments

# If idea is good, implement yourself:
# 1. Understand their approach
# 2. Write your own implementation
# 3. Commit (no attribution needed for ideas)
# 4. Close PR with explanation
```

---

## Batch Processing

For repos with many issues:

```bash
# Get all issues as JSON
gh issue list -R owner/repo --state open --json number,title,createdAt,author | \
  jq '.[] | select(.createdAt >= "2025-01-01")'

# Process in order
for num in $(gh issue list -R owner/repo --state open --json number -q '.[].number'); do
  echo "=== Issue #$num ==="
  gh issue view $num -R owner/repo
  read -p "Action? (f=fix, c=close, s=skip): " action
  # ... handle action
done
```

---

## Validation

After reviewing all items:

```bash
# Verify no open 2025+ issues remain unaddressed
ru review --dry-run --json 2>/dev/null | \
  jq '[.items[] | select(.created_at >= "2025-01-01" and .state == "open")] | length'

# Should be 0 (or only surfaced-to-user items)

# Verify commits reference issues
git log --oneline --grep="Closes #" -10
```

---

## Quick Reference

```bash
# Discover
ru review --dry-run
gh issue list -R owner/repo --state open

# Investigate
gh issue view NUMBER -R owner/repo
git log --since="ISSUE_DATE" --oneline

# Act
gh issue close NUMBER -R owner/repo -c "REASON"
gh issue comment NUMBER -R owner/repo -b "MESSAGE"
gh pr close NUMBER -R owner/repo -c "REASON"

# Never
gh pr merge NUMBER  # NEVER DO THIS
```

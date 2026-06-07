# GitHub Workflows — Reference

## Table of Contents
- [Quick PR Workflow](#quick-pr-workflow)
- [Code Review Workflow](#code-review-workflow)
- [Release Workflow](#release-workflow)
- [Issue Triage Workflow](#issue-triage-workflow)
- [CI/CD Monitoring](#cicd-monitoring)

---

## Quick PR Workflow

```bash
# 1. Create branch
git checkout -b feature/my-feature

# 2. Make changes, commit
git add . && git commit -m "Add feature"

# 3. Push branch
git push -u origin feature/my-feature

# 4. Create PR
gh pr create --fill

# Or all at once with title
gh pr create --title "Add feature" --body "Description"
```

---

## Code Review Workflow

```bash
# 1. List open PRs
gh pr list

# 2. Checkout PR locally
gh pr checkout 45

# 3. Review code
# ... inspect changes ...

# 4. View diff
gh pr diff 45

# 5. Check CI status
gh pr checks 45

# 6. Submit review
gh pr review 45 --approve
# or
gh pr review 45 --request-changes --body "Please fix X"

# 7. Merge when ready
gh pr merge 45 --squash --delete-branch
```

---

## Release Workflow

```bash
# 1. Create tag
git tag v1.0.0
git push origin v1.0.0

# 2. Create release with auto-generated notes
gh release create v1.0.0 --generate-notes

# 3. Upload assets
gh release upload v1.0.0 ./dist/app.zip ./dist/app.tar.gz

# Alternative: Create release with custom notes
gh release create v1.0.0 --title "Version 1.0" --notes "
## What's New
- Feature A
- Feature B

## Bug Fixes
- Fixed issue #123
"
```

---

## Issue Triage Workflow

```bash
# 1. List unlabeled issues
gh issue list --label ""

# 2. View issue
gh issue view 123

# 3. Add labels
gh issue edit 123 --add-label "bug,priority:high"

# 4. Assign
gh issue edit 123 --add-assignee @me

# 5. Comment
gh issue comment 123 --body "Looking into this"

# 6. Close when done
gh issue close 123 --comment "Fixed in #456"
```

---

## CI/CD Monitoring

```bash
# 1. Check current PR status
gh pr checks

# 2. Watch running workflow
gh run watch

# 3. View failed run logs
gh run view --log-failed

# 4. Rerun failed jobs
gh run rerun --failed

# 5. List all recent runs
gh run list --limit 20

# 6. Filter by workflow
gh run list --workflow=test.yml
```

---

## Fork & Contribute Workflow

```bash
# 1. Fork and clone
gh repo fork owner/repo --clone

# 2. Create feature branch
git checkout -b feature/contribution

# 3. Make changes, commit, push
git add . && git commit -m "Add feature"
git push -u origin feature/contribution

# 4. Create PR to upstream
gh pr create --base owner:main --head youruser:feature/contribution
```

---

## Quick Commands

| Task | Command |
|------|---------|
| Create PR from branch | `gh pr create --fill` |
| Checkout PR | `gh pr checkout 45` |
| Merge with squash | `gh pr merge 45 --squash --delete-branch` |
| Check CI status | `gh pr checks` |
| Watch CI run | `gh run watch` |
| Create release | `gh release create v1.0 --generate-notes` |
| Close issue | `gh issue close 123` |

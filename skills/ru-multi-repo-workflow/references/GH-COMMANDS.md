# gh CLI Command Reference

> **Quick lookup:** Ctrl+F for what you need.

---

## Command Index

| Category | Commands |
|----------|----------|
| [Issues](#issues) | list, view, comment, close, reopen, edit |
| [Pull Requests](#pull-requests) | list, view, diff, close, comment |
| [Repos](#repos) | list, view, clone |
| [Auth](#auth) | status, login, token |

---

## Issues

### List Issues

```bash
# Basic listing
gh issue list -R owner/repo

# With state filter
gh issue list -R owner/repo --state open
gh issue list -R owner/repo --state closed
gh issue list -R owner/repo --state all

# With limit
gh issue list -R owner/repo --limit 100

# JSON output (for parsing)
gh issue list -R owner/repo --json number,title,createdAt,labels,author,state

# Filter by label
gh issue list -R owner/repo --label "bug"
gh issue list -R owner/repo --label "bug" --label "high-priority"

# Filter by author
gh issue list -R owner/repo --author username

# Filter by assignee
gh issue list -R owner/repo --assignee username
gh issue list -R owner/repo --assignee @me

# Search within issues
gh issue list -R owner/repo --search "keyword in:title"
gh issue list -R owner/repo --search "keyword in:body"
```

### View Issue

```bash
# Basic view (human-readable)
gh issue view NUMBER -R owner/repo

# Include comments
gh issue view NUMBER -R owner/repo --comments

# JSON output
gh issue view NUMBER -R owner/repo --json number,title,body,createdAt,author,labels,comments
```

### Comment on Issue

```bash
# Add comment
gh issue comment NUMBER -R owner/repo -b "Your message here"

# Multi-line comment (heredoc)
gh issue comment NUMBER -R owner/repo -b "$(cat <<'EOF'
First line.

Second paragraph with details.

- Bullet point
- Another point
EOF
)"

# Comment from file
gh issue comment NUMBER -R owner/repo -F /path/to/message.md
```

### Close Issue

```bash
# Close with comment
gh issue close NUMBER -R owner/repo -c "Closing because: reason"

# Close without comment
gh issue close NUMBER -R owner/repo

# Close as not planned
gh issue close NUMBER -R owner/repo --reason "not planned" -c "Won't implement"

# Close as completed
gh issue close NUMBER -R owner/repo --reason "completed" -c "Fixed in abc123"
```

### Reopen Issue

```bash
gh issue reopen NUMBER -R owner/repo
gh issue reopen NUMBER -R owner/repo -c "Reopening because: reason"
```

### Edit Issue

```bash
# Add label
gh issue edit NUMBER -R owner/repo --add-label "bug"

# Remove label
gh issue edit NUMBER -R owner/repo --remove-label "needs-triage"

# Change title
gh issue edit NUMBER -R owner/repo --title "New title"

# Add assignee
gh issue edit NUMBER -R owner/repo --add-assignee username
```

---

## Pull Requests

### List PRs

```bash
# Basic listing
gh pr list -R owner/repo

# With state filter
gh pr list -R owner/repo --state open
gh pr list -R owner/repo --state closed
gh pr list -R owner/repo --state merged
gh pr list -R owner/repo --state all

# JSON output
gh pr list -R owner/repo --json number,title,createdAt,author,state,isDraft

# Filter by author
gh pr list -R owner/repo --author username

# Filter by base branch
gh pr list -R owner/repo --base main
```

### View PR

```bash
# Basic view
gh pr view NUMBER -R owner/repo

# Include comments
gh pr view NUMBER -R owner/repo --comments

# JSON output
gh pr view NUMBER -R owner/repo --json number,title,body,createdAt,author,files,commits
```

### View PR Diff

```bash
# Full diff (THE INTEL)
gh pr diff NUMBER -R owner/repo

# Diff with color
gh pr diff NUMBER -R owner/repo --color always | less -R

# List changed files only
gh pr view NUMBER -R owner/repo --json files | jq '.files[].path' -r
```

### Close PR (Never Merge!)

```bash
# Close with comment
gh pr close NUMBER -R owner/repo -c "Thanks! See policy note about contributions."

# Close without comment
gh pr close NUMBER -R owner/repo

# Delete branch after close (if in your repo)
gh pr close NUMBER -R owner/repo --delete-branch
```

### Comment on PR

```bash
gh pr comment NUMBER -R owner/repo -b "Your message here"
```

---

## Repos

### List Your Repos

```bash
# All your repos
gh repo list --limit 100

# JSON output
gh repo list --json name,owner,isPrivate --limit 100

# Filter by visibility
gh repo list --visibility public
gh repo list --visibility private
```

### View Repo

```bash
gh repo view owner/repo
gh repo view owner/repo --json name,description,defaultBranchRef,issues,pullRequests
```

---

## Auth

### Check Auth Status

```bash
gh auth status
```

### Login

```bash
gh auth login
gh auth login --with-token < token.txt
```

### Get Token

```bash
gh auth token
```

---

## JSON Output Patterns

### Common Fields

```bash
# Issues
--json number,title,body,createdAt,updatedAt,author,labels,state,comments

# PRs
--json number,title,body,createdAt,author,state,isDraft,files,commits,comments

# Repos
--json name,owner,description,isPrivate,defaultBranchRef
```

### jq Combinations

```bash
# Filter 2025+ issues
gh issue list -R owner/repo --json number,title,createdAt --limit 100 \
  | jq '[.[] | select(.createdAt >= "2025-01-01T00:00:00Z")]'

# Get just numbers
gh issue list -R owner/repo --json number | jq '.[].number'

# Format as "NUM: TITLE"
gh issue list -R owner/repo --json number,title \
  | jq '.[] | "\(.number): \(.title)"' -r

# Count by label
gh issue list -R owner/repo --json labels --limit 100 \
  | jq '[.[].labels[].name] | group_by(.) | map({label: .[0], count: length})'

# Get author usernames
gh issue list -R owner/repo --json author --limit 100 \
  | jq '[.[].author.login] | unique'
```

---

## Batch Operations

### Process All Open Issues

```bash
REPO="owner/repo"

for num in $(gh issue list -R "$REPO" --json number | jq '.[].number'); do
  echo "=== Issue #$num ==="
  gh issue view "$num" -R "$REPO"
  echo ""
  # Add your processing logic here
done
```

### Close Multiple Issues

```bash
REPO="owner/repo"
ISSUES="1 2 3 4 5"

for num in $ISSUES; do
  gh issue close "$num" -R "$REPO" -c "Batch close: reason"
done
```

### Comment on Multiple Issues

```bash
REPO="owner/repo"
MESSAGE="This has been addressed in the latest release."

for num in $(gh issue list -R "$REPO" --label "fixed" --json number | jq '.[].number'); do
  gh issue comment "$num" -R "$REPO" -b "$MESSAGE"
  gh issue close "$num" -R "$REPO"
done
```

---

## Error Handling

| Error | Cause | Fix |
|-------|-------|-----|
| `gh: command not found` | gh not installed | `brew install gh` |
| `authentication required` | Not logged in | `gh auth login` |
| `Could not resolve to a Repository` | Wrong repo name | Check owner/repo format |
| `rate limit exceeded` | API quota exhausted | Wait or use token with higher limit |
| `resource not accessible` | Permission denied | Check repo access |

---

## Pro Tips

### Use Aliases

```bash
# In ~/.config/gh/config.yml or via gh alias set
gh alias set il 'issue list'
gh alias set iv 'issue view'
gh alias set ic 'issue close'

# Usage
gh il -R owner/repo
gh iv 42 -R owner/repo
gh ic 42 -R owner/repo -c "Fixed"
```

### Cache Repo Context

```bash
# Inside a cloned repo, -R is optional
cd /data/projects/my_repo
gh issue list  # Automatically uses origin remote
```

### Pipe-Friendly Output

```bash
# Always use --json for scripting
gh issue list -R owner/repo --json number,title | jq ...

# Human output goes to stderr with --json
gh issue list -R owner/repo --json number 2>/dev/null | jq ...
```

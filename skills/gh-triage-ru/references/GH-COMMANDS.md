# gh Command Reference for Issue/PR Triage

Quick reference for `gh` commands used in issue/PR management.

---

## Discovery

```bash
# List all open issues
gh issue list -R owner/repo --state open

# List with JSON for scripting
gh issue list -R owner/repo --state open --json number,title,createdAt,author

# Filter to 2025+
gh issue list -R owner/repo --state open --json number,title,createdAt | \
  jq '.[] | select(.createdAt >= "2025-01-01")'

# List open PRs
gh pr list -R owner/repo --state open

# Count open items
gh issue list -R owner/repo --state open --json number | jq length
```

---

## Viewing Details

### Issues

```bash
# View issue
gh issue view NUMBER -R owner/repo

# View with comments
gh issue view NUMBER -R owner/repo --comments

# JSON for parsing
gh issue view NUMBER -R owner/repo --json title,body,comments,createdAt,author,labels

# View specific fields
gh issue view NUMBER -R owner/repo --json body -q '.body'
```

### Pull Requests

```bash
# View PR summary
gh pr view NUMBER -R owner/repo

# View the diff (CRITICAL)
gh pr diff NUMBER -R owner/repo

# View with comments
gh pr view NUMBER -R owner/repo --comments

# JSON for parsing
gh pr view NUMBER -R owner/repo --json title,body,files,additions,deletions
```

---

## Taking Action

### Closing Issues

```bash
# Close with comment
gh issue close NUMBER -R owner/repo -c "Reason for closing"

# Close without comment (not recommended)
gh issue close NUMBER -R owner/repo

# Reopen
gh issue reopen NUMBER -R owner/repo
```

### Commenting

```bash
# Add comment
gh issue comment NUMBER -R owner/repo -b "Your comment here"

# Multi-line comment (HEREDOC)
gh issue comment NUMBER -R owner/repo -b "$(cat <<'EOF'
First line

Second paragraph.

- Bullet 1
- Bullet 2
EOF
)"
```

### Closing PRs

```bash
# Close without merging (ALWAYS USE THIS)
gh pr close NUMBER -R owner/repo -c "Reason for closing"

# NEVER DO THIS
gh pr merge NUMBER  # DON'T
```

---

## Labels

```bash
# Add label
gh issue edit NUMBER -R owner/repo --add-label "bug"

# Remove label
gh issue edit NUMBER -R owner/repo --remove-label "bug"

# List available labels
gh label list -R owner/repo
```

---

## Bulk Operations

```bash
# Close all pre-2024 issues as stale
gh issue list -R owner/repo --state open --json number,createdAt | \
  jq -r '.[] | select(.createdAt < "2024-01-01") | .number' | \
  xargs -I {} gh issue close {} -R owner/repo -c "Closing as stale"

# Add label to multiple issues
for num in 1 2 3 4 5; do
  gh issue edit $num -R owner/repo --add-label "reviewed"
done
```

---

## API Access (Advanced)

```bash
# Get issue comments via API
gh api repos/owner/repo/issues/NUMBER/comments

# Get PR review comments
gh api repos/owner/repo/pulls/NUMBER/comments

# Get PR files changed
gh api repos/owner/repo/pulls/NUMBER/files
```

---

## ru Integration

```bash
# Discovery via ru (recommended)
ru review --dry-run --json 2>/dev/null | jq '.items'

# Filter to specific repo
ru review --dry-run --json 2>/dev/null | \
  jq '[.items[] | select(.repo == "owner/repo")]'

# Summary stats
ru review --dry-run --json 2>/dev/null | jq '.summary'
```

---

## Tips

1. **Always use `-R owner/repo`** - Avoid ambiguity
2. **Use `--json` for scripting** - Parseable output
3. **Use jq for filtering** - More flexible than grep
4. **Check rate limits** - `gh api rate_limit`
5. **HEREDOC for long comments** - Preserves formatting

# Pitfalls & Troubleshooting

> **Quick lookup:** Ctrl+F for your error message or symptom.

## Contents

| Section | Jump |
|---------|------|
| Quick Diagnosis | [→](#quick-diagnosis) |
| ru Problems | [→](#ru-problems) |
| gh Problems | [→](#gh-problems) |
| GH Actions Problems | [→](#gh-actions-problems-phase-4) |
| Workflow Pitfalls | [→](#workflow-pitfalls) |
| jq Problems | [→](#jq-problems) |
| Policy Mistakes | [→](#policy-mistakes) |
| Diagnostic Workflow | [→](#diagnostic-workflow) |
| Recovery Commands | [→](#recovery-commands) |
| Pro Tips | [→](#pro-tips) |

---

## Quick Diagnosis

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| `ru: command not found` | ru not in PATH | Add to PATH or use full path |
| `gh: command not found` | gh not installed | `brew install gh` |
| `authentication required` | gh not logged in | `gh auth login` |
| 0 issues but repo has issues | Wrong repo name format | Use `owner/repo` exactly |
| ru sync fails | Auth or network | `ru doctor`, check `gh auth status` |
| ru review hangs | ntm/tmux issue | Use `--mode=local` |
| Stale local code | Didn't sync | Always `ru sync` first |
| Wrong issue closed | Copy-paste error | Reopen with `gh issue reopen` |
| No workflow triggered | Wrong branch or no trigger | Check `on:` in workflow yaml |
| CI fails, passes locally | Environment diff | Check CI logs for versions |
| Flaky test | Timing/network in tests | `gh run rerun RUN_ID --failed` |

---

## ru Problems

### `ru: command not found`

```bash
# Check if installed
which ru
ls -la /data/projects/repo_updater/ru

# Add to PATH (temporary)
export PATH="/data/projects/repo_updater:$PATH"

# Add to PATH (permanent)
echo 'export PATH="/data/projects/repo_updater:$PATH"' >> ~/.bashrc
```

### `ru doctor` Fails

```bash
# Check dependencies
which git gh jq

# Check gh auth
gh auth status

# Check network
curl -I https://api.github.com
```

### `ru sync` Fails on Some Repos

```bash
# Check which failed
ru sync --json 2>/dev/null | jq '[.repos[] | select(.status == "failed")]'

# Common causes:
# - Auth issue: gh auth login
# - Dirty working tree: commit or stash first (carefully!)
# - Network timeout: retry
# - Repo deleted: remove from config
```

### `ru review` Hangs or Fails

```bash
# Use local mode instead of ntm
ru review --mode=local --dry-run

# Check ntm status
ntm --robot-status

# Kill stuck sessions
tmux kill-server  # Nuclear option
```

### `ru review --dry-run` Shows No Items

```bash
# Check if repos have issues
gh issue list -R owner/repo --state open

# May need to refresh
ru sync -j4

# Check ru config
cat ~/.config/ru/config
```

---

## gh Problems

### `gh: command not found`

```bash
# Install
brew install gh        # macOS
apt install gh         # Ubuntu/Debian
```

### `authentication required`

```bash
gh auth login

# Or with token
echo "YOUR_TOKEN" | gh auth login --with-token
```

### `Could not resolve to a Repository`

```bash
# Wrong format
gh issue list -R repo           # WRONG
gh issue list -R owner/repo     # RIGHT

# Check exact name
gh repo view owner/repo
```

### `rate limit exceeded`

```bash
# Check rate limit
gh api rate_limit | jq '.rate'

# Wait for reset
gh api rate_limit | jq '.rate.reset | todate'

# Use authenticated requests (higher limit)
gh auth status  # Ensure logged in
```

### `resource not accessible by integration`

```bash
# Check you have access
gh repo view owner/repo

# May need different auth scope
gh auth refresh -s repo
```

---

## GH Actions Problems (Phase 4)

### No Workflow Triggered After Push

```bash
# Check if workflows exist
ls .github/workflows/*.yml

# Check workflow triggers
grep -A5 "^on:" .github/workflows/*.yml

# Common cause: pushing to wrong branch
git branch --show-current
# Workflow may only trigger on main/master
```

### `gh run watch` Shows Nothing

```bash
# Check if any runs exist
gh run list --limit 5

# May need to wait a moment after push
sleep 5 && gh run list --limit 1
```

### CI Fails But Passes Locally

```bash
# Common causes:
# 1. Different OS (CI is usually Linux)
# 2. Different tool versions
# 3. Missing env vars in CI
# 4. Hardcoded paths

# Check CI environment
gh run view RUN_ID --log | head -100

# Check for version mismatches
grep -E "version|using" .github/workflows/*.yml
```

### Flaky Tests (Pass Sometimes, Fail Others)

```bash
# Rerun just the failed jobs
gh run rerun RUN_ID --failed

# If still flaky, check for:
# - Timing dependencies
# - Network calls without mocks
# - Shared state between tests
# - Random seed issues
```

### Release Workflow Fails

```bash
# Common causes:
# 1. Tag already exists on remote
# 2. Signing key expired/missing
# 3. Artifact upload failed
# 4. Version mismatch

# Check release workflow logs
gh run view RUN_ID --log-failed

# Delete and re-push tag if needed
git tag -d v1.2.3
git push origin :refs/tags/v1.2.3
# fix issue, then:
git tag v1.2.3
git push origin v1.2.3
```

### Can't See Workflow Logs

```bash
# Download logs for offline viewing
gh run download RUN_ID

# View specific job logs
gh run view RUN_ID --log --job=JOB_ID

# List jobs in a run
gh run view RUN_ID --json jobs | jq '.jobs[].name'
```

### Rate Limited on CI Checks

```bash
# Check rate limit
gh api rate_limit | jq '.rate'

# Wait and retry
# GH Actions API has separate limits from regular API
```

---

## Workflow Pitfalls

### Closing Wrong Issue

```bash
# Reopen immediately
gh issue reopen NUMBER -R owner/repo -c "Closed in error, reopening."
```

### Responding to Wrong Repo

```bash
# Always double-check -R flag
gh issue view 42 -R owner/repo  # Verify before acting
gh issue close 42 -R owner/repo -c "..."
```

### Trusting User-Submitted Code

**Never do this:**
```bash
# WRONG: Running user's code
git checkout user-branch
cargo run  # Executing untrusted code!
```

**Do this instead:**
```bash
# RIGHT: Read the diff, understand the fix, implement yourself
gh pr diff NUMBER -R owner/repo
# Read and understand
# Implement your own fix based on understanding
```

### Forgetting to Sync First

```bash
# Your local code is stale
# Bug appears "not reproduced" but actually you're testing old code

# ALWAYS sync first
ru sync -j4
# THEN verify issues
```

### Processing Pre-2025 Issues as Current

```bash
# Filter by date FIRST
gh issue list -R owner/repo --json number,title,createdAt --limit 100 \
  | jq '[.[] | select(.createdAt >= "2025-01-01T00:00:00Z")]'

# Don't waste time on stale issues
```

---

## jq Problems

### jq Returns null

```bash
# Check structure first
gh issue list -R owner/repo --json number,title | jq 'keys'
gh issue list -R owner/repo --json number,title | jq '.[0] | keys'

# Use // for defaults
| jq '.items // []'
```

### jq Filter Returns Empty

```bash
# Debug step by step
| jq 'length'              # How many items?
| jq '.[0]'                # What does first look like?
| jq '.[0].createdAt'      # Does field exist?
```

### Date Comparison Issues

```bash
# ISO 8601 string comparison works for dates
| jq 'select(.createdAt >= "2025-01-01T00:00:00Z")'

# Note the T and Z for full ISO format
# gh uses: 2025-01-15T10:30:00Z
# Not: 2025-01-15 or 2025-01-15 10:30:00
```

---

## Policy Mistakes

### Accidentally Merging a PR

**This should never happen, but if it does:**

```bash
# Revert immediately
git revert MERGE_COMMIT_SHA
git push

# Comment on PR
gh pr comment NUMBER -R owner/repo -b "Merged in error, reverted. Per project policy, we don't accept outside contributions."
```

### Implementing Feature Without Approval

If you implemented something complex without checking with the user:

1. Don't push yet
2. Describe what you built
3. Ask if they want it
4. Be prepared to discard

### Closing Valid Issue as Invalid

```bash
# Reopen with apology
gh issue reopen NUMBER -R owner/repo -c "Reopening—I misread this initially. Taking another look."
```

---

## Diagnostic Workflow

When something isn't working:

```bash
# 1. Check tools
which ru gh git jq

# 2. Check auth
gh auth status

# 3. Check ru health
ru doctor

# 4. Sync repos
ru sync -j4

# 5. Simple test
gh issue list -R YOUR_KNOWN_REPO --limit 5

# 6. Check rate limit
gh api rate_limit | jq '.rate.remaining'
```

---

## Recovery Commands

### Undo Accidental Close

```bash
gh issue reopen NUMBER -R owner/repo
gh pr reopen NUMBER -R owner/repo  # If PR
```

### Fix Wrong Comment

```bash
# Can't edit via CLI, must use web UI
# Or add correction comment
gh issue comment NUMBER -R owner/repo -b "Correction to above: ..."
```

### Recover from Stale Local Repo

```bash
# Hard reset to remote (CAREFUL: loses local changes)
git -C /data/projects/REPO fetch origin
git -C /data/projects/REPO reset --hard origin/main

# Or safer: just re-clone
rm -rf /data/projects/REPO
gh repo clone owner/repo /data/projects/REPO
```

---

## Pro Tips

### Dry Run Everything First

```bash
# See what ru would do
ru review --dry-run

# Check issue before closing
gh issue view NUMBER -R owner/repo
# THEN close
gh issue close NUMBER -R owner/repo -c "..."
```

### Use Shell History

```bash
# Re-run last gh command with different number
gh issue view 42 -R owner/repo
# Edit: change 42 to 43
gh issue view 43 -R owner/repo
```

### Keep Response Templates Ready

```bash
# In a file: ~/.config/responses/stale.md
cat > /tmp/response.md << 'EOF'
Closing as stale—the codebase has changed significantly since this was filed. Please reopen with fresh reproduction steps if still relevant.
EOF

gh issue close NUMBER -R owner/repo -F /tmp/response.md
```

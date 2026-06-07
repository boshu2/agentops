# GitHub CLI Commands — Complete Reference

## Table of Contents
- [Authentication](#authentication)
- [Repositories](#repositories)
- [Pull Requests](#pull-requests)
- [Issues](#issues)
- [Actions](#actions)
- [Releases](#releases)
- [Gists](#gists)
- [Search](#search)
- [Secrets & Variables](#secrets--variables)
- [SSH & GPG Keys](#ssh--gpg-keys)
- [Extensions & Config](#extensions--config)

---

## Authentication

```bash
# Check auth status
gh auth status

# Login
gh auth login

# Refresh token
gh auth refresh

# Logout
gh auth logout
```

---

## Repositories

```bash
# Clone repository
gh repo clone owner/repo

# Create new repo
gh repo create my-repo --public --source=. --push

# Fork repository
gh repo fork owner/repo --clone

# View repo in browser
gh repo view --web

# List your repos
gh repo list

# Delete repo
gh repo delete owner/repo --yes
```

---

## Pull Requests

```bash
# List PRs
gh pr list

# Create PR
gh pr create --title "Add feature" --body "Description"
gh pr create --fill

# View PR
gh pr view 45
gh pr view 45 --web

# Checkout PR locally
gh pr checkout 45

# Review PR
gh pr review 45 --approve
gh pr review 45 --request-changes --body "Please fix X"
gh pr review 45 --comment --body "Looks good but..."

# Merge PR
gh pr merge 45 --squash
gh pr merge 45 --merge
gh pr merge 45 --rebase
gh pr merge 45 --squash --delete-branch

# Close PR
gh pr close 45

# List PR checks
gh pr checks 45

# View PR diff
gh pr diff 45

# Edit PR
gh pr edit 45 --title "New title"
gh pr edit 45 --add-label "bug"
gh pr edit 45 --add-reviewer username
```

---

## Issues

```bash
# List issues
gh issue list
gh issue list --state closed
gh issue list --label "bug"

# Create issue
gh issue create --title "Bug: Login fails" --body "Description"
gh issue create  # interactive

# View issue
gh issue view 123

# Close issue
gh issue close 123

# Reopen issue
gh issue reopen 123

# Comment on issue
gh issue comment 123 --body "Working on this"

# Edit issue
gh issue edit 123 --add-assignee @me
gh issue edit 123 --add-label "bug,priority:high"
gh issue edit 123 --title "New title"
```

---

## Actions

```bash
# List workflow runs
gh run list

# View run details
gh run view 12345

# Watch run in progress
gh run watch 12345

# View run logs
gh run view 12345 --log

# Rerun failed jobs
gh run rerun 12345 --failed

# List workflows
gh workflow list

# Run workflow manually
gh workflow run deploy.yml
gh workflow run deploy.yml -f environment=production

# Disable/enable workflow
gh workflow disable deploy.yml
gh workflow enable deploy.yml
```

---

## Releases

```bash
# List releases
gh release list

# Create release
gh release create v1.0.0 --title "Version 1.0" --notes "Release notes"
gh release create v1.0.0 --generate-notes

# Upload assets
gh release upload v1.0.0 ./dist/app.zip

# Download assets
gh release download v1.0.0

# Delete release
gh release delete v1.0.0

# View release
gh release view v1.0.0
```

---

## Gists

```bash
# Create gist
gh gist create file.txt --public

# Create from stdin
echo "Hello" | gh gist create -

# List gists
gh gist list

# View gist
gh gist view GIST_ID

# Edit gist
gh gist edit GIST_ID

# Delete gist
gh gist delete GIST_ID
```

---

## Search

```bash
# Search repos
gh search repos "react hooks" --limit 10

# Search issues
gh search issues "bug authentication" --repo owner/repo

# Search PRs
gh search prs "fix memory leak" --state open

# Search code
gh search code "function handleAuth" --repo owner/repo
```

---

## Secrets & Variables

### Secrets (for Actions)

```bash
# List secrets
gh secret list

# Set secret
gh secret set MY_SECRET

# Set from file
gh secret set MY_SECRET < secret.txt

# Delete secret
gh secret delete MY_SECRET
```

### Variables (for Actions)

```bash
# List variables
gh variable list

# Set variable
gh variable set MY_VAR --body "value"

# Delete variable
gh variable delete MY_VAR
```

---

## SSH & GPG Keys

```bash
# List SSH keys
gh ssh-key list

# Add SSH key
gh ssh-key add ~/.ssh/id_ed25519.pub --title "My laptop"

# Delete SSH key
gh ssh-key delete KEY_ID

# List GPG keys
gh gpg-key list

# Add GPG key
gh gpg-key add key.gpg
```

---

## Extensions & Config

### Extensions

```bash
# List installed extensions
gh extension list

# Install extension
gh extension install owner/gh-extension

# Browse extensions
gh extension browse

# Upgrade extensions
gh extension upgrade --all
```

### Aliases

```bash
# Create alias
gh alias set pv 'pr view'

# List aliases
gh alias list

# Delete alias
gh alias delete pv
```

### Configuration

```bash
# View config
gh config list

# Set default editor
gh config set editor vim

# Set default browser
gh config set browser "open"

# Set git protocol
gh config set git_protocol ssh
```

---

## Labels & Projects

### Labels

```bash
# List labels
gh label list

# Create label
gh label create "priority:high" --color FF0000 --description "High priority"

# Delete label
gh label delete "priority:high"
```

### Projects

```bash
# List projects
gh project list

# View project
gh project view 1

# Create project
gh project create --title "My Project"
```

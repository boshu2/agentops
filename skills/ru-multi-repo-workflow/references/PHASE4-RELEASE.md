# Phase 4: Release & Monitor — Deep Reference

> **Goal:** Ensure CI passes after commits. Iterate on failures until green. Never leave broken builds.

## Contents

| Section | Jump |
|---------|------|
| Philosophy | [→](#the-philosophy) |
| Detection | [→](#detection-which-repos-have-ci) |
| The Monitor Loop | [→](#the-monitor-loop) |
| Ephemeral File Filter | [→](#ephemeral-file-filter) |
| Common CI Failures | [→](#common-ci-failures) |
| Iteration Strategy | [→](#iteration-strategy) |
| When to Surface | [→](#when-to-surface-vs-just-fix) |
| Validation | [→](#validation) |

---

## The Philosophy

**You break it, you fix it.** A push that breaks CI is not done until CI is green.

The iteration loop:
1. Push
2. Watch
3. If red: diagnose → fix → push → watch
4. Repeat until green

**Don't leave broken builds for "later".** Later never comes.

---

## Detection: Which Repos Have CI?

### Quick Check

```bash
# Has any workflow?
test -d .github/workflows && ls .github/workflows/*.yml 2>/dev/null

# Has release workflow?
grep -l "release" .github/workflows/*.yml 2>/dev/null

# Has CI that runs on push?
grep -l "push:" .github/workflows/*.yml 2>/dev/null
```

### Categorize Repos

| Has `.github/workflows/`? | Has release.yml? | Action |
|---------------------------|------------------|--------|
| No | — | Commit only, done |
| Yes | No | Commit, watch CI, iterate |
| Yes | Yes | Commit, watch CI, optionally tag for release |

### Batch Detection Across Repos

```bash
# Find all repos with CI
for repo in /data/projects/*/; do
  if ls "$repo/.github/workflows/"*.yml 2>/dev/null | head -1 > /dev/null; then
    echo "CI: $(basename $repo)"
  fi
done
```

---

## The Monitor Loop

### Step 1: Push and Observe

```bash
git push

# Immediately check if workflow triggered
gh run list --limit 3
```

### Step 2: Watch Active Run

```bash
# Blocks until complete, shows live status
gh run watch

# Or watch specific run
gh run watch RUN_ID
```

### Step 3: Handle Failure

```bash
# Get failed logs
gh run view RUN_ID --log-failed

# Get full logs for a job
gh run view RUN_ID --log --job=JOB_ID

# Download all logs (for complex debugging)
gh run download RUN_ID
```

### Step 4: Fix and Iterate

```bash
# Make the fix
# ... edit files ...

# Commit with context
git add . && git commit -m "$(cat <<'EOF'
fix(ci): resolve test failure in auth module

- TestAuthRefresh was timing out due to missing mock
- Added proper timeout handling

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"

# Push and watch again
git push && gh run watch
```

### Step 5: Confirm Green

```bash
# Verify the run succeeded
gh run list --limit 1

# Should show ✓ completed
```

---

## Ephemeral File Filter

### NEVER Commit These

```gitignore
# === TRANSIENT ===
*.log
*.tmp
*.swp
*.bak
*~
.cache/

# === BUILD ARTIFACTS ===
target/           # Rust
node_modules/     # Node
dist/             # General
build/            # General
__pycache__/      # Python
*.pyc
.next/            # Next.js
.turbo/           # Turborepo

# === SECRETS (CRITICAL) ===
.env
.env.*
*.key
*.pem
*.p12
credentials.*
secrets.*
**/secrets/
.gcloud/
.aws/

# === IDE/EDITOR ===
.idea/
.vscode/
*.sublime-*
.project
.classpath

# === OS JUNK ===
.DS_Store
Thumbs.db
desktop.ini
```

### Decision Matrix

| File Pattern | Commit? | Reason |
|--------------|---------|--------|
| `src/**/*.rs` | Yes | Source code |
| `tests/**/*` | Yes | Test code |
| `Cargo.lock` | Yes | Reproducible builds |
| `target/debug/*` | **NO** | Build artifact |
| `.env` | **NEVER** | Contains secrets |
| `.env.example` | Yes | Template, no secrets |
| `coverage/` | No | Generated |
| `*.min.js` | Usually no | Generated, unless vendored |

### Pre-Commit Check

```bash
# Scan for potential secrets before commit
git diff --cached --name-only | xargs -I{} sh -c '
  grep -lE "(password|secret|key|token|api_key|private)" "{}" 2>/dev/null && echo "⚠️  Check: {}"
'
```

---

## Common CI Failures

### Test Failures

```bash
# Get the failing test output
gh run view RUN_ID --log-failed | grep -A 20 "FAILED\|Error\|assertion"

# Common causes:
# - Flaky test (timing, network)
# - Missing mock/fixture
# - Actual regression
```

**Fix pattern:**
```bash
# Run locally first
cargo test  # or npm test, pytest, etc.

# If passes locally but fails CI:
# - Check CI environment differences
# - Check for hardcoded paths
# - Check for timezone issues
```

### Build Failures

```bash
# Typical causes:
# - Missing dependency
# - Version mismatch
# - Platform-specific issue

# Check the build step logs
gh run view RUN_ID --log | grep -B 5 "error\[E"
```

### Linter/Format Failures

```bash
# Auto-fix locally
cargo fmt           # Rust
npm run lint:fix    # JS/TS
black .             # Python

# Then commit the fix
git add . && git commit -m "style: auto-format"
```

### Dependency Issues

```bash
# Lock file out of sync
cargo update        # Rust
npm install         # Node (regenerates lock)
pip freeze > requirements.txt  # Python

# Commit updated lock file
git add Cargo.lock && git commit -m "chore: update lock file"
```

---

## Iteration Strategy

### The Golden Rule

**Each fix should be a focused commit.** Don't bundle unrelated fixes.

### Iteration Checklist

- [ ] Read the FULL error message
- [ ] Reproduce locally if possible
- [ ] Make minimal fix
- [ ] Commit with context
- [ ] Push and watch
- [ ] Repeat if still red

### When to Stop Iterating

| Scenario | Action |
|----------|--------|
| Fixed after 1-2 tries | Done, move on |
| Same error after 3 tries | **SURFACE** — may need judgment |
| Different error each time | Systematic issue, **SURFACE** |
| Flaky (passes sometimes) | Note and move on, or add retry |
| Environment-specific | May need CI config change |

---

## When to Surface vs Just Fix

### Just Fix (Mechanical)

- Formatting issues
- Missing import
- Typo in test
- Lock file sync
- Simple deprecation warning

### SURFACE (Needs Judgment)

- Persistent failure after 3 attempts
- Test that contradicts expected behavior
- CI config that seems wrong
- Security warning in dependencies
- Breaking change in upstream

### SURFACE FORMAT

```
🔴 CI FAILING: owner/repo — Run #12345

**Failure:** test_user_authentication
**Error:** "Connection refused to localhost:5432"

**Attempted fixes:**
1. Added postgres service to CI config — still fails
2. Checked if test needs network — unclear
3. Compared to other repos — they use mocks

**This may require judgment:**
- [ ] Should this test use a real DB or mock?
- [ ] Is the CI config missing a service?
- [ ] Is this test actually valuable?

**Logs:** `gh run view 12345 --log-failed`
**Workflow:** `.github/workflows/ci.yml:45`
```

---

## Release Workflows

### Tagging for Release

```bash
# After CI is green, if you want to release:
git tag v1.2.3
git push origin v1.2.3

# Watch the release workflow
gh run list --workflow=release.yml --limit 1
gh run watch
```

### Release Failure Recovery

```bash
# If release workflow fails:
gh run view RUN_ID --log-failed

# Common issues:
# - Signing key expired
# - Artifact upload failed
# - Version already exists

# Fix, delete tag, re-tag:
git tag -d v1.2.3
git push origin :refs/tags/v1.2.3
# ... fix the issue ...
git tag v1.2.3
git push origin v1.2.3
```

---

## Batch Operations

### Watch Multiple Repos

```bash
# Check all repos for running workflows
for repo in owner/repo1 owner/repo2; do
  echo "=== $repo ==="
  gh run list -R "$repo" --limit 2
done
```

### Find All Failing CI

```bash
# Across all repos you manage
gh run list --json status,name,headBranch,url \
  | jq '[.[] | select(.status == "failure")] | .[:10]'
```

---

## Validation

### Before Moving to Next Repo

```bash
# Confirm green
gh run list --limit 1 --json conclusion | jq '.[0].conclusion'
# Should output: "success"
```

### End of Session Check

```bash
# No failing runs across recent work
gh run list --limit 10 --json conclusion,name \
  | jq '[.[] | select(.conclusion == "failure")]'
# Should be empty: []
```

---

## Quick Reference

```bash
# The loop
git push
gh run watch
# if red:
gh run view RUN_ID --log-failed
# fix, commit, push, watch again

# Key commands
gh run list --limit 5              # Recent runs
gh run view RUN_ID                 # Run details
gh run view RUN_ID --log-failed    # Just failures
gh run watch                       # Live monitor
gh run rerun RUN_ID --failed       # Retry failed jobs
```

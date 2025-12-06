# Git Hooks for Auto-Codex Session Capture

This directory contains Git hooks that enable automatic session capture to the codex notebook.

## Hooks

### pre-commit (Codex agent index)

Keeps `.codex/agents-index.yaml` synchronized with `.claude/agents/*.md`.

**When it runs:** Before commit, but only if staged changes touch `.claude/agents/`
**What it does:** Runs `python3 tools/scripts/update-codex-agent-index.py` and stages the refreshed manifest.
**Can it fail?:** Yes – if regeneration fails. Fix the error, rerun `make codex-agents-index`, then retry commit.

### prepare-commit-msg

Injects the 4-section template into commit messages:

```
Context: [problem/request that triggered this]
Solution: [approach taken]
Learning: [reusable insight gained]
Impact: [what improved]
```

**When it runs:** Before you edit the commit message
**What it does:** Adds template if not already present
**Can it fail?:** No - always exits 0

### post-commit

Calls `capture_session.py` to extract session info and append to codex.

**When it runs:** After successful commit
**What it does:** Runs capture_session.py in background
**Can it fail?:** No - always exits 0, runs async
**Log location:** `.git/hooks/capture-session.log`

## Installation

### Automatic (recommended)

```bash
make install-hooks
```

### Manual

```bash
ln -sf ../../tools/scripts/git-hooks/pre-commit .git/hooks/pre-commit
ln -sf ../../tools/scripts/git-hooks/prepare-commit-msg .git/hooks/prepare-commit-msg
ln -sf ../../tools/scripts/git-hooks/post-commit .git/hooks/post-commit
chmod +x tools/scripts/git-hooks/pre-commit
chmod +x tools/scripts/git-hooks/prepare-commit-msg
chmod +x tools/scripts/git-hooks/post-commit
```

## Usage

Just commit normally:

```bash
git commit
# Template is injected automatically
# Fill out Context/Solution/Learning/Impact
# Commit completes
# Session is captured automatically
```

## Troubleshooting

### Hook not running

```bash
# Check if hooks are installed
ls -la .git/hooks/prepare-commit-msg
ls -la .git/hooks/post-commit

# Should be symlinks to tools/scripts/git-hooks/
```

### Template not appearing

```bash
# Test hook manually
.git/hooks/prepare-commit-msg .git/COMMIT_EDITMSG
cat .git/COMMIT_EDITMSG
# Should see template
```

### Capture not working

```bash
# Check log
tail -20 .git/hooks/capture-session.log

# Test capture manually
python3 tools/scripts/knowledge-os/capture_session.py --commit HEAD --dry-run
```

### Skip capture for one commit

```bash
git commit --no-verify
```

### Uninstall hooks

```bash
make uninstall-hooks
# Or manually:
rm .git/hooks/pre-commit
rm .git/hooks/prepare-commit-msg
rm .git/hooks/post-commit
```

## Safety

Hooks are designed to NEVER break commits:

- Always exit 0 (even on error)
- Run in background (don't block user)
- Log errors instead of printing
- Save backup if codex append fails

## Testing

```bash
# Test in isolation
make test-capture

# Test hooks in real repo
make install-hooks
git commit --allow-empty -m "test: Hook test"
# Check if template appeared
# Check if session was captured
tail docs/reference/codex-ops-notebook.md
```

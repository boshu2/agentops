#!/bin/bash
# Dangerous Git Operations Guard
# Blocks destructive git commands and suggests safe alternatives.

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# Read all stdin
INPUT=$(cat)

# Source shared helpers for structured failure output (from plugin install dir, not repo root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/hook-helpers.sh
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"

# Extract tool_input.command from JSON
COMMAND=$(echo "$INPUT" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/^"command"[[:space:]]*:[[:space:]]*"//;s/"$//')

# Hot path: no git, no problem
echo "$COMMAND" | grep -q "git" || exit 0

# Warn if .agents/ files may be staged. Commit attempts with staged additions or
# modifications are blocked below; deletions are allowed for one-time cleanup.
if echo "$COMMAND" | grep -qE 'git\s+add' && echo "$COMMAND" | grep -qE '\.agents/|\s\.\s*$|\s-A'; then
    echo "Warning: repo-root .agents/ is local runtime state and must stay gitignored. Review: git status .agents/" >&2
fi
if echo "$COMMAND" | grep -qE 'git\s+commit' && git diff --cached --name-only --diff-filter=ACMR -- .agents 2>/dev/null | grep -q '^\.agents/'; then
    write_failure "dangerous_git" "git commit .agents" 2 "repo-root .agents commit blocked"
    echo "Blocked: repo-root .agents/ is local runtime state. Use: git restore --staged .agents/" >&2
    exit 2
fi

# Allow-list (checked before block-list)
echo "$COMMAND" | grep -qE 'push.*--force-with-lease' && exit 0

# Block-list with safe alternatives
if echo "$COMMAND" | grep -qE 'push\s+.*(-f|--force)'; then
  write_failure "dangerous_git" "git push --force" 2 "force push blocked"
  echo "Blocked: force push. Use --force-with-lease instead." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'reset\s+--hard'; then
  write_failure "dangerous_git" "git reset --hard" 2 "hard reset blocked"
  echo "Blocked: hard reset. Use git stash or git reset --soft." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'clean\s+-f'; then
  write_failure "dangerous_git" "git clean -f" 2 "force clean blocked"
  echo "Blocked: force clean. Review with git clean -n first." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'checkout\s+\.'; then
  write_failure "dangerous_git" "git checkout ." 2 "checkout dot blocked"
  echo "Blocked: checkout dot. Use git stash to preserve changes." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'restore\s+(--staged\s+)?\.'; then
  write_failure "dangerous_git" "git restore ." 2 "restore dot blocked"
  echo "Blocked: restore dot. Use git stash to preserve changes." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'restore\s+--source'; then
  write_failure "dangerous_git" "git restore --source" 2 "restore from source blocked"
  echo "Blocked: restore from source. Use git stash or git diff to review first." >&2
  exit 2
fi

if echo "$COMMAND" | grep -qE 'branch\s+-D'; then
  write_failure "dangerous_git" "git branch -D" 2 "force branch delete blocked"
  echo "Blocked: force branch delete. Use git branch -d (safe delete)." >&2
  exit 2
fi

# No match — allow
exit 0

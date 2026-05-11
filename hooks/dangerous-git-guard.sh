#!/bin/bash
# Dangerous Git Operations Guard
# Blocks destructive git commands and suggests safe alternatives.
# practices: [sre, resilience-patterns, design-by-contract]

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# Read all stdin
INPUT=$(cat)

# Source shared helpers for structured failure output (from plugin install dir, not repo root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/../lib/hook-helpers.sh" ]]; then
  # shellcheck source=../lib/hook-helpers.sh
  . "$SCRIPT_DIR/../lib/hook-helpers.sh"
elif [[ -f "$SCRIPT_DIR/hook-helpers.sh" ]]; then
  # Backward-compatible fallback for older plugin caches that copied helpers
  # into hooks/ directly.
  # shellcheck source=../lib/hook-helpers.sh
  . "$SCRIPT_DIR/hook-helpers.sh"
else
  echo "Warning: AgentOps hook helper missing; dangerous-git-guard skipped." >&2
  exit 0
fi

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"

extract_command() {
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // empty' 2>/dev/null && return 0
  fi
  printf '%s' "$INPUT" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/^"command"[[:space:]]*:[[:space:]]*"//;s/"$//'
}

git_push_uses_force() {
  local command="$1"
  local token
  local saw_git=0
  local saw_push=0
  local -a tokens

  # This guard only needs enough tokenization to distinguish option tokens from
  # branch/ref names. Quoted whitespace in ref names is outside normal git use.
  read -r -a tokens <<< "$command"
  for token in "${tokens[@]}"; do
    token="${token#\'}"
    token="${token%\'}"
    token="${token#\"}"
    token="${token%\"}"

    if [[ "$saw_git" -eq 0 ]]; then
      [[ "$token" == "git" || "$token" == */git ]] && saw_git=1
      continue
    fi

    if [[ "$saw_push" -eq 0 ]]; then
      [[ "$token" == "push" ]] && saw_push=1
      continue
    fi

    [[ "$token" == "--" ]] && return 1
    case "$token" in
      --force-with-lease|--force-with-lease=*) ;;
      --force|--force=*) return 0 ;;
      -[!-]*)
        [[ "$token" == *f* ]] && return 0
        ;;
    esac
  done

  return 1
}

# Extract tool_input.command from JSON
COMMAND="$(extract_command)"

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

# Block-list with safe alternatives
if git_push_uses_force "$COMMAND"; then
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

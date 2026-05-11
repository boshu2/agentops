#!/bin/bash
# Edit Scope Guard — PreToolUse hook for /scope skill (issue soc-irg1.3).
# practices: [ddd-bounded-context, design-by-contract]
#
# Reads Claude Code tool-input JSON from stdin. If `.agents/scope.lock` declares
# one or more `frozen_dirs`, blocks edits whose target path is outside every
# frozen directory.
#
# Defensive defaults (per pre-mortem Finding 3):
#   - malformed JSON → exit 0 (fail-open, log warning to stderr)
#   - missing target path → exit 0 (nothing to check)
#   - missing or empty lock file → exit 0 (no enforcement)
#
# Activation tested by tests/hooks/test-edit-scope-guard-fires.sh.

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# --- Finding 3 amendment (verbatim defensive parse) ---
HOOK_INPUT=$(cat)
if ! echo "$HOOK_INPUT" | jq -e . >/dev/null 2>&1; then
  echo "edit-scope-guard: malformed JSON input, allowing edit (fail-open)" >&2
  exit 0
fi
TARGET_PATH=$(echo "$HOOK_INPUT" | jq -r '.tool.params.file_path // .tool.params.command // empty')
if [ -z "$TARGET_PATH" ]; then
  exit 0   # nothing to check; allow
fi
# --- end Finding 3 amendment ---

# Best-effort: if input came from a Bash tool, TARGET_PATH currently holds the
# full command line. Try to extract the first token that looks like a path
# (something with a `/` and no shell metacharacters).
case "$TARGET_PATH" in
  *' '*)
    BASH_PATH=$(echo "$TARGET_PATH" | tr ' ' '\n' | grep -E '^[A-Za-z0-9_./-]+/' | head -1 || true)
    if [ -n "$BASH_PATH" ]; then
      TARGET_PATH="$BASH_PATH"
    fi
    ;;
esac

# Resolve repo root and lock-file path. Wave 1 hardcodes `.agents/scope.lock`;
# Wave 2 (issue I5) migrates to `lib/ao-paths.sh`.
ROOT="${AO_SCOPE_LOCK_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
LOCK="${AO_SCOPE_LOCK:-$ROOT/.agents/scope.lock}"

# Lock missing or empty → no enforcement.
if [ ! -s "$LOCK" ]; then
  exit 0
fi

# Lock present but unreadable / malformed → fail-open (do not block edits).
if ! LOCK_CONTENTS=$(cat "$LOCK" 2>/dev/null) || ! echo "$LOCK_CONTENTS" | jq -e . >/dev/null 2>&1; then
  echo "edit-scope-guard: malformed lock file at $LOCK, allowing edit (fail-open)" >&2
  exit 0
fi

# Empty frozen_dirs → no enforcement.
FROZEN_COUNT=$(echo "$LOCK_CONTENTS" | jq -r '.frozen_dirs // [] | length')
if [ "$FROZEN_COUNT" = "0" ]; then
  exit 0
fi

# Normalize target path to repo-relative for prefix comparison.
case "$TARGET_PATH" in
  /*)
    REL_PATH="${TARGET_PATH#"$ROOT/"}"
    ;;
  *)
    REL_PATH="$TARGET_PATH"
    ;;
esac

FROZEN_DIRS_TXT=$(echo "$LOCK_CONTENTS" | jq -r '.frozen_dirs[]')

while IFS= read -r dir; do
  [ -z "$dir" ] && continue
  # Strip trailing slash so we can match both "foo" and "foo/bar".
  norm="${dir%/}"
  case "$REL_PATH" in
    "$norm"|"$norm"/*)
      exit 0
      ;;
  esac
done <<EOF
$FROZEN_DIRS_TXT
EOF

# No frozen dir matched — block.
JOINED=$(echo "$FROZEN_DIRS_TXT" | tr '\n' ',' | sed 's/,$//')
echo "edit-scope-guard: $REL_PATH outside frozen scope [$JOINED]" >&2
exit 2

#!/usr/bin/env bash
# PreToolUse hook: fast Go validation before git commit.
#
# Fast path (<5s): go vet + go build on every commit.
# Targeted tests: only for cli/internal/ changes (skip cmd/ao — deferred to pre-push).
#
# Blocking (exit 2) if checks fail — prevents committing broken code.
set -euo pipefail

[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0

INPUT=$(cat)
# Guard empty stdin: jq fails on empty input, so exit early
[[ -z "$INPUT" ]] && exit 0
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || TOOL_NAME=""
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || COMMAND=""

# Only trigger on Bash tool
[[ "$TOOL_NAME" == "Bash" ]] || exit 0

# Only trigger on git commit commands
case "$COMMAND" in
  *"git commit"*) ;;
  *) exit 0 ;;
esac

# Prefer user-local Go (~/.local/go) over system Go when present.
# Hooks don't inherit interactive-shell PATH, so the apt-provided go-1.22
# would otherwise be picked up even when the user has a newer local install.
if [[ -x "$HOME/.local/go/bin/go" ]]; then
  export PATH="$HOME/.local/go/bin:$PATH"
  export GOROOT="$HOME/.local/go"
fi

# Need go compiler
command -v go &>/dev/null || exit 0

# Must be in a Go project (has go.mod)
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
GO_MOD=""
for candidate in "$REPO_ROOT/go.mod" "$REPO_ROOT/cli/go.mod"; do
  if [[ -f "$candidate" ]]; then
    GO_MOD="$candidate"
    break
  fi
done
[[ -n "$GO_MOD" ]] || exit 0

GO_DIR="$(dirname "$GO_MOD")"

# Scope gate: skip the Go fast-gate entirely when no Go files are staged.
# Prevents this hook from blocking non-Go commits (docs, skills, shell, YAML).
# Matches *.go files and go.{mod,sum,work,work.sum} at any depth.
STAGED_FILES="$(git diff --cached --name-only 2>/dev/null || true)"
if ! printf '%s\n' "$STAGED_FILES" | grep -qE '\.go$|(^|/)go\.mod$|(^|/)go\.sum$|(^|/)go\.work(\.sum)?$'; then
  exit 0
fi

# --- Fast gate: vet + build (<5s) ---
echo "Pre-commit: go vet..." >&2
if ! (cd "$GO_DIR" && go vet ./... 2>&1) >&2; then
  echo "BLOCKED: go vet failed. Fix before committing." >&2
  exit 2
fi

echo "Pre-commit: go build..." >&2
if ! (cd "$GO_DIR" && go build ./... 2>&1) >&2; then
  echo "BLOCKED: go build failed. Fix before committing." >&2
  exit 2
fi

# --- Targeted tests: only cli/internal/ packages ---
STAGED_FILES="$(git diff --cached --name-only 2>/dev/null || true)"

# Check for cmd/ao changes — defer to pre-push
if printf '%s\n' "$STAGED_FILES" | grep -q 'cli/cmd/ao/'; then
  echo "Pre-commit: cmd/ao changes detected — full test suite deferred to pre-push" >&2
fi

# Run tests only for changed cli/internal/ packages
INTERNAL_CHANGED="$(printf '%s\n' "$STAGED_FILES" | grep '^cli/internal/' | grep '\.go$' || true)"
if [[ -n "$INTERNAL_CHANGED" ]]; then
  # Extract unique package directories relative to GO_DIR
  PKGS="$(printf '%s\n' "$INTERNAL_CHANGED" \
    | xargs -n1 dirname \
    | sort -u \
    | sed "s|^cli/|./|")"

  if [[ -n "$PKGS" ]]; then
    echo "Pre-commit: testing changed internal packages..." >&2
    # shellcheck disable=SC2086
    if ! (cd "$GO_DIR" && go test -count=1 -short $PKGS 2>&1 | tail -10) >&2; then
      echo "BLOCKED: internal package tests failed. Fix before committing." >&2
      exit 2
    fi
  fi
fi

exit 0

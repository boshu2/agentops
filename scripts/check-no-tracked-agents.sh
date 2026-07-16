#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${NO_TRACKED_AGENTS_REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$NO_TRACKED_AGENTS_REPO_ROOT" && pwd)"
else
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

# The one declarative exception: the AgentOps project config is repo
# configuration (e.g. the tracker pin), not session/runtime state, and must
# survive fresh clones. Everything else under .agents/ stays untracked.
ALLOWED_TRACKED='.agents/ao/config.yaml'

errors=0
tracked_all="$(git -C "$REPO_ROOT" ls-files -- .agents 2>/dev/null || true)"
deleted="$(
  git -C "$REPO_ROOT" diff --name-only --diff-filter=D HEAD -- .agents 2>/dev/null
  git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=D -- .agents 2>/dev/null
)"
tracked="$(comm -23 \
  <(printf '%s\n' "$tracked_all" | sed '/^$/d' | sort -u) \
  <(printf '%s\n' "$deleted" | sed '/^$/d' | sort -u) \
  | grep -Fxv "$ALLOWED_TRACKED" || true)"
staged="$(git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=ACMR -- .agents 2>/dev/null \
  | grep -Fxv "$ALLOWED_TRACKED" || true)"

if [[ -n "$tracked" ]]; then
  echo "ERROR: repo-root .agents paths are tracked:" >&2
  printf '%s\n' "$tracked" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ -n "$staged" ]]; then
  echo "ERROR: repo-root .agents paths are staged:" >&2
  printf '%s\n' "$staged" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ ! -f "$REPO_ROOT/.gitignore" ]] \
  || ! grep -Eq '^[[:space:]]*/\.agents/\*?[[:space:]]*($|#)' "$REPO_ROOT/.gitignore"; then
  echo "ERROR: root .gitignore must contain an explicit '/.agents/' (or '/.agents/*') rule." >&2
  errors=1
fi

if grep -nE '^[[:space:]]*!/?\.agents(/|$)' "$REPO_ROOT/.gitignore" \
  | grep -vE '!/\.agents/ao/($|config\.yaml$)' >&2; then
  echo "ERROR: root .gitignore must not re-include repo-root .agents paths (only .agents/ao/config.yaml is allowed)." >&2
  errors=1
fi

[[ "$errors" -eq 0 ]] || exit 1
echo "no tracked repo-root .agents state (config.yaml exception applied)"

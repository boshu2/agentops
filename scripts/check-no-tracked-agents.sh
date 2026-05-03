#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${NO_TRACKED_AGENTS_REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$NO_TRACKED_AGENTS_REPO_ROOT" && pwd)"
else
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

tracked="$(git -C "$REPO_ROOT" ls-files -- .agents 2>/dev/null || true)"
staged="$(git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=ACMR -- .agents 2>/dev/null || true)"
errors=0

if [[ -n "$tracked" ]]; then
  echo "ERROR: repo-root .agents paths are tracked. Remove them from the index:" >&2
  echo "  git rm -r --cached .agents" >&2
  echo "$tracked" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ -n "$staged" ]]; then
  echo "ERROR: repo-root .agents paths are staged for add/modify/rename/copy." >&2
  echo "These files are local agent runtime state and must not be committed:" >&2
  echo "$staged" | sed 's/^/  - /' >&2
  errors=1
fi

if [[ ! -f "$REPO_ROOT/.gitignore" ]]; then
  echo "ERROR: root .gitignore missing; cannot enforce /.agents/ ignore policy." >&2
  errors=1
else
  if ! grep -Eq '^[[:space:]]*/\.agents/[[:space:]]*($|#)' "$REPO_ROOT/.gitignore"; then
    echo "ERROR: root .gitignore must contain an explicit '/.agents/' ignore rule." >&2
    errors=1
  fi
  reincludes="$(grep -nE '^[[:space:]]*!/?\.agents(/|$)' "$REPO_ROOT/.gitignore" || true)"
  if [[ -n "$reincludes" ]]; then
    echo "ERROR: root .gitignore re-includes repo-root .agents paths:" >&2
    echo "$reincludes" | sed 's/^/  /' >&2
    errors=1
  fi
fi

if [[ "$errors" -ne 0 ]]; then
  exit 1
fi

echo "no tracked repo-root .agents state"

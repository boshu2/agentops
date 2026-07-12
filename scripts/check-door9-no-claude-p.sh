#!/usr/bin/env bash
# check-door9-no-claude-p.sh — guard executable code against Claude print execution.
#
# Door 9 / LAW 0: AgentOps must not route workers through Claude print mode.
# Scan tracked executable files wherever they live, plus production Go source.
# Git's executable bit keeps prose, fixtures, and other source mentions out.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

shell_command_boundary='(^[[:space:]]*|;[[:space:]]+|&&[[:space:]]+|\|\|[[:space:]]+|\|[[:space:]]+)'
patterns=(
  'DefaultRuntimeCommand[[:space:]]*=[[:space:]]*"claude"'
  'StringVar\(&[^,]+,[[:space:]]*"runtime-cmd",[[:space:]]*"claude"'
  'runtimeCmd[[:space:]]*:=[[:space:]]*"claude"'
  'exec\.Command(Context)?\([^)]*"claude"[^)]*"(-p|--print)"'
  'exec\.Command(Context)?\([^)]*"(-p|--print)"[^)]*"claude"'
  "${shell_command_boundary}claude[[:space:]]+(-p|--print)"
  "${shell_command_boundary}claude[^\"]*--permission-mode[[:space:]]+bypassPermissions"
)

files=()
while IFS= read -r -d '' entry; do
  metadata="${entry%%$'\t'*}"
  file="${entry#*$'\t'}"
  mode="${metadata%% *}"

  if [[ "$mode" == "100755" || ( "$file" == *.go && "$file" != *_test.go ) ]]; then
    files+=("$file")
  fi
done < <(cd "$REPO_ROOT" && git ls-files --stage -z)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "check-door9-no-claude-p: FAIL — no tracked executable or production Go files found" >&2
  exit 1
fi

hits=""
for pattern in "${patterns[@]}"; do
  match="$(cd "$REPO_ROOT" && grep -InE "$pattern" -- "${files[@]}" 2>/dev/null || true)"
  if [[ -n "$match" ]]; then
    hits+=$'\n'"# pattern: $pattern"$'\n'"$match"$'\n'
  fi
done

if [[ -n "$hits" ]]; then
  echo "check-door9-no-claude-p: FAIL — tracked executable code can route to Claude print mode" >&2
  printf '%s\n' "$hits" >&2
  echo "Use a headless codex runner or another non-Claude headless worker for RPI defaults; Claude print mode is forbidden." >&2
  exit 1
fi

echo "check-door9-no-claude-p: PASS — no Claude print defaults or direct tracked executable paths"

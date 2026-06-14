#!/usr/bin/env bash
# check-door9-no-claude-p.sh — guard phased RPI against Claude print execution.
#
# Door 9 / LAW 0: AgentOps must not route agent workers through `claude -p` or
# `claude --print`. This guard checks production RPI Go surfaces for the known
# breach class: a Claude default that is later combined with generic `-p`
# prompt execution, plus direct production invocations of Claude print mode.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

targets=(
  "cli/internal/rpi"
  "cli/cmd/ao"
)

patterns=(
  'DefaultRuntimeCommand[[:space:]]*=[[:space:]]*"claude"'
  'StringVar\(&[^,]+,[[:space:]]*"runtime-cmd",[[:space:]]*"claude"'
  'runtimeCmd[[:space:]]*:=[[:space:]]*"claude"'
  'exec\.Command(Context)?\([^)]*"claude"[^)]*"(-p|--print)"'
  'exec\.Command(Context)?\([^)]*"(-p|--print)"[^)]*"claude"'
  'claude[[:space:]]+(-p|--print)'
)

mapfile -t files < <(
  cd "$REPO_ROOT"
  git ls-files -- "${targets[@]}" \
    | grep -E '\.go$' \
    | grep -Ev '(^|/)[^/]+_test\.go$' \
    || true
)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "check-door9-no-claude-p: FAIL — no production Go files found under RPI targets" >&2
  exit 1
fi

hits=""
for pattern in "${patterns[@]}"; do
  match="$(cd "$REPO_ROOT" && grep -nE "$pattern" "${files[@]}" 2>/dev/null || true)"
  if [[ -n "$match" ]]; then
    hits+=$'\n'"# pattern: $pattern"$'\n'"$match"$'\n'
  fi
done

if [[ -n "$hits" ]]; then
  echo "check-door9-no-claude-p: FAIL — phased RPI production code can route to Claude print mode" >&2
  printf '%s\n' "$hits" >&2
  echo "Use codex exec or another non-Claude headless worker for RPI defaults; Claude print mode is forbidden." >&2
  exit 1
fi

echo "check-door9-no-claude-p: PASS — no Claude print defaults or direct production exec paths in phased RPI"

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

failures=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

require_contains() {
  local file="$1"
  local needle="$2"
  local message="$3"
  if ! grep -Fq -- "$needle" "$file"; then
    fail "$message
  missing: $needle
  file: $file"
  fi
}

require_not_contains() {
  local file="$1"
  local needle="$2"
  local message="$3"
  if grep -Fq -- "$needle" "$file"; then
    fail "$message
  unexpected: $needle
  file: $file"
  fi
}

echo "=== Codex lifecycle guard validation ==="

entry_files=(
  "skills-codex/brainstorm/SKILL.md"
  "skills-codex/discovery/SKILL.md"
  "skills-codex/research/SKILL.md"
  "skills-codex/implement/SKILL.md"
  "skills-codex/status/SKILL.md"
  "skills-codex/recover/SKILL.md"
  "skills-codex/crank/SKILL.md"
  "skills-codex/rpi/SKILL.md"
  "skills-codex/brainstorm/prompt.md"
  "skills-codex/discovery/prompt.md"
  "skills-codex/research/prompt.md"
  "skills-codex/implement/prompt.md"
  "skills-codex/status/prompt.md"
  "skills-codex/recover/prompt.md"
  "skills-codex/crank/prompt.md"
  "skills-codex/rpi/prompt.md"
)

closeout_files=(
  "skills-codex/post-mortem/SKILL.md"
  "skills-codex/handoff/SKILL.md"
  "skills-codex/post-mortem/prompt.md"
  "skills-codex/handoff/prompt.md"
)

for file in "${entry_files[@]}"; do
  require_contains "$file" 'ao codex ensure-start' "entry skill must use ao codex ensure-start"
  require_not_contains "$file" 'ao codex start 2>/dev/null || true' "entry skill must not hand-roll ao codex start guards"
  require_not_contains "$file" '.agents/ao/codex/state.json' "entry skill must not parse Codex lifecycle state directly"
done

for file in "${closeout_files[@]}"; do
  require_contains "$file" 'ao codex ensure-stop' "closeout skill must use ao codex ensure-stop"
  require_not_contains "$file" 'ao codex stop --auto-extract' "closeout skill must not call ao codex stop directly"
done

require_contains "skills-codex/quickstart/SKILL.md" 'ao codex ensure-start' "quickstart should describe ensure-start for Codex entry skills"
require_contains "skills-codex/quickstart/SKILL.md" 'ao codex ensure-stop' "quickstart should describe ensure-stop for Codex closeout skills"
require_contains "skills-codex/using-agentops/SKILL.md" 'ao codex ensure-start' "using-agentops should document ensure-start"
require_contains "skills-codex/using-agentops/SKILL.md" 'ao codex ensure-stop' "using-agentops should document ensure-stop"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-start' "Codex override catalog should reference ensure-start"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-stop' "Codex override catalog should reference ensure-stop"

if [[ $failures -ne 0 ]]; then
  echo "Codex lifecycle guard validation failed with $failures issue(s)." >&2
  exit 1
fi

echo "Codex lifecycle guard validation passed."

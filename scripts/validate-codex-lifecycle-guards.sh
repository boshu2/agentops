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

require_regex() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if ! grep -Eq -- "$pattern" "$file"; then
    fail "$message
  missing pattern: $pattern
  file: $file"
  fi
}

require_not_regex() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if grep -Eq -- "$pattern" "$file"; then
    fail "$message
  unexpected pattern: $pattern
  file: $file"
  fi
}

echo "=== Codex lifecycle guard validation ==="

entry_files=(
  "skills-codex/discovery/SKILL.md"
  "skills-codex/research/SKILL.md"
  "skills-codex/implement/SKILL.md"
  "skills-codex/status/SKILL.md"
  "skills-codex/recover/SKILL.md"
  "skills-codex/crank/SKILL.md"
  "skills-codex/rpi/SKILL.md"
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

tracker_guidance_files=(
  "skills-codex/status/SKILL.md"
  "skills-codex/recover/SKILL.md"
  "skills-codex/implement/SKILL.md"
  "skills-codex/crank/SKILL.md"
  "skills-codex/post-mortem/SKILL.md"
  "skills-codex/handoff/SKILL.md"
  "skills-codex/rpi/prompt.md"
  "skills-codex-overrides/catalog.json"
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

# quickstart folded into status, using-agentops into inject (ag-s43tg, 2026-06-12);
# the ensure-start/stop lifecycle assertions live on status (the Codex entry/closeout doc).
require_contains "skills-codex/status/SKILL.md" 'ao codex ensure-start' "status (absorbs quickstart) should describe ensure-start for Codex entry skills"
require_contains "skills-codex/status/SKILL.md" 'ao codex ensure-stop' "status (absorbs quickstart) should describe ensure-stop for Codex closeout skills"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-start' "Codex override catalog should reference ensure-start"
require_contains "skills-codex-overrides/catalog.json" 'ao codex ensure-stop' "Codex override catalog should reference ensure-stop"

for file in "${tracker_guidance_files[@]}"; do
  require_regex "$file" '\bbr\b' "Codex tracker guidance should point at br/beads_rust"
  require_not_regex "$file" '(^|[^[:alnum:]_-])`?bd([`[:space:]]|$)|BD_' "Codex tracker guidance must not default to legacy bd/Dolt"
done

if [[ $failures -ne 0 ]]; then
  echo "Codex lifecycle guard validation failed with $failures issue(s)." >&2
  exit 1
fi

echo "Codex lifecycle guard validation passed."

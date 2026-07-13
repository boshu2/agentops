#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
failures=0

require_contains() {
  local file="$1" needle="$2" message="$3"
  if ! grep -Fq -- "$needle" "$file"; then
    printf 'FAIL: %s\n  missing: %s\n  file: %s\n' "$message" "$needle" "$file" >&2
    failures=$((failures + 1))
  fi
}

require_absent() {
  local path="$1" message="$2"
  if [[ -e "$path" ]]; then
    printf 'FAIL: %s\n  unexpected: %s\n' "$message" "$path" >&2
    failures=$((failures + 1))
  fi
}

echo '=== Codex four-umbrella contract ==='

require_contains skills-codex/rpi/SKILL.md 'Validate -> Learn -> orchestrator' \
  'RPI must preserve the legal post-verdict order'
require_contains skills-codex/rpi/SKILL.md 'Learn is the only post-verdict handoff' \
  'RPI must route every verdict through Learn'
require_contains skills-codex/rpi/SKILL.md 'Only the orchestrator may invoke Premortem' \
  'RPI must keep Premortem control with the orchestrator'
require_contains skills-codex/rpi/SKILL.md '$crank .agents/rpi/execution-packet.json' \
  'RPI must preserve the no-beads execution-packet handoff'
require_contains skills-codex/rpi/references/phase-data-contracts.md 'discovery, crank, validate, learn' \
  'RPI must require four ordered receipts'
require_contains skills-codex/rpi/references/phase-data-contracts.md '`phase_receipts`' \
  'RPI must keep disk-backed receipt evidence'
require_contains skills-codex/rpi/references/agile-replan-loop.md 'material_change' \
  'RPI must carry material plan impact'
require_contains skills-codex/rpi/references/agile-replan-loop.md 'no_change' \
  'RPI must carry explicit no-change behavior'
require_contains skills-codex/rpi/references/agile-replan-loop.md 'terminal' \
  'RPI must carry terminal behavior'

require_contains skills-codex/crank/SKILL.md 'one Crank invocation ends at one accepted wave' \
  'Crank must stop at the one-wave boundary'
require_contains skills-codex/crank/SKILL.md 'does not invoke Discovery, Learn, or Premortem' \
  'Crank must not own between-wave transitions'
require_contains skills-codex/crank/references/wave-patterns.md 'not duplicate inline judges inside Crank' \
  'Crank must not duplicate the Validate judge'

require_contains skills-codex/learn/SKILL.md 'plan_impact' \
  'Learn must emit plan impact'
require_contains skills-codex/learn/SKILL.md 'does not invoke Premortem' \
  'Learn must not control Premortem'
require_contains skills-codex/premortem/SKILL.md 'explicit orchestrator request' \
  'Premortem must accept only an orchestrator-owned between-wave plan'
require_contains skills-codex/evolve/SKILL.md 'Validate -> Learn -> orchestrator' \
  'Evolve must preserve the four-umbrella cycle'

require_contains skills-codex/rpi/prompt.md 'Record phase receipts in `.agents/rpi/execution-packet.json` and each phase summary' \
  'RPI generated prompt must preserve receipt enforcement'
require_contains skills-codex/crank/prompt.md 'End after one wave.' \
  'Crank generated prompt must preserve the one-wave boundary'
require_contains skills-codex/evolve/prompt.md 'Drive the lead cycle in-session through the skills' \
  'Evolve generated prompt must preserve in-session skill orchestration'
require_contains skills-codex/premortem/prompt.md 'Between waves, accept only a changed plan' \
  'Premortem generated prompt must preserve the orchestrator boundary'

for skill in rpi crank evolve premortem; do
  require_absent "skills-codex-overrides/$skill" \
    "$skill is parity-only and must not retain a stale hand-maintained override"
done

for validator in \
  skills-codex/rpi/scripts/validate.sh \
  skills-codex/crank/scripts/validate.sh \
  skills-codex/learn/scripts/validate.sh; do
  if ! bash "$validator" >/dev/null; then
    printf 'FAIL: Codex validator rejected contract: %s\n' "$validator" >&2
    failures=$((failures + 1))
  fi
done

if [[ $failures -ne 0 ]]; then
  printf 'Codex four-umbrella contract: FAIL (%d)\n' "$failures" >&2
  exit 1
fi

echo 'Codex four-umbrella contract: PASS'

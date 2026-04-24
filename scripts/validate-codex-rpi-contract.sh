#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

failures=0

require_contains() {
  local file="$1"
  local needle="$2"
  local message="$3"
  if ! grep -Fq -- "$needle" "$file"; then
    echo "FAIL: $message" >&2
    echo "  missing: $needle" >&2
    echo "  file: $file" >&2
    failures=$((failures + 1))
  fi
}

require_not_contains() {
  local file="$1"
  local needle="$2"
  local message="$3"
  if grep -Fq -- "$needle" "$file"; then
    echo "FAIL: $message" >&2
    echo "  unexpected: $needle" >&2
    echo "  file: $file" >&2
    failures=$((failures + 1))
  fi
}

echo "=== Codex RPI contract validation ==="

require_contains "skills-codex/rpi/SKILL.md" '$crank .agents/rpi/execution-packet.json' \
  'rpi must define the no-beads implementation handoff through execution-packet.json'
require_contains "skills-codex/rpi/SKILL.md" '$validation --complexity=<level>' \
  'rpi must define standalone validation when no epic_id exists'
require_not_contains "skills-codex/rpi/SKILL.md" '$crank <objective-id>' \
  'rpi must not use an undefined objective-id handoff'

require_contains "skills-codex/crank/SKILL.md" 'Given `$crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:' \
  'crank must accept execution-packet.json as a first-class input'
require_contains "skills-codex/crank/SKILL.md" '**Execution-packet/file mode:**' \
  'crank must define execution-packet/file-backed behavior explicitly'

require_contains "skills-codex/rpi/references/phase-data-contracts.md" '$crank .agents/rpi/execution-packet.json' \
  'phase-data contracts must document the no-beads discovery-to-implementation handoff'
require_contains "skills-codex/rpi/references/phase-data-contracts.md" 'standalone `$validation`' \
  'phase-data contracts must document standalone validation when no epic exists'

require_contains "skills-codex/discovery/references/output-templates.md" 'this execution packet becomes the' \
  'discovery output template must explain file-backed handoff when no epic is created'

echo "=== Codex skill chaining defaults ==="

require_contains "skills-codex/rpi/SKILL.md" 'RPI delegates via `$discovery`, `$crank`, `$validation` as **separate skill invocations**' \
  'rpi must default to Codex skill chaining across phases'
require_contains "skills-codex/discovery/SKILL.md" 'Discovery delegates to `$brainstorm` (conditional), `$design` (conditional), `$research`, `$plan`, and `$pre-mortem` as **separate skill invocations**' \
  'discovery must default to Codex skill chaining across discovery sub-skills'
require_contains "skills-codex/validation/SKILL.md" 'Validation delegates to `$vibe`, `$post-mortem`, `$retro`, and `$forge`' \
  'validation must default to Codex skill chaining across validation sub-skills'
require_contains "skills-codex/rpi/prompt.md" 'do not hand RPI orchestration to wrapper commands' \
  'rpi Codex prompt must reject wrapper-command orchestration'

require_contains "skills-codex/evolve/SKILL.md" 'Treat `ao evolve` and `ao rpi` as terminal' \
  'evolve must classify ao evolve/ao rpi as terminal wrappers, not Codex defaults'
require_contains "skills-codex/evolve/prompt.md" 'Do not shell out to `ao evolve`, `ao rpi`, or any wrapper command for the lead cycle.' \
  'evolve prompt must prohibit wrapper-command lead cycles'
require_contains "skills-codex-overrides/evolve/prompt.md" 'Do not shell out to `ao evolve`, `ao rpi`, or any wrapper command for the lead cycle.' \
  'evolve override prompt must preserve wrapper-command prohibition'

require_contains "skills-codex/autodev/SKILL.md" 'In Codex, `$autodev` hands work to `$evolve` or `$rpi` as skill invocations.' \
  'autodev must hand off to Codex skills by default'
require_contains "skills-codex/autodev/prompt.md" 'Do not use `ao evolve` or `ao rpi` as the Codex' \
  'autodev prompt must reject ao evolve/ao rpi as Codex handoff'
require_contains "skills-codex/using-agentops/SKILL.md" 'Codex skill orchestration default is `$skill` chaining.' \
  'using-agentops must document $skill chaining as the Codex default'

require_not_contains "skills-codex/evolve/SKILL.md" 'through $rpi and ao evolve' \
  'evolve must not describe ao evolve as a peer default to $rpi'
require_not_contains "skills-codex/evolve/prompt.md" 'for `ao evolve`:' \
  'evolve prompt must not frame $evolve as only a frontend for ao evolve'
require_not_contains "skills-codex-overrides/evolve/prompt.md" 'for `ao evolve`:' \
  'evolve override prompt must not frame $evolve as only a frontend for ao evolve'
require_not_contains "skills-codex/autodev/SKILL.md" 'use `$evolve` or `ao evolve`' \
  'autodev routing must not offer ao evolve as the Codex default'
require_not_contains "skills-codex/autodev/SKILL.md" 'use `$rpi` or `ao rpi`' \
  'autodev routing must not offer ao rpi as the Codex default'
require_not_contains "skills-codex/autodev/prompt.md" 'to `$evolve`, `ao evolve`, `$rpi`, or `ao rpi`' \
  'autodev prompt must not offer wrapper commands as peer handoffs'

if [[ $failures -ne 0 ]]; then
  echo "Codex RPI contract validation failed with $failures issue(s)." >&2
  exit 1
fi

echo "Codex RPI contract validation passed."

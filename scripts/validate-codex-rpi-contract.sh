#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2016
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

failures=0

# ag-2vz5v: resolve skill paths through the dispositions ledger so folds/cuts
# auto-retarget. Identity fallback keeps hermetic test copies self-contained.
if [[ -f "$REPO_ROOT/scripts/lib/resolve-skill-path.sh" ]]; then
  # shellcheck source=lib/resolve-skill-path.sh
  source "$REPO_ROOT/scripts/lib/resolve-skill-path.sh"
else
  resolve_skill_path() { printf '%s\n' "$1"; }
fi

require_contains() {
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
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
  local file
  file="$(resolve_skill_path "$1")"
  [[ -n "$file" ]] || return 0 # cut slug: resolver warned; skip visibly
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
require_contains "skills-codex/rpi/SKILL.md" '$validate --complexity=<level>' \
  'rpi must define standalone validation when no epic_id exists'
require_not_contains "skills-codex/rpi/SKILL.md" '$crank <objective-id>' \
  'rpi must not use an undefined objective-id handoff'

require_contains "skills-codex/crank/SKILL.md" 'Given `$crank [epic-id | .agents/rpi/execution-packet.json | plan-file.md | "description"]`:' \
  'crank must accept execution-packet.json as a first-class input'
require_contains "skills-codex/crank/SKILL.md" '**Execution-packet/file mode:**' \
  'crank must define execution-packet/file-backed behavior explicitly'

require_contains "skills-codex/rpi/references/phase-data-contracts.md" '$crank .agents/rpi/execution-packet.json' \
  'phase-data contracts must document the no-beads discovery-to-implementation handoff'
require_contains "skills-codex/rpi/references/phase-data-contracts.md" 'standalone `$validate`' \
  'phase-data contracts must document standalone validation when no epic exists'

require_contains "skills-codex/discovery/references/output-templates.md" 'this execution packet becomes the' \
  'discovery output template must explain file-backed handoff when no epic is created'

echo "=== Codex skill chaining defaults ==="

require_contains "skills-codex/rpi/SKILL.md" 'RPI delegates via `$discovery`, `$crank`, `$validate` as **separate skill invocations**' \
  'rpi must default to Codex skill chaining across phases'
# brainstorm/design folded into discovery, vibe into validate (ag-s43tg, 2026-06-12):
# the chaining contract now names only surviving skills; absorbed modes are internal.
require_contains "skills-codex/discovery/SKILL.md" 'Discovery runs brainstorm and design as internal modes (absorbed, ag-s43tg) and delegates to `$research`, `$plan`, and `$pre-mortem` as **separate skill invocations**' \
  'discovery must default to Codex skill chaining across discovery sub-skills'
require_contains "skills-codex/validate/SKILL.md" 'vibe` → `--mode=post-impl`' \
  'validate must document the absorbed vibe quick mode'
require_contains "skills-codex/rpi/prompt.md" 'do not hand RPI orchestration to wrapper commands' \
  'rpi Codex prompt must reject wrapper-command orchestration'

require_contains "skills-codex/evolve/SKILL.md" 'Treat retired CLI wrappers as terminal' \
  'evolve must classify the retired ao evolve/ao rpi CLIs as terminal wrappers, not Codex defaults (ag-llni: ao evolve deleted)'
require_contains "skills-codex/evolve/prompt.md" 'Drive the lead cycle in-session through the skills; do not shell out to a CLI loop wrapper.' \
  'evolve prompt must prohibit wrapper-command lead cycles'
require_contains "skills-codex-overrides/evolve/prompt.md" 'Drive the lead cycle in-session through the skills; do not shell out to a CLI loop wrapper.' \
  'evolve override prompt must preserve wrapper-command prohibition'

# autodev retired --into evolve (2026-07-02): the contract-management surface
# and its skill-invocation handoff doctrine live in evolve's fold section now.
require_contains "skills-codex/evolve/SKILL.md" 'absorbed from $autodev' \
  'evolve twin must carry the absorbed autodev contract-management section'
require_contains "skills-codex/evolve/SKILL.md" 'In Codex, `$autodev` hands work to `$evolve` or `$rpi` as skill invocations.' \
  'absorbed autodev section must hand off to Codex skills by default'
# using-agentops folded into inject (ag-s43tg, 2026-06-12). The inject Codex twin
# was dropped when inject was demoted to the experimental tier
# (age-focus-membrane-bookkeeper-m1wg.19): the corpus-flywheel skills ship no
# Codex twin, so there is no skills-codex/inject/SKILL.md to assert the
# $skill-chaining default against. The doctrine lives in the source skill only.

require_not_contains "skills-codex/evolve/SKILL.md" 'through $rpi and ao evolve' \
  'evolve must not describe ao evolve as a peer default to $rpi'
require_not_contains "skills-codex/evolve/prompt.md" 'for `ao evolve`:' \
  'evolve prompt must not frame $evolve as only a frontend for ao evolve'
require_not_contains "skills-codex-overrides/evolve/prompt.md" 'for `ao evolve`:' \
  'evolve override prompt must not frame $evolve as only a frontend for ao evolve'
require_not_contains "skills-codex/evolve/SKILL.md" 'use `$evolve` or `ao evolve`' \
  'absorbed autodev routing must not offer ao evolve as the Codex default'
require_not_contains "skills-codex/evolve/SKILL.md" 'use `$rpi` or `ao rpi`' \
  'absorbed autodev routing must not offer ao rpi as the Codex default'
require_not_contains "skills-codex/evolve/prompt.md" 'to `$evolve`, `ao evolve`, `$rpi`, or `ao rpi`' \
  'evolve prompt must not offer wrapper commands as peer handoffs'

if [[ $failures -ne 0 ]]; then
  echo "Codex RPI contract validation failed with $failures issue(s)." >&2
  exit 1
fi

echo "Codex RPI contract validation passed."

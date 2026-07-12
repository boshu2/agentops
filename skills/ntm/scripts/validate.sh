#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Exact Markdown contract literals: backticks are data, never substitutions.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- Keep NTM opt-in because the operator chooses the orchestration substrate; never start, register, or probe it merely because it is available.'
  '## Software Factory Mesh'
  '**Operator-choice invariant:** NTM is an optional substrate selected explicitly by the operator; a cold `ao pawl review` and ordinary in-session work do not require an NTM session.'
  '**Pane-role contract:** Producer panes own disjoint worktrees and write scopes; tester panes run deterministic checks; fresh-context refuter panes judge without mutation; integrator panes act only after an `ao pawl review` `CONFIRMED` verdict. Never collapse producer and refuter for one candidate.'
  '**Agent Mail handoff:** Before two or more writers run, reserve disjoint paths. Handoff on one Agent Mail thread with bead, pane role, worktree, reserved paths, exact HEAD, evidence paths, and next action; require recipient acknowledgement and release reservations at completion.'
  '**Pawl authority:** NTM may host or tend warm reviewer panes, but `ao pawl review` owns independent verdict and admission. Do not inject keys into an in-flight pawl pane, and do not treat pane agreement as confirmation.'
  '**Failure routing:** A plain `REFUTED` verdict returns to the producer for automatic repair and revalidation. Only a tripped circuit breaker enters `HOLD` and receives exactly one bounded helper consultation before the candidate re-earns an independent verdict.'
  '## Output Specification'
  '- **Validation command:** run `skills/ntm/scripts/validate.sh` for this mesh contract; for a live action, verify `ntm --robot-snapshot` plus attention/mail/git/bead state.'
  '- **Downstream handoff:** send the verified state and evidence on the existing Agent Mail thread to the named next role; acknowledgement and released/transferred reservations are the completion marker.'
  '## Quality Checklist'
  '- Operator choice is explicit: no NTM session or pawl service is started merely because the substrate exists.'
  '- Pane roles remain separated, write scopes are disjoint, and every multi-writer handoff is acknowledged through Agent Mail.'
  '- Deterministic evidence and a fresh-context `ao pawl review` verdict—not producer or pane consensus—control admission.'
  '- Plain `REFUTED` work auto-repairs; only a tripped breaker gets one helper, and every lock or reservation is released or transferred.'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "ntm mesh validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

validate_contract "$SKILL"
delete_one_negative_fixture
echo "ntm operator-choice factory mesh: PASS"

#!/usr/bin/env bats

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  REPO="$(cd "$PACK/../.." && pwd)"
  FRAGMENT="$PACK/template-fragments/breaker-escalation.template.md"
}

@test "retry-limit breaker is single-sourced into every role" {
  [ -s "$FRAGMENT" ]

  for role in planner builder verifier agy-verifier opus-verifier breaker-helper; do
    run grep -F 'append_fragments = ["law0", "breaker-escalation", "sentinel"]' \
      "$PACK/agents/$role/agent.toml"
    [ "$status" -eq 0 ]

    run grep -E '3 redo rounds|round-limit.*human|human decides' \
      "$PACK/agents/$role/prompt.template.md"
    [ "$status" -ne 0 ]
  done
}

@test "retry-limit routes through one helper before human" {
  for required in \
    'CIRCUIT-BREAKER-TRIP -> HOLD' \
    'HOLD -> ONE-HELPER' \
    'fresh context or an available cross-family model' \
    'HELPER-UNSTUCK -> AUTO-REDO' \
    'HELPER-ESCALATE -> HUMAN' \
    'hard time, cost, or quota ceiling is actually spent'; do
    run grep -F "$required" "$FRAGMENT"
    [ "$status" -eq 0 ]
  done
}

@test "goal blocked-status accounting is not a work breaker" {
  run grep -F 'repeated-turn threshold governs status bookkeeping only' "$FRAGMENT"
  [ "$status" -eq 0 ]
  run grep -F 'never substitutes for this work-escalation path' "$FRAGMENT"
  [ "$status" -eq 0 ]
}

@test "RPI retry references route exhausted attempts through the helper" {
  for ref in \
    "$REPO/skills/rpi/references/gate-retry-logic.md" \
    "$REPO/skills-codex/rpi/references/gate-retry-logic.md"; do
    run grep -F 'Manual intervention needed' "$ref"
    [ "$status" -ne 0 ]
    run grep -F 'HOLD -> ONE-HELPER' "$ref"
    [ "$status" -eq 0 ]
    run grep -F 'HELPER-UNSTUCK -> AUTO-REDO' "$ref"
    [ "$status" -eq 0 ]
    run grep -F 'HELPER-ESCALATE -> HUMAN' "$ref"
    [ "$status" -eq 0 ]
  done
}

@test "pack exhaustion documentation enters HOLD before operator" {
  for file in \
    "$PACK/formulas/membrane-quest.toml" \
    "$PACK/README.md" \
    "$PACK/QUICKSTART.md"; do
    run grep -F 'ONE-HELPER' "$file"
    [ "$status" -eq 0 ]
  done

  run grep -F 'STAYS OPEN for a human' "$PACK/formulas/membrane-quest.toml"
  [ "$status" -ne 0 ]
  run grep -F 'exhausted attempts leave the bead open by design' "$PACK/QUICKSTART.md"
  [ "$status" -ne 0 ]
}

@test "Gas City executable path reserves and routes the helper recovery attempt" {
  run grep -F 'max_attempts = 6' "$PACK/formulas/membrane-quest.toml"
  [ "$status" -eq 0 ]
  run grep -F 'session new "$HELPER_TARGET"' "$PACK/membrane/close-gate.sh"
  [ "$status" -eq 0 ]
  run grep -F 'session reset "$HELPER_TARGET"' "$PACK/membrane/close-gate.sh"
  [ "$status" -ne 0 ]
  run grep -F 'CIRCUIT-BREAKER-TRIP -> HOLD -> ONE-HELPER' "$PACK/membrane/close-gate.sh"
  [ "$status" -eq 0 ]
  run grep -F 'HELPER-UNSTUCK -> AUTO-REDO' "$PACK/membrane/close-gate.sh"
  [ "$status" -eq 0 ]
  run grep -F 'HELPER-ESCALATE -> HUMAN' "$PACK/membrane/close-gate.sh"
  [ "$status" -eq 0 ]
  [ -s "$PACK/agents/breaker-helper/agent.toml" ]
  [ -s "$PACK/agents/breaker-helper/prompt.template.md" ]
}

@test "entrypoint skills and narrative consumers preserve the helper rung" {
  for file in \
    "$REPO/skills/validate/SKILL.md" \
    "$REPO/skills-codex/validate/SKILL.md" \
    "$REPO/skills-codex-overrides/crank/references/failure-recovery.md" \
    "$REPO/images/gemini/skills/discovery/SKILL.md" \
    "$REPO/docs/brownian-ratchet.md"; do
    run grep -Ei 'fresh-context|fresh context|cross-family|helper' "$file"
    [ "$status" -eq 0 ]
    run grep -Ei 'HELPER-UNSTUCK|UNSTUCK|resume on' "$file"
    [ "$status" -eq 0 ]
    run grep -Ei 'HELPER-ESCALATE|ESCALATE.*human|human.*ESCALATE' "$file"
    [ "$status" -eq 0 ]
  done

  run grep -F 'After 3 failures, escalate:' \
    "$REPO/skills-codex-overrides/crank/references/failure-recovery.md"
  [ "$status" -ne 0 ]
  run grep -F 'failed 3x, manual intervention needed' \
    "$REPO/images/gemini/skills/discovery/SKILL.md"
  [ "$status" -ne 0 ]
  run grep -F 'After 3 failures, mark as BLOCKER and mail the human' \
    "$REPO/docs/brownian-ratchet.md"
  [ "$status" -ne 0 ]
}

#!/usr/bin/env bats
# Plan-pawl duel e2e dogfood (age-plan-pawl-9yib.7).
#
# Runs the REAL `ao plan-pawl decide` (not a stub) end-to-end over duel-verdict
# fixtures that stand in for the two distinct-family judge panes a discovery
# STEP 3.5 duel produces over a sample FANOUT plan. The assertion inventory mirrors
# the bead's acceptance:
#   - quorum gate clears ONLY on no-FAIL (>= 2 distinct families) -> PASS / exit 0
#   - a seeded FAIL triggers auto-redo                            -> REDO / exit 3
#   - round > max-rounds                                          -> BLOCKED / exit 4
#   - a mechanical WARN is auto-applied (+ re-judge)              -> REDO / exit 3
#   - a judgment WARN is surfaced, not blocking                   -> PASS / exit 0
# Plus the fail-closed floor (single family, off-roster, malformed) the duel
# inherits from `ao plan-pawl decide`.

setup_file() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  export REPO_ROOT
  AO_BIN="$(mktemp -d)/ao"
  export AO_BIN
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao )
}

setup() {
  DUEL="$(mktemp -d)"
  # A sample FANOUT plan the judges duel over (the artifact under review).
  cat >"$DUEL/synthesis-packet.yaml" <<'YAML'
kind: SynthesisPacket
selected_plan: merged
synthesis_rationale: ["architecture fork — fanout class, plan-pawl gated"]
YAML
}

teardown() { rm -rf "$DUEL"; }

judge() { # family disposition [warn_class]
  local f="$1" d="$2" wc="${3:-}"
  if [[ -n "$wc" ]]; then
    printf '{"family":"%s","disposition":"%s","warn_class":"%s"}\n' "$f" "$d" "$wc" >"$DUEL/$f.json"
  else
    printf '{"family":"%s","disposition":"%s"}\n' "$f" "$d" >"$DUEL/$f.json"
  fi
}

@test "quorum clears on no-FAIL: claude+gpt PASS -> PASS (exit 0)" {
  judge claude PASS; judge gpt PASS
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3
  [ "$status" -eq 0 ]
  [[ "$output" == *"decision: PASS"* ]]
}

@test "seeded FAIL triggers auto-redo -> REDO (exit 3)" {
  judge claude PASS; judge gpt FAIL
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3
  [ "$status" -eq 3 ]
  [[ "$output" == *"decision: REDO"* ]]
}

@test "round > max-rounds -> BLOCKED (exit 4, max-attempts breaker)" {
  judge claude PASS; judge gpt FAIL
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 4 --max-rounds 3
  [ "$status" -eq 4 ]
  [[ "$output" == *"breaker: max-attempts"* ]]
}

@test "mechanical WARN is auto-applied -> REDO (exit 3)" {
  judge claude PASS; judge gpt WARN mechanical
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3 --json
  [ "$status" -eq 3 ]
  [[ "$output" == *'"auto_applied"'* ]]
}

@test "judgment WARN is surfaced, not blocking -> PASS (exit 0)" {
  judge claude PASS; judge gpt WARN judgment
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3 --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"surfaced_warns"'* ]]
}

@test "fail-closed: single family does not meet quorum -> REDO (exit 3)" {
  judge claude PASS
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3
  [ "$status" -eq 3 ]
  [[ "$output" == *"quorum"* ]]
}

@test "fail-closed: an off-roster pane cannot pad quorum -> REDO (exit 3)" {
  judge claude PASS; judge gpt PASS; judge llama PASS
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3
  [ "$status" -eq 3 ]
  [[ "$output" == *"decision: REDO"* ]]
}

@test "fail-closed: a malformed judge verdict (no disposition) -> REDO (exit 3)" {
  # A missing/unknown disposition must be counted as a FAIL (fail-closed), never
  # silently treated as a clean PASS — so the duel REDOs, it does not pass.
  judge claude PASS
  printf '{"family":"gpt"}\n' >"$DUEL/gpt.json"
  run "$AO_BIN" plan-pawl decide --dir "$DUEL" --round 1 --max-rounds 3
  [ "$status" -eq 3 ]
  [[ "$output" == *"decision: REDO"* ]]
}

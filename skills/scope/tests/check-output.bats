#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  CHECK="$SKILL_DIR/scripts/check-output.sh"
  FIX="$(mktemp -d)"
}

teardown() { rm -rf "$FIX"; }

@test "complete review maps every bounded pattern to one axiom" {
  cat >"$FIX/complete.yaml" <<'YAML'
decision: COMPLETE
write_scope:
  include: ["skills/widget/**"]
  exclude: ["skills/widget/generated/**"]
generated_companions: ["skills-codex/widget/**"]
axioms:
  - fact: "Widget behavior is owned by its canonical package."
    patterns: ["include:skills/widget/**"]
  - fact: "Generated files have a separate owner and projection."
    patterns: ["exclude:skills/widget/generated/**", "generated:skills-codex/widget/**"]
gaps: []
ambiguities: []
checked: ["acceptance criterion A", "projection owner"]
not_checked: ["runtime-derived final diff"]
YAML
  run "$CHECK" "$FIX/complete.yaml"
  [ "$status" -eq 0 ]
  [[ "$output" == *'3 mapped patterns'* ]]
}

@test "old shape presence baseline accepts traversal that the contract rejects" {
  cat >"$FIX/traversal.yaml" <<'YAML'
decision: COMPLETE
write_scope:
  include: ["../outside/**"]
  exclude: []
generated_companions: []
axioms:
  - fact: "Claimed source location."
    patterns: ["include:../outside/**"]
gaps: []
ambiguities: []
checked: ["caller statement"]
not_checked: []
YAML
  run grep -q '^write_scope:' "$FIX/traversal.yaml"
  [ "$status" -eq 0 ]
  run "$CHECK" "$FIX/traversal.yaml"
  [ "$status" -ne 0 ]
}

@test "unmapped and contradictory exact patterns fail the done condition" {
  cat >"$FIX/unmapped.yaml" <<'YAML'
decision: COMPLETE
write_scope:
  include: ["skills/a/**"]
  exclude: ["skills/a/**"]
generated_companions: []
axioms:
  - fact: "A owns behavior."
    patterns: ["include:skills/a/**"]
gaps: []
ambiguities: []
checked: ["owner"]
not_checked: []
YAML
  run "$CHECK" "$FIX/unmapped.yaml"
  [ "$status" -ne 0 ]
}

@test "incomplete intent is representable only as NEEDS_INPUT with named gaps" {
  cat >"$FIX/needs.yaml" <<'YAML'
decision: NEEDS_INPUT
write_scope:
  include: ["skills/a/**"]
  exclude: []
generated_companions: []
axioms:
  - fact: "A is the tentative owner."
    patterns: ["include:skills/a/**"]
gaps: ["Generated projection owner is unknown."]
ambiguities: []
checked: ["tentative owner"]
not_checked: ["generated companion"]
YAML
  run "$CHECK" "$FIX/needs.yaml"
  [ "$status" -eq 0 ]
  sed -i.bak 's/NEEDS_INPUT/COMPLETE/' "$FIX/needs.yaml"
  run "$CHECK" "$FIX/needs.yaml"
  [ "$status" -ne 0 ]
}

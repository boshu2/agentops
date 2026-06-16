#!/usr/bin/env bats
# Regression coverage for scripts/check-skill-isolation.sh (ag-skill-isolation-ci-gate-jxpbx).
# The guard must be portable on macOS/BSD awk and Linux/gawk, and its self-test
# must exercise the Skill(...) matching path that previously used GNU awk arrays.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-skill-isolation.sh"
  ROOT="$(mktemp -d)"
}

teardown() {
  rm -rf "$ROOT"
}

@test "runs without awk errors on a clean fixture" {
  mkdir -p "$ROOT/crank"
  cat > "$ROOT/crank/SKILL.md" <<'EOF'
---
name: crank
---
# /crank

Plain implementation instructions.
EOF

  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 0 ]
  [[ "$output" != *"awk:"* ]]
  [[ "$output" == *"PASS"* ]]
}

@test "--self-test passes and reports no awk dialect error" {
  run bash "$SCRIPT" --self-test
  [ "$status" -eq 0 ]
  [[ "$output" == *"self-test PASS"* ]]
  [[ "$output" != *"awk:"* ]]
}

@test "catches a sealed phase Skill call compression pattern" {
  mkdir -p "$ROOT/crank"
  cat > "$ROOT/crank/SKILL.md" <<'EOF'
---
name: crank
---
# /crank

Skill(skill="research", args="inline the discovery pass")
EOF

  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"phase-skill calling another phase skill (target=research)"* ]]
  [[ "$output" != *"awk:"* ]]
}

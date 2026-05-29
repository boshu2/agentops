#!/usr/bin/env bats
# Regression for ag-cw2y item 1: heal --strict must flag a user-invocable skill
# that has no row in docs/contracts/skill-dispositions.yaml. This is the exact
# gate that silently passed when /burndown (ag-3yl8 #600) was added, costing a
# CI round. The check is fixture-driven via the HEAL_REPO_ROOT override.

setup() {
  HEAL="$BATS_TEST_DIRNAME/../../skills/heal-skill/scripts/heal.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/skills/foo" "$FIX/docs/contracts"
  cat > "$FIX/skills/foo/SKILL.md" <<'EOF'
---
name: foo
description: A fixture skill for the dispositions coverage check.
skill_api_version: 1
---
# foo
EOF
  cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'EOF'
dispositions:
  - skill:          bar
    domain:         "BC1 Corpus"
    hexagonal_role: domain
    disposition:    keep
    rationale:      "unrelated existing row"
EOF
}

teardown() { rm -rf "$FIX"; }

@test "heal flags a user-invocable skill missing from skill-dispositions.yaml" {
  run env HEAL_REPO_ROOT="$FIX" bash "$HEAL" --check skills/foo
  [[ "$output" == *"MISSING_DISPOSITION"* ]]
  [[ "$output" == *"foo"* ]]
}

@test "heal does NOT flag a skill that has a dispositions row" {
  cat >> "$FIX/docs/contracts/skill-dispositions.yaml" <<'EOF'
  - skill:          foo
    domain:         "BC1 Corpus"
    hexagonal_role: domain
    disposition:    keep
    rationale:      "now covered"
EOF
  run env HEAL_REPO_ROOT="$FIX" bash "$HEAL" --check skills/foo
  [[ "$output" != *"MISSING_DISPOSITION"* ]]
}

@test "heal --strict exits non-zero and names the missing disposition" {
  run env HEAL_REPO_ROOT="$FIX" bash "$HEAL" --check --strict skills/foo
  [ "$status" -eq 1 ]
  [[ "$output" == *"MISSING_DISPOSITION"* ]]
}

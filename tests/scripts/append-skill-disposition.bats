#!/usr/bin/env bats
# ag-cw2y item-1 scaffold-half: skill-builder must append a skill-dispositions.yaml
# row for a new skill so it is one-shot-green against heal.sh Check 12. The helper
# is idempotent and repo-root-injectable (so init.sh can call it and tests can
# fixture it).

setup() {
  HELPER="$BATS_TEST_DIRNAME/../../scripts/append-skill-disposition.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/docs/contracts"
  cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'EOF'
dispositions:
  - skill:          existing
    domain:         "BC1 Corpus"
    hexagonal_role: domain
    disposition:    keep
    rationale:      "already here"
EOF
}

teardown() { rm -rf "$FIX"; }

@test "appends a dispositions row for a new skill" {
  run bash "$HELPER" newskill "$FIX"
  [ "$status" -eq 0 ]
  grep -qE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+newskill[[:space:]]*$" "$FIX/docs/contracts/skill-dispositions.yaml"
}

@test "appended row carries a valid BC domain and a TODO rationale" {
  bash "$HELPER" newskill "$FIX"
  # The row's domain must be a real bounded context (so check-bounded-contexts-drift passes)
  run grep -A4 'skill:          newskill' "$FIX/docs/contracts/skill-dispositions.yaml"
  [[ "$output" == *"BC4 Factory"* ]]
  [[ "$output" == *"TODO"* ]]
}

@test "is idempotent — running twice does not duplicate the row" {
  bash "$HELPER" newskill "$FIX"
  bash "$HELPER" newskill "$FIX"
  count=$(grep -cE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+newskill[[:space:]]*$" "$FIX/docs/contracts/skill-dispositions.yaml")
  [ "$count" -eq 1 ]
}

@test "does not touch an already-present skill" {
  run bash "$HELPER" existing "$FIX"
  [ "$status" -eq 0 ]
  count=$(grep -cE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+existing[[:space:]]*$" "$FIX/docs/contracts/skill-dispositions.yaml")
  [ "$count" -eq 1 ]
}

@test "is newline-safe — appending to a file missing its trailing newline does not fuse rows" {
  # Strip the trailing newline so the ledger ends mid-line (the corruption
  # trigger: a bare `cat >>` used to fuse the new row onto the rationale line).
  printf '%s' "$(cat "$FIX/docs/contracts/skill-dispositions.yaml")" > "$FIX/docs/contracts/skill-dispositions.yaml"
  [ -n "$(tail -c1 "$FIX/docs/contracts/skill-dispositions.yaml")" ]  # precondition: no trailing \n

  run bash "$HELPER" newskill "$FIX"
  [ "$status" -eq 0 ]
  # The prior last line must survive intact, un-fused with the new row.
  grep -qE '^[[:space:]]*rationale:[[:space:]]+"already here"$' "$FIX/docs/contracts/skill-dispositions.yaml"
  # And the new row must start on its own line.
  grep -qE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+newskill[[:space:]]*$" "$FIX/docs/contracts/skill-dispositions.yaml"
}

#!/usr/bin/env bats
# pawl-route-emit-catch.bats — age-2yh2 seam lock. The ROUTED multi-family REFUTED
# path in pawl.sh cmd_route records a structured membrane catch carrying the refuting
# reviewer's REAL finding: `ao membrane catch --evidence <refuting lane's capture>`
# (the two-tier REFUTED-reason salvage lives Go-side, age-ulab), replacing the generic
# "standing-pawl route: …" pseudo-class as the only record. THIS guards the bash seam:
# refuting-lane evidence selection (_refuting_evidence: first refuter, canonical
# cc->cod->agy order; NO refuter -> no file -> no catch), the exact flags the emit
# passes (--bead/--evidence/--head/--mode/--scope head), the PAWL_CATCH_CLASS/
# PAWL_CATCH_DETECTOR passthrough, and the fail-safe non-blocking contract (a broken
# or absent ao never blocks the REFUTED exit). Mirrors pawl-review-emit-catch.bats,
# which locks the equivalent seam on the cold pawl-review path.

setup() {
  REPO_ROOT_REAL="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT_REAL/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  # Recording ao stub: one arg per line so assertions are order-exact.
  cat > "$TMP/ao" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/recorded-args"
exit 0
EOF
  chmod +x "$TMP/ao"
  # shellcheck source=/dev/null
  source "$SCRIPT"   # source-guard returns before dispatch
}

teardown() { rm -rf "$TMP"; }

# --- _refuting_evidence: which lane's capture carries the reviewer's finding ---

@test "age-2yh2: _refuting_evidence picks the FIRST refuter in canonical order (cc before cod/agy)" {
  run _refuting_evidence REFUTED REFUTED CONFIRMED e-cc e-cod e-agy
  [ "$status" -eq 0 ]
  [ "$output" = "e-cc" ]
}

@test "age-2yh2: a lone codex refuter selects the codex lane evidence" {
  run _refuting_evidence CONFIRMED REFUTED "" e-cc e-cod e-agy
  [ "$status" -eq 0 ]
  [ "$output" = "e-cod" ]
}

@test "age-2yh2: an agy-only refute (others n/a or timed out) selects the agy lane evidence" {
  run _refuting_evidence n/a "" REFUTED e-cc e-cod e-agy
  [ "$status" -eq 0 ]
  [ "$output" = "e-agy" ]
}

@test "age-2yh2: NO substantive refuter (timeouts/insufficient) -> empty (no finding to salvage)" {
  run _refuting_evidence "" "" n/a e-cc e-cod e-agy
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- _route_emit_catch: the ao membrane catch seam ---

@test "age-2yh2: _route_emit_catch delegates extraction to ao membrane catch --evidence" {
  AO_BIN="$TMP/ao"
  evidence="$TMP/ev.txt"; printf 'REFUTED: the loop drops the last element\nPAWL r1f2 REFUTED\n' > "$evidence"
  _route_emit_catch age-seam abcdef0123 multi-model "$evidence"
  [ -f "$TMP/recorded-args" ]
  mapfile -t args < "$TMP/recorded-args"
  [ "${args[0]}" = "membrane" ]
  [ "${args[1]}" = "catch" ]
  expected="membrane catch --bead age-seam --evidence $evidence --head abcdef0123 --mode multi-model --scope head"
  [ "${args[*]}" = "$expected" ]
}

@test "age-2yh2: PAWL_CATCH_CLASS/DETECTOR pass through as --class/--detector-pattern" {
  AO_BIN="$TMP/ao"
  evidence="$TMP/ev.txt"; printf 'REFUTED: off-by-one\n' > "$evidence"
  PAWL_CATCH_CLASS="stale-retired-surface" PAWL_CATCH_DETECTOR='foo[0-9]+' \
    _route_emit_catch age-seam abcdef0123 multi-model "$evidence"
  mapfile -t args < "$TMP/recorded-args"
  joined="${args[*]}"
  [[ "$joined" == *"--class stale-retired-surface"* ]]
  [[ "$joined" == *"--detector-pattern foo[0-9]+"* ]]
}

@test "age-2yh2: empty/missing evidence file -> silent no-op (nothing recorded, returns 0)" {
  AO_BIN="$TMP/ao"
  run _route_emit_catch age-seam abcdef0123 multi-model ""
  [ "$status" -eq 0 ]
  run _route_emit_catch age-seam abcdef0123 multi-model "$TMP/missing.txt"
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/recorded-args" ]
}

@test "age-2yh2: fail-safe — a failing ao never blocks (non-blocking contract)" {
  printf '#!/usr/bin/env bash\nexit 1\n' > "$TMP/ao"; chmod +x "$TMP/ao"
  AO_BIN="$TMP/ao"
  evidence="$TMP/ev.txt"; printf 'REFUTED: x\n' > "$evidence"
  run _route_emit_catch age-seam abcdef0123 multi-model "$evidence"
  [ "$status" -eq 0 ]
}

@test "age-2yh2: no ao resolvable -> silent no-op (returns 0, nothing recorded)" {
  AO_BIN="$TMP/does-not-exist"
  # Point the repo-build fallbacks away from the real checkout AND empty PATH of any ao.
  ROOT="$TMP"
  evidence="$TMP/ev.txt"; printf 'REFUTED: x\n' > "$evidence"
  PATH="$TMP" run _route_emit_catch age-seam abcdef0123 multi-model "$evidence"
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/recorded-args" ]
}

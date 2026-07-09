#!/usr/bin/env bats
# pawl-review-emit-catch.bats — age-ulab seam lock. emit_pawl_catch collapsed to a
# single `ao membrane catch --evidence` call: the two-tier REFUTED reason salvage,
# domain-from-top-dir, and paths-from-git all moved Go-side (unit-tested in
# cli/cmd/ao/membrane_test.go). THIS guards the bash seam that remains: the exact
# flags the collapsed function passes (--bead/--evidence/--head/--mode/--scope),
# the PAWL_CATCH_CLASS/PAWL_CATCH_DETECTOR passthrough, and the fail-safe
# non-blocking contract (a broken ao never blocks the REFUTED exit).

setup() {
  REPO_ROOT_REAL="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT_REAL/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  # Recording ao stub: one arg per line so assertions are order-exact.
  cat > "$TMP/ao" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/recorded-args"
exit 0
EOF
  chmod +x "$TMP/ao"
  # shellcheck source=/dev/null
  source "$SCRIPT" 2>/dev/null || true
}

teardown() { rm -rf "$TMP"; }

@test "age-ulab: emit_pawl_catch delegates extraction to ao membrane catch --evidence" {
  AO_BIN="$TMP/ao"
  bead="age-seam"; head="abcdef0123"; scope="staged"
  evidence="$TMP/ev.txt"; printf 'VERDICT: REFUTED — x\n' > "$evidence"
  emit_pawl_catch multi-model
  [ -f "$TMP/recorded-args" ]
  mapfile -t args < "$TMP/recorded-args"
  [ "${args[0]}" = "membrane" ]
  [ "${args[1]}" = "catch" ]
  expected="membrane catch --bead age-seam --evidence $evidence --head abcdef0123 --mode multi-model --scope staged"
  [ "${args[*]}" = "$expected" ]
}

@test "age-ulab: PAWL_CATCH_CLASS/DETECTOR pass through as --class/--detector-pattern" {
  AO_BIN="$TMP/ao"
  bead="age-seam"; head="abcdef0123"; scope="head"
  evidence="$TMP/ev.txt"; printf 'PAWL r123 REFUTED\n' > "$evidence"
  PAWL_CATCH_CLASS="stale-retired-surface" PAWL_CATCH_DETECTOR='foo[0-9]+' emit_pawl_catch fresh-context
  mapfile -t args < "$TMP/recorded-args"
  joined="${args[*]}"
  [[ "$joined" == *"--class stale-retired-surface"* ]]
  [[ "$joined" == *"--detector-pattern foo[0-9]+"* ]]
}

@test "age-ulab: fail-safe — a failing ao never blocks (non-blocking contract)" {
  printf '#!/usr/bin/env bash\nexit 1\n' > "$TMP/ao"; chmod +x "$TMP/ao"
  AO_BIN="$TMP/ao"
  bead="age-seam"; head="abcdef0123"; scope="head"
  evidence="$TMP/missing-evidence.txt" # unreadable evidence must not matter either
  run emit_pawl_catch fresh-context
  [ "$status" -eq 0 ]
}

@test "age-ulab: no ao resolvable -> silent no-op (returns 0, nothing recorded)" {
  AO_BIN="$TMP/does-not-exist"
  # Point resolve_ao's repo-build fallbacks away from the real checkout AND empty
  # PATH of any installed ao.
  REPO_ROOT="$TMP"
  bead="age-seam"; head="abcdef0123"; scope="head"; evidence="$TMP/ev.txt"
  PATH="$TMP" run emit_pawl_catch fresh-context
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/recorded-args" ]
}

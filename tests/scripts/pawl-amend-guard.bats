#!/usr/bin/env bats
# pawl-amend-guard.bats — amend-into-#trivial-bind trap detector (ebec.11).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/lib/pawl-amend-guard.sh"
  R="$BATS_TEST_TMPDIR/r"
  mkdir -p "$R/docs/provenance"
  git init -q "$R"; git -C "$R" config user.email t@t.local; git -C "$R" config user.name t
  echo x > "$R/code.go"; echo '{"g":0}' > "$R/docs/provenance/ledger.jsonl"
  git -C "$R" add -A; git -C "$R" commit -qm "feat(x): real work (age-x)"
  unset PAWL_NO_AMEND_GUARD
}

@test "provenance-only #trivial bind -> safe (0)" {
  echo '{"g":1}' >> "$R/docs/provenance/ledger.jsonl"
  git -C "$R" add -A; git -C "$R" commit -qm "chore(provenance): bind pawl CONFIRMED verdict for age-x #trivial"
  run pawl_amend_guard "$R" HEAD
  [ "$status" -eq 0 ]
}

@test "amend trap: #trivial commit carrying code -> refuse (2) with recovery" {
  # simulate the trap: a #trivial-subject commit that also changes code
  echo y >> "$R/code.go"; echo '{"g":1}' >> "$R/docs/provenance/ledger.jsonl"
  git -C "$R" add -A; git -C "$R" commit -qm "chore(provenance): bind pawl CONFIRMED verdict for age-x #trivial"
  run pawl_amend_guard "$R" HEAD
  [ "$status" -eq 2 ]
  [[ "$output" == *"amend-into-#trivial-bind trap"* ]]
  [[ "$output" == *"code.go"* ]]
  [[ "$output" == *"rebuild the bead's code as ONE feat"* ]]
}

@test "plain feat (not #trivial) -> safe (0)" {
  echo y >> "$R/code.go"; git -C "$R" add -A; git -C "$R" commit -qm "fix(x): more work (age-x)"
  run pawl_amend_guard "$R" HEAD
  [ "$status" -eq 0 ]
}

@test "mid-subject #trivial mention (not a trailing marker) -> safe (0)" {
  echo y >> "$R/code.go"; git -C "$R" add -A
  git -C "$R" commit -qm "fix(pawl): stop #trivial from bypassing the gate (age-x)"
  run pawl_amend_guard "$R" HEAD
  [ "$status" -eq 0 ]
}

@test "opt-out PAWL_NO_AMEND_GUARD=1 -> safe (0) even on the trap" {
  echo y >> "$R/code.go"; git -C "$R" add -A
  git -C "$R" commit -qm "chore(provenance): bind pawl CONFIRMED verdict for age-x #trivial"
  PAWL_NO_AMEND_GUARD=1 run pawl_amend_guard "$R" HEAD
  [ "$status" -eq 0 ]
}

#!/usr/bin/env bats
# §G Meta-gates from the cross-family review — B91 (coverage manifest) and
# B92 (hermetic real-repo verification). Codex gaps 1 + 2.

setup() {
  load helpers2
}

@test "B91: every appended behavior maps to a mechanical verifier in a checked-in coverage manifest" {
  # the manifest is checked in and covers EVERY appended behavior id
  [ -s "$COVERAGE_MANIFEST" ]
  for n in $(seq 74 93); do
    grep -Eq "^B$n[[:space:]]" "$COVERAGE_MANIFEST"
  done

  # a manifest checker script enforces it mechanically
  [ -x "$COVERAGE_CHECKER" ]
  run bash "$COVERAGE_CHECKER"
  [ "$status" -eq 0 ]

  # dropping a behavior's mapping is caught by id
  m="$BATS_TEST_TMPDIR/cm.txt"
  grep -Ev '^B92[[:space:]]' "$COVERAGE_MANIFEST" > "$m"
  run bash "$COVERAGE_CHECKER" --manifest "$m"
  [ "$status" -ne 0 ]
  [[ "$output" == *"B92"* ]]

  # a mapped script that does not exist is caught by name
  cp "$COVERAGE_MANIFEST" "$m"
  printf 'B99 script:scripts/zz-does-not-exist.sh\n' >> "$m"
  run bash "$COVERAGE_CHECKER" --manifest "$m"
  [ "$status" -ne 0 ]
  [[ "$output" == *"zz-does-not-exist"* ]]

  # a mapped bats test the suite entry point does not select is caught
  cp "$COVERAGE_MANIFEST" "$m"
  printf 'B98 bats:99-zz-not-in-suite.bats#B98\n' >> "$m"
  run bash "$COVERAGE_CHECKER" --manifest "$m"
  [ "$status" -ne 0 ]
  [[ "$output" == *"B98"* || "$output" == *"99-zz-not-in-suite"* ]]
}

@test "B92: real-repo verification is hermetic — no check dirties or damages the operator checkout" {
  # the hermetic wrapper is checked in; mutating cutover verifiers exist with
  # their scratch-target flag seams (so the checkout is never the target)
  [ -x "$REAL_REPO_ROOT/$V_HERMETIC" ]

  clone="$(real_repo_clone)"

  # a clean command passes through with its exit status and zero residue claims
  run bash -c "cd '$clone' && $V_HERMETIC true"
  [ "$status" -eq 0 ]
  run bash -c "cd '$clone' && $V_HERMETIC false"
  [ "$status" -ne 0 ]
  [[ "$output" != *"verifier residue"* ]]

  # leftover paths are rejected: nonzero with "verifier residue" naming them
  run bash -c "cd '$clone' && $V_HERMETIC bash -c 'echo dirt > zz-residue.txt'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"verifier residue"* ]]
  [[ "$output" == *"zz-residue.txt"* ]]
  rm -f "$clone/zz-residue.txt"

  # a HEAD SHA change is rejected the same way
  run bash -c "cd '$clone' && $V_HERMETIC git commit --allow-empty -qm residue-probe"
  [ "$status" -ne 0 ]
  [[ "$output" == *"verifier residue"* ]]

  # the wrapper records pre/post itself (status + HEAD), so a verifier that
  # crashes mid-run still cannot hide residue behind a forgotten restore step
  grep -Eq 'status --porcelain' "$REAL_REPO_ROOT/$V_HERMETIC"
  grep -Eq 'rev-parse|HEAD' "$REAL_REPO_ROOT/$V_HERMETIC"

  # and the operator checkout itself stayed byte-identical through THIS test:
  # every mutation above targeted the disposable clone
  [ -z "$(git -C "$REAL_REPO_ROOT" status --porcelain -- zz-residue.txt)" ]
}

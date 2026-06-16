#!/usr/bin/env bats
# intake.sh — the front-door fuse of the minimal operating model. It enforces
# (mechanically, not by goodwill) the two things the 3-hour basement run lacked:
# a defined "done" and a bounded descent; and it classifies blast-radius so the
# operator knows whether to stay SOLO (chaos) or summon the cross-family Navi
# (pawl). These cases are the acceptance evidence: each fuse trips, each class
# routes. Runs in an isolated temp cwd so the recorded intake doesn't touch the
# repo.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/intake.sh"
  WORK="$(mktemp -d)"
  cd "$WORK"
}

teardown() {
  rm -rf "$WORK"
}

@test "fuse 1: no done-test stops you (exit 1)" {
  run "$SCRIPT" --intent "fix the read path"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no done-test"* ]]
}

@test "fuse 2: >2 prerequisite layers stops you (the basement alarm, exit 1)" {
  run "$SCRIPT" --intent "fix the read path" --done-test "am inbox works" --depth 3
  [ "$status" -eq 1 ]
  [[ "$output" == *"prerequisite layers deep"* ]]
}

@test "chaos: a reversible edit routes SOLO (exit 0, CHAOS)" {
  run "$SCRIPT" --intent "edit a doc to fix a typo" --done-test "renders"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CHAOS"* ]]
  [[ "$output" == *"Solo"* ]]
}

@test "pawl: pushing to shared trunk routes to the Navi (exit 0, PAWL)" {
  run "$SCRIPT" --intent "merge the fix to shared trunk" --done-test "ci green"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
  [[ "$output" == *"Navi"* ]]
}

@test "pawl: a credential rotation routes to the Navi (exit 0, PAWL)" {
  run "$SCRIPT" --intent "rotate the deploy credential" --done-test "old key revoked"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "pawl: changing gate/enforcement logic routes to the Navi (exit 0, PAWL)" {
  run "$SCRIPT" --intent "generalize the pawls.md enforcement rule" \
      --done-test "bats green" --surfaces "docs/contracts/pawls.md gate logic"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "fuse 1: a whitespace-only done-test still stops you (exit 1)" {
  run "$SCRIPT" --intent "fix the read path" --done-test "   "
  [ "$status" -eq 1 ]
  [[ "$output" == *"no done-test"* ]]
}

# Navi-found omissions: the doc names these classes as pawls; the classifier must route them PAWL.
@test "omission: open a github issue routes PAWL (external/forge)" {
  run "$SCRIPT" --intent "open a github issue for the bug" --done-test "issue url"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "omission: write to prod database routes PAWL (shared state)" {
  run "$SCRIPT" --intent "write to prod database" --done-test "row present"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "omission: disable an alert routes PAWL" {
  run "$SCRIPT" --intent "disable alert for the noisy check" --done-test "alert off"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "omission: regenerate a registry routes PAWL" {
  run "$SCRIPT" --intent "regenerate registry from source" --done-test "registry rebuilt"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "omission: grant a role routes PAWL (authz)" {
  run "$SCRIPT" --intent "grant role to the new agent" --done-test "role applied"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

# Navi-found false positives: substrings must NOT trip the classifier.
@test "guard: 'investigate' does not false-trip on the substring 'gate' (CHAOS)" {
  run "$SCRIPT" --intent "investigate a bug in the parser" --done-test "root cause found"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CHAOS"* ]]
}

@test "guard: 'accessibility' does not false-trip on the substring 'access' (CHAOS)" {
  run "$SCRIPT" --intent "local accessibility copy fix" --done-test "renders"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CHAOS"* ]]
}

@test "guard: colloquial 'issue' (a bug) is not a forge issue (CHAOS)" {
  run "$SCRIPT" --intent "fix the parser issue" --done-test "parser passes"
  [ "$status" -eq 0 ]
  [[ "$output" == *"CHAOS"* ]]
}

@test "guard: multi-space phrasing still matches (disable   alert -> PAWL)" {
  run "$SCRIPT" --intent "disable   alert for the noisy check" --done-test "alert off"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

# Documented pawls.md examples the triage must catch (word-order / external / contract).
@test "pawl: git push --force (word order) routes PAWL" {
  run "$SCRIPT" --intent "git push --force origin main" --done-test "remote updated"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "pawl: posting/sending externally routes PAWL" {
  run "$SCRIPT" --intent "post the release note to slack" --done-test "posted"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "pawl: repoint a canary routes PAWL (contract change)" {
  run "$SCRIPT" --intent "repoint the canary to the new contract" --done-test "canary green"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL"* ]]
}

@test "two intakes in the same second each leave a distinct record" {
  "$SCRIPT" --intent "one" --done-test "done" >/dev/null
  "$SCRIPT" --intent "two" --done-test "done" >/dev/null
  n="$(ls .agents/rpi/intake/*.json | wc -l | tr -d ' ')"
  [ "$n" -eq 2 ]
}

@test "a recorded intake JSON is written and carries the class" {
  run "$SCRIPT" --intent "edit a doc" --done-test "renders"
  [ "$status" -eq 0 ]
  rec="$(ls .agents/rpi/intake/*.json)"
  [ -f "$rec" ]
  grep -q '"class": "CHAOS"' "$rec"
  grep -q '"done_test": "renders"' "$rec"
}

@test "usage: --intent is required (exit 2)" {
  run "$SCRIPT" --done-test "x"
  [ "$status" -eq 2 ]
}

@test "usage: bad --depth is a usage error (exit 2)" {
  run "$SCRIPT" --intent "x" --done-test "y" --depth notanumber
  [ "$status" -eq 2 ]
}

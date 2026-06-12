#!/usr/bin/env bats
# §A Harness repair — B74–B76 (judge amendment 1 + minor 5c).
# Executable definitions of done for the frozen scenarios in ../behaviors.md.

setup() {
  load helpers2
  sandbox_setup
}

teardown() {
  sandbox_teardown
}

@test "B74: fixture gate.d directory survives every lane clone" {
  # the directory is tracked, not incidental (a .gitkeep or equivalent)
  tracked="$(git -C "$SEED" ls-files -- scripts/gate.d/)"
  [ -n "$tracked" ]

  # a fresh lane clone carries the directory
  lane="$(new_lane b74)"
  [ -d "$lane/scripts/gate.d" ]

  # the redirect-crash class is structurally gone: a shell redirect into the
  # directory succeeds (exit 0, file present)
  run bash -c "echo 'exit 0' > '$lane/scripts/gate.d/99-probe.sh'"
  [ "$status" -eq 0 ]
  [ -f "$lane/scripts/gate.d/99-probe.sh" ]

  # the harness carries a self-check that fails the run if the tracked entry
  # ever disappears
  grep -q "fixture defect: scripts/gate.d untracked" \
    "$BASE_SUITE_DIR/run-acceptance.sh" "$BASE_SUITE_DIR/helpers.bash"
}

@test "B75: full suite is red ON ASSERTION — audit-red.sh is checked in, manifest-wired, and passes" {
  # the red-on-assertion audit is itself a checked-in script
  [ -f "$AUDIT_RED" ]
  [ -x "$AUDIT_RED" ]

  # the enumerated-count floor is asserted against the B91 coverage manifest,
  # never hardcoded to 73
  grep -q "coverage-manifest" "$AUDIT_RED"
  ! grep -Eq '(-eq|==) *73\b' "$AUDIT_RED"

  # running the audit passes: zero ok, not-ok == enumerated count, zero
  # harness crashes, every failure trace pointing at a test-body assertion
  # line (incl. the ten previously-poisoned tests), no byte-identical
  # if/else branches anywhere in the suite
  run bash "$AUDIT_RED"
  [ "$status" -eq 0 ]
  [[ "$output" != *"No such file or directory"* ]]
}

@test "B76: B57 dead conditional repaired — post-push reruns assert 'already landed' distinctly" {
  export LAND_STALE_TTL=2

  # crash at the post-push phases: the rerun is a distinct already-landed
  # no-op that adds ZERO new patch-ids
  for phase in push pre-release; do
    lane="$(new_lane "pp-$phase" "feat-pp-$phase")"
    add_skill "$lane" "zz-b76-$phase"
    LAND_TEST_CRASH_AFTER="$phase" start_land "C$phase" "$lane"
    wait_land "C$phase"
    sleep 3   # let the lock go stale
    land "$lane" --abort || true
    before_ids="$(remote_patch_ids | sort)"
    run land "$lane"
    [ "$status" -eq 0 ]
    [[ "$output" == *"already landed"* ]]
    after_ids="$(remote_patch_ids | sort)"
    [ "$before_ids" = "$after_ids" ]
  done

  # crash at the pre-push phases: the rerun completes a REAL land exactly
  # once (no "already landed" required)
  for phase in rebase regen-write regen-commit gate; do
    lane="$(new_lane "pr-$phase" "feat-pr-$phase")"
    add_skill "$lane" "zz-b76-$phase"
    LAND_TEST_CRASH_AFTER="$phase" start_land "D$phase" "$lane"
    wait_land "D$phase"
    sleep 3
    land "$lane" --abort || true
    run land "$lane"
    [ "$status" -eq 0 ]
    c="$(fresh_clone)"
    [ -d "$c/skills/zz-b76-$phase" ]
    [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]
  done

  # structurally: no if/else with byte-identical branches anywhere in the
  # base suite (the B57 dead conditional is extinct)
  run find_identical_if_else "$BASE_SUITE_DIR"/*.bats
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

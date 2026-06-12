#!/usr/bin/env bats
# §F Substrate seam, lock-dir pin, bead hygiene — B88–B90 (amendments 4 + 5).

setup() {
  load helpers2
  sandbox_setup
}

teardown() {
  sandbox_teardown
}

@test "B88: the acceptance contract is implementation-agnostic through ONE LAND_BIN seam, including installed hooks" {
  lane="$(new_lane b88 feat-b88)"

  # the seam is load-bearing: exporting LAND_BIN to a probe stub flips the
  # observed SUT output to the stub's
  stub="$SANDBOX/probe-land"
  printf '#!/bin/sh\necho "PROBE-LAND-STUB: $*"\nexit 0\n' > "$stub"
  chmod +x "$stub"
  LAND_BIN="$stub" run land "$lane" --status
  [ "$status" -eq 0 ]
  [[ "$output" == *"PROBE-LAND-STUB"* ]]

  # no direct SUT invocation outside helpers.bash and fixture-authoring
  # heredocs, across BOTH suites
  run find_direct_sut_invocations "$BASE_SUITE_DIR"/*.bats "$RUN2_TESTS_DIR"/*.bats
  [ "$status" -eq 0 ]
  [ -z "$output" ]

  # the seam extends to INSTALLED artifacts: with LAND_BIN set to a probe,
  # the installed chain consults the probe, not a hardcoded path
  make_chained_hook "$lane"
  run land "$lane" --install
  [ "$status" -eq 0 ]
  dispatch="$SANDBOX/dispatch-probe"
  printf '#!/bin/sh\necho "DISPATCH-PROBE consulted" >> "%s"\nexit 0\n' \
    "$SANDBOX/dispatch.log" > "$dispatch"
  chmod +x "$dispatch"
  ( cd "$lane" \
    && git commit --allow-empty -qm b88-probe \
    && LAND_BIN="$dispatch" git push origin HEAD:refs/heads/main ) || true
  [ -f "$SANDBOX/dispatch.log" ]
  grep -q 'DISPATCH-PROBE consulted' "$SANDBOX/dispatch.log"

  # spec.md carries the implementation-choice note: an "ao land" substrate is
  # permitted and preferred over hardening concurrency-critical Bash 3.2, and
  # either substrate must pass the identical suite via the LAND_BIN seam
  grep -q 'LAND_BIN' "$RUN1_SPEC"
  grep -Eiq 'ao land' "$RUN1_SPEC"
}

@test "B89: LAND_LOCK_DIR's production default is pinned, deterministic, and origin-IDENTITY-keyed" {
  export XDG_STATE_HOME="$SANDBOX/state"
  unset LAND_LOCK_DIR

  # two clones of the SAME origin (one with a differing URL spelling) resolve
  # the IDENTICAL documented default lock_dir
  l1="$(new_lane b89a feat-b89a)"
  l2="$(new_lane b89b feat-b89b)"
  git -C "$l2" remote set-url origin "file://$REMOTE"
  run status_json "$l1"
  [ "$status" -eq 0 ]
  d1="$(printf '%s' "$output" | jq -r '.lock_dir')"
  [ -n "$d1" ] && [ "$d1" != "null" ]
  run status_json "$l2"
  [ "$status" -eq 0 ]
  d2="$(printf '%s' "$output" | jq -r '.lock_dir')"
  [ "$d1" = "$d2" ]
  case "$d1" in "$SANDBOX/state/land/"*) : ;; *) false ;; esac

  # resolution is pure: --status created no lock files (B34)
  [ ! -e "$d1/lock.json" ]

  # equivalent github spellings digest to the SAME lock_dir
  dgh=""
  for url in "git@github.com:org/repo.git" "ssh://git@github.com/org/repo" \
             "https://github.com/org/repo.git" "https://github.com/org/repo"; do
    git -C "$l1" remote set-url origin "$url"
    run status_json "$l1"
    [ "$status" -eq 0 ]
    d="$(printf '%s' "$output" | jq -r '.lock_dir')"
    [ -z "$dgh" ] && dgh="$d"
    [ "$d" = "$dgh" ]
  done

  # a DIFFERENT origin identity resolves a DIFFERENT lock_dir
  git -C "$l1" remote set-url origin "git@github.com:org/other.git"
  run status_json "$l1"
  [ "$status" -eq 0 ]
  dother="$(printf '%s' "$output" | jq -r '.lock_dir')"
  [ "$dother" != "$dgh" ]
  [ "$dgh" != "$d1" ]

  # the exact pinned formula + canonicalization rule are printed in --help
  git -C "$l1" remote set-url origin "$REMOTE"
  run land "$l1" --help
  printf '%s' "$output" | grep -Eq 'XDG_STATE_HOME|\.local/state'
  printf '%s' "$output" | grep -Eiq 'canonical'

  # mutual exclusion actually flows through the default: two concurrent lands
  # from same-identity clones with DIFFERING origin spellings serialize (B6)
  add_skill "$l1" zz-b89-a
  add_skill "$l2" zz-b89-b
  start_land A "$l1"
  start_land B "$l2"
  wait_land A
  wait_land B
  [ "$ST_A" -eq 0 ]
  [ "$ST_B" -eq 0 ]
  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b89-a" ]
  [ -d "$c/skills/zz-b89-b" ]
  [ -f "$d1/audit.jsonl" ]
  run hold_overlap_check "$d1/audit.jsonl"
  [ "$status" -eq 0 ]
}

@test "B90: every bead's regression criteria are self-contained runnable commands; the sweep EXECUTES them and fails closed" {
  # the sweep is a checked-in script
  [ -x "$REAL_REPO_ROOT/$V_BEAD_SWEEP" ]

  # FAIL CLOSED: a missing ledger path exits nonzero naming it — no silent skip
  run env BR_BEADS_DIR=/nonexistent-beads-ledger bash -c \
    "cd '$REAL_REPO_ROOT' && $V_BEAD_SWEEP"
  [ "$status" -ne 0 ]
  [[ "$output" == *"nonexistent-beads-ledger"* ]]

  # FAIL CLOSED: br absent from PATH exits nonzero naming the missing tool
  run env PATH=/usr/bin:/bin bash -c "cd '$REAL_REPO_ROOT' && $V_BEAD_SWEEP"
  [ "$status" -ne 0 ]
  [[ "$output" == *"br"* ]]

  # FAIL CLOSED: an unknown bead id exits nonzero naming the id
  run bash -c "cd '$REAL_REPO_ROOT' && $V_BEAD_SWEEP ag-zz-does-not-exist"
  [ "$status" -ne 0 ]
  [[ "$output" == *"ag-zz-does-not-exist"* ]]

  # the live sweep over all landing-redesign beads: every ACCEPTANCE /
  # regression criterion is a full runnable command (path + filter + env),
  # zero shorthand-only criteria, every extracted command EXECUTES from the
  # repo root and selects ≥1 test; the only passing outcomes per command are
  # red-on-assertion or green
  run bash -c "cd '$REAL_REPO_ROOT' && $V_BEAD_SWEEP"
  [ "$status" -eq 0 ]
}

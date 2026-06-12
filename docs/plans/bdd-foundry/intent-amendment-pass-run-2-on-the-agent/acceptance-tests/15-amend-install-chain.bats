#!/usr/bin/env bats
# §D Cutover: hook-chain-aware --install — B82–B85 — §G crash-safety B93, and
# §H rollout evidence B94 (the drift-guard split of B85's second concern).
# Sandbox clones reproduce the real repo's pre-push shape (beads marker block
# + pre-push.local cockpit gate), instrumented via $PROBE_LOG.

setup() {
  load helpers2
  sandbox_setup
}

teardown() {
  sandbox_teardown
}

@test "B82: --install CHAINS onto a beads-managed pre-push hook, never clobbers, in documented order" {
  lane="$(new_lane b82 feat-b82)"
  make_chained_hook "$lane"
  hook="$lane/.git/hooks/pre-push"
  beads_pre="$(extract_segment "$hook" "$BEADS_BEGIN_RE" "$BEADS_END_RE" | sha256_stdin)"
  local_pre="$(sha256_file "$lane/.git/hooks/pre-push.local")"

  run land "$lane" --install
  [ "$status" -eq 0 ]

  # the documented order is printed by --help
  run land "$lane" --help
  printf '%s' "$output" | grep -Eiq 'beads.*(pre-push\.local|cockpit).*guard'

  # a direct push executes ALL THREE segments in the documented order and is
  # rejected by the guard with B17 semantics preserved through the chain
  : > "$PROBE_LOG"
  add_skill "$lane" zz-b82
  run bash -c "cd '$lane' && git push origin HEAD:refs/heads/main 2>&1"
  [ "$status" -ne 0 ]
  [[ "$output" == *"use scripts/land.sh"* ]]
  seq="$(tr '\n' ' ' < "$PROBE_LOG")"
  [[ "$seq" == "beads-segment cockpit-gate"* ]]

  # never clobbers: beads segment + pre-push.local byte-identical; hook executable
  [ "$(extract_segment "$hook" "$BEADS_BEGIN_RE" "$BEADS_END_RE" | sha256_stdin)" = "$beads_pre" ]
  [ "$(sha256_file "$lane/.git/hooks/pre-push.local")" = "$local_pre" ]
  [ -x "$hook" ]

  # when land.sh itself pushes (live lock + nonce per B63), the full chain
  # still runs and permits the push
  : > "$PROBE_LOG"
  run land "$lane"
  [ "$status" -eq 0 ]
  grep -q 'beads-segment' "$PROBE_LOG"
  grep -q 'cockpit-gate' "$PROBE_LOG"
  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b82" ]
}

@test "B83: --install is idempotent and upgrades ONLY its own guard segment" {
  lane="$(new_lane b83 feat-b83)"
  make_chained_hook "$lane"
  hook="$lane/.git/hooks/pre-push"
  run land "$lane" --install
  [ "$status" -eq 0 ]

  # idempotent rerun: exit 0, "already installed", byte-identical hook
  pre="$(sha256_file "$hook")"
  run land "$lane" --install
  [ "$status" -eq 0 ]
  [[ "$output" == *"already installed"* ]]
  [ "$(sha256_file "$hook")" = "$pre" ]

  # downgrade the guard's version string in place between its own markers
  beads_pre="$(extract_segment "$hook" "$BEADS_BEGIN_RE" "$BEADS_END_RE" | sha256_stdin)"
  local_pre="$(sha256_file "$lane/.git/hooks/pre-push.local")"
  inject_guard_defect "$lane" stale
  guard_stale="$(extract_segment "$hook" "$GUARD_BEGIN_RE" "$GUARD_END_RE" | sha256_stdin)"

  # rerun upgrades ONLY the guard segment, logs the upgrade, exits 0
  run land "$lane" --install
  [ "$status" -eq 0 ]
  [[ "$output" == *"guard upgraded"* ]]
  [ "$(extract_segment "$hook" "$BEADS_BEGIN_RE" "$BEADS_END_RE" | sha256_stdin)" = "$beads_pre" ]
  [ "$(sha256_file "$lane/.git/hooks/pre-push.local")" = "$local_pre" ]
  [ "$(extract_segment "$hook" "$GUARD_BEGIN_RE" "$GUARD_END_RE" | sha256_stdin)" != "$guard_stale" ]
  ! grep -q 'LAND GUARD v0\.0\.1' "$hook"
}

@test "B84: --install policy PINNED — install on hookless, refuse on unrecognized foreign hooks" {
  # hookless clone: a guard hook is installed; B17 rejection + B63 land hold
  lane="$(new_lane b84 feat-b84)"
  rm -f "$lane/.git/hooks/pre-push"
  run land "$lane" --install
  [ "$status" -eq 0 ]
  add_skill "$lane" zz-b84
  run bash -c "cd '$lane' && git push origin HEAD:refs/heads/main 2>&1"
  [ "$status" -ne 0 ]
  [[ "$output" == *"use scripts/land.sh"* ]]
  run land "$lane"
  [ "$status" -eq 0 ]

  # the policy is decided at freeze: printed in --help and stated in spec.md
  run land "$lane" --help
  [[ "$output" == *"not recognized"* ]]
  grep -Eq 'not recognized|chain manually' "$RUN1_SPEC"

  # foreign hooks REFUSE across every control-flow trap variant, with zero
  # byte changes and no guard text anywhere in .git/hooks/
  for variant in exit0 exectrap noshebang noexec; do
    fl="$(new_lane "b84-$variant")"
    write_foreign_hook "$fl" "$variant"
    pre="$(sha256_file "$fl/.git/hooks/pre-push")"
    run land "$fl" --install
    [ "$status" -ne 0 ]
    [[ "$output" == *"not recognized"* ]]
    [[ "$output" == *"chain manually"* ]]
    [ "$(sha256_file "$fl/.git/hooks/pre-push")" = "$pre" ]
    ! grep -rq 'LAND GUARD' "$fl/.git/hooks/"
  done

  # recognized chains are unaffected by the refusal policy
  cl="$(new_lane b84-chain)"
  make_chained_hook "$cl"
  run land "$cl" --install
  [ "$status" -eq 0 ]
}

# B85/B94 — the drift-guard split (bead 'done' is 1:1 with a runnable test):
#   B85 — run2-install-verify's done-criterion; strict verify ONLY; green at
#         that bead's own close (no dependence on the checked-in rollout
#         evidence records)
#   B94 — run2-rollout-evidence's terminal criterion (split of B85's second
#         concern at drift-guard repair); asserts the CHECKED-IN per-clone
#         records and EXECUTES scripts/check-rollout-evidence.sh in-test
@test "B85: strict verify — pinned JSON keys, five DISTINCT named defects, naked-clone diagnosis, residual stated, checker ships bead-scoped" {
  lane="$(new_lane b85 feat-b85)"
  make_chained_hook "$lane"
  run land "$lane" --install
  [ "$status" -eq 0 ]

  # verify is machine-readable with the pinned JSON keys and stable output
  run land "$lane" --install --verify
  [ "$status" -eq 0 ]
  printf '%s' "$output" | jq -e '.guard_present == true' > /dev/null
  printf '%s' "$output" | jq -e '.guard_version | type == "string"' > /dev/null
  printf '%s' "$output" | jq -e 'has("chain")' > /dev/null
  printf '%s' "$output" | jq -e 'has("defects")' > /dev/null
  v1="$(land "$lane" --install --verify)"
  v2="$(land "$lane" --install --verify)"
  [ "$v1" = "$v2" ]

  # keys + exit codes documented in --help; pinned residual statement carried
  # by --help AND the spec: the lock is HOST-LOCAL, cross-host serialization
  # is out of scope and owned by the ag-arpk disposition
  run land "$lane" --help
  for k in guard_present guard_version chain defects; do
    [[ "$output" == *"$k"* ]]
  done
  [[ "$output" == *"host-local"* ]]
  [[ "$output" == *"ag-arpk"* ]]
  grep -qi 'host-local' "$RUN1_SPEC"
  grep -q 'ag-arpk' "$RUN1_SPEC"

  # one injected fault per case: each named by a DISTINCT defect token in the
  # defects output ("a distinct named defect for EACH" — not merely nonempty)
  toks="$BATS_TEST_TMPDIR/defect-tokens"
  : > "$toks"
  for defect in stale duplicate unpaired noexec order; do
    dl="$(new_lane "b85-$defect")"
    make_chained_hook "$dl"
    run land "$dl" --install
    [ "$status" -eq 0 ]
    inject_guard_defect "$dl" "$defect"
    run land "$dl" --install --verify
    [ "$status" -ne 0 ]
    printf '%s' "$output" | jq -e '(.defects | length) >= 1' > /dev/null
    printf '%s\n' "$output" | jq -re '.defects[0]' >> "$toks"
  done
  [ "$(sort -u "$toks" | wc -l | tr -d ' ')" -eq 5 ]

  # a naked clone exits nonzero naming exactly what is missing
  nl="$(new_lane b85-naked)"
  rm -f "$nl/.git/hooks/pre-push"
  run land "$nl" --install --verify
  [ "$status" -ne 0 ]
  printf '%s' "$output" | jq -e '.guard_present == false' > /dev/null

  # the rollout-evidence checker SHIPS WITH THIS BEAD: executable, and rejects
  # a stale SYNTHETIC manifest — no dependence on the checked-in evidence
  # records (those are run2-rollout-evidence's deliverable)
  [ -x "$REAL_REPO_ROOT/$V_ROLLOUT" ]
  e="$BATS_TEST_TMPDIR/ev.jsonl"
  for h in mac bushido; do
    printf '{"host":"%s","repo_sha":"0000000000000000000000000000000000000000","guard_version":"v0.0.0","command":"scripts/land.sh --install --verify","timestamp":"2026-06-12T00:00:00Z","verify":{}}\n' "$h"
  done > "$e"
  run bash -c "cd '$REAL_REPO_ROOT' && $V_ROLLOUT --manifest '$e'"
  [ "$status" -ne 0 ]
}

@test "B94: rollout evidence — CHECKED-IN per-clone records, fresh, checker-validated on the real repo" {
  # rollout evidence: a CHECKED-IN manifest with one record per live clone
  # (Mac + bushido), full record shape, and a staleness-rejecting checker.
  # Terminal criterion of run2-rollout-evidence (B94, split from B85). The
  # former compound acceptance's `scripts/check-rollout-evidence.sh` half is
  # folded in here (the $V_ROLLOUT executions below), so the bead's
  # ACCEPTANCE is one bats filter, not a chained shell command.
  [ -s "$ROLLOUT_EVIDENCE" ]
  jq -es 'length >= 2' "$ROLLOUT_EVIDENCE" > /dev/null
  jq -es 'all(.[]; has("host") and has("repo_sha") and has("guard_version")
              and has("command") and has("timestamp") and has("verify"))' \
    "$ROLLOUT_EVIDENCE" > /dev/null
  [ -x "$REAL_REPO_ROOT/$V_ROLLOUT" ]
  run bash -c "cd '$REAL_REPO_ROOT' && $V_ROLLOUT"
  [ "$status" -eq 0 ]
  e="$BATS_TEST_TMPDIR/ev.jsonl"
  jq -c '.repo_sha = "0000000000000000000000000000000000000000"' "$ROLLOUT_EVIDENCE" > "$e"
  run bash -c "cd '$REAL_REPO_ROOT' && $V_ROLLOUT --manifest '$e'"
  [ "$status" -ne 0 ]
}

@test "B93: --install is crash-safe — atomic write, backup, byte-identical hook on failure" {
  lane="$(new_lane b93 feat-b93)"
  make_chained_hook "$lane"
  hook="$lane/.git/hooks/pre-push"
  pre="$(sha256_file "$hook")"

  # each injected failure separately: kill mid-write, rename failure, chmod
  # failure — nonzero with a structured error naming the failed step; the
  # surviving hook is byte-identical (temp-file + atomic rename, never in-place)
  for step in write rename chmod; do
    LAND_TEST_INSTALL_FAIL="$step" run land "$lane" --install
    [ "$status" -ne 0 ]
    [[ "$output" == *"$step"* ]]
    [ "$(sha256_file "$hook")" = "$pre" ]
  done

  # full/read-only target stand-in: refused, hook intact, no temp wreckage
  chmod a-w "$lane/.git/hooks"
  run land "$lane" --install
  [ "$status" -ne 0 ]
  chmod u+w "$lane/.git/hooks"
  [ "$(sha256_file "$hook")" = "$pre" ]
  [ -z "$(find "$lane/.git/hooks" -name 'pre-push*.tmp*' 2>/dev/null)" ]

  # successful install: executable hook + backup of the prior hook at the
  # documented path, documented in --help
  run land "$lane" --install
  [ "$status" -eq 0 ]
  [ -x "$hook" ]
  backup="$lane/.git/hooks/pre-push.pre-land-install.bak"
  [ -f "$backup" ]
  [ "$(sha256_file "$backup")" = "$pre" ]
  run land "$lane" --help
  [[ "$output" == *"pre-land-install.bak"* ]]

  # B83's idempotent rerun does not delete the backup
  run land "$lane" --install
  [ "$status" -eq 0 ]
  [ -f "$backup" ]
}

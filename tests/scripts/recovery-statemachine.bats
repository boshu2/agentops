#!/usr/bin/env bats
# recovery-statemachine.bats — acceptance for the M2 Codex/local recovery state
# machine (age-d16-self-hosting-route-nkr.3). These are FAILURE-INJECTION tests:
# each branch (fix-forward | re-scope-as-new-acceptance | andon) is driven by an
# actual injected failure, not the happy path. They assert the three crisp-
# terminal invariants from the bead: NO spin (bounded retries), NO silent defer
# (every path emits a terminal verdict + a bead mutation), NO mis-close
# (recovery NEVER calls `br close` on the bead).
#
# `br` and the recheck/remediate commands are injected via a fake on PATH +
# command strings, so the cases are deterministic and offline.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/recovery-statemachine.sh"
  FIX="$(mktemp -d)"
  BR_LOG="$FIX/br-calls.log"
  : > "$BR_LOG"

  # Fake `br`: logs every invocation to BR_LOG; `create` prints a fixed id to
  # stdout (so $(br create ...) capture works); everything else is a no-op.
  mkdir -p "$FIX/bin"
  cat > "$FIX/bin/br" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$BR_LOG"
case "\${1:-}" in
  create)
    # BR_CREATE_FAIL injects a realistic non-zero br create (locked DB, bad deps).
    if [ -n "\${BR_CREATE_FAIL:-}" ]; then echo "br: create failed" >&2; exit 7; fi
    echo "ag-rescope-newid" ;;
  *) : ;;
esac
exit 0
EOF
  chmod +x "$FIX/bin/br"
  PATH="$FIX/bin:$PATH"

  export BEADS_DIR="$FIX/_beads"
  mkdir -p "$BEADS_DIR"
}

teardown() { rm -rf "$FIX"; }

# --- fix-forward: remediation fixes it, recheck goes green --------------------
@test "fix-forward: red recheck recovers after one remediation -> recovered, exit 0" {
  local counter="$FIX/recheck-count"
  # recheck: fail the 1st call, pass the 2nd (the post-remediation call).
  local recheck="n=\$(cat '$counter' 2>/dev/null || echo 0); n=\$((n+1)); echo \$n > '$counter'; [ \"\$n\" -ge 2 ]"

  run "$SCRIPT" --bead ag-1 --failure-kind flake \
    --recheck-cmd "$recheck" --remediate-cmd "true"

  [ "$status" -eq 0 ]
  [[ "$output" == *'"terminal_state":"recovered"'* ]]
  [[ "$output" == *'"spin":false'* ]]
  # mutation happened (a comment), and the bead was NOT closed.
  grep -q "comments add ag-1" "$BR_LOG"
  ! grep -q "^close ag-1" "$BR_LOG"
}

# --- fix-forward exhausted -> andon (no spin) --------------------------------
@test "fix-forward: recheck stays red -> escalates to andon, exit 3, bounded remediation" {
  local mark="$FIX/remediation-runs"
  : > "$mark"
  local remediate="echo x >> '$mark'"      # counts how many times remediation ran
  local recheck="false"                     # never recovers

  run "$SCRIPT" --bead ag-2 --failure-kind drift \
    --recheck-cmd "$recheck" --remediate-cmd "$remediate"

  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
  # NO SPIN: remediation ran at most FIX_FORWARD_BUDGET (1) times.
  [ "$(wc -l < "$mark")" -le 1 ]
  grep -q "update ag-2 --add-label andon" "$BR_LOG"
  ! grep -q "^close ag-2" "$BR_LOG"
}

# --- re-scope-as-new-acceptance ---------------------------------------------
@test "rescope: files a new acceptance bead blocking the original -> rescoped, original not closed" {
  run "$SCRIPT" --bead ag-3 --failure-kind rescope \
    --rescope-scenario "Given the original target is unreachable, When retried, Then a narrower target is used." \
    --reason "API the bead assumed does not exist"

  [ "$status" -eq 0 ]
  [[ "$output" == *'"terminal_state":"rescoped"'* ]]
  [[ "$output" == *'"new_acceptance":"ag-rescope-newid"'* ]]
  # the failure became a NEW acceptance: a follow-up bead that blocks the original.
  grep -q "create .*blocks:ag-3" "$BR_LOG"
  grep -q "update ag-3 --add-label rescoped" "$BR_LOG"
  # NO MIS-CLOSE: the original is not closed.
  ! grep -q "^close ag-3" "$BR_LOG"
}

# --- re-scope when br create itself fails must NOT silently defer -------------
@test "rescope: non-zero br create escalates to andon (no silent defer under set -e), exit 3" {
  export BR_CREATE_FAIL=1
  run "$SCRIPT" --bead ag-9 --failure-kind rescope \
    --rescope-scenario "Given the target is unreachable, When retried, Then narrow it."
  unset BR_CREATE_FAIL

  # A non-zero br create must NOT abort silently with exit 1 — it must reach a
  # terminal andon (verdict + bead mutation), exit 3.
  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
  grep -q "update ag-9 --add-label andon" "$BR_LOG"
  ! grep -q "^close ag-9" "$BR_LOG"
}

# --- re-scope without a scenario must NOT silently defer ----------------------
@test "rescope: missing --rescope-scenario falls to andon (no silent defer), exit 3" {
  run "$SCRIPT" --bead ag-4 --failure-kind rescope

  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
  ! grep -q "^close ag-4" "$BR_LOG"
}

# --- andon: hard failure pulls the cord -------------------------------------
@test "andon: hard failure labels + comments the bead, does NOT close it, exit 3" {
  run "$SCRIPT" --bead ag-5 --failure-kind hard --reason "infra unreachable"

  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
  grep -q "update ag-5 --add-label andon" "$BR_LOG"
  grep -q "comments add ag-5" "$BR_LOG"
  ! grep -q "^close ag-5" "$BR_LOG"
}

# --- default-safe: an unknown failure-kind is andon, never a quiet pass ------
@test "classify: unknown failure-kind defaults to andon (default-safe), exit 3" {
  run "$SCRIPT" --bead ag-6 --failure-kind wat

  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
}

# --- usage errors are loud (exit 2), not deferred ---------------------------
@test "usage: missing --bead exits 2" {
  run "$SCRIPT" --failure-kind hard
  [ "$status" -eq 2 ]
}

@test "usage: missing --failure-kind exits 2" {
  run "$SCRIPT" --bead ag-7
  [ "$status" -eq 2 ]
}

# --- dry-run makes the decision but mutates nothing --------------------------
@test "dry-run: andon decision emitted with NO bead mutation" {
  run "$SCRIPT" --bead ag-8 --failure-kind hard --dry-run

  [ "$status" -eq 3 ]
  [[ "$output" == *'"terminal_state":"andon"'* ]]
  # no real br call was made (dry-run short-circuits br_do).
  [ ! -s "$BR_LOG" ]
}

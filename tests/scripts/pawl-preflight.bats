#!/usr/bin/env bats
# pawl-preflight.bats — the deterministic pre-reviewer gate (age-verification-economics-ebec.9).
# Unit-tests the pure decision logic of pawl_preflight() with stubbed batteries,
# mirroring how pawl-adaptive.bats sources pawl.sh and tests pure functions.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/lib/pawl-preflight.sh"
  WORK="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$WORK"
  # clean env each test
  unset PAWL_NO_PREFLIGHT PAWL_UNTRUSTED_REPO PAWL_PREFLIGHT_CMD AO_BIN
}

@test "opt-out PAWL_NO_PREFLIGHT=1 -> proceed (0), never runs a battery" {
  PAWL_PREFLIGHT_CMD='echo ran-battery; exit 1'  # would be RED if run
  PAWL_NO_PREFLIGHT=1
  run pawl_preflight head "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" != *"ran-battery"* ]]
}

@test "untrusted repo -> proceed (0), never runs a battery" {
  PAWL_PREFLIGHT_CMD='echo ran-battery; exit 1'
  PAWL_UNTRUSTED_REPO=1
  run pawl_preflight head "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" != *"ran-battery"* ]]
}

@test "scope != head (staged) -> proceed (0), no committed HEAD to gate" {
  PAWL_PREFLIGHT_CMD='echo ran-battery; exit 1'
  run pawl_preflight staged "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" != *"ran-battery"* ]]
}

@test "explicit PAWL_PREFLIGHT_CMD nonzero -> RED (3), reviewer must NOT dispatch" {
  PAWL_PREFLIGHT_CMD='echo "custom battery: 2 failures"; exit 1'
  run pawl_preflight head "$WORK"
  [ "$status" -eq 3 ]
  [[ "$output" == *"PAWL-PREFLIGHT: the deterministic battery is RED"* ]]
  [[ "$output" == *"custom battery: 2 failures"* ]]
}

@test "explicit PAWL_PREFLIGHT_CMD zero -> proceed (0), reviewer dispatched" {
  PAWL_PREFLIGHT_CMD='echo "custom battery green"; exit 0'
  run pawl_preflight head "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PREFLIGHT passed (PAWL_PREFLIGHT_CMD green)"* ]]
}

@test "default gate: demonstrably-ran + fail -> RED (3)" {
  # Stub an ao that prints the gate summary marker and fails.
  cat > "$WORK/ao" <<'SH'
#!/usr/bin/env bash
echo "fast/head: 54 checks — 53 pass, 0 warn, 1 fail, 0 skip"
exit 1
SH
  chmod +x "$WORK/ao"
  AO_BIN="$WORK/ao"
  run pawl_preflight head "$WORK"
  [ "$status" -eq 3 ]
  [[ "$output" == *"the deterministic battery is RED"* ]]
}

@test "default gate: demonstrably-ran + green -> proceed (0)" {
  cat > "$WORK/ao" <<'SH'
#!/usr/bin/env bash
echo "fast/head: 54 checks — 54 pass, 0 warn, 0 fail, 0 skip"
exit 0
SH
  chmod +x "$WORK/ao"
  AO_BIN="$WORK/ao"
  run pawl_preflight head "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PREFLIGHT passed (deterministic battery green)"* ]]
}

@test "default gate: nonzero WITHOUT the ran-marker (trust-guard/crash) -> SKIP (0), never false-block" {
  # ao that fails the way the RCE trust-guard does — no "checks —" summary.
  cat > "$WORK/ao" <<'SH'
#!/usr/bin/env bash
echo "Error: refusing to run repo script: the running ao binary is not inside the checkout (RCE guard)" >&2
exit 1
SH
  chmod +x "$WORK/ao"
  AO_BIN="$WORK/ao"
  run pawl_preflight head "$WORK"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PREFLIGHT skipped"* ]]
}

@test "default gate: no ao resolvable -> SKIP (0), proceed" {
  # No AO_BIN, no repo cli/bin/ao; a PATH with the coreutils but NO ao anywhere
  # (system ao lives in ~/.local/bin or ~/go/bin, excluded here).
  run env -u AO_BIN PATH="/usr/bin:/bin" bash -c "source '$REPO_ROOT/scripts/lib/pawl-preflight.sh'; pawl_preflight head '$WORK'"
  [ "$status" -eq 0 ]
}

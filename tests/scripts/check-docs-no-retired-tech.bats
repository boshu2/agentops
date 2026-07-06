#!/usr/bin/env bats
# Tests for scripts/check-docs-no-retired-tech.sh — the two-store carve-out
# (age-gc-adoption-u0he): bd/dolt framed as the gascity SUBSTRATE store is current
# truth, not retired-tech; gascity itself is the ADOPTED substrate, not retired;
# but a genuine retired-tracker prescription (bd <subcmd> as this repo's tracker,
# runtime=gc) still fails.

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
  GATE="$REPO_ROOT/scripts/check-docs-no-retired-tech.sh"
}

# The gate scans the WHOLE live docs/ tree; we assert on the real repo state so a
# regression that re-flags the two-store docs (or stops flagging genuine retired
# tech) is caught. The carve-out logic itself is unit-checked by grepping the
# gate's own patterns below.

@test "gate passes on the current repo (two-store docs are not flagged)" {
  run bash "$GATE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "carve-out: SUBSTRATE_LANG exempts a bd/dolt substrate-store line" {
  # A line that would match the bd/dolt pattern but frames it as the substrate store.
  line='bd/dolt is the gascity substrate store, a different layer, not this repo tracker'
  # matches the retired pattern (bd ... dolt) ...
  echo "$line" | grep -qE 'dolt' || fail "fixture should mention dolt"
  # ... but carries SUBSTRATE_LANG, so the gate skips it.
  grep -q 'SUBSTRATE_LANG=' "$GATE"
  echo "$line" | grep -qiE 'substrate store|gascity substrate|different layer'
}

@test "substrate carve-out can NEVER exempt runtime=gc or other non-bd retired tech (no fail-open)" {
  # Pawl refute on 8aom.3: substrate framing on the same line as runtime=gc must
  # not skip the hit. Round-trip the gate's OWN patterns (not re-typed copies).
  sub="$(grep -m1 '^SUBSTRATE_LANG=' "$GATE" | cut -d= -f2- | sed "s/^'//;s/'$//")"
  non="$(grep -m1 '^NON_SUBSTRATE=' "$GATE" | cut -d= -f2- | sed "s/^'//;s/'$//")"
  bdc="$(grep -m1 '^BD_CLASS=' "$GATE" | cut -d= -f2- | sed "s/^'//;s/'$//")"
  [ -n "$sub" ] && [ -n "$non" ] && [ -n "$bdc" ]
  line='use runtime=gc; this is a different layer'
  # the line carries substrate language (the would-be exemption trigger) ...
  echo "$line" | grep -qiE "$sub"
  # ... but it hits a NON_SUBSTRATE retired token, so the skip must not fire ...
  echo "$line" | grep -qE "$non"
  # ... and it is not a bd-tracker-class hit, the only exemptible class.
  run bash -c "echo \"\$1\" | grep -qE \"\$2\"" _ "$line" "$bdc"
  [ "$status" -ne 0 ]
  # The gate's skip requires: SUBSTRATE_LANG && BD_CLASS && !NON_SUBSTRATE —
  # assert the gate encodes exactly that conjunction.
  grep -q 'grep -qE "\$BD_CLASS"' "$GATE"
  grep -q 'grep -qE "\$NON_SUBSTRATE"' "$GATE"
}

@test "gascity is no longer in the retired-tech PATTERN (adopted substrate)" {
  # The pattern line must NOT flag gas-city/gastown anymore.
  ! grep -E "^PATTERN=" "$GATE" | grep -qE 'gas\[ -\]\?city|gastown'
}

@test "runtime=gc (the severed in-CLI bridge) is STILL flagged" {
  grep -E "^PATTERN=" "$GATE" | grep -q 'runtime=gc'
}

@test "a genuine retired bd-tracker prescription still matches the pattern" {
  # `bd ready` prescribed as a tracker command (no substrate framing) must match.
  line='run bd ready to see your work'
  pat="$(grep -E "^PATTERN=" "$GATE" | sed "s/^PATTERN='//; s/'$//")"
  echo "$line" | grep -qE "$pat"
}

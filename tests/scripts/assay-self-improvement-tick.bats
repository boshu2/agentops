#!/usr/bin/env bats
# assay-self-improvement-tick.bats — acceptance for the M4 ASSAY tick
# (age-d16-self-hosting-route-nkr.5). Drives the loop end-to-end:
#   SENSOR (a fixture provenance ledger) -> ASSAY (default + injected miner)
#     -> GATE (a follow-up suggestion bead is FILED via a stubbed `br create`).
#
# These mirror M2's recovery-statemachine.bats discipline: `br` and the miner
# are injected (a fake on PATH + a --mine-cmd string) so the cases are
# deterministic and offline. They assert the crisp-terminal invariants from the
# bead: evidence-in -> >=1 bead out (UNATTENDED); BOUNDED (capped, no daemon);
# NO silent defer (every path emits one verdict; a failed gate escalates loudly);
# NO mis-close (the tick NEVER calls `br close`).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/assay/self-improvement-tick.sh"
  FIX="$(mktemp -d)"
  BR_LOG="$FIX/br-calls.log"
  : > "$BR_LOG"

  # Fake `br`: logs every invocation; `create` prints a fixed id to stdout (so
  # $(br create ...) capture works), honoring --silent. BR_CREATE_FAIL injects a
  # realistic non-zero create (locked DB / bad deps).
  mkdir -p "$FIX/bin"
  cat > "$FIX/bin/br" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$BR_LOG"
case "\${1:-}" in
  create)
    if [ -n "\${BR_CREATE_FAIL:-}" ]; then echo "br: create failed" >&2; exit 7; fi
    echo "ag-assay-newid" ;;
  *) : ;;
esac
exit 0
EOF
  chmod +x "$FIX/bin/br"
  PATH="$FIX/bin:$PATH"

  export BEADS_DIR="$FIX/_beads"
  mkdir -p "$BEADS_DIR"

  # A fixture ledger with real-shaped verdict rows (two DISTINCT completed-run
  # beads) plus a non-verdict (landed) row the assay must ignore.
  LEDGER="$FIX/ledger.jsonl"
  cat > "$LEDGER" <<'EOF'
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-100@cafef00","from_type":"landed","to_id":"cafef00dbabe1234","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"landed ag-100","trust_tier":"inferred","ts":"2026-06-16T20:00:00Z"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-740@cafef00","from_type":"verdict","to_id":"cafef00dbabe1234","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict ag-740 disposition=CONFIRMED","trust_tier":"inferred","ts":"2026-06-16T20:49:17Z"}
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-742@deadbeef","from_type":"verdict","to_id":"deadbeefcafe5678","to_type":"commit","relation":"wasDerivedFrom","evidence_ref":"pawl-verdict ag-742 disposition=CONFIRMED","trust_tier":"inferred","ts":"2026-06-16T20:49:20Z"}
EOF
}

teardown() { rm -rf "$FIX"; }

# --- CORE: evidence-in -> >=1 follow-up bead out, UNATTENDED -----------------
@test "evidence-in -> files >=1 follow-up suggestion bead (default assay), exit 0" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1

  [ "$status" -eq 0 ]
  [[ "$output" == *'"state":"filed"'* ]]
  [[ "$output" == *'"suggestions_filed":1'* ]]
  # the follow-up RE-ENTERED the front door: a real `br create` with the label.
  grep -q "create ASSAY follow-up.*--labels assay-suggestion" "$BR_LOG"
  # the filed bead id is reported back in the verdict.
  [[ "$output" == *'"filed_beads":["ag-assay-newid"]'* ]]
  # NO MIS-CLOSE: the tick never closes a bead.
  ! grep -q "^close " "$BR_LOG"
}

# --- the suggestion cites the completed-run evidence (head_sha) --------------
@test "filed bead body cites the mined bead + head commit (most-recent evidence)" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1
  [ "$status" -eq 0 ]
  # most-recent verdict row is ag-742@deadbeef -> the title cites ag-742.
  grep -q "create ASSAY follow-up from ag-742" "$BR_LOG"
}

# --- BOUNDED gate: caps beads filed regardless of miner output ---------------
@test "bounded: a miner emitting many suggestions files at most --max-suggestions" {
  # Injected miner ignores stdin and emits 5 suggestion lines; max=2 caps at 2.
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 2 \
    --mine-cmd 'printf "ag-a\tsha-a\tref-a\nag-b\tsha-b\tref-b\nag-c\tsha-c\tref-c\nag-d\tsha-d\tref-d\nag-e\tsha-e\tref-e\n"'

  [ "$status" -eq 0 ]
  [[ "$output" == *'"suggestions_filed":2'* ]]
  [[ "$output" == *'"bounded":true'* ]]
  # exactly two create calls — the cap held.
  [ "$(grep -c '^create ASSAY' "$BR_LOG")" -eq 2 ]
}

# --- DEFAULT ASSAY does not silently DROP a bead across the reverse() backends -
# Regression for the printf '%s' (no trailing newline) bug: EVIDENCE captured
# without a trailing newline made `tac`/`tail -r` GLUE the unterminated final
# verdict row onto its neighbor, so the awk saw two JSON rows on one line and the
# second distinct bead's suggestion was SILENTLY dropped (a silent under-file —
# the "no silent defer" invariant violated). Both distinct beads (ag-740, ag-742)
# must surface when the cap allows both.
@test "default assay surfaces ALL distinct beads (no silent drop from reverse-glue)" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 2
  [ "$status" -eq 0 ]
  [[ "$output" == *'"suggestions_filed":2'* ]]
  # both distinct completed-run beads re-entered the front door.
  grep -q "create ASSAY follow-up from ag-742" "$BR_LOG"
  grep -q "create ASSAY follow-up from ag-740" "$BR_LOG"
  [ "$(grep -c '^create ASSAY' "$BR_LOG")" -eq 2 ]
}

# --- NO daemon / NO spin: a single terminal verdict, run completes -----------
@test "no daemon: emits exactly ONE terminal verdict line and terminates" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1
  [ "$status" -eq 0 ]
  # exactly one JSON verdict line on stdout (no looping output).
  [ "$(printf '%s\n' "$output" | grep -c '"tick":"self-improvement-assay"')" -eq 1 ]
  [[ "$output" == *'"daemonized":false'* ]]
  [[ "$output" == *'"spin":false'* ]]
}

# --- NO-EVIDENCE: crisp terminal, NO bead (not a silent no-op) ---------------
@test "no-evidence: empty ledger -> no-evidence verdict, NO bead filed, exit 0" {
  : > "$FIX/empty.jsonl"
  run "$SCRIPT" --ledger "$FIX/empty.jsonl"

  [ "$status" -eq 0 ]
  [[ "$output" == *'"state":"no-evidence"'* ]]
  [[ "$output" == *'"suggestions_filed":0'* ]]
  ! grep -q "^create " "$BR_LOG"
}

@test "no-evidence: ledger with only non-verdict rows -> no-evidence, NO bead" {
  cat > "$FIX/landed-only.jsonl" <<'EOF'
{"from_id":"ag-1@x","from_type":"landed","to_id":"x","to_type":"commit","evidence_ref":"landed ag-1"}
EOF
  run "$SCRIPT" --ledger "$FIX/landed-only.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"state":"no-evidence"'* ]]
  ! grep -q "^create " "$BR_LOG"
}

# --- NO silent defer: a non-zero `br create` ESCALATES loudly (exit 4) --------
@test "gate-fail: non-zero br create -> loud terminal gate-failed, exit 4 (no silent set -e abort)" {
  export BR_CREATE_FAIL=1
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1
  unset BR_CREATE_FAIL

  [ "$status" -eq 4 ]
  [[ "$output" == *'"state":"gate-failed"'* ]]
  [[ "$output" == *'"error":"br-create-nonzero"'* ]]
  ! grep -q "^close " "$BR_LOG"
}

# --- NO silent defer: a non-zero miner ESCALATES loudly (exit 4) -------------
@test "gate-fail: non-zero --mine-cmd -> loud terminal gate-failed, exit 4" {
  run "$SCRIPT" --ledger "$LEDGER" --mine-cmd 'exit 9'

  [ "$status" -eq 4 ]
  [[ "$output" == *'"state":"gate-failed"'* ]]
  [[ "$output" == *'"error":"mine-cmd-nonzero"'* ]]
  ! grep -q "^create " "$BR_LOG"
}

# --- NO silent defer: default-assay failure ESCALATES loudly (exit 4) --------
# Regression for the refuter's catch: when the default (no --mine-cmd) assay
# pipeline fails, the unguarded substitution used to abort under set -e with NO
# verdict (a silent defer). Force a failure by shadowing `awk` with a stub that
# exits non-zero, and assert a loud gate-failed terminal instead.
@test "gate-fail: default-assay pipeline failure -> loud gate-failed, exit 4 (no silent abort)" {
  cat > "$FIX/bin/awk" <<'EOF'
#!/usr/bin/env bash
exit 3
EOF
  chmod +x "$FIX/bin/awk"

  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1

  [ "$status" -eq 4 ]
  [[ "$output" == *'"state":"gate-failed"'* ]]
  [[ "$output" == *'"error":"default-assay-failed"'* ]]
  ! grep -q "^create " "$BR_LOG"
}

# --- a miner that proposes nothing -> crisp no-suggestions, NO bead ----------
@test "no-suggestions: miner emits nothing -> no-suggestions verdict, NO bead, exit 0" {
  run "$SCRIPT" --ledger "$LEDGER" --mine-cmd 'true'
  [ "$status" -eq 0 ]
  [[ "$output" == *'"state":"no-suggestions"'* ]]
  ! grep -q "^create " "$BR_LOG"
}

# --- DRY-RUN: decides + emits a verdict but mutates nothing -------------------
@test "dry-run: emits the decision with NO br mutation" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions 1 --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *'"state":"dry-run"'* ]]
  [[ "$output" == *'"suggestions_filed":1'* ]]
  # no real br call (no create) was made.
  ! grep -q "^create " "$BR_LOG"
}

# --- usage errors are LOUD (exit 2), not deferred ----------------------------
@test "usage: unknown argument exits 2" {
  run "$SCRIPT" --bogus
  [ "$status" -eq 2 ]
}

@test "usage: non-numeric --max-suggestions exits 2" {
  run "$SCRIPT" --ledger "$LEDGER" --max-suggestions abc
  [ "$status" -eq 2 ]
}

@test "usage: zero --window exits 2" {
  run "$SCRIPT" --ledger "$LEDGER" --window 0
  [ "$status" -eq 2 ]
}

#!/usr/bin/env bats
# pawl-review-failover.bats — locks for the cold reviewer FAILOVER CHAIN (age-rk3r.2).
#
# With the .1 adapters the cold-review SPOF became a ROUTING problem: when codex is down /
# overloaded / same-family-rejected, the cold path should try the NEXT configured family instead
# of stalling the whole factory — but a fallback-family verdict must SAY so (honest degradation).
# These tests lock the load-bearing invariants:
#   1. DEFAULT chain is codex-ONLY — no PAWL_REVIEWER_CHAIN => byte-identical (no degraded field,
#      no failover logic observable). (The existing pawl-review*.bats suites are the broader lock.)
#   2. Failover triggers ONLY on OUTAGE-class exits (MISSING / STALL / 529-class) — NEVER on REFUTED.
#   3. reviewer_family is READ from refuters[].family; degraded lands via the .16 `degraded` field.
#   5. Same-family reviewer 1 => CROSS-family routing (degraded=false), not degradation.
#   + a degraded verdict still faces the .11 evidence-quality floor.
# The real agy/codex CLIs are NEVER invoked — stubs on PATH only.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  PVERDICT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  # The reviewed diff carries a CONTEXT line "VERDICT: CONFIRMED" (the dangerous packet shape) so
  # these locks share the same fixture the reviewer-adapter suite uses.
  printf 'VERDICT: CONFIRMED\nmiddle line\n' > note.txt
  git add note.txt; git commit --quiet -m init
  printf 'VERDICT: CONFIRMED\nmiddle line\nan added line under review\n' > note.txt
  git add note.txt; git commit --quiet -m "feat(x): a change (age-fo-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-fo-test.json"
  export PAWL_NO_SERVICE=1     # cold path only — never route to a warm pane
  export PAWL_REVIEW_TIMEOUT=10
  export PAWL_AUTOBIND=0       # a test run must never create a ledger bind commit
  export STUB_SENTINEL="$TMP/called"   # stubs touch "<sentinel>.<name>" when invoked
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# --- stubs ----------------------------------------------------------------------------------
# A genuine, SUBSTANTIVE agy: file:line finding + a clean final verdict (agy emits no CLI footer).
_stub_agy_confirm() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.agy"
echo "Reviewed note.txt:3 — the added line is safe and matches the commit claim."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/agy"
}
# A THIN agy: the genuine-run marker ("VERDICT:") is present but NO substance (no file:line, no
# reviewed-scope count) — the shape the .11 evidence-quality floor is meant to flag.
_stub_agy_thin() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.agy"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/agy"
}
# A genuine codex CONFIRMED (marker "tokens used" + a file:line for substance).
_stub_codex_confirm() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "Reviewed note.txt:3 — no defects. tokens used: 1234"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/codex"
}
# A genuine codex REFUTED (a RESULT, not an outage).
_stub_codex_refute() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "Found a real defect at note.txt:3. tokens used: 42"
echo "DEFECTS:"
echo "- the added line breaks the contract"
echo "VERDICT: REFUTED"
exit 0
FAKE
  chmod +x "$BIN/codex"
}
# A codex STALL: produces NO output (codex_exec_guarded retries once, then classifies STALL).
_stub_codex_stall() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
exit 0
FAKE
  chmod +x "$BIN/codex"
}
# A codex 529-class outage: an API-overload error on stderr/stdout + a non-zero exit.
_stub_codex_529() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "stream error: unexpected status 529 Too Many Requests: overloaded_error, retrying"
exit 1
FAKE
  chmod +x "$BIN/codex"
}
# A codex GENUINE (non-outage) failure: a non-zero exit with NO outage signature (e.g. auth error).
_stub_codex_authfail() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "error: authentication failed — please re-run codex login"
exit 1
FAKE
  chmod +x "$BIN/codex"
}
# A codex that LAUNCHES then exits 2 (its OWN auth/config failure, NO outage signature).
# The refuter's repro: a launched rc=2 was wrongly classified MISSING → an outage → failover.
# It must fail CLOSED (a truly-absent binary is the pre-launch precondition's job, not this).
_stub_codex_authfail_rc2() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.codex"
cat >/dev/null
echo codex
echo "error: config invalid — please fix ~/.codex/config"
exit 2
FAKE
  chmod +x "$BIN/codex"
}

# ============================================================================================
# ACCEPTANCE
# ============================================================================================

@test "chain codex,agy + codex MISSING -> agy reviews, family gemini, degraded=true, exit 0" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_confirm
  # codex bin absent (CODEX_EXEC_BIN points at a name not on PATH) => the codex precondition MISSES.
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex,agy" \
      bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "gemini" ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
  [[ "$(jq -r '.refuters[0].context_id' "$VFILE")" == agy-fresh-* ]]
  [[ "$output" == *"reviewer 'codex' MISSING"* ]]
  [[ "$output" == *"DEGRADED verdict"* ]]
}

@test "invariant 2: codex REFUTED + chain codex,agy -> exit 3, NO failover (agy never called), NO verdict" {
  _stub_codex_refute
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 3 ]
  [ ! -f "$VFILE" ]
  [ ! -f "$TMP/called.agy" ]      # the REFUTED is FINAL — reviewer 2 is never asked to overturn it
  [[ "$output" == *"REFUTED"* ]]
  [[ "$output" != *"failing over"* ]]
}

@test "invariant 1: NO chain -> byte-identical (single codex, family codex, NO degraded field, no failover noise)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "codex" ]
  [ "$(jq -r 'has("degraded")' "$VFILE")" = "false" ]   # ABSENT, not false
  [[ "$output" != *"degraded"* ]]
  [[ "$output" != *"failing over"* ]]
}

@test "cross-family routing: author gpt + chain codex,agy -> agy selected, NO degraded, codex SKIPPED (not called)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm    # on PATH but must be SKIPPED (same family as the gpt author)
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head --author-family gpt
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "gemini" ]
  [ "$(jq -r 'has("degraded")' "$VFILE")" = "false" ]   # correct routing != degradation
  [ ! -f "$TMP/called.codex" ]    # codex is same-family — skipped, never invoked
  [ -f "$TMP/called.agy" ]
}

@test "outage routing: codex STALL (empty output) + chain codex,agy -> agy, degraded=true" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_stall
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "gemini" ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
  [[ "$output" == *"STALL"* ]]
}

@test "outage routing: codex 529-class (non-zero + overload text) + chain codex,agy -> agy, degraded=true, trail labels 529-class" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_529
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "gemini" ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
  [[ "$output" == *"529-class"* ]]
}

@test "invariant 2 (precision): a NON-outage codex failure (auth error) does NOT fail over even with a fallback" {
  _stub_codex_authfail
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 1 ]             # fail-closed, NOT a failover
  [ ! -f "$VFILE" ]
  [ ! -f "$TMP/called.agy" ]      # a genuine (non-outage) failure never triggers the chain
  [[ "$output" != *"failing over"* ]]
}

@test "invariant 2 (refuter repro): a LAUNCHED codex that exits 2 (its own auth/config failure) does NOT fail over" {
  # rc=2 from a LAUNCHED reviewer is its own failure, NOT a missing-binary outage
  # (that is the pre-launch precondition's job). Old code classified it MISSING →
  # an outage → failover, writing a degraded CONFIRMED from agy on a codex config error.
  _stub_codex_authfail_rc2
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 1 ]             # fail-closed, NOT a failover
  [ ! -f "$VFILE" ]               # no degraded CONFIRMED fabricated
  [ ! -f "$TMP/called.agy" ]      # agy (the fallback) is NEVER invoked
  [[ "$output" != *"failing over"* ]]
}

@test "invariant 2 (unit): _is_outage_class treats STALL(124)/529-class as outages; rc=2/rc=125(ECHO)/rc=1 fail-closed" {
  # Source the classifier (the script's line-690 guard returns on source, before main).
  source "$SCRIPT"
  _is_outage_class 124 /dev/null   # STALL → outage
  [ "$?" -eq 0 ]
  run _is_outage_class 2 /dev/null   # launched auth/config failure → NOT an outage
  [ "$status" -eq 1 ]
  run _is_outage_class 125 /dev/null # ECHO malfunction → fail-closed, NOT an outage
  [ "$status" -eq 1 ]
  run _is_outage_class 1 /dev/null   # generic non-zero, no signature → NOT an outage
  [ "$status" -eq 1 ]
  # But rc≠0 WITH an overload signature IS a 529-class outage.
  ov="$TMP/ov.txt"; printf 'stream error: status 529 overloaded_error\n' > "$ov"
  _is_outage_class 2 "$ov"
  [ "$?" -eq 0 ]
  [ "$_OUTAGE_LABEL" = "529-class" ]
}

@test ".11 floor (advisory): a DEGRADED verdict with THIN evidence -> floor WARNs but still authorizes (exit 0), degraded=true" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_thin
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex,agy" PAWL_FLOOR_ENFORCE=0 \
      bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
  [[ "$output" == *"PAWL-FLOOR: WARN"* ]]   # the .11 floor fires on the degraded fallback (wired)
}

@test ".11 floor (enforcing): a DEGRADED verdict with THIN evidence HOLDs -> check fails, exit 1" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_thin
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex,agy" PAWL_FLOOR_ENFORCE=1 \
      bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-FLOOR: HOLD"* ]]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]   # verdict was written (degraded) then HELD at check
}

@test "a DEGRADED verdict with SUBSTANTIVE evidence passes the floor cleanly (exit 0, degraded=true)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_confirm    # carries note.txt:3 -> substance present
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex,agy" PAWL_FLOOR_ENFORCE=1 \
      bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
  [[ "$output" != *"PAWL-FLOOR: HOLD"* ]]
}

@test "same-family whole chain (author gpt, chain codex only) -> exit 2, SAME model family, NO verdict" {
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex" bash "$SCRIPT" age-fo-test --scope head --author-family gpt
  [ "$status" -eq 2 ]
  [[ "$output" == *"SAME model family"* ]]
  [ ! -f "$VFILE" ]
}

@test "chain tolerates whitespace: PAWL_REVIEWER_CHAIN=\"codex, agy\" + codex MISSING -> agy, degraded=true" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex, agy" \
      bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 0 ]
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "gemini" ]
  [ "$(jq -r '.degraded' "$VFILE")" = "true" ]
}

@test "unknown reviewer in the chain -> fail-closed refusal (exit 2), NO verdict" {
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" PAWL_REVIEWER_CHAIN="codex,totally-not-a-model" bash "$SCRIPT" age-fo-test --scope head
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown reviewer"* ]]
  [ ! -f "$VFILE" ]
}

# ============================================================================================
# pawl-verdict.sh write --degraded render support (age-rk3r.2)
# ============================================================================================

@test "pawl-verdict write --degraded true -> JSON degraded:true (boolean)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  ev="$TMP/ev.txt"; printf 'Reviewed foo.go:12 — safe.\nVERDICT: CONFIRMED\n' > "$ev"
  run bash "$PVERDICT" write age-w 0 --disposition CONFIRMED --head 1234567890abcdef \
      --author-context author-claude-age-w --author-family claude \
      --refuter "gemini:CONFIRMED:agy-fresh-age-w:$ev" --degraded true --dir "$TMP/verdicts"
  [ "$status" -eq 0 ]
  [ "$(jq -r '.degraded' "$TMP/verdicts/age-w.json")" = "true" ]
  [ "$(jq -r '.degraded | type' "$TMP/verdicts/age-w.json")" = "boolean" ]
}

@test "pawl-verdict write --degraded false -> JSON degraded:false; absent -> field OMITTED (byte-identical default)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  ev="$TMP/ev.txt"; printf 'Reviewed foo.go:12 — safe.\nVERDICT: CONFIRMED\n' > "$ev"
  bash "$PVERDICT" write age-f 0 --disposition CONFIRMED --head 1234567890abcdef \
      --author-context author-claude-age-f --author-family claude \
      --refuter "codex:CONFIRMED:codex-fresh-age-f:$ev" --degraded false --dir "$TMP/verdicts" >/dev/null
  [ "$(jq -r '.degraded' "$TMP/verdicts/age-f.json")" = "false" ]
  bash "$PVERDICT" write age-n 0 --disposition CONFIRMED --head 1234567890abcdef \
      --author-context author-claude-age-n --author-family claude \
      --refuter "codex:CONFIRMED:codex-fresh-age-n:$ev" --dir "$TMP/verdicts" >/dev/null
  [ "$(jq -r 'has("degraded")' "$TMP/verdicts/age-n.json")" = "false" ]
}

@test "pawl-verdict write --degraded <garbage> -> exit 2 (fail-closed, not a non-boolean the schema would reject)" {
  ev="$TMP/ev.txt"; printf 'x\nVERDICT: CONFIRMED\n' > "$ev"
  run bash "$PVERDICT" write age-g 0 --disposition CONFIRMED --head 1234567890abcdef \
      --author-context a --author-family claude --refuter "codex:CONFIRMED:c:$ev" --degraded maybe --dir "$TMP/verdicts"
  [ "$status" -eq 2 ]
  [[ "$output" == *"--degraded must be 'true' or 'false'"* ]]
  [ ! -f "$TMP/verdicts/age-g.json" ]
}

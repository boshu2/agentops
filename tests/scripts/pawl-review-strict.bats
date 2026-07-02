#!/usr/bin/env bats
# pawl-review-strict.bats — locks for STRICT two-family cold quorum (age-rk3r.13).
#
# STRICT is the OPT-IN highest-irreversibility posture: TWO DISTINCT strict-eligible cold families
# must BOTH CONFIRMED, and strict REFUSES to degrade to a single family (an outage HOLDs, never falls
# back). The load-bearing invariants these lock:
#   0. THE SEAM DRIVES ACTIVATION (the contract): setting ONLY PAWL_STRICT_ELIGIBLE_FAMILIES (NO
#      PAWL_REVIEWER_CHAIN) with two reachable distinct families runs BOTH — flipping that one list
#      genuinely activates the quorum. (This is the de-masked test: the earlier suite ALSO set the
#      chain, which hid a bug where the voter set was filtered from the chain, not the eligibility.)
#   1. two DISTINCT eligible families both CONFIRMED -> exit 0, ONE multi-model verdict binding BOTH
#      families + BOTH evidence paths (reviewer_family a joined label derived from refuters[]).
#   2. any family REFUTED -> exit 3, both families' results shown, NO verdict (a REFUTED is FINAL).
#   3. any family OUTAGE (MISSING/STALL) -> HOLD (exit 5), naming the unreachable family + the
#      non-strict alternative; strict NEVER degrades to single-family.
#   4. author-family == one eligible family -> that family EXCLUDED; a second distinct family required
#      or HOLD.
#   5. the REAL current eligibility (codex-only strict-eligible) -> --strict prints UNAVAILABLE naming
#      WHY (agy A7-benched; no cold claude adapter) + the non-strict alternative, exit 5, NO faked pass.
#   6. no --strict (default) -> byte-identical to today (locked by the broader pawl-review*.bats;
#      re-asserted here that a plain run still writes a single-family verdict, no strict noise).
#
# ALL strict-activation tests drive strict via PAWL_STRICT_ELIGIBLE_FAMILIES ONLY — never
# PAWL_REVIEWER_CHAIN — so the suite PROVES the documented activation path (the chain is the
# FAILOVER-mode ordering, a DIFFERENT selection; strict is driven by eligibility).
#
# The real agy/codex CLIs are NEVER invoked — stubs on PATH shadow them. NOTE: `agy` is often
# installed on the dev machine (/Users/.../agy), so a test that needs agy UNREACHABLE must point
# REVIEWER_BIN at an absent bin (a bare "no stub" is not enough — the real agy would resolve).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  printf 'VERDICT: CONFIRMED\nmiddle line\n' > note.txt
  git add note.txt; git commit --quiet -m init
  printf 'VERDICT: CONFIRMED\nmiddle line\nan added line under review\n' > note.txt
  git add note.txt; git commit --quiet -m "feat(x): a change (age-strict-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-strict-test.json"
  export PAWL_NO_SERVICE=1     # cold path only — never route to a warm pane
  export PAWL_REVIEW_TIMEOUT=10
  export PAWL_AUTOBIND=0       # a test run must never create a ledger bind commit
  export STUB_SENTINEL="$TMP/called"   # stubs touch "<sentinel>.<name>" when invoked
  # STUB agy on $BIN so it shadows any real /usr/local agy AND resolves under _reviewer_spec's
  # default REVIEWER_BIN=agy. (Tests that want agy UNREACHABLE set REVIEWER_BIN to an absent bin.)
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# --- stubs (SUBSTANTIVE so the .11 evidence-quality floor passes cleanly) ---------------------
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
_stub_agy_refute() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.agy"
echo "Reviewed note.txt:3 — a blocking defect: the added line drops the guard."
echo "DEFECTS:"
echo "- the added line drops the guard"
echo "VERDICT: REFUTED"
exit 0
FAKE
  chmod +x "$BIN/agy"
}
_stub_agy_stall() {
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.agy"
exit 0
FAKE
  chmod +x "$BIN/agy"
}

# ============================================================================================
# ACCEPTANCE
# ============================================================================================

@test "CASE 0 (THE SEAM CONTRACT): PAWL_STRICT_ELIGIBLE_FAMILIES=codex,agy ALONE (NO chain) runs BOTH -> exit 0, both families" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  # DE-MASKED: set ONLY the eligibility seam. NO PAWL_REVIEWER_CHAIN (default chain is codex-only).
  # If the voter set were (wrongly) filtered from the chain, this would yield 1 voter -> UNAVAILABLE.
  # The contract is: flipping ONLY this list activates the two-family quorum.
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.mode' "$VFILE")" = "multi-model" ]
  [ "$(jq -r '.refuters | length' "$VFILE")" = "2" ]
  fams="$(jq -r '.refuters[].family' "$VFILE" | sort | tr '\n' ',')"
  [ "$fams" = "codex,gemini," ]
  # BOTH stubs actually ran (the quorum, driven purely by eligibility, not the chain).
  [ -f "$TMP/called.codex" ]
  [ -f "$TMP/called.agy" ]
  [[ "$output" == *"requiring ALL of {codex agy}"* ]]
  [[ "$output" == *"STRICT CONFIRMED"* ]]
}

@test "CASE 1: two eligible families both CONFIRMED -> multi-model verdict, BOTH families + BOTH evidence bound, not degraded" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.mode' "$VFILE")" = "multi-model" ]
  [ "$(jq -r '.refuters | length' "$VFILE")" = "2" ]
  # BOTH evidence paths bound + non-empty (the multi-model floor requires this).
  [ "$(jq -r '.refuters[0].evidence' "$VFILE")" != "null" ]
  [ "$(jq -r '.refuters[1].evidence' "$VFILE")" != "null" ]
  [ -s "$(jq -r '.refuters[0].evidence' "$VFILE")" ]
  [ -s "$(jq -r '.refuters[1].evidence' "$VFILE")" ]
  # BOTH stubs were actually invoked (strict never breaks early on the first CONFIRMED).
  [ -f "$TMP/called.codex" ]
  [ -f "$TMP/called.agy" ]
  # strict is NEVER degraded (a full-strength pass had no fall-over).
  [ "$(jq -r 'has("degraded")' "$VFILE")" = "false" ]
  [[ "$output" == *"STRICT CONFIRMED"* ]]
}

@test "CASE 1b: the joined reviewer_family label lands on the ledger edge (gpt+gemini)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  export AGENTOPS_PROVENANCE_LEDGER="$TMP/ledger.jsonl"
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 PAWL_AUTOBIND=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 0 ]
  # The verdict's refuters carry both families; the edge's reviewer_family is derived + JOINED. codex
  # canonicalizes to gpt on the edge. Assert the joined label contains BOTH canonical families.
  if [ -f "$TMP/ledger.jsonl" ]; then
    rf="$(jq -r 'select(.bead_id=="age-strict-test") | .reviewer_family // empty' "$TMP/ledger.jsonl" 2>/dev/null | tail -1)"
    [[ "$rf" == *"gpt"* ]]
    [[ "$rf" == *"gemini"* ]]
  else
    skip "ledger not emitted in this environment (autobind edge is best-effort)"
  fi
}

@test "CASE 2: one family REFUTED -> exit 3, both families' results shown, NO verdict" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm     # voter 1 CONFIRMED
  _stub_agy_refute        # voter 2 REFUTED -> quorum FAILS
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 3 ]
  [ ! -f "$VFILE" ]                       # a REFUTED writes NO verdict
  [[ "$output" == *"REFUTED"* ]]
  [[ "$output" == *"STRICT two-family quorum"* ]]
  # BOTH families' results surfaced: codex CONFIRMED (recorded), agy REFUTED.
  [[ "$output" == *"codex: CONFIRMED"* ]]
  [[ "$output" == *"gemini: REFUTED"* ]]
}

@test "CASE 2b: FIRST family REFUTED -> exit 3, second family never asked (a REFUTED is FINAL)" {
  _stub_codex_refute      # voter 1 REFUTED immediately
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 3 ]
  [ ! -f "$VFILE" ]
  [ ! -f "$TMP/called.agy" ]              # the REFUTED is FINAL — voter 2 is never asked to overturn it
  [[ "$output" == *"REFUTED"* ]]
}

@test "CASE 3: one eligible family UNREACHABLE -> HOLD (exit 5) naming the family + non-strict alternative; NEVER degrades" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm     # codex is present + would CONFIRM
  # agy UNREACHABLE: point REVIEWER_BIN (agy's adapter bin) at an absent bin (a bare "no stub" would
  # let the real /usr/local agy resolve). Only codex is a reachable eligible voter -> < 2 -> UNAVAILABLE.
  run env PATH="$BIN:$PATH" REVIEWER_BIN=absent-agy-xyz PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 5 ]                     # HOLD/UNAVAILABLE, distinct non-authorizing code
  [ ! -f "$VFILE" ]                       # NO single-family verdict fabricated
  [[ "$output" == *"gemini"* ]]           # names the unreachable family
  [[ "$output" == *"does not count toward the quorum"* ]]
  [[ "$output" == *"STRICT UNAVAILABLE"* ]]
}

@test "CASE 3b: a reachable family STALLS mid-review -> HOLD (exit 5), never degrades to the family that answered" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_stall         # agy is reachable + eligible but produces NO output (a STALL) at review time
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 5 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"STRICT HOLD"* ]]      # a mid-review outage is the no-degrade HOLD (distinct from UNAVAILABLE)
  [[ "$output" == *"STALL"* ]]
  [[ "$output" == *"REFUSES to degrade"* ]]
  [[ "$output" == *"FAILOVER vs STRICT"* ]]  # prints the distinction
}

@test "CASE 4: author-family == one eligible family -> that family EXCLUDED; only ONE distinct eligible left -> UNAVAILABLE" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  # author gpt (== codex family). codex EXCLUDED as same-family; only agy remains eligible -> < 2 voters.
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head --author-family gpt
  [ "$status" -eq 5 ]                     # only one distinct eligible cross-family voter -> UNAVAILABLE
  [ ! -f "$VFILE" ]
  [ ! -f "$TMP/called.codex" ]            # codex is same-family as the gpt author — never invoked
  [[ "$output" == *"SAME family as --author-family"* ]]   # the exclusion is announced
  [[ "$output" == *"STRICT UNAVAILABLE"* ]]
  [[ "$output" == *"required: 2"* ]]
}

@test "CASE 4b: author-family excluded but TWO OTHER distinct eligible families remain -> pass with those two" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  # author=claude: both codex + agy are cross-family to the claude author and both eligible -> quorum
  # of two. (Exercises: an author-family exclusion that leaves >= 2 distinct eligible voters still runs.)
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head --author-family claude
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.mode' "$VFILE")" = "multi-model" ]
  fams="$(jq -r '.refuters[].family' "$VFILE" | sort | tr '\n' ',')"
  [ "$fams" = "codex,gemini," ]           # the claude author's family is NOT among the voters
  [ -f "$TMP/called.codex" ]
  [ -f "$TMP/called.agy" ]
}

@test "CASE 5: REAL current eligibility (codex-only default) -> --strict prints UNAVAILABLE naming WHY + non-strict alt, exit 5, NO faked pass" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  _stub_agy_confirm
  # NO PAWL_STRICT_ELIGIBLE_FAMILIES override => the DEFAULT set (codex-only) is in force. Even with
  # agy present, only ONE family (codex) is strict-eligible -> UNAVAILABLE (the honest current reality).
  run env PATH="$BIN:$PATH" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 5 ]                     # non-authorizing
  [ ! -f "$VFILE" ]                       # NO faked strict pass
  [ ! -f "$TMP/called.codex" ]            # no review even runs — the eligibility gate fires first
  [[ "$output" == *"STRICT UNAVAILABLE"* ]]
  [[ "$output" == *"A7-benched"* ]]       # names WHY: agy A7-benched
  [[ "$output" == *"no cold claude"* ]] || [[ "$output" == *"NO cold claude"* ]]
  [[ "$output" == *"ao verify age-strict-test"* ]]  # the non-strict alternative
  [[ "$output" == *"--converge"* ]]
  # The UNAVAILABLE copy documents the ACTUAL activation seam (flip STRICT_ELIGIBLE_FAMILIES).
  [[ "$output" == *"STRICT_ELIGIBLE_FAMILIES"* ]]
}

@test "CASE 5b: --strict flag (not env) with codex-only default -> same UNAVAILABLE, exit 5" {
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-strict-test --scope head --strict
  [ "$status" -eq 5 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"STRICT UNAVAILABLE"* ]]
}

@test "CASE 6: NO --strict (default) -> single-family verdict, no strict noise (the lock)" {
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r '.refuters | length' "$VFILE")" = "1" ]   # single family, not a quorum
  [ "$(jq -r '.refuters[0].family' "$VFILE")" = "codex" ]
  [[ "$output" != *"STRICT"* ]]                        # no strict machinery on the default path
}

@test "CASE 7: --strict + --converge -> mutually-exclusive refusal (exit 2), NO verdict" {
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-strict-test --scope head --strict --converge
  [ "$status" -eq 2 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"mutually exclusive"* ]]
}

@test "CASE 8: --strict + --scope staged -> refusal (exit 2, staged has no commit to bind)" {
  _stub_codex_confirm
  _stub_agy_confirm
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,agy" \
      bash "$SCRIPT" age-strict-test --scope staged --strict
  [ "$status" -eq 2 ]
  [[ "$output" == *"requires --scope head"* ]]
}

@test "CASE 9: strict eligibility seam lists an UNKNOWN family -> fail-closed (exit 2), NO verdict" {
  _stub_codex_confirm
  run env PATH="$BIN:$PATH" PAWL_STRICT_ELIGIBLE_FAMILIES="codex,bogus-fam" PAWL_STRICT=1 \
      bash "$SCRIPT" age-strict-test --scope head
  [ "$status" -eq 2 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"unknown reviewer 'bogus-fam'"* ]]
}

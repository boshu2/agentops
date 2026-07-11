#!/usr/bin/env bats
# age-rk3r.11: EVIDENCE-QUALITY FLOOR — a CONFIRMED verdict must carry review SUBSTANCE
# (at least one file:line finding OR an explicit reviewed-scope attestation) PLUS, for each
# refuter attributable to a cold reviewer adapter, that adapter's genuine-run marker in its
# evidence (codex="tokens used", agy="VERDICT:", per the .1 contract in scripts/lib/codex-exec.sh).
#
# The floor ships ADVISORY-FIRST: it MEASURES + WARNS but does NOT block until the flip date
# (FLOOR_ENFORCE_AFTER), so the false-positive rate on real reviews can be observed for one
# cycle before it fail-closes. This suite pins BOTH postures CLOCK-INDEPENDENTLY via the
# PAWL_FLOOR_ENFORCE override (and the date gate via PAWL_FLOOR_ENFORCE_AFTER), and drift-locks
# the per-adapter marker map against the codex-exec.sh reviewer-adapter contract.
#
# FIXTURE FIDELITY: verdicts are produced by the PRODUCTION writer (`pawl-verdict.sh write`),
# never hand-rolled JSON. Evidence files are the reviewer's verbatim output — the exact shape
# pawl-review.sh writes to the evidence file (it `printf`s the reviewer block straight to it).
# `ao` is STUBBED via PATH so the write's provenance sensor is hermetic; `jq` is real.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  LIB="$REPO_ROOT/scripts/lib/codex-exec.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin" "$TMP/verdicts"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/ao"; chmod +x "$TMP/bin/ao"
  export PATH="$TMP/bin:$PATH"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"

  # --- evidence shapes (the reviewer's verbatim output = the production evidence shape) ---
  # thin: the 155-byte "no blocking defects" stub — no location, no count, no marker.
  printf 'Reviewed the change. No blocking defects found.\nVERDICT: CONFIRMED\n' > "$TMP/thin.txt"
  # substantive: cites file:line + carries the codex genuine-run marker.
  printf 'Read scripts/pawl-verdict.sh:455 and cli/cmd/ao/foo.go:12 — placed correctly, fails closed.\nNOTES: none blocking.\nVERDICT: CONFIRMED\ntokens used: 4211\n' > "$TMP/subst.txt"
  # attestation-only: a files-reviewed COUNT (no file:line) + codex marker.
  printf 'Files reviewed: 2 (scripts/pawl-verdict.sh, schemas/pawl-verdict.v1.schema.json). Nothing blocking.\nVERDICT: CONFIRMED\ntokens used: 900\n' > "$TMP/attest.txt"
  # file:line substance present, but the codex "tokens used" marker is MISSING.
  printf 'Read scripts/pawl-verdict.sh:455 — fine.\nVERDICT: CONFIRMED\n' > "$TMP/nomarker_gpt.txt"
  # file:line substance present, but the agy "VERDICT:" marker is MISSING (no verdict token).
  printf 'Read scripts/pawl-verdict.sh:455 — fine. Nothing blocking; approving.\n' > "$TMP/nomarker_agy.txt"
}

teardown() {
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# write a CONFIRMED verdict via the PRODUCTION writer (fixture fidelity).
write_v() {
  local bead="$1" refuter="$2"
  bash "$SCRIPT" write "$bead" 0 \
    --disposition CONFIRMED --head "$SHA" --author-context author-ctx \
    --refuter "$refuter" --dir "$TMP/verdicts" >/dev/null 2>&1
}

# --- ACCEPTANCE 1: thin evidence -> post-flip HOLD naming the floor; pre-flip WARN-only -----
@test "thin evidence (no finding, no attestation): ENFORCE => HOLD naming the evidence-quality floor" {
  write_v thin "claude:CONFIRMED:fresh:$TMP/thin.txt"   # claude => no adapter marker, isolate the substance floor
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check thin 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"HOLD (evidence-quality floor)"* ]]
  [[ "$output" == *"merge refused"* ]]
}

@test "thin evidence: ADVISORY (pre-flip) still AUTHORIZES, WARN-only (behavior-lock window)" {
  write_v thin "claude:CONFIRMED:fresh:$TMP/thin.txt"
  PAWL_FLOOR_ENFORCE=0 run bash "$SCRIPT" check thin 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
  [[ "$output" == *"WARN (advisory; evidence-quality floor)"* ]]
}

# --- ACCEPTANCE 2: a real substantive review passes UNTOUCHED, even when enforcing ----------
@test "substantive evidence (file:line finding): ENFORCE => AUTHORIZES" {
  write_v subst "gpt:CONFIRMED:fresh:$TMP/subst.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check subst 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "attestation-only evidence (files-reviewed count, no file:line): ENFORCE => AUTHORIZES" {
  write_v attest "gpt:CONFIRMED:fresh:$TMP/attest.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check attest 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "real-review fixture (checked-in substantive evidence) passes UNTOUCHED under ENFORCE (fixture fidelity)" {
  local fx="$REPO_ROOT/tests/fixtures/provenance/pawl-review-substantive-evidence.txt"
  [ -f "$fx" ]
  write_v realfx "gpt:CONFIRMED:fresh:$fx"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check realfx 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

# --- ACCEPTANCE 3: an adapter without its genuine-run marker -> post-flip HOLD naming it ----
@test "gpt/codex refuter missing 'tokens used' marker: ENFORCE => HOLD naming the adapter" {
  write_v nomk_gpt "gpt:CONFIRMED:fresh:$TMP/nomarker_gpt.txt"   # has file:line substance, no marker
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check nomk_gpt 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"HOLD (adapter genuine-run floor)"* ]]
  [[ "$output" == *"family 'gpt'"* ]]
  [[ "$output" == *"tokens used"* ]]
}

@test "standing codex pawl-pane evidence without cold-adapter marker: ENFORCE => AUTHORIZES" {
  # Standing TUI evidence is not the cold Codex adapter subprocess; its proof surface is the
  # nonce-bound route transcript plus captured pane output, so it must not inherit the adapter
  # footer requirement. The previous test still locks the cold-adapter rejection.
  write_v standing_codex "gpt:CONFIRMED:codex-pawl-pane-gpt55:$TMP/nomarker_gpt.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check standing_codex 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

@test "agy/gemini refuter missing 'VERDICT:' marker: ENFORCE => HOLD naming the adapter" {
  write_v nomk_agy "agy:CONFIRMED:fresh:$TMP/nomarker_agy.txt"   # has file:line substance, no VERDICT: token
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check nomk_agy 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"HOLD (adapter genuine-run floor)"* ]]
  [[ "$output" == *"family 'gemini'"* ]]
}

@test "missing adapter marker: ADVISORY (pre-flip) WARNs but AUTHORIZES (advisory-first for markers too)" {
  write_v nomk_gpt "gpt:CONFIRMED:fresh:$TMP/nomarker_gpt.txt"
  PAWL_FLOOR_ENFORCE=0 run bash "$SCRIPT" check nomk_gpt 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
  [[ "$output" == *"WARN (advisory; adapter genuine-run floor)"* ]]
}

@test "claude refuter (no cold-adapter marker defined in .1) is advisory-SKIPPED: ENFORCE + substance => AUTHORIZES" {
  # nomarker_gpt.txt has file:line substance but no marker; claude has no expected marker,
  # so the marker floor must not HOLD it (duel amendment 1: advisory when an adapter has no marker).
  write_v cl "claude:CONFIRMED:fresh:$TMP/nomarker_gpt.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check cl 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
}

# --- HONESTY (acceptance-tested): the output SAYS it measures substance, not correctness ----
@test "honesty caveat: the floor output states it measures SUBSTANCE, NOT correctness" {
  write_v subst "gpt:CONFIRMED:fresh:$TMP/subst.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check subst 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"measures review SUBSTANCE"* ]]
  [[ "$output" == *"NOT correctness"* ]]
  [[ "$output" == *"can still be wrong"* ]]
}

# --- FLIP MECHANISM: the DATE gate drives enforce/advisory when no override is set ----------
@test "date gate: a PAST flip date (no override) ENFORCES => thin HOLDs" {
  write_v thin "claude:CONFIRMED:fresh:$TMP/thin.txt"
  PAWL_FLOOR_ENFORCE_AFTER=2000-01-01 run bash "$SCRIPT" check thin 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"HOLD (evidence-quality floor)"* ]]
}

@test "date gate: a FUTURE flip date (no override) stays ADVISORY => thin AUTHORIZES" {
  write_v thin "claude:CONFIRMED:fresh:$TMP/thin.txt"
  PAWL_FLOOR_ENFORCE_AFTER=2999-01-01 run bash "$SCRIPT" check thin 0 --dir "$TMP/verdicts" --head "$SHA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"merge authorized"* ]]
  [[ "$output" == *"(ADVISORY)"* ]]
}

# --- DRIFT LOCK: the floor's per-family marker map MUST match the .1 adapter contract -------
@test "drift lock: _family_genuine_marker mirrors codex-exec.sh reviewer_adapter_marker (codex + agy)" {
  # Source the .1 lib and read the CONTRACT markers straight from it.
  # shellcheck source=/dev/null
  . "$LIB"
  local codex_marker agy_marker
  codex_marker="$(reviewer_adapter_marker codex)"
  agy_marker="$(reviewer_adapter_marker agy)"
  [ "$codex_marker" = "tokens used" ]
  [ "$agy_marker" = "VERDICT:" ]
  # Now prove pawl-verdict.sh's RUNTIME map emits the SAME strings: a gpt refuter with no
  # marker HOLDs naming the codex marker; a gemini refuter with no marker HOLDs naming agy's.
  write_v drift_gpt "gpt:CONFIRMED:fresh:$TMP/nomarker_gpt.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check drift_gpt 0 --dir "$TMP/verdicts" --head "$SHA"
  [[ "$output" == *"$codex_marker"* ]]
  write_v drift_agy "agy:CONFIRMED:fresh:$TMP/nomarker_agy.txt"
  PAWL_FLOOR_ENFORCE=1 run bash "$SCRIPT" check drift_agy 0 --dir "$TMP/verdicts" --head "$SHA"
  [[ "$output" == *"$agy_marker"* ]]
}

# --- REUSABILITY (duel amendment 2): the floor is a callable function for the .2 degraded path
@test "pawl_evidence_floor is a reusable function callable outside do_check (for the .2 failover path)" {
  write_v subst "gpt:CONFIRMED:fresh:$TMP/subst.txt"
  write_v thin  "claude:CONFIRMED:fresh:$TMP/thin.txt"
  # Source the script (its `check` dispatch RETURNs, never exits, so all functions — incl.
  # pawl_evidence_floor — are left defined), then invoke the floor directly and assert its
  # documented return contract (0 = authorize / advisory, 1 = enforcing + violation). This
  # is the seam .2's degraded-fallback path calls to gate a failover verdict.
  run bash -c '
    set -uo pipefail
    . "'"$SCRIPT"'" check __sourced-noop__ 0 --dir "'"$TMP"'/verdicts" >/dev/null 2>&1 || true
    declare -F pawl_evidence_floor >/dev/null || { echo "NOT-A-FUNCTION"; exit 9; }
    PAWL_FLOOR_ENFORCE=1 pawl_evidence_floor "'"$TMP"'/verdicts/subst.json" >/dev/null 2>&1 && echo "subst=authorize"
    PAWL_FLOOR_ENFORCE=1 pawl_evidence_floor "'"$TMP"'/verdicts/thin.json"  >/dev/null 2>&1 || echo "thin=hold"
  '
  [[ "$output" != *"NOT-A-FUNCTION"* ]]
  [[ "$output" == *"subst=authorize"* ]]
  [[ "$output" == *"thin=hold"* ]]
}

# age-pawl-intent-zhndq.10 (measured 2026-07-11): the floor's false-positive rate on REAL reviews is
# 76% (199/261 live verdicts FLOOR-fail, incl. verdicts from reviews that caught real defects) — the
# check conflates "no defects found" with "no review performed", so a clean CONFIRM always fails it.
# The 2026-07-16 auto-flip would therefore have started HOLDing ~76% of real reviews on a CALENDAR
# TRIGGER with nobody watching. This test pins the defusal: enforcement must NOT be reachable by the
# clock alone in the near term — it is an explicit, evidence-gated decision (or PAWL_FLOOR_ENFORCE=1).
@test "floor: the enforce date cannot auto-fire in the near term (76% FP measured; explicit flip only)" {
  # The default flip date must be comfortably in the future, not days away.
  run bash -c 'grep -E "^FLOOR_ENFORCE_AFTER=" "'"$BATS_TEST_DIRNAME"'/../../scripts/pawl-verdict.sh"'
  [ "$status" -eq 0 ]
  [[ "$output" != *"2026-07-16"* ]]     # the defused time-bomb
  [[ "$output" == *"2027-"* ]]          # pushed out deliberately
  # And the operator override still works both ways (tests/opt-in enforcement stays possible).
  [[ "$output" == *"PAWL_FLOOR_ENFORCE_AFTER"* ]]
}

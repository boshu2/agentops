#!/usr/bin/env bats
# pawl-verdict-meter.bats — reviewer counterexamples for the age-ivoq meter at
# the pawl-verdict.sh `write` chokepoint. Each test is an EXECUTED-red
# counterexample from the cross-family review of 5b70fafe2:
#
#   1. FAIL-OPEN: a leading-zero "tokens used" total (08) used to hit bash
#      OCTAL arithmetic ("value too great for base") — a FATAL expansion error
#      that aborted the write BEFORE the verdict existed. A meter bug must
#      never block verdict writing; extraction must be structurally guarded.
#   2. IDEMPOTENCY: the usage dedup key included the measured values (tokens,
#      wall), so re-running the same bead/run with a different wall reading
#      double-counted spend. The key is the review event's identity
#      (bead, run, phase) — never its measurements.
#   3. PROVENANCE: (a) same-line parse grabbed the FIRST number on the line
#      ("attempt 2 tokens used: 1,234" -> 2); (b) a MEASURED zero was
#      reclassified as a bytes/4 estimate; (c) evidence paths containing ':'
#      were truncated by ${tok##*:}.
#
# Sandbox pattern mirrors tests/integration/test-pawl-verdict-yield-emit.sh:
# copy the script under test INTO a temp repo so its script-relative REPO_ROOT
# (and thus YIELD_ROOT + the verdict dir) resolve to the sandbox.

setup() {
  command -v jq >/dev/null 2>&1 || skip "jq not available"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  AO_BUILT="$REPO_ROOT/cli/bin/ao"
  [ -x "$AO_BUILT" ] || skip "no built ao at cli/bin/ao (make -C cli build)"

  TMP="$(mktemp -d)"
  BIN="$TMP/bin"; mkdir -p "$BIN"; ln -sf "$AO_BUILT" "$BIN/ao"
  PATH="$BIN:$PATH"

  REPO="$TMP/repo"
  mkdir -p "$REPO"
  cd "$REPO" || return 1
  git init -q -b main
  git config user.email t@t
  git config user.name t
  mkdir -p .agents/yield .agents/pawl-verdicts docs/provenance scripts/lib
  cp "$REPO_ROOT/scripts/pawl-verdict.sh" scripts/pawl-verdict.sh
  cp "$REPO_ROOT/scripts/lib/diff-identity.sh" scripts/lib/diff-identity.sh
  chmod +x scripts/pawl-verdict.sh
  PAWL="$REPO/scripts/pawl-verdict.sh"
  echo seed > seed.txt
  git add -A
  git commit -qm seed --no-verify
  HEAD_SHA="$(git rev-parse HEAD)"
  LEDGER="$REPO/.agents/yield/yield-ledger.jsonl"
}

teardown() {
  cd / || true
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP" 2>/dev/null || true
}

# usage_row <bead> — print "<tokens_total> <tokens_source>" for the bead's usage event(s)
usage_row() {
  jq -r --arg b "$1" \
    'select(.event=="usage" and .bead_id==$b) | "\(.body.tokens_total) \(.body.tokens_source)"' \
    "$LEDGER" 2>/dev/null
}

# usage_count <bead> — number of usage rows for the bead
usage_count() {
  jq -c --arg b "$1" 'select(.event=="usage" and .bead_id==$b)' "$LEDGER" 2>/dev/null | grep -c .
}

@test "DEFECT 1 fail-open: leading-zero next-line total (08) must not abort the verdict write" {
  printf 'VERDICT: CONFIRMED\ntokens used\n08\n' > "$TMP/ev-octal"
  run bash "$PAWL" write age-octal 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-octal" \
    --wall-seconds 5
  # The meter must NEVER block verdict creation (fail-open contract).
  [ "$status" -eq 0 ]
  [ -f "$REPO/.agents/pawl-verdicts/age-octal.json" ]
  # And the leading-zero total reads base-10: measured 8, never octal/abort.
  [ "$(usage_row age-octal)" = "8 measured" ]
}

@test "DEFECT 2 idempotency: same bead/run with wall 10 then 11 emits exactly ONE usage row" {
  printf 'VERDICT: CONFIRMED\ntokens used: 1,234\n' > "$TMP/ev-idem"
  bash "$PAWL" write age-idem 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-idem" \
    --wall-seconds 10 >/dev/null 2>&1 || true
  bash "$PAWL" write age-idem 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-idem" \
    --wall-seconds 11 >/dev/null 2>&1 || true
  # The dedup key is the review event's identity (bead, run, phase, attempt) —
  # a different measured wall reading must NOT double-count the same review.
  [ "$(usage_count age-idem)" = "1" ]
}

@test "DEFECT 2b attempt distinctness: two CONFIRMED attempts in one run emit TWO usage rows, and a replay dedups" {
  # Round-2 reviewer counterexample (age-ivoq): identity-only dedup WITHOUT the
  # attempt silently dropped legitimate re-review spend. Two CONFIRMED reviews
  # of one bead in one run (attempt 1 on head A, then reworked and attempt 2 on
  # head B) are DISTINCT reviews that each spent tokens; keying on (bead, run,
  # phase) alone collapsed them to one row and under-counted R. A REPLAY of the
  # SAME attempt still dedups (that is the wall-reading-changes case from 2).
  printf 'VERDICT: CONFIRMED\ntokens used: 1,000\n' > "$TMP/ev-a1"
  printf 'VERDICT: CONFIRMED\ntokens used: 2,000\n' > "$TMP/ev-a2"
  bash "$PAWL" write age-att 0 \
    --disposition CONFIRMED --head "0000000000000000000000000000000000000001" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-a1" \
    --attempt 1 --wall-seconds 10 >/dev/null 2>&1 || true
  bash "$PAWL" write age-att 0 \
    --disposition CONFIRMED --head "0000000000000000000000000000000000000002" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-a2" \
    --attempt 2 --wall-seconds 12 >/dev/null 2>&1 || true
  [ "$(usage_count age-att)" = "2" ]
  # Replay attempt 2 with a different wall reading — same identity, still one row.
  bash "$PAWL" write age-att 0 \
    --disposition CONFIRMED --head "0000000000000000000000000000000000000002" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-a2" \
    --attempt 2 --wall-seconds 13 >/dev/null 2>&1 || true
  [ "$(usage_count age-att)" = "2" ]
}

@test "DEFECT 3a anchor: 'attempt 2 tokens used: 1,234' extracts 1234, not the 2" {
  printf 'attempt 2 tokens used: 1,234\nVERDICT: CONFIRMED\n' > "$TMP/ev-anchor"
  bash "$PAWL" write age-anchor 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-anchor" \
    --wall-seconds 5 >/dev/null 2>&1 || true
  [ "$(usage_row age-anchor)" = "1234 measured" ]
}

@test "DEFECT 3b measured zero: 'tokens used: 0' stays tokens_source=measured with value 0" {
  printf 'VERDICT: CONFIRMED\ntokens used: 0\n' > "$TMP/ev-zero"
  bash "$PAWL" write age-zero 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-zero" \
    --wall-seconds 5 >/dev/null 2>&1 || true
  # Zero means measured-zero — never reclassified as a bytes/4 estimate.
  [ "$(usage_row age-zero)" = "0 measured" ]
  run jq -e '.cost.tokens_est==0 and .cost.estimated==false' \
    "$REPO/.agents/pawl-verdicts/age-zero.json"
  [ "$status" -eq 0 ]
}

@test "DEFECT 3c colon path: evidence path containing ':' is metered intact" {
  mkdir -p "$TMP/ev:dir"
  printf 'VERDICT: CONFIRMED\ntokens used: 5,000\n' > "$TMP/ev:dir/ev-colon"
  bash "$PAWL" write age-colon 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev:dir/ev-colon" \
    --wall-seconds 5 >/dev/null 2>&1 || true
  # Evidence is everything after the third field — a ':' inside the path must
  # not truncate it (the old \${tok##*:} split lost the file entirely).
  [ "$(usage_row age-colon)" = "5000 measured" ]
}

@test "DEFECT 3d malformed total: tokens-used-1x is NOT recorded as measured (round-3 counterexample)" {
  printf 'VERDICT: CONFIRMED\ntokens used: 1x\n' > "$TMP/ev-mal"
  bash "$PAWL" write age-mal 0 \
    --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-mal" --wall-seconds 5 >/dev/null 2>&1 || true
  row="$(usage_row age-mal)"
  [[ "$row" != "1 measured" ]]
  [[ "$row" == *estimated* || "$row" == *unknown* ]]
}

@test "DEFECT 3e attempt 0 coerces to 1 intentionally, not a silent phantom collision" {
  printf 'VERDICT: CONFIRMED\ntokens used: 1,000\n' > "$TMP/ev-z1"
  printf 'VERDICT: CONFIRMED\ntokens used: 2,000\n' > "$TMP/ev-z2"
  bash "$PAWL" write age-z 0 \
    --disposition CONFIRMED --head "0000000000000000000000000000000000000011" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-z1" --attempt 0 --wall-seconds 10 >/dev/null 2>&1 || true
  bash "$PAWL" write age-z 0 \
    --disposition CONFIRMED --head "0000000000000000000000000000000000000012" --author-context ctx-author \
    --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev-z2" --attempt 1 --wall-seconds 11 >/dev/null 2>&1 || true
  [ "$(usage_count age-z)" = "1" ]
}

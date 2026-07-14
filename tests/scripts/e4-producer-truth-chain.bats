#!/usr/bin/env bats
# E4.TEST (age-membrane-memory-arch-tz2s.3.4) — producer-truth + seam wiring
# verified end to end. Asserts the WHOLE medallion chain in ONE isolated flow
# (the component seams are tested individually elsewhere; this ties them):
#
#   BRONZE  a real session transcript with per-message usage blocks
#     -> SILVER  `ao yield emit usage` lands a yield-ledger usage event whose
#                tokens_in/out are the REAL footprint derived from the transcript
#                (the age-ptts producer seam), NOT the old hardcoded 0
#     -> PROVENANCE  a CONFIRMED pawl verdict emits a verdict->commit PROV-O edge
#                    (trunk-only / CONFIRMED-only, per the z9p0 council Option A)
#
# Real components only (ao, pawl-verdict.sh); ledgers isolated by running from a
# throwaway repo (both ao yield + the provenance emit resolve their ledger by
# walking up from CWD), so the real repo ledgers are never touched.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  if [ -n "${AO_TEST_BIN:-}" ] && [ -x "${AO_TEST_BIN}" ]; then
    AO="$AO_TEST_BIN"
  elif [ -x "$REPO_ROOT/cli/bin/ao" ]; then
    AO="$REPO_ROOT/cli/bin/ao"
  else
    AO="$BATS_FILE_TMPDIR/ao"
    ( cd "$REPO_ROOT/cli" && go build -o "$AO" ./cmd/ao ) || AO=""
  fi
  export REPO_ROOT AO
}

setup() {
  # This suite asserts the STRICT verdict->ledger EDGE contract (line 83 requires
  # the PROV ledger to actually gain the edge). Strip any ambient
  # PAWL_EDGE_FAIL_OPEN the CI bats harness sets suite-wide (wave 1), so a broken
  # emit is a hard failure here rather than a silent warn-and-continue.
  unset PAWL_EDGE_FAIL_OPEN
  WORK="$BATS_TEST_TMPDIR/proj"
  mkdir -p "$WORK"
  ( cd "$WORK" && git init -q && git config user.email t@t && git config user.name t \
      && echo seed > seed.txt && git add . && git commit -qm "seed ag-e4 work" )
  YIELD_LEDGER="$WORK/.agents/yield/yield-ledger.jsonl"
  PROV_LEDGER="$WORK/docs/provenance/ledger.jsonl"
}

@test "E4 chain: real transcript -> silver usage (real tokens) -> provenance verdict->commit edge" {
  [ -n "$AO" ] || skip "no ao binary available"

  # ── BRONZE: a faithful session transcript (production parser shape) ─────────
  cat > "$WORK/session.jsonl" <<'EOF'
{"type":"assistant","timestamp":"2026-04-11T12:00:00Z","message":{"role":"assistant","content":"a","usage":{"input_tokens":100,"cache_read_input_tokens":400,"output_tokens":30}}}
{"type":"assistant","timestamp":"2026-04-11T12:00:05Z","message":{"role":"assistant","content":"b","usage":{"input_tokens":50,"cache_creation_input_tokens":200,"output_tokens":20}}}
EOF
  # The fixture is a KNOWN-GOOD faithful transcript, so ao MUST derive non-zero
  # from it. A skip is legitimate ONLY when ao cannot parse at all (non-zero exit
  # -> parser absent/too old); a "0 0" from a SUCCESSFUL parse is a producer
  # REGRESSION and must FAIL the test, not skip it.
  if EXP_PAIR="$("$AO" yield tokens --transcript "$WORK/session.jsonl" --pair 2>/dev/null)"; then
    read -r EXP_IN EXP_OUT <<< "$EXP_PAIR"
  else
    skip "ao yield tokens errored on the fixture (parser unavailable/too old)"
  fi
  [ "${EXP_IN:-0}" -gt 0 ]   # regression guard: a working parser MUST NOT derive 0

  # ── BRONZE -> SILVER: emit the usage event with REAL derived tokens ─────────
  ( cd "$WORK" && "$AO" yield emit usage --bead ag-e4 --run e4-run \
      --json "{\"tokens_in\":$EXP_IN,\"tokens_out\":$EXP_OUT,\"cost_usd\":0,\"wall_clock_s\":0,\"model\":\"m\",\"phase\":\"implement\"}" ) >/dev/null
  [ -f "$YIELD_LEDGER" ]
  run python3 -c "
import json
ti=to=None
for l in open('$YIELD_LEDGER'):
    l=l.strip()
    if not l: continue
    e=json.loads(l)
    if e.get('event')=='usage' and e.get('bead_id')=='ag-e4':
        b=e.get('body',{}) or {}; ti=b.get('tokens_in'); to=b.get('tokens_out')
print('%s %s' % (ti,to))
"
  [ "$output" = "$EXP_IN $EXP_OUT" ]   # silver carries the REAL footprint
  [ "$output" != "0 0" ]               # not the old hardcoded 0

  # ── SILVER -> PROVENANCE: a CONFIRMED verdict emits a verdict->commit edge ──
  HEAD_SHA="$(git -C "$WORK" rev-parse HEAD)"
  EV="$WORK/evidence.txt"; printf 'reviewer ran\n' > "$EV"
  # AO_BIN pins the trusted ao binary for the verdict->ledger edge emit. In CI
  # `ao` is NOT on PATH, so without this pin pawl-verdict.sh finds no trusted ao
  # and the edge never binds ("no trusted ao binary found"). $AO is the built
  # binary resolved in setup_file (test already skipped above if empty).
  ( cd "$WORK" && AO_BIN="$AO" bash "$REPO_ROOT/scripts/pawl-verdict.sh" write ag-e4 0 \
      --disposition CONFIRMED --head "$HEAD_SHA" \
      --author-context e4-author --author-family claude --mode fresh-context \
      --refuter codex:CONFIRMED:e4-refuter:"$EV" --dir "$WORK/verdicts" ) >/dev/null
  [ -f "$PROV_LEDGER" ]
  # the trunk-only PROV-O edge: a wasDerivedFrom verdict->commit for this bead
  grep -q '"relation":"wasDerivedFrom"' "$PROV_LEDGER"
  grep -q 'ag-e4' "$PROV_LEDGER"
  run python3 -c "
import json
ok=False
for l in open('$PROV_LEDGER'):
    l=l.strip()
    if not l: continue
    e=json.loads(l)
    if e.get('relation')=='wasDerivedFrom' and e.get('from_type')=='verdict' and e.get('to_type')=='commit' and 'ag-e4' in str(e.get('from_id','')):
        ok=True
print('edge' if ok else 'none')
"
  [ "$output" = "edge" ]   # provenance verdict->commit edge present
}

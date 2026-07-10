#!/usr/bin/env bash
# test-pawl-verdict-yield-emit.sh — age-uxva acceptance.
#
# Proves the pawl-verdict.sh `write` chokepoint emits a yield-ledger gate-verdict
# for every PANEL verdict (so catches are logged even with no merge attempt):
#   1. a REFUTED cross-family verdict → one gate-verdict event (REFUTED, cross_family=true)
#   2. emission is idempotent on re-run (no duplicate)
#   3. a CONFIRMED verdict also emits
#   4. family normalization: codex+gpt (same canonical family) → cross_family=false
#   5. the canonical checkout is not dirtied (ledger lives under gitignored .agents/)
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PAWL_VERDICT="$ROOT/scripts/pawl-verdict.sh"
# Prefer a freshly-built ao; fall back to PATH.
AO_BUILT="$ROOT/cli/bin/ao"

fails=0
check() { if eval "$2"; then echo "PASS: $1"; else echo "FAIL: $1"; fails=$((fails + 1)); fi; }

command -v jq >/dev/null 2>&1 || { echo "SKIP: jq not available"; exit 0; }
if [[ ! -x "$AO_BUILT" ]] && ! command -v ao >/dev/null 2>&1; then
  echo "SKIP: no ao binary (build cli/bin/ao first)"; exit 0
fi

TMP="$(mktemp -d)"
cleanup() { chmod -R u+w "$TMP" 2>/dev/null || true; /bin/rm -rf "$TMP" 2>/dev/null || true; }
trap cleanup EXIT

# Put the built ao on PATH (the script calls `ao` by name).
BIN="$TMP/bin"; mkdir -p "$BIN"
if [[ -x "$AO_BUILT" ]]; then ln -sf "$AO_BUILT" "$BIN/ao"; fi
export PATH="$BIN:$PATH"

# sandbox git repo
REPO="$TMP/repo"
mkdir -p "$REPO"; cd "$REPO" || exit 1
git init -q -b main; git config user.email t@t; git config user.name t
mkdir -p .agents/yield .agents/pawl-verdicts docs/provenance scripts
# Copy the script under test INTO the sandbox so its REPO_ROOT (SCRIPT_DIR/..)
# resolves to the sandbox, not the real repo (pawl-verdict.sh is root-relative
# to its own location, not cwd).
cp "$PAWL_VERDICT" scripts/pawl-verdict.sh; chmod +x scripts/pawl-verdict.sh
PAWL_VERDICT="$REPO/scripts/pawl-verdict.sh"
echo seed > seed.txt; git add -A; git commit -qm seed --no-verify
HEAD_SHA="$(git rev-parse HEAD)"
LEDGER="$REPO/.agents/yield/yield-ledger.jsonl"

# helper: count gate-verdict events for a bead+disposition
gv_count() { jq -c --arg b "$1" --arg d "$2" 'select(.event=="gate-verdict" and .bead_id==$b and .body.disposition==$d)' "$LEDGER" 2>/dev/null | grep -c . ; }

# 1) REFUTED cross-family verdict (claude + gpt), no merge
echo ev1 > "$TMP/ev1"; echo ev2 > "$TMP/ev2"
bash "$PAWL_VERDICT" write age-test-r 0 \
  --disposition REFUTED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "claude:REFUTED:ctx-claude:$TMP/ev1" \
  --refuter "gpt:REFUTED:ctx-gpt:$TMP/ev2" \
  --domain concurrency --reason "missed a fail-open" >/dev/null 2>&1 || true

check "REFUTED verdict emitted exactly one gate-verdict event" \
  "[ \"\$(gv_count age-test-r REFUTED)\" = '1' ]"
check "the event carries cross_family=true (2 distinct families)" \
  "jq -e 'select(.event==\"gate-verdict\" and .bead_id==\"age-test-r\") | .body.cross_family==true' '$LEDGER' >/dev/null"
check "refuter_families normalized to [claude,gpt]" \
  "[ \"\$(jq -r 'select(.event==\"gate-verdict\" and .bead_id==\"age-test-r\") | .body.refuter_families | sort | join(\",\")' '$LEDGER')\" = 'claude,gpt' ]"
check "what-was-missed reason recorded" \
  "jq -e 'select(.event==\"gate-verdict\" and .bead_id==\"age-test-r\") | .body.reason==\"missed a fail-open\"' '$LEDGER' >/dev/null"

# 2) idempotency: identical re-run adds NO duplicate
bash "$PAWL_VERDICT" write age-test-r 0 \
  --disposition REFUTED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "claude:REFUTED:ctx-claude:$TMP/ev1" \
  --refuter "gpt:REFUTED:ctx-gpt:$TMP/ev2" \
  --domain concurrency --reason "missed a fail-open" >/dev/null 2>&1 || true
check "re-run emits NO duplicate (still exactly one)" \
  "[ \"\$(gv_count age-test-r REFUTED)\" = '1' ]"

# 3) CONFIRMED verdict also emits
bash "$PAWL_VERDICT" write age-test-c 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
check "CONFIRMED verdict emitted a gate-verdict event" \
  "[ \"\$(gv_count age-test-c CONFIRMED)\" = '1' ]"

# 4) normalization: codex + gpt are the SAME canonical family → cross_family=false
bash "$PAWL_VERDICT" write age-test-norm 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "codex:CONFIRMED:ctx-codex:$TMP/ev1" \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev2" >/dev/null 2>&1 || true
check "codex+gpt collapse to one family → cross_family=false (not gamed)" \
  "jq -e 'select(.event==\"gate-verdict\" and .bead_id==\"age-test-norm\") | .body.cross_family==false' '$LEDGER' >/dev/null"

# 4b) JOIN PROOF (cross-family REFUTE was a false alarm): an `accept` whose
#     gate_verdict_ref.head_sha == the REVIEWED head admits against the chokepoint's
#     CONFIRMED gate-verdict at that same head. reconcile-pr's accept uses cur_head
#     (the reviewed head), NOT the merge/rebound head — so gauge A admits it (no loss).
bash "$PAWL_VERDICT" write age-join 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
( cd "$REPO" && ao yield emit accept --bead age-join --run age-join \
  --json "{\"merge_sha\":\"deadbeef1234567\",\"merged_by\":\"test\",\"gate_verdict_ref\":{\"bead_id\":\"age-join\",\"head_sha\":\"$HEAD_SHA\"}}" ) >/dev/null 2>&1 || true
check "accept@reviewed-head ADMITS against the chokepoint CONFIRMED (aligned default run: 0 unadmitted, A=1)" \
  "[ \"\$(cd '$REPO' && ao yield gauge --run age-join --json 2>/dev/null | jq -r '.gauges.unadmitted_accepts, .gauges.a_accepted | tostring' | paste -sd, -)\" = '0,1' ]"

# 4c) NEGATIVE (locks the run_id-mismatch REFUTE): an accept emitted under a
#     DIVERGENT run_id (the old reconcile-$bead default) does NOT admit against
#     the chokepoint CONFIRMED (which is under $bead) — same-run join only. This
#     is exactly the break the run_id alignment fix prevents.
bash "$PAWL_VERDICT" write age-divrun 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
( cd "$REPO" && ao yield emit accept --bead age-divrun --run "reconcile-age-divrun" \
  --json "{\"merge_sha\":\"deadbeef1234567\",\"merged_by\":\"test\",\"gate_verdict_ref\":{\"bead_id\":\"age-divrun\",\"head_sha\":\"$HEAD_SHA\"}}" ) >/dev/null 2>&1 || true
check "divergent run_id accept is UNADMITTED (proves alignment is load-bearing)" \
  "[ \"\$(cd '$REPO' && ao yield gauge --run reconcile-age-divrun --json 2>/dev/null | jq -r '.gauges.unadmitted_accepts')\" = '1' ]"

# 4d) dedup is RUN-SCOPED: the same bead/head/attempt/disposition/reason in TWO
#     different runs must BOTH emit (gauge joins per-run; a later run re-emitting
#     must not be suppressed by an earlier run's row), while a same-run re-run
#     stays deduped. Drives the run via AGENTOPS_RUN_ID (the symmetric override).
AGENTOPS_RUN_ID=run-r1 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
AGENTOPS_RUN_ID=run-r2 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
AGENTOPS_RUN_ID=run-r2 bash "$PAWL_VERDICT" write age-dedup 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" >/dev/null 2>&1 || true
check "cross-run NOT deduped, same-run IS deduped (exactly 2 events: r1 + r2)" \
  "[ \"\$(gv_count age-dedup CONFIRMED)\" = '2' ]"

# 4e) cwd-INDEPENDENCE: invoked from a repo SUBDIR, the emit still lands in the
#     canonical REPO_ROOT ledger (ao yield resolves by cwd, so the emit cd's to
#     REPO_ROOT) — NOT a subdir ledger. Otherwise a subdir-run accept and a
#     root-run gate-verdict split into different ledgers and never join.
mkdir -p "$REPO/sub/deep"
( cd "$REPO/sub/deep" && bash "$PAWL_VERDICT" write age-cwd 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$TMP/ev1" ) >/dev/null 2>&1 || true
check "subdir-invoked emit lands in the REPO_ROOT ledger (cwd-independent)" \
  "[ \"\$(gv_count age-cwd CONFIRMED)\" = '1' ]"
check "subdir-invoked emit did NOT create a stray subdir ledger" \
  "[ ! -e '$REPO/sub/deep/.agents/yield/yield-ledger.jsonl' ]"

# 6) age-ivoq METER: the write chokepoint records REAL reviewer usage. codex exec
#    prints a cumulative "tokens used" total into the evidence transcript; the
#    verdict cost must carry THAT number (estimated=false), not bytes/4 of a
#    57-byte summary — and a companion usage event (phase=review) must land in
#    the yield ledger with explicit tokens_source/cost_source, so the D17 ruler
#    reads data instead of 549/549 silent zeros.
usage_count() { jq -c --arg b "$1" 'select(.event=="usage" and .bead_id==$b)' "$LEDGER" 2>/dev/null | grep -c . ; }
EV_METERED="$TMP/ev-metered"
printf 'No blocking defects found.\n\nVERDICT: CONFIRMED\ntokens used\n17,068\n' > "$EV_METERED"
bash "$PAWL_VERDICT" write age-meter 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$EV_METERED" \
  --wall-seconds 42 >/dev/null 2>&1 || true
check "verdict cost carries the REAL codex total (tokens_est=17068, estimated=false)" \
  "jq -e '.cost.tokens_est==17068 and .cost.estimated==false and .cost.wall_seconds==42' '$REPO/.agents/pawl-verdicts/age-meter.json' >/dev/null"
check "usage event: measured tokens_total + explicit sources (never a silent zero)" \
  "jq -e 'select(.event==\"usage\" and .bead_id==\"age-meter\") | .body.tokens_total==17068 and .body.tokens_source==\"measured\" and .body.cost_source==\"unknown\" and .body.phase==\"review\" and .body.wall_clock_s==42 and .body.model==\"gpt\"' '$LEDGER' >/dev/null"

# 6b) no usage surface in the evidence → the HONEST estimate path: bytes/4,
#     flagged estimated=true on the verdict and tokens_source=estimated on the
#     usage event — an estimate never masquerades as a measurement.
EV_PLAIN="$TMP/ev-plain"
printf 'VERDICT: CONFIRMED — reviewed the full diff, no blocking defects found here.\n' > "$EV_PLAIN"
bash "$PAWL_VERDICT" write age-meter-est 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$EV_PLAIN" \
  --wall-seconds 7 >/dev/null 2>&1 || true
check "no tokens-used line → verdict cost stays a flagged estimate (estimated=true)" \
  "jq -e '.cost.estimated==true and .cost.tokens_est>0' '$REPO/.agents/pawl-verdicts/age-meter-est.json' >/dev/null"
check "usage event records the estimate as tokens_source=estimated" \
  "jq -e 'select(.event==\"usage\" and .bead_id==\"age-meter-est\") | .body.tokens_source==\"estimated\" and .body.tokens_total>0 and .body.cost_source==\"unknown\"' '$LEDGER' >/dev/null"

# 6c) usage emit is idempotent per run: a literal re-run adds NO duplicate spend
#     (double-counting would inflate R exactly like the gate-verdict dup would Q).
bash "$PAWL_VERDICT" write age-meter 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$EV_METERED" \
  --wall-seconds 42 >/dev/null 2>&1 || true
check "usage re-run emits NO duplicate (still exactly one for age-meter)" \
  "[ \"\$(usage_count age-meter)\" = '1' ]"

# 6d) legacy caller (no --wall-seconds): the verdict stays byte-identical (no
#     cost object), but the review's REAL tokens are still metered into the
#     yield ledger — usage availability doesn't depend on the wall-clock flag.
bash "$PAWL_VERDICT" write age-meter-nowall 0 \
  --disposition CONFIRMED --head "$HEAD_SHA" --author-context ctx-author \
  --refuter "gpt:CONFIRMED:ctx-gpt:$EV_METERED" >/dev/null 2>&1 || true
check "no --wall-seconds → verdict carries NO cost object (legacy contract intact)" \
  "jq -e 'has(\"cost\") | not' '$REPO/.agents/pawl-verdicts/age-meter-nowall.json' >/dev/null"
check "…but the usage event still carries the measured tokens (wall_clock_s=0)" \
  "jq -e 'select(.event==\"usage\" and .bead_id==\"age-meter-nowall\") | .body.tokens_total==17068 and .body.tokens_source==\"measured\" and .body.wall_clock_s==0' '$LEDGER' >/dev/null"

# 5) the yield log does NOT pollute the tracked tree — it lives under gitignored
#    .agents/. (The provenance ledger under docs/ is a separate, pre-existing
#    tracked artifact and is not this change's concern.)
printf '.agents/\n' > .gitignore; git add .gitignore; git commit -qm gitignore --no-verify
check "yield ledger is gitignored (the membrane log never dirties the tracked tree)" \
  "git -C '$REPO' check-ignore .agents/yield/yield-ledger.jsonl >/dev/null"
check "no yield artifact appears as a tracked/untracked-non-ignored change" \
  "[ -z \"\$(git -C '$REPO' status --porcelain .agents/)\" ]"

echo
if [[ "$fails" -eq 0 ]]; then echo "ALL PASS (21 checks)"; exit 0; fi
echo "$fails CHECK(S) FAILED"; exit 1

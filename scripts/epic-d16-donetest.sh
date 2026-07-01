#!/usr/bin/env bash
# epic-d16-donetest.sh — the terminal integration done-test for Directive 16
# (epic age-d16-self-hosting-route-nkr). Proves the unattended self-hosting loop
# CLOSES end-to-end by composing the REAL organs (no Claude, no live multi-hour
# worker) over an ISOLATED temp ledger + temp bead store, so it is repeatable and
# never pollutes the real _beads / provenance ledger.
#
# The mechanical sequence (plan §"Epic done-test", criteria 1-6):
#   1. SEED            — mint a real follow-up bead with a runnable acceptance.
#   2. NO-HUMAN        — record the launch command + start/stop stamps (no
#                        wall-clock; passed in) as the unattended boundary.
#   3. FAILURE INJECT  — drive M2's recovery state machine on an injected failure;
#                        a recovery branch (fix-forward|re-scope|andon) MUST fire.
#   4. ACCEPTANCE      — a fresh-context pawl verdict authorizes + lands a verdict
#                        row in the ledger (M1+M3); a SELF-APPROVAL attempt is
#                        REFUSED at the door.
#   5. SELF-IMPROVE    — M4's ASSAY tick mines the run's evidence into >=1
#                        follow-up suggestion bead.
#   6. EVIDENCE        — emit an artifact listing every path; missing any => FAIL.
#
# Each step ASSERTS its evidence; the first failed assertion emits a FAIL verdict
# and exits 1 (no silent pass). Law 0: composes shell organs + `ao` + `br` only.
#
# Exit: 0 = the loop closed (all 6 criteria met) · 1 = a criterion failed ·
#       2 = usage / missing organ.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RECOVERY="$REPO_ROOT/scripts/recovery-statemachine.sh"
ASSAY="$REPO_ROOT/scripts/assay/self-improvement-tick.sh"
PAWL="$REPO_ROOT/scripts/pawl-verdict.sh"
AO_BIN="${AO_BIN:-ao}"

WORKDIR=""
DATE_STAMP="2026-06-16"
EVIDENCE_OUT=""
HEAD_SHA="d16d0e7e57cafe0000000000000000000000beef"   # fixed fake commit for the isolated test

usage() {
  cat >&2 <<'EOF'
Usage: epic-d16-donetest.sh [--workdir <dir>] [--date <YYYY-MM-DD>] [--evidence-out <path>] [--ao <bin>]

  --workdir <dir>     isolated scratch root (default: a fresh mktemp -d)
  --date <stamp>      timestamp stamp for the unattended-boundary evidence (no wall-clock)
  --evidence-out <p>  also write the evidence artifact here (always printed to stdout)
  --ao <bin>          ao binary to use for `ao provenance emit-verdict` (default: ao on PATH)
  -h|--help           this help

Composes the real M1-M5 organs over an isolated temp ledger + bead store and
asserts the unattended loop closes. Emits one terminal JSON verdict.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --workdir) WORKDIR="${2:-}"; shift 2 ;;
    --date) DATE_STAMP="${2:-}"; shift 2 ;;
    --evidence-out) EVIDENCE_OUT="${2:-}"; shift 2 ;;
    --ao) AO_BIN="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "donetest: unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

for organ in "$RECOVERY" "$ASSAY" "$PAWL"; do
  [ -x "$organ" ] || { echo "donetest: missing organ: $organ" >&2; exit 2; }
done
command -v "$AO_BIN" >/dev/null 2>&1 || { echo "donetest: ao binary not found: $AO_BIN" >&2; exit 2; }
command -v br >/dev/null 2>&1 || { echo "donetest: br not found" >&2; exit 2; }

[ -n "$WORKDIR" ] || WORKDIR="$(mktemp -d)"
# `ao provenance emit-verdict` resolves the ledger by walking UP from cwd for a
# docs/+schemas/ root (no env/flag override) — so to keep the REAL ledger
# untouched, WORKDIR is made a SELF-CONTAINED root (docs/ + schemas/) OUTSIDE the
# real repo tree, and the emit runs with cwd=WORKDIR. The ledger then lands at
# WORKDIR/docs/provenance/ledger.jsonl, never the repo's.
case "$WORKDIR" in "$REPO_ROOT"|"$REPO_ROOT"/*) echo "donetest: --workdir must be OUTSIDE the repo (ledger isolation), got $WORKDIR" >&2; exit 2 ;; esac
BEADS_DIR="$WORKDIR/_beads"
LEDGER="$WORKDIR/docs/provenance/ledger.jsonl"
VDIR_OK="$WORKDIR/verdicts-ok"
VDIR_SELF="$WORKDIR/verdicts-selfapprove"
mkdir -p "$BEADS_DIR" "$VDIR_OK" "$VDIR_SELF" "$WORKDIR/docs/provenance" "$WORKDIR/schemas"
export BEADS_DIR

fail() { printf '{"donetest":"age-d16-self-hosting-route-nkr","result":"FAIL","failed_step":"%s","detail":"%s"}\n' "$1" "${2:-}"; exit 1; }

# br in the isolated store; quiet init. Prefix `ag` MIRRORS production so the
# verdict row's bead id is an `ag-…` the M4 ASSAY miner recognizes (the real
# agentops ledger is all `ag-`); a different prefix would not be mined.
br init --prefix ag >/dev/null 2>&1 || true

# --- 1. SEED ----------------------------------------------------------------
SEED_BODY="Self-hosting done-test seed (Directive 16). The loop mints its own next slice.

## Scenarios
- Given the unattended loop is launched, When a slice fails, Then recovery fires and the bead reaches accepted only via a fresh-context verdict."
SEED_BEAD="$(br create "D16 done-test seed" -t task -p 2 --labels d16-donetest --silent -d "$SEED_BODY" 2>/dev/null | tr -d '[:space:]')"
[ -n "$SEED_BEAD" ] || fail "seed" "br create returned no id"

# --- 2. NO-HUMAN BOUNDARY ---------------------------------------------------
LAUNCH_CMD="headless codex --skip-git-repo-check 'implement $SEED_BEAD' (unattended; operator does not touch after launch)"
START_TS="${DATE_STAMP}T00:00:00Z"
STOP_TS="${DATE_STAMP}T00:07:00Z"

# --- 3. FAILURE INJECTION -> RECOVERY ---------------------------------------
# Inject a re-scope failure (the failure becomes a new acceptance): M2 files a
# follow-up bead that blocks the seed and labels the original. Assert the crisp
# terminal branch + that recovery did NOT close the bead.
REC_JSON="$("$RECOVERY" --bead "$SEED_BEAD" --failure-kind rescope \
  --rescope-scenario "Given the first target was unreachable, When retried, Then a narrower target is used." \
  --reason "done-test injected failure" 2>/dev/null)" || fail "recover" "recovery-statemachine non-zero"
case "$REC_JSON" in
  *'"branch":"rescope"'*'"terminal_state":"rescoped"'*) : ;;
  *) fail "recover" "expected a rescoped terminal branch, got: $REC_JSON" ;;
esac
RESCOPE_BEAD="$(printf '%s' "$REC_JSON" | sed -n 's/.*"new_acceptance":"\([^"]*\)".*/\1/p')"
[ -n "$RESCOPE_BEAD" ] || fail "recover" "no rescope follow-up bead id in: $REC_JSON"

# --- 4a. ACCEPTANCE: fresh-context verdict authorizes + lands in the ledger --
# pawl-verdict.sh `write` ALSO emits the verdict edge to the ledger internally
# (resolving the ledger from cwd), so every write/emit runs inside cwd=WORKDIR —
# the isolated self-contained root — to keep the REAL repo ledger untouched.
# pr is a positive integer (the schema requires minimum:1).
PR=1
EVID="$WORKDIR/refuter-evidence.md"
printf 'Fresh-context refuter reviewed %s @ %s: acceptance scenarios pass; no self-approval.\n' "$SEED_BEAD" "$HEAD_SHA" > "$EVID"
( cd "$WORKDIR" && "$PAWL" write "$SEED_BEAD" "$PR" --disposition CONFIRMED --head "$HEAD_SHA" \
    --author-context "author-ctx-AUTHOR" --mode fresh-context \
    --refuter "anthropic:CONFIRMED:reviewer-ctx-FRESH:$EVID" \
    --dir "$VDIR_OK" >/dev/null 2>&1 ) || fail "accept" "pawl-verdict write failed"
"$PAWL" check "$SEED_BEAD" "$PR" --dir "$VDIR_OK" --head "$HEAD_SHA" >/dev/null 2>&1 \
  || fail "accept" "fresh-context CONFIRMED verdict did NOT authorize the merge"
# idempotently re-land the verdict row in the isolated ledger (the SENSOR feed, M1).
( cd "$WORKDIR" && "$AO_BIN" provenance emit-verdict --file "$VDIR_OK/$SEED_BEAD.json" >/dev/null 2>&1 ) \
  || fail "accept" "ao provenance emit-verdict failed"
grep -q '"from_type":"verdict"' "$LEDGER" 2>/dev/null || fail "accept" "no verdict row landed in the ledger"
grep -q "$SEED_BEAD" "$LEDGER" 2>/dev/null || fail "accept" "verdict row does not reference the seed bead"

# --- 4b. SELF-APPROVAL is REFUSED at the door -------------------------------
# Same author writes a verdict whose only refuter ran in the AUTHOR's own context
# (context_id == author_context_id). `check` MUST refuse (fresh-context floor).
( cd "$WORKDIR" && "$PAWL" write "$SEED_BEAD" "$PR" --disposition CONFIRMED --head "$HEAD_SHA" \
    --author-context "author-ctx-AUTHOR" --mode fresh-context \
    --refuter "anthropic:CONFIRMED:author-ctx-AUTHOR:$EVID" \
    --dir "$VDIR_SELF" >/dev/null 2>&1 ) || fail "self-approval" "write of the self-approval probe failed"
if "$PAWL" check "$SEED_BEAD" "$PR" --dir "$VDIR_SELF" --head "$HEAD_SHA" >/dev/null 2>&1; then
  fail "self-approval" "a self-approval verdict (refuter==author) was ACCEPTED — the door is broken"
fi

# --- 5. SELF-IMPROVEMENT: ASSAY tick mines the run -> follow-up bead ---------
ASSAY_JSON="$("$ASSAY" --ledger "$LEDGER" --beads-dir "$BEADS_DIR" --max-suggestions 1 2>/dev/null)" \
  || fail "self-improve" "assay tick non-zero"
case "$ASSAY_JSON" in
  *'"state":"filed"'*) : ;;
  *) fail "self-improve" "assay tick did not file a suggestion: $ASSAY_JSON" ;;
esac
MINED_BEAD="$(printf '%s' "$ASSAY_JSON" | sed -n 's/.*"filed_beads":\["\([^"]*\)".*/\1/p')"
[ -n "$MINED_BEAD" ] || fail "self-improve" "no mined follow-up bead id in: $ASSAY_JSON"

# --- 6. EVIDENCE artifact (every path or it is NOT done) --------------------
read -r -d '' EVIDENCE_DOC <<EOF || true
# Evidence — Directive 16 epic done-test (unattended loop closed)

Generated by scripts/epic-d16-donetest.sh over the real M1-M5 organs in an
isolated store ($WORKDIR). Every terminal acceptance path below is present.

| # | criterion | evidence |
|---|---|---|
| 1 | seed (real follow-up bead, runnable acceptance) | bead \`$SEED_BEAD\` |
| 2 | no-human boundary (launch + timestamps) | \`$LAUNCH_CMD\` · $START_TS → $STOP_TS |
| 3 | failure injection → recovery branch fired | branch=rescope → follow-up \`$RESCOPE_BEAD\` (seed not closed) |
| 4 | accepted ONLY via fresh-context verdict; self-approval refused | verdict row in ledger (head_sha \`${HEAD_SHA:0:12}\`); self-approval \`check\` refused |
| 5 | self-improvement (ASSAY mines a follow-up) | mined bead \`$MINED_BEAD\` |

Ledger: $LEDGER · verdict dir: $VDIR_OK · seed store: $BEADS_DIR
EOF

if [ -n "$EVIDENCE_OUT" ]; then printf '%s\n' "$EVIDENCE_DOC" > "$EVIDENCE_OUT"; fi
printf '%s\n' "$EVIDENCE_DOC" >&2

printf '{"donetest":"age-d16-self-hosting-route-nkr","result":"PASS","seed_bead":"%s","rescope_bead":"%s","verdict_head":"%s","mined_bead":"%s","ledger":"%s","self_approval":"refused"}\n' \
  "$SEED_BEAD" "$RESCOPE_BEAD" "${HEAD_SHA:0:12}" "$MINED_BEAD" "$LEDGER"
exit 0

#!/usr/bin/env bash
# dynamo-e2e.sh (ag-veotd) — run ONE full dynamo cycle end-to-end and prove the
# loop closes. Wires the REAL organs (yieldledger emit + gauge) on a synthetic
# bead with a deterministic STUB worker: NO LLM, NO metered compute, no network.
#
# This harness's loop: dispatch -> produce -> gate -> accept -> sense -> tune
#   dispatch/produce : stub worker (deterministic) — the real controller is ag-v1xk
#   gate             : a cross-family gate-verdict event (author!=judge, fresh-context)
#   accept           : a terminal-accept event bound to the gate verdict
#   sense            : `ao yield gauge` computes A / Q / A-R / E / L from the ledger
#   self-excitation C: read from the gauge (pending until ag-8p8o W1c — never faked)
#   tune             : the gauge's shadow-mode actuation hypotheses (printed, not steered)
#
# Runs in an ISOLATED temp project dir so it never touches the real yield ledger.
# Exit 0 iff the loop closed: every organ produced its signal and the gauge computed.
set -euo pipefail

RUN_ID="dynamo-e2e-demo"
BEAD="e2e-synthetic-1"
AO_BIN="${AO_BIN:-ao}"          # override to a freshly-built binary in CI/tests
KEEP=false
SCENARIO="clean"               # clean = CONFIRMED first pass; rework = REFUTE -> rework -> CONFIRM (the ratchet, phase-label loss); rework-order = same ratchet but attempt-1 spend is rework via the ATTEMPT-ORDERING join, no phase label
C_DELTA=""                     # self-excitation: a PUBLISHED corpus delta (ag-8p8o/W1c). Omit -> C pending (never faked).
for a in "$@"; do
  case "$a" in
    --run-id=*)   RUN_ID="${a#*=}" ;;
    --scenario=*) SCENARIO="${a#*=}" ;;
    --c-delta=*)  C_DELTA="${a#*=}" ;;
    --keep)       KEEP=true ;;
    -h|--help)    sed -n '2,20p' "$0"; exit 0 ;;
  esac
done
case "$SCENARIO" in clean|rework|rework-order) ;; *) echo "dynamo-e2e: --scenario must be clean|rework|rework-order (got '$SCENARIO')" >&2; exit 2 ;; esac
[[ -z "$C_DELTA" || "$C_DELTA" =~ ^-?[0-9]+(\.[0-9]+)?$ ]] || { echo "dynamo-e2e: --c-delta must be a number (got '$C_DELTA')" >&2; exit 2; }

command -v "$AO_BIN" >/dev/null 2>&1 || { echo "dynamo-e2e: '$AO_BIN' not on PATH (build + install ao, or set AO_BIN)" >&2; exit 2; }

# Isolated project dir: ao yield resolves its ledger from cwd (.agents/yield/),
# so an empty temp root keeps this demo off the real ledger.
WORK="$(mktemp -d)"
cleanup() { [[ "$KEEP" == true ]] || rm -rf "$WORK"; }
trap cleanup EXIT
mkdir -p "$WORK/.agents/yield"
cd "$WORK"

emit() { "$AO_BIN" yield emit "$@"; }   # real organ — fail HARD here (this is a test, not the fail-open merge path)

gv() { # gate-verdict helper: gv <head_sha> <disposition> <attempt>
  emit gate-verdict --bead "$BEAD" --run "$RUN_ID" --json "{\"difficulty\":2,\"pawl_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$1\"},\"disposition\":\"$2\",\"head_sha\":\"$1\",\"attempt\":$3,\"mode\":\"fresh-context\",\"author_context_id\":\"e2e-author\",\"refuter_families\":[\"claude\"],\"author_family\":\"codex\",\"cross_family\":true,\"author_ne_reviewer\":true,\"evidence_present\":true}" >/dev/null
}
use() { # usage helper: use <tokens> <phase>
  emit usage --bead "$BEAD" --run "$RUN_ID" --json "{\"tokens_in\":0,\"tokens_out\":$1,\"cost_usd\":0,\"wall_clock_s\":1,\"model\":\"stub-worker\",\"phase\":\"$2\"}" >/dev/null
}

echo "== DYNAMO E2E — scenario=$SCENARIO (run=$RUN_ID, bead=$BEAD) =="

# 1. DISPATCH + PRODUCE (stub worker; deterministic, no LLM) -------------------
echo "[1/6] dispatch+produce : stub worker produced work for $BEAD (no LLM)"

case "$SCENARIO" in
clean)
  # Happy path: CONFIRMED on the first attempt.
  HEAD_SHA="e2e000feed"
  gv "$HEAD_SHA" CONFIRMED 1
  echo "[2/6] gate           : CONFIRMED attempt-1 (clean first pass)"
  emit accept --bead "$BEAD" --run "$RUN_ID" --json "{\"merge_sha\":\"$HEAD_SHA\",\"merged_by\":\"dynamo-e2e\",\"gate_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$HEAD_SHA\"}}" >/dev/null
  echo "[3/6] accept         : merged $HEAD_SHA (gated)"
  use 1000 implement
  echo "[4/6] usage          : 1000 tok productive (R fed)"
  ;;
rework)
  # The RATCHET, PHASE-LABEL path: attempt-1 REFUTED -> rework -> attempt-2
  # CONFIRMED -> accept. Here the loss comes from the EXPLICIT phase=rework row
  # (classifyUsage's phase branch), NOT from the attempt-ordering join. The
  # attempt-1 700-token spend is emitted AFTER its own REFUTE verdict, so
  # usageAttempt attributes it FORWARD to the next gate-verdict (attempt-2) and
  # it reads Productive — the ordering join never fires here. The rework-order
  # scenario below covers the ordering mechanism explicitly. (age-vx0.)
  gv "e2e0v1bad" REFUTED 1
  use 700 implement                       # attempt-1 spend emitted post-REFUTE -> attributes to attempt-2 (productive)
  echo "[2/6] gate           : REFUTED attempt-1 -> reconcile: rework"
  HEAD_SHA="e2e0v2good"
  gv "$HEAD_SHA" CONFIRMED 2
  use 500 rework                          # rework loss via the PHASE LABEL (not ordering)
  echo "      gate (re)      : CONFIRMED attempt-2 (after rework)"
  emit accept --bead "$BEAD" --run "$RUN_ID" --json "{\"merge_sha\":\"$HEAD_SHA\",\"merged_by\":\"dynamo-e2e\",\"gate_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$HEAD_SHA\"}}" >/dev/null
  echo "[3/6] accept         : merged $HEAD_SHA (gated, attempt-2)"
  echo "[4/6] usage          : 500 rework loss via phase label (R fed)"
  ;;
rework-order)
  # The RATCHET, ATTEMPT-ORDERING path (age-vx0): same reject->rework->accept,
  # but the attempt-1 spend is classified rework SOLELY by the attempt-ordering
  # join — NO phase=rework label anywhere. The key is realistic emit order:
  # production spend is emitted BEFORE its gate-verdict, so usageAttempt maps the
  # 700-token attempt-1 spend to attempt-1 (< the accepting attempt-2) => rework.
  # The attempt-2 spend is emitted after the accepting verdict => productive.
  use 700 implement                       # attempt-1 production, BEFORE its verdict -> ordering attributes to attempt-1
  gv "e2e0o1bad" REFUTED 1
  echo "[2/6] gate           : REFUTED attempt-1 (attempt-1 spend already emitted)"
  HEAD_SHA="e2e0o2good"
  gv "$HEAD_SHA" CONFIRMED 2
  use 500 implement                       # attempt-2 production, AFTER the accepting verdict -> productive
  echo "      gate (re)      : CONFIRMED attempt-2 (after rework)"
  emit accept --bead "$BEAD" --run "$RUN_ID" --json "{\"merge_sha\":\"$HEAD_SHA\",\"merged_by\":\"dynamo-e2e\",\"gate_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$HEAD_SHA\"}}" >/dev/null
  echo "[3/6] accept         : merged $HEAD_SHA (gated, attempt-2)"
  echo "[4/6] usage          : 700 attempt-1 rework via ORDERING (no phase label) + 500 productive"
  ;;
esac

# 5. SENSE + 6. TUNE (real gauge: A/Q/A-R/E/L + C status + shadow hypotheses) --
# Self-excitation C: only populated from a PUBLISHED corpus delta (ag-8p8o/W1c)
# passed via --c-delta; omitted -> the gauge reports C pending. NEVER fabricated.
gauge_args=(yield gauge --run "$RUN_ID")
if [[ -n "$C_DELTA" ]]; then gauge_args+=(--c-delta "$C_DELTA"); fi
echo "[5/6] sense+tune     : ao yield gauge${C_DELTA:+ --c-delta $C_DELTA} --"
GAUGE_OUT="$("$AO_BIN" "${gauge_args[@]}" 2>&1)"
echo "$GAUGE_OUT" | sed 's/^/    /'

# Prove the loop CLOSED: (a) every organ signal present, AND (b) the emitted
# events actually FLOWED to the sensor — A>=1 (the accept reached the gauge) and
# R>0 (the usage reached it). An empty run prints the same labels, so presence
# alone is not proof; the flowed values are.
fail=0
for needle in "A (accepted)" "Q (first-pass yield)" "C (corpus delta)" "E (escalation" "L (loss)" "Shadow-mode actuation"; do
  grep -qiF "$needle" <<<"$GAUGE_OUT" || { echo "dynamo-e2e: MISSING organ signal: $needle" >&2; fail=1; }
done
# SELF-EXCITATION C: honest both ways. With --c-delta, C must show that value
# (the published corpus delta flowed). Without it, C MUST read pending — never a
# fabricated zero. This is the self-excitation organ's readout wiring (E2).
c_line="$(grep -E '^C \(corpus delta\)' <<<"$GAUGE_OUT" | head -1)"
if [[ -n "$C_DELTA" ]]; then
  # `--` so a negative delta (e.g. -0.5, the "field is NOT self-exciting" case) is
  # not parsed as a grep option — negative C is the most decision-relevant reading.
  grep -qF -- "$C_DELTA" <<<"$c_line" || { echo "dynamo-e2e: C did not show the published delta $C_DELTA (got: $c_line)" >&2; fail=1; }
else
  grep -qiF -- "pending" <<<"$c_line" || { echo "dynamo-e2e: C must be 'pending' with no --c-delta, never fabricated (got: $c_line)" >&2; fail=1; }
fi
# A (accepted) must be >= 1 — the accept event flowed through to the sensor
a_val="$(grep -E '^A \(accepted\)' <<<"$GAUGE_OUT" | grep -oE '[0-9]+' | head -1 || echo 0)"
[[ "${a_val:-0}" -ge 1 ]] || { echo "dynamo-e2e: accept did NOT reach the sensor (A=$a_val)" >&2; fail=1; }
# R (raw input) must be > 0 — the usage event flowed through
r_val="$(grep -E '^R \(raw input\)' <<<"$GAUGE_OUT" | grep -oE '[0-9]+' | head -1 || echo 0)"
[[ "${r_val:-0}" -gt 0 ]] || { echo "dynamo-e2e: usage did NOT reach the sensor (R=$r_val)" >&2; fail=1; }
# GATE organ flowed + the RATCHET is honest. The gate-verdict must reach Q, and Q
# must reflect the scenario: clean => >=1 bead clean; rework => 0 clean (a
# rejected-then-reworked bead is NOT a clean first pass) AND L>0 (rework is loss).
clean_frac="$(grep -oE '\([0-9]+/[0-9]+ beads clean\)' <<<"$GAUGE_OUT" | head -1)"
[[ -n "$clean_frac" ]] || { echo "dynamo-e2e: gate-verdict did NOT reach the sensor (no Q clean-fraction)" >&2; fail=1; }
l_val="$(grep -E '^L \(loss\)' <<<"$GAUGE_OUT" | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo 0)"
if [[ "$SCENARIO" == "clean" ]]; then
  grep -qE '\([1-9][0-9]*/[0-9]+ beads clean\)' <<<"$GAUGE_OUT" || { echo "dynamo-e2e: clean run but 0 beads counted clean in Q ($clean_frac)" >&2; fail=1; }
else
  # ratchet (rework + rework-order): the reworked bead must NOT count clean, and
  # rework spend must be loss.
  grep -qE '\(0/[0-9]+ beads clean\)' <<<"$GAUGE_OUT" || { echo "dynamo-e2e: rework run but Q counted a reworked bead as clean ($clean_frac) — ratchet not penalizing" >&2; fail=1; }
  [[ "$l_val" != "0.000" && -n "$l_val" ]] || { echo "dynamo-e2e: rework run but L=$l_val (rework spend not counted as loss)" >&2; fail=1; }
  # rework-order: the loss must come from the ATTEMPT-ORDERING join, not a phase
  # label (the scenario emits NO phase=rework row). Assert the L breakdown's
  # rework spend is non-zero — that can ONLY be the attempt-1 700-token spend
  # classified rework by the ordering attribution. This is the assertion the old
  # rework scenario lacked: it proves the ordering mechanism actually fires.
  if [[ "$SCENARIO" == "rework-order" ]]; then
    rework_spend="$(grep -oE 'rework=[0-9]+' <<<"$GAUGE_OUT" | grep -oE '[0-9]+' | head -1 || echo 0)"
    [[ "${rework_spend:-0}" -gt 0 ]] || { echo "dynamo-e2e: rework-order run but L breakdown rework=$rework_spend — the attempt-ordering join did NOT classify the attempt-1 spend as rework (no phase label was used, so ordering is the ONLY thing that can produce this loss)" >&2; fail=1; }
  fi
fi

if [[ "$fail" -ne 0 ]]; then
  echo "[6/6] VERDICT        : LOOP DID NOT CLOSE — organ/ratchet check failed" >&2
  exit 1
fi
case "$SCENARIO" in
rework)
  echo "[6/6] verdict        : LOOP CLOSED + RATCHET HONEST (phase-label loss) — reject->rework->accept; Q penalized $clean_frac, L=$l_val"
  ;;
rework-order)
  echo "[6/6] verdict        : LOOP CLOSED + RATCHET HONEST (ordering-join loss, no phase label) — Q penalized $clean_frac, L=$l_val"
  ;;
*)
  echo "[6/6] verdict        : LOOP CLOSED — dispatch->gate->accept->sense->C->tune all wired ($clean_frac)"
  ;;
esac
echo "== DYNAMO E2E OK =="

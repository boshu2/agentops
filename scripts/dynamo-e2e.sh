#!/usr/bin/env bash
# dynamo-e2e.sh (ag-veotd) — run ONE full dynamo cycle end-to-end and prove the
# loop closes. Wires the REAL organs (yieldledger emit + gauge) on a synthetic
# bead with a deterministic STUB worker: NO LLM, NO metered compute, no network.
#
# The loop (per SYSTEM.md): dispatch -> produce -> gate -> accept -> sense -> tune
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
for a in "$@"; do
  case "$a" in
    --run-id=*) RUN_ID="${a#*=}" ;;
    --keep)     KEEP=true ;;
    -h|--help)  sed -n '2,20p' "$0"; exit 0 ;;
  esac
done

command -v "$AO_BIN" >/dev/null 2>&1 || { echo "dynamo-e2e: '$AO_BIN' not on PATH (build + install ao, or set AO_BIN)" >&2; exit 2; }

# Isolated project dir: ao yield resolves its ledger from cwd (.agents/yield/),
# so an empty temp root keeps this demo off the real ledger.
WORK="$(mktemp -d)"
cleanup() { [[ "$KEEP" == true ]] || rm -rf "$WORK"; }
trap cleanup EXIT
mkdir -p "$WORK/.agents/yield"
cd "$WORK"

emit() { "$AO_BIN" yield emit "$@"; }   # real organ — fail HARD here (this is a test, not the fail-open merge path)

echo "== DYNAMO E2E — one full cycle (run=$RUN_ID, bead=$BEAD) =="

# 1. DISPATCH + PRODUCE (stub worker; deterministic, no LLM) -------------------
echo "[1/6] dispatch+produce : stub worker produced work for $BEAD (no LLM)"
HEAD_SHA="e2e000feed"   # synthetic commit the gate reviews + the accept binds to

# 2. GATE (cross-family verdict: author != judge, fresh-context, CONFIRMED) ----
emit gate-verdict --bead "$BEAD" --run "$RUN_ID" --json "{\"difficulty\":2,\"pawl_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$HEAD_SHA\"},\"disposition\":\"CONFIRMED\",\"head_sha\":\"$HEAD_SHA\",\"attempt\":1,\"mode\":\"fresh-context\",\"author_context_id\":\"e2e-author\",\"refuter_families\":[\"claude\"],\"author_family\":\"codex\",\"cross_family\":true,\"author_ne_reviewer\":true,\"evidence_present\":true}" >/dev/null
echo "[2/6] gate           : CONFIRMED (fresh-context, author!=judge)"

# 3. ACCEPT (terminal accept bound to the gate verdict) -----------------------
emit accept --bead "$BEAD" --run "$RUN_ID" --json "{\"merge_sha\":\"$HEAD_SHA\",\"merged_by\":\"dynamo-e2e\",\"gate_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$HEAD_SHA\"}}" >/dev/null
echo "[3/6] accept         : merged $HEAD_SHA (gated)"

# 4. USAGE (per-bead spend — the R denominator) -------------------------------
emit usage --bead "$BEAD" --run "$RUN_ID" --json "{\"tokens_in\":0,\"tokens_out\":1000,\"cost_usd\":0,\"wall_clock_s\":1,\"model\":\"stub-worker\",\"phase\":\"implement\"}" >/dev/null
echo "[4/6] usage          : recorded (R fed)"

# 5. SENSE + 6. TUNE (real gauge: A/Q/A-R/E/L + C status + shadow hypotheses) --
echo "[5/6] sense+tune     : ao yield gauge --"
GAUGE_OUT="$("$AO_BIN" yield gauge --run "$RUN_ID" 2>&1)"
echo "$GAUGE_OUT" | sed 's/^/    /'

# Prove the loop CLOSED: (a) every organ signal present, AND (b) the emitted
# events actually FLOWED to the sensor — A>=1 (the accept reached the gauge) and
# R>0 (the usage reached it). An empty run prints the same labels, so presence
# alone is not proof; the flowed values are.
fail=0
for needle in "A (accepted)" "Q (first-pass yield)" "C (corpus delta)" "E (escalation" "L (loss)" "Shadow-mode actuation"; do
  grep -qiF "$needle" <<<"$GAUGE_OUT" || { echo "dynamo-e2e: MISSING organ signal: $needle" >&2; fail=1; }
done
# A (accepted) must be >= 1 — the accept event flowed through to the sensor
a_val="$(grep -E '^A \(accepted\)' <<<"$GAUGE_OUT" | grep -oE '[0-9]+' | head -1 || echo 0)"
[[ "${a_val:-0}" -ge 1 ]] || { echo "dynamo-e2e: accept did NOT reach the sensor (A=$a_val)" >&2; fail=1; }
# R (raw input) must be > 0 — the usage event flowed through
r_val="$(grep -E '^R \(raw input\)' <<<"$GAUGE_OUT" | grep -oE '[0-9]+' | head -1 || echo 0)"
[[ "${r_val:-0}" -gt 0 ]] || { echo "dynamo-e2e: usage did NOT reach the sensor (R=$r_val)" >&2; fail=1; }
# GATE organ flowed: the CONFIRMED attempt-1 gate-verdict must be counted clean in
# Q (>=1 bead clean). Without this, A>=1 only proves accept; this proves the gate
# verdict reached the sensor and was scored (fresh-context reviewer's note).
grep -qE '\([1-9][0-9]*/[0-9]+ beads clean\)' <<<"$GAUGE_OUT" || { echo "dynamo-e2e: gate-verdict did NOT reach the sensor (0 beads counted clean in Q)" >&2; fail=1; }

if [[ "$fail" -ne 0 ]]; then
  echo "[6/6] VERDICT        : LOOP DID NOT CLOSE — missing organ signal(s)" >&2
  exit 1
fi
echo "[6/6] verdict        : LOOP CLOSED — dispatch->gate->accept->sense->C->tune all wired"
echo "== DYNAMO E2E OK =="

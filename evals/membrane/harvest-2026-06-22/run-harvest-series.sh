#!/usr/bin/env bash
# run-harvest-series.sh <producer-label> <mlx-endpoint> <mlx-model> <series-file> [run-tag]
#
# ONE harvest run = ONE escape_rate observation appended to a growing series, so
# E5 (the SPC governor) gets a real TIME-SERIES instead of an n=1 snapshot. SPC is
# meaningless on a single sample; this is the volume builder.
#
# Chains the proven pieces: scripts/eval-membrane.sh (weak producer × codex
# frontier cross-family membrane × deterministic oracle) -> harvest-to-ledger.sh
# (isolated ledger) -> appends one JSONL row to <series-file>:
#   {run_id, ts, producer, n_tasks, degraded, n_false_dones, n_caught, n_missed,
#    membrane_miss_rate, catch_rate}
# membrane_miss_rate = n_missed / n_false_dones — the rate E5 governs (how often a
# false-done slips past the membrane). catch_rate = n_caught / n_false_dones.
#
# Fail-closed on the row's INPUTS: if eval-membrane fails, the scorecard is
# missing, the row build fails, or the append fails, the run aborts WITHOUT
# appending (a partial/unmeasured run must never enter the series). The isolated
# yield-ledger emission (harvest-to-ledger.sh) is a SEPARATE best-effort side
# artifact — its failure is logged but does NOT block the row, because the series
# metric is read from the scorecard, not the ledger. So a row is only ever the
# real, complete scorecard measurement; it is never fabricated from a failed stage.
set -uo pipefail

LABEL="${1:?producer-label}"; ENDPOINT="${2:?mlx-endpoint}"; MODEL="${3:?mlx-model}"
SERIES="${4:?series-file}"; TAG="${5:-r}"
# Absolutize SERIES against the CALLER's cwd BEFORE the cd below — otherwise a
# relative <series-file> would resolve against the repo root and silently write
# the wrong file.
case "$SERIES" in /*) ;; *) SERIES="$(pwd)/$SERIES" ;; esac
HARVEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HARVEST_DIR/../../.." && pwd)"
# Export the ao binary path so the child harvest-to-ledger.sh resolves the same one.
export AGENTOPS_AO_BIN="${AGENTOPS_AO_BIN:-/tmp/ao-pawl}"
cd "$REPO_ROOT" || { echo "run-harvest-series: cannot cd repo root" >&2; exit 1; }

# SCOPE: this is a SEQUENTIAL volume runner — the batch driver invokes it one run
# at a time (a for-loop). It is not designed for concurrent invocation against the
# same series file (no flock on the append); running two at once against one series
# is out of scope. The run id below is still made collision-proof regardless, by
# folding in the mktemp-unique WORK basename (so even concurrent runs differ).
WORK="$(mktemp -d "${TMPDIR:-/tmp}/harvest-series.XXXXXX")" || exit 1
n_prior="$( [ -f "$SERIES" ] && wc -l < "$SERIES" || echo 0 )"; n_prior="$(printf '%s' "$n_prior" | tr -d ' ')"
# Unique run id without Date.now/rand: tag + hash of (label,model,size,unique-workdir).
RUN_ID="harvest-${TAG}-$(printf '%s' "${LABEL}-${MODEL}-${n_prior}-$(basename "$WORK")" | shasum | cut -c1-8)"
SC="$WORK/scorecard.json"
LROOT="$WORK/ledger"

echo "run-harvest-series: run=$RUN_ID producer=$LABEL model=$MODEL" >&2

PRODUCER_CMD="MLX_ENDPOINT=$ENDPOINT MLX_MODEL=$MODEL bash evals/membrane/producers/local-mlx-producer.sh \"\$1\" \"\$2\" \"\$3\""

# Membrane choice (HARVEST_MEMBRANE=codex|local, default codex):
#   codex — frontier cross-family reviewer, wrapped in a hard timeout (eval-membrane
#           puts NONE on the membrane call, so a stalled `codex exec` — seen live, a
#           22-min hang — would freeze the run; gtimeout kills it → degraded → continue).
#   local — the validated local-MLX membrane (docs/evals/local-membrane-vs-codex-2026-06-23.md:
#           9/9 trap-corpus concordance with codex, no API cost, no stall tax). The OPERATOR
#           owns keeping HARVEST_MEMBRANE_MODEL in a DIFFERENT family than the producer.
# For measuring CODEX's (the production gate's) miss rate, use codex; for cheap reliable
# eval volume when that distinction doesn't matter, use local.
TO_BIN="$(command -v gtimeout || command -v timeout || true)"
# Fail-closed on the membrane choice: an UNKNOWN value (typo like 'locl') must NOT
# silently fall through to codex — that would run the wrong reviewer (API cost /
# stall) and mislabel the auditable row. Accept ONLY codex|local; else abort.
case "${HARVEST_MEMBRANE:-codex}" in
  local)
    MEMBRANE_LABEL="local-mlx"
    # EXPORT the tunables (quoted assignment — safe for spaces/metacharacters) so the
    # wrapper reads them from the environment. Do NOT interpolate values into the
    # command string (that would word-split / shell-interpret them). The producer's
    # own inline `MLX_ENDPOINT=…` overrides these for the producer command, so the
    # membrane gets the membrane endpoint/model and the producer gets its own.
    export MLX_ENDPOINT="${HARVEST_MEMBRANE_ENDPOINT:-http://127.0.0.1:8100/v1/chat/completions}"
    export MLX_MODEL="${HARVEST_MEMBRANE_MODEL:-mlx-community/Qwen2.5-Coder-32B-Instruct-4bit}"
    # shellcheck disable=SC2016  # the "$1" is intentionally literal — eval-membrane expands it via `bash -c "$MEMBRANE_CMD" _ "$prompt"`.
    MEMBRANE_CMD='bash evals/membrane/membranes/local-mlx-membrane.sh "$1"'
    echo "run-harvest-series: membrane=LOCAL ($MLX_MODEL) — validated concordant w/ codex on the trap corpus; no codex stall tax." >&2
    ;;
  codex)
    MEMBRANE_LABEL="codex"
    # shellcheck disable=SC2016  # the "$1" is intentionally literal here — eval-membrane.sh expands it via `bash -c "$MEMBRANE_CMD" _ "$prompt"`.
    MEMBRANE_CMD='codex exec --skip-git-repo-check "$1"'
    if [ -n "$TO_BIN" ]; then
      MEMBRANE_CMD="$TO_BIN 240 codex exec --skip-git-repo-check \"\$1\""
    else
      # No timeout binary => the self-heal is OFF. Do NOT pretend to be self-healing:
      # warn loudly so the operator knows a stalled codex review can hang indefinitely.
      echo "run-harvest-series: WARNING — no gtimeout/timeout on PATH; membrane self-heal DISABLED. A stalled 'codex exec' review can hang this run indefinitely. Install coreutils (gtimeout) for unattended use." >&2
    fi
    ;;
  *)
    echo "run-harvest-series: unknown HARVEST_MEMBRANE='${HARVEST_MEMBRANE}' — want 'codex' or 'local'. Refusing to fall back to a wrong reviewer." >&2
    exit 2
    ;;
esac
if ! bash scripts/eval-membrane.sh \
      --producer-cmd "$PRODUCER_CMD" --producer-label "$LABEL" \
      --membrane-cmd "$MEMBRANE_CMD" --membrane-label "$MEMBRANE_LABEL" \
      --timeout 240 --output "$SC" >"$WORK/eval.log" 2>&1; then
  echo "run-harvest-series: eval-membrane FAILED for $RUN_ID — no row appended" >&2
  tail -3 "$WORK/eval.log" >&2; exit 1
fi
[ -s "$SC" ] || { echo "run-harvest-series: no scorecard for $RUN_ID — no row appended" >&2; exit 1; }

# Best-effort SIDE artifact: emit the escape chains into an isolated ledger. The
# series row does NOT depend on this — its data is the scorecard — so a ledger
# failure is logged, not fatal (see the fail-closed note in the header).
bash "$HARVEST_DIR/harvest-to-ledger.sh" "$SC" "$LROOT" "$RUN_ID" >"$WORK/flow.log" 2>&1 \
  || echo "run-harvest-series: WARN harvest-to-ledger non-zero for $RUN_ID (see series anyway)" >&2

# Compute the series row straight from the scorecard (the deterministic truth).
ROW="$(RUN_ID="$RUN_ID" LABEL="$LABEL" MEMBRANE_LABEL="$MEMBRANE_LABEL" python3 -c '
import os, json, sys
d = json.load(open(sys.argv[1]))
t = d.get("totals")
# Fail-closed on scorecard CONTENTS: a present-but-malformed scorecard (missing or
# renamed totals keys) must be REJECTED, not silently defaulted to 0/None — that
# would append a clean-looking zero row and corrupt the series.
required = ("tasks", "false_done", "caught", "escaped", "degraded")
if not isinstance(t, dict) or any(k not in t for k in required):
    sys.stderr.write("run-harvest-series: scorecard missing/renamed totals keys (%r) — rejecting, no row\n" % (t,))
    sys.exit(1)
fd = int(t["false_done"]); caught = int(t["caught"]); missed = int(t["escaped"]); deg = int(t["degraded"])
# Coherence: a present scorecard can still be incoherent. Every false-done is
# either caught or escaped (degraded tasks are NOT counted in false_done), so
# caught+escaped MUST equal false_done; all counts non-negative. Reject otherwise
# rather than append plausible-looking bad data.
if min(fd, caught, missed, deg) < 0 or (caught + missed) != fd:
    sys.stderr.write("run-harvest-series: incoherent totals fd=%d caught=%d escaped=%d deg=%d — rejecting, no row\n"
                     % (fd, caught, missed, deg))
    sys.exit(1)
miss_rate = (missed / fd) if fd else None
catch_rate = (caught / fd) if fd else None
print(json.dumps({
  "run_id": os.environ["RUN_ID"],
  "ts": sys.argv[2],
  "producer": os.environ["LABEL"],
  "membrane": os.environ["MEMBRANE_LABEL"],
  "n_tasks": int(t.get("tasks", 0)),
  "degraded": int(t.get("degraded", 0)),
  "n_false_dones": fd,
  "n_caught": caught,
  "n_missed": missed,
  "membrane_miss_rate": miss_rate,
  "catch_rate": catch_rate,
}))
' "$SC" "$(python3 -c 'import os,sys,datetime; print(datetime.datetime.fromtimestamp(os.path.getmtime(sys.argv[1]), datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))' "$SC")")" || { echo "run-harvest-series: row build failed" >&2; exit 1; }

# Fail-closed on the append itself: an unwritable/missing series path must NOT
# print "appended" and exit 0 (that silently loses the observation).
if ! printf '%s\n' "$ROW" >> "$SERIES"; then
  echo "run-harvest-series: FAILED to append row to $SERIES — observation LOST" >&2
  exit 1
fi
echo "run-harvest-series: appended -> $SERIES" >&2
printf '%s\n' "$ROW"
# Leave $WORK under TMPDIR (OS-reaped); no rm -rf (destructive-guard + safety).

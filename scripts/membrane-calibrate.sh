#!/usr/bin/env bash
# membrane-calibrate.sh — the standing RULER for the verification membrane's
# catch-rate (age-e508.2).
#
# WHAT IT IS (and is NOT):
#   ADR-0011 names the structural problem: a COMPETENT membrane catches nearly
#   everything at review, so real escapes are structurally rare — and a thing that
#   never produces a signal cannot measure its own drift. This harness gives the
#   membrane a STANDING ruler instead of a one-shot memory: it runs the current
#   COLD membrane against a FROZEN weak-producer trap corpus and re-measures its
#   catch-rate + false-refute-rate on the SAME inputs every time, so any change is
#   attributable to the MEMBRANE, never to producer noise (the producer arm is
#   frozen code, not a stochastic model — evals/membrane/frozen/).
#
#   HONESTY (ADR-0011): this is CALIBRATION of the PROVEN membrane — a measurement
#   that the verification still works. It is NOT evidence that the escape-corpus
#   COMPOUNDS or that a knowledge moat exists (both remain demoted/unproven —
#   ADR-0011, ADR-0004). The ruler measures; it does not vindicate the flywheel.
#
# PER-ADAPTER (duel D3): the reviewer is pluggable via --membrane-cmd/--membrane-label,
#   so this is ALSO the instrument that measures a FALLBACK reviewer family's
#   catch-rate. Each --membrane-label keeps its OWN trend history — calibrate the
#   codex baseline AND any fallback adapter (agy/gemini, local-mlx) with the same
#   ruler. Keep the reviewer in a DIFFERENT family than the trap authors intended.
#
# BUDGET (bounded, per ADR-0009 no-daemon): one run issues (#traps + 1 smoke)
#   reviewer calls — the default 5-trap corpus ⇒ ≤ 6 reviewer calls. It NEVER
#   drives a producer model (the corpus is frozen), so producer token cost is
#   zero; total cost is the reviewer's. Wall time ~2–8 min on codex; ~O(10k)
#   tokens. Scheduling is substrate-delegated (a cron line, never an in-repo
#   daemon) — see the suggested crontab at the bottom of this header.
#
# ENTRYPOINT: `ao membrane calibrate [flags]` (thin wrapper) or this script directly.
# SUGGESTED CRON (weekly codex baseline; substrate-owned, not a repo daemon):
#   # 07:00 every Monday — recalibrate the cold membrane and commit the evidence.
#   0 7 * * 1  cd /path/to/agentops && AGENTOPS_TRUST_REPO=1 ao membrane calibrate --membrane-label codex >> ~/.agentops/membrane-calibrate.log 2>&1
#
# POSIX/macOS-portable bash (no GNU-only flags).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
EVAL_MEMBRANE="$SCRIPT_DIR/eval-membrane.sh"
FROZEN_ROOT="$REPO_ROOT/evals/membrane/frozen"
FROZEN_PRODUCER="$REPO_ROOT/evals/membrane/producers/frozen-trap-producer.sh"
TASKS_DIR="$REPO_ROOT/evals/membrane/tasks"

# Default membrane = the cross-family codex reviewer. Pull the literal invocation
# from the shared codex lib so it stays in one place (matches eval-membrane.sh).
. "$SCRIPT_DIR/lib/codex-exec.sh"
DEFAULT_MEMBRANE_CMD="$(codex_exec_producer_template membrane)"

MEMBRANE_CMD="${MEMBRANE_CMD:-$DEFAULT_MEMBRANE_CMD}"
MEMBRANE_LABEL="${MEMBRANE_LABEL:-codex}"
MEMBRANE_TIMEOUT="${MEMBRANE_TIMEOUT:-240}"
OUT_DIR="${OUT_DIR:-$REPO_ROOT/docs/evals}"
NOW="${CALIBRATION_NOW:-}"

usage() {
	cat <<'USAGE'
Usage: scripts/membrane-calibrate.sh [options]

Calibrate the cold verification membrane against the FROZEN weak-producer trap
corpus and emit a dated evidence file with verbatim per-trap outcomes, aggregate
catch-rate / false-refute-rate, and an honest trend vs the prior run.

Options:
  --membrane-cmd <c>    Reviewer command template ($1 = reviewer prompt); must
                        print a line 'VERDICT: ACK' or 'VERDICT: REFUTE'.
                        Default: the cross-family codex reviewer.
  --membrane-label <s>  Adapter label; KEYS the per-adapter trend history.
                        Default: codex. Use e.g. agy-gemini or local-mlx for a
                        fallback reviewer family (duel D3 per-adapter calibration).
  --membrane-timeout <s> Hard per-review timeout (default 240; 0 disables).
  --out-dir <dir>       Where to write the evidence file + history.jsonl
                        (default: docs/evals).
  --now <stamp>         Override the run date (YYYY-MM-DD); for reproducible tests.
  -h, --help            Show this help.

BUDGET: (#traps + 1 smoke) reviewer calls; the frozen corpus is never re-generated
so producer cost is zero. Default 5 traps ⇒ ≤6 reviewer calls, ~2–8 min on codex.

SCHEDULING is substrate-delegated (ADR-0009): a cron line, never an in-repo
daemon. Example weekly baseline:
  0 7 * * 1  cd /path/to/agentops && ao membrane calibrate --membrane-label codex
USAGE
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--membrane-cmd)
		MEMBRANE_CMD="$2"
		shift 2
		;;
	--membrane-label)
		MEMBRANE_LABEL="$2"
		shift 2
		;;
	--membrane-timeout)
		MEMBRANE_TIMEOUT="$2"
		shift 2
		;;
	--out-dir)
		OUT_DIR="$2"
		shift 2
		;;
	--now)
		NOW="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown option: $1" >&2
		exit 1
		;;
	esac
done

[[ -f "$EVAL_MEMBRANE" ]] || {
	echo "error: eval-membrane.sh not found at $EVAL_MEMBRANE" >&2
	exit 1
}
[[ -d "$FROZEN_ROOT" ]] || {
	echo "error: frozen corpus not found at $FROZEN_ROOT" >&2
	exit 1
}
[[ -f "$FROZEN_PRODUCER" ]] || {
	echo "error: frozen producer not found at $FROZEN_PRODUCER" >&2
	exit 1
}
[[ -n "$NOW" ]] || NOW="$(date -u +%Y-%m-%d)"

# The corpus = every task that has a frozen solution (the frozen dir defines it).
TASKS=()
for d in "$FROZEN_ROOT"/*/; do
	[[ -d "$d" ]] || continue
	TASKS+=("$(basename "$d")")
done
[[ ${#TASKS[@]} -gt 0 ]] || {
	echo "error: no frozen tasks under $FROZEN_ROOT" >&2
	exit 1
}

# Content-only fingerprint of the corpus (relative path + file digest, so it is
# stable across machines/worktrees). A change here means the ruler moved — the
# trend logic downgrades a cross-corpus comparison to RE-BASELINE, never a false
# REGRESSION.
corpus_fingerprint() {
	(
		cd "$FROZEN_ROOT" && find . -type f | LC_ALL=C sort | while IFS= read -r f; do
			printf 'frozen/%s ' "$f"
			shasum -a 256 "$f" | awk '{print $1}'
		done
	)
	local t
	for t in "${TASKS[@]}"; do
		(
			cd "$TASKS_DIR/$t" && find . -type f | LC_ALL=C sort | while IFS= read -r f; do
				printf 'tasks/%s/%s ' "$t" "$f"
				shasum -a 256 "$f" | awk '{print $1}'
			done
		)
	done
}
CORPUS_HASH="$(corpus_fingerprint | shasum -a 256 | awk '{print $1}')"

mkdir -p "$OUT_DIR"
# Sanitize the label for filenames (defensive: keep [a-z0-9._-]).
SAFE_LABEL="$(printf '%s' "$MEMBRANE_LABEL" | tr -c 'A-Za-z0-9._-' '-')"
# Disambiguate same-date/same-label re-runs so a second calibration NEVER overwrites the
# first's verbatim evidence while its append-only history row still points at that file
# (corrupting the append-only record). First run: <...>-<date>.md; later same-date runs:
# <...>-<date>-2.md, -3, ... — each history row references its own intact evidence file.
CAL_BASE="$OUT_DIR/membrane-calibration-$SAFE_LABEL-$NOW"
CAL_SUFFIX=""
CAL_SEQ=2
while [[ -e "$CAL_BASE$CAL_SUFFIX.md" || -e "$CAL_BASE$CAL_SUFFIX.scorecard.json" ]]; do
	CAL_SUFFIX="-$CAL_SEQ"
	CAL_SEQ=$((CAL_SEQ + 1))
done
SCORECARD="$CAL_BASE$CAL_SUFFIX.scorecard.json"
EVIDENCE="$CAL_BASE$CAL_SUFFIX.md"
HISTORY="$OUT_DIR/membrane-calibration-history.jsonl"

# --- run the eval over the frozen corpus with the chosen membrane --------------
TASK_ARGS=()
for t in "${TASKS[@]}"; do
	TASK_ARGS+=(--task "$t")
done

echo "membrane-calibrate: running ${#TASKS[@]} frozen traps against membrane '$MEMBRANE_LABEL' (corpus $CORPUS_HASH)…" >&2

bash "$EVAL_MEMBRANE" \
	--tasks-dir "$TASKS_DIR" \
	"${TASK_ARGS[@]}" \
	--producer-cmd "bash '$FROZEN_PRODUCER' \"\$1\"" \
	--producer-label "frozen-trap-corpus" \
	--membrane-cmd "$MEMBRANE_CMD" \
	--membrane-label "$MEMBRANE_LABEL" \
	--membrane-timeout "$MEMBRANE_TIMEOUT" \
	--output "$SCORECARD"

# Stamp generated_at (eval-membrane emits a placeholder for the caller to set).
GENERATED_AT="${CALIBRATION_NOW:+${CALIBRATION_NOW}T00:00:00Z}"
[[ -n "$GENERATED_AT" ]] || GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
tmp_sc="$SCORECARD.tmp"
sed "s|GENERATED_AT_PLACEHOLDER|$GENERATED_AT|" "$SCORECARD" >"$tmp_sc" && mv "$tmp_sc" "$SCORECARD"

# --- prior record for this adapter (for the honest trend diff) -----------------
PRIOR_JSON=""
if [[ -f "$HISTORY" ]]; then
	PRIOR_JSON="$(CAL_LABEL="$MEMBRANE_LABEL" python3 -c '
import json, os, sys
label = os.environ["CAL_LABEL"]
prior = None
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        rec = json.loads(line)
    except Exception:
        continue
    if rec.get("membrane") == label:
        prior = rec
print(json.dumps(prior) if prior is not None else "")
' <"$HISTORY")"
fi

# --- render the evidence file + compute the trend verdict + new record ---------
# All report logic is in one python pass (reads the scorecard, computes the honest
# trend, writes the markdown, and emits the new history record on stdout). No
# smoothing: REGRESSION is any catch_rate drop or false_refute_rate rise on an
# UNCHANGED corpus; a changed corpus is RE-BASELINE, never a silent comparison.
NEW_RECORD="$(
	CAL_SCORECARD="$SCORECARD" \
		CAL_PRIOR="$PRIOR_JSON" \
		CAL_MD_OUT="$EVIDENCE" \
		CAL_LABEL="$MEMBRANE_LABEL" \
		CAL_CORPUS_HASH="$CORPUS_HASH" \
		CAL_NOW="$NOW" \
		CAL_GENERATED_AT="$GENERATED_AT" \
		python3 - <<'PY'
import json, os, textwrap

sc = json.load(open(os.environ["CAL_SCORECARD"]))
prior_raw = os.environ.get("CAL_PRIOR", "").strip()
prior = json.loads(prior_raw) if prior_raw else None
label = os.environ["CAL_LABEL"]
corpus_hash = os.environ["CAL_CORPUS_HASH"]
now = os.environ["CAL_NOW"]
generated_at = os.environ["CAL_GENERATED_AT"]
md_out = os.environ["CAL_MD_OUT"]

totals = sc["totals"]
rates = sc["rates"]
per_task = sc["per_task"]
catch = rates.get("catch_rate")
frr = rates.get("false_refute_rate")


def fmt(x):
    return "n/a" if x is None else f"{x:.4f}"


# ---- honest trend verdict ----------------------------------------------------
def trend_verdict():
    if prior is None:
        return ("BASELINE", "First calibration for this adapter — no prior run to diff. This row IS the baseline.")
    p_catch = prior.get("catch_rate")
    p_frr = prior.get("false_refute_rate")
    p_hash = prior.get("corpus_hash")
    p_ts = prior.get("ts", "?")
    if p_hash != corpus_hash:
        return ("RE-BASELINE",
                f"The corpus changed since the prior run ({p_ts}): hash {p_hash} -> {corpus_hash}. "
                f"Rates are NOT comparable across a changed ruler; treat this as a new baseline.")
    # Same corpus — a real comparison.
    parts = []
    if catch is not None and p_catch is not None:
        parts.append(f"catch_rate {p_catch:.4f} -> {catch:.4f} (Δ {catch - p_catch:+.4f})")
    if frr is not None and p_frr is not None:
        parts.append(f"false_refute_rate {p_frr:.4f} -> {frr:.4f} (Δ {frr - p_frr:+.4f})")
    delta_str = "; ".join(parts) if parts else "rates undefined on one side"
    worse = False
    better = False
    if catch is not None and p_catch is not None:
        if catch < p_catch:
            worse = True
        elif catch > p_catch:
            better = True
    if frr is not None and p_frr is not None:
        if frr > p_frr:
            worse = True
        elif frr < p_frr:
            better = True
    if catch is None or frr is None or p_catch is None or p_frr is None:
        verdict = "INDETERMINATE"
        note = f"A rate is undefined (n/a) on one side, so no honest comparison is possible. Prior {p_ts}: {delta_str}."
    elif worse:
        verdict = "REGRESSION"
        note = (f"On the SAME corpus, the membrane got WORSE vs the prior run ({p_ts}): {delta_str}. "
                f"The cold membrane missed a false-done it used to catch (or false-refuted a control it used to pass). "
                f"Investigate the reviewer/adapter before trusting its verdicts.")
    elif better:
        verdict = "IMPROVEMENT"
        note = f"On the SAME corpus, the membrane IMPROVED vs the prior run ({p_ts}): {delta_str}."
    else:
        verdict = "STABLE"
        note = f"On the SAME corpus, the membrane is unchanged vs the prior run ({p_ts}): {delta_str}."
    return (verdict, note)


verdict, trend_note = trend_verdict()

# ---- per-trap verbatim table -------------------------------------------------
disp = {"caught": "false-done", "escaped": "false-done", "false_refute": "control",
        "correct_ack": "control", "degraded": "—", "dry": "—"}


def esc(s):
    return (s or "").replace("|", "\\|").replace("\n", " ").strip()


rows = []
for e in per_task:
    rows.append("| `{task}` | {kind} | {oracle} | **{verdict}** | {klass} | {why} |".format(
        task=e["task"],
        kind=disp.get(e.get("class", ""), "—"),
        oracle=("PASS (done)" if e.get("oracle_pass") in (True, "true") else "FAIL (false-done)"),
        verdict=e.get("verdict", "—"),
        klass=e.get("class", "—"),
        why=esc(e.get("why", "")) or "_(none)_",
    ))
table = "\n".join(rows)

# ---- markdown ----------------------------------------------------------------
md = f"""# Membrane calibration — `{label}` — {now}

> **What this is:** a standing-ruler measurement of the COLD verification
> membrane's catch-rate against the FROZEN weak-producer trap corpus
> (`evals/membrane/frozen/`). The producer arm is frozen code — not a stochastic
> model — so this run is reproducible byte-for-byte from the same corpus, and any
> change is attributable to the MEMBRANE (`{label}`), not producer noise.
>
> **HONESTY (ADR-0011):** this CALIBRATES the *proven* membrane — it confirms the
> verification still works and tracks its drift. It is **NOT** evidence that the
> escape-corpus *compounds* or that a knowledge moat exists (both remain
> demoted/unproven — see ADR-0011 and ADR-0004). The ruler measures; it does not
> vindicate the flywheel.

## Trend verdict: **{verdict}**

{trend_note}

| Field | Value |
|---|---|
| Adapter (reviewer) | `{label}` |
| Producer | `{sc.get('producer', 'frozen-trap-corpus')}` (frozen, deterministic) |
| Corpus fingerprint | `{corpus_hash}` |
| Run date | {now} (`{generated_at}`) |
| Traps (false-done) | {totals['false_done']} |
| Controls (true-done) | {totals['true_done']} |
| Degraded (excluded) | {totals['degraded']} |

## Aggregate rates

| Metric | Value | Meaning |
|---|---|---|
| **catch_rate** | **{fmt(catch)}** | caught / false_done — fraction of shipped false-dones the membrane REFUTED (higher is better) |
| **false_refute_rate** | **{fmt(frr)}** | false_refute / true_done — fraction of correct controls the membrane wrongly REFUTED (lower is better) |
| caught | {totals['caught']} | false-dones correctly REFUTED |
| escaped | {totals['escaped']} | false-dones the membrane MISSED (ACKed) |
| correct_ack | {totals['correct_ack']} | controls correctly ACKed |
| false_refute | {totals['false_refute']} | controls wrongly REFUTED |

{('> Note: ' + rates['note']) if rates.get('note') else ''}

## Per-trap outcomes (verbatim)

Each row is the membrane's own verdict + its verbatim `WHY:` — no summarization.

| Trap | Kind | Oracle | Verdict | Class | WHY (verbatim) |
|---|---|---|---|---|---|
{table}

## Reproduce

```bash
# Re-run this exact calibration (frozen corpus + this adapter):
ao membrane calibrate --membrane-label {label}
# or directly:
bash scripts/membrane-calibrate.sh --membrane-label {label}
```

The trend spine is `docs/evals/membrane-calibration-history.jsonl` (append-only,
one record per run, keyed by adapter label). A `RE-BASELINE` verdict means the
corpus fingerprint changed — rates before/after are not comparable.

<!-- calibration-record: machine-readable; do not hand-edit -->
"""

open(md_out, "w").write(md)

record = {
    "ts": generated_at,
    "date": now,
    "membrane": label,
    "producer": sc.get("producer", "frozen-trap-corpus"),
    "corpus_hash": corpus_hash,
    "catch_rate": catch,
    "false_refute_rate": frr,
    "caught": totals["caught"],
    "escaped": totals["escaped"],
    "false_done": totals["false_done"],
    "true_done": totals["true_done"],
    "false_refute": totals["false_refute"],
    "correct_ack": totals["correct_ack"],
    "degraded": totals["degraded"],
    "verdict": verdict,
    "evidence_file": os.path.basename(md_out),
}
print(json.dumps(record))
PY
)"

# Append the new record to the append-only trend spine.
printf '%s\n' "$NEW_RECORD" >>"$HISTORY"

VERDICT="$(printf '%s' "$NEW_RECORD" | python3 -c 'import json,sys;print(json.load(sys.stdin)["verdict"])')"
CATCH="$(printf '%s' "$NEW_RECORD" | python3 -c 'import json,sys;print(json.load(sys.stdin)["catch_rate"])')"
FRR="$(printf '%s' "$NEW_RECORD" | python3 -c 'import json,sys;print(json.load(sys.stdin)["false_refute_rate"])')"

echo "membrane-calibrate: $MEMBRANE_LABEL — verdict=$VERDICT catch_rate=$CATCH false_refute_rate=$FRR" >&2
echo "  evidence:  $EVIDENCE" >&2
echo "  scorecard: $SCORECARD" >&2
echo "  history:   $HISTORY" >&2

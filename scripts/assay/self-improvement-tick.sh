#!/usr/bin/env bash
# self-improvement-tick.sh — the M4 ASSAY tick (age-d16-self-hosting-route-nkr.5).
#
# WHAT: a BOUNDED periodic tick that closes the self-improvement loop —
#   SENSOR (read a completed run's evidence from the provenance ledger)
#     -> ASSAY (run bounded miners over that evidence to derive suggestions)
#       -> GATE (each suggestion re-enters the front door as a follow-up bead).
# It is the net-new ORCHESTRATION over EXISTING surfaces; it does NOT rebuild
# any miner. The default assay is a pure-shell read of the ledger; production
# wires a heavier existing miner via --mine-cmd (e.g. `ao harvest`, /curate).
#
# CRISP TERMINAL BEHAVIOR (mirrors scripts/recovery-statemachine.sh, M2):
#   - NO daemon / NO spin: a single pass over a bounded ledger window; no loop,
#     no sleep, no background. The verdict reports bounded:true,daemonized:false.
#   - NO silent defer: every path emits exactly ONE terminal JSON verdict line;
#     a non-zero `br create` ESCALATES loudly (exit 4) instead of aborting under
#     set -e with no verdict. "Nothing to mine" still emits a no-evidence verdict.
#   - NO mis-close: the tick NEVER closes a bead. It only FILES follow-ups;
#     acceptance/close authority stays at the merge/pawl door (reconcile-pr.sh).
#   - BOUNDED gate: at most --max-suggestions beads are filed per tick.
#
# Non-goals: unbounded/daemonized mining; rebuilding forge/compile/inject/curate;
# inventing dream/harvest miners; flywheel/gold/wiki/corpus-PROMOTE (Mossy's
# lane). Law 0 throughout: only grep/sed/printf/br — never `claude -p`.
#
# Exit codes (a scheduling loop branches on these):
#   0  ok        — tick completed (suggestions filed, or no-evidence no-op)
#   2  usage     — bad/insufficient arguments (loud, not deferred)
#   4  gate-fail — a miner or `br create` failed; LOUD terminal, never silent
set -euo pipefail

# Bound on the GATE: at most this many follow-up beads filed per tick. The "no
# runaway" guard — a single completed run can seed at most N suggestions.
DEFAULT_MAX_SUGGESTIONS=1
# Bound on the SENSOR: mine only the last N ledger rows (the most recent run's
# evidence), never the whole history. The "no unbounded scan" guard.
DEFAULT_WINDOW=50

usage() {
  cat >&2 <<'EOF'
Usage: self-improvement-tick.sh [--ledger <path>] [--window <n>] \
         [--max-suggestions <n>] [--mine-cmd <cmd>] [--beads-dir <dir>] [--dry-run]

  --ledger <path>          provenance ledger to mine (default: docs/provenance/ledger.jsonl)
  --window <n>             mine only the last N ledger rows (default: 50; the SENSOR bound)
  --max-suggestions <n>    cap follow-up beads filed this tick (default: 1; the GATE bound)
  --mine-cmd <cmd>         ASSAY miner; the ledger window is fed on its STDIN and it
                           prints suggestion lines to STDOUT (one per line). Default:
                           a built-in ledger assay. Wire an existing miner here in
                           production (e.g. "ao harvest --quiet"). A non-zero exit
                           ESCALATES (exit 4) — never a silent defer.
  --beads-dir <dir>        BEADS_DIR for the front-door `br create` (default: $PWD/_beads)
  --dry-run                decide + emit the verdict; file NO bead
  -h|--help                this help

Emits ONE structured terminal line (JSON) on stdout. Never spins, never
daemonizes, never silently defers, never closes a bead.
EOF
}

LEDGER="docs/provenance/ledger.jsonl"
WINDOW="$DEFAULT_WINDOW"
MAX_SUGGESTIONS="$DEFAULT_MAX_SUGGESTIONS"
MINE_CMD=""
BEADS_DIR_ARG=""
DRY_RUN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --ledger) LEDGER="${2:-}"; shift 2 ;;
    --window) WINDOW="${2:-}"; shift 2 ;;
    --max-suggestions) MAX_SUGGESTIONS="${2:-}"; shift 2 ;;
    --mine-cmd) MINE_CMD="${2:-}"; shift 2 ;;
    --beads-dir) BEADS_DIR_ARG="${2:-}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "assay-tick: unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

# Loud usage validation — a bad bound is an error, never a silent default.
case "$WINDOW" in (*[!0-9]*|'') echo "assay-tick: --window must be a positive integer, got '$WINDOW'" >&2; exit 2 ;; esac
case "$MAX_SUGGESTIONS" in (*[!0-9]*|'') echo "assay-tick: --max-suggestions must be a positive integer, got '$MAX_SUGGESTIONS'" >&2; exit 2 ;; esac
[ "$WINDOW" -ge 1 ] || { echo "assay-tick: --window must be >= 1" >&2; exit 2 ; }
[ "$MAX_SUGGESTIONS" -ge 1 ] || { echo "assay-tick: --max-suggestions must be >= 1" >&2; exit 2 ; }

BEADS_DIR="${BEADS_DIR_ARG:-${BEADS_DIR:-$PWD/_beads}}"
export BEADS_DIR

# emit_terminal <state> <evidence_rows> <suggestions_filed> [extra_json] -> the
# single crisp terminal verdict. bounded/daemonized/spin are constants by
# construction (this script has no loop, no sleep, no background).
emit_terminal() {
  local state="$1" rows="$2" filed="$3" extra="${4:-}"
  printf '{"tick":"self-improvement-assay","state":"%s","ledger":"%s","window":%s,"evidence_rows":%s,"suggestions_filed":%s,"bounded":true,"daemonized":false,"spin":false%s}\n' \
    "$state" "$LEDGER" "$WINDOW" "$rows" "$filed" "${extra:+,$extra}"
}

# ---------------------------------------------------------------------------
# SENSOR — read the most recent run's evidence (the bounded ledger window).
# Evidence = verdict rows (pawl/gate verdicts that a completed run produced).
# Absent/empty ledger -> a crisp no-evidence verdict (NOT a silent no-op).
# ---------------------------------------------------------------------------
if [ ! -s "$LEDGER" ]; then
  emit_terminal "no-evidence" 0 0
  exit 0
fi

# Bounded window: last N rows only. Keep just the verdict rows — those are the
# completed-run signals worth mining (landed edges are M1's lane, not assay's).
EVIDENCE="$(tail -n "$WINDOW" "$LEDGER" | grep '"from_type":"verdict"' || true)"
EVIDENCE_ROWS="$(printf '%s' "$EVIDENCE" | grep -c '"from_type":"verdict"' || true)"
[ -n "$EVIDENCE_ROWS" ] || EVIDENCE_ROWS=0

if [ "$EVIDENCE_ROWS" -eq 0 ]; then
  emit_terminal "no-evidence" 0 0
  exit 0
fi

# ---------------------------------------------------------------------------
# ASSAY — derive suggestion lines from the evidence (bounded). Each suggestion
# line is TAB-delimited: <bead>\t<commit_sha>\t<evidence_ref>. The default is a
# pure-shell read of the ledger; --mine-cmd swaps in an existing heavier miner
# (the evidence window on its stdin), reusing — never rebuilding — that surface.
# ---------------------------------------------------------------------------

# The default assay derives one suggestion per DISTINCT bead seen in the window's
# verdict rows (most-recent first). A real, bounded mine of the SENSOR — it
# guarantees >=1 suggestion whenever evidence exists, so the acceptance holds
# deterministically. reverse() must NEVER exit non-zero (it feeds an unguarded
# command substitution; a non-zero exit there would abort under set -e with no
# verdict — a silent defer). `tac` (GNU) and `tail -r` (BSD) are platform-split,
# so we end with a pure-awk reverser that exists everywhere and always exits 0.
reverse() {
  if command -v tac >/dev/null 2>&1; then tac
  elif tail -r </dev/null >/dev/null 2>&1; then tail -r
  else awk '{ a[NR] = $0 } END { for (i = NR; i >= 1; i--) print a[i] }'
  fi
}

# Run the miner. A non-zero miner exit is a LOUD gate failure, not a silent
# defer (mirrors M2's `if ! ...` capture under set -e).
if [ -n "$MINE_CMD" ]; then
  if ! SUGGESTIONS="$(printf '%s\n' "$EVIDENCE" | bash -c "$MINE_CMD")"; then
    emit_terminal "gate-failed" "$EVIDENCE_ROWS" 0 "\"error\":\"mine-cmd-nonzero\""
    exit 4
  fi
else
  # Default assay over the bounded window. Guarded like the --mine-cmd capture:
  # a non-zero pipeline ESCALATES loudly (exit 4) instead of aborting silently
  # under set -e with no verdict (the M2-class fail-open the refuter caught).
  # NOTE: printf '%s\n' (trailing newline) is load-bearing. EVIDENCE is captured
  # without a trailing newline; a `tac`/`tail -r` reverse of an unterminated final
  # record GLUES it to its neighbor, so the awk sees two JSON rows on one line and
  # silently drops the second bead's suggestion (the awk fallback splits correctly,
  # masking it on GNU-bare boxes). Terminate the stream so all 3 reversers agree.
  if ! SUGGESTIONS="$(printf '%s\n' "$EVIDENCE" | reverse | awk '
    {
      line=$0
      if (match(line, /ag-[a-zA-Z0-9._-]+/)) { bead=substr(line, RSTART, RLENGTH) } else next
      if (seen[bead]++) next
      sha="unknown"; ref="pawl-verdict"
      if (match(line, /"to_id":"[^"]*"/)) { t=substr(line,RSTART,RLENGTH); gsub(/"to_id":"|"$/,"",t); sha=t }
      if (match(line, /"evidence_ref":"[^"]*"/)) { r=substr(line,RSTART,RLENGTH); gsub(/"evidence_ref":"|"$/,"",r); ref=r }
      printf "%s\t%s\t%s\n", bead, sha, ref
    }')"; then
    emit_terminal "gate-failed" "$EVIDENCE_ROWS" 0 "\"error\":\"default-assay-failed\""
    exit 4
  fi
fi

# BOUNDED gate: never file more than --max-suggestions beads, however many the
# miner proposed. This is the "no runaway" cap.
SUGGESTIONS="$(printf '%s\n' "$SUGGESTIONS" | grep -v '^[[:space:]]*$' | head -n "$MAX_SUGGESTIONS" || true)"

if [ -z "$SUGGESTIONS" ]; then
  # Evidence existed but the miner proposed nothing actionable — crisp terminal,
  # no bead, no defer.
  emit_terminal "no-suggestions" "$EVIDENCE_ROWS" 0
  exit 0
fi

# ---------------------------------------------------------------------------
# GATE — each suggestion re-enters the front door as a follow-up bead, UNATTENDED.
# The tick NEVER closes a bead; it only files follow-ups (br create). A non-zero
# `br create` escalates loudly (exit 4), never a silent set -e abort.
# ---------------------------------------------------------------------------
FILED=0
FILED_IDS=""
while IFS=$'\t' read -r bead sha ref; do
  [ -n "$bead" ] || continue
  body="ASSAY-tick suggestion derived from completed-run evidence.

Mined from: ${LEDGER} (verdict row)
Source bead: ${bead}
Head commit: ${sha}
Evidence ref: ${ref}

## Scenarios
- Given the completed run's evidence for ${bead} (${ref}), When this follow-up is worked, Then a concrete improvement to that surface is implemented and re-validated through the pawl door.

Filed unattended by scripts/assay/self-improvement-tick.sh (M4). Re-enters the front door for triage; acceptance/close authority remains at the merge/pawl door."

  if [ "$DRY_RUN" -eq 1 ]; then
    echo "assay-tick[dry-run]: would file follow-up bead for $bead (sha=$sha)" >&2
    FILED=$((FILED + 1))
    continue
  fi

  if ! new_id="$(br create "ASSAY follow-up from $bead" -t task -p 2 \
        --labels assay-suggestion --silent \
        --description "$body")"; then
    emit_terminal "gate-failed" "$EVIDENCE_ROWS" "$FILED" "\"error\":\"br-create-nonzero\",\"failed_for\":\"$bead\""
    exit 4
  fi
  new_id="$(printf '%s' "$new_id" | tr -d '[:space:]')"
  [ -n "$new_id" ] || { emit_terminal "gate-failed" "$EVIDENCE_ROWS" "$FILED" "\"error\":\"br-create-empty-id\",\"failed_for\":\"$bead\""; exit 4; }
  FILED=$((FILED + 1))
  FILED_IDS="${FILED_IDS:+$FILED_IDS,}\"$new_id\""
done <<EOF
$SUGGESTIONS
EOF

if [ "$DRY_RUN" -eq 1 ]; then
  emit_terminal "dry-run" "$EVIDENCE_ROWS" "$FILED" "\"filed_beads\":[]"
  exit 0
fi

emit_terminal "filed" "$EVIDENCE_ROWS" "$FILED" "\"filed_beads\":[$FILED_IDS]"
exit 0

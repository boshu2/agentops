#!/usr/bin/env bash
# gc-outcomes-report.sh — thin READ-ONLY rollup of a Gas City's outcomes for
# the agentops br ledger (age-gc-adoption-u0he.6; carried from age-gc-mvp-w2-nuiw.8).
#
# NOT a live sync. This reads a city's event stream + bead store and prints a
# report the operator (or a bead note) can consume: closed work beads with
# outcomes, membrane verdicts where stamped, and open in-progress work. The
# tracker seam it serves: br = the agentops ledger; bd/dolt = the city's own
# store; outcomes cross ONE WAY, via this report (skills/beads-br carve-out,
# age-gc-integrate-8aom.2). It never writes to either store.
#
# Usage: scripts/gc-outcomes-report.sh <city-dir> [--since <dur>] [--json]
#   <city-dir>   a gc city root (contains .gc-home/ or an env.sh exporting GC_HOME)
#   --since      lookback window for close events (default 24h; Ns/Nm/Nh/Nd)
#   --json       emit the raw rows as JSON lines instead of the text report
set -euo pipefail

usage() { sed -n '3,14p' "$0" | sed 's/^# \{0,1\}//'; }

CITY=""
SINCE="24h"
AS_JSON=0
while [ $# -gt 0 ]; do
  case "$1" in
    --since) [ $# -ge 2 ] || { echo "gc-outcomes-report: --since needs a value" >&2; exit 2; }
             SINCE="$2"; shift 2 ;;
    --json)  AS_JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*)      echo "gc-outcomes-report: unknown flag $1" >&2; exit 2 ;;
    *)       CITY="$1"; shift ;;
  esac
done
[ -n "$CITY" ] || { usage >&2; exit 2; }
[ -d "$CITY" ] || { echo "gc-outcomes-report: no such city dir: $CITY (remedy: pass the city root)" >&2; exit 1; }
# Canonicalize ONCE to an absolute physical path — every derived value
# (GC_HOME, GC_CITY_PATH, cd targets) shares this single resolution. A
# relative or symlinked argument otherwise re-resolves differently after cd
# (round-8 pawl finding: GC_HOME=hq/.gc-home targeted a path INSIDE the city).
CITY="$(cd "$CITY" && pwd -P)" || { echo "gc-outcomes-report: cannot resolve city dir" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || { echo "gc-outcomes-report: jq is required (remedy: brew install jq)" >&2; exit 1; }
# Honor GC_BIN (install-gc-city.sh --gc-binary cities export it via env.sh);
# fall back to PATH resolution.
GC="${GC_BIN:-gc}"
command -v "$GC" >/dev/null 2>&1 || { echo "gc-outcomes-report: gc not found (remedy: source $CITY/env.sh first, or set GC_BIN)" >&2; exit 1; }

# Pin BOTH gc-resolution env vars to the REQUESTED city — never honor
# inherited values. An inherited GC_CITY_PATH or GC_HOME from another city's
# shell (env.sh sourced earlier) would make gc read a different city while
# this report labels itself as $CITY — a wrong-object certification (pawl
# findings, rounds 6-7, 2026-07-06). The city-dir ARGUMENT is the only
# source of truth here.
[ -d "$CITY/.gc-home" ] || { echo "gc-outcomes-report: $CITY has no .gc-home — not an installed city (remedy: run scripts/install-gc-city.sh $CITY first)" >&2; exit 1; }
GC_HOME="$CITY/.gc-home"
export GC_HOME
export GC_CITY_PATH="$CITY"

# Closed work beads in the window. Order-tracking/session housekeeping beads are
# noise for a tracker rollup — keep real work: quests, molecules, tasks that are
# not order-run lifecycle rows.
# Fail-closed reads: a rollup that cannot read the city must say so, never
# certify "none" from a failed read (unreachable is not absent).
if ! events_json="$(cd "$CITY" && "$GC" events --type bead.closed --since "$SINCE" 2>&1)"; then
  echo "gc-outcomes-report: gc events failed — refusing to report from a failed read (remedy: is the city running? source $CITY/env.sh). First error lines: $(printf '%s' "$events_json" | head -2)" >&2
  exit 1
fi

# Parse-validate BEFORE filtering (fail-closed: non-JSON output — wrong binary,
# banner text, partial write — must refuse, not read as an empty city).
if [ -n "$events_json" ] && ! printf '%s\n' "$events_json" | jq -e -s 'type=="array"' >/dev/null 2>&1; then
  echo "gc-outcomes-report: gc events output is not parseable JSONL — refusing to report (first lines: $(printf '%s' "$events_json" | head -1 | cut -c1-120))" >&2
  exit 1
fi
rows="$(printf '%s\n' "$events_json" | jq -c '
  # Both event payload shapes are canonical: the wrapped control row
  # (.payload.bead) AND the raw bead snapshot under .payload (gc local
  # fallback; pinned by gc internal/api/event_payloads_test.go). Dropping the
  # raw shape silently reported "none in window" for real closed work.
  (.payload.bead // (.payload | select(type=="object" and has("id") and has("status")))) as $b
  | select($b != null)
  | select(($b.labels // []) | any(startswith("order-run:")) | not)
  | select(($b.issue_type // "task") as $t | ($t == "session" or $t == "agent" or $t == "role" or $t == "rig") | not)
  | select(($b.title // "") | startswith("nudge:") | not)
  | {
      id: $b.id,
      title: $b.title,
      closed_at: .ts,
      outcome: ($b.metadata["gc.outcome"] // $b.metadata.close_reason // "closed"),
      work_commit: ($b.metadata["gc.work_commit"] // ""),
      source_bead: ($b.metadata["gc.source_bead_id"] // ""),
      verdict: ($b.metadata["gc.verdict_disposition"] // "")
    }')" || { echo "gc-outcomes-report: filtering events failed — refusing to report" >&2; exit 1; }

if ! open_json="$(cd "$CITY" && "$GC" bd list --json 2>&1)"; then
  echo "gc-outcomes-report: gc bd list failed — refusing to report from a failed read (remedy: is the city store up?). First error lines: $(printf '%s' "$open_json" | head -2)" >&2
  exit 1
fi
if ! printf '%s\n' "$open_json" | jq -e 'type=="array"' >/dev/null 2>&1; then
  echo "gc-outcomes-report: gc bd list output is not a JSON array — refusing to report (first line: $(printf '%s' "$open_json" | head -1 | cut -c1-120))" >&2
  exit 1
fi
open_rows="$(printf '%s\n' "$open_json" | jq -c '
  .[]
  | select(.status == "open" or .status == "in_progress")
  | select(.issue_type != "session" and .issue_type != "agent" and .issue_type != "role" and .issue_type != "rig")
  | {id, title, status, assignee: (.assignee // "")}')" || { echo "gc-outcomes-report: filtering bd list failed — refusing to report" >&2; exit 1; }

if [ "$AS_JSON" -eq 1 ]; then
  printf '%s\n' "$rows"
  printf '%s\n' "$open_rows"
  exit 0
fi

city_name="$(basename "$CITY")"
echo "# gc outcomes — city '$city_name' (window: $SINCE)"
echo
echo "## Closed work"
if [ -n "$rows" ]; then
  printf '%s\n' "$rows" | jq -r '"- \(.id)  \(.title)  [\(.outcome)]\(if .work_commit != "" then "  commit=\(.work_commit[0:12])" else "" end)\(if .source_bead != "" then "  source=\(.source_bead)" else "" end)"'
else
  echo "- (none in window)"
fi
echo
echo "## Open / in-progress work"
if [ -n "$open_rows" ]; then
  printf '%s\n' "$open_rows" | jq -r '"- \(.id)  \(.title)  [\(.status)]\(if .assignee != "" then "  assignee=\(.assignee)" else "" end)"'
else
  echo "- (none)"
fi
echo
echo "_read-only rollup; br remains the agentops ledger — paste relevant lines into a bead note, do not sync stores_"

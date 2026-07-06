#!/usr/bin/env bash
# verification-economics-report.sh — observational economics surface for the
# verification membrane (age-verification-economics-ebec.2; assessment:
# docs/audits/verification-economics.md).
#
# Reports what the repo can MEASURE today about verification yield and spend:
#   - structural verdict edges from docs/provenance/ledger.jsonl
#     (from_type=verdict): totals, refute rate, per-reviewer-family and
#     per-month breakdowns;
#   - git verdict-bind subjects ("bind pawl ...") as a volume cross-check
#     (SKIPped on shallow clones — the ledger section still reports);
#   - cost columns (cost/verdict, cost/catch, VOR) — printed UNMEASURED until
#     the meter (age-verification-economics-ebec.1) attaches usage fields to
#     verdict edges. READ-ONLY: writes nothing.
#
# Exit: 0 = report printed; 1 = dead instrument (ledger unreadable, or zero
# verdict edges — kin of provenance-feed-health); 2 = usage error.
#
# Usage: scripts/verification-economics-report.sh [--repo <dir>] [--ledger <file>] [--json]
set -euo pipefail

usage() { sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; }

REPO=""
LEDGER=""
AS_JSON=0
while [ $# -gt 0 ]; do
  case "$1" in
    --repo)   [ $# -ge 2 ] || { echo "verification-economics: --repo needs a value" >&2; exit 2; }
              REPO="$2"; shift 2 ;;
    --ledger) [ $# -ge 2 ] || { echo "verification-economics: --ledger needs a value" >&2; exit 2; }
              LEDGER="$2"; shift 2 ;;
    --json)   AS_JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *)        echo "verification-economics: unknown arg $1 (see --help)" >&2; exit 2 ;;
  esac
done

command -v jq >/dev/null 2>&1 || { echo "verification-economics: FAIL — jq is required (remedy: brew install jq)" >&2; exit 1; }
REPO="${REPO:-$(git rev-parse --show-toplevel)}"
LEDGER="${LEDGER:-$REPO/docs/provenance/ledger.jsonl}"
[ -r "$LEDGER" ] || { echo "verification-economics: FAIL — ledger unreadable at $LEDGER (remedy: run from the repo root or pass --ledger)" >&2; exit 1; }

# Git side: subject-grep volume cross-check. Shallow history would undercount,
# so SKIP the section rather than report a wrong number.
shallow="$(git -C "$REPO" rev-parse --is-shallow-repository 2>/dev/null || echo true)"
if [ "$shallow" = "true" ]; then
  binds_total=0; binds_confirmed=0; binds_refuted=0
else
  binds_total="$(git -C "$REPO" log --oneline --grep="bind pawl" 2>/dev/null | wc -l | tr -d '[:space:]')"
  binds_confirmed="$(git -C "$REPO" log --oneline --grep="bind pawl CONFIRMED" 2>/dev/null | wc -l | tr -d '[:space:]')"
  binds_refuted="$(git -C "$REPO" log --oneline --grep="bind pawl REFUTED" 2>/dev/null | wc -l | tr -d '[:space:]')"
fi

report_json="$(jq -R -s \
  --argjson bt "${binds_total:-0}" \
  --argjson bc "${binds_confirmed:-0}" \
  --argjson br "${binds_refuted:-0}" \
  --arg shallow "$shallow" \
  --arg now "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '
  def disp: (.evidence_ref // "")
    | if test("disposition=CONFIRMED") then "CONFIRMED"
      elif test("disposition=REFUTED") then "REFUTED"
      else "UNKNOWN" end;
  (split("\n") | map(select(length > 0) | (fromjson? // empty))) as $recs
  | ($recs | map(select(.from_type == "verdict"))) as $v
  | ($v | map(select(disp == "CONFIRMED")) | length) as $c
  | ($v | map(select(disp == "REFUTED")) | length) as $r
  | ($c + $r) as $n
  | {
      generated_at: $now,
      ledger: {
        records: ($recs | length),
        verdicts: ($v | length),
        confirmed: $c,
        refuted: $r,
        refute_rate: (if $n > 0 then ($r / $n) else null end),
        by_family: ($v | group_by(.reviewer_family // "unknown")
          | map({key: (.[0].reviewer_family // "unknown"), value: length}) | from_entries),
        by_month: ($v | map({m: ((.ts // "unknown")[0:7]), d: disp}) | group_by(.m)
          | map({key: .[0].m, value: {
              confirmed: (map(select(.d == "CONFIRMED")) | length),
              refuted:   (map(select(.d == "REFUTED"))   | length)}})
          | from_entries)
      },
      git_binds: {
        total: $bt, confirmed: $bc, refuted: $br,
        other: ($bt - $bc - $br),
        shallow: ($shallow == "true")
      },
      cost: {
        status: "UNMEASURED",
        meter_bead: "age-verification-economics-ebec.1",
        note: "cost/verdict, cost/catch, and VOR require usage fields on verdict edges"
      }
    }' "$LEDGER")"

verdicts="$(jq -r '.ledger.verdicts' <<<"$report_json")"
if [ "$verdicts" -eq 0 ]; then
  echo "verification-economics: FAIL — dead instrument: 0 verdict edges in $LEDGER (emitters silent? kin of provenance-feed-health)" >&2
  exit 1
fi

if [ "$AS_JSON" -eq 1 ]; then
  printf '%s\n' "$report_json"
  exit 0
fi

jq -r '
  def pct1: (. * 1000 | round) as $m
    | "\($m / 10)" + (if ($m % 10) == 0 then ".0" else "" end);
  "Verification economics — observational report (\(.generated_at))",
  "",
  "provenance ledger (structural verdict edges):",
  "  records: \(.ledger.records)",
  "  verdict edges: \(.ledger.verdicts) (CONFIRMED: \(.ledger.confirmed) / REFUTED: \(.ledger.refuted))",
  "  refute rate: \(.ledger.refute_rate | pct1)% (\(.ledger.refuted)/\(.ledger.confirmed + .ledger.refuted))",
  "  by family:",
  (.ledger.by_family | to_entries | sort_by(.key)[] | "    \(.key): \(.value)"),
  "  by month (CONFIRMED/REFUTED):",
  (.ledger.by_month | to_entries | sort_by(.key)[] | "    \(.key)  \(.value.confirmed)/\(.value.refuted)"),
  "",
  "git verdict binds (subject cross-check):",
  (if .git_binds.shallow
   then "  git binds: SKIP (shallow clone — bind history incomplete)"
   else "  git binds: \(.git_binds.total) total (\(.git_binds.confirmed) CONFIRMED / \(.git_binds.refuted) REFUTED / \(.git_binds.other) other)" end),
  "",
  "economics:",
  "  cost per verdict: UNMEASURED — land the meter (\(.cost.meter_bead))",
  "  cost per catch: UNMEASURED — land the meter (\(.cost.meter_bead))",
  "  verification overhead ratio (VOR): UNMEASURED — land the meter (\(.cost.meter_bead))"
' <<<"$report_json"

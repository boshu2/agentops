#!/usr/bin/env bash
# intake.sh — the front-door fuse for the minimal operating model.
#
# Before work starts, this enforces the two things the 3-hour basement run
# proved we lack: a defined "done", and a bounded descent. It also classifies
# the work's blast-radius so you know whether to stay SOLO (chaos — just build)
# or summon the cross-family Navi (pawl — one-way door, needs a receipt).
#
# Mechanical, not goodwill: no done-test => it stops you (exit 1). The basement
# alarm (>2 prerequisite layers) => it stops you (exit 1). See
# docs/contracts/pawls.md for the blast-radius rule this classifies against.
#
# SCOPE — advisory triage, NOT the pawl enforcement. The classification is a
# cheap early-warning so you summon the Navi at INTAKE rather than after 3 hours.
# The un-leakable enforcement lives at the doors: scripts/reconcile-pr.sh gates
# the merge, dcg gates destructive commands. A pawl this classifier misses is
# still caught at the door (you just summon the Navi later than ideal) — it is
# never silently shipped. So the classifier is kept simple on purpose.
#
# Usage:
#   intake.sh --intent "<goal>" --done-test "<how you'll know it's done>" \
#             [--surfaces "<what it touches>"] [--depth <prereq layers, default 0>]
#
# Exit codes:
#   0  proceed — intake recorded; class (CHAOS|PAWL) printed
#   1  STOP — fuse tripped (no done-test, or >2 prerequisite layers)
#   2  usage error
set -euo pipefail

intent=""; done_test=""; surfaces=""; depth=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --intent)    intent="${2:-}"; shift 2 ;;
    --done-test) done_test="${2:-}"; shift 2 ;;
    --surfaces)  surfaces="${2:-}"; shift 2 ;;
    --depth)     depth="${2:-0}"; shift 2 ;;
    -h|--help)   sed -n '2,21p' "$0"; exit 0 ;;
    *) echo "intake: unknown arg: $1" >&2; exit 2 ;;
  esac
done

[[ -n "$intent" ]] || { echo "intake: --intent is required" >&2; exit 2; }

# Fuse 1 — destination must be defined. No done-test, no start.
# (trim whitespace first: "   " is not a done-test.)
done_test_trimmed=$(printf '%s' "$done_test" | tr -d '[:space:]')
if [[ -z "$done_test_trimmed" ]]; then
  echo "STOP: no done-test. Define how you'll know it's done before you start." >&2
  echo "  (the destination, not the route — set what 'arrived' means.)" >&2
  exit 1
fi

# Fuse 2 — bounded descent. >2 prerequisite layers = you're in a basement.
if ! [[ "$depth" =~ ^[0-9]+$ ]]; then
  echo "intake: --depth must be a non-negative integer" >&2; exit 2
fi
if (( depth > 2 )); then
  echo "STOP: $depth prerequisite layers deep for '$intent'." >&2
  echo "  Climb out and re-aim, or shrink the goal. (the basement alarm.)" >&2
  exit 1
fi

# Blast-radius classification — default CHAOS; PAWL on an irreversible match.
# Mirrors docs/contracts/pawls.md: mutates shared state | changes enforcement /
# gate logic | external effect | hard rollback = pawl. Everything else is chaos.
# HEURISTIC + fail-toward-PAWL: it covers the named pawl classes; on a true
# ambiguity it over-flags (extra review is cheap; a missed one-way door is not),
# and the cross-family Navi / human is the final arbiter. By DESIGN it does not
# do verb-sense disambiguation — a one-way-door keyword ("github", "deploy")
# routes PAWL even in a read-only phrasing ("read a github issue"); the human
# de-escalates. Matching is WHOLE-WORD over a normalized, space-squeezed token
# stream so substrings ("investiGATE", "ACCESSibility") and multi-space phrases
# ("disable   alert") behave — the single leading+trailing space bounds tokens.
norm=$(printf ' %s %s ' "$intent" "$surfaces" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' ' ' | tr -s ' ')
pawl_re=' (shared trunk|push to trunk|push to main|push trunk|push main|push origin|push force|force push|merge to trunk|merge to main|history rewrite|rm rf|reset hard|drop database|drop table|prod database|production database|prod store|shared store|delete|destroy|deploy|publish|post|send|email|external|api call|github|gitlab|forge|pull request|merge request|repoint|canary|accepted|schema|contract|migration|gate|gating|enforcement|pawl|pawls|ci|workflow|yml|yaml|disable alert|silence alert|credential|secret|token|rotate|permission|authz|access control|grant|revoke|role|registry|regenerate|regen|prod|production|spend|quota|paid|billing) '
class="CHAOS"
if printf '%s' "$norm" | grep -Eq "$pawl_re"; then
  class="PAWL"
fi

# Record — mechanical evidence the intake happened. mktemp keeps each run's
# record distinct even when two fire in the same second (BSD date has no %N).
rec_dir=".agents/rpi/intake"
mkdir -p "$rec_dir"
ts=$(date +%Y%m%dT%H%M%S)
# BSD mktemp needs the X's trailing, so make a unique base then add .json.
rec=$(mktemp "$rec_dir/${ts}-XXXXXX")
mv "$rec" "$rec.json"; rec="$rec.json"
python3 - "$ts" "$intent" "$done_test" "$surfaces" "$depth" "$class" > "$rec" <<'PY'
import json, sys
ts, intent, done, surf, depth, cls = sys.argv[1:7]
json.dump({"ts": ts, "intent": intent, "done_test": done,
           "surfaces": surf, "depth": int(depth), "class": cls}, sys.stdout)
sys.stdout.write("\n")
PY

echo "intake: $class — recorded $rec"
if [[ "$class" == "PAWL" ]]; then
  echo "  PAWL (one-way door). Summon the cross-family Navi for the receipt BEFORE the door."
  echo "  fresh-context default; opt up to multi-model only for the highest-irreversibility doors."
else
  echo "  CHAOS (reversible). Solo — just build it. Fresh-context subagent for any review."
  echo "  If this touches shared/irreversible state not detected here, it's a missing pawl — treat as PAWL."
fi

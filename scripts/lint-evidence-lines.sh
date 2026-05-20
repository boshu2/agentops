#!/usr/bin/env bash
# lint-evidence-lines.sh — pre-flight check for PR-body `Evidence:` lines.
#
# Anti-pattern #7 (AP#7, see scripts/verify-gate-claim.sh) requires that each
# `Evidence:` line in a PR body appears verbatim in the workflow's job logs.
# The most common failure mode is parenthetical narration appended to an
# otherwise-correct path:
#
#     Evidence: tests/scripts/foo.bats (10/10 passing)   ← AP#7 will fail
#     Evidence: tests/scripts/foo.bats                    ← AP#7 will pass
#
# CI logs never contain "(10/10 passing)" or similar prose, so the verbatim
# match misses. This script runs the same extraction regex CI uses
# (`sed -n 's/^Evidence:[[:space:]]*//p'`) and flags each extracted claim for
# AP#7-incompatible content with a clear remediation message.
#
# Inputs:
#   <pr-number>       — fetch via `gh pr view <N> --json body`
#   --body <file>     — read body from a local file (testing / pre-push hook)
#   --stdin           — read body from stdin
#
# Flags:
#   --json            — machine-readable output (one record per claim)
#   --strict          — non-zero exit when even advisory issues found
#                       (default: only "blocking" issues — parens, empty,
#                       markdown table — cause non-zero exit)
#
# Exit codes:
#   0 — no blocking issues
#   1 — at least one blocking issue found
#   2 — usage error
#   3 — input fetch failed (gh missing, no body, etc.)
#
# Standalone — no project dependencies. Designed to run pre-push or as an
# advisory CI step before validate-pr-evidence-claims.

set -euo pipefail

MODE="pr"
PR_NUM=""
BODY_FILE=""
JSON=0
STRICT=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --body) shift; MODE="file"; BODY_FILE="${1:-}" ;;
    --stdin) MODE="stdin" ;;
    --json) JSON=1 ;;
    --strict) STRICT=1 ;;
    -h|--help) usage 0 ;;
    --*) echo "lint-evidence-lines: unknown flag: $1" >&2; usage 2 ;;
    *) PR_NUM="$1" ;;
  esac
  shift || true
done

read_body() {
  case "$MODE" in
    pr)
      if [ -z "$PR_NUM" ]; then
        echo "lint-evidence-lines: missing <pr-number>" >&2
        usage 2
      fi
      if ! command -v gh >/dev/null 2>&1; then
        echo "lint-evidence-lines: gh CLI not available" >&2
        exit 3
      fi
      gh pr view "$PR_NUM" --json body --jq .body 2>/dev/null
      ;;
    file)
      if [ -z "$BODY_FILE" ] || [ ! -r "$BODY_FILE" ]; then
        echo "lint-evidence-lines: cannot read body file: $BODY_FILE" >&2
        exit 3
      fi
      cat "$BODY_FILE"
      ;;
    stdin)
      cat -
      ;;
  esac
}

body="$(read_body)"
if [ -z "$body" ]; then
  echo "lint-evidence-lines: empty body" >&2
  exit 3
fi

# Match CI's exact extraction logic so we lint exactly what AP#7 will see.
# `tee` into a tmpfile so we can iterate the claims twice.
claims_file="$(mktemp)"
trap 'rm -f "$claims_file"' EXIT

printf '%s\n' "$body" | sed -n 's/^Evidence:[[:space:]]*//p' > "$claims_file"

if [ ! -s "$claims_file" ]; then
  # No Evidence: lines at all. AP#7 skips in this case — informational only.
  if [ "$JSON" -eq 1 ]; then
    echo '{"claims": [], "blocking": 0, "advisory": 0, "skipped_by_ap7": true}'
  else
    echo "lint-evidence-lines: no Evidence: lines in body — AP#7 will skip"
  fi
  exit 0
fi

# Classify each claim. The detection rules are tuned to the AP#7 failure
# modes we've seen in practice; expand the table as new patterns surface.
declare -a records=()
blocking=0
advisory=0

# Tabs and double-spaces tend to drop from CI log lines, so warn about them.
# Markdown emphasis (`*`, `_`, backtick) breaks verbatim match. Pipe (`|`)
# only matters inside a markdown table cell but is a common copy-paste sin.
classify_claim() {
  local claim="$1"
  local issues=()
  local severity="ok"

  if [ -z "$(printf '%s' "$claim" | tr -d '[:space:]')" ]; then
    issues+=("empty:Evidence: line has no content; AP#7 will fail")
    severity="blocking"
  fi
  if printf '%s' "$claim" | grep -q '([^)]*)'; then
    local paren
    paren="$(printf '%s' "$claim" | grep -oE '\([^)]*\)' | head -1)"
    issues+=("parens:contains parenthetical \"$paren\" — CI logs don't carry prose; strip the parens")
    severity="blocking"
  fi
  if printf '%s' "$claim" | grep -qE '\*\*|__|\*[^*]|`'; then
    issues+=("markdown:contains markdown emphasis/backtick — verbatim match will miss")
    severity="blocking"
  fi
  if printf '%s' "$claim" | grep -q '|'; then
    issues+=("pipe:contains a markdown table separator '|' — verbatim match likely to miss")
    severity="blocking"
  fi
  if printf '%s' "$claim" | grep -qE '[[:space:]]$'; then
    issues+=("trailing-ws:line ends in whitespace — sed extracts it as part of the claim")
    severity="advisory"
  fi
  if printf '%s' "$claim" | grep -qE '^\s*(see|cf|note:)'; then
    issues+=("prose:line opens with prose keyword — likely commentary not a path")
    severity="blocking"
  fi

  if [ "${#issues[@]}" -eq 0 ]; then
    records+=("$(printf '{"claim":%s,"severity":"ok","issues":[]}' "$(json_string "$claim")")")
    return
  fi
  if [ "$severity" = "blocking" ]; then
    blocking=$((blocking + 1))
  else
    advisory=$((advisory + 1))
  fi
  local issues_json
  issues_json="$(printf '%s\n' "${issues[@]}" | jq -R . | jq -sc .)"
  records+=("$(printf '{"claim":%s,"severity":"%s","issues":%s}' \
    "$(json_string "$claim")" "$severity" "$issues_json")")
}

json_string() {
  # Escape a bash string for JSON inclusion.
  printf '%s' "$1" | jq -Rs .
}

while IFS= read -r claim; do
  classify_claim "$claim"
done < "$claims_file"

if [ "$JSON" -eq 1 ]; then
  printf '{"claims":[%s],"blocking":%d,"advisory":%d,"skipped_by_ap7":false}\n' \
    "$(IFS=,; echo "${records[*]}")" \
    "$blocking" "$advisory"
else
  total="$(wc -l < "$claims_file" | tr -d ' ')"
  echo "lint-evidence-lines: $total Evidence: line(s) inspected; $blocking blocking, $advisory advisory"
  i=0
  while IFS= read -r claim; do
    i=$((i + 1))
    rec="${records[i-1]}"
    sev="$(printf '%s' "$rec" | jq -r .severity)"
    case "$sev" in
      ok) echo "  OK   [$i] $claim" ;;
      advisory)
        echo "  WARN [$i] $claim"
        printf '%s' "$rec" | jq -r '.issues[]' | sed 's/^/         /'
        ;;
      blocking)
        echo "  FAIL [$i] $claim"
        printf '%s' "$rec" | jq -r '.issues[]' | sed 's/^/         /'
        ;;
    esac
  done < "$claims_file"
  if [ "$blocking" -gt 0 ] || { [ "$STRICT" -eq 1 ] && [ "$advisory" -gt 0 ]; }; then
    echo
    echo "lint-evidence-lines: see .agents/learnings/2026-05-20-ap7-evidence-line-format.md"
  fi
fi

if [ "$blocking" -gt 0 ]; then
  exit 1
fi
if [ "$STRICT" -eq 1 ] && [ "$advisory" -gt 0 ]; then
  exit 1
fi
exit 0

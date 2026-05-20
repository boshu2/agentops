#!/usr/bin/env bash
# export-session-summary.sh — roll up a session's outcomes into one markdown.
#
# At teardown, the durable artifacts of a session are scattered:
#   - .agents/evolve/cycle-history.jsonl   ← the cycle ledger
#   - .agents/evolve/session-state.json    ← resume state
#   - bd memories added in window          ← persistent learnings
#   - git log in window                    ← commits + merges
#   - gh pr list merged in window          ← shipped PRs
#
# This script produces a single Markdown digest at
# `.agents/evolve/session-summary-<UTC>.md` from those sources. Designed
# for hand-off-to-next-session and for human readout.
#
# Inputs (all optional):
#   --since <ref-or-time>   git ref (e.g. HEAD~50) OR ISO-8601 time
#                            (default: 24h ago)
#   --out <path>            output file path (default: auto-generated)
#   --stdout                emit to stdout (skip file write)
#   --no-bd                 skip bd memories section
#   --no-prs                skip GitHub merged-PR section (no gh call)
#
# Exit codes:
#   0 — wrote a summary file (or printed to stdout)
#   2 — usage error
#   3 — required input missing (no cycle-history.jsonl AND no other source)

set -euo pipefail

SINCE=""
OUT_PATH=""
TO_STDOUT=0
INCLUDE_BD=1
INCLUDE_PRS=1

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --since) shift; SINCE="${1:-}" ;;
    --out) shift; OUT_PATH="${1:-}" ;;
    --stdout) TO_STDOUT=1 ;;
    --no-bd) INCLUDE_BD=0 ;;
    --no-prs) INCLUDE_PRS=0 ;;
    -h|--help) usage 0 ;;
    *) echo "export-session-summary: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

if [ -z "$SINCE" ]; then
  # Default window: 24 hours ago in UTC ISO-8601.
  SINCE="$(date -u -d '24 hours ago' +%FT%TZ 2>/dev/null || date -u -v-24H +%FT%TZ 2>/dev/null || echo "")"
fi

# Resolve git "--since" arg. If $SINCE looks like an ISO timestamp keep it;
# if it looks like a ref, translate to that ref's commit date.
git_since_arg() {
  local s="$1"
  if [ -z "$s" ]; then
    echo ""
    return
  fi
  if git rev-parse --verify --quiet "$s^{commit}" >/dev/null 2>&1; then
    git show --no-patch --format=%cI "$s" 2>/dev/null
  else
    printf '%s' "$s"
  fi
}

NOW_UTC="$(date -u +%FT%TZ)"
DEFAULT_OUT=".agents/evolve/session-summary-$(date -u +%Y%m%dT%H%M%SZ).md"
[ -z "$OUT_PATH" ] && OUT_PATH="$DEFAULT_OUT"

# Build sections into a temp buffer first, then write atomically.
TMP_OUT="$(mktemp)"
trap 'rm -f "$TMP_OUT"' EXIT

section_header() {
  printf '\n## %s\n\n' "$1" >> "$TMP_OUT"
}

# 1. Outcomes — high-level rollup ----------------------------------------
printf '# Session summary — %s\n\n' "$NOW_UTC" > "$TMP_OUT"
printf '*Window:* %s → %s\n\n' "$SINCE" "$NOW_UTC" >> "$TMP_OUT"

section_header "Outcomes"

GIT_SINCE_ARG="$(git_since_arg "$SINCE")"
commit_count=0
if [ -n "$GIT_SINCE_ARG" ]; then
  commit_count="$(git log --since="$GIT_SINCE_ARG" --oneline 2>/dev/null | wc -l | tr -d ' ')"
fi

cycle_count=0
productive_count=0
if [ -r .agents/evolve/cycle-history.jsonl ]; then
  # Filter cycles by their `ts` field. If $SINCE is iso, lexicographic
  # compare works; if it's an empty string, count everything.
  if [ -n "$SINCE" ]; then
    cycle_count="$(jq -cs --arg s "$SINCE" '[.[] | select(.ts != null and .ts >= $s)] | length' \
      .agents/evolve/cycle-history.jsonl 2>/dev/null || echo 0)"
    productive_count="$(jq -cs --arg s "$SINCE" '[.[] | select(.ts != null and .ts >= $s and .result == "productive")] | length' \
      .agents/evolve/cycle-history.jsonl 2>/dev/null || echo 0)"
  else
    cycle_count="$(wc -l < .agents/evolve/cycle-history.jsonl | tr -d ' ')"
    productive_count="$(jq -cs '[.[] | select(.result == "productive")] | length' .agents/evolve/cycle-history.jsonl 2>/dev/null || echo 0)"
  fi
fi

merged_pr_count=0
if [ "$INCLUDE_PRS" -eq 1 ] && command -v gh >/dev/null 2>&1; then
  if [ -n "$GIT_SINCE_ARG" ]; then
    merged_pr_count="$(gh pr list --state merged --limit 200 \
      --json mergedAt --jq "[.[] | select(.mergedAt >= \"$GIT_SINCE_ARG\")] | length" \
      2>/dev/null || echo 0)"
  fi
fi

printf -- '- Commits: **%s**\n' "$commit_count" >> "$TMP_OUT"
printf -- '- Cycles: **%s** total, **%s** productive\n' "$cycle_count" "$productive_count" >> "$TMP_OUT"
printf -- '- PRs merged: **%s**\n' "$merged_pr_count" >> "$TMP_OUT"

# 2. Cycle ledger — compressed ------------------------------------------
if [ -r .agents/evolve/cycle-history.jsonl ] && [ "$cycle_count" -gt 0 ]; then
  section_header "Cycle ledger (compressed)"
  if [ -n "$SINCE" ]; then
    jq -r --arg s "$SINCE" '
      select(.ts != null and .ts >= $s) |
      "- **cycle \(.cycle)** [\(.result // "?")] \(.mode // "?") — \(.notes // "" | .[0:140])"
    ' .agents/evolve/cycle-history.jsonl >> "$TMP_OUT" 2>/dev/null || true
  else
    jq -r '
      "- **cycle \(.cycle)** [\(.result // "?")] \(.mode // "?") — \(.notes // "" | .[0:140])"
    ' .agents/evolve/cycle-history.jsonl >> "$TMP_OUT" 2>/dev/null || true
  fi
fi

# 3. New memories ---------------------------------------------------------
if [ "$INCLUDE_BD" -eq 1 ] && command -v bd >/dev/null 2>&1; then
  section_header "New memories"
  # `bd memories` lists all; we filter by created_at if available, else dump
  # the last ~20 as a coarse window.
  bd_json="$(bd memories --json 2>/dev/null || true)"
  if [ -n "$bd_json" ]; then
    if [ -n "$SINCE" ]; then
      printf '%s' "$bd_json" | jq -r --arg s "$SINCE" '
        try (.[] | select((.created_at // .updated_at // "") >= $s) |
             "- **\(.key)** — \(.content // "" | .[0:160])")
        catch empty
      ' >> "$TMP_OUT" 2>/dev/null || true
    else
      printf '%s' "$bd_json" | jq -r '
        try (sort_by(.created_at // .updated_at // "") | reverse | .[0:20][] |
             "- **\(.key)** — \(.content // "" | .[0:160])")
        catch empty
      ' >> "$TMP_OUT" 2>/dev/null || true
    fi
  fi
fi

# 4. Commits in window ---------------------------------------------------
if [ -n "$GIT_SINCE_ARG" ] && [ "$commit_count" -gt 0 ]; then
  section_header "Commits"
  git log --since="$GIT_SINCE_ARG" --pretty=format:'- `%h` %s' 2>/dev/null >> "$TMP_OUT" || true
  printf '\n' >> "$TMP_OUT"
fi

# 5. Merged PRs in window ------------------------------------------------
if [ "$INCLUDE_PRS" -eq 1 ] && command -v gh >/dev/null 2>&1 \
   && [ -n "$GIT_SINCE_ARG" ] && [ "$merged_pr_count" -gt 0 ]; then
  section_header "Merged PRs"
  gh pr list --state merged --limit 200 \
    --json number,title,mergedAt \
    --jq "[.[] | select(.mergedAt >= \"$GIT_SINCE_ARG\")] | reverse | .[] |
           \"- #\(.number) \(.title)\"" 2>/dev/null >> "$TMP_OUT" || true
fi

# 6. Carry-forward — open in-flight ---------------------------------------
section_header "Carry-forward"

if [ -r .agents/evolve/session-state.json ]; then
  in_flight="$(jq -r '.batch_prs // [] | map(tostring) | join(", ")' .agents/evolve/session-state.json 2>/dev/null || true)"
  goal="$(jq -r '.goal // "(none)"' .agents/evolve/session-state.json 2>/dev/null || echo "(none)")"
  printf -- '- Goal: **%s**\n' "$goal" >> "$TMP_OUT"
  printf -- '- PRs in flight at snapshot: **%s**\n' "${in_flight:-none}" >> "$TMP_OUT"
fi
if command -v bd >/dev/null 2>&1; then
  ready_count="$(bd ready --json 2>/dev/null | jq 'length' 2>/dev/null || echo "?")"
  printf -- '- Open ready beads: **%s**\n' "$ready_count" >> "$TMP_OUT"
fi

# Atomic write or stdout --------------------------------------------------
if [ "$TO_STDOUT" -eq 1 ]; then
  cat "$TMP_OUT"
else
  mkdir -p "$(dirname "$OUT_PATH")"
  mv "$TMP_OUT" "$OUT_PATH"
  # Disable the EXIT trap since we've already moved the file.
  trap - EXIT
  echo "export-session-summary: wrote $OUT_PATH"
fi

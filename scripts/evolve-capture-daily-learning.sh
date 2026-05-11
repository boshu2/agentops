#!/usr/bin/env bash
# evolve-capture-daily-learning.sh
# End-of-day consolidator for the /evolve all-day loop's self-reflection.
#
# Reads:
#   .agents/evolve/daily-learning-log-YYYY-MM-DD.md  (per-cycle micro-capture, append-only)
#   .agents/evolve/cycle-history.jsonl                 (all cycles, filtered to today)
#   .agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md  (prior days, for pattern detection)
#
# Writes:
#   .agents/learnings/YYYY-MM-DD-evolve-loop-learnings.md  (today's consolidated reflection)
#
# Side effects:
#   - If a friction pattern is detected in 2+ daily learning files, files a bd issue under
#     the evolution-roadmap label with the evolve-improvement label.
#
# Idempotent: re-running overwrites today's consolidated file but leaves the daily log alone.
#
# Usage:
#   bash scripts/evolve-capture-daily-learning.sh           # consolidate today
#   bash scripts/evolve-capture-daily-learning.sh --dry-run # preview, no writes
#   bash scripts/evolve-capture-daily-learning.sh DATE      # consolidate explicit YYYY-MM-DD
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

DRY_RUN=false
DATE=""
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) DATE="$arg" ;;
    -h|--help)
      command sed -n '2,/^set -euo/p' "$0" | command head -25
      exit 0
      ;;
  esac
done

[[ -z "$DATE" ]] && DATE="$(date +%Y-%m-%d)"

LOG=".agents/evolve/daily-learning-log-${DATE}.md"
HISTORY=".agents/evolve/cycle-history.jsonl"
OUT=".agents/learnings/${DATE}-evolve-loop-learnings.md"

if [[ ! -f "$LOG" && ! -f "$HISTORY" ]]; then
  echo "no daily log or cycle history; nothing to consolidate for $DATE" >&2
  exit 0
fi

# --- Gather: today's cycles from history ---
TODAY_CYCLES=""
if [[ -f "$HISTORY" ]]; then
  TODAY_CYCLES="$(jq -c --arg d "$DATE" 'select(.started_at|startswith($d))' "$HISTORY" 2>/dev/null || true)"
fi

TOTAL=$(printf '%s\n' "$TODAY_CYCLES" | grep -c . 2>/dev/null || echo 0)
PRODUCTIVE=$(printf '%s\n' "$TODAY_CYCLES" | jq -r 'select(.result=="improved") | .cycle' 2>/dev/null | wc -l | tr -d ' ')
SCOUTS=$(printf '%s\n' "$TODAY_CYCLES" | jq -r 'select(.result=="harvested") | .cycle' 2>/dev/null | wc -l | tr -d ' ')
IDLE=$(printf '%s\n' "$TODAY_CYCLES" | jq -r 'select(.result=="idle" or .result=="unchanged") | .cycle' 2>/dev/null | wc -l | tr -d ' ')
REGRESSED=$(printf '%s\n' "$TODAY_CYCLES" | jq -r 'select(.result=="regressed") | .cycle' 2>/dev/null | wc -l | tr -d ' ')

# --- Gather: micro-capture lines from today's daily log ---
MICRO_LINES=""
if [[ -f "$LOG" ]]; then
  MICRO_LINES="$(command grep -E '^- ' "$LOG" 2>/dev/null || true)"
fi
MICRO_COUNT=$(printf '%s\n' "$MICRO_LINES" | grep -c . 2>/dev/null || echo 0)

# --- Extract friction tags (lines beginning with "- FRICTION:" in daily log) ---
FRICTIONS=""
if [[ -n "$MICRO_LINES" ]]; then
  FRICTIONS="$(printf '%s\n' "$MICRO_LINES" | command grep -iE 'FRICTION:' 2>/dev/null || true)"
fi
FRICTION_COUNT=$(printf '%s\n' "$FRICTIONS" | grep -c . 2>/dev/null || echo 0)

# --- Cross-day pattern detection: scan prior evolve-loop-learnings files for matching friction tags ---
PRIOR_DAYS="$(command find .agents/learnings -maxdepth 1 -name '*-evolve-loop-learnings.md' -not -name "${DATE}*" 2>/dev/null | sort -r | head -7)"
RECURRING=""
if [[ -n "$FRICTIONS" && -n "$PRIOR_DAYS" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    tag="$(echo "$line" | command sed -nE 's/.*FRICTION:[[:space:]]*([A-Za-z0-9-]+).*/\1/p')"
    [[ -z "$tag" ]] && continue
    if echo "$PRIOR_DAYS" | xargs -I {} command grep -l "FRICTION: ${tag}" {} 2>/dev/null | head -1 | grep -q .; then
      RECURRING="${RECURRING}- ${tag} (also seen in prior days)"$'\n'
    fi
  done <<< "$FRICTIONS"
fi

# --- Build the consolidated learning markdown ---
TMP="$(mktemp)"
{
cat <<EOF
---
id: learning-${DATE}-evolve-loop
type: learning
date: ${DATE}
category: process
confidence: high
maturity: provisional
utility: 0.7
source: evolve-loop-self-reflection
---

# Learning: /evolve loop self-reflection — ${DATE}

## Counts

| Metric | Count |
|--------|-------|
| Total cycles | ${TOTAL} |
| Productive (commit landed) | ${PRODUCTIVE} |
| Scout (queue annotated, no commit) | ${SCOUTS} |
| Idle | ${IDLE} |
| Regressed | ${REGRESSED} |
| Micro-captures in daily log | ${MICRO_COUNT} |
| Friction tags this session | ${FRICTION_COUNT} |

## Cycle ledger (today)

EOF

if [[ -n "$TODAY_CYCLES" ]]; then
  printf '%s\n' "$TODAY_CYCLES" | jq -r '"- cycle " + (.cycle|tostring) + " [" + (.result // "?") + "] " + (.work_ref // "") + " — " + (.title // "")[0:90]' 2>/dev/null
else
  echo "_(no cycles found for ${DATE})_"
fi

cat <<EOF

## Per-cycle micro-captures

EOF

if [[ -n "$MICRO_LINES" ]]; then
  printf '%s\n' "$MICRO_LINES"
else
  echo "_(no micro-captures appended to daily log)_"
fi

cat <<EOF

## Frictions observed

EOF

if [[ "$FRICTION_COUNT" -gt 0 ]]; then
  printf '%s\n' "$FRICTIONS"
else
  echo "_(no friction tags this session)_"
fi

cat <<EOF

## Recurring (seen on 2+ days)

EOF

if [[ -n "$RECURRING" ]]; then
  printf '%s' "$RECURRING"
else
  echo "_(no recurring patterns vs prior daily learnings)_"
fi

cat <<EOF

## Promotion candidates

If any line in "Recurring" matches a friction that has appeared on 2+ days, a bead under the \`evolution-roadmap\` label with \`evolve-improvement\` should be filed (or an existing one updated) to fold it into the skill improvement queue. See \`docs/plans/2026-05-11-evolution-roadmap.md\` section LC for the protocol.

## Source

Auto-generated by \`scripts/evolve-capture-daily-learning.sh\` from \`${LOG}\` + \`${HISTORY}\`.
EOF
} > "$TMP"

if [[ "$DRY_RUN" == "true" ]]; then
  echo "=== would write $OUT ==="
  command cat "$TMP"
  rm -f "$TMP"
  exit 0
fi

command mkdir -p "$(dirname "$OUT")"
command mv "$TMP" "$OUT"
echo "wrote: $OUT"

# --- Auto-file evolve-improvement bead for recurring frictions ---
if [[ -n "$RECURRING" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    tag="$(echo "$line" | command sed -nE 's/.*- ([A-Za-z0-9-]+).*/\1/p')"
    [[ -z "$tag" ]] && continue
    title="LC-followup: Address recurring /evolve friction \"${tag}\""
    # Idempotency: skip if a bead with this title already exists open
    if bd list --status=open --json 2>/dev/null | jq -e --arg t "$title" '.[] | select(.title == $t)' >/dev/null 2>&1; then
      echo "skip (open bead exists): $title"
      continue
    fi
    bd create \
      --title="$title" \
      --description="Friction \"${tag}\" appeared in /evolve loop self-reflection on ${DATE} and at least one prior day. Auto-filed by scripts/evolve-capture-daily-learning.sh. Review .agents/learnings/${DATE}-evolve-loop-learnings.md and the matching prior-day file for context. Acceptance: patch skills/evolve/SKILL.md or related script to address the friction; close this bead when the next /evolve session does not surface the same tag." \
      --type=task \
      --priority=2 \
      --labels="evolution-roadmap,evolve-improvement,auto-filed" \
      >/dev/null 2>&1 && echo "filed bead: $title" || echo "FAILED to file: $title"
  done <<< "$RECURRING"
fi

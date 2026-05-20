#!/usr/bin/env bash
# audit-bd-memories.sh — surface duplicate / stale-surface bd memories.
#
# 156+ bd memories as of 2026-05-20. Without curation, recall quality
# degrades (same lesson stored 3 ways; old lessons referencing retired
# surfaces like Ollama / shepherd-cron). This script does NOT delete
# anything; it produces a markdown report at
# `.agents/audits/bd-memories-<YYYY-MM-DD>.md` with three sections:
#
#   NEAR-DUPLICATES         memory pairs with content jaccard >= threshold
#   RETIRED-SURFACE         memories whose body mentions terms in
#                            the retired-surfaces list
#   SUMMARY                 total / candidates-for-review counts
#
# Operator reviews and selectively runs `bd forget <key>`.
#
# Flags:
#   --threshold <0..1>   Jaccard similarity floor for near-duplicates
#                         (default: 0.65)
#   --out <path>         Output markdown path
#                         (default: .agents/audits/bd-memories-<date>.md)
#   --stdout             Emit markdown to stdout (skip file write)
#   --retired <csv>      Override retired-surface keywords list
#   --no-retired         Skip retired-surface section
#   --no-dups            Skip near-duplicate section
#   --json               Machine-readable summary (skips markdown)
#
# Exit codes:
#   0 — audit completed (whether candidates were found or not)
#   2 — usage error
#   3 — bd unavailable or returned no memories

set -euo pipefail

THRESHOLD="0.65"
OUT_PATH=""
TO_STDOUT=0
JSON=0
INCLUDE_DUPS=1
INCLUDE_RETIRED=1
RETIRED_DEFAULT="ollama,shepherd-cron,openclaw,gemma,morai-codex,d:\\\\dream,dreamworker"
RETIRED_LIST="$RETIRED_DEFAULT"

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --threshold) shift; THRESHOLD="${1:-0.65}" ;;
    --out) shift; OUT_PATH="${1:-}" ;;
    --stdout) TO_STDOUT=1 ;;
    --retired) shift; RETIRED_LIST="${1:-}" ;;
    --no-retired) INCLUDE_RETIRED=0 ;;
    --no-dups) INCLUDE_DUPS=0 ;;
    --json) JSON=1 ;;
    -h|--help) usage 0 ;;
    *) echo "audit-bd-memories: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

if ! command -v bd >/dev/null 2>&1; then
  echo "audit-bd-memories: bd CLI not available" >&2
  exit 3
fi

DATE_STR="$(date -u +%Y-%m-%d)"
if [ -z "$OUT_PATH" ]; then
  OUT_PATH=".agents/audits/bd-memories-$DATE_STR.md"
fi

# Step 1: parse `bd memories` into a TSV of "key\tcontent". The format is:
#   Memories (N):                  ← header line
#                                  ← blank
#     <key>                        ← 2-space indent
#       <content snippet>...       ← 4-space indent (may be truncated)
#                                  ← blank between memories
TMP_TSV="$(mktemp)"
# The tokens dir is `$TMP_TSV.tokens`, so use -rf to clean the whole sibling set.
trap 'rm -rf "$TMP_TSV" "$TMP_TSV".*' EXIT

bd memories 2>/dev/null | awk '
  /^Memories \(/ { next }
  /^  [^ ]/ {
    if (key) { print key "\t" content }
    sub(/^  /, ""); key=$0; content=""; next
  }
  /^    / {
    sub(/^    /, "")
    content = (content == "" ? $0 : content " " $0)
    next
  }
  /^$/ { next }
  END { if (key) { print key "\t" content } }
' > "$TMP_TSV"

count=$(wc -l < "$TMP_TSV" | tr -d ' ')
if [ "$count" -eq 0 ]; then
  echo "audit-bd-memories: no memories found" >&2
  exit 3
fi

# Step 2: near-duplicate detection via Jaccard on word-token sets.
# We compute one token-set file per memory under $TMP_TSV.tokens/<n>,
# then walk pairs.
mkdir -p "$TMP_TSV.tokens"
i=0
keys_file="$TMP_TSV.keys"
: > "$keys_file"
while IFS=$'\t' read -r key content; do
  i=$((i + 1))
  printf '%s\n' "$key" >> "$keys_file"
  printf '%s\n' "$content" | tr 'A-Z' 'a-z' | tr -c 'a-z0-9' '\n' \
    | awk 'length($0) >= 3' | sort -u > "$TMP_TSV.tokens/$i"
done < "$TMP_TSV"

# Helper: jaccard A B → prints decimal 0..1 (0 when both empty)
jaccard() {
  local a="$1" b="$2" union inter
  inter="$(comm -12 "$a" "$b" 2>/dev/null | wc -l | tr -d ' ')"
  union="$(cat "$a" "$b" | sort -u | wc -l | tr -d ' ')"
  if [ "$union" -eq 0 ]; then
    echo "0"
  else
    awk -v i="$inter" -v u="$union" 'BEGIN { printf "%.3f", i/u }'
  fi
}

# Collect (key_a, key_b, score) for pairs above threshold.
DUPS_FILE="$TMP_TSV.dups"
: > "$DUPS_FILE"
if [ "$INCLUDE_DUPS" -eq 1 ] && [ "$count" -gt 1 ]; then
  for ((a=1; a<count; a++)); do
    key_a="$(sed -n "${a}p" "$keys_file")"
    for ((b=a+1; b<=count; b++)); do
      score="$(jaccard "$TMP_TSV.tokens/$a" "$TMP_TSV.tokens/$b")"
      # awk for compare so we can compare decimals robustly
      if awk -v s="$score" -v t="$THRESHOLD" 'BEGIN { exit !(s+0 >= t+0) }'; then
        key_b="$(sed -n "${b}p" "$keys_file")"
        printf '%s\t%s\t%s\n' "$score" "$key_a" "$key_b" >> "$DUPS_FILE"
      fi
    done
  done
  # Sort highest-score first.
  sort -r -o "$DUPS_FILE" "$DUPS_FILE"
fi

dup_count="$(wc -l < "$DUPS_FILE" | tr -d ' ')"

# Step 3: retired-surface scan.
RETIRED_FILE="$TMP_TSV.retired"
: > "$RETIRED_FILE"
if [ "$INCLUDE_RETIRED" -eq 1 ] && [ -n "$RETIRED_LIST" ]; then
  # Convert csv to alternation regex.
  pattern="$(printf '%s' "$RETIRED_LIST" | tr ',' '|')"
  while IFS=$'\t' read -r key content; do
    if printf '%s' "$content" | grep -iqE "$pattern"; then
      hit="$(printf '%s' "$content" | grep -ioE "$pattern" | head -1)"
      printf '%s\t%s\n' "$key" "$hit" >> "$RETIRED_FILE"
    fi
  done < "$TMP_TSV"
fi
retired_count="$(wc -l < "$RETIRED_FILE" | tr -d ' ')"

# Step 4: emit output.
if [ "$JSON" -eq 1 ]; then
  printf '{"total":%d,"near_duplicates":%d,"retired_candidates":%d,"threshold":%s}\n' \
    "$count" "$dup_count" "$retired_count" "$THRESHOLD"
  exit 0
fi

emit_markdown() {
  printf '# bd memories audit — %s\n\n' "$DATE_STR"
  printf '*Inspected %d memories. Jaccard threshold: %s.*\n\n' "$count" "$THRESHOLD"
  printf '## Summary\n\n'
  printf -- '- Total memories: **%d**\n' "$count"
  printf -- '- Near-duplicate pairs (>= %s jaccard): **%d**\n' "$THRESHOLD" "$dup_count"
  printf -- '- Retired-surface candidates: **%d**\n' "$retired_count"

  if [ "$INCLUDE_DUPS" -eq 1 ]; then
    printf '\n## Near-duplicates\n\n'
    if [ "$dup_count" -eq 0 ]; then
      printf '*(none)*\n'
    else
      printf '| Score | Key A | Key B |\n'
      printf '|---|---|---|\n'
      awk -F'\t' '{ printf "| %s | `%s` | `%s` |\n", $1, $2, $3 }' "$DUPS_FILE"
    fi
  fi

  if [ "$INCLUDE_RETIRED" -eq 1 ]; then
    printf '\n## Retired-surface candidates\n\n'
    if [ "$retired_count" -eq 0 ]; then
      printf '*(none)*\n'
    else
      printf '*Pattern: %s*\n\n' "$RETIRED_LIST"
      printf '| Key | Matched term |\n'
      printf '|---|---|\n'
      awk -F'\t' '{ printf "| `%s` | %s |\n", $1, $2 }' "$RETIRED_FILE"
    fi
  fi
  printf '\n---\n*Generated by `scripts/audit-bd-memories.sh`. Operator reviews and selectively runs `bd forget <key>`.*\n'
}

if [ "$TO_STDOUT" -eq 1 ]; then
  emit_markdown
else
  mkdir -p "$(dirname "$OUT_PATH")"
  emit_markdown > "$OUT_PATH"
  echo "audit-bd-memories: wrote $OUT_PATH"
  echo "audit-bd-memories: $count memories scanned, $dup_count near-dup pair(s), $retired_count retired-surface match(es)"
fi

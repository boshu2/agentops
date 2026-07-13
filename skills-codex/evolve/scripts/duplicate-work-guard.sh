#!/usr/bin/env bash
# duplicate-work-guard.sh — before the evolve-cron-rpi discovery loop creates a
# tracking bead, check whether an existing OPEN or CLOSED bead already covers the
# same work. Exits 1 (with the matching bead) when a duplicate is found, 0 when
# the candidate is genuinely new.
#
# Why this exists (ag-6jt/ag-2je): a stale phase-1 handoff kept re-seeding beads
# for work already merged or already tracked. The prior guard only matched EXACT
# OPEN-bead titles, so same-surface, different-wording dups slipped through
# (ag-b8m≈ag-jov, ag-6kw≈ag-c2i — 4th recurrence). This guard matches on:
#   1. exact normalized title (lowercased, punctuation-stripped), OR
#   2. significant-token overlap (Jaccard-style: shared / candidate tokens),
# across both open and closed beads.
#
# Usage:  duplicate-work-guard.sh "<candidate bead title>"
# Exit:   0 no match · 1 duplicate found · 2 usage error
# Env:    DUP_GUARD_THRESHOLD (default 0.6) overlap ratio to call it a dup
#         DUP_GUARD_MIN_SHARED (default 3)  min shared significant tokens
#         BD_BIN (default "bd")             bd binary (override for tests)
set -euo pipefail

THRESHOLD="${DUP_GUARD_THRESHOLD:-0.6}"
MIN_SHARED="${DUP_GUARD_MIN_SHARED:-3}"
BD_BIN="${BD_BIN:-bd}"

usage() {
  echo "usage: duplicate-work-guard.sh \"<candidate bead title>\"" >&2
  exit 2
}

[ "$#" -ge 1 ] || usage
candidate="$1"
# Reject empty / whitespace-only titles.
[ -n "${candidate//[[:space:]]/}" ] || usage

# All issues including closed (the dup-seeding failure spans merged/closed work).
beads_json="$("$BD_BIN" list --all --limit 0 --json 2>/dev/null || true)"
[ -n "$beads_json" ] || beads_json='[]'

match="$(
  printf '%s' "$beads_json" | jq -r \
    --arg cand "$candidate" \
    --argjson threshold "$THRESHOLD" \
    --argjson min "$MIN_SHARED" '
    # Normalize a title to a set of significant (>=3 char) tokens.
    def norm: (. // "")
      | ascii_downcase
      | gsub("[^a-z0-9]+"; " ")
      | split(" ")
      | map(select(length >= 3))
      | unique;
    ($cand | norm) as $c
    | ($c | length) as $cn
    | [ .[]
        | . as $b
        | ($b.title | norm) as $t
        | ([ $t[] | select(. as $x | $c | index($x) != null) ] | length) as $shared
        | { id: $b.id, status: $b.status, title: $b.title,
            exact: ($cn > 0 and $t == $c),
            shared: $shared,
            ratio: (if $cn > 0 then ($shared / $cn) else 0 end) } ]
    | map(select(.exact or (.shared >= $min and .ratio >= $threshold)))
    | if length == 0 then empty
      else (max_by(.ratio)) | "DUPLICATE: \(.id) [\(.status)] \(.title)"
      end
'
)"

if [ -n "$match" ]; then
  echo "$match"
  exit 1
fi

echo "OK: no existing work matches \"$candidate\""
exit 0

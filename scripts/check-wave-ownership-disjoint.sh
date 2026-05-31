#!/usr/bin/env bash
set -euo pipefail

# check-wave-ownership-disjoint.sh — Wave PRE-gate: assert node-state ownership is
# pairwise-disjoint BEFORE a parallel wave of agents is spawned (ag-lmdx.6).
#
# WHY: the real convergence invariant (ag-lmdx, Dolt-expert review 2026-05-30) is
# that two agents must NEVER concurrently own the same artifact's STATE — that is a
# genuine Dolt cell conflict on the context_artifact state projection / the
# append-only state_transition log. File-disjointness
# (scripts/check-file-manifest-overlap.sh, an advisory post-merge shadow) is the
# weak textual shadow of this. This gate is the BLOCKING node-ownership form: it
# runs before any worker starts and refuses to launch the wave on any overlap.
#
# INPUT: a wave manifest — a JSON array of slices, each:
#   { "id": "<slice-id>", "subject": "<text>", "owns": ["<node-state-key>", ...] }
# A node-state key names the artifact whose lifecycle state the slice transitions
# (e.g. an artifact_id, optionally "<artifact_id>:<state>"). Two slices that both
# declare ownership of the same key would concurrently transition that artifact's
# state — the conflict this gate blocks.
#
# Read from a file argument or stdin ("-" or no arg).
#
# EXIT CODES:
#   0  ownership is pairwise-disjoint (or nothing to check) — wave may spawn
#   1  overlapping node-state ownership detected — wave is BLOCKED
#   2  usage / unparseable input (missing jq is a soft skip, exit 0)

usage() {
  cat >&2 <<'EOF'
usage: check-wave-ownership-disjoint.sh [WAVE_MANIFEST_JSON | -]

Wave pre-gate: assert each slice's owned node-state set is pairwise-disjoint
before spawning a parallel wave. Reads a JSON array of {id, subject, owns[]}
from the given file or stdin. Exit 0 = disjoint (proceed), 1 = overlap (blocked),
2 = bad usage/input.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

# Requires bash 4+ for associative arrays.
if [[ "${BASH_VERSINFO[0]}" -lt 4 ]]; then
  echo "WARN: bash 4+ required (have ${BASH_VERSION}) — skipping wave ownership pre-gate"
  exit 0
fi

INPUT="${1:--}"
if [[ "$INPUT" == "-" ]]; then
  INPUT="/dev/stdin"
elif [[ ! -e "$INPUT" ]]; then
  echo "ERROR: manifest not found: $INPUT" >&2
  usage
  exit 2
fi

if ! command -v jq &>/dev/null; then
  echo "WARN: jq not found — skipping wave ownership pre-gate"
  exit 0
fi

# Slurp once so stdin is consumed a single time and we can validate structure.
MANIFEST="$(cat "$INPUT")"

# Structure: must be a JSON array. Anything else is an input error (exit 2).
if ! TYPE="$(jq -r 'type' <<<"$MANIFEST" 2>/dev/null)"; then
  echo "ERROR: input is not valid JSON" >&2
  exit 2
fi
if [[ "$TYPE" != "array" ]]; then
  echo "ERROR: wave manifest must be a JSON array of slices (got: $TYPE)" >&2
  exit 2
fi

SLICE_COUNT="$(jq -r 'length' <<<"$MANIFEST")"
if [[ "$SLICE_COUNT" -eq 0 ]]; then
  echo "SKIP: empty wave manifest — no slices to check"
  exit 0
fi

# A slice declaring no owned node-states cannot be proven disjoint. That is a
# planning gap the wave operator must close before launch, so it is a hard fail
# (a wave pre-gate that silently passes un-declared ownership would be useless).
MISSING="$(jq -r '.[] | select(.owns == null or (.owns | length) == 0) | .id // "unknown"' <<<"$MANIFEST")"
if [[ -n "$MISSING" ]]; then
  while IFS= read -r slice_id; do
    echo "BLOCKED: slice $slice_id declares no node-state ownership — cannot prove disjointness"
  done <<<"$MISSING"
  echo "Wave pre-gate FAILED: every slice must declare its owned node-state set"
  exit 1
fi

CONFLICTS=0
declare -A NODE_OWNER

# Emit (slice_id, node-state-key) pairs and detect the first claimant of each key.
# A second claimant of any key is a genuine node-state ownership conflict.
while IFS=$'\t' read -r slice_id node; do
  [[ -z "$node" ]] && continue
  if [[ -n "${NODE_OWNER[$node]:-}" ]]; then
    if [[ "${NODE_OWNER[$node]}" == "$slice_id" ]]; then
      # Same slice listing the same node twice is a duplicate, not a cross-slice
      # conflict — note it but do not block the wave on it.
      echo "WARN: slice $slice_id lists node-state '$node' more than once"
      continue
    fi
    echo "CONFLICT: node-state '$node' owned by both slice ${NODE_OWNER[$node]} and slice $slice_id"
    CONFLICTS=$((CONFLICTS + 1))
  else
    NODE_OWNER["$node"]="$slice_id"
  fi
done < <(jq -r '.[] | .id as $id | .owns[]? | [$id, .] | @tsv' <<<"$MANIFEST")

if [[ $CONFLICTS -gt 0 ]]; then
  echo "Wave pre-gate FAILED: $CONFLICTS overlapping node-state ownership conflict(s) — wave NOT spawned"
  exit 1
fi

echo "Wave pre-gate PASSED: $SLICE_COUNT slice(s) own pairwise-disjoint node-state sets — wave may spawn"
exit 0

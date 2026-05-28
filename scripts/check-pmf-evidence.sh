#!/usr/bin/env bash
# check-pmf-evidence.sh — gate for PMF / productivity claims in public docs.
#
# Public docs (PRODUCT.md, README.md, docs/launch/*) cannot cite ONLY
# .agents/ paths to back a measurable claim — .agents/ is gitignored so the
# evidence is not reachable by anyone reading the doc. Use
# scripts/export-evidence.sh to promote the artifact to docs/evidence/<bead>/
# first, then cite the tracked path.
#
# This script scans a target file (default PRODUCT.md) for cite-shaped lines
# and flags any that reference only .agents/ paths without also referencing
# a tracked docs/evidence/ promotion.
#
# A "cite-shaped line" is heuristic — any line containing a path of the form
#   .agents/<...>.<ext>
# or wrapped in backticks/brackets that names a `.agents/` artifact. We
# tolerate `.agents/` mentions inside fenced code blocks marked
# `<!-- internal -->` so we don't lint our own commentary about the rule.
#
# Usage:
#   scripts/check-pmf-evidence.sh                # default targets
#   scripts/check-pmf-evidence.sh PRODUCT.md     # explicit target
#   scripts/check-pmf-evidence.sh --json file    # machine-readable
#   scripts/check-pmf-evidence.sh --list         # list violations across defaults
#
# Exit codes:
#   0 — clean (no .agents/-only citations in scanned files)
#   1 — at least one violation found
#   2 — usage error
#   3 — target file not readable

set -euo pipefail

JSON=0
LIST=0
declare -a TARGETS=()

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --json) JSON=1 ;;
    --list) LIST=1 ;;
    -h|--help) usage 0 ;;
    --*) echo "check-pmf-evidence: unknown flag: $1" >&2; usage 2 ;;
    *) TARGETS+=("$1") ;;
  esac
  shift || true
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
if [ "${#TARGETS[@]}" -eq 0 ]; then
  # Default scan set: top-level public docs + launch artifacts.
  TARGETS=("$ROOT/PRODUCT.md" "$ROOT/README.md")
  while IFS= read -r f; do
    TARGETS+=("$f")
  done < <(find "$ROOT/docs/launch" -maxdepth 2 -type f -name '*.md' 2>/dev/null || true)
fi

violations_file="$(mktemp)"
trap 'rm -f "$violations_file"' EXIT
violation_count=0

scan_one() {
  local f="$1"
  if [ ! -r "$f" ]; then
    if [ "$LIST" -eq 0 ] && [ "${#TARGETS[@]}" -eq 1 ]; then
      # Explicit single-target read failure is a hard error.
      echo "check-pmf-evidence: target not readable: $f" >&2
      return 3
    fi
    return 0
  fi

  # Find lines that mention .agents/ paths.
  local lineno=0
  while IFS= read -r line; do
    lineno=$((lineno + 1))
    # Skip lines inside the documented internal-comment shape.
    if printf '%s' "$line" | grep -qE '<!--[[:space:]]*internal[[:space:]]*-->'; then
      continue
    fi
    if printf '%s' "$line" | grep -qE '\.agents/[A-Za-z0-9._/-]+'; then
      # Does the same file also promote this artifact via docs/evidence/?
      # We require a docs/evidence/ mention SOMEWHERE in the file (not
      # necessarily the same line) — heuristic but easy to satisfy.
      if ! grep -qE 'docs/evidence/[A-Za-z0-9._/-]+' "$f"; then
        printf '%s\t%d\t%s\n' "$f" "$lineno" "$line" >> "$violations_file"
        violation_count=$((violation_count + 1))
      fi
    fi
  done < "$f"
}

# Hard error: a single explicit target that's not readable should NOT be
# silently skipped. Detect this case before entering the per-target loop
# (the `|| true` inside the loop would otherwise swallow `return 3`).
if [ "${#TARGETS[@]}" -eq 1 ] && [ "$LIST" -eq 0 ] && [ ! -r "${TARGETS[0]}" ]; then
  echo "check-pmf-evidence: target not readable: ${TARGETS[0]}" >&2
  exit 3
fi

for f in "${TARGETS[@]}"; do
  scan_one "$f" || true
done

# Recount from the file — the in-function counter doesn't always survive
# `set -euo pipefail` + the function-call boundary across all shells, and
# counting wc -l of the appended-to file is the authoritative source.
if [ -s "$violations_file" ]; then
  violation_count="$(wc -l < "$violations_file" | tr -d ' ')"
else
  violation_count=0
fi

if [ "$JSON" -eq 1 ]; then
  if [ -s "$violations_file" ]; then
    jq -Rn --slurpfile vs <(jq -Rn 'inputs | split("\t") | {file: .[0], line: (.[1]|tonumber), text: .[2]}' < "$violations_file" 2>/dev/null) \
       '{violations: ($vs[0] // []), count: ($vs[0] | length)}' 2>/dev/null || \
       printf '{"count": %d}\n' "$violation_count"
  else
    printf '{"violations":[],"count":0}\n'
  fi
else
  if [ -s "$violations_file" ]; then
    echo "check-pmf-evidence: FAIL — $violation_count .agents/-only citation(s) in public docs:"
    awk -F'\t' '{ printf "  %s:%d\n    %s\n", $1, $2, $3 }' "$violations_file"
    echo
    echo "Fix: promote each .agents/ artifact via scripts/export-evidence.sh,"
    echo "then cite the tracked docs/evidence/<bead-id>/<file> path instead."
  else
    if [ "$LIST" -eq 1 ] || [ "${#TARGETS[@]}" -gt 1 ]; then
      echo "check-pmf-evidence: OK — ${#TARGETS[@]} file(s) scanned, 0 violations"
    else
      echo "check-pmf-evidence: OK — ${TARGETS[0]} clean"
    fi
  fi
fi

if [ "$violation_count" -gt 0 ]; then
  exit 1
fi
exit 0

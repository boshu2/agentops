#!/usr/bin/env bash
# check-docs-duplicates.sh
#
# Anti-regrowth gate (age-gate-the-ungated-egwt.12). A docs-lifecycle sweep found
# docs/workflows/meta-observer/pattern-guide.md was a byte-identical copy of
# docs/workflows/meta-observer-pattern.md (556 lines). One was deleted and the
# inbound links repointed. This gate keeps a byte-identical duplicate from ever
# growing back: it shasums every LIVE doc and FAILS, naming BOTH files, on any
# byte-identical pair where the file exceeds the size threshold below.
#
# Why a size floor: tiny docs (a one-line stub, an empty section placeholder, a
# short redirect) can be legitimately identical without being a maintenance
# hazard. The concern is a large document duplicated wholesale — two copies of a
# 500-line guide that will drift out of sync. MIN_DUP_LINES sets that floor: a
# pair is only reported when BOTH files have MORE than this many lines. It is a
# named variable, documented here, so the threshold is one obvious knob.
#
# Scope = the shared LIVE-doc set (scripts/lib/docs-scope.sh: docs/**/*.md minus
# dated/historical-archive dirs). ADRs and archive trees are never in scope, so
# an intentional historical snapshot that happens to match a live doc is exempt
# by construction.
#
# Exit: 0 clean (no byte-identical over-threshold pair) · 1 duplicate pair(s)
#       found · 2 usage/setup error.

# Strict mode + hijack-proof REPO_ROOT come from the shared hardened preamble
# (scripts/lib/preamble.sh — the egwt.10 ratchet). `CDPATH=` is an intentional
# env-prefix (clears CDPATH for that one cd), not an assignment — hence SC1007.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

# Shared LIVE-doc scope resolution — resolved via the preamble's absolute
# REPO_ROOT before any cd (a relative ${BASH_SOURCE[0]} would resolve wrongly
# after the cd below; the .1 lane's pawl caught that on a sibling gate).
# shellcheck source=scripts/lib/docs-scope.sh
. "$REPO_ROOT/scripts/lib/docs-scope.sh"
# Pin the docs scope root to THIS repo (the DOCS_ROOT env seam is for lib tests).
# shellcheck disable=SC2034 # DOCS_ROOT is read by the sourced docs-scope.sh lib.
DOCS_ROOT="$REPO_ROOT"

cd "$REPO_ROOT" || exit 2

# ---- threshold -------------------------------------------------------------
# A byte-identical pair is only reported when BOTH files have MORE than this many
# lines. ~50 lines: below it, an incidental short-doc match is not a duplication
# hazard worth blocking on.
MIN_DUP_LINES="${MIN_DUP_LINES:-50}"

# ---- scan ------------------------------------------------------------------
mapfile -t DOCS < <(docs_scope_live_files)

# Bucket live docs by content hash; only files over the line threshold are
# eligible to form a reported pair.
declare -A HASH_TO_FILES=()
declare -i scanned=0 eligible=0
for f in "${DOCS[@]}"; do
  scanned=$((scanned + 1))
  [ -r "$f" ] || continue
  # Line count gate: skip small docs entirely (they can never form a reported
  # pair, so no need to hash them).
  lines="$(wc -l < "$f" | tr -d ' ')"
  [ "$lines" -gt "$MIN_DUP_LINES" ] || continue
  eligible=$((eligible + 1))
  h="$(shasum "$f" | awk '{print $1}')"
  if [ -n "${HASH_TO_FILES[$h]:-}" ]; then
    HASH_TO_FILES["$h"]="${HASH_TO_FILES[$h]}"$'\n'"$f"
  else
    HASH_TO_FILES["$h"]="$f"
  fi
done

# ---- report ----------------------------------------------------------------
dup_pairs=()
for h in "${!HASH_TO_FILES[@]}"; do
  # A hash bucket with >1 file is a byte-identical set.
  files="${HASH_TO_FILES[$h]}"
  count="$(printf '%s\n' "$files" | grep -c .)"
  if [ "$count" -gt 1 ]; then
    # Emit every unordered pair in the bucket, naming both files.
    mapfile -t members < <(printf '%s\n' "$files")
    n="${#members[@]}"
    for ((i = 0; i < n; i++)); do
      for ((j = i + 1; j < n; j++)); do
        dup_pairs+=("${members[i]} == ${members[j]}")
      done
    done
  fi
done

echo "check-docs-duplicates: scanned $scanned live docs ($eligible over ${MIN_DUP_LINES}-line threshold)"

if [ "${#dup_pairs[@]}" -gt 0 ]; then
  echo "FAIL — ${#dup_pairs[@]} byte-identical live-doc pair(s) over the ${MIN_DUP_LINES}-line threshold:" >&2
  for p in "${dup_pairs[@]}"; do
    echo "  - $p" >&2
  done
  echo >&2
  echo "Fix: delete one copy and repoint every inbound link to the survivor" >&2
  echo "     (rg -n '<basename>' to find them). A large doc must not exist twice." >&2
  exit 1
fi

echo "PASS — no byte-identical live-doc pair over the ${MIN_DUP_LINES}-line threshold."
exit 0

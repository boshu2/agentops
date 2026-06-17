#!/usr/bin/env bash
# check-provenance-merge-sha.sh — mesh join guard: merge_sha must resolve on trunk (age-0tn).
#
# Exit codes:
#   0 = all merge_sha values are ancestors of origin/main (or no merge_sha rows)
#   1 = --strict and one or more merge_shas are not on trunk
#   2 = script error
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${PROVENANCE_LEDGER:-$REPO_ROOT/docs/provenance/ledger.jsonl}"
TRUNK="${AGENTOPS_PROVENANCE_TRUNK_REF:-origin/main}"
STRICT="${AGENTOPS_PROVENANCE_MERGE_SHA_STRICT:-0}"

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    --trunk) TRUNK="${2:-}"; shift ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-provenance-merge-sha: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ ! -f "$LEDGER" ]; then
  echo "check-provenance-merge-sha: FAIL — ledger missing at $LEDGER" >&2
  exit 2
fi

if ! git rev-parse --verify --quiet "$TRUNK" >/dev/null 2>&1; then
  echo "check-provenance-merge-sha: WARN — trunk ref $TRUNK not resolvable; skipping mesh check"
  exit 0
fi

bad=0
while IFS= read -r sha; do
  [ -z "$sha" ] && continue
  if git merge-base --is-ancestor "$sha" "$TRUNK" 2>/dev/null; then
    continue
  fi
  echo "check-provenance-merge-sha: merge_sha $sha is NOT on trunk $TRUNK" >&2
  bad=$((bad + 1))
done < <(jq -r 'select(.merge_sha != null and (.merge_sha | length) > 0) | .merge_sha' "$LEDGER" 2>/dev/null | sort -u)

if [ "$bad" -eq 0 ]; then
  echo "check-provenance-merge-sha: PASS (all merge_shas on $TRUNK)"
  exit 0
fi

if [ "$STRICT" = "1" ]; then
  echo "check-provenance-merge-sha: FAIL — $bad merge_sha(s) off trunk; run scripts/post-land-provenance-emit.sh after push" >&2
  exit 1
fi

echo "check-provenance-merge-sha: WARN — $bad merge_sha(s) off trunk (pre-age-0tn historical rows may remain until re-emitted)"
exit 0

#!/usr/bin/env bash
# Reject known private paths in outgoing Git history and the staged index.
# This is a path guard, not a content classifier or a deletion/cleanup guard.
# Native BD data/exports/local routing and the preserved legacy estate stay
# private, including .beads directories below the root. Only the existing root
# .beads/identity.toml may be publicly versioned.
set -euo pipefail

BASE_REF="${CORPUS_PATH_GUARD_BASE:-origin/main}"
base=""
if git rev-parse --verify -q "$BASE_REF" >/dev/null 2>&1; then
  base="$(git merge-base "$BASE_REF" HEAD 2>/dev/null || true)"
fi

# Keep paths NUL-delimited throughout: Git quotes newlines, tabs, quotes and
# other bytes in line-oriented output, which can hide an anchored private path.
paths_file="$(mktemp "${TMPDIR:-/tmp}/agentops-corpus-paths.XXXXXX")"
trap 'rm -f "$paths_file"' EXIT

# Inspect every outgoing commit, not merely the endpoint diff: adding a secret
# and deleting it again still sends the introducing commit. Without a usable
# base, conservatively inspect all locally reachable HEAD history. Include merge
# resolutions and disable rename detection so a moved private file is seen as
# an addition. Pure deletion of an already-public path adds no private bytes.
history_ref="HEAD"
if [ -n "$base" ]; then
  history_ref="$base..HEAD"
fi
if ! git log --format= --name-only -z --full-history -m --no-renames \
  --diff-filter=ACMT "$history_ref" -- > "$paths_file"; then
  echo "check-corpus-path-guard: FAIL — cannot read committed history" >&2
  exit 2
fi
if ! git diff --cached --name-only -z --no-renames --diff-filter=ACMT -- >> "$paths_file"; then
  echo "check-corpus-path-guard: FAIL — cannot read staged paths" >&2
  exit 2
fi

count=0
failed=0
while IFS= read -r -d '' path; do
  [ -n "$path" ] || continue
  count=$((count + 1))
  case "$path" in
    .beads/identity.toml) continue ;;
    .agents/learnings/*|_beads/*|.beads|.beads/*|*/.beads|*/.beads/*|docs/wiki/*)
      if [ "$failed" -eq 0 ]; then
        echo "check-corpus-path-guard: FAIL — private artifact path(s) heading to the PUBLIC repo:" >&2
      fi
      failed=1
      printf '  forbidden: %q\n' "$path" >&2
      ;;
  esac
done < "$paths_file"

if [ "$failed" -ne 0 ]; then
  echo "  fix: remove private data from the outgoing history/index, not only the working tree." >&2
  echo "       Keep BD data/exports/local routing, legacy tracker data and private learnings private." >&2
  echo "       Only .beads/identity.toml is exempt; docs/wiki/ remains blocked pending traceability." >&2
  exit 1
fi

echo "check-corpus-path-guard: ok (${count} path observations scanned, no private artifacts)"

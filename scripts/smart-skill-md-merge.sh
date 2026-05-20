#!/usr/bin/env bash
# smart-skill-md-merge.sh — auto-resolve "## Reference Documents" list-item
# conflicts during rebase by keeping both sides.
#
# Pattern: when both branches add distinct list items inside a Reference
# Documents (or similar additive list) block, the merge is strictly
# additive and safe to auto-resolve. This script detects that exact
# conflict shape and resolves it; bails on any other conflict.
#
# Derivation: cycles 223 + 339 of 2026-05-20 session — parallel skill-ref
# PRs all touched the same `## Reference Documents` block in their parent
# skill's SKILL.md, producing identical-shape conflicts. Initial blind
# `git checkout --ours` script silently dropped one PR's content. This
# script handles the pattern explicitly.
#
# Bead: soc-trhs
#
# Usage:
#   smart-skill-md-merge.sh <file>             # resolve in place
#   smart-skill-md-merge.sh --dry-run <file>   # print what would change
#   smart-skill-md-merge.sh --check <file>     # exit 0 if resolvable, 1 if not

set -euo pipefail

DRY_RUN=0
CHECK_ONLY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --check)   CHECK_ONLY=1; shift ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) break ;;
  esac
done

if [[ $# -ne 1 ]]; then
  echo "usage: $0 [--dry-run|--check] <file>" >&2
  exit 2
fi

FILE="$1"

if [[ ! -f "$FILE" ]]; then
  echo "smart-skill-md-merge: not a file: $FILE" >&2
  exit 2
fi

# Only operate on markdown files (avoid accidental damage to code)
case "$FILE" in
  *.md|*.MD) ;;
  *) echo "smart-skill-md-merge: refusing to touch non-markdown file: $FILE" >&2; exit 2 ;;
esac

# Scan for conflict markers; verify we can resolve every one
RESOLVED=$(awk '
  BEGIN { in_conflict = 0; safe = 1; out = ""; resolved = 0 }

  # Start of conflict
  /^<<<<<<< / {
    in_conflict = 1
    side = "ours"
    ours_lines = ""
    theirs_lines = ""
    next
  }

  /^=======$/ && in_conflict {
    side = "theirs"
    next
  }

  /^>>>>>>> / && in_conflict {
    # End of conflict — decide if it is safe to merge
    # Safe iff: every non-empty line on BOTH sides is a markdown list item
    # (starts with -, +, or *, optionally indented)
    ok = 1

    n = split(ours_lines, a_ours, "\n")
    for (i = 1; i <= n; i++) {
      L = a_ours[i]
      if (L == "") continue
      if (L !~ /^[[:space:]]*[-+*][[:space:]]/) { ok = 0; break }
    }
    if (ok) {
      m = split(theirs_lines, a_theirs, "\n")
      for (i = 1; i <= m; i++) {
        L = a_theirs[i]
        if (L == "") continue
        if (L !~ /^[[:space:]]*[-+*][[:space:]]/) { ok = 0; break }
      }
    }

    if (!ok) {
      safe = 0
      # Re-emit the conflict markers verbatim so caller sees the file is unchanged
      out = out "<<<<<<< OURS_DUMMY\n" ours_lines "=======\n" theirs_lines ">>>>>>> THEIRS_DUMMY\n"
      in_conflict = 0
      ours_lines = ""
      theirs_lines = ""
      next
    }

    # Safe — emit OURS lines then THEIRS lines, dedup
    seen_keys = ""
    n = split(ours_lines, a_ours, "\n")
    for (i = 1; i <= n; i++) {
      L = a_ours[i]
      if (L == "") continue
      key = "[" L "]"
      if (index(seen_keys, key) == 0) {
        out = out L "\n"
        seen_keys = seen_keys " " key
      }
    }
    m = split(theirs_lines, a_theirs, "\n")
    for (i = 1; i <= m; i++) {
      L = a_theirs[i]
      if (L == "") continue
      key = "[" L "]"
      if (index(seen_keys, key) == 0) {
        out = out L "\n"
        seen_keys = seen_keys " " key
      }
    }
    resolved++
    in_conflict = 0
    ours_lines = ""
    theirs_lines = ""
    next
  }

  in_conflict && side == "ours"   { ours_lines = ours_lines $0 "\n"; next }
  in_conflict && side == "theirs" { theirs_lines = theirs_lines $0 "\n"; next }

  { out = out $0 "\n" }

  END {
    # Trim final extra newline AWK adds
    sub(/\n$/, "", out)
    if (!safe) { exit 1 }
    if (resolved == 0) { exit 2 }
    printf "%s", out
  }
' "$FILE")
RC=$?

case "$RC" in
  0)
    if [[ "$CHECK_ONLY" -eq 1 ]]; then
      echo "smart-skill-md-merge: resolvable: $FILE"
      exit 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
      echo "smart-skill-md-merge: would resolve $FILE"
      echo "--- resolved content ---"
      echo "$RESOLVED"
      echo "--- end ---"
      exit 0
    fi
    printf '%s\n' "$RESOLVED" > "$FILE"
    echo "smart-skill-md-merge: resolved $FILE"
    exit 0
    ;;
  1)
    echo "smart-skill-md-merge: NOT auto-resolvable (non-list conflict content): $FILE" >&2
    exit 1
    ;;
  2)
    echo "smart-skill-md-merge: no conflicts found in $FILE" >&2
    exit 2
    ;;
  *)
    echo "smart-skill-md-merge: unexpected awk exit $RC" >&2
    exit "$RC"
    ;;
esac

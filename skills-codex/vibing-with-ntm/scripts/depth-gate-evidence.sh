#!/usr/bin/env bash
# depth-gate-evidence.sh — Generate the 3-part evidence an OC-034 depth-gate requires
#
# Usage: depth-gate-evidence.sh <scope_path> [verify_cmd]
# Example: depth-gate-evidence.sh crates/foo 'cargo test --lib --package foo'
#
# Emits:
#   1. grep counts (unwrap/todo/unimplemented/panic) + 3 hottest files
#   2. Top-3 hottest-file function signatures (first 3 fn lines each)
#   3. Last 20 lines of the verify-command output
#
# Pipe this into a paste-ready review reply or save to REVIEW_EVIDENCE.md.
# Per OC-034 (Depth-Gate Prompts) + PROMPTS.md Depth-Gate Prompt.

set -u
SCOPE="${1:?usage: depth-gate-evidence.sh <scope_path> [verify_cmd]}"
VERIFY_CMD="${2:-}"

hr() { printf '\n── %s ──\n' "$*"; }

hr "1. Risk-surface counts (unwrap/todo/unimplemented/panic) in $SCOPE"
for PAT in 'unwrap\(\)' 'todo!' 'unimplemented!' 'panic!'; do
  COUNT=$(grep -rEnc "$PAT" "$SCOPE" 2>/dev/null | awk -F: '{sum+=$2} END {print sum+0}')
  printf '  %-18s = %d\n' "$PAT" "$COUNT"
done

hr "Top-3 hottest files by combined match count"
grep -rEn 'unwrap\(\)|todo!|unimplemented!|panic!' "$SCOPE" 2>/dev/null \
  | awk -F: '{print $1}' | sort | uniq -c | sort -rn | head -3 \
  | while read -r N FILE; do
    echo "  $N matches  $FILE"
  done

hr "2. Function signatures from top-3 hottest files (first 3 fn signatures each)"
grep -rEn 'unwrap\(\)|todo!|unimplemented!|panic!' "$SCOPE" 2>/dev/null \
  | awk -F: '{print $1}' | sort | uniq -c | sort -rn | head -3 \
  | awk '{print $2}' \
  | while read -r FILE; do
    echo ""
    echo "--- $FILE ---"
    # Grab first 3 lines that look like language-specific function signatures
    grep -nE '^(pub |async |fn |func |def |function )' "$FILE" 2>/dev/null | head -3 \
      || echo "  (no obvious function signatures found — inspect manually)"
  done

if [ -n "$VERIFY_CMD" ]; then
  hr "3. Verify command output (last 20 lines)"
  echo "  \$ $VERIFY_CMD"
  sh -c "$VERIFY_CMD" 2>&1 | tail -20
else
  hr "3. Verify command (none given — run the repo's test command for this scope manually)"
  echo "  (Pass verify command as 2nd arg, e.g.: depth-gate-evidence.sh <scope> 'cargo test --lib -p <crate>')"
fi

echo
echo "─────────────────────────────────────────────────────────────"
echo "Feed this output into the Depth-Gate Prompt (PROMPTS.md) to verify a 'clean' claim."

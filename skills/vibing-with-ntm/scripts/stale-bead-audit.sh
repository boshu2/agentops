#!/usr/bin/env bash
# stale-bead-audit.sh — list in_progress beads whose work already shipped
#
# Usage: stale-bead-audit.sh [repo_path]
#
# For every bead with status=in_progress, checks git log for a commit whose
# message references the bead ID. Prints:
#   - SHIPPED: bead <id> → commit <sha>   (safe to close with `br close`)
#   - STALE:   bead <id> (no referencing commit)
#   - ORPHAN:  `br show` disagrees with `br list` (DB/JSONL drift — human intervention)
#
# Per OC-033 (adaptive stalled-bead threshold) + RECOVERY.md "Stale In-Progress Beads".
# PRINTS ONLY — does not mutate bead state. Feed into br close manually.

set -u
REPO="${1:-$PWD}"

# Handle both old ({issues:[...]}) and new ([...] at top) JSON shapes (AP-53).
IPS=$(br list --status=in_progress --json 2>/dev/null \
  | jq -r 'if type=="object" then .issues else . end | .[].id')

if [ -z "$IPS" ]; then
  echo "No in_progress beads."
  exit 0
fi

printf '%-18s %-9s %s\n' "BEAD" "VERDICT" "EVIDENCE"
printf '%s\n' "───────────────────────────────────────────────────────────────────────────"

for ID in $IPS; do
  # Orphan check — br show should work if br list reports it
  if ! br show "$ID" >/dev/null 2>&1; then
    printf '%-18s %-9s %s\n' "$ID" "ORPHAN" "br show fails despite br list — see fixing-beads-problems skill"
    continue
  fi

  SHA=$(git -C "$REPO" log --all --grep="$ID" --format='%h' 2>/dev/null | head -1)
  if [ -n "$SHA" ]; then
    MSG=$(git -C "$REPO" log -1 --format='%s' "$SHA" 2>/dev/null | head -c 60)
    printf '%-18s %-9s commit %s  "%s"\n' "$ID" "SHIPPED" "$SHA" "$MSG"
  else
    UPDATED=$(br show "$ID" --json 2>/dev/null | jq -r '.updated_at // "?"')
    printf '%-18s %-9s no referencing commit (last updated %s)\n' "$ID" "STALE" "$UPDATED"
  fi
done

echo
echo "Next steps:"
echo "  • SHIPPED → br close <id> --reason 'Shipped in <sha>'"
echo "  • STALE   → verify with assignee; if abandoned, br update <id> --status=open"
echo "  • ORPHAN  → /fixing-beads-problems (DB/JSONL drift recovery)"

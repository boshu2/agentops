#!/usr/bin/env bash
# check-docs-no-retired-tech.sh
#
# Ratchet from the 2026-06-17 docs staleness sweep (epic
# age-docs-staleness-remediation-cko). Fails if a LIVE doc reintroduces a
# command/phrase for a retired subsystem:
#   - bd/Dolt tracker      → use `BEADS_DIR=$PWD/_beads br <cmd>`
#   - Gas City / gastown   → out-of-session substrate is NTM + MCP Agent Mail
#   - agentopsd / in-repo daemon (ADR-0009 deleted it)
#   - runtime=gc / gc bridge (removed from the CLI)
#   - `ao init --hooks` / hook-install (3.0 is hookless)
#   - "branch protection blocks" / "CI is the authoritative gate" (push-to-main;
#     the local pre-push Go gate is the routine release authority)
#
# A doc opts OUT (is historical by design) by ANY of:
#   - a RETIRED / HISTORICAL / SUPERSEDED banner in its first 15 lines
#   - living under docs/adr/ or a dated-archive dir
#   - a *-migration / *-retirement / *-sunset / *-closeout filename, or CHANGELOG
#
# Patterns are command/phrase-precise (not bare words) to avoid flagging prose
# like "ships no daemon" or "gc" inside other words.
#
# Exit: 0 clean · 1 offender(s) found · 2 usage/setup error

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Live-doc scope: exclude dated/historical archives (same set as the sweep).
mapfile -t DOCS < <(find docs -name '*.md' \
  -not -path 'docs/adr/*' \
  -not -path 'docs/audits/*' -not -path 'docs/plans/*' -not -path 'docs/brainstorms/*' \
  -not -path 'docs/council*/*' -not -path 'docs/handoffs/*' -not -path 'docs/learnings/*' \
  -not -path 'docs/evidence/*' -not -path 'docs/releases/*' -not -path 'docs/convergence/*' \
  -not -path 'docs/rescope/*' -not -path 'docs/reduction/*' -not -path 'docs/migration-trackers/*' \
  -not -path 'docs/sovereignty-proof/*' -not -path 'docs/rfcs/*' -not -path 'docs/code-map/*' \
  | sort)

# Command/phrase-precise live-staleness patterns.
PATTERN='\bbd (ready|list|show|update|close|create|dep|vc|dolt|ping|context|doctor|init|sync|merge-slot)\b|pip install beads|brew upgrade beads|\bgt sling\b|gas[ -]?city|gastown|agentopsd|runtime=gc|ao init --hooks|branch protection blocks|CI is the authoritative gate'

# Doc-type exemptions: migration / upgrade / retirement records and the catalog
# index legitimately name retired subsystems while describing the move off them.
is_exempt() {
  local f="$1"
  case "$f" in
    *-migration*|*-retirement*|*-sunset*|*-closeout*|*CHANGELOG*) return 0 ;;
    *MIGRATION*|*UPGRADING*|*documentation-index*) return 0 ;;
  esac
  # self-declared historical banner in the first 15 lines
  if head -n 15 "$f" | grep -qiE 'RETIRED|HISTORICAL|SUPERSEDED'; then
    return 0
  fi
  return 1
}

# A matched line that ALSO carries removal/past-tense language is DESCRIBING the
# retirement, not prescribing the retired tool — not an offender.
REMOVAL_LANG='[Rr]emoved|[Rr]etired|[Dd]eleted|[Dd]eprecat|[Ss]uperseded|no longer|is gone|are gone|that procedure is gone|not a (selectable|valid)|NOT a selectable|rejected by|was \*\*removed'

declare -i scanned=0 exempt=0
offenders=()
for f in "${DOCS[@]}"; do
  scanned=$((scanned + 1))
  if is_exempt "$f"; then exempt=$((exempt + 1)); continue; fi
  if hits=$(grep -nEi "$PATTERN" "$f" 2>/dev/null); then
    while IFS= read -r line; do
      # skip lines that describe the removal rather than prescribe the tool
      printf '%s' "$line" | grep -qE "$REMOVAL_LANG" && continue
      offenders+=("$f:$line")
    done <<< "$hits"
  fi
done

echo "check-docs-no-retired-tech: scanned $scanned live docs ($exempt exempt as historical)"
if [ "${#offenders[@]}" -eq 0 ]; then
  echo "PASS — no live doc reintroduces a retired-subsystem command/phrase."
  exit 0
fi

echo "FAIL — ${#offenders[@]} retired-tech reference(s) in live docs:" >&2
printf '  - %s\n' "${offenders[@]}" >&2
echo >&2
echo "Fix: convert to current truth, or add a 'RETIRED/HISTORICAL' banner in the first 15 lines if the doc is intentionally historical." >&2
exit 1

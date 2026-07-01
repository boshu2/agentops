#!/usr/bin/env bash
# check-docs-no-retired-tech.sh
#
# Ratchet from the 2026-06-17 docs staleness sweep (epic
# age-docs-staleness-remediation-cko). Fails if a LIVE doc reintroduces a
# command/phrase for a retired subsystem:
#   - bd/Dolt tracker      → use `BEADS_DIR="$(ao beads dir)" br <cmd>`
#   - worktree-local beads → use `ao beads dir`, never `$PWD/_beads` or `git -C _beads`
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

# Shared LIVE-doc scope + historical-exemption resolution (age-gate-the-ungated-egwt.1).
# Resolve via $ROOT (absolutized BEFORE the cd above) — a relative
# ${BASH_SOURCE[0]} (e.g. `cd scripts && ./check-...`) would resolve wrongly
# after the cd (pawl catch on the first land attempt).
. "$ROOT/scripts/lib/docs-scope.sh"
# Pin the scope root: this gate always scans ITS repo's docs/, never a tree an
# inherited DOCS_ROOT env var points at (the injection seam is for the lib's tests).
DOCS_ROOT="$ROOT"

# Live-doc scope: exclude dated/historical archives (same set as the sweep).
mapfile -t DOCS < <(docs_scope_live_files)

# Command/phrase-precise live-staleness patterns.
PATTERN='\bbd (ready|list|show|update|close|create|dep|vc|dolt|ping|context|doctor|init|sync|merge-slot)\b|BEADS_DIR=\$PWD/_beads|git -C _beads|pip install beads|brew upgrade beads|\bgt sling\b|gas[ -]?city|gastown|agentopsd|runtime=gc|ao init --hooks|branch protection blocks|CI is the authoritative gate'

# Doc-type exemptions (filename globs + first-15-lines historical banner + adr/)
# are resolved by the shared lib's docs_scope_is_exempt; see scripts/lib/docs-scope.sh.

# A matched line that ALSO carries removal/past-tense language is DESCRIBING the
# retirement, not prescribing the retired tool — not an offender.
REMOVAL_LANG='[Rr]emoved|[Rr]etired|[Dd]eleted|[Dd]eprecat|[Ss]uperseded|no longer|is gone|are gone|that procedure is gone|not a (selectable|valid)|NOT a selectable|rejected by|was \*\*removed'

declare -i scanned=0 exempt=0
offenders=()
for f in "${DOCS[@]}"; do
  scanned=$((scanned + 1))
  if docs_scope_is_exempt "$f"; then exempt=$((exempt + 1)); continue; fi
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

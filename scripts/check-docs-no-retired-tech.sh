#!/usr/bin/env bash
# check-docs-no-retired-tech.sh
#
# Ratchet from the 2026-06-17 docs staleness sweep (epic
# age-docs-staleness-remediation-cko). Fails if a LIVE doc reintroduces a
# command/phrase for a retired subsystem:
#   - bd/Dolt AS THIS REPO'S TRACKER → use `BEADS_DIR="$(ao beads dir)" br <cmd>`.
#     NOTE (two-store truth): bd/dolt is NOT globally retired — it is the gascity
#     SUBSTRATE store (a different layer). A line framed as the substrate store
#     (SUBSTRATE_LANG below) is current truth, not a retired-tracker prescription.
#   - worktree-local beads → use `ao beads dir`, never `$PWD/_beads` or `git -C _beads`
#   - agentopsd / in-repo daemon (ADR-0009 deleted it)
#   - runtime=gc / gc bridge (removed from the CLI — the severed in-CLI bridge, NOT
#     gascity itself: gascity is the ADOPTED substrate now, not retired tech)
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
# shellcheck disable=SC2034 # consumed by the sourced docs-scope.sh lib.
DOCS_ROOT="$ROOT"

# Live-doc scope: exclude dated/historical archives (same set as the sweep).
mapfile -t DOCS < <(docs_scope_live_files)

# Command/phrase-precise live-staleness patterns.
#
# Removed `ao` command surfaces (recon-2026-07-02 audit A5). Only verbs VERIFIED
# absent from the default build are listed: `ao rpi` (removed, f61c5f0e7),
# `ao orchestrate` (//go:build legacy), `ao evolve` (removed). Deliberately NOT
# listed: `ao flywheel` (LIVE), `ao corpus`/`ao inject` (real //go:build flywheel
# commands), `ao tick`/`ao loop` (legacy-tagged, not removed) — flagging those
# would break the build's own docs. The `/rpi` and `/evolve` SKILLS still exist;
# this pattern is command-precise (`ao <verb>`), so it does not match `/rpi` etc.
# NOTE: `gas[ -]?city|gastown` was REMOVED from this pattern (age-gc-adoption-u0he):
# gascity is now the ADOPTED substrate, not retired tech. The severed in-CLI
# bridge stays flagged via `runtime=gc`.
PATTERN='\bbd (ready|list|show|update|close|create|dep|vc|dolt|ping|context|doctor|init|sync|merge-slot)\b|BEADS_DIR=\$PWD/_beads|git -C _beads|pip install beads|brew upgrade beads|\bgt sling\b|agentopsd|runtime=gc|ao init --hooks|branch protection blocks|CI is the authoritative gate|\bao (rpi|orchestrate|evolve)\b'

# Doc-type exemptions (filename globs + first-15-lines historical banner + adr/)
# are resolved by the shared lib's docs_scope_is_exempt; see scripts/lib/docs-scope.sh.

# A matched line that ALSO carries removal/past-tense language is DESCRIBING the
# retirement, not prescribing the retired tool — not an offender.
# NOTE: `legacy` and `load-bearing` are deliberately NOT removal-language. They
# were added by a sweep agent to make weak annotations pass, but "ao rpi is
# load-bearing legacy" is exactly the STALE framing this gate exists to catch
# (ao rpi is REMOVED, not legacy) — and they would weaken every other pattern
# (a doc saying "bd is legacy" is not describing a removal). Genuine removal /
# past-tense wording only.
REMOVAL_LANG='[Rr]emoved|[Rr]etired|[Dd]eleted|[Dd]eprecat|[Ss]uperseded|[Hh]istorical|[Ff]ormer(ly)?|no longer|is gone|are gone|that procedure is gone|not (the )?live|not a (selectable|valid)|NOT a selectable|rejected by|was \*\*removed'

# Two-store carve-out (age-gc-adoption-u0he): a `bd`/`dolt` mention framed as the
# gascity SUBSTRATE store is CURRENT TRUTH (a different layer from this repo's br
# tracker), not a retired-tracker prescription. Such a line is exempt — but ONLY
# when the hit is bd-tracker-class. Any other retired token on the same line
# (runtime=gc, agentopsd, …) keeps the line flagged: substrate framing must
# never fail-open the non-substrate retired tech (pawl refute on 8aom.3).
SUBSTRATE_LANG='substrate store|gascity substrate|different layer|substrate (that )?a gas'
BD_CLASS='\bbd (ready|list|show|update|close|create|dep|vc|dolt|ping|context|doctor|init|sync|merge-slot)\b'
NON_SUBSTRATE='BEADS_DIR=\$PWD/_beads|git -C _beads|pip install beads|brew upgrade beads|\bgt sling\b|agentopsd|runtime=gc|ao init --hooks|branch protection blocks|CI is the authoritative gate'

declare -i scanned=0 exempt=0
offenders=()
for f in "${DOCS[@]}"; do
  scanned=$((scanned + 1))
  if docs_scope_is_exempt "$f"; then exempt=$((exempt + 1)); continue; fi
  if hits=$(grep -nEi "$PATTERN" "$f" 2>/dev/null); then
    while IFS= read -r line; do
      # skip lines that describe the removal rather than prescribe the tool
      printf '%s' "$line" | grep -qE "$REMOVAL_LANG" && continue
      # skip lines framing bd/dolt as the gascity substrate store (two-store truth)
      # — bd-tracker-class hits ONLY; a co-occurring non-substrate retired token
      # (runtime=gc, agentopsd, …) keeps the line flagged (no fail-open).
      if printf '%s' "$line" | grep -qiE "$SUBSTRATE_LANG" \
         && printf '%s' "$line" | grep -qE "$BD_CLASS" \
         && ! printf '%s' "$line" | grep -qE "$NON_SUBSTRATE"; then
        continue
      fi
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

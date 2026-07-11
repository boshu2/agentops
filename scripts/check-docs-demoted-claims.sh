#!/usr/bin/env bash
# check-docs-demoted-claims.sh
#
# Honesty gate (age-gate-the-ungated-egwt.6). SIBLING of
# check-docs-no-retired-tech.sh — a docs-scoped lexicon gate with its OWN
# lifecycle. ADR-0004 and ADR-0011 DEMOTED the "corpus moat" / "escape-corpus
# compounds" claims to a named UNPROVEN hypothesis facing a structural
# data-starvation headwind. This gate fails when a LIVE narrative doc still
# asserts one of the demoted claims *indicatively* (as proven), or ships an
# uncited multiplier / peer-review claim, unless the line carries an explicit
# hedge.
#
# Three banned lexicon classes (built by scanning the actual offenders on the
# burn-down pages, not from theory):
#   (a) "Peer-reviewed" claims with NO adjacent citation
#       (a same-line author-year, URL, footnote marker, or named source exempts).
#   (b) Uncited multiplier claims: `\b[0-9]+(\.[0-9]+)?x\b` on a line that also
#       carries speedup/faster/improvement language.
#   (c) Compounding / moat / flywheel-thesis phrasings asserted indicatively as
#       proven (e.g. "the corpus is the moat", "the flywheel thesis is validated",
#       "empirically confirmed", "The Flywheel Is The Product",
#       "self-improving through"). Conditional/mechanism phrasings — "knowledge
#       compounds WHEN retrieval × usage beats decay" — are NOT offenders; the
#       hedge/conditional exemption lets them through.
#
# Exemptions (a doc / line opts OUT):
#   - ADRs (docs/adr/**) and dated-archive dirs — via docs_scope (never in scope).
#   - Evidence / eval files (docs/evals/**, docs/evidence/**) — a doc that
#     REPORTS a measured delta is allowed to state its number.
#   - Banner-exempt docs (RETIRED/HISTORICAL/SUPERSEDED banner) — via docs_scope.
#   - Any matched LINE that carries an explicit hedge:
#     unproven / hypothesis / unvalidated / not yet proven / demoted, OR a
#     conditional ("When …", "If …").
#
# Baseline + shrink ratchet (scripts/.docs-demoted-claims-baseline,
# FILENAME-pinned):
#   - a NON-baselined live doc with a banned phrase → exit 1.
#   - a BASELINED file that now has ZERO findings → exit 1 demanding a prune
#     (the shrink ratchet; the baseline may only get smaller).
#
# Blocking: currently ADVISORY in seed.go (Blocking:false). One clean advisory
# cycle then it flips Blocking. The exit codes below are already correct for the
# future flip — the bats twin proves the blocking case.
#
# Exit: 0 clean · 1 offender(s) / stale baseline entry · 2 usage/setup error

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Shared LIVE-doc scope + historical-exemption resolution (age-gate-the-ungated-egwt.1).
# Resolve via $ROOT (absolutized BEFORE the cd above) — a relative
# ${BASH_SOURCE[0]} (e.g. `cd scripts && ./check-...`) would resolve wrongly
# after the cd (a prior pawl REFUTED that on the sibling's first land attempt).
. "$ROOT/scripts/lib/docs-scope.sh"
# Shared shrink-only ratchet mechanics (baseline parse + stale set-diff) —
# age-ratchet-lib-extraction-bv7d.2. Parse mode cr-strip preserves this gate's
# original entry parsing (CR strip + comment/blank skip) byte-for-byte.
. "$ROOT/scripts/lib/ratchet.sh"
# Pin the scope root: this gate always scans ITS repo's docs/, never a tree an
# inherited DOCS_ROOT env var points at (the injection seam is for tests).
# shellcheck disable=SC2034 # DOCS_ROOT is read by the sourced docs-scope.sh lib.
DOCS_ROOT="$ROOT"

BASELINE="$ROOT/scripts/.docs-demoted-claims-baseline"

# ---- lexicon regexes -------------------------------------------------------

# (a) bare "peer-review(ed)" assertion.
CLASS_A='peer[- ]?review'
# A same-line citation exempts a peer-review claim: an author-year "(1995)", a
# URL, a footnote marker "[^", "et al.", or a named classical source.
CITE='\([12][0-9]{3}\)|https?://|\[\^|et al\.|\bDarr\b|\bBoone\b|\bEbbinghaus\b'

# (b) N(.N)x multiplier …
CLASS_B='\b[0-9]+(\.[0-9]+)?x\b'
# … on a line that also carries speedup/improvement language.
CLASS_B_CTX='speedup|faster|improvement|\bimprove'

# (c) demoted-claim phrasings asserted indicatively as proven.
CLASS_C='(is|are) the moat|(flywheel|corpus) thesis is (empirically )?(validated|confirmed|proven)|the flywheel thesis is (validated|confirmed|proven)|empirically confirmed|Validated Thesis|Validated by Evidence|Laws proven|self-improving through|knowledge compounds regardless|flywheel multiplies it|[Ff]lywheel [Ii]s [Tt]he [Pp]roduct'

# A matched LINE that ALSO carries an explicit hedge (or is conditional) is
# honest / mechanism-describing, not an over-claim — exempt at the line level.
HEDGE='unproven|hypothesis|unvalidated|not yet proven|demoted|[Ww]hen |[Ii]f '

# Evidence/eval doc trees: a doc that REPORTS a measured delta may state numbers.
is_evidence_doc() {
  case "$1" in
    docs/evals/*|*/docs/evals/*|docs/evidence/*|*/docs/evidence/*) return 0 ;;
  esac
  return 1
}

# find_offenders FILE — emit "CLASS|line" for each banned, un-hedged line.
find_offenders() {
  local f="$1" ln txt
  # (a)
  while IFS= read -r ln; do
    txt="${ln#*:}"
    printf '%s' "$txt" | grep -qEi "$CITE" && continue
    printf '%s' "$txt" | grep -qEi "$HEDGE" && continue
    echo "peer-review-uncited|$ln"
  done < <(grep -nEi "$CLASS_A" "$f" 2>/dev/null || true)
  # (b)
  while IFS= read -r ln; do
    txt="${ln#*:}"
    printf '%s' "$txt" | grep -qEi "$CLASS_B_CTX" || continue
    printf '%s' "$txt" | grep -qEi "$HEDGE" && continue
    echo "multiplier-uncited|$ln"
  done < <(grep -nEi "$CLASS_B" "$f" 2>/dev/null || true)
  # (c)
  while IFS= read -r ln; do
    txt="${ln#*:}"
    printf '%s' "$txt" | grep -qEi "$HEDGE" && continue
    echo "compounding-as-proven|$ln"
  done < <(grep -nEi "$CLASS_C" "$f" 2>/dev/null || true)
}

# ---- load baseline (FILENAME-pinned; one live-doc path per non-comment line) --
# ratchet_load_pinned owns the parse (cr-strip = this gate's original shape);
# an unreadable baseline is loud (rc 2) instead of a strict-mode abort.
baseline_data="$(ratchet_load_pinned "$BASELINE" cr-strip)" \
  || { echo "check-docs-demoted-claims: cannot read baseline $BASELINE" >&2; exit 2; }
declare -A BASELINED=()
while IFS= read -r bl; do
  [ -n "$bl" ] && BASELINED["$bl"]=1
done <<< "$baseline_data"

mapfile -t DOCS < <(docs_scope_live_files)

declare -i scanned=0 exempt=0
new_offenders=()          # non-baselined live doc with a banned phrase
declare -A OFFENDING=()    # file -> 1 for every file that currently offends
for f in "${DOCS[@]}"; do
  scanned=$((scanned + 1))
  if docs_scope_is_exempt "$f"; then exempt=$((exempt + 1)); continue; fi
  if is_evidence_doc "$f"; then exempt=$((exempt + 1)); continue; fi
  hits="$(find_offenders "$f")"
  [ -z "$hits" ] && continue
  OFFENDING["$f"]=1
  if [ -z "${BASELINED[$f]:-}" ]; then
    while IFS= read -r h; do
      [ -z "$h" ] && continue
      new_offenders+=("$f|$h")
    done <<< "$hits"
  fi
done

# ---- shrink ratchet: a baselined file with ZERO findings must be pruned ------
# ratchet_stale_entries = baselined − currently-offending (emitted LC_ALL=C
# sorted — deterministic; the pre-migration order was unspecified hash order).
stale_baseline=()
while IFS= read -r bl; do
  [ -n "$bl" ] && stale_baseline+=("$bl")
done < <(printf '%s\n' "${!OFFENDING[@]}" | ratchet_stale_entries "$BASELINE" cr-strip)

echo "check-docs-demoted-claims: scanned $scanned live docs ($exempt exempt), baseline pins ${#BASELINED[@]} file(s)"

rc=0
if [ "${#new_offenders[@]}" -gt 0 ]; then
  rc=1
  echo "FAIL — ${#new_offenders[@]} demoted/unproven claim(s) in non-baselined live docs:" >&2
  for o in "${new_offenders[@]}"; do
    file="${o%%|*}"; rest="${o#*|}"; cls="${rest%%|*}"; line="${rest#*|}"
    echo "  - $file [$cls]: $line" >&2
  done
  echo >&2
  echo "Fix: hedge the claim to match ADR-0004 (corpus moat unproven) / ADR-0011" >&2
  echo "     (escape-corpus compounding unproven — structural data-starvation), or" >&2
  echo "     add a citation. Then leave the file OUT of the baseline (or prune it)." >&2
fi

if [ "${#stale_baseline[@]}" -gt 0 ]; then
  rc=1
  echo "FAIL — ${#stale_baseline[@]} baseline entry(ies) now have zero findings — PRUNE them (shrink ratchet):" >&2
  printf '  - %s\n' "${stale_baseline[@]}" >&2
  echo >&2
  echo "Remove the cleaned file(s) from $BASELINE — the baseline may only shrink." >&2
fi

if [ "$rc" -eq 0 ]; then
  echo "PASS — no non-baselined live doc over-claims a demoted hypothesis; baseline is tight."
fi
exit "$rc"

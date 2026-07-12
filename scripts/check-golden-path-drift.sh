#!/usr/bin/env bash
# check-golden-path-drift.sh
#
# Ratchet (age-a-plus-report-card-ieyp2.3): entry docs must not reintroduce
# retired golden paths after PR #902 converged the first-value story onto the
# skill loop (/plan → /implement → /validate).
#
# Scoped ONLY to entry surfaces (not historical ADRs / evals / archives):
#   README.md
#   docs/index.md
#   docs/getting-started/index.md
#   docs/first-value-path.md
#
# Banned prescriptions (phrase-precise; removal/past-tense language exempts):
#   - `ao factory start` as a golden / first-value entry
#   - `/vibe` as the primary validation / first-value path
#   - council-packet as the first-value journey
#   - `ao verify` as the product front door / first-value path
#
# Blocking: ADVISORY (Blocking:false) for one clean cycle, then flips Blocking
# — same lifecycle as docs.demoted-claims.
#
# Exit: 0 clean · 1 offender(s) · 2 usage/setup error

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ENTRY_DOCS=(
  "README.md"
  "docs/index.md"
  "docs/getting-started/index.md"
  "docs/first-value-path.md"
)

# Past-tense / historical framing on the same line exempts the hit.
REMOVAL_LANG='[Rr]emoved|[Rr]etired|[Dd]eleted|[Dd]eprecat|[Ss]uperseded|[Hh]istorical|[Ff]ormer(ly)?|no longer|not (the )?|do not|don'\''t|avoid|banned|never|instead of|was |were '

offenders=0

scan_file() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  [ -f "$file" ] || return 0
  # Use grep -nE; skip lines that carry removal language.
  while IFS= read -r line; do
    local num="${line%%:*}"
    local text="${line#*:}"
    if printf '%s\n' "$text" | grep -Eiq "$REMOVAL_LANG"; then
      continue
    fi
    printf 'FAIL: %s:%s  [%s]  %s\n' "$file" "$num" "$label" "$text"
    offenders=$((offenders + 1))
  done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
}

for f in "${ENTRY_DOCS[@]}"; do
  # ao factory start presented as an operator entry / golden path
  scan_file "$f" 'ao factory start' 'ao-factory-start-as-golden'
  # /vibe as primary first-value / validation front door (skill still exists; ban is entry framing)
  scan_file "$f" '/vibe[[:space:]]+(as|is|for)?[[:space:]]*(primary|first|the front|front door)|first[- ]value[^[:alnum:]]+/vibe|/vibe[[:space:]]+→' 'vibe-as-primary'
  # council packet as first-value journey
  scan_file "$f" 'council[- ]packet.*(first[- ]value|golden|start here)|first[- ]value.*council[- ]packet' 'council-packet-as-first-value'
  # ao verify as product front door / first-value
  scan_file "$f" 'ao verify.*(front door|first[- ]value|golden)|first[- ]value.*ao verify|front door.*ao verify' 'ao-verify-as-front-door'
done

if [ "$offenders" -gt 0 ]; then
  echo ""
  echo "FAIL: $offenders golden-path drift hit(s) in entry docs."
  echo "Repair: restore the skill-loop first-value path (/plan → /implement → /validate),"
  echo "or rephrase with explicit past-tense/removal language. See docs/first-value-path.md."
  exit 1
fi

echo "PASS: golden-path entry docs clean (no retired first-value prescriptions)"
exit 0

#!/usr/bin/env bash
# check-doc-skill-refs.sh — Audit backtick-slash skill references in doctrine docs.
#
# Doctrine docs cite skills as inline code like `/plan`, slash headings like
# ### /validate, or inline commands like `/validate --mode=pr`. Skills get
# renamed, folded, or retired, and the doc citations rot silently — nothing
# checks that a cited `/skillname` still resolves to an existing skills/<dir>.
# This checker closes that gap for the load-bearing doctrine/router docs:
#
#   AGENTS.md
#   CLAUDE.md
#   docs/ARCHITECTURE.md
#   docs/SKILLS.md
#   docs/architecture/operating-loop.md
#   skills/SKILL-TIERS.md
#
# (docs/SKILL-ROUTER.md, the curated router, is covered by --all-docs — see below.)
#
# A reference is the first token after the slash inside a backtick span
# (`/deps audit` -> deps). Lines that carry a retirement marker
# (retired|folded|legacy|historical, case-insensitive) are exempt — citing a
# retired skill in a retirement note is correct, not rot.
#
# --all-docs widens the scan from those hard-pinned files to the union of them
# PLUS the full LIVE docs/** set resolved by scripts/lib/docs-scope.sh, gated by
# a FILENAME-pinned baseline (scripts/.docs-skill-refs-baseline). Under
# --all-docs the check is a shrink-only ratchet:
#   - a NON-baselined live doc with a dead slash-ref  -> FAIL
#   - a baselined file that no longer offends          -> FAIL (prune it)
# The default (no --all-docs) behavior is byte-identical to before: it scans
# exactly the hard-pinned files and never consults the baseline.
#
# Usage:
#   bash scripts/check-doc-skill-refs.sh [--strict] [--all-docs] \
#       [--skills-root DIR] [--docs-root DIR] [--baseline FILE]
#
#   --strict        exit non-zero when any finding exists (default: advisory, exit 0)
#   --all-docs      scan the union of the pinned docs + the live docs/** set,
#                   gated by the filename-pinned shrink baseline
#   --skills-root   directory holding skills/<name>/ (default: <repo>/skills)
#   --docs-root     directory the doc paths are resolved against (default: <repo>)
#   --baseline      baseline file for --all-docs (default: <repo>/scripts/.docs-skill-refs-baseline)
#   -h, --help      show this help
#
# Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
set -uo pipefail

# Absolutize the script dir + repo root BEFORE any cd. A relative
# ${BASH_SOURCE[0]} sourced/used after a cd resolves wrongly (a prior lane's
# pawl REFUTED a relative-source-after-cd); resolve everything up front.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Shared shrink-only ratchet mechanics — used ONLY by the --all-docs path
# (age-ratchet-lib-extraction-bv7d.8); the DEFAULT mode stays byte-identical
# and never touches the baseline. Parse mode trailing-comment = the original
# strip-#-then-trim on all real baseline shapes.
# shellcheck source=scripts/lib/ratchet.sh
. "$SCRIPT_DIR/lib/ratchet.sh"

STRICT=0
ALL_DOCS=0
SKILLS_ROOT="$ROOT/skills"
DOCS_ROOT="$ROOT"
BASELINE="$ROOT/scripts/.docs-skill-refs-baseline"

# Hard-pinned docs scanned in DEFAULT mode, relative to --docs-root. Missing docs
# are skipped (testability: a fixture docs-root may carry only one of them).
#
# NOTE: docs/SKILL-ROUTER.md (the curated router) is deliberately NOT in this
# DEFAULT list — adding it would change the default "N doc(s) scanned" line and
# break the byte-identical-default contract. It is instead covered by --all-docs
# (it is a LIVE docs/** file, is never baselined here, and so a dead `/skill`
# ref in it FAILS under --all-docs --strict). --all-docs is the gate's operative
# mode (see cli/internal/gates/checks/seed.go docs.skill-refs row).
DOCS=(
    "AGENTS.md"
    "CLAUDE.md"
    "docs/ARCHITECTURE.md"
    "docs/SKILLS.md"
    "docs/architecture/operating-loop.md"
    "skills/SKILL-TIERS.md"
)

# Lines carrying any of these markers are exempt from the check.
EXEMPT_RE='retired|folded|legacy|historical'

# An inline skill reference is a backtick, a slash, a slug, then either the closing
# backtick (`/plan`) or a space introducing args (`/deps audit`). Content with
# a second slash (`/mnt/c/...`) never matches: slugs cannot contain '/'.
# shellcheck disable=SC2016 # the backtick is a literal markdown delimiter, not a subshell
INLINE_REF_RE='`/[a-z][a-z0-9_-]*[` ]'

# Docs/SKILLS.md also uses headings as router entries, for example:
#   ### /rpi
#   ### /validate --mode=post-impl
HEADING_REF_RE='^#{2,6}[[:space:]]+/[a-z][a-z0-9_-]*([[:space:](]|$)'

usage() {
    cat <<'USAGE'
check-doc-skill-refs.sh — audit backtick-slash skill references in doctrine docs.

Scans AGENTS.md, CLAUDE.md, docs/ARCHITECTURE.md, docs/SKILLS.md,
docs/architecture/operating-loop.md, and skills/SKILL-TIERS.md for skill
citations (`/skillname`) and slash-command headings, then verifies each resolves
to an existing skills/<dir>. Lines containing retired|folded|legacy|historical
are exempt (retirement notes legitimately cite gone skills).

--all-docs widens the scan to the union of those pinned files PLUS the full LIVE
docs/** set (scripts/lib/docs-scope.sh — which includes docs/SKILL-ROUTER.md,
the curated router), gated by a filename-pinned shrink baseline
(scripts/.docs-skill-refs-baseline): a non-baselined live doc that offends FAILS,
and a baselined file that no longer offends FAILS demanding a prune. Detection
stays slash-syntax + headings ONLY — never bare skill names.

Usage:
  bash scripts/check-doc-skill-refs.sh [--strict] [--all-docs] \
      [--skills-root DIR] [--docs-root DIR] [--baseline FILE]

  --strict        exit non-zero when any finding exists (default: advisory, exit 0)
  --all-docs      scan the union of pinned docs + live docs/**, ratcheted by baseline
  --skills-root   directory holding skills/<name>/ (default: <repo>/skills)
  --docs-root     directory the doc paths are resolved against (default: <repo>)
  --baseline      baseline file for --all-docs (default: <repo>/scripts/.docs-skill-refs-baseline)
  -h, --help      show this help

Exit codes: 0 = clean (or advisory), 1 = findings in --strict, 2 = usage error.
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift ;;
        --all-docs) ALL_DOCS=1; shift ;;
        --skills-root) SKILLS_ROOT="${2:-}"; shift 2 ;;
        --docs-root) DOCS_ROOT="${2:-}"; shift 2 ;;
        --baseline) BASELINE="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --*) echo "ERROR: unknown flag: $1 (try: bash scripts/check-doc-skill-refs.sh --help)" >&2; exit 2 ;;
        *) echo "ERROR: unexpected argument: $1 (try: bash scripts/check-doc-skill-refs.sh --help)" >&2; exit 2 ;;
    esac
done

if [[ ! -d "$SKILLS_ROOT" ]]; then
    echo "ERROR: skills root not found: $SKILLS_ROOT" >&2
    exit 1
fi
if [[ ! -d "$DOCS_ROOT" ]]; then
    echo "ERROR: docs root not found: $DOCS_ROOT" >&2
    exit 1
fi

# doc_offends DOC -> 0 if the doc (relative to DOCS_ROOT) carries >=1 dead
# slash-ref / heading-ref (after the per-line retirement exemption), else 1.
# nearest_live_skill SLUG — emit a "; did you mean `/<skill>`?" suggestion when a
# dead slug has an obvious live successor. Simple, deterministic heuristic:
#   1. an exact substring match either way (slug ⊂ skill or skill ⊂ slug),
#      preferring the shortest such skill (e.g. `hooks-authoring` -> `cc-hooks`
#      via the shared `hooks` token would miss; we also match a shared leading
#      token before the first '-'), else
#   2. a skill sharing the slug's leading token (before the first '-').
# Prints the suggestion suffix (may be empty). No network, no fuzzy scoring.
nearest_live_skill() {
    local slug="$1"
    local head="${slug%%-*}"
    local best="" bestlen=9999 name
    for d in "$SKILLS_ROOT"/*/; do
        [[ -d "$d" ]] || continue
        name="$(basename "$d")"
        # substring either direction, or a shared leading token
        if [[ "$name" == *"$slug"* || "$slug" == *"$name"* \
              || "$name" == "$head"-* || "$name" == *-"$head" || "$name" == *-"$head"-* ]]; then
            if [[ "${#name}" -lt "$bestlen" ]]; then best="$name"; bestlen="${#name}"; fi
        fi
    done
    # Special-case the load-bearing rename this gate exists to catch.
    if [[ -z "$best" && "$slug" == *hooks* && -d "$SKILLS_ROOT/cc-hooks" ]]; then
        best="cc-hooks"
    fi
    [[ -n "$best" ]] && printf '; did you mean `/%s`?' "$best"
}

# Prints one "FINDING ..." line per dead reference to stdout.
doc_offends() {
    local doc="$1"
    local path="$DOCS_ROOT/$doc"
    local offend=1 lineno line slug tok
    [[ -f "$path" ]] || return 1

    while IFS= read -r hit; do
        lineno="${hit%%:*}"
        line="${hit#*:}"
        # Retirement-note exemption: the whole line is out of scope.
        if grep -qiE "$EXEMPT_RE" <<<"$line"; then
            continue
        fi
        if [[ "$line" =~ ^#{2,6}[[:space:]]+/([a-z][a-z0-9_-]*)([[:space:]\(]|$) ]]; then
            slug="${BASH_REMATCH[1]}"
            if [[ ! -d "$SKILLS_ROOT/$slug" ]]; then
                echo "FINDING ${doc}:${lineno}: \`/${slug}\` does not resolve to a skill under ${SKILLS_ROOT}$(nearest_live_skill "$slug")"
                offend=0
            fi
        fi
        while IFS= read -r tok; do
            slug="${tok#\`/}"     # strip leading backtick + slash
            slug="${slug%?}"      # strip trailing backtick-or-space
            if [[ ! -d "$SKILLS_ROOT/$slug" ]]; then
                echo "FINDING ${doc}:${lineno}: \`/${slug}\` does not resolve to a skill under ${SKILLS_ROOT}$(nearest_live_skill "$slug")"
                offend=0
            fi
        done < <(grep -oE "$INLINE_REF_RE" <<<"$line" | sort -u)
    done < <(grep -nE "$INLINE_REF_RE|$HEADING_REF_RE" "$path" || true)

    return "$offend"
}

DOCS_SCANNED=0
FINDINGS=0

if [[ "$ALL_DOCS" -eq 0 ]]; then
    # DEFAULT mode — byte-identical to the historical behavior: scan only the
    # hard-pinned docs, never consult the baseline.
    for doc in "${DOCS[@]}"; do
        [[ -f "$DOCS_ROOT/$doc" ]] || continue
        DOCS_SCANNED=$((DOCS_SCANNED + 1))
        while IFS= read -r finding; do
            echo "$finding"
            FINDINGS=$((FINDINGS + 1))
        done < <(doc_offends "$doc")
    done

    echo "check-doc-skill-refs: ${DOCS_SCANNED} doc(s) scanned, ${FINDINGS} unresolved skill reference(s)"
    if [[ "$FINDINGS" -gt 0 ]]; then
        echo "fix: point each \`/skillname\` at an existing skills/<dir>, or mark the line retired/folded/legacy/historical" >&2
    fi
    if [[ "$FINDINGS" -gt 0 && "$STRICT" -eq 1 ]]; then
        exit 1
    fi
    exit 0
fi

# --all-docs mode: union of the pinned docs + the LIVE docs/** set, gated by the
# filename-pinned shrink baseline.

# Resolve the live-doc set via the shared lib. DOCS_ROOT is honored by the lib
# (it emits docs/**-relative paths anchored at DOCS_ROOT), so a fixture tree
# works too. Source the ABSOLUTIZED lib path (SCRIPT_DIR captured pre-cd).
# shellcheck source=scripts/lib/docs-scope.sh
. "$SCRIPT_DIR/lib/docs-scope.sh"

# Assemble the scan set: pinned docs first, then live docs, de-duplicated,
# preserving order.
declare -A _seen=()
SCAN=()
for doc in "${DOCS[@]}"; do
    [[ -n "${_seen[$doc]:-}" ]] && continue
    _seen[$doc]=1
    SCAN+=("$doc")
done
while IFS= read -r doc; do
    [[ -z "$doc" ]] && continue
    [[ -n "${_seen[$doc]:-}" ]] && continue
    _seen[$doc]=1
    SCAN+=("$doc")
done < <(DOCS_ROOT="$DOCS_ROOT" docs_scope_live_files)

# Load the baseline (allowed offenders), one file path per line; '#' comments and
# blank lines ignored. Track which entries actually get consumed so we can flag
# stale ones.
declare -A BASELINED=()
declare -A BASELINE_HIT=()
baseline_data="$(ratchet_load_pinned "$BASELINE" trailing-comment)" \
    || { echo "ERROR: cannot read baseline $BASELINE" >&2; exit 2; }
while IFS= read -r bl; do
    [[ -n "$bl" ]] && BASELINED["$bl"]=1
done <<< "$baseline_data"

NEW_OFFENDERS=0
for doc in "${SCAN[@]}"; do
    [[ -f "$DOCS_ROOT/$doc" ]] || continue
    DOCS_SCANNED=$((DOCS_SCANNED + 1))
    findings_out="$(doc_offends "$doc")"
    if [[ -n "$findings_out" ]]; then
        # This doc offends.
        n=$(grep -c '^FINDING ' <<<"$findings_out")
        FINDINGS=$((FINDINGS + n))
        if [[ -n "${BASELINED[$doc]:-}" ]]; then
            BASELINE_HIT["$doc"]=1   # baselined offender — allowed, but recorded
        else
            NEW_OFFENDERS=$((NEW_OFFENDERS + 1))
            echo "NEW-OFFENDER ${doc}: not in baseline but carries a dead skill reference:" >&2
            sed 's/^FINDING //' <<<"$findings_out" >&2
        fi
    fi
done

# Stale-baseline (prune) check: any baselined file that either no longer offends
# or no longer exists in scope must be pruned from the baseline.
# ratchet_stale_entries = baselined − still-offending (emitted LC_ALL=C
# sorted — deterministic; pre-migration order was unspecified hash order).
STALE_BASELINE=0
STALE_LIST=()
while IFS= read -r bl; do
    [[ -n "$bl" ]] || continue
    STALE_BASELINE=$((STALE_BASELINE + 1))
    STALE_LIST+=("$bl")
done < <(printf '%s\n' "${!BASELINE_HIT[@]}" | ratchet_stale_entries "$BASELINE" trailing-comment)

echo "check-doc-skill-refs --all-docs: ${DOCS_SCANNED} doc(s) scanned, ${FINDINGS} unresolved skill reference(s) across ${#BASELINED[@]} baselined file(s)"
if [[ "$NEW_OFFENDERS" -gt 0 ]]; then
    echo "FAIL: ${NEW_OFFENDERS} live doc(s) outside the baseline carry a dead \`/skill\` reference." >&2
    echo "fix: point each \`/skillname\` at an existing skills/<dir>, mark the line retired/folded/legacy/historical, or (only for owner-locked pages) add the file to ${BASELINE}." >&2
fi
if [[ "$STALE_BASELINE" -gt 0 ]]; then
    echo "FAIL: ${STALE_BASELINE} baselined file(s) no longer offend — prune them from ${BASELINE}:" >&2
    for s in "${STALE_LIST[@]}"; do echo "  $s" >&2; done
fi

if [[ "$STRICT" -eq 1 && ( "$NEW_OFFENDERS" -gt 0 || "$STALE_BASELINE" -gt 0 ) ]]; then
    exit 1
fi
exit 0

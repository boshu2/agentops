#!/usr/bin/env bash
# check-doc-skill-refs.sh — every `/skill` a doc cites must resolve to skills/<dir>.
#
# Docs cite skills as inline code like `/plan`, slash headings like
# ### /validate, or inline commands like `/validate --mode=pr`. Skills get
# renamed, folded, or retired, and the doc citations rot silently. This check
# holds each cited slug to the fact that skills/<slug>/ exists.
#
# Scope: the pinned non-docs doctrine files (AGENTS.md, CLAUDE.md,
# skills/SKILL-TIERS.md) plus the LIVE docs/** set resolved by
# scripts/lib/docs-scope.sh (which includes docs/SKILL-ROUTER.md, the curated
# router). A reference is the first token after the slash inside a backtick
# span (`/deps audit` -> deps), or a slash-command heading. Bare skill names are
# never matched.
#
# The scan is gated by a FILENAME-pinned, shrink-only baseline
# (scripts/.docs-skill-refs-baseline):
#   - a NON-baselined doc with a dead slash-ref  -> FAIL (NEW-OFFENDER)
#   - a baselined file that no longer offends    -> FAIL (prune it)
#
# Usage:
#   bash scripts/check-doc-skill-refs.sh [--skills-root DIR] [--docs-root DIR] [--baseline FILE]
#
#   --skills-root   directory holding skills/<name>/ (default: <repo>/skills)
#   --docs-root     directory the doc paths are resolved against (default: <repo>)
#   --baseline      baseline file (default: <repo>/scripts/.docs-skill-refs-baseline)
#   -h, --help      show this help
#
# Exit codes: 0 = clean, 1 = new offender or stale baseline entry, 2 = usage error.
set -uo pipefail

# Absolutize the script dir + repo root BEFORE any cd. A relative
# ${BASH_SOURCE[0]} sourced/used after a cd resolves wrongly (a prior lane's
# pawl REFUTED a relative-source-after-cd); resolve everything up front.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Shared shrink-only ratchet mechanics (age-ratchet-lib-extraction-bv7d.8).
# Parse mode trailing-comment = strip-#-then-trim on all real baseline shapes.
# shellcheck source=scripts/lib/ratchet.sh
. "$SCRIPT_DIR/lib/ratchet.sh"
# Shared LIVE docs/** scope.
# shellcheck source=scripts/lib/docs-scope.sh
. "$SCRIPT_DIR/lib/docs-scope.sh"

SKILLS_ROOT="$ROOT/skills"
DOCS_ROOT="$ROOT"
BASELINE="$ROOT/scripts/.docs-skill-refs-baseline"

# Doctrine files outside docs/** that the live-doc set never reaches, relative
# to --docs-root. Missing files are skipped (a fixture docs-root may carry only
# one of them).
PINNED=(
    "AGENTS.md"
    "CLAUDE.md"
    "skills/SKILL-TIERS.md"
)

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
    sed -n '2,29p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
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

# nearest_live_skill SLUG — emit a "; did you mean `/<skill>`?" suggestion when a
# dead slug has an obvious live successor: the shortest skill that contains the
# slug (or is contained by it), or that shares the slug's leading token (before
# the first '-'), e.g. `hooks-authoring` -> `cc-hooks`. Prints the suggestion
# suffix (may be empty). No network, no fuzzy scoring.
nearest_live_skill() {
    local slug="$1"
    local head="${slug%%-*}"
    local best="" bestlen=9999 name
    for d in "$SKILLS_ROOT"/*/; do
        [[ -d "$d" ]] || continue
        name="$(basename "$d")"
        if [[ "$name" == *"$slug"* || "$slug" == *"$name"* \
              || "$name" == "$head"-* || "$name" == *-"$head" || "$name" == *-"$head"-* ]]; then
            if [[ "${#name}" -lt "$bestlen" ]]; then best="$name"; bestlen="${#name}"; fi
        fi
    done
    [[ -n "$best" ]] && printf '; did you mean `/%s`?' "$best"
}

# report_dead DOC LINENO SLUG — print one FINDING line when SLUG is not a skill.
report_dead() {
    local doc="$1" lineno="$2" slug="$3"
    [[ -d "$SKILLS_ROOT/$slug" ]] && return 0
    echo "FINDING ${doc}:${lineno}: \`/${slug}\` does not resolve to a skill under ${SKILLS_ROOT}$(nearest_live_skill "$slug")"
}

# doc_findings DOC — one "FINDING ..." line per dead reference in DOC (relative
# to DOCS_ROOT) on stdout; nothing when the doc is clean or absent.
doc_findings() {
    local doc="$1"
    local file="$DOCS_ROOT/$doc"
    local hit lineno line tok slug
    [[ -f "$file" ]] || return 0

    while IFS= read -r hit; do
        lineno="${hit%%:*}"
        line="${hit#*:}"
        if [[ "$line" =~ ^#{2,6}[[:space:]]+/([a-z][a-z0-9_-]*)([[:space:]\(]|$) ]]; then
            report_dead "$doc" "$lineno" "${BASH_REMATCH[1]}"
        fi
        while IFS= read -r tok; do
            slug="${tok#\`/}"     # strip leading backtick + slash
            slug="${slug%?}"      # strip trailing backtick-or-space
            report_dead "$doc" "$lineno" "$slug"
        done < <(grep -oE "$INLINE_REF_RE" <<<"$line" | sort -u)
    done < <(grep -nE "$INLINE_REF_RE|$HEADING_REF_RE" "$file" || true)
}

# Scan set: pinned files first, then the live docs, de-duplicated in order.
declare -A _seen=()
SCAN=()
while IFS= read -r doc; do
    [[ -n "$doc" && -z "${_seen[$doc]:-}" ]] || continue
    _seen[$doc]=1
    SCAN+=("$doc")
done < <(printf '%s\n' "${PINNED[@]}"; DOCS_ROOT="$DOCS_ROOT" docs_scope_live_files)

# Load the baseline (allowed offenders), one file path per line; '#' comments and
# blank lines ignored.
declare -A BASELINED=()
declare -A BASELINE_HIT=()
baseline_data="$(ratchet_load_pinned "$BASELINE" trailing-comment)" \
    || { echo "ERROR: cannot read baseline $BASELINE" >&2; exit 2; }
while IFS= read -r bl; do
    [[ -n "$bl" ]] && BASELINED["$bl"]=1
done <<< "$baseline_data"

DOCS_SCANNED=0
FINDINGS=0
NEW_OFFENDERS=0
for doc in "${SCAN[@]}"; do
    [[ -f "$DOCS_ROOT/$doc" ]] || continue
    DOCS_SCANNED=$((DOCS_SCANNED + 1))
    findings_out="$(doc_findings "$doc")"
    [[ -n "$findings_out" ]] || continue
    n=$(grep -c '^FINDING ' <<<"$findings_out")
    FINDINGS=$((FINDINGS + n))
    if [[ -n "${BASELINED[$doc]:-}" ]]; then
        BASELINE_HIT["$doc"]=1   # baselined offender — allowed, but recorded
    else
        NEW_OFFENDERS=$((NEW_OFFENDERS + 1))
        echo "NEW-OFFENDER ${doc}: not in baseline but carries a dead skill reference:" >&2
        sed 's/^FINDING //' <<<"$findings_out" >&2
    fi
done

# Stale-baseline (prune) check: any baselined file that either no longer offends
# or no longer exists in scope must be pruned from the baseline.
# ratchet_stale_entries = baselined − still-offending (LC_ALL=C sorted).
STALE_LIST=()
while IFS= read -r bl; do
    [[ -n "$bl" ]] && STALE_LIST+=("$bl")
done < <(printf '%s\n' "${!BASELINE_HIT[@]}" | ratchet_stale_entries "$BASELINE" trailing-comment)

echo "check-doc-skill-refs: ${DOCS_SCANNED} doc(s) scanned, ${FINDINGS} unresolved skill reference(s) across ${#BASELINED[@]} baselined file(s)"
if [[ "$NEW_OFFENDERS" -gt 0 ]]; then
    echo "FAIL: ${NEW_OFFENDERS} doc(s) outside the baseline carry a dead \`/skill\` reference." >&2
    echo "fix: point each \`/skillname\` at an existing skills/<dir>, or (only for owner-locked pages) add the file to ${BASELINE}." >&2
fi
if [[ "${#STALE_LIST[@]}" -gt 0 ]]; then
    echo "FAIL: ${#STALE_LIST[@]} baselined file(s) no longer offend — prune them from ${BASELINE}:" >&2
    for s in "${STALE_LIST[@]}"; do echo "  $s" >&2; done
fi

if [[ "$NEW_OFFENDERS" -gt 0 || "${#STALE_LIST[@]}" -gt 0 ]]; then
    exit 1
fi
exit 0

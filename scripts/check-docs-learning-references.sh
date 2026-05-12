#!/usr/bin/env bash
# check-docs-learning-references.sh — walk docs/plans/ and docs/learnings/
# for specific dated references to .agents/learnings/YYYY-MM-DD-*.md and
# assert a matching docs/learnings/<basename>.md exists OR the reference
# is annotated as documentary/local-only.
#
# Source: soc-w6vh.5.1 — second acceptance clause of soc-w6vh.5
# (Export evolve-cycle learning evidence to durable reviewed surfaces).
# Cycle 61 commit 3022cdf8 landed the first export; this gate catches
# future docs from drifting back to local-only-only references.
#
# Scope (v1):
#  - Walk only docs/plans/ + docs/learnings/ (high-claim-density surfaces)
#  - Match `.agents/learnings/YYYY-MM-DD-<slug>.md` pattern only
#  - PASS if docs/learnings/<basename>.md exists
#  - PASS if reference line includes one of: `(local-only)`, `(documentary)`,
#    `(template)`, `# documentary` annotation tags
#  - FAIL otherwise (gate is blocking by default; --warn-only mode for
#    transition period)
#
# Sibling pattern: scripts/check-contracts-structural-floor.sh — same
# shape (walk surface, assert structural property, emit clear failure
# messages with fix hint).
#
# Usage:
#   bash scripts/check-docs-learning-references.sh             # blocking
#   bash scripts/check-docs-learning-references.sh --warn-only # advisory
#
# Exits 0 on pass, 1 on fail (unless --warn-only), 2 on misuse.
#
# practices: [continuous-integration, design-by-contract, code-complete]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WARN_ONLY=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --warn-only) WARN_ONLY=1; shift;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "Unknown arg: $1" >&2; exit 2;;
    esac
done

# Surfaces to walk
SURFACES=("docs/plans" "docs/learnings")

errors=0
checked=0

for surface in "${SURFACES[@]}"; do
    surface_path="$REPO_ROOT/$surface"
    [[ -d "$surface_path" ]] || continue

    while IFS= read -r -d '' md_file; do
        # Read each line; match specific-dated .agents/learnings/ references
        # The pattern: .agents/learnings/YYYY-MM-DD-<slug>.md
        while IFS=':' read -r lineno line; do
            checked=$((checked + 1))

            # Extract the matched path itself (first hit on the line).
            # `|| true` keeps the script alive under set -e + pipefail when
            # the outer grep matched a date prefix but the line carries a
            # truncated path (e.g. `.agents/learnings/2026-05-11-...`).
            ref_path=$(printf '%s' "$line" | grep -oE '\.agents/learnings/[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Za-z0-9_./-]+\.md' | head -1 || true)
            [[ -z "$ref_path" ]] && continue

            basename=$(basename "$ref_path")

            # Check if the line carries an explicit annotation tag
            if printf '%s' "$line" | grep -qE '\((local-only|documentary|template)\)|# documentary'; then
                continue
            fi

            # Check if the docs/learnings/ mirror exists
            mirror="$REPO_ROOT/docs/learnings/$basename"
            if [[ -f "$mirror" ]]; then
                continue
            fi

            # No mirror, no annotation → error
            rel="${md_file#$REPO_ROOT/}"
            echo "FAIL: $rel:$lineno references absent .agents/learnings file with no durable mirror or annotation" >&2
            echo "  reference:  $ref_path" >&2
            echo "  expected:   docs/learnings/$basename" >&2
            echo "  fix: export rationale to docs/learnings/, OR annotate the reference line with '(local-only)' / '(documentary)' / '(template)'" >&2
            errors=$((errors + 1))
        done < <(grep -nE '\.agents/learnings/[0-9]{4}-[0-9]{2}-[0-9]{2}-' "$md_file" 2>/dev/null || true)
    done < <(find "$surface_path" -name '*.md' -type f -print0 2>/dev/null)
done

if [[ $errors -eq 0 ]]; then
    echo "check-docs-learning-references: PASS (${checked} specific-dated .agents/learnings refs in docs/plans/ + docs/learnings/; all mirrored or annotated)"
    exit 0
fi

if [[ "$WARN_ONLY" -eq 1 ]]; then
    echo "check-docs-learning-references: WARN ($errors un-mirrored reference(s); --warn-only)" >&2
    exit 0
fi

echo "check-docs-learning-references: FAIL ($errors un-mirrored reference(s))" >&2
exit 1

#!/usr/bin/env bash
# resolve-skill-path.sh — shared skill-path resolver over the dispositions ledger (ag-2vz5v).
#
# resolve_skill_path <path>
#   Reads the `historical:` section of docs/contracts/skill-dispositions.yaml
#   (flat-format line parsing — awk only, no yq) and routes the path's
#   skills/<slug>/ or skills-codex/<slug>/ segment through it:
#     state: merged-into  -> prints the path with the slug rewritten to the target
#     state: cut          -> prints nothing, warns on stderr (caller skips visibly),
#                            returns 0
#     no historical row   -> prints the path unchanged (byte-identical)
#   Paths without a skills/<slug>/ segment, a missing ledger, or a malformed
#   row all degrade to identity. Slug matching is exact (a `plan` row never
#   rewrites `plan-foundry` paths).
#
# Test seam: set SKILL_DISPOSITIONS_FILE to point at a fixture ledger.
#
# Sourced by validate-codex-rpi-contract.sh, validate-codex-lifecycle-guards.sh,
# check-hookless-cold-start.sh, and pre-push-gate.sh. These ARE the pre-push
# gate: on a repo where no historical row matches a routed path, behavior must
# stay byte-identical.

_RESOLVE_SKILL_PATH_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

resolve_skill_path() {
    local path="$1"
    local ledger="${SKILL_DISPOSITIONS_FILE:-$_RESOLVE_SKILL_PATH_LIB_DIR/../../docs/contracts/skill-dispositions.yaml}"

    # Identity: no ledger, or no skills/<slug>/ segment to resolve.
    if [[ ! -f "$ledger" ]] || \
       [[ ! "$path" =~ ^(.*/)?(skills|skills-codex)/([^/]+)(/.*)$ ]]; then
        printf '%s\n' "$path"
        return 0
    fi
    local pre="${BASH_REMATCH[1]}"
    local tree="${BASH_REMATCH[2]}"
    local slug="${BASH_REMATCH[3]}"
    local rest="${BASH_REMATCH[4]}"

    # Flat-format line parse of the historical: section. Entries are
    # 2-space-indented `  <slug>:` keys with 4-space-indented fields;
    # the section ends at the next top-level key (e.g. dispositions:).
    local row
    row="$(awk -v slug="$slug" '
        /^historical:/      { in_hist = 1; next }
        in_hist && /^[^ #]/ { in_hist = 0 }
        !in_hist            { next }
        /^  [^ #:]+:[[:space:]]*$/ {
            cur = $0
            sub(/^  /, "", cur)
            sub(/:[[:space:]]*$/, "", cur)
            in_slug = (cur == slug)
            next
        }
        in_slug && $1 == "state:"       { state = $2 }
        in_slug && $1 == "merged-into:" { target = $2 }
        END { if (state != "") print state "\t" target }
    ' "$ledger")"

    if [[ -z "$row" ]]; then
        printf '%s\n' "$path"
        return 0
    fi
    local state="${row%%$'\t'*}"
    local target="${row#*$'\t'}"

    case "$state" in
        merged-into)
            if [[ -z "$target" ]]; then
                # Malformed row (merged-into without a target): identity.
                printf '%s\n' "$path"
            else
                printf '%s%s/%s%s\n' "$pre" "$tree" "$target" "$rest"
            fi
            ;;
        cut)
            echo "resolve_skill_path: skill '$slug' is cut in the dispositions ledger; skipping $path" >&2
            ;;
        *)
            printf '%s\n' "$path"
            ;;
    esac
    return 0
}

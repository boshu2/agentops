#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/bd-audit.sh [--auto-close] [--json] [--strict]
# Audits open beads for staleness, already-fixed state, and consolidation candidates.
# Exit 0 always unless --strict is passed and any issues found.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# BEADS_DOLT_SERVER_PORT default — allows bd invocations to find the Dolt server without manual export
export BEADS_DOLT_SERVER_PORT="${BEADS_DOLT_SERVER_PORT:-3307}"

# Flags
AUTO_CLOSE=false
JSON_OUTPUT=false
STRICT=false

for arg in "$@"; do
    case "$arg" in
        --auto-close) AUTO_CLOSE=true ;;
        --json)       JSON_OUTPUT=true ;;
        --strict)     STRICT=true ;;
    esac
done

# Dependency checks
if ! command -v bd &>/dev/null; then
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        echo '{"error":"bd CLI not found","likely_fixed":[],"likely_stale":[],"consolidatable":[],"summary":{"likely_fixed":0,"likely_stale":0,"consolidatable":0,"total":0}}'
    else
        echo "WARN: bd CLI not found — skipping bead audit" >&2
    fi
    exit 0
fi

if ! command -v jq &>/dev/null; then
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        echo '{"error":"jq not found","likely_fixed":[],"likely_stale":[],"consolidatable":[],"summary":{"likely_fixed":0,"likely_stale":0,"consolidatable":0,"total":0}}'
    else
        echo "WARN: jq not found — skipping bead audit" >&2
    fi
    exit 0
fi

cd "${REPO_ROOT}"

# Collect open + in_progress beads
OPEN_BEADS="$(bd list --status open --json 2>/dev/null || echo '[]')"
INPROG_BEADS="$(bd list --status in_progress --json 2>/dev/null || echo '[]')"
ALL_BEADS="$(echo "${OPEN_BEADS}" "${INPROG_BEADS}" | jq -s 'add // []')"

TOTAL="$(echo "${ALL_BEADS}" | jq 'length')"

if [[ "$TOTAL" -eq 0 ]]; then
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        echo '{"likely_fixed":[],"likely_stale":[],"consolidatable":[],"summary":{"likely_fixed":0,"likely_stale":0,"consolidatable":0,"total":0}}'
    else
        echo "bd-audit: no open beads found"
    fi
    exit 0
fi

# Result buckets
LIKELY_FIXED=()
LIKELY_STALE=()
# file_path -> list of bead IDs (for consolidation)
declare -A FILE_TO_BEADS

# Per-bead structured results for JSON
declare -a FIXED_ENTRIES=()
declare -a STALE_ENTRIES=()

while IFS= read -r bead; do
    BEAD_ID="$(echo "${bead}" | jq -r '.id // empty')"
    TITLE="$(echo "${bead}"    | jq -r '.title // ""')"
    DESC="$(echo "${bead}"     | jq -r '.description // ""')"

    [[ -z "$BEAD_ID" ]] && continue

    # --- Fixed check: commit referencing the bead ID ---
    COMMIT_MATCH=""
    COMMIT_MATCH="$(timeout 5 git log --all --oneline --grep="${BEAD_ID}" 2>/dev/null | head -3 || true)"

    if [[ -n "$COMMIT_MATCH" ]]; then
        LIKELY_FIXED+=("${BEAD_ID}")
        ESCAPED_COMMITS="$(echo "${COMMIT_MATCH}" | jq -Rs '.')"
        FIXED_ENTRIES+=("{\"id\":\"${BEAD_ID}\",\"title\":$(echo "${TITLE}" | jq -Rs '.'),\"reason\":\"commit_match\",\"evidence\":${ESCAPED_COMMITS}}")
        if [[ "$AUTO_CLOSE" == "true" ]]; then
            bd update "${BEAD_ID}" --status closed \
                --append-notes "Auto-closed by bd-audit.sh: commit evidence found: ${COMMIT_MATCH}" \
                2>/dev/null || true
        fi
        continue
    fi

    # --- Fixed check: file-based (files mentioned in desc modified since bead creation) ---
    CREATED_AT="$(
        bd show "${BEAD_ID}" --json 2>/dev/null \
            | jq -r 'if type == "array" then (.[0].created_at // empty) else (.created_at // empty) end' \
            || true
    )"
    if [[ -n "$CREATED_AT" ]]; then
        # Extract file paths from description (lines starting with - or ` containing /)
        FILE_PATHS="$(echo "${DESC}" | grep -oE '[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}' | grep '/' | head -10 || true)"
        if [[ -n "$FILE_PATHS" ]]; then
            # Track for consolidation
            while IFS= read -r fpath; do
                [[ -z "$fpath" ]] && continue
                if [[ -v "FILE_TO_BEADS[$fpath]" ]]; then
                    FILE_TO_BEADS["$fpath"]="${FILE_TO_BEADS[$fpath]} ${BEAD_ID}"
                else
                    FILE_TO_BEADS["$fpath"]="${BEAD_ID}"
                fi
            done <<< "${FILE_PATHS}"

            FILE_CHANGES=""
            while IFS= read -r fpath; do
                [[ -z "$fpath" ]] && continue
                FILE_CHANGES+="$(timeout 5 git log --oneline --since="${CREATED_AT}" -- "${fpath}" 2>/dev/null | head -3 || true)"
            done <<< "${FILE_PATHS}"

            if [[ -n "$FILE_CHANGES" ]]; then
                LIKELY_FIXED+=("${BEAD_ID}")
                ESCAPED_CHANGES="$(echo "${FILE_CHANGES}" | jq -Rs '.')"
                FIXED_ENTRIES+=("{\"id\":\"${BEAD_ID}\",\"title\":$(echo "${TITLE}" | jq -Rs '.'),\"reason\":\"file_modified_since_creation\",\"evidence\":${ESCAPED_CHANGES}}")
                if [[ "$AUTO_CLOSE" == "true" ]]; then
                    bd update "${BEAD_ID}" --status closed \
                        --append-notes "Auto-closed by bd-audit.sh: mentioned files modified since creation." \
                        2>/dev/null || true
                fi
                continue
            fi
        fi
    fi

    # --- Stale check: extract patterns/symbols from description, grep codebase ---
    # Look for code identifiers: camelCase, snake_case, function names in backticks
    # shellcheck disable=SC2016
    PATTERNS="$(echo "${DESC}" | grep -oE '`[^`]+`' | tr -d '`' | head -5 || true)"
    PATTERNS+=$'\n'"$(echo "${DESC}" | grep -oE '\b[a-z][a-zA-Z0-9_]{5,}\b' | head -5 || true)"

    FOUND_ANY=false
    while IFS= read -r pattern; do
        [[ -z "$pattern" ]] && continue
        if timeout 5 grep -rq --include='*.go' --include='*.py' --include='*.sh' \
            --include='*.ts' --include='*.js' --include='*.md' \
            -l "${pattern}" . 2>/dev/null; then
            FOUND_ANY=true
            break
        fi
    done <<< "${PATTERNS}"

    if [[ "$FOUND_ANY" == "false" ]] && [[ -n "$(echo "${PATTERNS}" | tr -d '[:space:]')" ]]; then
        LIKELY_STALE+=("${BEAD_ID}")
        STALE_ENTRIES+=("{\"id\":\"${BEAD_ID}\",\"title\":$(echo "${TITLE}" | jq -Rs '.'),\"reason\":\"referenced_patterns_not_found\"}")
    fi

done < <(echo "${ALL_BEADS}" | jq -c '.[]')

# --- Consolidation check: 2+ beads sharing the same file path ---
CONSOLIDATABLE_IDS=()
declare -a CONSOLIDATE_ENTRIES=()
for fpath in "${!FILE_TO_BEADS[@]}"; do
    ID_LIST="${FILE_TO_BEADS[$fpath]}"
    ID_COUNT="$(echo "${ID_LIST}" | wc -w | tr -d ' ')"
    if [[ "$ID_COUNT" -ge 2 ]]; then
        for bid in ${ID_LIST}; do
            # Avoid duplicates
            already=false
            for existing in "${CONSOLIDATABLE_IDS[@]:-}"; do
                [[ "$existing" == "$bid" ]] && already=true && break
            done
            if [[ "$already" == "false" ]]; then
                CONSOLIDATABLE_IDS+=("$bid")
            fi
        done
        ESCAPED_IDS="$(echo "${ID_LIST}" | jq -Rs 'split(" ") | map(select(. != ""))')"
        CONSOLIDATE_ENTRIES+=("{\"file\":$(echo "${fpath}" | jq -Rs '.'),\"bead_ids\":${ESCAPED_IDS}}")
    fi
done

FIXED_COUNT="${#LIKELY_FIXED[@]}"
STALE_COUNT="${#LIKELY_STALE[@]}"
CONSOL_COUNT="${#CONSOLIDATABLE_IDS[@]}"

if [[ "$JSON_OUTPUT" == "true" ]]; then
    # Build JSON arrays
    FIXED_ARR="[$(IFS=','; echo "${FIXED_ENTRIES[*]:-}")]"
    STALE_ARR="[$(IFS=','; echo "${STALE_ENTRIES[*]:-}")]"
    CONSOL_ARR="[$(IFS=','; echo "${CONSOLIDATE_ENTRIES[*]:-}")]"
    FLAGGED_TOTAL=$(( FIXED_COUNT + STALE_COUNT + CONSOL_COUNT ))
    FLAGGED_PCT=0
    if [[ "$TOTAL" -gt 0 ]]; then
        FLAGGED_PCT=$(( FLAGGED_TOTAL * 100 / TOTAL ))
    fi
    echo "{\"likely_fixed\":${FIXED_ARR},\"likely_stale\":${STALE_ARR},\"consolidatable\":${CONSOL_ARR},\"summary\":{\"likely_fixed\":${FIXED_COUNT},\"likely_stale\":${STALE_COUNT},\"consolidatable\":${CONSOL_COUNT},\"total\":${TOTAL},\"flagged_pct\":${FLAGGED_PCT}}}"
else
    echo "=== bd-audit results ==="
    echo "Total open beads: ${TOTAL}"
    echo "likely-fixed:       ${FIXED_COUNT}"
    echo "likely-stale:       ${STALE_COUNT}"
    echo "consolidatable:     ${CONSOL_COUNT}"
    if [[ "${#LIKELY_FIXED[@]}" -gt 0 ]]; then
        echo ""
        echo "Likely fixed: ${LIKELY_FIXED[*]}"
    fi
    if [[ "${#LIKELY_STALE[@]}" -gt 0 ]]; then
        echo ""
        echo "Likely stale: ${LIKELY_STALE[*]}"
    fi
    if [[ "${#CONSOLIDATABLE_IDS[@]}" -gt 0 ]]; then
        echo ""
        echo "Consolidatable: ${CONSOLIDATABLE_IDS[*]}"
        for entry in "${CONSOLIDATE_ENTRIES[@]:-}"; do
            echo "  ${entry}"
        done
    fi
fi

# Exit behavior
if [[ "$STRICT" == "true" ]] && [[ $(( FIXED_COUNT + STALE_COUNT + CONSOL_COUNT )) -gt 0 ]]; then
    exit 1
fi
exit 0

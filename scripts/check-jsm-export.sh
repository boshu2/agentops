#!/usr/bin/env bash
# check-jsm-export.sh - validate an AgentOps skill through a temporary JSM export copy.
#
# Usage:
#   scripts/check-jsm-export.sh skills/standards
#   scripts/check-jsm-export.sh --json skills/research
set -euo pipefail

FORMAT="human"
KEEP_TEMP=0
SKILL_PATH=""

usage() {
    sed -n '2,7p' "$0"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --json)
            FORMAT="json"
            shift
            ;;
        --keep-temp)
            KEEP_TEMP=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            if [[ -n "$SKILL_PATH" ]]; then
                echo "Only one skill path is supported" >&2
                exit 2
            fi
            SKILL_PATH=$1
            shift
            ;;
    esac
done

if [[ -z "$SKILL_PATH" ]]; then
    usage >&2
    exit 2
fi

if ! command -v jsm >/dev/null 2>&1; then
    echo "jsm is not on PATH" >&2
    exit 127
fi

if [[ ! -d "$SKILL_PATH" || ! -f "$SKILL_PATH/SKILL.md" ]]; then
    echo "Not a skill directory with SKILL.md: $SKILL_PATH" >&2
    exit 2
fi

SOURCE_ABS=$(cd "$(dirname "$SKILL_PATH")" && pwd)/$(basename "$SKILL_PATH")
SKILL_NAME=$(basename "$SOURCE_ABS")
TMP_PARENT=$(mktemp -d)
TMP_SKILL="$TMP_PARENT/$SKILL_NAME"
VALIDATION_JSON=$(mktemp)
VALIDATION_ERR=$(mktemp)

# shellcheck disable=SC2329 # invoked by the EXIT trap below
cleanup() {
    rm -f "$VALIDATION_JSON" "$VALIDATION_ERR"
    if [[ "$KEEP_TEMP" -eq 0 ]]; then
        rm -rf "$TMP_PARENT"
    fi
}
trap cleanup EXIT

source_exec_before=$(find "$SOURCE_ABS" -path '*/scripts/*' -type f -perm -111 2>/dev/null | wc -l | tr -d ' ')
cp -R "$SOURCE_ABS" "$TMP_SKILL"

normalized=0
if [[ -d "$TMP_SKILL/scripts" ]]; then
    normalized=$(find "$TMP_SKILL/scripts" -type f -perm -111 2>/dev/null | wc -l | tr -d ' ')
    find "$TMP_SKILL/scripts" -type f -perm -111 -exec chmod a-x {} + 2>/dev/null || true
fi

set +e
jsm validate "$TMP_SKILL" --json > "$VALIDATION_JSON" 2> "$VALIDATION_ERR"
validate_exit=$?
set -e

source_exec_after=$(find "$SOURCE_ABS" -path '*/scripts/*' -type f -perm -111 2>/dev/null | wc -l | tr -d ' ')

if jq empty "$VALIDATION_JSON" >/dev/null 2>&1; then
    success=$(jq -r '.success // false' "$VALIDATION_JSON")
    if [[ "$success" == "true" ]]; then
        classification="package_clean"
    elif jq -e '.errors[]? | (.message // "") | test("exceeding limit of 50")' "$VALIDATION_JSON" >/dev/null 2>&1; then
        classification="mega_skill_file_limit"
    else
        classification="validation_failed"
    fi
else
    success="false"
    classification="validator_error"
fi

if [[ "$FORMAT" == "json" ]]; then
    if jq empty "$VALIDATION_JSON" >/dev/null 2>&1; then
        jq -n \
            --arg source "$SOURCE_ABS" \
            --arg temp_skill "$TMP_SKILL" \
            --arg classification "$classification" \
            --argjson validate_exit "$validate_exit" \
            --argjson normalized_executable_scripts "$normalized" \
            --argjson source_executable_scripts_before "$source_exec_before" \
            --argjson source_executable_scripts_after "$source_exec_after" \
            --slurpfile validation "$VALIDATION_JSON" \
            '{
              source: $source,
              temp_skill: $temp_skill,
              classification: $classification,
              validate_exit: $validate_exit,
              normalized_executable_scripts: $normalized_executable_scripts,
              source_executable_scripts_before: $source_executable_scripts_before,
              source_executable_scripts_after: $source_executable_scripts_after,
              source_modes_unchanged: ($source_executable_scripts_before == $source_executable_scripts_after),
              validation: $validation[0]
            }'
    else
        jq -n \
            --arg source "$SOURCE_ABS" \
            --arg temp_skill "$TMP_SKILL" \
            --arg classification "$classification" \
            --arg stderr "$(cat "$VALIDATION_ERR")" \
            --arg stdout "$(cat "$VALIDATION_JSON")" \
            --argjson validate_exit "$validate_exit" \
            '{
              source: $source,
              temp_skill: $temp_skill,
              classification: $classification,
              validate_exit: $validate_exit,
              stdout: $stdout,
              stderr: $stderr
            }'
    fi
else
    printf 'source: %s\n' "$SOURCE_ABS"
    printf 'classification: %s\n' "$classification"
    printf 'normalized_executable_scripts: %s\n' "$normalized"
    printf 'source_executable_scripts_before: %s\n' "$source_exec_before"
    printf 'source_executable_scripts_after: %s\n' "$source_exec_after"
    printf 'source_modes_unchanged: %s\n' "$([[ "$source_exec_before" == "$source_exec_after" ]] && echo true || echo false)"
    if jq empty "$VALIDATION_JSON" >/dev/null 2>&1; then
        jq '{success, errors, warnings}' "$VALIDATION_JSON"
    else
        cat "$VALIDATION_ERR" >&2
        cat "$VALIDATION_JSON"
    fi
fi

case "$classification" in
    package_clean|mega_skill_file_limit)
        exit 0
        ;;
    *)
        exit 1
        ;;
esac

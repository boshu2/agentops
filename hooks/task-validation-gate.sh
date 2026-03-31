#!/bin/bash
# task-validation-gate.sh - TaskCompleted hook: validate task metadata before completion
# Reads task JSON from stdin, checks metadata.validation rules.
# Exit 0 = pass (or no validation). Exit 2 = block completion.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_TASK_VALIDATION_DISABLED:-}" = "1" ] && exit 0

# Recursion depth limit: max 3 retry attempts
RETRY_DEPTH_FILE="/tmp/.ao-task-validation-depth-$$"
CURRENT_DEPTH=$(cat "$RETRY_DEPTH_FILE" 2>/dev/null || echo 0)
if [[ "$CURRENT_DEPTH" -ge 3 ]]; then
  echo "WARN: task validation retry limit (3) reached — skipping" >&2
  rm -f "$RETRY_DEPTH_FILE"
  exit 0
fi
echo $((CURRENT_DEPTH + 1)) > "$RETRY_DEPTH_FILE"

# Metadata gate mode: warn (default) or strict
METADATA_GATE="${AGENTOPS_METADATA_GATE:-warn}"

# Read all stdin
INPUT=$(cat)

# Require jq — fail open without it
if ! command -v jq >/dev/null 2>&1; then
    exit 0
fi

# Error log directory (repo-local)
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
ERROR_LOG_DIR="$ROOT/.agents/ao"
ERROR_LOG="$ERROR_LOG_DIR/hook-errors.log"

# Execute validations from repo root so relative paths are predictable.
cd "$ROOT" 2>/dev/null || true

# Restricted command execution: uses shared validate_restricted_cmd from hook-helpers.sh
run_restricted() {
    local cmd="$1"
    if ! validate_restricted_cmd "$cmd" "command" 2>/dev/null; then
        log_error "BLOCKED: $cmd"
        echo "VALIDATION BLOCKED: $(validate_restricted_cmd "$cmd" "command" 2>&1)" >&2
        exit 2
    fi
    local -a cmd_parts
    read -ra cmd_parts <<< "$cmd"
    "${cmd_parts[@]}" >/dev/null 2>&1
}

# Like run_restricted but captures stdout+stderr instead of discarding.
run_restricted_capture() {
    local cmd="$1"
    if ! validate_restricted_cmd "$cmd" "assertion command" 2>/dev/null; then
        log_error "BLOCKED: $cmd"
        echo "VALIDATION BLOCKED: $(validate_restricted_cmd "$cmd" "assertion command" 2>&1)" >&2
        return 1
    fi
    local -a cmd_parts
    read -ra cmd_parts <<< "$cmd"
    "${cmd_parts[@]}" 2>&1
}

log_error() {
    mkdir -p "$ERROR_LOG_DIR" 2>/dev/null
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) task-validation-gate: $1" >> "$ERROR_LOG" 2>/dev/null
}

# Source hook-helpers from plugin install dir, not repo root (security: prevents malicious repo sourcing)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/hook-helpers.sh
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

CONSTRAINT_INDEX="$ROOT/.agents/constraints/index.json"
ACTIVE_CONSTRAINTS='[]'
ACTIVE_CONSTRAINT_COUNT=0
if [ -f "$CONSTRAINT_INDEX" ]; then
    if ! ACTIVE_CONSTRAINTS=$(jq -c '[.constraints // [] | .[]? | select(.status == "active")]' "$CONSTRAINT_INDEX" 2>/dev/null); then
        write_failure "constraint" "load_constraint_index" 1 "malformed active constraint index"
        echo "VALIDATION FAILED: constraint index is unreadable: $CONSTRAINT_INDEX" >&2
        exit 2
    fi
    ACTIVE_CONSTRAINT_COUNT=$(printf '%s' "$ACTIVE_CONSTRAINTS" | jq -r 'length' 2>/dev/null || echo 0)
fi

# Resolve user-provided file paths to repo-rooted absolute paths.
# Returns non-zero if path escapes ROOT or cannot be normalized.
resolve_repo_path() {
    local raw_path="$1"
    local candidate dir base normalized_dir normalized_path

    [ -n "$raw_path" ] || return 1
    case "$raw_path" in
        *$'\n'*|*$'\r'*) return 1 ;;
    esac

    if [[ "$raw_path" = /* ]]; then
        candidate="$raw_path"
    else
        candidate="$ROOT/$raw_path"
    fi

    dir=$(dirname -- "$candidate")
    base=$(basename -- "$candidate")
    normalized_dir=$(cd "$dir" 2>/dev/null && pwd -P) || return 1
    normalized_path="$normalized_dir/$base"

    case "$normalized_path" in
        "$ROOT"|"$ROOT"/*)
            printf '%s\n' "$normalized_path"
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

collect_changed_files() {
    {
        git diff --cached --name-only --diff-filter=ACMR 2>/dev/null || true
        git diff --name-only --diff-filter=ACMR 2>/dev/null || true
        git ls-files --others --exclude-standard 2>/dev/null || true
    } | awk 'NF' | sort -u
}

collect_constraint_target_files() {
    {
        printf '%s' "$INPUT" | jq -r '
            def arr(x):
                if x == null then
                    []
                elif (x | type) == "array" then
                    x
                elif (x | type) == "object" then
                    [x]
                else
                    []
                end;

            [
                (arr(.metadata.files)[]?),
                (arr(.metadata.validation.files_exist)[]?),
                (arr(.metadata.validation.content_check)[]? | .file? // empty)
            ] | .[]
        ' 2>/dev/null || true
        collect_changed_files
    } | awk 'NF' | while IFS= read -r raw_path; do
        [ -n "$raw_path" ] || continue
        resolved=$(resolve_repo_path "$raw_path" 2>/dev/null) || continue
        rel=$(to_repo_relative_path "$resolved")
        printf '%s\n' "${rel#./}"
    done | awk 'NF' | sort -u
}

file_matches_language() {
    local file="$1"
    local language="$2"

    case "$language" in
        go) [[ "$file" == *.go ]] ;;
        python) [[ "$file" == *.py ]] ;;
        shell) [[ "$file" == *.sh ]] ;;
        markdown) [[ "$file" == *.md ]] ;;
        yaml) [[ "$file" == *.yaml || "$file" == *.yml ]] ;;
        json) [[ "$file" == *.json ]] ;;
        typescript) [[ "$file" == *.ts || "$file" == *.tsx ]] ;;
        javascript) [[ "$file" == *.js || "$file" == *.jsx || "$file" == *.mjs || "$file" == *.cjs ]] ;;
        *)
            return 1
            ;;
    esac
}

filter_constraint_target_files() {
    local applies_to_json="$1"
    local target_files="$2"
    local glob_count language_count path_file glob language matched

    glob_count=$(printf '%s' "$applies_to_json" | jq -r '(.path_globs // []) | length' 2>/dev/null || echo 0)
    language_count=$(printf '%s' "$applies_to_json" | jq -r '(.languages // []) | length' 2>/dev/null || echo 0)

    while IFS= read -r path_file; do
        [ -n "$path_file" ] || continue

        if [ "$glob_count" -gt 0 ] 2>/dev/null; then
            matched=0
            while IFS= read -r glob; do
                [ -n "$glob" ] || continue
                # shellcheck disable=SC2053
                if [[ "$path_file" == $glob ]]; then
                    matched=1
                    break
                fi
            done <<< "$(printf '%s' "$applies_to_json" | jq -r '.path_globs[]?' 2>/dev/null)"
            [ "$matched" -eq 1 ] || continue
        fi

        if [ "$language_count" -gt 0 ] 2>/dev/null; then
            matched=0
            while IFS= read -r language; do
                [ -n "$language" ] || continue
                if file_matches_language "$path_file" "$language"; then
                    matched=1
                    break
                fi
            done <<< "$(printf '%s' "$applies_to_json" | jq -r '.languages[]?' 2>/dev/null)"
            [ "$matched" -eq 1 ] || continue
        fi

        printf '%s\n' "$path_file"
    done <<< "$target_files" | sort -u
}

constraint_failure() {
    local constraint_id="$1"
    local detail="$2"
    local command_ref="${3:-constraint}"

    write_failure "constraint" "$command_ref" 1 "$constraint_id: $detail"
    echo "VALIDATION FAILED: constraint $constraint_id — $detail" >&2
}

run_constraint_content_pattern() {
    local constraint_id="$1"
    local detector_json="$2"
    local applicable_files="$3"
    local mode pattern message resolved_file rel_file

    mode=$(printf '%s' "$detector_json" | jq -r '.mode // "must_contain"' 2>/dev/null)
    pattern=$(printf '%s' "$detector_json" | jq -r '.pattern // empty' 2>/dev/null)
    message=$(printf '%s' "$detector_json" | jq -r '.message // empty' 2>/dev/null)

    if [ -z "$pattern" ]; then
        constraint_failure "$constraint_id" "content_pattern detector is missing pattern" "content_pattern"
        return 2
    fi

    while IFS= read -r rel_file; do
        [ -n "$rel_file" ] || continue
        resolved_file=$(resolve_repo_path "$rel_file" 2>/dev/null) || {
            constraint_failure "$constraint_id" "path escapes repo root: $rel_file" "content_pattern"
            return 2
        }
        if [ ! -f "$resolved_file" ]; then
            constraint_failure "$constraint_id" "target file not found: $rel_file" "content_pattern"
            return 2
        fi

        case "$mode" in
            must_contain)
                if ! grep -qF -- "$pattern" "$resolved_file" 2>/dev/null; then
                    constraint_failure "$constraint_id" "${message:-expected literal pattern '$pattern' in $rel_file}" "content_pattern"
                    return 2
                fi
                ;;
            must_not_contain)
                if grep -qF -- "$pattern" "$resolved_file" 2>/dev/null; then
                    constraint_failure "$constraint_id" "${message:-forbidden literal pattern '$pattern' found in $rel_file}" "content_pattern"
                    return 2
                fi
                ;;
            *)
                constraint_failure "$constraint_id" "unsupported content_pattern mode: $mode" "content_pattern"
                return 2
                ;;
        esac
    done <<< "$applicable_files"

    return 0
}

run_constraint_paired_files() {
    local constraint_id="$1"
    local detector_json="$2"
    local all_target_files="$3"
    local applicable_files="$4"
    local pattern exclude companion message rel_companion derived_companion rel_file

    pattern=$(printf '%s' "$detector_json" | jq -r '.pattern // empty' 2>/dev/null)
    exclude=$(printf '%s' "$detector_json" | jq -r '.exclude // empty' 2>/dev/null)
    companion=$(printf '%s' "$detector_json" | jq -r '.companion // empty' 2>/dev/null)
    message=$(printf '%s' "$detector_json" | jq -r '.message // empty' 2>/dev/null)

    if [ -z "$pattern" ] || [ -z "$companion" ]; then
        constraint_failure "$constraint_id" "paired_files detector requires pattern and companion" "paired_files"
        return 2
    fi

    while IFS= read -r rel_file; do
        [ -n "$rel_file" ] || continue
        # shellcheck disable=SC2053
        if [[ "$rel_file" != $pattern ]]; then
            continue
        fi
        # shellcheck disable=SC2053
        if [ -n "$exclude" ] && [[ "$rel_file" == $exclude ]]; then
            continue
        fi

        derived_companion=$(derive_companion_path "$rel_file" "$companion")
        rel_companion="${derived_companion#./}"
        if ! printf '%s\n' "$all_target_files" | grep -Fx -- "$rel_companion" >/dev/null 2>&1; then
            constraint_failure "$constraint_id" "${message:-missing companion '$rel_companion' for '$rel_file'}" "paired_files"
            return 2
        fi
    done <<< "$applicable_files"

    return 0
}

run_constraint_restricted_command() {
    local constraint_id="$1"
    local detector_json="$2"
    local command message reason
    local -a cmd_parts

    command=$(printf '%s' "$detector_json" | jq -r '.command // empty' 2>/dev/null)
    message=$(printf '%s' "$detector_json" | jq -r '.message // empty' 2>/dev/null)

    if [ -z "$command" ]; then
        constraint_failure "$constraint_id" "restricted_command detector is missing command" "restricted_command"
        return 2
    fi

    if ! validate_restricted_cmd "$command" "constraint command" 2>/dev/null; then
        reason=$(validate_restricted_cmd "$command" "constraint command" 2>&1)
        constraint_failure "$constraint_id" "$reason" "restricted_command"
        return 2
    fi

    read -ra cmd_parts <<< "$command"
    if ! "${cmd_parts[@]}" >/dev/null 2>&1; then
        constraint_failure "$constraint_id" "${message:-constraint command failed: $command}" "restricted_command"
        return 2
    fi

    return 0
}

evaluate_active_constraints() {
    local constraints_json="$1"
    local target_files="$2"
    local constraint_json id detector_json detector_kind applies_to_json applicable_files
    local requires_issue_type

    requires_issue_type=$(printf '%s' "$constraints_json" | jq -r 'any(.[]?; ((.applies_to.issue_types // []) | length) > 0)' 2>/dev/null)
    if [ "$requires_issue_type" = "true" ] && [ -z "$ISSUE_TYPE" ]; then
        constraint_failure "issue_type" "active constraints require metadata.issue_type on the task payload" "constraint"
        return 2
    fi

    while IFS= read -r constraint_json; do
        [ -n "$constraint_json" ] || continue
        id=$(printf '%s' "$constraint_json" | jq -r '.id // empty' 2>/dev/null)
        detector_json=$(printf '%s' "$constraint_json" | jq -c '.detector // {}' 2>/dev/null)
        detector_kind=$(printf '%s' "$detector_json" | jq -r '.kind // empty' 2>/dev/null)
        applies_to_json=$(printf '%s' "$constraint_json" | jq -c '.applies_to // {}' 2>/dev/null)

        if [ -z "$id" ] || [ -z "$detector_kind" ]; then
            constraint_failure "${id:-unknown}" "active constraint entry is missing required id or detector.kind" "constraint"
            return 2
        fi

        if ! printf '%s' "$applies_to_json" | jq -e '. == {} or ((.scope // "files") == "files")' >/dev/null 2>&1; then
            constraint_failure "$id" "unsupported applies_to.scope in active constraint" "$detector_kind"
            return 2
        fi

        if printf '%s' "$applies_to_json" | jq -e '((.issue_types // []) | length) > 0' >/dev/null 2>&1; then
            if ! printf '%s' "$applies_to_json" | jq -e --arg issue_type "$ISSUE_TYPE" '(.issue_types // []) | index($issue_type) != null' >/dev/null 2>&1; then
                continue
            fi
        fi

        applicable_files=$(filter_constraint_target_files "$applies_to_json" "$target_files")
        if [ -z "$applicable_files" ] && [ "$detector_kind" != "restricted_command" ]; then
            continue
        fi

        case "$detector_kind" in
            content_pattern)
                run_constraint_content_pattern "$id" "$detector_json" "$applicable_files" || return 2
                ;;
            paired_files)
                run_constraint_paired_files "$id" "$detector_json" "$target_files" "$applicable_files" || return 2
                ;;
            restricted_command)
                run_constraint_restricted_command "$id" "$detector_json" || return 2
                ;;
            *)
                constraint_failure "$id" "unsupported active detector kind: $detector_kind" "$detector_kind"
                return 2
                ;;
        esac
    done <<< "$(printf '%s' "$constraints_json" | jq -c '.[]' 2>/dev/null)"

    return 0
}

derive_companion_path() {
    local source_file="$1"
    local companion_template="$2"
    local source_dir source_name source_basename source_ext companion

    source_dir=$(dirname -- "$source_file")
    source_name=$(basename -- "$source_file")
    source_basename="$source_name"
    source_ext=""
    if [[ "$source_name" == *.* ]]; then
        source_basename="${source_name%.*}"
        source_ext=".${source_name##*.}"
    fi

    companion="$companion_template"
    companion="${companion//\{dir\}/$source_dir}"
    companion="${companion//\{basename\}/$source_basename}"
    companion="${companion//\{ext\}/$source_ext}"
    companion="${companion#./}"
    while [[ "$companion" == *"//"* ]]; do
        companion="${companion//\/\//\/}"
    done
    printf '%s\n' "$companion"
}

is_impl_issue_type() {
    case "$1" in
        feature|bug|task) return 0 ;;
        *) return 1 ;;
    esac
}

is_validation_exempt_issue_type() {
    case "$1" in
        docs|chore|ci) return 0 ;;
        *) return 1 ;;
    esac
}

# Extract metadata.validation — fail open on parse errors
if ! VALIDATION=$(echo "$INPUT" | jq -r '.metadata.validation // empty' 2>/dev/null); then
    log_error "JSON parse error on stdin"
    exit 0
fi

# Resolve issue type from common payload locations.
ISSUE_TYPE=$(echo "$INPUT" | jq -r '
    (
        .issue_type // .type // .kind //
        .metadata.issue_type // .metadata.type // .metadata.kind //
        .issue.issue_type // .issue.type // .issue.kind // empty
    )
    | if type == "string" then ascii_downcase | gsub("^\\s+|\\s+$"; "") else "" end
' 2>/dev/null)

# No validation metadata.
if [ -z "$VALIDATION" ] || [ "$VALIDATION" = "null" ]; then
    if is_impl_issue_type "$ISSUE_TYPE"; then
        if [ "$METADATA_GATE" = "strict" ]; then
            write_failure "validation_metadata" "metadata.validation" 1 "missing metadata.validation for issue_type '$ISSUE_TYPE'"
            echo "VALIDATION FAILED: metadata.validation is required for issue_type '$ISSUE_TYPE' (feature|bug|task)" >&2
            exit 2
        fi

        echo "WARN: metadata.validation missing for issue_type '$ISSUE_TYPE' — set AGENTOPS_METADATA_GATE=strict to enforce" >&2
        if [ "$ACTIVE_CONSTRAINT_COUNT" -eq 0 ]; then
            exit 0
        fi
        VALIDATION='{}'
    elif is_validation_exempt_issue_type "$ISSUE_TYPE"; then
        # Explicit exemption path for non-implementation tasks.
        if [ "$ACTIVE_CONSTRAINT_COUNT" -eq 0 ]; then
            exit 0
        fi
        VALIDATION='{}'
    else
        # Unknown/legacy tasks remain fail-open for backward compatibility.
        if [ "$ACTIVE_CONSTRAINT_COUNT" -eq 0 ]; then
            exit 0
        fi
        VALIDATION='{}'
    fi
fi

# Normalize common validation fields once for policy checks and execution.
FILES_EXIST=$(echo "$VALIDATION" | jq -c '
    if (.files_exist // null) == null then
        []
    elif (.files_exist | type) == "array" then
        .files_exist
    else
        []
    end
' 2>/dev/null)
FILES_EXIST="${FILES_EXIST:-[]}"

CONTENT_CHECKS=$(echo "$VALIDATION" | jq -c '
    if (.content_check // null) == null then
        []
    elif (.content_check | type) == "array" then
        .content_check
    elif (.content_check | type) == "object" then
        [ .content_check ]
    else
        []
    end
' 2>/dev/null)
CONTENT_CHECKS="${CONTENT_CHECKS:-[]}"

TESTS_CMD=$(echo "$VALIDATION" | jq -r '.tests // empty' 2>/dev/null)

# Strict policy for implementation tasks: tests + structural evidence required.
if is_impl_issue_type "$ISSUE_TYPE"; then
    if [ -z "$TESTS_CMD" ] || [ "$TESTS_CMD" = "null" ]; then
        if [ "$METADATA_GATE" = "strict" ]; then
            write_failure "validation_metadata" "metadata.validation.tests" 1 "missing tests for issue_type '$ISSUE_TYPE'"
            echo "VALIDATION FAILED: metadata.validation.tests is required for issue_type '$ISSUE_TYPE'" >&2
            exit 2
        else
            echo "WARN: metadata.validation.tests missing for issue_type '$ISSUE_TYPE' — set AGENTOPS_METADATA_GATE=strict to enforce" >&2
        fi
    fi

    FILE_COUNT=$(echo "$FILES_EXIST" | jq -r 'length' 2>/dev/null)
    CHECK_COUNT=$(echo "$CONTENT_CHECKS" | jq -r 'length' 2>/dev/null)
    FILE_COUNT="${FILE_COUNT:-0}"
    CHECK_COUNT="${CHECK_COUNT:-0}"
    if [ "$FILE_COUNT" -eq 0 ] && [ "$CHECK_COUNT" -eq 0 ]; then
        if [ "$METADATA_GATE" = "strict" ]; then
            write_failure "validation_metadata" "metadata.validation" 1 "missing structural checks for issue_type '$ISSUE_TYPE'"
            echo "VALIDATION FAILED: metadata.validation for issue_type '$ISSUE_TYPE' must include files_exist or content_check" >&2
            exit 2
        else
            echo "WARN: metadata.validation missing structural checks for issue_type '$ISSUE_TYPE' — set AGENTOPS_METADATA_GATE=strict to enforce" >&2
        fi
    fi
fi

CONSTRAINT_TARGET_FILES=$(collect_constraint_target_files)

# --- Validation checks ---

# 1. files_exist: array of paths
if [ -n "$FILES_EXIST" ] && [ "$FILES_EXIST" != "null" ]; then
    FILE_COUNT=$(echo "$FILES_EXIST" | jq -r 'length' 2>/dev/null)
    if [ -n "$FILE_COUNT" ] && [ "$FILE_COUNT" -gt 0 ] 2>/dev/null; then
        for i in $(seq 0 $((FILE_COUNT - 1))); do
            FILE_PATH=$(echo "$FILES_EXIST" | jq -r ".[$i]" 2>/dev/null)
            if [ -n "$FILE_PATH" ] && [ "$FILE_PATH" != "null" ]; then
                RESOLVED_FILE=$(resolve_repo_path "$FILE_PATH") || {
                    log_error "blocked files_exist path outside repo root: $FILE_PATH"
                    write_failure "files_exist" "resolve_repo_path" 1 "path escapes repo root: $FILE_PATH"
                    echo "VALIDATION FAILED: files_exist — path escapes repo root: $FILE_PATH" >&2
                    exit 2
                }
                if [ ! -f "$RESOLVED_FILE" ]; then
                    # Collect all missing files from this check
                    MISSING_FILES="$FILE_PATH"
                    for j in $(seq $((i + 1)) $((FILE_COUNT - 1))); do
                        NEXT_FILE=$(echo "$FILES_EXIST" | jq -r ".[$j]" 2>/dev/null)
                        if [ -n "$NEXT_FILE" ] && [ "$NEXT_FILE" != "null" ]; then
                            NEXT_RESOLVED=$(resolve_repo_path "$NEXT_FILE" 2>/dev/null) || continue
                            if [ ! -f "$NEXT_RESOLVED" ]; then
                                MISSING_FILES="$MISSING_FILES, $NEXT_FILE"
                            fi
                        fi
                    done
                    write_failure "files_exist" "test -f" 1 "missing files: $MISSING_FILES"
                    echo "VALIDATION FAILED: files_exist — missing files: $MISSING_FILES" >&2
                    exit 2
                fi
            fi
        done
    fi
fi

# 2. content_check: array of {file, pattern}
if [ -n "$CONTENT_CHECKS" ] && [ "$CONTENT_CHECKS" != "null" ]; then
    CHECK_COUNT=$(echo "$CONTENT_CHECKS" | jq -r 'length' 2>/dev/null)
    if [ -n "$CHECK_COUNT" ] && [ "$CHECK_COUNT" -gt 0 ] 2>/dev/null; then
        for i in $(seq 0 $((CHECK_COUNT - 1))); do
            CHECK_FILE=$(echo "$CONTENT_CHECKS" | jq -r ".[$i].file" 2>/dev/null)
            CHECK_PATTERN=$(echo "$CONTENT_CHECKS" | jq -r ".[$i].pattern" 2>/dev/null)
            if [ -n "$CHECK_FILE" ] && [ "$CHECK_FILE" != "null" ] && [ -n "$CHECK_PATTERN" ] && [ "$CHECK_PATTERN" != "null" ]; then
                RESOLVED_CHECK_FILE=$(resolve_repo_path "$CHECK_FILE") || {
                    log_error "blocked content_check path outside repo root: $CHECK_FILE"
                    write_failure "content_check" "resolve_repo_path" 1 "path escapes repo root: $CHECK_FILE"
                    echo "VALIDATION FAILED: content_check — path escapes repo root: $CHECK_FILE" >&2
                    exit 2
                }
                if ! grep -qF "$CHECK_PATTERN" "$RESOLVED_CHECK_FILE" 2>/dev/null; then
                    write_failure "content_check" "grep" 1 "pattern '$CHECK_PATTERN' not found in file $CHECK_FILE"
                    echo "VALIDATION FAILED: content_check — pattern '$CHECK_PATTERN' not found in file $CHECK_FILE" >&2
                    echo "  Expected pattern: $CHECK_PATTERN" >&2
                    echo "  File: $CHECK_FILE" >&2
                    exit 2
                fi
            fi
        done
    fi
fi

# 3. paired_files: array of {pattern, exclude, companion, message}
PAIRED_RULES=$(echo "$VALIDATION" | jq -r '.paired_files // empty' 2>/dev/null)
if [ -n "$PAIRED_RULES" ] && [ "$PAIRED_RULES" != "null" ]; then
    RULE_COUNT=$(echo "$PAIRED_RULES" | jq -r 'length' 2>/dev/null)
    if [ -n "$RULE_COUNT" ] && [ "$RULE_COUNT" -gt 0 ] 2>/dev/null; then
        CHANGED_FILES=$(collect_changed_files)
        if [ -n "$CHANGED_FILES" ]; then
            for i in $(seq 0 $((RULE_COUNT - 1))); do
                RULE_PATTERN=$(echo "$PAIRED_RULES" | jq -r ".[$i].pattern // empty" 2>/dev/null)
                RULE_EXCLUDE=$(echo "$PAIRED_RULES" | jq -r ".[$i].exclude // empty" 2>/dev/null)
                RULE_COMPANION=$(echo "$PAIRED_RULES" | jq -r ".[$i].companion // empty" 2>/dev/null)
                RULE_MESSAGE=$(echo "$PAIRED_RULES" | jq -r ".[$i].message // empty" 2>/dev/null)

                if [ -z "$RULE_PATTERN" ] || [ -z "$RULE_COMPANION" ]; then
                    continue
                fi

                while IFS= read -r CHANGED_FILE; do
                    [ -n "$CHANGED_FILE" ] || continue

                    # shellcheck disable=SC2053
                    if [[ "$CHANGED_FILE" != $RULE_PATTERN ]]; then
                        continue
                    fi
                    # shellcheck disable=SC2053
                    if [ -n "$RULE_EXCLUDE" ] && [[ "$CHANGED_FILE" == $RULE_EXCLUDE ]]; then
                        continue
                    fi

                    DERIVED_COMPANION=$(derive_companion_path "$CHANGED_FILE" "$RULE_COMPANION")
                    RESOLVED_COMPANION=$(resolve_repo_path "$DERIVED_COMPANION") || {
                        log_error "blocked paired_files companion outside repo root: $DERIVED_COMPANION"
                        write_failure "paired_files" "resolve_repo_path" 1 "path escapes repo root: $DERIVED_COMPANION"
                        echo "VALIDATION FAILED: paired_files — path escapes repo root: $DERIVED_COMPANION" >&2
                        exit 2
                    }
                    REL_COMPANION=$(to_repo_relative_path "$RESOLVED_COMPANION")
                    REL_COMPANION="${REL_COMPANION#./}"

                    if ! printf '%s\n' "$CHANGED_FILES" | grep -Fx -- "$REL_COMPANION" >/dev/null 2>&1; then
                        FAIL_DETAIL="missing companion '$REL_COMPANION' for changed file '$CHANGED_FILE'"
                        write_failure "paired_files" "$RULE_PATTERN" 1 "$FAIL_DETAIL"
                        if [ -n "$RULE_MESSAGE" ]; then
                            echo "VALIDATION FAILED: paired_files — $RULE_MESSAGE" >&2
                        else
                            echo "VALIDATION FAILED: paired_files — $FAIL_DETAIL" >&2
                        fi
                        echo "  Changed file: $CHANGED_FILE" >&2
                        echo "  Expected companion: $REL_COMPANION" >&2
                        exit 2
                    fi
                done <<< "$CHANGED_FILES"
            done
        fi
    fi
fi

# 3.5 active constraints: .agents/constraints/index.json
if [ "$ACTIVE_CONSTRAINT_COUNT" -gt 0 ] 2>/dev/null; then
    if ! evaluate_active_constraints "$ACTIVE_CONSTRAINTS" "$CONSTRAINT_TARGET_FILES"; then
        exit 2
    fi
fi

# 4. tests: command string
if [ -n "$TESTS_CMD" ] && [ "$TESTS_CMD" != "null" ]; then
    if ! run_restricted "$TESTS_CMD"; then
        write_failure "test" "$TESTS_CMD" "$?" "test command failed"
        echo "VALIDATION FAILED: tests — command failed: $TESTS_CMD" >&2
        echo "  Suggested: /bug-hunt --test-failure .agents/ao/last-failure.json" >&2
        exit 2
    fi
fi

# 5. lint: command string
LINT_CMD=$(echo "$VALIDATION" | jq -r '.lint // empty' 2>/dev/null)
if [ -n "$LINT_CMD" ] && [ "$LINT_CMD" != "null" ]; then
    if ! run_restricted "$LINT_CMD"; then
        write_failure "lint" "$LINT_CMD" "$?" "lint command failed"
        echo "VALIDATION FAILED: lint — command failed: $LINT_CMD" >&2
        echo "  Suggested: /bug-hunt --test-failure .agents/ao/last-failure.json" >&2
        exit 2
    fi
fi

# 6. command: command string
GENERIC_CMD=$(echo "$VALIDATION" | jq -r '.command // empty' 2>/dev/null)
if [ -n "$GENERIC_CMD" ] && [ "$GENERIC_CMD" != "null" ]; then
    if ! run_restricted "$GENERIC_CMD"; then
        write_failure "command" "$GENERIC_CMD" "$?" "command failed"
        echo "VALIDATION FAILED: command — command failed: $GENERIC_CMD" >&2
        echo "  Suggested: /bug-hunt --test-failure .agents/ao/last-failure.json" >&2
        exit 2
    fi
fi

# 7. assertions: array of {command, expect_pattern, description}
# Behavioral assertions — run command and check output for expected pattern.
# Closes the gap between "tests run" and "tests verify correct behavior."
ASSERTIONS=$(echo "$VALIDATION" | jq -c '.assertions // []' 2>/dev/null)
ASSERTION_COUNT=$(echo "$ASSERTIONS" | jq -r 'length' 2>/dev/null || echo 0)

if [ -n "$ASSERTION_COUNT" ] && [ "$ASSERTION_COUNT" -gt 0 ] 2>/dev/null; then
    for i in $(seq 0 $((ASSERTION_COUNT - 1))); do
        A_CMD=$(echo "$ASSERTIONS" | jq -r ".[$i].command" 2>/dev/null)
        A_PATTERN=$(echo "$ASSERTIONS" | jq -r ".[$i].expect_pattern" 2>/dev/null)
        A_DESC=$(echo "$ASSERTIONS" | jq -r ".[$i].description // \"assertion $i\"" 2>/dev/null)

        # Skip if command or pattern empty (braces fix operator precedence: || before &&)
        { [ -z "$A_CMD" ] || [ "$A_CMD" = "null" ]; } && continue
        { [ -z "$A_PATTERN" ] || [ "$A_PATTERN" = "null" ]; } && continue

        # Pre-validate regex — grep exits 2 on malformed patterns
        printf '' | grep -qE "$A_PATTERN" 2>/dev/null
        REGEX_RC=$?
        if [ "$REGEX_RC" -eq 2 ]; then
            write_failure "behavioral_assertion" "$A_CMD" 2 "Invalid regex pattern: $A_PATTERN"
            echo "VALIDATION FAILED: behavioral assertion '$A_DESC' — invalid regex pattern: $A_PATTERN" >&2
            exit 2
        fi

        # Run through restricted executor with output capture
        A_OUTPUT=$(run_restricted_capture "$A_CMD") || {
            write_failure "behavioral_assertion" "$A_CMD" 2 "Command failed: $A_DESC"
            echo "VALIDATION FAILED: behavioral assertion '$A_DESC' — command failed: $A_CMD" >&2
            exit 2
        }

        # Check output for expected pattern (regex via grep -E)
        if ! echo "$A_OUTPUT" | grep -qE "$A_PATTERN"; then
            write_failure "behavioral_assertion" "$A_CMD" 2 "Expected pattern '$A_PATTERN' not found in output: $A_DESC"
            echo "VALIDATION FAILED: behavioral assertion '$A_DESC' — expected pattern '$A_PATTERN' not found in output" >&2
            echo "  Command: $A_CMD" >&2
            echo "  Pattern: $A_PATTERN" >&2
            exit 2
        fi
    done
fi

# 8. embedded_parity: auto-check when hook files changed
# Enforces hooks/ <-> cli/embedded/hooks/ parity after any hook modification.
CHANGED_FILES_LIST=$(collect_changed_files 2>/dev/null || true)
HOOK_CHANGED=0
if [ -n "$CHANGED_FILES_LIST" ]; then
    if echo "$CHANGED_FILES_LIST" | grep -qE '^(hooks/|lib/hook-helpers\.sh)'; then
        HOOK_CHANGED=1
    fi
fi

if [ "$HOOK_CHANGED" -eq 1 ]; then
    PARITY_SCRIPT="$ROOT/scripts/validate-embedded-sync.sh"
    if [ -f "$PARITY_SCRIPT" ] && [ -x "$PARITY_SCRIPT" ]; then
        if ! bash "$PARITY_SCRIPT" >/dev/null 2>&1; then
            write_failure "embedded_parity" "validate-embedded-sync.sh" 1 "hooks/ and cli/embedded/hooks/ are out of sync"
            echo "VALIDATION FAILED: embedded_parity — hooks and embedded copies are out of sync" >&2
            echo "  Run: cd cli && make sync-hooks" >&2
            exit 2
        fi
    fi
fi

# All checks passed
exit 0

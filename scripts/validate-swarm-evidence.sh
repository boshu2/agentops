#!/usr/bin/env bash
# validate-swarm-evidence.sh — Validate swarm worker result evidence against schema contract.
# Usage: bash scripts/validate-swarm-evidence.sh [result-file.json|directory]
#   - With a file argument: validates that single result file
#   - With a directory argument: validates all *.json files in the directory
#   - With no arguments: scans .agents/swarm/results/ if it exists; passes gracefully if absent
# Exit 0 = PASS, Exit 1 = FAIL (with structured error output)
#
# Wave patterns reference: skills/crank/references/wave-patterns.md
# (defines FIRE loop, acceptance checks, and wave checkpoint patterns)
set -euo pipefail

RESULT_FILE="${1:-}"

# No-argument mode: scan default evidence directory, pass gracefully if absent
if [[ -z "$RESULT_FILE" ]]; then
    EVIDENCE_DIR=".agents/swarm/results"
    if [[ ! -d "$EVIDENCE_DIR" ]]; then
        echo "SKIP: no evidence directory ($EVIDENCE_DIR) — nothing to validate"
        exit 0
    fi
    EVIDENCE_FILES=()
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        while IFS= read -r -d '' f; do
            [[ "$f" == *.json ]] && EVIDENCE_FILES+=("$f")
        done < <(git ls-files -z -- "$EVIDENCE_DIR" 2>/dev/null || true)
        if [[ ${#EVIDENCE_FILES[@]} -eq 0 ]]; then
            echo "SKIP: no tracked evidence files in $EVIDENCE_DIR — nothing to validate"
            exit 0
        fi
    else
        while IFS= read -r -d '' f; do
            EVIDENCE_FILES+=("$f")
        done < <(find "$EVIDENCE_DIR" -maxdepth 1 -name '*.json' -type f -print0 2>/dev/null)
    fi
    if [[ ${#EVIDENCE_FILES[@]} -eq 0 ]]; then
        echo "SKIP: no evidence files in $EVIDENCE_DIR — nothing to validate"
        exit 0
    fi
    TOTAL_ERRORS=0
    for ef in "${EVIDENCE_FILES[@]}"; do
        echo "--- Validating: $ef ---"
        if ! bash "$0" "$ef"; then
            TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
        fi
    done
    if [[ $TOTAL_ERRORS -gt 0 ]]; then
        echo "EVIDENCE BATCH FAILED: $TOTAL_ERRORS file(s) failed validation"
        exit 1
    fi
    echo "EVIDENCE BATCH PASS: ${#EVIDENCE_FILES[@]} file(s) validated"
    exit 0
fi

# Directory argument: validate all JSON files within
if [[ -d "$RESULT_FILE" ]]; then
    EVIDENCE_FILES=()
    while IFS= read -r -d '' f; do
        EVIDENCE_FILES+=("$f")
    done < <(find "$RESULT_FILE" -maxdepth 1 -name '*.json' -type f -print0 2>/dev/null)
    if [[ ${#EVIDENCE_FILES[@]} -eq 0 ]]; then
        echo "SKIP: no evidence files in $RESULT_FILE — nothing to validate"
        exit 0
    fi
    TOTAL_ERRORS=0
    for ef in "${EVIDENCE_FILES[@]}"; do
        echo "--- Validating: $ef ---"
        if ! bash "$0" "$ef"; then
            TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
        fi
    done
    if [[ $TOTAL_ERRORS -gt 0 ]]; then
        echo "EVIDENCE BATCH FAILED: $TOTAL_ERRORS file(s) failed validation"
        exit 1
    fi
    echo "EVIDENCE BATCH PASS: ${#EVIDENCE_FILES[@]} file(s) validated"
    exit 0
fi

if [[ ! -f "$RESULT_FILE" ]]; then
    echo "Usage: bash scripts/validate-swarm-evidence.sh [result-file.json|directory]"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq required for evidence validation"
    exit 1
fi

ERRORS=0
WARNINGS=0

# ---------------------------------------------------------------------------
# Schema shape validation (schemas/swarm-evidence.schema.json)
# Applies to ALL result files (not just completion-typed). Required fields:
#   - status (one of the documented enum values)
#   - task OR task_id (at least one)
# Optional ajv validation runs if ajv is installed; jq fallback otherwise.
# ---------------------------------------------------------------------------

# Verify file is valid JSON
if ! jq empty "$RESULT_FILE" >/dev/null 2>&1; then
    echo "FAIL: $RESULT_FILE is not valid JSON"
    exit 1
fi

# Recommended (warn-only): at least one identifier field. Filename is the
# canonical identifier so missing in-file ID is a soft failure to preserve
# historical evidence. New workers should set 'task'.
HAS_TASK=$(jq -e 'has("task") or has("task_id") or has("issue_id") or has("epic") or has("worker") or has("name")' "$RESULT_FILE" >/dev/null 2>&1 && echo "yes" || echo "no")
if [[ "$HAS_TASK" == "no" ]]; then
    echo "WARN: no identifier field (task/task_id/issue_id/epic/worker/name) — filename serves as ID"
    WARNINGS=$((WARNINGS + 1))
fi

# Required: status with valid enum
RAW_STATUS=$(jq -r '.status // ""' "$RESULT_FILE")
if [[ -z "$RAW_STATUS" ]]; then
    echo "FAIL: missing required field 'status'"
    ERRORS=$((ERRORS + 1))
else
    case "$RAW_STATUS" in
        done|complete|completed|pass|partial|failed|fail|blocked|not-applicable|already-implemented|research-only|skipped) ;;
        *)
            echo "FAIL: status '$RAW_STATUS' not in allowed enum (done|complete|completed|pass|partial|failed|fail|blocked|not-applicable|already-implemented|research-only|skipped)"
            ERRORS=$((ERRORS + 1))
            ;;
    esac
fi

# Optional ajv-based full schema check (graceful fallback)
SCHEMA_PATH="$(dirname "$0")/../schemas/swarm-evidence.schema.json"
if [[ -f "$SCHEMA_PATH" ]] && command -v ajv >/dev/null 2>&1; then
    if ! ajv validate -s "$SCHEMA_PATH" -d "$RESULT_FILE" >/dev/null 2>&1; then
        echo "WARN: ajv schema validation failed for $RESULT_FILE (run 'ajv validate -s $SCHEMA_PATH -d $RESULT_FILE' for details)"
        WARNINGS=$((WARNINGS + 1))
    fi
fi

# Parse result
TYPE=$(jq -r '.type // "unknown"' "$RESULT_FILE")
STATUS=$(jq -r '.status // "unknown"' "$RESULT_FILE")

# Blocked results skip evidence validation
if [[ "$TYPE" == "blocked" ]] || [[ "$STATUS" == "blocked" ]]; then
    echo "SKIP: blocked result — evidence not required"
    exit 0
fi

# Completion results require evidence
if [[ "$TYPE" == "completion" ]]; then
    # Check artifacts exist
    ARTIFACTS_COUNT=$(jq -r '.artifacts | length // 0' "$RESULT_FILE" 2>/dev/null || echo 0)
    if [[ "$ARTIFACTS_COUNT" -eq 0 ]]; then
        echo "FAIL: completion result missing artifacts array"
        ERRORS=$((ERRORS + 1))
    fi

    # Check evidence block exists
    HAS_EVIDENCE=$(jq -e '.evidence' "$RESULT_FILE" >/dev/null 2>&1 && echo "yes" || echo "no")
    if [[ "$HAS_EVIDENCE" == "no" ]]; then
        echo "FAIL: completion result missing evidence block"
        ERRORS=$((ERRORS + 1))
    else
        # Check required_checks array
        REQ_CHECKS=$(jq -r '.evidence.required_checks // [] | length' "$RESULT_FILE")
        if [[ "$REQ_CHECKS" -eq 0 ]]; then
            echo "FAIL: evidence.required_checks is empty — at least one check required"
            ERRORS=$((ERRORS + 1))
        fi

        # For each required check, verify it exists in checks and has PASS verdict
        for check_name in $(jq -r '.evidence.required_checks[]' "$RESULT_FILE" 2>/dev/null); do
            CHECK_EXISTS=$(jq -e --arg name "$check_name" '.evidence.checks[$name]' "$RESULT_FILE" >/dev/null 2>&1 && echo "yes" || echo "no")
            if [[ "$CHECK_EXISTS" == "no" ]]; then
                echo "FAIL: required check '$check_name' missing from evidence.checks"
                ERRORS=$((ERRORS + 1))
                continue
            fi

            VERDICT=$(jq -r --arg name "$check_name" '.evidence.checks[$name].verdict // "missing"' "$RESULT_FILE")
            if [[ "$VERDICT" == "FAIL" ]]; then
                echo "FAIL: required check '$check_name' has FAIL verdict"
                ERRORS=$((ERRORS + 1))
            elif [[ "$VERDICT" == "SKIP" ]]; then
                echo "WARN: required check '$check_name' has SKIP verdict"
                WARNINGS=$((WARNINGS + 1))
            elif [[ "$VERDICT" != "PASS" ]]; then
                echo "FAIL: required check '$check_name' has invalid verdict: $VERDICT"
                ERRORS=$((ERRORS + 1))
            fi
        done
    fi
fi

# Summary
if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "EVIDENCE VALIDATION FAILED: $ERRORS error(s), $WARNINGS warning(s)"
    exit 1
fi

if [[ $WARNINGS -gt 0 ]]; then
    echo ""
    echo "EVIDENCE VALIDATION WARN: $WARNINGS warning(s)"
    exit 0
fi

echo "EVIDENCE VALIDATION PASS"
exit 0

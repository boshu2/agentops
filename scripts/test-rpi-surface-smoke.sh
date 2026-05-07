#!/bin/bash
# test-evolve-cycle-smoke.sh — clean-room smoke for the static RPI surface.
#
# Runs a fixed set of static health checks an operator (or CI job) can use to
# verify the RPI surface is intact before scheduling a real cycle. Does NOT
# trigger an actual RPI run (no daemon, no workspace mutations).
#
# Each check prints one line:
#   [PASS]  check-name — detail
#   [WARN]  check-name — detail   (informational, does not fail the script)
#   [FAIL]  check-name — detail
#
# Final summary:
#   SMOKE: PASS (N checks)
#   SMOKE: FAIL (M failures, K warnings)
#
# Exit 0 only if zero [FAIL] lines were printed.
#
# Usage:
#   scripts/test-evolve-cycle-smoke.sh           # quiet mode
#   scripts/test-evolve-cycle-smoke.sh -v        # verbose: also print command output
#   scripts/test-evolve-cycle-smoke.sh --verbose # same

set -euo pipefail

VERBOSE=0
for arg in "$@"; do
    case "$arg" in
        -v|--verbose) VERBOSE=1 ;;
        -h|--help)
            sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown arg: $arg (use -v or --verbose)" >&2
            exit 2
            ;;
    esac
done

# Resolve repo root from any cwd inside the tree.
if ! REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"; then
    echo "[FAIL] repo-root — not inside a git working tree" >&2
    echo "SMOKE: FAIL (1 failures, 0 warnings)"
    exit 1
fi
cd "$REPO_ROOT"

PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0

emit() {
    # emit <level> <name> <detail>
    local level="$1" name="$2" detail="$3"
    printf '[%s] %s — %s\n' "$level" "$name" "$detail"
    case "$level" in
        PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
        WARN) WARN_COUNT=$((WARN_COUNT + 1)) ;;
        FAIL) FAIL_COUNT=$((FAIL_COUNT + 1)) ;;
    esac
}

# Run a command, capture combined output, optionally show under -v.
# Usage: run_capture <var-name-for-output> <cmd...>
run_capture() {
    local outvar="$1"
    shift
    local out rc
    out="$("$@" 2>&1)" && rc=0 || rc=$?
    if [[ "$VERBOSE" -eq 1 ]]; then
        printf '    $ %s\n' "$*"
        if [[ -n "$out" ]]; then
            printf '%s\n' "$out" | sed 's/^/      /'
        fi
        printf '    exit=%d\n' "$rc"
    fi
    printf -v "$outvar" '%s' "$out"
    return "$rc"
}

# ---------------------------------------------------------------------------
# Check 1: Bin presence (ao + bd)
# ---------------------------------------------------------------------------
check_bins() {
    local missing=()
    command -v ao >/dev/null 2>&1 || missing+=("ao")
    command -v bd >/dev/null 2>&1 || missing+=("bd")
    if [[ ${#missing[@]} -gt 0 ]]; then
        emit FAIL bin-presence "missing on PATH: ${missing[*]}"
        return
    fi

    # ao --version OR ao version (newer binaries use the latter).
    local ao_out
    if run_capture ao_out ao --version || run_capture ao_out ao version; then
        local first
        first="$(printf '%s\n' "$ao_out" | head -1)"
        emit PASS bin-presence "ao + bd present (ao: ${first})"
    else
        emit FAIL bin-presence "ao binary present but neither --version nor version exited 0"
        return
    fi

    # bd version probe is informational only — exotic builds may differ.
    # shellcheck disable=SC2034 # bd_out is set by run_capture via name-ref
    local bd_out
    if run_capture bd_out bd --version || run_capture bd_out bd version; then
        :
    else
        emit WARN bd-version "bd version probe failed (non-fatal)"
    fi
}

# ---------------------------------------------------------------------------
# Check 2: execution-packet schema parses as JSON
# ---------------------------------------------------------------------------
check_schema_valid() {
    local schema="schemas/execution-packet.schema.json"
    if [[ ! -f "$schema" ]]; then
        emit FAIL schema-valid "missing $schema"
        return
    fi
    local out
    if run_capture out python3 -c "import json,sys; json.load(open('$schema')); print('ok')"; then
        emit PASS schema-valid "$schema parses as JSON"
    else
        emit FAIL schema-valid "python3 json.load failed for $schema"
    fi
}

# ---------------------------------------------------------------------------
# Check 3: skill isolation lint clean
# ---------------------------------------------------------------------------
check_lint_clean() {
    local script="scripts/check-skill-isolation.sh"
    if [[ ! -x "$script" ]]; then
        emit FAIL lint-clean "$script missing or not executable"
        return
    fi
    local out
    if run_capture out bash "$script" -q; then
        emit PASS lint-clean "check-skill-isolation.sh -q passed"
    else
        emit FAIL lint-clean "check-skill-isolation.sh -q failed"
    fi
}

# ---------------------------------------------------------------------------
# Check 4: orphan-commit warning (informational only)
# ---------------------------------------------------------------------------
check_dangling() {
    local out
    out="$(git fsck --no-reflogs --no-progress 2>&1 | grep -i 'dangling commit' || true)"
    if [[ "$VERBOSE" -eq 1 && -n "$out" ]]; then
        printf '    git fsck --no-reflogs --no-progress | grep dangling-commit:\n'
        printf '%s\n' "$out" | head -5 | sed 's/^/      /'
    fi
    if [[ -z "$out" ]]; then
        emit PASS dangling-commits "git fsck reported no dangling commits"
    else
        local n
        n="$(printf '%s\n' "$out" | wc -l | tr -d ' ')"
        emit WARN dangling-commits "$n dangling commit(s) (informational)"
    fi
}

# ---------------------------------------------------------------------------
# Check 5: bead workspace probe
# ---------------------------------------------------------------------------
check_bd_workspace() {
    if ! command -v bd >/dev/null 2>&1; then
        emit WARN bd-workspace "bd not on PATH (covered by bin-presence)"
        return
    fi
    # `bd ready | head -1` can exit 141 (SIGPIPE) even when bd produced rows;
    # capture all output then take the first non-empty line ourselves.
    local out rc=0
    out="$(bd ready 2>&1)" || rc=$?
    local first
    first="$(printf '%s\n' "$out" | sed -n '/./{p;q;}')"
    if [[ "$VERBOSE" -eq 1 ]]; then
        printf '    $ bd ready (first non-empty line)\n'
        printf '      %s\n' "${first:-<empty>}"
        printf '    exit=%d\n' "$rc"
    fi
    if [[ -z "$first" || "$first" =~ ^([Ee]rror|FATAL) ]]; then
        emit WARN bd-workspace "bd ready returned nothing/error — workspace may not be initialized"
    else
        emit PASS bd-workspace "bd ready returned a row"
    fi
}

# ---------------------------------------------------------------------------
# Check 6: schema includes epic_criteria + bead_criteria (UW0/UW1 contract)
# ---------------------------------------------------------------------------
check_schema_fields() {
    local schema="schemas/execution-packet.schema.json"
    if [[ ! -f "$schema" ]]; then
        emit FAIL schema-fields "missing $schema"
        return
    fi
    local detail=""
    if command -v jq >/dev/null 2>&1; then
        local epic_t bead_t
        epic_t="$(jq -r '.properties.epic_criteria.type // "missing"' "$schema" 2>&1)"
        bead_t="$(jq -r '.properties.bead_criteria.type // "missing"' "$schema" 2>&1)"
        if [[ "$VERBOSE" -eq 1 ]]; then
            printf '    epic_criteria.type=%s bead_criteria.type=%s\n' "$epic_t" "$bead_t"
        fi
        if [[ "$epic_t" == "array" && ( "$bead_t" == "object" || "$bead_t" == "array" ) ]]; then
            emit PASS schema-fields "epic_criteria + bead_criteria present (jq)"
            return
        fi
        detail="epic=$epic_t bead=$bead_t"
    else
        # Fall back to grep if jq unavailable.
        if grep -F -q '"epic_criteria"' "$schema" && grep -F -q '"bead_criteria"' "$schema"; then
            emit PASS schema-fields "epic_criteria + bead_criteria present (grep)"
            return
        fi
        detail="grep did not find both keys"
    fi
    emit FAIL schema-fields "UW0/UW1 contract drift — $detail"
}

# ---------------------------------------------------------------------------
# Run all checks
# ---------------------------------------------------------------------------
check_bins
check_schema_valid
check_lint_clean
check_dangling
check_bd_workspace
check_schema_fields

TOTAL=$((PASS_COUNT + WARN_COUNT + FAIL_COUNT))

if [[ "$FAIL_COUNT" -eq 0 ]]; then
    echo "SMOKE: PASS ($TOTAL checks)"
    exit 0
else
    echo "SMOKE: FAIL ($FAIL_COUNT failures, $WARN_COUNT warnings)"
    exit 1
fi

#!/usr/bin/env bash
set -euo pipefail

# Toolchain Validate - Run all available linters/scanners
# Outputs structured findings to $TOOLCHAIN_OUTPUT_DIR (default: $TMPDIR/agentops-tooling/)
#
# Usage: ./scripts/toolchain-validate.sh [OPTIONS]
#
# Options:
#   --quick   Skip slow tools (tests, comprehensive scans)
#   --json    Output summary as JSON to stdout
#   --gate    Exit non-zero on CRITICAL or HIGH findings
#
# Exit Codes:
#   0 - Pass (no critical/high findings, or --gate not specified)
#   1 - Script error
#   2 - CRITICAL findings found (with --gate)
#   3 - HIGH findings only (with --gate)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"
OUTPUT_DIR="${TOOLCHAIN_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-tooling}"

# Parse arguments
QUICK=false
JSON_OUTPUT=false
GATE=false

for arg in "$@"; do
    case $arg in
        --quick) QUICK=true ;;
        --json) JSON_OUTPUT=true ;;
        --gate) GATE=true ;;
        --help|-h)
            head -20 "$0" | grep "^#" | sed 's/^# *//'
            exit 0
            ;;
        *)
            echo "Unknown option: $arg" >&2
            exit 1
            ;;
    esac
done

# Initialize output directory
mkdir -p "$OUTPUT_DIR"

# Determine scope (for --gate, default to changed files only)
TARGET_FILES=()

in_git_repo() {
    git rev-parse --git-dir >/dev/null 2>&1
}

collect_target_files() {
    if ! in_git_repo; then
        return 0
    fi

    local files=""

    # Prefer staged changes (pre-commit gate)
    files="$(git diff --name-only --cached 2>/dev/null || true)"
    if [[ -n "$files" ]]; then
        printf "%s\n" "$files"
        return 0
    fi

    # Then unstaged changes
    files="$(git diff --name-only 2>/dev/null || true)"
    if [[ -n "$files" ]]; then
        printf "%s\n" "$files"
        return 0
    fi

    # Finally, most recent commit (post-commit gate)
    files="$(git show --name-only --pretty=format: HEAD 2>/dev/null || true)"
    if [[ -n "$files" ]]; then
        printf "%s\n" "$files"
        return 0
    fi

    return 0
}

if [[ "$GATE" == "true" ]]; then
    while IFS= read -r f; do
        [[ -z "$f" ]] && continue
        TARGET_FILES+=("$REPO_ROOT/$f")
    done < <(collect_target_files)
fi

target_has_ext() {
    local ext="$1"
    if [[ "${#TARGET_FILES[@]}" -eq 0 ]]; then
        return 1
    fi
    local f
    for f in "${TARGET_FILES[@]}"; do
        [[ "$f" == *".$ext" ]] && return 0
    done
    return 1
}

target_has_any_ext() {
    local ext
    for ext in "$@"; do
        if target_has_ext "$ext"; then
            return 0
        fi
    done
    return 1
}

# Counters
CRITICAL_COUNT=0
HIGH_COUNT=0
MEDIUM_COUNT=0
LOW_COUNT=0
TOOLS_RUN=0
TOOLS_SKIPPED=0

# Tool output files and status
declare -A TOOL_STATUS

log() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo "$1"
    fi
}

run_tool() {
    local name="$1"
    local output_file="$OUTPUT_DIR/${name}.txt"
    shift

    if ! command -v "$1" &>/dev/null; then
        log "  [SKIP] $name - not installed"
        echo "NOT_INSTALLED" > "$output_file"
        TOOL_STATUS["$name"]="not_installed"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 1
    fi

    log "  [RUN]  $name"
    TOOLS_RUN=$((TOOLS_RUN + 1))
    return 0
}

discover_go_modules() {
    find "$REPO_ROOT" -name go.mod -type f -print0 2>/dev/null | xargs -0 -n1 dirname 2>/dev/null || true
}

ensure_json_or_error() {
    local tool="$1"
    local json_file="$2"
    local stderr_file="$3"

    if [[ -s "$stderr_file" ]] && [[ ! -s "$json_file" ]]; then
        {
            echo "ERROR"
            echo ""
            cat "$stderr_file"
        } > "$json_file"
        TOOL_STATUS["$tool"]="error"
        return 1
    fi

    if [[ ! -s "$json_file" ]]; then
        echo "ERROR: no output produced" > "$json_file"
        TOOL_STATUS["$tool"]="error"
        return 1
    fi

    if ! jq empty "$json_file" >/dev/null 2>&1; then
        {
            echo "ERROR: non-JSON output"
            echo ""
            cat "$json_file"
            if [[ -s "$stderr_file" ]]; then
                echo ""
                echo "STDERR:"
                cat "$stderr_file"
            fi
        } > "$json_file"
        TOOL_STATUS["$tool"]="error"
        return 1
    fi

    return 0
}

# ============================================================================
# TOOL: ruff (Python linting)
# ============================================================================
run_ruff() {
    local output_file="$OUTPUT_DIR/ruff.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_ext "py"; then
        echo "NO_PYTHON_FILES_IN_TARGET" > "$output_file"
        TOOL_STATUS["ruff"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "ruff" ruff; then return 0; fi

    # Check if there are Python files
    if ! find "$REPO_ROOT" -name "*.py" -type f | head -1 | grep -q .; then
        echo "NO_PYTHON_FILES" > "$output_file"
        TOOL_STATUS["ruff"]="skipped"
        return 0
    fi

    # Run ruff and capture output
    if ruff check "$REPO_ROOT" --output-format=concise > "$output_file" 2>&1; then
        echo "CLEAN" > "$output_file"
        TOOL_STATUS["ruff"]="pass"
    else
        # Ruff concise/full output doesn't expose stable severities; count all issues.
        local issues
        issues=$(grep -cE "^[^:]+:[0-9]+:[0-9]+:" "$output_file" 2>/dev/null || true)
        issues=${issues:-0}
        issues=$(echo "$issues" | tr -d '[:space:]')
        HIGH_COUNT=$((HIGH_COUNT + issues))
        TOOL_STATUS["ruff"]="findings"
    fi
}

# ============================================================================
# TOOL: golangci-lint (Go linting)
# ============================================================================
run_golangci() {
    local output_file="$OUTPUT_DIR/golangci-lint.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_any_ext go mod sum; then
        echo "NO_GO_CHANGES_IN_TARGET" > "$output_file"
        TOOL_STATUS["golangci-lint"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "golangci-lint" golangci-lint; then return 0; fi

    local modules
    modules="$(discover_go_modules)"
    if [[ -z "$modules" ]]; then
        echo "NO_GO_FILES" > "$output_file"
        TOOL_STATUS["golangci-lint"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    local cache_dir="$OUTPUT_DIR/.golangci-cache"
    local go_cache_dir="$OUTPUT_DIR/.go-cache"
    mkdir -p "$cache_dir" "$go_cache_dir"

    : > "$output_file"
    local had_findings=false

    while IFS= read -r module_dir; do
        [[ -z "$module_dir" ]] && continue
        {
            echo "== golangci-lint: $module_dir =="
        } >> "$output_file"

        if (cd "$module_dir" && GOLANGCI_LINT_CACHE="$cache_dir" GOCACHE="$go_cache_dir" golangci-lint run ./...) >> "$output_file" 2>&1; then
            echo "" >> "$output_file"
        else
            echo "" >> "$output_file"
            had_findings=true
        fi
    done <<< "$modules"

    local issues
    issues=$(grep -cE "^[^:]+:[0-9]+:[0-9]+:" "$output_file" 2>/dev/null || true)
    issues=${issues:-0}
    issues=$(echo "$issues" | tr -d '[:space:]')
    if [[ "$issues" -eq 0 ]] && [[ "$had_findings" == "false" ]]; then
        echo "CLEAN" > "$output_file"
        TOOL_STATUS["golangci-lint"]="pass"
    else
        HIGH_COUNT=$((HIGH_COUNT + issues))
        TOOL_STATUS["golangci-lint"]="findings"
    fi
}

# ============================================================================
# TOOL: gitleaks (secret scanning)
# ============================================================================
run_gitleaks() {
    local output_file="$OUTPUT_DIR/gitleaks.txt"

    if [[ "$QUICK" == "true" ]]; then
        echo "SKIPPED_QUICK_MODE" > "$output_file"
        TOOL_STATUS["gitleaks"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "gitleaks" gitleaks; then return 0; fi

    # Use repo config if available, --no-color to avoid ANSI codes
    local config_flag=()
    if [[ -f "$REPO_ROOT/.gitleaks.toml" ]]; then
        config_flag=(--config "$REPO_ROOT/.gitleaks.toml")
    fi

    if gitleaks detect --source="$REPO_ROOT" --no-git --no-color "${config_flag[@]}" > "$output_file" 2>&1; then
        echo "CLEAN" > "$output_file"
        TOOL_STATUS["gitleaks"]="pass"
    else
        # Count leaks - gitleaks outputs one block per finding
        local leaks
        leaks=$(grep -c "Secret:" "$output_file" 2>/dev/null || true)
        leaks=${leaks:-0}
        leaks=$(echo "$leaks" | tr -d '[:space:]')
        CRITICAL_COUNT=$((CRITICAL_COUNT + leaks))
        TOOL_STATUS["gitleaks"]="findings"
    fi
}

# ============================================================================
# TOOL: shellcheck (shell script linting)
# ============================================================================
run_shellcheck() {
    local output_file="$OUTPUT_DIR/shellcheck.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_ext "sh"; then
        echo "NO_SHELL_FILES_IN_TARGET" > "$output_file"
        TOOL_STATUS["shellcheck"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "shellcheck" shellcheck; then return 0; fi

    # Find all shell scripts
    local scripts
    if [[ "$GATE" == "true" ]] && [[ "${#TARGET_FILES[@]}" -gt 0 ]]; then
        scripts="$(printf "%s\n" "${TARGET_FILES[@]}" | grep -E '\\.sh$' || true)"
    else
        scripts="$(find "$REPO_ROOT" -name "*.sh" -type f ! -path "*/.git/*" 2>/dev/null || true)"
    fi

    if [[ -z "$scripts" ]]; then
        echo "NO_SHELL_FILES_IN_TARGET" > "$output_file"
        TOOL_STATUS["shellcheck"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    # Run shellcheck
    echo "$scripts" | xargs shellcheck -x -f gcc > "$output_file" 2>&1 || true

    if [[ ! -s "$output_file" ]]; then
        echo "CLEAN" > "$output_file"
        TOOL_STATUS["shellcheck"]="pass"
    else
        # Count by severity (shellcheck gcc format: "file:line:col: error: message")
        local errors warnings
        errors=$(grep -cE ": error:" "$output_file" 2>/dev/null || true)
        errors=${errors:-0}
        errors=$(echo "$errors" | tr -d '[:space:]')
        warnings=$(grep -cE ": warning:" "$output_file" 2>/dev/null || true)
        warnings=${warnings:-0}
        warnings=$(echo "$warnings" | tr -d '[:space:]')
        HIGH_COUNT=$((HIGH_COUNT + errors))
        MEDIUM_COUNT=$((MEDIUM_COUNT + warnings))
        if [[ $errors -gt 0 || $warnings -gt 0 ]]; then
            TOOL_STATUS["shellcheck"]="findings"
        else
            TOOL_STATUS["shellcheck"]="pass"
        fi
    fi
}

# ============================================================================
# TOOL: radon (Python complexity)
# ============================================================================
run_radon() {
    local output_file="$OUTPUT_DIR/radon.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_ext "py"; then
        echo "NO_PYTHON_FILES_IN_TARGET" > "$output_file"
        TOOL_STATUS["radon"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "radon" radon; then return 0; fi

    # Check if there are Python files
    if ! find "$REPO_ROOT" -name "*.py" -type f | head -1 | grep -q .; then
        echo "NO_PYTHON_FILES" > "$output_file"
        TOOL_STATUS["radon"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    # Run radon for cyclomatic complexity (min C = 11+)
    radon cc "$REPO_ROOT" -a -s --min C > "$output_file" 2>&1 || true

    if [[ ! -s "$output_file" ]]; then
        echo "CLEAN" > "$output_file"
        TOOL_STATUS["radon"]="pass"
    else
        # Count high complexity functions
        local complex
        complex=$(grep -cE "^\s+[A-Z] " "$output_file" 2>/dev/null || true)
        complex=${complex:-0}
        complex=$(echo "$complex" | tr -d '[:space:]')
        HIGH_COUNT=$((HIGH_COUNT + complex))
        if [[ $complex -gt 0 ]]; then
            TOOL_STATUS["radon"]="findings"
        else
            TOOL_STATUS["radon"]="pass"
        fi
    fi
}

# ============================================================================
# TOOL: pytest (Python tests) - skipped in quick mode
# ============================================================================
run_pytest() {
    if [[ "$QUICK" == "true" ]]; then
        log "  [SKIP] pytest - quick mode"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        echo "SKIPPED_QUICK_MODE" > "$OUTPUT_DIR/pytest.txt"
        TOOL_STATUS["pytest"]="skipped"
        return 0
    fi

    local output_file="$OUTPUT_DIR/pytest.txt"

    if ! run_tool "pytest" pytest; then return 0; fi

    # Check if there are test files
    if ! find "$REPO_ROOT" -name "test_*.py" -o -name "*_test.py" | head -1 | grep -q .; then
        echo "NO_TEST_FILES" > "$output_file"
        TOOL_STATUS["pytest"]="skipped"
        return 0
    fi

    # Run pytest with minimal output
    if pytest "$REPO_ROOT" --tb=short -q > "$output_file" 2>&1; then
        echo "PASS" >> "$output_file"
        TOOL_STATUS["pytest"]="pass"
    else
        local failures
        failures=$(grep -cE "^FAILED" "$output_file" 2>/dev/null || true)
        failures=${failures:-0}
        failures=$(echo "$failures" | tr -d '[:space:]')
        CRITICAL_COUNT=$((CRITICAL_COUNT + failures))
        TOOL_STATUS["pytest"]="findings"
    fi
}

# ============================================================================
# TOOL: go test - skipped in quick mode
# ============================================================================
run_gotest() {
    if [[ "$QUICK" == "true" ]]; then
        log "  [SKIP] go test - quick mode"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        echo "SKIPPED_QUICK_MODE" > "$OUTPUT_DIR/gotest.txt"
        TOOL_STATUS["go-test"]="skipped"
        return 0
    fi

    local output_file="$OUTPUT_DIR/gotest.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_any_ext go mod sum; then
        echo "NO_GO_CHANGES_IN_TARGET" > "$output_file"
        TOOL_STATUS["go-test"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "go-test" go; then return 0; fi

    local modules
    modules="$(discover_go_modules)"
    if [[ -z "$modules" ]]; then
        echo "NO_GO_MODULES" > "$output_file"
        TOOL_STATUS["go-test"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    local go_cache_dir="$OUTPUT_DIR/.go-cache"
    mkdir -p "$go_cache_dir"

    : > "$output_file"
    local had_failures=false
    local failures=0

    while IFS= read -r module_dir; do
        [[ -z "$module_dir" ]] && continue

        {
            echo "== go test: $module_dir =="
        } >> "$output_file"

        if (cd "$module_dir" && GOCACHE="$go_cache_dir" go test ./... -short) >> "$output_file" 2>&1; then
            echo "" >> "$output_file"
        else
            echo "" >> "$output_file"
            had_failures=true
        fi
    done <<< "$modules"

    failures=$(grep -c "^--- FAIL" "$output_file" 2>/dev/null || true)
    failures=${failures:-0}
    failures=$(echo "$failures" | tr -d '[:space:]')
    if [[ "$failures" -eq 0 ]] && [[ "$had_failures" == "false" ]]; then
        echo "PASS" >> "$output_file"
        TOOL_STATUS["go-test"]="pass"
    else
        CRITICAL_COUNT=$((CRITICAL_COUNT + failures))
        TOOL_STATUS["go-test"]="findings"
    fi
}

# ============================================================================
# TOOL: semgrep (SAST security patterns)
# ============================================================================
run_semgrep() {
    local output_file="$OUTPUT_DIR/semgrep.txt"
    local stderr_file="$OUTPUT_DIR/semgrep.stderr.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_any_ext go py js ts tsx jsx java rb php cs; then
        echo "NO_CODE_FILES_IN_TARGET" > "$output_file"
        TOOL_STATUS["semgrep"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "semgrep" semgrep; then return 0; fi

    : > "$stderr_file"

    local ssl_cert_file=""
    if command -v python3 >/dev/null 2>&1; then
        ssl_cert_file="$(python3 -c 'import certifi; print(certifi.where())' 2>/dev/null || true)"
    fi

    # Exclude rules expected in CLI/DevOps tooling:
    # dangerous-exec-command: CLI tool runs subprocesses by design
    # detected-pgp-private-key-block: pattern in security scanning script, not an actual key
    # path-join-resolve-traversal: skills installer uses path joins with user input by design
    # import-text-template: CLI uses text/template for output formatting
    # unsafe-deserialization-interface: standard Go JSON unmarshal into interface{}
    # dynamic-urllib-use-detected: reverse-engineer scripts fetch URLs by design
    local exclude_rules=(
        --exclude-rule go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
        --exclude-rule generic.secrets.security.detected-pgp-private-key-block.detected-pgp-private-key-block
        --exclude-rule javascript.lang.security.audit.path-traversal.path-join-resolve-traversal.path-join-resolve-traversal
        --exclude-rule go.lang.security.audit.xss.import-text-template.import-text-template
        --exclude-rule go.lang.security.deserialization.unsafe-deserialization-interface.go-unsafe-deserialization-interface
        --exclude-rule python.lang.security.audit.dynamic-urllib-use-detected.dynamic-urllib-use-detected
    )

    if [[ -n "$ssl_cert_file" ]]; then
        SSL_CERT_FILE="$ssl_cert_file" semgrep scan --config=auto "$REPO_ROOT" --json --quiet "${exclude_rules[@]}" > "$output_file" 2> "$stderr_file" || true
    else
        semgrep scan --config=auto "$REPO_ROOT" --json --quiet "${exclude_rules[@]}" > "$output_file" 2> "$stderr_file" || true
    fi

    if ! ensure_json_or_error "semgrep" "$output_file" "$stderr_file"; then
        return 0
    fi

    local critical high
    critical=$(jq '[.results[]? | select(.extra.severity == "ERROR")] | length' "$output_file" 2>/dev/null || echo 0)
    high=$(jq '[.results[]? | select(.extra.severity == "WARNING")] | length' "$output_file" 2>/dev/null || echo 0)
    critical=${critical:-0}
    high=${high:-0}
    critical=$(echo "$critical" | tr -d '[:space:]')
    high=$(echo "$high" | tr -d '[:space:]')
    CRITICAL_COUNT=$((CRITICAL_COUNT + critical))
    HIGH_COUNT=$((HIGH_COUNT + high))
    TOOL_STATUS["semgrep"]=$([[ "$critical" -gt 0 || "$high" -gt 0 ]] && echo "findings" || echo "pass")
}

# ============================================================================
# TOOL: trivy (dependency vulnerabilities)
# ============================================================================
run_trivy() {
    local output_file="$OUTPUT_DIR/trivy.txt"
    local stderr_file="$OUTPUT_DIR/trivy.stderr.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_any_ext go mod sum json lock yaml yml; then
        echo "NO_DEPENDENCY_CHANGES_IN_TARGET" > "$output_file"
        TOOL_STATUS["trivy"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "trivy" trivy; then return 0; fi

    : > "$stderr_file"

    local docker_cfg
    docker_cfg="$(mktemp -d)"

    local cache_dir="$OUTPUT_DIR/.trivy-cache"
    mkdir -p "$cache_dir"

    local db_repo="${TRIVY_DB_REPOSITORY:-ghcr.io/aquasecurity/trivy-db:2}"

    local db_flag=()
    if trivy fs --help 2>/dev/null | grep -q -- '--db-repository'; then
        db_flag=(--db-repository "$db_repo")
    fi

    DOCKER_CONFIG="$docker_cfg" TRIVY_CACHE_DIR="$cache_dir" trivy fs "$REPO_ROOT" \
        --severity CRITICAL,HIGH \
        --format json \
        "${db_flag[@]}" \
        > "$output_file" 2> "$stderr_file" || true
    rm -rf "$docker_cfg"

    # In sandboxed / offline environments, allow trivy to be skipped gracefully.
    if [[ -s "$stderr_file" ]] && grep -qiE 'no such host|dial tcp|lookup .*: no such host' "$stderr_file"; then
        {
            echo "SKIPPED: network unavailable"
            echo ""
            cat "$stderr_file"
        } > "$output_file"
        TOOL_STATUS["trivy"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! ensure_json_or_error "trivy" "$output_file" "$stderr_file"; then
        return 0
    fi

    local critical high
    critical=$(jq '[.Results[]?.Vulnerabilities[]? | select(.Severity == "CRITICAL")] | length' "$output_file" 2>/dev/null || echo 0)
    high=$(jq '[.Results[]?.Vulnerabilities[]? | select(.Severity == "HIGH")] | length' "$output_file" 2>/dev/null || echo 0)
    critical=${critical:-0}
    high=${high:-0}
    critical=$(echo "$critical" | tr -d '[:space:]')
    high=$(echo "$high" | tr -d '[:space:]')
    if [[ "$critical" -gt 0 ]] || [[ "$high" -gt 0 ]]; then
        CRITICAL_COUNT=$((CRITICAL_COUNT + critical))
        HIGH_COUNT=$((HIGH_COUNT + high))
        TOOL_STATUS["trivy"]="findings"
    else
        TOOL_STATUS["trivy"]="pass"
    fi
}

# ============================================================================
# TOOL: gosec (Go security)
# ============================================================================
run_gosec() {
    local output_file="$OUTPUT_DIR/gosec.txt"
    local stderr_file="$OUTPUT_DIR/gosec.stderr.txt"

    if [[ "$GATE" == "true" ]] && ! target_has_any_ext go mod sum; then
        echo "NO_GO_CHANGES_IN_TARGET" > "$output_file"
        TOOL_STATUS["gosec"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    if ! run_tool "gosec" gosec; then return 0; fi

    local modules
    modules="$(discover_go_modules)"
    if [[ -z "$modules" ]]; then
        echo "NO_GO_MODULES" > "$output_file"
        TOOL_STATUS["gosec"]="skipped"
        TOOLS_SKIPPED=$((TOOLS_SKIPPED + 1))
        return 0
    fi

    : > "$stderr_file"
    : > "$output_file"

    while IFS= read -r module_dir; do
        [[ -z "$module_dir" ]] && continue

        local module_json
        module_json="$(mktemp)"
        local module_stderr
        module_stderr="$(mktemp)"

        {
            echo "== gosec: $module_dir =="
        } >> "$output_file"

        # Exclude rules expected in CLI tools:
        # G104: unhandled errors (common in deferred cleanup)
        # G115: integer overflow uintptr->int (f.Fd() safe on all platforms)
        # G204: subprocess execution (CLI tool runs commands by design)
        # G301: dir perms (CLI creates user-owned dirs)
        # G302: file mode bits (CLI creates user-owned files)
        # G304: file path from variable (CLI takes paths as arguments)
        # G306: file perms (CLI creates user-owned files)
        # G702: command injection via taint (CLI runs user-specified commands)
        # G703: path traversal via taint (CLI operates on user-specified paths)
        (cd "$module_dir" && gosec -quiet -fmt json -exclude=G104,G115,G204,G301,G302,G304,G306,G702,G703 ./... > "$module_json" 2> "$module_stderr") || true

        if jq empty "$module_json" >/dev/null 2>&1; then
            cat "$module_json" >> "$output_file"
            echo "" >> "$output_file"
        else
            {
                echo "ERROR: gosec produced non-JSON output"
                if [[ -s "$module_stderr" ]]; then
                    echo ""
                    cat "$module_stderr"
                fi
            } >> "$stderr_file"
        fi

        rm -f "$module_json" "$module_stderr"
    done <<< "$modules"

    # If we have errors, keep tool status but don't treat as findings.
    if [[ -s "$stderr_file" ]]; then
        TOOL_STATUS["gosec"]="error"
        cat "$stderr_file" >> "$output_file"
        return 0
    fi

    # Count issues across combined JSON blocks (best-effort)
    local issues
    issues=$(grep -c '"severity":' "$output_file" 2>/dev/null || true)
    issues=${issues:-0}
    issues=$(echo "$issues" | tr -d '[:space:]')
    if [[ "$issues" -gt 0 ]]; then
        HIGH_COUNT=$((HIGH_COUNT + issues))
        TOOL_STATUS["gosec"]="findings"
    else
        TOOL_STATUS["gosec"]="pass"
    fi
}

# ============================================================================
# TOOL: hadolint (Dockerfile)
# ============================================================================
run_hadolint() {
    local output_file="$OUTPUT_DIR/hadolint.txt"

    if ! run_tool "hadolint" hadolint; then return 0; fi

    local dockerfiles
    dockerfiles=$(find "$REPO_ROOT" -name "Dockerfile*" -type f 2>/dev/null)
    if [[ -z "$dockerfiles" ]]; then
        echo "NO_DOCKERFILES" > "$output_file"
        TOOL_STATUS["hadolint"]="skipped"
        return 0
    fi

    if echo "$dockerfiles" | xargs hadolint --format json > "$output_file" 2>&1; then
        TOOL_STATUS["hadolint"]="pass"
    else
        local errors warnings
        errors=$(jq '[.[] | select(.level == "error")] | length' "$output_file" 2>/dev/null || echo 0)
        warnings=$(jq '[.[] | select(.level == "warning")] | length' "$output_file" 2>/dev/null || echo 0)
        errors=${errors:-0}
        warnings=${warnings:-0}
        errors=$(echo "$errors" | tr -d '[:space:]')
        warnings=$(echo "$warnings" | tr -d '[:space:]')
        HIGH_COUNT=$((HIGH_COUNT + errors))
        MEDIUM_COUNT=$((MEDIUM_COUNT + warnings))
        TOOL_STATUS["hadolint"]="findings"
    fi
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

log ""
log "Toolchain Validation"
log "===================="
log "Target: $REPO_ROOT"
log "Output: $OUTPUT_DIR"
if [[ "$GATE" == "true" ]] && [[ "${#TARGET_FILES[@]}" -gt 0 ]]; then
    log "Scope: changed files only"
fi
log ""

# Run all tools
log "Running tools..."
run_ruff
run_golangci
run_gitleaks
run_shellcheck
run_radon
run_semgrep
run_trivy
run_gosec
run_hadolint
run_pytest
run_gotest

log ""

# Compute gate status once
if [[ $CRITICAL_COUNT -gt 0 ]]; then
    GATE_STATUS="BLOCKED_CRITICAL"
elif [[ $HIGH_COUNT -gt 0 ]]; then
    GATE_STATUS="BLOCKED_HIGH"
else
    GATE_STATUS="PASS"
fi

# Build tools JSON object
TOOLS_JSON="{"
first=true
for tool in ruff golangci-lint gitleaks shellcheck radon semgrep trivy gosec hadolint pytest go-test; do
    status="${TOOL_STATUS[$tool]:-not_run}"
    if [[ "$first" == "true" ]]; then
        first=false
    else
        TOOLS_JSON="$TOOLS_JSON,"
    fi
    TOOLS_JSON="$TOOLS_JSON \"$tool\": \"$status\""
done
TOOLS_JSON="$TOOLS_JSON }"

# Generate summary
SUMMARY=$(cat <<EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "target": "$REPO_ROOT",
  "tools_run": $TOOLS_RUN,
  "tools_skipped": $TOOLS_SKIPPED,
  "tools": $TOOLS_JSON,
  "findings": {
    "critical": $CRITICAL_COUNT,
    "high": $HIGH_COUNT,
    "medium": $MEDIUM_COUNT,
    "low": $LOW_COUNT
  },
  "gate_status": "$GATE_STATUS",
  "output_dir": "$OUTPUT_DIR"
}
EOF
)

# Write summary file
echo "$SUMMARY" > "$OUTPUT_DIR/summary.json"

# Output based on mode
if [[ "$JSON_OUTPUT" == "true" ]]; then
    echo "$SUMMARY"
else
    log "Summary"
    log "-------"
    log "  Tools run: $TOOLS_RUN"
    log "  Tools skipped: $TOOLS_SKIPPED"
    log ""
    log "  Findings:"
    log "    CRITICAL: $CRITICAL_COUNT"
    log "    HIGH:     $HIGH_COUNT"
    log "    MEDIUM:   $MEDIUM_COUNT"
    log "    LOW:      $LOW_COUNT"
    log ""

    if [[ "$GATE_STATUS" == "BLOCKED_CRITICAL" ]]; then
        log "  Gate: BLOCKED - ${CRITICAL_COUNT} critical findings"
    elif [[ "$GATE_STATUS" == "BLOCKED_HIGH" ]]; then
        log "  Gate: BLOCKED - ${HIGH_COUNT} high findings"
    else
        log "  Gate: PASS"
    fi

    log ""
    log "Full output: $OUTPUT_DIR"
fi

# Exit code logic
if [[ "$GATE" == "true" ]]; then
    if [[ "$GATE_STATUS" == "BLOCKED_CRITICAL" ]]; then
        exit 2
    elif [[ "$GATE_STATUS" == "BLOCKED_HIGH" ]]; then
        exit 3
    fi
fi

exit 0

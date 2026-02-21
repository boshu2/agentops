#!/usr/bin/env bash
set -euo pipefail

# ci-local-release.sh
# Release-grade local CI gate. Mirrors validate/release pipeline checks locally
# and adds CLI smoke coverage for hooks install and RPI paths.
#
# Usage:
#   ./scripts/ci-local-release.sh              # full gate (parallel where possible)
#   ./scripts/ci-local-release.sh --fast       # skip heavy checks (~20s vs ~100s)
#   ./scripts/ci-local-release.sh --skip-e2e-install
#   ./scripts/ci-local-release.sh --security-mode quick
#
# Exit codes:
#   0 = all checks passed
#   1 = one or more checks failed

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="$REPO_ROOT/.agents/releases/local-ci/$RUN_ID"
mkdir -p "$ARTIFACT_DIR"

SKIP_E2E_INSTALL=false
SECURITY_MODE="full"
FAST_MODE=false

usage() {
    cat <<'USAGE'
Usage: scripts/ci-local-release.sh [options]

Options:
  --fast               Skip heavy checks (race tests, security gate, SBOM, hook integration)
  --skip-e2e-install   Skip tests/e2e-install-test.sh
  --security-mode      quick|full (default: full)
  -h, --help           Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --fast)
            FAST_MODE=true
            shift
            ;;
        --skip-e2e-install)
            SKIP_E2E_INSTALL=true
            shift
            ;;
        --security-mode)
            SECURITY_MODE="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

if [[ "$SECURITY_MODE" != "quick" && "$SECURITY_MODE" != "full" ]]; then
    echo "Invalid --security-mode: $SECURITY_MODE (expected quick or full)" >&2
    exit 1
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

errors=0

pass() { echo -e "${GREEN}  ✓${NC} $1"; }
fail() { echo -e "${RED}  ✗${NC} $1"; errors=$((errors + 1)); }
warn() { echo -e "${YELLOW}  !${NC} $1"; }

run_step() {
    local name="$1"
    shift
    echo ""
    echo -e "${BLUE}== $name ==${NC}"
    if "$@"; then
        pass "$name"
    else
        fail "$name"
    fi
}

# --- Parallel step infrastructure ---
# Each parallel step writes its exit code to a temp file.
# After wait, we collect results.

PARALLEL_DIR="$(mktemp -d)"
PARALLEL_PIDS=()
PARALLEL_NAMES=()

run_step_bg() {
    local name="$1"
    shift
    local slug
    slug="$(echo "$name" | tr ' /' '__' | tr -cd 'A-Za-z0-9_-')"
    (
        "$@" > "$PARALLEL_DIR/${slug}.out" 2>&1
        echo $? > "$PARALLEL_DIR/${slug}.rc"
    ) &
    PARALLEL_PIDS+=($!)
    PARALLEL_NAMES+=("$name|$slug")
}

collect_parallel() {
    # Wait for all background jobs
    for pid in "${PARALLEL_PIDS[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    # Report results
    for entry in "${PARALLEL_NAMES[@]}"; do
        local name="${entry%%|*}"
        local slug="${entry##*|}"
        local rc_file="$PARALLEL_DIR/${slug}.rc"
        local out_file="$PARALLEL_DIR/${slug}.out"

        echo ""
        echo -e "${BLUE}== $name ==${NC}"

        # Show output (truncated to avoid noise)
        if [[ -f "$out_file" ]]; then
            local lines
            lines=$(wc -l < "$out_file")
            if [[ "$lines" -gt 20 ]]; then
                tail -20 "$out_file"
                echo "  ... ($lines lines total, showing last 20)"
            else
                cat "$out_file"
            fi
        fi

        local rc=1
        if [[ -f "$rc_file" ]]; then
            rc=$(cat "$rc_file")
        fi

        if [[ "$rc" -eq 0 ]]; then
            pass "$name"
        else
            fail "$name"
        fi
    done

    # Reset for next parallel batch
    PARALLEL_PIDS=()
    PARALLEL_NAMES=()
}

check_required_cmds() {
    local missing=0
    local tools=("bash" "git" "jq" "go" "shellcheck")
    for tool in "${tools[@]}"; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            echo "Missing required tool: $tool"
            missing=1
        fi
    done

    if ! command -v markdownlint >/dev/null 2>&1 && ! command -v npx >/dev/null 2>&1; then
        echo "Missing markdownlint runner: install markdownlint-cli or npx"
        missing=1
    fi

    [[ "$missing" -eq 0 ]]
}

run_shellcheck() {
    local files=()
    while IFS= read -r -d '' file; do
        files+=("$file")
    done < <(find . -name "*.sh" -type f -not -path "./.git/*" -print0)

    if [[ "${#files[@]}" -eq 0 ]]; then
        echo "No shell files found."
        return 0
    fi

    shellcheck --severity=error "${files[@]}"
}

run_markdownlint() {
    local md_files=()
    while IFS= read -r file; do
        md_files+=("$file")
    done < <(git ls-files '*.md')

    if [[ "${#md_files[@]}" -eq 0 ]]; then
        echo "No tracked markdown files found."
        return 0
    fi

    if command -v markdownlint >/dev/null 2>&1; then
        markdownlint "${md_files[@]}"
    else
        npx -y markdownlint-cli "${md_files[@]}"
    fi
}

run_security_scan_patterns() {
    local patterns=(
        "password.*=.*['\"][^'\"]{8,}['\"]"
        "api[_-]?key.*=.*['\"][^'\"]{16,}['\"]"
        "secret.*=.*['\"][^'\"]{8,}['\"]"
        "(access|auth|refresh|bearer)[_-]?token.*=.*['\"][^'\"]{16,}['\"]"
        "AWS[_A-Z]*=.*['\"][A-Z0-9]{16,}['\"]"
    )

    local found=0
    for pattern in "${patterns[@]}"; do
        if grep -r -i -E "$pattern" \
            --binary-files=without-match \
            --exclude-dir=.git \
            --exclude-dir=.agents \
            --exclude-dir=.tmp \
            --exclude-dir=tests \
            --exclude-dir=testdata \
            --exclude-dir=cli/testdata \
            --exclude-dir=cli/bin \
            --exclude="ao" \
            --exclude="*.md" \
            --exclude="*.jsonl" \
            --exclude="*.sh" \
            --exclude="validate.yml" \
            . 2>/dev/null; then
            found=1
        fi
    done

    [[ "$found" -eq 0 ]]
}

run_dangerous_pattern_scan() {
    local dangerous=(
        "rm -rf /"
        "curl.*\\| *sh"
        "curl.*\\| *bash"
        "wget.*\\| *sh"
    )

    local found=0
    for pattern in "${dangerous[@]}"; do
        if grep -r -E "$pattern" \
            --binary-files=without-match \
            --include="*.sh" \
            --exclude-dir=.git \
            --exclude-dir=.agents \
            --exclude-dir=.tmp \
            --exclude-dir=tests \
            --exclude-dir=cli/testdata \
            --exclude="install-opencode.sh" \
            --exclude="ci-local-release.sh" \
            . 2>/dev/null; then
            echo "Found dangerous pattern: $pattern"
            found=1
        fi
    done

    [[ "$found" -eq 0 ]]
}

check_manifest_version_consistency() {
    local plugin_version
    local marketplace_meta_version
    local marketplace_plugin_version

    plugin_version="$(jq -r '.version' .claude-plugin/plugin.json)"
    marketplace_meta_version="$(jq -r '.metadata.version' .claude-plugin/marketplace.json)"
    marketplace_plugin_version="$(jq -r '.plugins[0].version' .claude-plugin/marketplace.json)"

    if [[ "$plugin_version" != "$marketplace_meta_version" ]]; then
        echo "Version mismatch: plugin.json=$plugin_version, marketplace metadata=$marketplace_meta_version"
        return 1
    fi
    if [[ "$plugin_version" != "$marketplace_plugin_version" ]]; then
        echo "Version mismatch: plugin.json=$plugin_version, marketplace plugins[0]=$marketplace_plugin_version"
        return 1
    fi

    echo "Version consistency OK: $plugin_version"
    return 0
}

run_go_build_and_tests() {
    (
        cd cli
        go test -race -coverprofile=coverage.out -covermode=atomic ./... -v
        go tool cover -func=coverage.out | tail -1
    )
}

run_go_build_only() {
    (
        cd cli
        go build ./cmd/ao/
        go vet ./...
    )
}

run_release_binary_validation() {
    local version
    version="$(git describe --tags --always --dirty 2>/dev/null || true)"
    if [[ -z "$version" ]]; then
        version="v$(jq -r '.version' .claude-plugin/plugin.json)"
    fi

    (
        cd cli
        make build
    )

    ./scripts/validate-release.sh "$REPO_ROOT/cli/bin/ao" "$version"
}

generate_sbom_artifacts() {
    local version
    local cdx_file
    local spdx_file

    version="$(jq -r '.version' .claude-plugin/plugin.json)"
    cdx_file="$ARTIFACT_DIR/sbom-v${version}.cyclonedx.json"
    spdx_file="$ARTIFACT_DIR/sbom-v${version}.spdx.json"

    trivy fs --format cyclonedx --output "$cdx_file" "$REPO_ROOT" >/dev/null
    trivy fs --format spdx-json --output "$spdx_file" "$REPO_ROOT" >/dev/null

    jq -e '.bomFormat == "CycloneDX"' "$cdx_file" >/dev/null
    jq -e '.spdxVersion' "$spdx_file" >/dev/null

    echo "SBOM (CycloneDX): $cdx_file"
    echo "SBOM (SPDX):      $spdx_file"
}

run_security_gate() {
    local output_file="$ARTIFACT_DIR/security-gate-${SECURITY_MODE}.json"
    ./scripts/security-gate.sh --mode "$SECURITY_MODE" --require-tools --json > "$output_file"
    jq -e '.gate_status' "$output_file" >/dev/null
    echo "Security report:  $output_file"
}

run_hooks_install_smoke() {
    local tmp_home
    tmp_home="$(mktemp -d)"
    local rc=0

    HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" hooks install || rc=$?
    if [[ "$rc" -eq 0 ]]; then
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" hooks show || rc=$?
    fi
    if [[ "$rc" -eq 0 ]]; then
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" hooks install --full --source-dir "$REPO_ROOT" --force || rc=$?
    fi
    if [[ "$rc" -eq 0 ]] && [[ ! -f "$tmp_home/.claude/settings.json" ]]; then
        rc=1
    fi
    if [[ "$rc" -eq 0 ]] && [[ ! -f "$tmp_home/.agentops/hooks/session-start.sh" ]]; then
        rc=1
    fi

    rm -rf "$tmp_home"
    return "$rc"
}

run_init_hooks_rpi_smoke() {
    local tmp_home
    local tmp_repo
    tmp_home="$(mktemp -d)"
    tmp_repo="$(mktemp -d)"
    local rc=0

    git -C "$tmp_repo" init -q
    (
        cd "$tmp_repo"
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" init --hooks
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi status
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi --help >/dev/null
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi phased --help >/dev/null
    ) || rc=$?

    rm -rf "$tmp_home" "$tmp_repo"
    return "$rc"
}

# ═══════════════════════════════════════════════════════
#  Execution
# ═══════════════════════════════════════════════════════

START_TIME=$(date +%s)

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
if [[ "$FAST_MODE" == "true" ]]; then
    echo -e "${BLUE}  AgentOps Local CI (Release Gate) — FAST MODE${NC}"
    echo -e "${YELLOW}  Skipping: race tests, security gate, SBOM, hook integration${NC}"
else
    echo -e "${BLUE}  AgentOps Local CI (Release Gate)${NC}"
fi
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo "Artifacts: $ARTIFACT_DIR"

# ── Phase 1: Quick sequential checks (must pass before heavy work) ──

run_step "Required tool check" check_required_cmds

# ── Phase 2: Parallel independent checks ──
# These have zero dependencies on each other.

run_step_bg "Doc-release gate" ./tests/docs/validate-doc-release.sh
run_step_bg "Manifest schema validation" ./scripts/validate-manifests.sh --repo-root "$REPO_ROOT"
run_step_bg "Manifest version consistency" check_manifest_version_consistency
run_step_bg "Hook preflight" ./scripts/validate-hook-preflight.sh
run_step_bg "Embedded sync check" ./scripts/validate-embedded-sync.sh
run_step_bg "Secret pattern scan" run_security_scan_patterns
run_step_bg "Dangerous shell pattern scan" run_dangerous_pattern_scan

collect_parallel

# ── Phase 3: Parallel medium-weight checks ──

run_step_bg "CLI docs parity" ./scripts/generate-cli-reference.sh --check
run_step_bg "ShellCheck" run_shellcheck
run_step_bg "Markdownlint" run_markdownlint
run_step_bg "Smoke tests" ./tests/smoke-test.sh --verbose
run_step_bg "CLI integration smoke tests" ./tests/integration/test-cli-commands.sh

collect_parallel

# ── Phase 4: Heavy checks (skipped in --fast mode) ──

if [[ "$FAST_MODE" == "true" ]]; then
    warn "Skipped Go race tests (--fast)"
    warn "Skipped Hook integration tests (--fast)"
    warn "Skipped SBOM generation (--fast)"
    warn "Skipped Security gate (--fast)"

    # Still build the binary (fast) and run smoke tests against it
    run_step "Go build + vet" run_go_build_only
    run_step "Release binary validation" run_release_binary_validation
else
    # These are the heavy hitters — run them in parallel
    run_step_bg "Go build + race tests" run_go_build_and_tests
    run_step_bg "Hook integration tests" ./tests/hooks/test-hooks.sh
    run_step_bg "Generate SBOM artifacts (CycloneDX + SPDX)" generate_sbom_artifacts
    run_step_bg "Security toolchain gate (${SECURITY_MODE}, require tools)" run_security_gate

    collect_parallel

    run_step "Release binary validation" run_release_binary_validation
fi

# ── Phase 5: CLI smoke tests (need built binary) ──

run_step_bg "Hook install smoke (minimal + full)" run_hooks_install_smoke
run_step_bg "ao init --hooks + ao rpi smoke" run_init_hooks_rpi_smoke

collect_parallel

# ── Phase 6: E2E (optional) ──

if [[ "$SKIP_E2E_INSTALL" == "true" ]]; then
    warn "Skipped E2E install test (--skip-e2e-install)"
else
    run_step "E2E install test" ./tests/e2e-install-test.sh
fi

# ═══════════════════════════════════════════════════════
#  Summary
# ═══════════════════════════════════════════════════════

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
if [[ "$errors" -gt 0 ]]; then
    echo -e "${RED}  LOCAL CI FAILED ($errors failing check(s)) [${ELAPSED}s]${NC}"
    echo "  Scan/SBOM artifacts: $ARTIFACT_DIR"
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    rm -rf "$PARALLEL_DIR"
    exit 1
fi

echo -e "${GREEN}  LOCAL CI PASSED [${ELAPSED}s]${NC}"
echo "  Scan/SBOM artifacts: $ARTIFACT_DIR"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
rm -rf "$PARALLEL_DIR"
exit 0

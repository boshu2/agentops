#!/usr/bin/env bash
set -euo pipefail

# ci-local-release.sh
# Release-grade local CI gate. Mirrors validate/release pipeline checks locally
# and adds CLI smoke coverage for hooks install and RPI paths.
#
# Usage:
#   ./scripts/ci-local-release.sh              # full gate (parallel where possible)
#   ./scripts/ci-local-release.sh --fast       # skip heavy checks (~20s vs ~100s)
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
SECURITY_TMP_BASE="${TMPDIR:-/tmp}/agentops-security-local-ci/$RUN_ID"

SECURITY_MODE="full"
FAST_MODE=false
RELEASE_VERSION_OVERRIDE=""

USER_MAX_JOBS=""

usage() {
    cat <<'USAGE'
Usage: scripts/ci-local-release.sh [options]

Options:
  --fast               Skip heavy checks (race tests, security gate, SBOM, hook integration)
  --release-version V  Record artifacts against the target release version (for release audits)
  --security-mode      quick|full (default: full)
  --jobs N             Max parallel jobs (default: half CPU cores, min 4)
  -h, --help           Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --fast)
            FAST_MODE=true
            shift
            ;;
        --release-version)
            RELEASE_VERSION_OVERRIDE="${2:-}"
            shift 2
            ;;
        --security-mode)
            SECURITY_MODE="${2:-}"
            shift 2
            ;;
        --jobs)
            USER_MAX_JOBS="${2:-}"
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

if [[ -n "$RELEASE_VERSION_OVERRIDE" ]]; then
    RELEASE_VERSION_OVERRIDE="${RELEASE_VERSION_OVERRIDE#v}"
    if [[ ! "$RELEASE_VERSION_OVERRIDE" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
        echo "Invalid --release-version: $RELEASE_VERSION_OVERRIDE" >&2
        exit 1
    fi
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

release_version() {
    if [[ -n "$RELEASE_VERSION_OVERRIDE" ]]; then
        printf '%s\n' "$RELEASE_VERSION_OVERRIDE"
        return 0
    fi

    jq -r '.version' .claude-plugin/plugin.json
}

artifact_dir_rel() {
    printf '.agents/releases/local-ci/%s\n' "$RUN_ID"
}

# --- Parallel step infrastructure ---
# Each parallel step writes its exit code to a temp file.
# After wait, we collect results.
# Concurrency is capped at MAX_JOBS to avoid CPU saturation.

PARALLEL_DIR="$(mktemp -d)"
ALL_PIDS=()     # every PID ever spawned (for cleanup)
PARALLEL_PIDS=()
PARALLEL_NAMES=()

# Cap parallel jobs: half the cores or 4, whichever is larger.
if command -v sysctl >/dev/null 2>&1; then
    _NCPU=$(sysctl -n hw.logicalcpu 2>/dev/null || echo 4)
elif [[ -f /proc/cpuinfo ]]; then
    _NCPU=$(grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 4)
else
    _NCPU=4
fi
MAX_JOBS=$(( _NCPU / 2 ))
[[ "$MAX_JOBS" -lt 4 ]] && MAX_JOBS=4
if [[ -n "$USER_MAX_JOBS" ]]; then
    MAX_JOBS="$USER_MAX_JOBS"
fi

# --- Cleanup trap: kill leaked children and temp dirs ---
cleanup() {
    local sig="${1:-EXIT}"
    # Kill any surviving background PIDs
    for pid in "${ALL_PIDS[@]}"; do
        kill "$pid" 2>/dev/null && wait "$pid" 2>/dev/null || true
    done
    rm -rf "$PARALLEL_DIR"
    if [[ "$sig" != "EXIT" ]]; then
        echo ""
        echo -e "${RED}  Interrupted — cleaned up ${#ALL_PIDS[@]} background job(s)${NC}"
        exit 130
    fi
}
trap 'cleanup INT'  INT
trap 'cleanup TERM' TERM
trap 'cleanup EXIT' EXIT

# _throttle waits until fewer than MAX_JOBS are running.
_throttle() {
    while true; do
        local running=0
        for pid in "${PARALLEL_PIDS[@]}"; do
            kill -0 "$pid" 2>/dev/null && running=$((running + 1))
        done
        [[ "$running" -lt "$MAX_JOBS" ]] && break
        sleep 0.2
    done
}

run_step_bg() {
    local name="$1"
    shift
    _throttle
    local slug
    slug="$(echo "$name" | tr ' /' '__' | tr -cd 'A-Za-z0-9_-')"
    (
        "$@" > "$PARALLEL_DIR/${slug}.out" 2>&1
        echo $? > "$PARALLEL_DIR/${slug}.rc"
    ) &
    PARALLEL_PIDS+=($!)
    ALL_PIDS+=($!)
    PARALLEL_NAMES+=("$name|$slug")
}

collect_parallel() {
    # Wait for all background jobs in this batch
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
    done < <(find . -name "*.sh" -type f \
        -not -path "./.git/*" \
        -not -path "./.claude/*" \
        -not -path "./.agents/*" \
        -print0 2>/dev/null)

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
            --exclude-dir=.claude \
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
            --exclude="*_test.go" \
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
            --exclude-dir=.claude \
            --exclude-dir=.agents \
            --exclude-dir=.tmp \
            --exclude-dir=tests \
            --exclude-dir=cli/testdata \
            --exclude="install-opencode.sh" \
            --exclude="install-codex.sh" \
            --exclude="install-codex-plugin.sh" \
            --exclude="install-codex-native-skills.sh" \
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
        go build ./cmd/ao/
        go vet ./...
        go test -race -coverprofile=coverage.out -covermode=atomic -count=1 ./...
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

write_release_artifact_manifest() {
    if ! command -v jq >/dev/null 2>&1; then
        echo "Skipping release artifact manifest: jq unavailable"
        return 0
    fi

    local version
    local repo_version
    local generated_at
    local manifest_file
    local sbom_cyclonedx=""
    local sbom_spdx=""
    local security_report=""
    local fast_mode_json=false

    version="$(release_version)"
    repo_version="$(jq -r '.version' .claude-plugin/plugin.json)"
    generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    manifest_file="$ARTIFACT_DIR/release-artifacts.json"

    [[ "$FAST_MODE" == "true" ]] && fast_mode_json=true

    if [[ -f "$ARTIFACT_DIR/sbom-v${version}.cyclonedx.json" ]]; then
        sbom_cyclonedx="sbom-v${version}.cyclonedx.json"
    fi
    if [[ -f "$ARTIFACT_DIR/sbom-v${version}.spdx.json" ]]; then
        sbom_spdx="sbom-v${version}.spdx.json"
    fi
    if [[ -f "$ARTIFACT_DIR/security-gate-${SECURITY_MODE}.json" ]]; then
        security_report="security-gate-${SECURITY_MODE}.json"
    fi

    jq -n \
        --arg run_id "$RUN_ID" \
        --arg generated_at "$generated_at" \
        --arg artifact_dir "$(artifact_dir_rel)" \
        --arg release_version "$version" \
        --arg repo_version "$repo_version" \
        --arg security_mode "$SECURITY_MODE" \
        --arg sbom_cyclonedx "$sbom_cyclonedx" \
        --arg sbom_spdx "$sbom_spdx" \
        --arg security_report "$security_report" \
        --argjson fast_mode "$fast_mode_json" \
        '{
          schema_version: 1,
          run_id: $run_id,
          generated_at: $generated_at,
          artifact_dir: $artifact_dir,
          release_version: $release_version,
          repo_version: $repo_version,
          fast_mode: $fast_mode,
          security_mode: $security_mode,
          sbom_cyclonedx: (if $sbom_cyclonedx == "" then null else $sbom_cyclonedx end),
          sbom_spdx: (if $sbom_spdx == "" then null else $sbom_spdx end),
          security_report: (if $security_report == "" then null else $security_report end)
        }' > "$manifest_file"

    echo "Release artifact manifest: $manifest_file"
}

generate_sbom_artifacts() {
    local version
    local cdx_file
    local spdx_file

    version="$(release_version)"
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
    local security_dir="$SECURITY_TMP_BASE/security"
    local tooling_dir="$SECURITY_TMP_BASE/tooling"
    mkdir -p "$security_dir" "$tooling_dir"

    SECURITY_GATE_OUTPUT_DIR="$security_dir" \
    TOOLCHAIN_OUTPUT_DIR="$tooling_dir" \
    TOOLCHAIN_GITLEAKS_MODE="${TOOLCHAIN_GITLEAKS_MODE:-range}" \
    TOOLCHAIN_GITLEAKS_RANGE="${TOOLCHAIN_GITLEAKS_RANGE:-origin/main..HEAD}" \
    TOOLCHAIN_GITLEAKS_GOMAXPROCS="${TOOLCHAIN_GITLEAKS_GOMAXPROCS:-2}" \
    ./scripts/security-gate.sh --mode "$SECURITY_MODE" --json > "$output_file"
    jq -e '.gate_status' "$output_file" >/dev/null
    echo "Security report:  $output_file"
    echo "Security artifacts: $security_dir"
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
echo "Max parallel jobs: $MAX_JOBS"

# ── Phase 1: Quick sequential checks (must pass before heavy work) ──

run_step "Required tool check" check_required_cmds

# ── Phase 2: Parallel independent checks ──
# These have zero dependencies on each other.

run_step_bg "Doc-release gate" ./tests/docs/validate-doc-release.sh
run_step_bg "Manifest schema validation" ./scripts/validate-manifests.sh --repo-root "$REPO_ROOT"
run_step_bg "Manifest version consistency" check_manifest_version_consistency
run_step_bg "Hook preflight" ./scripts/validate-hook-preflight.sh
run_step_bg "Hooks/docs parity" ./scripts/validate-hooks-doc-parity.sh
run_step_bg "CI policy/docs parity" ./scripts/validate-ci-policy-parity.sh
run_step_bg "Worktree disposition gate" ./scripts/check-worktree-disposition.sh
run_step_bg "Skill integrity" bash ./skills/heal-skill/scripts/heal.sh --strict
run_step_bg "Skill runtime parity" bash ./scripts/validate-skill-runtime-parity.sh
run_step_bg "Codex runtime sections" bash ./scripts/validate-codex-runtime-sections.sh
# Codex skill parity removed — skills-codex/ is manually maintained
# run_step_bg "Codex skill parity" bash ./scripts/validate-codex-skill-parity.sh
# run_step_bg "Codex install bundle parity" bash ./scripts/validate-codex-install-bundle.sh
run_step_bg "Codex artifact manifest" bash ./scripts/validate-codex-generated-manifest.sh
run_step_bg "Codex artifact metadata" bash ./scripts/validate-codex-generated-artifacts.sh --scope worktree
run_step_bg "Codex backbone prompts" bash ./scripts/validate-codex-backbone-prompts.sh
run_step_bg "Next-work contract parity" bash ./scripts/validate-next-work-contract-parity.sh
run_step_bg "Skill runtime formats" bash ./scripts/validate-skill-runtime-formats.sh
run_step_bg "Contract compatibility gate" ./scripts/check-contract-compatibility.sh
run_step_bg "Embedded sync check" ./scripts/validate-embedded-sync.sh
run_step_bg "Secret pattern scan" run_security_scan_patterns
run_step_bg "Dangerous shell pattern scan" run_dangerous_pattern_scan
run_step_bg "Skill CLI snippets" bash ./scripts/validate-skill-cli-snippets.sh
run_step_bg "Command/test pairing gate" ./scripts/check-go-command-test-pair.sh
run_step_bg "MemRL feedback loop health" ./scripts/check-memrl-health.sh
run_step_bg "Doctor health check" ./scripts/check-doctor-health.sh
run_step_bg "Release cadence check" ./scripts/release-cadence-check.sh

collect_parallel

# ── Phase 3: Parallel medium-weight checks ──

run_step_bg "CLI docs parity" ./scripts/generate-cli-reference.sh --check
run_step_bg "ShellCheck" run_shellcheck
run_step_bg "Markdownlint" run_markdownlint
run_step_bg "Smoke tests" ./tests/smoke-test.sh --verbose
run_step_bg "Skill lint" bash ./tests/skills/run-all.sh
run_step_bg "Headless runtime skill smoke" bash ./scripts/validate-headless-runtime-skills.sh
run_step_bg "CLI integration smoke tests" ./tests/integration/test-cli-commands.sh
run_step_bg "Command/test pairing gate tests" ./tests/scripts/test-go-command-test-pair.sh
run_step_bg "Go fast scope tests" bats ./tests/scripts/validate-go-fast.bats
run_step_bg "Skill runtime parity tests" bash ./tests/scripts/test-skill-runtime-parity.sh
run_step_bg "Skill CLI snippet tests" bash ./tests/scripts/test-skill-cli-snippets.sh
run_step_bg "Codex plugin install tests" bash ./tests/scripts/test-codex-plugin-install.sh
run_step_bg "Codex native install tests" bash ./tests/scripts/test-codex-native-skills-install.sh
run_step_bg "Codex artifact manifest tests" bash ./tests/scripts/test-codex-generated-manifest.sh
run_step_bg "Codex artifact metadata tests" bash ./tests/scripts/test-codex-generated-artifacts.sh
run_step_bg "Codex backbone prompt tests" bash ./tests/scripts/test-codex-backbone-prompts.sh
run_step_bg "Dev hook install tests" bash ./tests/scripts/test-install-dev-hooks.sh
run_step_bg "Git hook shim tests" bash ./tests/scripts/test-githook-shims.sh
run_step_bg "Validate-local tests" bash ./tests/scripts/test-validate-local.sh
run_step_bg "Headless runtime skill smoke tests" bash ./tests/scripts/test-headless-runtime-skills.sh
run_step_bg "Constraint compiler BATS wrapper" ./tests/hooks/test-constraint-compiler.sh
run_step_bg "cmd/ao coverage floor gate" ./scripts/check-cmdao-coverage-floor.sh

collect_parallel

# ── Phase 3b: Remote-parity checks ──
# These run in CI (validate.yml) but were missing from local gate.

run_step_bg "Coverage ratchet check" ./scripts/coverage-ratchet.sh --check
run_step_bg "Skill schema validation" ./scripts/validate-skill-schema.sh --verbose
run_step_bg "Learning coherence" ./scripts/validate-learning-coherence.sh
run_step_bg "JSON flag consistency" ./tests/cli/test-json-flag-consistency.sh

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
run_step_bg "Release smoke test (all commands)" ./scripts/release-smoke-test.sh --skip-build

collect_parallel

# ═══════════════════════════════════════════════════════
#  Summary
# ═══════════════════════════════════════════════════════

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

write_release_artifact_manifest

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
if [[ "$errors" -gt 0 ]]; then
    echo -e "${RED}  LOCAL CI FAILED ($errors failing check(s)) [${ELAPSED}s]${NC}"
    echo "  Scan/SBOM artifacts: $ARTIFACT_DIR"
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    exit 1
fi

echo -e "${GREEN}  LOCAL CI PASSED [${ELAPSED}s]${NC}"
echo "  Scan/SBOM artifacts: $ARTIFACT_DIR"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
exit 0

#!/usr/bin/env bash
set -euo pipefail

# ci-local-release.sh
# Release-grade local CI gate. Mirrors validate/release pipeline checks locally
# and adds CLI smoke coverage for init and RPI paths.
#
# Usage:
#   ./scripts/ci-local-release.sh              # full gate (parallel where possible)
#   ./scripts/ci-local-release.sh --fast       # skip heavy checks (~20s vs ~100s)
#   ./scripts/ci-local-release.sh --security-mode quick
#   ./scripts/ci-local-release.sh --release-version X.Y.Z --hil-target 'local:gpu:ao version && ao init --help && ao rpi status'
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
LOCAL_CI_MUTATION_LANE="local-ci-release"
LOCAL_CI_MUTATION_ESCAPE_HATCH="operator-run-release-validation"

SECURITY_MODE="full"
FAST_MODE=false
RELEASE_VERSION_OVERRIDE=""

USER_MAX_JOBS=""
RELEASE_READINESS_MODE="${AGENTOPS_RELEASE_READINESS_MODE:-}"
RELEASE_HIL_WAIVER="${AGENTOPS_RELEASE_HIL_WAIVER:-}"
RELEASE_HIL_TARGET_ARGS=()

usage() {
    cat <<'USAGE'
Usage: scripts/ci-local-release.sh [options]

Options:
  --fast               Skip heavy checks (race tests, security gate, SBOM, hook integration)
  --release-version V  Record artifacts against the target release version (for release audits)
  --readiness-mode M   official|advisory|fast (default: official only with --release-version)
  --hil-target SPEC    Add HIL target evidence; repeatable (local:<name>:<cmd> or ssh:<name>:<host>:<cmd>)
  --hil-waiver TEXT    Record an explicit HIL waiver for release readiness
  --security-mode      quick|full (default: full)
  --jobs N             Max parallel jobs (default: half CPU cores, min 4)
  -h, --help           Show this help

Environment:
  AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1
      Allow release smoke to update tracked AgentOps metadata.
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
        --readiness-mode)
            RELEASE_READINESS_MODE="${2:-}"
            shift 2
            ;;
        --hil-target)
            RELEASE_HIL_TARGET_ARGS+=("${2:-}")
            shift 2
            ;;
        --hil-waiver)
            RELEASE_HIL_WAIVER="${2:-}"
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

if [[ -n "$RELEASE_READINESS_MODE" && \
      "$RELEASE_READINESS_MODE" != "official" && \
      "$RELEASE_READINESS_MODE" != "advisory" && \
      "$RELEASE_READINESS_MODE" != "fast" ]]; then
    echo "Invalid --readiness-mode: $RELEASE_READINESS_MODE (expected official, advisory, or fast)" >&2
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
            --exclude-dir=.gc \
            --exclude-dir=.claude \
            --exclude-dir=.agents \
            --exclude-dir=.tmp \
            --exclude-dir=.venv \
            --exclude-dir=.venv-docs \
            --exclude-dir=venv \
            --exclude-dir=_site \
            --exclude-dir=site \
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
            . 2>/dev/null | grep -v 'Getenv\|os\.Environ\|DOLT_PASSWORD' | grep -q .; then
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
    version="$(release_version)"

    (
        cd cli
        make build VERSION="$version"
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
    local release_readiness=""
    local hil_evidence=""
    local vil_evidence=""
    local digital_twin_evidence=""
    local eval_fast_report=""
    local eval_baseline_audit=""
    local fast_mode_json=false

    version="$(release_version)"
    repo_version="$(jq -r '.version' .claude-plugin/plugin.json)"
    generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    manifest_file="$ARTIFACT_DIR/release-artifacts.json"
    local git_sha
    git_sha="$(git rev-parse HEAD 2>/dev/null || echo "unknown")"

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
    if [[ -f "$ARTIFACT_DIR/release-readiness.json" ]]; then
        release_readiness="release-readiness.json"
    fi
    if [[ -f "$ARTIFACT_DIR/hil-evidence.json" ]]; then
        hil_evidence="hil-evidence.json"
    fi
    if [[ -f "$ARTIFACT_DIR/digital-twin-evidence.json" ]]; then
        vil_evidence="digital-twin-evidence.json"
        digital_twin_evidence="digital-twin-evidence.json"
    fi
    if [[ -f "$ARTIFACT_DIR/eval-agentops-fast.json" ]]; then
        eval_fast_report="eval-agentops-fast.json"
    fi
    if [[ -f "$ARTIFACT_DIR/eval-baseline-audit.json" ]]; then
        eval_baseline_audit="eval-baseline-audit.json"
    fi

    jq -n \
        --arg run_id "$RUN_ID" \
        --arg generated_at "$generated_at" \
        --arg artifact_dir "$(artifact_dir_rel)" \
        --arg release_version "$version" \
        --arg repo_version "$repo_version" \
        --arg git_sha "$git_sha" \
        --arg security_mode "$SECURITY_MODE" \
        --arg sbom_cyclonedx "$sbom_cyclonedx" \
        --arg sbom_spdx "$sbom_spdx" \
        --arg security_report "$security_report" \
        --arg release_readiness "$release_readiness" \
        --arg hil_evidence "$hil_evidence" \
        --arg vil_evidence "$vil_evidence" \
        --arg digital_twin_evidence "$digital_twin_evidence" \
        --arg eval_fast_report "$eval_fast_report" \
        --arg eval_baseline_audit "$eval_baseline_audit" \
        --argjson fast_mode "$fast_mode_json" \
        '{
          schema_version: 1,
          run_id: $run_id,
          generated_at: $generated_at,
          artifact_dir: $artifact_dir,
          release_version: $release_version,
          repo_version: $repo_version,
          git_sha: $git_sha,
          fast_mode: $fast_mode,
          security_mode: $security_mode,
          sbom_cyclonedx: (if $sbom_cyclonedx == "" then null else $sbom_cyclonedx end),
          sbom_spdx: (if $sbom_spdx == "" then null else $sbom_spdx end),
          security_report: (if $security_report == "" then null else $security_report end),
          release_readiness: (if $release_readiness == "" then null else $release_readiness end),
          hil_evidence: (if $hil_evidence == "" then null else $hil_evidence end),
          vil_evidence: (if $vil_evidence == "" then null else $vil_evidence end),
          digital_twin_evidence: (if $digital_twin_evidence == "" then null else $digital_twin_evidence end),
          eval_fast_report: (if $eval_fast_report == "" then null else $eval_fast_report end),
          eval_baseline_audit: (if $eval_baseline_audit == "" then null else $eval_baseline_audit end)
        }' > "$manifest_file"

    echo "Release artifact manifest: $manifest_file"
}

write_tag_index() {
    local version
    version="$(release_version)"

    # Only write an index entry when a meaningful version is known.
    # Skip if version looks like a git describe dirty/hash ref (no semver dot).
    if [[ -z "$version" ]] || [[ "$version" != *.* ]]; then
        return 0
    fi

    local tag_index="$REPO_ROOT/.agents/releases/local-ci/tag-index.txt"
    local tag="v${version}"

    # Append (or create): "<tag> <run_id> <generated_at>"
    mkdir -p "$(dirname "$tag_index")"
    local generated_at
    generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '%s %s %s\n' "$tag" "$RUN_ID" "$generated_at" >> "$tag_index"
    echo "Tag index updated: $tag_index ($tag -> $RUN_ID)"
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

run_init_rpi_smoke() {
    local tmp_home
    local tmp_repo
    tmp_home="$(mktemp -d)"
    tmp_repo="$(mktemp -d)"
    local rc=0

    git -C "$tmp_repo" init -q
    (
        cd "$tmp_repo"
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" init
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi status
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi --help >/dev/null
        HOME="$tmp_home" "$REPO_ROOT/cli/bin/ao" rpi phased --help >/dev/null
    ) || rc=$?

    rm -rf "$tmp_home" "$tmp_repo"
    return "$rc"
}

release_readiness_mode() {
    if [[ -n "$RELEASE_READINESS_MODE" ]]; then
        printf '%s\n' "$RELEASE_READINESS_MODE"
    elif [[ "$FAST_MODE" == "true" ]]; then
        printf 'fast\n'
    elif [[ -n "$RELEASE_VERSION_OVERRIDE" ]]; then
        printf 'official\n'
    else
        printf 'advisory\n'
    fi
}

run_release_hil_evidence() {
    local mode
    local version
    local args=("--out" "$ARTIFACT_DIR/hil-evidence.json")

    mode="$(release_readiness_mode)"
    version="$(release_version)"
    if [[ -n "$version" ]]; then
        args+=("--expected-version" "$version")
    fi
    if [[ "$mode" == "official" ]]; then
        args+=("--required")
    fi
    if [[ -n "$RELEASE_HIL_WAIVER" ]]; then
        args+=("--waiver" "$RELEASE_HIL_WAIVER")
    fi
    for target in "${RELEASE_HIL_TARGET_ARGS[@]}"; do
        args+=("--target" "$target")
    done

    ./scripts/check-release-hil.sh "${args[@]}"
}

write_release_digital_twin_evidence() {
    local generated_at
    local status="pass"
    local reason="release smoke, hook install smoke, and ao init/rpi smoke completed before this artifact was written"

    generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    if [[ "$FAST_MODE" == "true" ]]; then
        status="skipped"
        reason="fast mode skips the full release digital-twin/VIL proof"
    elif [[ "$errors" -ne 0 ]]; then
        status="fail"
        reason="one or more preceding local release gates failed before digital-twin evidence was written"
    fi

    jq -n \
        --arg generated_at "$generated_at" \
        --arg artifact_dir "$(artifact_dir_rel)" \
        --arg status "$status" \
        --arg reason "$reason" \
        --argjson errors_before_artifact "$errors" \
        '{
          schema_version: 1,
          evidence_kind: "digital_twin",
          generated_at: $generated_at,
          artifact_dir: $artifact_dir,
          status: $status,
          reason: $reason,
          dimensions: {
            vil: {status: $status, evidence: "local release digital twin"},
            release_smoke: {status: $status},
            hook_install_smoke: {status: $status},
            rpi_smoke: {status: $status}
          },
          errors_before_artifact: $errors_before_artifact
        }' > "$ARTIFACT_DIR/digital-twin-evidence.json"

    [[ "$status" != "fail" ]]
}

run_release_eval_evidence() {
    if [[ "$FAST_MODE" == "true" ]]; then
        jq -n \
            --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            --arg artifact_dir "$(artifact_dir_rel)" \
            '{
              schema_version: 1,
              evidence_kind: "agentops_eval_fast",
              generated_at: $generated_at,
              artifact_dir: $artifact_dir,
              status: "skipped",
              reason: "fast mode skips release eval evidence"
            }' > "$ARTIFACT_DIR/eval-agentops-fast.json"
        jq -n \
            '{suite_count: 0, baseline_count: 0, policy_mismatch_count: 0, stale_suite_hashes: []}' \
            > "$ARTIFACT_DIR/eval-baseline-audit.json"
        return 0
    fi

    local eval_root="$ARTIFACT_DIR/eval-agentops"
    local run_root="$eval_root/runs"
    local log_file="$ARTIFACT_DIR/eval-agentops-fast.log"
    local run_dir=""
    local rc=0
    mkdir -p "$run_root"

    AO_BIN="$REPO_ROOT/cli/bin/ao" \
        ./scripts/eval-agentops.sh --fast --run-root "$run_root" --baseline-dir "$eval_root/baselines" > "$log_file" 2>&1 || rc=$?

    run_dir="$(find "$run_root" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1)"
    if [[ -n "$run_dir" && -f "$run_dir/baseline-audit.json" ]]; then
        cp "$run_dir/baseline-audit.json" "$ARTIFACT_DIR/eval-baseline-audit.json"
    else
        jq -n \
            '{suite_count: 0, baseline_count: 0, policy_mismatch_count: 0, stale_suite_hashes: []}' \
            > "$ARTIFACT_DIR/eval-baseline-audit.json"
        rc=1
    fi

    local status="pass"
    [[ "$rc" -eq 0 ]] || status="fail"

    jq -n \
        --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --arg artifact_dir "$(artifact_dir_rel)" \
        --arg status "$status" \
        --arg command "scripts/eval-agentops.sh --fast" \
        --arg run_dir "${run_dir#"$REPO_ROOT"/}" \
        --arg log "eval-agentops-fast.log" \
        --arg baseline_audit "eval-baseline-audit.json" \
        --argjson exit_code "$rc" \
        '{
          schema_version: 1,
          evidence_kind: "agentops_eval_fast",
          generated_at: $generated_at,
          artifact_dir: $artifact_dir,
          status: $status,
          command: $command,
          exit_code: $exit_code,
          run_dir: (if $run_dir == "" then null else $run_dir end),
          log: $log,
          baseline_audit: $baseline_audit
        }' > "$ARTIFACT_DIR/eval-agentops-fast.json"

    return "$rc"
}

check_release_readiness() {
    local mode
    local security_status="pass"
    local artifact_status="pass"
    local vil_status="pass"
    local eval_status="pass"
    local args

    mode="$(release_readiness_mode)"
    if [[ "$FAST_MODE" == "true" ]]; then
        security_status="skipped"
        artifact_status="skipped"
        vil_status="skipped"
        eval_status="skipped"
    elif [[ ! -f "$ARTIFACT_DIR/eval-agentops-fast.json" ]] || \
        ! jq -e '.status == "pass"' "$ARTIFACT_DIR/eval-agentops-fast.json" >/dev/null 2>&1; then
        eval_status="fail"
    fi
    if [[ "$FAST_MODE" != "true" ]] && \
        { [[ ! -f "$ARTIFACT_DIR/digital-twin-evidence.json" ]] || \
          ! jq -e '.status == "pass" and .dimensions.vil.status == "pass"' "$ARTIFACT_DIR/digital-twin-evidence.json" >/dev/null 2>&1; }; then
        vil_status="fail"
    fi

    args=(
        "--artifact-dir" "$ARTIFACT_DIR"
        "--out" "$ARTIFACT_DIR/release-readiness.json"
        "--mode" "$mode"
        "--threshold" "8"
        "--sil" "pass"
        "--vil" "$vil_status"
        "--artifacts" "$artifact_status"
        "--security" "$security_status"
        "--eval" "$eval_status"
    )

    if [[ -f "$ARTIFACT_DIR/hil-evidence.json" ]]; then
        args+=("--hil-file" "$ARTIFACT_DIR/hil-evidence.json")
    elif [[ -n "$RELEASE_HIL_WAIVER" ]]; then
        args+=("--hil-status" "waived" "--hil-waiver" "$RELEASE_HIL_WAIVER")
    else
        args+=("--hil-status" "skipped")
    fi

    ./scripts/check-release-readiness.sh "${args[@]}"
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
echo "Validation lane: $LOCAL_CI_MUTATION_LANE (writes $(artifact_dir_rel))"
echo "Mutation escape hatch: $LOCAL_CI_MUTATION_ESCAPE_HATCH"
echo "Release metadata guard: tracked .agents findings/citations stay stable unless AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1"
echo "Max parallel jobs: $MAX_JOBS"

# ── Phase 1: Quick sequential checks (must pass before heavy work) ──

run_step "Required tool check" check_required_cmds

# Capture ~/.agents content-hash snapshot before anything that could mutate it.
# Diffed at the end of the gate (see Phase 6 below). Complements the pre-emptive
# grep-based scripts/check-home-isolation.sh by catching runtime mutations,
# including the os.Chtimes mtime-bypass attack.
HASH_GATE_SNAPSHOT=""
if [[ -x "$REPO_ROOT/scripts/check-agents-hash-snapshot.sh" ]]; then
    HASH_GATE_SNAPSHOT="$("$REPO_ROOT/scripts/check-agents-hash-snapshot.sh" capture 2>/dev/null || echo "")"
fi

check_agents_hash_gate() {
    if [[ -z "$HASH_GATE_SNAPSHOT" ]]; then
        echo "snapshot not captured (check-agents-hash-snapshot.sh missing or failed)"
        return 0
    fi
    if [[ ! -x "$REPO_ROOT/scripts/check-agents-hash-snapshot.sh" ]]; then
        echo "check-agents-hash-snapshot.sh no longer executable"
        return 1
    fi
    if "$REPO_ROOT/scripts/check-agents-hash-snapshot.sh" diff "$HASH_GATE_SNAPSHOT"; then
        rm -f "$HASH_GATE_SNAPSHOT"
        return 0
    fi
    rm -f "$HASH_GATE_SNAPSHOT"
    return 1
}

# ── Phase 2: Parallel independent checks ──
# These have zero dependencies on each other.

run_step_bg "Doc-release gate" ./tests/docs/validate-doc-release.sh
run_step_bg "Manifest schema validation" ./scripts/validate-manifests.sh --repo-root "$REPO_ROOT"
run_step_bg "Manifest version consistency" check_manifest_version_consistency
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
run_step_bg "Competitive freshness tests" bash ./tests/scripts/test-competitive-freshness.sh
run_step_bg "Go fast scope tests" bats ./tests/scripts/validate-go-fast.bats
run_step_bg "Skill runtime parity tests" bash ./tests/scripts/test-skill-runtime-parity.sh
run_step_bg "Skill CLI snippet tests" bash ./tests/scripts/test-skill-cli-snippets.sh
run_step_bg "Codex plugin install tests" bash ./tests/scripts/test-codex-plugin-install.sh
run_step_bg "Codex native install tests" bash ./tests/scripts/test-codex-native-skills-install.sh
run_step_bg "Codex artifact manifest tests" bash ./tests/scripts/test-codex-generated-manifest.sh
run_step_bg "Codex artifact metadata tests" bash ./tests/scripts/test-codex-generated-artifacts.sh
run_step_bg "Codex backbone prompt tests" bash ./tests/scripts/test-codex-backbone-prompts.sh
run_step_bg "Validate-local tests" bash ./tests/scripts/test-validate-local.sh
run_step_bg "Headless runtime skill smoke tests" bash ./tests/scripts/test-headless-runtime-skills.sh

collect_parallel

# ── Phase 3b: Remote-parity checks ──
# These run in CI (validate.yml) but were missing from local gate.

run_step_bg "Skill schema validation" ./scripts/validate-skill-schema.sh --verbose
run_step_bg "Learning coherence" ./scripts/validate-learning-coherence.sh
run_step_bg "JSON flag consistency" ./tests/cli/test-json-flag-consistency.sh
run_step_bg "JSON flag temp workspace" ./tests/cli/test-json-flag-consistency-tempdir.sh

collect_parallel

# ── Phase 4: Heavy checks (skipped in --fast mode) ──

if [[ "$FAST_MODE" == "true" ]]; then
    warn "Skipped Go race tests (--fast)"
    warn "Skipped SBOM generation (--fast)"
    warn "Skipped Security gate (--fast)"
    warn "Skipped AgentOps contract canaries (--fast)"

    # Still build the binary (fast) and run smoke tests against it
    run_step "Go build + vet" run_go_build_only
    run_step "Release binary validation" run_release_binary_validation
else
    # These are the heavy hitters — run them in parallel
    run_step_bg "Go build + race tests" run_go_build_and_tests
    run_step_bg "Generate SBOM artifacts (CycloneDX + SPDX)" generate_sbom_artifacts
    run_step_bg "Security toolchain gate (${SECURITY_MODE}, require tools)" run_security_gate
    run_step_bg "AgentOps contract canaries" ./scripts/test-agentops-contract-canaries.sh

    collect_parallel

    run_step "Release binary validation" run_release_binary_validation
fi

# ── Phase 5: CLI smoke tests (need built binary) ──

run_step_bg "ao init + ao rpi smoke" run_init_rpi_smoke
run_step_bg "Release smoke test (all commands)" ./scripts/release-smoke-test.sh --skip-build

collect_parallel

run_step "Digital twin/VIL evidence" write_release_digital_twin_evidence
run_step "AgentOps eval evidence" run_release_eval_evidence

# ── Phase 6: Post-hoc ~/.agents content-hash gate ──
# Fails if any protected subtree under $HOME/.agents was mutated since
# the snapshot was captured in Phase 1.
run_step "Agents-hub content-hash gate" check_agents_hash_gate

# ── Phase 7: Release readiness evidence ──
# Official release audits (--release-version) require HIL evidence or an
# explicit waiver. Normal local runs and --fast runs still write advisory JSON.
run_step "HIL release evidence" run_release_hil_evidence
run_step "Release readiness score gate" check_release_readiness

# ═══════════════════════════════════════════════════════
#  Summary
# ═══════════════════════════════════════════════════════

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

write_release_artifact_manifest
write_tag_index

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

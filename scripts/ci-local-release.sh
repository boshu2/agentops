#!/usr/bin/env bash
set -euo pipefail

# ci-local-release.sh
# Release-grade local CI gate. Mirrors validate/release pipeline checks locally
# and adds CLI smoke coverage for hooks install and RPI paths.
#
# Usage:
#   ./scripts/ci-local-release.sh              # full gate (parallel where possible)
#   ./scripts/ci-local-release.sh --fast       # skip heavy checks (~20s vs ~100s)
#   ./scripts/ci-local-release.sh --ci-blocking # CI Validate blocking jobs only
#   ./scripts/ci-local-release.sh --security-mode quick
#   ./scripts/ci-local-release.sh --release-version X.Y.Z --hil-target 'local:gpu:ao version'
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
CI_BLOCKING_MODE=false
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
  --ci-blocking        Run the CI Validate blocking job set locally; excludes advisory jobs
                       and release-only artifact evidence. This mode rejects --fast.
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
        --ci-blocking)
            CI_BLOCKING_MODE=true
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

if [[ "$CI_BLOCKING_MODE" == "true" && "$FAST_MODE" == "true" ]]; then
    echo "Invalid option combination: --ci-blocking cannot be combined with --fast" >&2
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
          hil_evidence: (if $hil_evidence == "" then null else $hil_evidence end)
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

build_ci_ao_binary() {
    (
        cd cli
        mkdir -p bin
        go build -o bin/ao ./cmd/ao
    )
}

run_eval_baseline_audit_ci() {
    build_ci_ao_binary

    local out
    out="$(./cli/bin/ao eval baseline-audit --root evals/agentops-core --json)"
    printf '%s\n' "$out"

    local stale mismatch
    stale="$(printf '%s' "$out" | jq '(.stale_suite_hashes // []) | length' 2>/dev/null || echo "-1")"
    mismatch="$(printf '%s' "$out" | jq '.policy_mismatch_count // 0' 2>/dev/null || echo "-1")"
    if [[ "$stale" == "-1" || "$mismatch" == "-1" ]]; then
        echo "FAIL: could not parse baseline-audit output" >&2
        return 1
    fi
    if [[ "$stale" -gt 0 ]]; then
        echo "FAIL: stale_suite_hashes=$stale (a promoted baseline's suite SHA drifted)" >&2
        return 1
    fi
    echo "ok: stale_suite_hashes=0 (info: policy_mismatch_count=$mismatch)"
}

run_goals_validate_ci() {
    build_ci_ao_binary
    ./cli/bin/ao goals validate --json | jq -e '.valid == true'
    bash tests/e2e/goals-scenarios-link.sh
}

run_eval_skill_delta_ci() {
    if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
        if ! git diff --name-only HEAD~1 -- 'skills/**' | grep -q .; then
            echo "No skills/ changes detected — skipping eval-skill-delta"
            return 0
        fi
    fi

    local result
    result="$(bash scripts/eval-agent-harness.sh --task go-01 --agent echo --dry-run 2>/dev/null | tail -1)"
    printf '%s\n' "$result" | jq -e '.skipped == true' >/dev/null
    echo "eval harness dry-run: valid JSON, skipped=true"
}

run_skill_dependency_check_ci() {
    python3 - <<'PY'
import re
import sys
from pathlib import Path

skills_dir = Path("skills")
skills = {
    p.name for p in skills_dir.iterdir()
    if p.is_dir() and (p / "SKILL.md").exists()
}

missing = []
for skill in sorted(skills):
    content = (skills_dir / skill / "SKILL.md").read_text(encoding="utf-8")
    match = re.match(r"^---\n(.*?)\n---\n", content, re.S)
    if not match:
        continue

    in_dependencies = False
    for line in match.group(1).splitlines():
        stripped = line.strip()
        if stripped.startswith("dependencies:"):
            in_dependencies = True
            continue
        if in_dependencies and stripped.startswith("- "):
            dep = stripped[2:].split("#", 1)[0].strip().strip('"').strip("'")
            if dep and dep not in skills:
                missing.append((skill, dep))
            continue
        if in_dependencies and stripped and not stripped.startswith("-"):
            in_dependencies = False

if missing:
    print("Unresolved skill dependencies:")
    for skill, dep in missing:
        print(f"  - {skill} -> {dep}")
    sys.exit(1)

print(f"Skill dependencies resolved: {len(skills)} skills checked.")
PY
}

run_plugin_load_test_ci() {
    ./scripts/validate-manifests.sh --repo-root "$REPO_ROOT"

    local symlinks
    symlinks="$(find . -type l -not -path "./.git/*" 2>/dev/null || true)"
    if [[ -n "$symlinks" ]]; then
        echo "Found symlinks that will break standalone installation:"
        printf '%s\n' "$symlinks"
        return 1
    fi

    bash scripts/check-no-tracked-agents.sh

    local failed=0
    if [[ -d "skills" ]]; then
        local skill skill_name skill_count=0
        for skill in skills/*/; do
            [[ -d "$skill" ]] || continue
            skill_name="$(basename "$skill")"
            if [[ ! -f "${skill}SKILL.md" ]]; then
                echo "$skill_name: missing SKILL.md"
                failed=1
                continue
            fi
            if ! head -1 "${skill}SKILL.md" | grep -q "^---$"; then
                echo "$skill_name: SKILL.md missing YAML frontmatter"
                failed=1
                continue
            fi
            if ! grep -q "^name:" "${skill}SKILL.md"; then
                echo "$skill_name: SKILL.md missing name in frontmatter"
                failed=1
                continue
            fi
            skill_count=$((skill_count + 1))
        done
        echo "$skill_count skills valid"
    else
        echo "No skills/ directory found"
        failed=1
    fi

    if [[ -f "hooks/hooks.json" ]]; then
        jq empty "hooks/hooks.json"
    fi

    [[ "$failed" -eq 0 ]]
}

run_security_scan_ci() {
    run_security_scan_patterns
    run_dangerous_pattern_scan
}

run_contract_compatibility_ci() {
    ./scripts/check-contract-compatibility.sh
    ./scripts/validate-next-work-contract-parity.sh
}

run_three_gap_supergate_ci() {
    ./scripts/check-three-gap-supergate.sh --gap=all
    ./scripts/check-three-gap-supergate.sh --gap=council-coverage --strict-coverage \
        || echo "ADVISORY-WARN: --strict-coverage non-blocking failure (soc-33bw)"
}

run_retrieval_quality_ci() {
    (
        cd cli
        go build -o /tmp/ao-test ./cmd/ao
        /tmp/ao-test retrieval-bench --json | tee /tmp/retrieval-report.json
        local precision
        precision="$(python3 -c "import json; r=json.load(open('/tmp/retrieval-report.json')); print(r.get('avg_precision_at_k', r.get('avg_p_at_k', 0)))")"
        echo "Precision@K: $precision"
        if python3 -c "exit(0 if float('$precision') >= 0.1 else 1)" 2>/dev/null; then
            echo "Retrieval quality above minimum threshold"
        else
            echo "WARN: Retrieval precision below 0.1 — flywheel may be degraded"
        fi
    )
    AGENTOPS_RETRIEVAL_SMOKE_AO=/tmp/ao-test bash scripts/retrieval-quality-smoke.sh
}

run_go_build_ci() {
    (
        cd cli
        go build -o /tmp/ao-test ./cmd/ao
        go test -race -shuffle=on -coverprofile=coverage.out -covermode=atomic ./... -v 2>&1 | tee /tmp/go-test-output.txt
        go tool cover -func=coverage.out | tail -1
    )
    bash scripts/check-cmd-ao-coverage.sh --profile cli/coverage.out
    (
        cd cli
        make sync-hooks
        git diff --exit-code -- embedded/
    )

    local tools_dir="$SECURITY_TMP_BASE/go-tools"
    mkdir -p "$tools_dir"
    if ! command -v gocyclo >/dev/null 2>&1; then
        GOBIN="$tools_dir" go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
    fi
    PATH="$tools_dir:$PATH" ./scripts/check-go-complexity.sh --base HEAD~1 --warn 15 --fail 25
}

run_windows_smoke_ci() {
    case "${OS:-}:$(uname -s 2>/dev/null || printf unknown)" in
        Windows_NT:*|*:MINGW*|*:MSYS*|*:CYGWIN*)
            if command -v pwsh >/dev/null 2>&1; then
                pwsh -NoProfile -ExecutionPolicy Bypass -File tests/windows/test-windows-smoke.ps1
            elif command -v powershell >/dev/null 2>&1; then
                powershell -NoProfile -ExecutionPolicy Bypass -File tests/windows/test-windows-smoke.ps1
            else
                echo "windows-smoke requires pwsh or powershell on PATH" >&2
                return 1
            fi
            ;;
        *)
            echo "windows-smoke is a blocking CI job and requires a Windows host for exact local parity." >&2
            echo "Run scripts/ci-local-release.sh --ci-blocking from Windows, or verify the exact pushed SHA with scripts/verify-release-ci.sh." >&2
            return 1
            ;;
    esac
}

run_cli_integration_ci() {
    (
        cd cli
        make build
    )
    bash tests/integration/test-cli-commands.sh
    bash tests/integration/test-v218-commands.sh
    bash tests/hooks/test-hook-lifecycle.sh
    bash scripts/release-smoke-test.sh --skip-build
    cp cli/embedded/hooks/ao-*.sh hooks/
    bash tests/hooks/test-hooks.sh
}

run_file_manifest_overlap_ci() {
    bash -n scripts/check-file-manifest-overlap.sh

    local overlap_tmp clean_tmp
    overlap_tmp="$(mktemp)"
    clean_tmp="$(mktemp)"

    printf '%s\n' '[{"id":"1","subject":"A","files":["a.go"]},{"id":"2","subject":"B","files":["a.go"]}]' > "$overlap_tmp"
    ! scripts/check-file-manifest-overlap.sh "$overlap_tmp"

    printf '%s\n' '[{"id":"1","subject":"A","files":["a.go"]},{"id":"2","subject":"B","files":["b.go"]}]' > "$clean_tmp"
    scripts/check-file-manifest-overlap.sh "$clean_tmp"
    rm -f "$overlap_tmp" "$clean_tmp"
}

run_bats_tests_ci() {
    bats --print-output-on-failure tests/hooks/*.bats tests/scripts/*.bats
    bash tests/hooks/test-orphan-hooks.sh
}

run_ci_blocking_gate() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  AgentOps Local CI — CI Validate Blocking Jobs${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    echo "Artifacts: $ARTIFACT_DIR"
    echo "Max parallel jobs: $MAX_JOBS"
    echo "Advisory jobs excluded: agentops-eval-advisory, security-toolchain-gate, doctor-check, factory-claim-ledger-strict, practice-citations, check-test-staleness, swarm-evidence"

    run_step "Required tool check" check_required_cmds
    # ci-job:windows-smoke
    run_step "windows-smoke" run_windows_smoke_ci
    if [[ "$errors" -gt 0 ]]; then
        echo ""
        echo -e "${RED}  CI-BLOCKING LOCAL VALIDATION FAILED ($errors failing check(s))${NC}"
        return 1
    fi

    # ci-job:doc-release-gate
    run_step_bg "doc-release-gate" ./tests/docs/validate-doc-release.sh
    # ci-job:smoke-test
    run_step_bg "smoke-test" ./tests/smoke-test.sh --verbose
    # ci-job:hook-preflight
    run_step_bg "hook-preflight" ./scripts/validate-hook-preflight.sh
    # ci-job:pre-push-gate-wired
    run_step_bg "pre-push-gate-wired" ./scripts/check-pre-push-gate-wired.sh --dry-run-smoke
    # ci-job:standards-injector-completeness
    run_step_bg "standards-injector-completeness" ./scripts/check-standards-injector-completeness.sh
    # ci-job:validate-hooks-doc-parity
    run_step_bg "validate-hooks-doc-parity" ./scripts/validate-hooks-doc-parity.sh
    # ci-job:hook-output-schema-lint
    run_step_bg "hook-output-schema-lint" ./scripts/test-hooks-output.sh
    # ci-job:validate-ci-policy-parity
    run_step_bg "validate-ci-policy-parity" ./scripts/validate-ci-policy-parity.sh
    # ci-job:validate-codex-runtime-sections
    run_step_bg "validate-codex-runtime-sections" ./scripts/validate-codex-runtime-sections.sh
    # ci-job:validate-codex-generated-artifacts
    run_step_bg "validate-codex-generated-artifacts" ./scripts/validate-codex-generated-artifacts.sh --scope head
    # ci-job:validate-codex-backbone-prompts
    run_step_bg "validate-codex-backbone-prompts" ./scripts/validate-codex-backbone-prompts.sh
    # ci-job:validate-codex-override-coverage
    run_step_bg "validate-codex-override-coverage" ./scripts/validate-codex-override-coverage.sh
    # ci-job:validate-codex-rpi-contract
    run_step_bg "validate-codex-rpi-contract" bash scripts/validate-codex-rpi-contract.sh
    # ci-job:validate-codex-lifecycle-guards
    run_step_bg "validate-codex-lifecycle-guards" ./scripts/validate-codex-lifecycle-guards.sh
    # ci-job:validate-codex-parity-drift
    run_step_bg "validate-codex-parity-drift" ./scripts/check-codex-parity-drift.sh
    # ci-job:validate-quarantine-empty
    run_step_bg "validate-quarantine-empty" ./scripts/check-quarantine-empty.sh
    # ci-job:validate-wiring-closure
    run_step_bg "validate-wiring-closure" timeout 60 bash scripts/check-wiring-closure.sh
    # ci-job:validate-corpus-freshness
    run_step_bg "validate-corpus-freshness" env AGENTOPS_CORPUS_FRESHNESS_SKIP=1 ./scripts/check-corpus-freshness.sh
    # ci-job:validate-flywheel-compounding-snapshot
    run_step_bg "validate-flywheel-compounding-snapshot" ./scripts/check-flywheel-compounding-snapshot.sh
    # ci-job:validate-factory-yield-ledger
    run_step_bg "validate-factory-yield-ledger" ./scripts/check-factory-yield-ledger.sh
    # ci-job:validate-finding-registry
    run_step_bg "validate-finding-registry" ./scripts/check-finding-registry.sh
    # ci-job:validate-factory-admission
    run_step_bg "validate-factory-admission" ./scripts/check-factory-admission.sh
    # ci-job:validate-contracts-structural-floor
    run_step_bg "validate-contracts-structural-floor" ./scripts/check-contracts-structural-floor.sh
    # ci-job:validate-docs-learning-references
    run_step_bg "validate-docs-learning-references" ./scripts/check-docs-learning-references.sh
    # ci-job:validate-headless-runtime-skills
    run_step_bg "validate-headless-runtime-skills" ./scripts/validate-headless-runtime-skills.sh
    # ci-job:validate-skill-frontmatter
    run_step_bg "validate-skill-frontmatter" bash scripts/validate-skill-frontmatter.sh
    # ci-job:validate-context-map-drift
    run_step_bg "validate-context-map-drift" bash scripts/validate-context-map-drift.sh
    # ci-job:embedded-sync
    run_step_bg "embedded-sync" ./scripts/validate-embedded-sync.sh
    # ci-job:cli-docs-parity
    run_step_bg "cli-docs-parity" ./scripts/generate-cli-reference.sh --check
    # ci-job:registry-check
    run_step_bg "registry-check" ./scripts/generate-registry.sh --check
    # ci-job:shellcheck
    run_step_bg "shellcheck" run_shellcheck
    # ci-job:markdownlint
    run_step_bg "markdownlint" run_markdownlint
    # ci-job:security-scan
    run_step_bg "security-scan" run_security_scan_ci
    # ci-job:skill-integrity
    run_step_bg "skill-integrity" bash skills/heal-skill/scripts/heal.sh --strict
    # ci-job:skill-schema
    run_step_bg "skill-schema" ./scripts/validate-skill-schema.sh --verbose
    # ci-job:skill-dependency-check
    run_step_bg "skill-dependency-check" run_skill_dependency_check_ci
    # ci-job:contract-compatibility-gate
    run_step_bg "contract-compatibility-gate" run_contract_compatibility_ci
    # ci-job:memrl-health
    run_step_bg "memrl-health" ./scripts/check-memrl-health.sh
    # ci-job:plugin-load-test
    run_step_bg "plugin-load-test" run_plugin_load_test_ci
    # ci-job:skill-lint
    run_step_bg "skill-lint" bash tests/skills/run-all.sh
    # ci-job:learning-coherence
    run_step_bg "learning-coherence" bash scripts/validate-learning-coherence.sh .agents/learnings
    # ci-job:file-manifest-overlap
    run_step_bg "file-manifest-overlap" run_file_manifest_overlap_ci

    collect_parallel

    # ci-job:agentops-eval-baseline-audit
    run_step_bg "agentops-eval-baseline-audit" run_eval_baseline_audit_ci
    # ci-job:validate-flywheel-proof
    run_step_bg "validate-flywheel-proof" ./scripts/proof-run.sh
    # ci-job:validate-goals-validate
    run_step_bg "validate-goals-validate" run_goals_validate_ci
    # ci-job:validate-three-gap-supergate
    run_step_bg "validate-three-gap-supergate" run_three_gap_supergate_ci
    # ci-job:agentops-contract-canaries
    run_step_bg "agentops-contract-canaries" ./scripts/test-agentops-contract-canaries.sh
    # ci-job:eval-workbench-verify
    run_step_bg "eval-workbench-verify" bash scripts/check-eval-workbench.sh
    # ci-job:eval-skill-delta
    run_step_bg "eval-skill-delta" run_eval_skill_delta_ci
    # ci-job:retrieval-quality
    run_step_bg "retrieval-quality" run_retrieval_quality_ci
    # ci-job:go-build
    run_step_bg "go-build" run_go_build_ci
    # ci-job:cli-integration
    run_step_bg "cli-integration" run_cli_integration_ci
    # ci-job:json-flag-consistency
    run_step_bg "json-flag-consistency" bash -c 'cd cli && make build && cd .. && ./tests/cli/test-json-flag-consistency.sh'
    # ci-job:bats-tests
    run_step_bg "bats-tests" run_bats_tests_ci

    collect_parallel

    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
    if [[ "$errors" -gt 0 ]]; then
        echo -e "${RED}  CI-BLOCKING LOCAL VALIDATION FAILED ($errors failing check(s))${NC}"
        echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
        return 1
    fi
    echo -e "${GREEN}  CI-BLOCKING LOCAL VALIDATION PASSED${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
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
    local args=("--out" "$ARTIFACT_DIR/hil-evidence.json")

    mode="$(release_readiness_mode)"
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

check_release_readiness() {
    local mode
    local security_status="pass"
    local artifact_status="pass"
    local vil_status="pass"
    local args

    mode="$(release_readiness_mode)"
    if [[ "$FAST_MODE" == "true" ]]; then
        security_status="skipped"
        artifact_status="skipped"
        vil_status="skipped"
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
        "--eval" "pass"
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

if [[ "$CI_BLOCKING_MODE" == "true" ]]; then
    if run_ci_blocking_gate; then
        exit 0
    fi
    exit 1
fi

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
run_step_bg "Dev hook install tests" bash ./tests/scripts/test-install-dev-hooks.sh
run_step_bg "Git hook shim tests" bash ./tests/scripts/test-githook-shims.sh
run_step_bg "Validate-local tests" bash ./tests/scripts/test-validate-local.sh
run_step_bg "Headless runtime skill smoke tests" bash ./tests/scripts/test-headless-runtime-skills.sh
run_step_bg "Constraint compiler BATS wrapper" ./tests/hooks/test-constraint-compiler.sh

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
    warn "Skipped Hook integration tests (--fast)"
    warn "Skipped SBOM generation (--fast)"
    warn "Skipped Security gate (--fast)"
    warn "Skipped AgentOps contract canaries (--fast)"

    # Still build the binary (fast) and run smoke tests against it
    run_step "Go build + vet" run_go_build_only
    run_step "Release binary validation" run_release_binary_validation
else
    # These are the heavy hitters — run them in parallel
    run_step_bg "Go build + race tests" run_go_build_and_tests
    run_step_bg "Hook integration tests" ./tests/hooks/test-hooks.sh
    run_step_bg "Generate SBOM artifacts (CycloneDX + SPDX)" generate_sbom_artifacts
    run_step_bg "Security toolchain gate (${SECURITY_MODE}, require tools)" run_security_gate
    run_step_bg "AgentOps contract canaries" ./scripts/test-agentops-contract-canaries.sh

    collect_parallel

    run_step "Release binary validation" run_release_binary_validation
fi

# ── Phase 5: CLI smoke tests (need built binary) ──

run_step_bg "Hook install smoke (minimal + full)" run_hooks_install_smoke
run_step_bg "ao init --hooks + ao rpi smoke" run_init_hooks_rpi_smoke
run_step_bg "Release smoke test (all commands)" ./scripts/release-smoke-test.sh --skip-build

collect_parallel

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

#!/usr/bin/env bash
# pre-push-gate.sh — lightweight validation before push
#
# Runs the minimum checks to prevent broken code from landing on main.
# Designed to be fast (~10-20s cached) while catching the failures that
# ci-local-release.sh would catch later.
#
# Checks:
#   1. Go build + vet (if cli/ changed)
#   2. Go race tests on changed packages (via validate-go-fast.sh)
#  3d. .agents/ write-surface contract (catalogued top-level subdirs)
#   3. Command/test pairing for cli/cmd/ao Go changes
#   4. cmd/ao coverage floor gate
#  4b. Per-package coverage ratchet (full mode only)
#   5. Embedded hooks sync (cli/embedded/ matches hooks/)
#   6. Skill count sync
#   7. Worktree disposition
#   8. Skill runtime/CLI parity
#   9. Codex skill parity (skipped — manually maintained)
#  10. Codex install bundle parity (skipped — manually maintained)
#  11. Codex runtime section format
#  12. Skill integrity (references/xrefs)
#  13. Skill lint suite
#  14. Skill schema validation
#  15. Manifest schema validation
#  16. Codex artifact metadata
#  17. Codex backbone prompts
#  18. Codex override coverage
#  19. Next-work contract parity
#  19b. bd closeout contract parity
#  19c. Retrieval quality ratchet (warn-only until 500 indexed turns)
#  20. Skill runtime formats
#  21. Codex RPI contract validation
#  22. Codex lifecycle guard validation
#  23. Skill CLI snippets
#  24. Headless runtime skill smoke (full mode only)
#  24b. CLI docs parity
#  24c. AgentOps eval canaries (fast deterministic suites)
#  --- shifted from CI-only (v2.32) ---
#  25. Doc-release stabilization gate
#  25b. Release audit artifact refs
#  26. Contract compatibility
#  27. Hook preflight
#  28. Hooks/docs parity
#  29. CI policy parity
#  30. ShellCheck (fast: changed .sh only)
#  31. Plugin load test (symlink rejection)
#  32. Learning coherence
#  33. BATS orphan hooks audit
#  34. Skill citation parity (ao lookup → ao metrics cite)
#  35. Flywheel health (warn only, non-blocking)
#
# Usage:
#   scripts/pre-push-gate.sh [--scope auto|upstream|staged|worktree|head]
#   scripts/pre-push-gate.sh --fast [--scope ...]   # only checks relevant to changed files
#   (also called from .githooks/pre-push)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

if [[ -n "${GIT_DIR:-}" && -z "${GIT_WORK_TREE:-}" ]]; then
    export GIT_WORK_TREE="$REPO_ROOT"
fi

run_without_git_env() {
    local var_name
    local -a env_args=(env)
    while IFS='=' read -r var_name _; do
        [[ "$var_name" == GIT_* ]] || continue
        env_args+=("-u" "$var_name")
    done < <(env)
    "${env_args[@]}" "$@"
}

run_without_git_env_and_stdin() {
    run_without_git_env "$@" </dev/null
}

run_without_git_env_isolated_agents_home() {
    local tmp_home tmp_codex_home rc
    tmp_home="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prepush-home.XXXXXX")"
    tmp_codex_home="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prepush-codex.XXXXXX")"

    set +e
    HOME="$tmp_home" \
        CODEX_HOME="$tmp_codex_home" \
        AGENTS_HOME="$tmp_home/.agents" \
        run_without_git_env "$@"
    rc=$?
    set -e

    # Go module cache entries are intentionally read-only; make the isolated
    # tree removable so cleanup noise cannot mask the gate result.
    chmod -R u+w "$tmp_home" "$tmp_codex_home" 2>/dev/null || true
    rm -rf "$tmp_home" "$tmp_codex_home" 2>/dev/null || true
    return "$rc"
}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

errors=0
skipped=0
SCOPE="${PRE_PUSH_GO_SCOPE:-upstream}"
FAST_MODE=false
FAIL_FAST_SETTING="${PRE_PUSH_FAIL_FAST:-auto}"
FAIL_FAST_EFFECTIVE=false
FAIL_FAST_PENDING=false
HASH_GATE_SNAPSHOT=""
pass() { echo -e "${GREEN}  ok${NC}  $1"; }
skip() { echo -e "  --  $1 (skipped)"; skipped=$((skipped + 1)); }
warn() { echo -e "${YELLOW}WARN${NC}  $1"; }
indent_output() {
    while IFS= read -r line; do
        printf '    %s\n' "$line"
    done <<<"$1"
}
is_ci_env() {
    [[ -n "${CI:-}" || -n "${GITHUB_ACTIONS:-}" ]]
}
truthy() {
    case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|y|on|always) return 0 ;;
        *) return 1 ;;
    esac
}
cleanup_hash_snapshot() {
    if [[ -n "${HASH_GATE_SNAPSHOT:-}" ]]; then
        rm -f "$HASH_GATE_SNAPSHOT" 2>/dev/null || true
        HASH_GATE_SNAPSHOT=""
    fi
}
blocked_exit() {
    echo ""
    if [[ "$FAST_MODE" == "true" ]]; then
        echo -e "${RED}pre-push gate (fast): BLOCKED ($errors failures, $skipped skipped)${NC}"
    else
        echo -e "${RED}pre-push gate: BLOCKED ($errors failures)${NC}"
    fi
    cleanup_hash_snapshot
    exit 1
}
fail() {
    echo -e "${RED}FAIL${NC}  $1"
    errors=$((errors + 1))
    if [[ "${FAIL_FAST_EFFECTIVE:-false}" == "true" ]]; then
        FAIL_FAST_PENDING=true
    fi
}
maybe_fail_fast() {
    if [[ "${FAIL_FAST_PENDING:-false}" == "true" ]]; then
        warn "fail-fast enabled; stopping after first blocking failure"
        blocked_exit
    fi
}
run_hash_snapshot() {
    local timeout_seconds="${HASH_GATE_TIMEOUT_SECONDS:-15}"
    if command -v timeout >/dev/null 2>&1; then
        timeout "${timeout_seconds}s" scripts/check-agents-hash-snapshot.sh "$@"
        return $?
    fi
    scripts/check-agents-hash-snapshot.sh "$@"
}

usage() {
    cat <<'EOF'
Usage: scripts/pre-push-gate.sh [--fast] [--scope auto|upstream|staged|worktree|head]

Options:
  --fast       Only run checks relevant to changed files
  --scope      How to determine changed files (default: upstream)
  --fail-fast  Stop after first blocking failure
  --accumulate Continue after failures and report all blocking failures

Environment:
  PRE_PUSH_FAIL_FAST=0|1|auto   default auto: enabled for local --fast, off in CI
  PRE_PUSH_RUN_EVAL=1           run eval canaries even when eval files did not change
  PRE_PUSH_STRICT_EVAL=1        make local fast eval canaries blocking
  PRE_PUSH_AGENT_HEALTH=1       run local fast AgentOps health/ratchet checks
  PRE_PUSH_AGENT_HASH=1         force local fast agents-hub content hash gate
  PRE_PUSH_STRICT_WORKTREE=1    run worktree disposition in local fast mode
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --fast)
            FAST_MODE=true
            shift
            ;;
        --scope)
            SCOPE="${2:-}"
            shift 2
            ;;
        --fail-fast)
            FAIL_FAST_SETTING=1
            shift
            ;;
        --accumulate)
            FAIL_FAST_SETTING=0
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown arg: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

case "$SCOPE" in
    auto|upstream|staged|worktree|head) ;;
    *)
        echo "Invalid --scope: $SCOPE" >&2
        usage >&2
        exit 2
        ;;
esac

if [[ "$FAIL_FAST_SETTING" == "auto" ]]; then
    if [[ "$FAST_MODE" == "true" ]] && ! is_ci_env; then
        FAIL_FAST_EFFECTIVE=true
    fi
elif truthy "$FAIL_FAST_SETTING"; then
    FAIL_FAST_EFFECTIVE=true
fi

collect_all_changed() {
    case "$SCOPE" in
        upstream)
            git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
            ;;
        staged)
            git diff --name-only --cached 2>/dev/null || true
            ;;
        worktree)
            {
                git diff --name-only --cached 2>/dev/null || true
                git diff --name-only 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
        head)
            git show --name-only --pretty=format: HEAD 2>/dev/null || true
            ;;
        auto)
            {
                git diff --name-only '@{upstream}...HEAD' 2>/dev/null || true
                git diff --name-only --cached 2>/dev/null || true
                git diff --name-only 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
    esac
}

# --- Fast mode: detect changed file categories ---
HAS_GO=1
HAS_SKILL=1
HAS_HOOK=1
HAS_DOCS=1
HAS_SHELL=1
HAS_LEARNING=1
HAS_EVAL=1
HAS_CONTRACT=1
HAS_CI_POLICY=1
HAS_SWARM=1
HAS_CHANGELOG=1

if [[ "$FAST_MODE" == "true" ]]; then
    all_changed="$(collect_all_changed)"
    if echo "$all_changed" | grep -qE '^cli/'; then
        HAS_GO=1
    else
        HAS_GO=0
    fi
    if echo "$all_changed" | grep -qE '^skills/|^skills-codex|^tests/skills/'; then
        HAS_SKILL=1
    else
        HAS_SKILL=0
    fi
    if echo "$all_changed" | grep -qE '^hooks/|^cli/embedded/|^cli/Makefile$|^scripts/validate-embedded-sync\.sh$|^lib/|^skills/standards/references/|^skills/using-agentops/SKILL\.md$'; then
        HAS_HOOK=1
    else
        HAS_HOOK=0
    fi
    if echo "$all_changed" | grep -qE '^docs/|^README\.md|^CHANGELOG|^PRODUCT\.md|^SKILL-TIERS\.md'; then
        HAS_DOCS=1
    else
        HAS_DOCS=0
    fi
    if echo "$all_changed" | grep -qE '\.sh$'; then
        HAS_SHELL=1
    else
        HAS_SHELL=0
    fi
    if echo "$all_changed" | grep -qE '^\.agents/learnings/'; then
        HAS_LEARNING=1
    else
        HAS_LEARNING=0
    fi
    if echo "$all_changed" | grep -qE '^evals/|^schemas/eval-|^scripts/eval-agentops\.sh$|^cli/internal/eval/|^cli/cmd/ao/eval'; then
        HAS_EVAL=1
    else
        HAS_EVAL=0
    fi
    if echo "$all_changed" | grep -qE '^docs/contracts/|^schemas/|^scripts/check-contract-compatibility\.sh$|^docs/documentation-index\.md$'; then
        HAS_CONTRACT=1
    else
        HAS_CONTRACT=0
    fi
    if echo "$all_changed" | grep -qE '^\.github/workflows/validate\.yml$|^docs/CI-CD\.md$|^AGENTS\.md$|^scripts/validate-ci-policy-parity\.sh$'; then
        HAS_CI_POLICY=1
    else
        HAS_CI_POLICY=0
    fi
    if echo "$all_changed" | grep -qE '^\.agents/swarm/|^schemas/swarm-|^scripts/validate-swarm-evidence\.sh$'; then
        HAS_SWARM=1
    else
        HAS_SWARM=0
    fi
    if echo "$all_changed" | grep -qE '(^|/)CHANGELOG\.md$'; then
        HAS_CHANGELOG=1
    else
        HAS_CHANGELOG=0
    fi
fi

needs_check() {
    local category="$1"
    maybe_fail_fast
    if [[ "$FAST_MODE" != "true" ]]; then
        return 0
    fi
    case "$category" in
        go)       [[ "$HAS_GO" -eq 1 ]] ;;
        skill)    [[ "$HAS_SKILL" -eq 1 ]] ;;
        hook)     [[ "$HAS_HOOK" -eq 1 ]] ;;
        docs)     [[ "$HAS_DOCS" -eq 1 ]] ;;
        shell)    [[ "$HAS_SHELL" -eq 1 ]] ;;
        learning) [[ "$HAS_LEARNING" -eq 1 ]] ;;
        eval)     [[ "$HAS_EVAL" -eq 1 ]] ;;
        contract) [[ "$HAS_CONTRACT" -eq 1 ]] ;;
        ci_policy) [[ "$HAS_CI_POLICY" -eq 1 ]] ;;
        swarm)    [[ "$HAS_SWARM" -eq 1 ]] ;;
        changelog) [[ "$HAS_CHANGELOG" -eq 1 ]] ;;
        always)   return 0 ;;
        *)        return 0 ;;
    esac
}

collect_go_changed() {
    case "$SCOPE" in
        upstream)
            git diff --name-only '@{upstream}...HEAD' -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
            ;;
        staged)
            git diff --name-only --cached -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
            ;;
        worktree)
            {
                git diff --name-only --cached -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
                git diff --name-only -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
        head)
            git show --name-only --pretty=format: HEAD -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
            ;;
        auto)
            {
                git diff --name-only '@{upstream}...HEAD' -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
                git diff --name-only --cached -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
                git diff --name-only -- 'cli/*.go' 'cli/**/*.go' 'cli/go.mod' 'cli/go.sum' 2>/dev/null || true
            } | sed '/^[[:space:]]*$/d' | sort -u
            ;;
    esac
}

changed_paths() {
    if [[ -n "${all_changed:-}" ]]; then
        printf '%s\n' "$all_changed"
    else
        collect_all_changed
    fi
}

needs_release_audit_artifact_check() {
    changed_paths | grep -qE '^(docs/releases/.*-audit\.md|scripts/(ci-local-release|resolve-release-artifacts|validate-release-audit-artifacts)\.sh|tests/scripts/release-artifacts\.bats)$'
}

if [[ "$FAST_MODE" == "true" ]]; then
    echo "pre-push gate (fast): validating changed files before push..."
    echo "  go=$HAS_GO skill=$HAS_SKILL hook=$HAS_HOOK docs=$HAS_DOCS shell=$HAS_SHELL learning=$HAS_LEARNING eval=$HAS_EVAL contract=$HAS_CONTRACT ci_policy=$HAS_CI_POLICY swarm=$HAS_SWARM changelog=$HAS_CHANGELOG"
else
    echo "pre-push gate: validating before push..."
fi

# --- 1. Go build + vet ---
if needs_check go; then
    if command -v go >/dev/null 2>&1 && [[ -f cli/go.mod ]]; then
        go_changed="$(collect_go_changed)"
        if [[ -n "$go_changed" ]]; then
            if (cd cli && go build -o /dev/null ./cmd/ao 2>&1); then
                pass "go build"
            else
                fail "go build"
            fi
            if (cd cli && go vet ./... 2>&1); then
                pass "go vet"
            else
                fail "go vet"
            fi
        else
            pass "go build (no Go changes)"
        fi
    fi
else
    skip "go build + vet"
fi

# --- 2. Go race tests on changed scope ---
if needs_check go; then
    if [[ -x scripts/validate-go-fast.sh ]]; then
        if go_fast_output="$(scripts/validate-go-fast.sh --scope "$SCOPE" 2>&1)"; then
            pass "go test -race (changed scope)"
        else
            fail "go test -race (changed scope)"
            indent_output "$go_fast_output"
        fi
    else
        fail "missing executable: scripts/validate-go-fast.sh"
    fi
else
    skip "go test -race"
fi

# --- 3. Command/test pairing for command-surface changes ---
if needs_check go; then
    if [[ -x scripts/check-go-command-test-pair.sh ]]; then
        if pair_output="$(scripts/check-go-command-test-pair.sh 2>&1)"; then
            pass "command/test pairing"
        else
            fail "command/test pairing"
            indent_output "$pair_output"
        fi
    else
        fail "missing executable: scripts/check-go-command-test-pair.sh"
    fi
else
    skip "command/test pairing"
fi

# --- 3a. Mutation-route bypass guard (soc-8inr.5 amendment A2) ---
# Asserts cli/internal/daemon/ never registers a mutation route via direct
# mux.HandleFunc — registerMutationRoute / registerReadOnlyRoute is the only
# allowed registration path. Always-on (cost ~50ms): a bypass landing on main
# would silently expose ledger mutations without auth.
if [[ -x scripts/check-mutation-route-coverage.sh ]]; then
    if mutation_route_output="$(scripts/check-mutation-route-coverage.sh 2>&1)"; then
        pass "mutation route bypass guard"
    else
        fail "mutation route bypass guard"
        indent_output "$mutation_route_output"
    fi
else
    fail "missing executable: scripts/check-mutation-route-coverage.sh"
fi

# --- 3b. HOME isolation in harvest.*/RunIngest tests ---
if needs_check go; then
    if [[ -x scripts/check-home-isolation.sh ]]; then
        if home_iso_output="$(scripts/check-home-isolation.sh 2>&1)"; then
            pass "HOME isolation in test files"
        else
            fail "HOME isolation in test files"
            indent_output "$home_iso_output"
        fi
    else
        fail "missing executable: scripts/check-home-isolation.sh"
    fi
else
    skip "HOME isolation in test files"
fi

# --- 3c. Capture ~/.agents hash snapshot (diff'd at end of gate) ---
# This gate protects Go tests from mutating the operator's real agent hub. In
# local fast mode, skip it when no Go checks are running unless explicitly
# requested; user-hub state is noisy and unrelated to docs/shell pushes.
if [[ -x scripts/check-agents-hash-snapshot.sh ]] && \
    { [[ "$FAST_MODE" != "true" ]] || [[ "$HAS_GO" -eq 1 ]] || truthy "${PRE_PUSH_AGENT_HASH:-0}"; }; then
    hash_capture_err="$(mktemp)"
    if hash_capture_output="$(run_hash_snapshot capture 2>"$hash_capture_err")"; then
        if [[ -n "$hash_capture_output" && -f "$hash_capture_output" ]]; then
            HASH_GATE_SNAPSHOT="$hash_capture_output"
        elif is_ci_env; then
            fail "agents-hub content-hash gate snapshot not captured"
            indent_output "$(cat "$hash_capture_err")"
        else
            warn "agents-hub content-hash gate snapshot not captured; local mutation check skipped"
            indent_output "$(cat "$hash_capture_err")"
        fi
    else
        hash_capture_status=$?
        if [[ "$hash_capture_status" -eq 124 ]] && ! is_ci_env; then
            warn "agents-hub content-hash gate snapshot timed out locally after ${HASH_GATE_TIMEOUT_SECONDS:-15}s; use AGENTS_HUB_OVERRIDE or HASH_GATE_IGNORE_UNTRACKED=1 when shared-agent state is noisy"
        else
            fail "agents-hub content-hash gate snapshot failed"
            indent_output "$(cat "$hash_capture_err")"
        fi
    fi
    rm -f "$hash_capture_err"
else
    skip "agents-hub content-hash gate (no Go checks in local fast mode)"
fi

# --- 3d. .agents/ write-surface contract ---
if [[ -x scripts/check-agents-write-surfaces.sh && -f docs/contracts/agents-write-surfaces.md ]]; then
    if write_surfaces_output="$(scripts/check-agents-write-surfaces.sh 2>&1)"; then
        pass ".agents/ write-surface contract"
    else
        fail ".agents/ write-surface contract drifted"
        indent_output "$write_surfaces_output"
    fi
else
    skip ".agents/ write-surface contract"
fi

# --- 3e. No tracked repo-root .agents state ---
if [[ -x scripts/check-no-tracked-agents.sh ]]; then
    if no_tracked_agents_output="$(scripts/check-no-tracked-agents.sh 2>&1)"; then
        pass "no tracked .agents state"
    else
        fail "tracked .agents state"
        indent_output "$no_tracked_agents_output"
    fi
else
    fail "missing executable: scripts/check-no-tracked-agents.sh"
fi

# --- 5. Embedded hooks sync (full parity gate, always-on) ---
# Unconditional: even pure-Go diffs can interact with embedded fixtures, and the
# CI-side gate is unconditional. ~0.5-1s overhead. Caught 7/20 prior failures.
if [[ -x scripts/validate-embedded-sync.sh ]]; then
    if embed_output="$(./scripts/validate-embedded-sync.sh 2>&1)"; then
        pass "embedded hooks in sync"
    else
        fail "embedded hooks stale (run: cd cli && make sync-hooks)"
        indent_output "$embed_output"
    fi
else
    fail "missing executable: scripts/validate-embedded-sync.sh"
fi

# --- 5b. Test-fixture parity (hook coverage + pre-push helper stubs) ---
# Mirror CI assertions at pre-push speed: every hooks/*.sh has test coverage,
# and every helper-script reference in pre-push-gate.sh has a fake-repo stub.
# Targets the 17/20 bats-tests + 13/20 cli-integration parity-drift recurrence.
if needs_check hook || needs_check shell; then
    if [[ -x scripts/check-test-fixture-parity.sh ]]; then
        if parity_output="$(./scripts/check-test-fixture-parity.sh 2>&1)"; then
            pass "test-fixture parity (hooks + pre-push helper stubs)"
        else
            fail "test-fixture parity break (see below)"
            indent_output "$parity_output"
        fi
    fi
else
    skip "test-fixture parity"
fi

# --- 6. Skill count sync ---
if needs_check skill; then
    if [[ -x scripts/sync-skill-counts.sh ]]; then
        if scripts/sync-skill-counts.sh --check >/dev/null 2>&1; then
            pass "skill counts in sync"
        else
            fail "skill counts out of sync (run: scripts/sync-skill-counts.sh)"
        fi
    fi
else
    skip "skill counts"
fi

# --- 6b. CLI skills map count parity ---
# Always-on (cost ~50ms). Catches the 59a1efa3 -> 0f047c53 regression pattern
# where docs/cli-skills-map.md's "<N> generated CLI command headings" line
# drifted from the actual count in cli/docs/COMMANDS.md (declared 212, real 58).
if [[ -x scripts/validate-cli-skills-map.sh ]]; then
    if cli_map_output="$(scripts/validate-cli-skills-map.sh 2>&1)"; then
        pass "cli-skills-map count parity"
    else
        fail "cli-skills-map count parity"
        indent_output "$cli_map_output"
    fi
else
    fail "missing executable: scripts/validate-cli-skills-map.sh"
fi

# --- 7. Worktree disposition ---
# Full/release gates still enforce repository worktree governance. Local fast
# pre-push skips it by default because stale unrelated worktrees should not
# block pushing a scoped commit; opt in with PRE_PUSH_STRICT_WORKTREE=1.
if [[ "$FAST_MODE" != "true" ]] || is_ci_env || truthy "${PRE_PUSH_STRICT_WORKTREE:-0}"; then
    if [[ -x scripts/check-worktree-disposition.sh ]]; then
        if disposition_output="$(scripts/check-worktree-disposition.sh 2>&1)"; then
            pass "worktree disposition"
        else
            fail "worktree disposition"
            indent_output "$disposition_output"
        fi
    else
        fail "missing executable: scripts/check-worktree-disposition.sh"
    fi
else
    skip "worktree disposition (local fast; set PRE_PUSH_STRICT_WORKTREE=1)"
fi

# --- 8. Skill runtime/CLI parity ---
if needs_check skill; then
    if [[ -x scripts/validate-skill-runtime-parity.sh ]]; then
        if skill_runtime_output="$(scripts/validate-skill-runtime-parity.sh 2>&1)"; then
            pass "skill runtime parity"
        else
            fail "skill runtime parity"
            indent_output "$skill_runtime_output"
        fi
    else
        fail "missing executable: scripts/validate-skill-runtime-parity.sh"
    fi
else
    skip "skill runtime parity"
fi

# --- 9. Codex skill parity --- (removed: skills-codex/ is manually maintained)
skip "codex skill parity (manually maintained)"

# --- 10. Codex install bundle parity --- (removed: skills-codex/ is manually maintained)
skip "codex install bundle parity (manually maintained)"

# --- 11. Codex runtime section format ---
if needs_check skill; then
    if [[ -x scripts/validate-codex-runtime-sections.sh ]]; then
        if codex_runtime_output="$(scripts/validate-codex-runtime-sections.sh 2>&1)"; then
            pass "codex runtime sections"
        else
            fail "codex runtime sections"
            indent_output "$codex_runtime_output"
        fi
    else
        fail "missing executable: scripts/validate-codex-runtime-sections.sh"
    fi
else
    skip "codex runtime sections"
fi

# --- 12. Skill integrity ---
if needs_check skill; then
    if [[ -x skills/heal-skill/scripts/heal.sh ]]; then
        if skill_integrity_output="$(bash skills/heal-skill/scripts/heal.sh --strict 2>&1)"; then
            pass "skill integrity"
        else
            fail "skill integrity"
            indent_output "$skill_integrity_output"
        fi
    else
        fail "missing executable: skills/heal-skill/scripts/heal.sh"
    fi
else
    skip "skill integrity"
fi

# --- 13. Skill lint suite ---
if needs_check skill; then
    if [[ -x tests/skills/run-all.sh ]]; then
        if skill_lint_output="$(bash tests/skills/run-all.sh 2>&1)"; then
            pass "skill lint suite"
        else
            fail "skill lint suite"
            indent_output "$skill_lint_output"
        fi
    else
        fail "missing executable: tests/skills/run-all.sh"
    fi
else
    skip "skill lint suite"
fi

# --- 14. Skill schema validation ---
if needs_check skill; then
    if [[ -x scripts/validate-skill-schema.sh ]]; then
        if skill_schema_output="$(scripts/validate-skill-schema.sh 2>&1)"; then
            pass "skill schema validation"
        else
            fail "skill schema validation"
            indent_output "$skill_schema_output"
        fi
    else
        fail "missing executable: scripts/validate-skill-schema.sh"
    fi
else
    skip "skill schema validation"
fi

# --- 15. Manifest schema validation ---
if needs_check skill; then
    if [[ -x scripts/validate-manifests.sh ]]; then
        if manifest_output="$(scripts/validate-manifests.sh --repo-root . 2>&1)"; then
            pass "manifest schema validation"
        else
            fail "manifest schema validation"
            indent_output "$manifest_output"
        fi
    else
        fail "missing executable: scripts/validate-manifests.sh"
    fi
else
    skip "manifest schema validation"
fi

# --- 16. Codex artifact metadata ---
if needs_check skill; then
    if [[ -x scripts/validate-codex-generated-artifacts.sh ]]; then
        if codex_generated_output="$(scripts/validate-codex-generated-artifacts.sh --scope "$SCOPE" 2>&1)"; then
            pass "codex artifact metadata"
        else
            fail "codex artifact metadata"
            indent_output "$codex_generated_output"
        fi
    else
        fail "missing executable: scripts/validate-codex-generated-artifacts.sh"
    fi
else
    skip "codex artifact metadata"
fi

# --- 17. Codex backbone prompts ---
if needs_check skill; then
    if [[ -x scripts/validate-codex-backbone-prompts.sh ]]; then
        if codex_backbone_output="$(scripts/validate-codex-backbone-prompts.sh 2>&1)"; then
            pass "codex backbone prompts"
        else
            fail "codex backbone prompts"
            indent_output "$codex_backbone_output"
        fi
    else
        fail "missing executable: scripts/validate-codex-backbone-prompts.sh"
    fi
else
    skip "codex backbone prompts"
fi

# --- 18. Codex override coverage ---
if needs_check skill; then
    if [[ -x scripts/validate-codex-override-coverage.sh ]]; then
        if codex_override_output="$(scripts/validate-codex-override-coverage.sh 2>&1)"; then
            pass "codex override coverage"
        else
            fail "codex override coverage"
            indent_output "$codex_override_output"
        fi
    else
        fail "missing executable: scripts/validate-codex-override-coverage.sh"
    fi
else
    skip "codex override coverage"
fi

# --- 19. Next-work contract parity ---
if [[ -x scripts/validate-next-work-contract-parity.sh ]]; then
    if next_work_contract_output="$(scripts/validate-next-work-contract-parity.sh 2>&1)"; then
        pass "next-work contract parity"
    else
        fail "next-work contract parity"
        indent_output "$next_work_contract_output"
    fi
else
    fail "missing executable: scripts/validate-next-work-contract-parity.sh"
fi

# --- 19b. bd closeout contract parity ---
if [[ -x scripts/validate-bd-closeout-contract.sh ]]; then
    if bd_closeout_output="$(scripts/validate-bd-closeout-contract.sh 2>&1)"; then
        pass "bd closeout contract parity"
    else
        fail "bd closeout contract parity"
        indent_output "$bd_closeout_output"
    fi
else
    fail "missing executable: scripts/validate-bd-closeout-contract.sh"
fi

# --- 19c. Retrieval quality ratchet ---
if [[ "$FAST_MODE" != "true" ]] || truthy "${PRE_PUSH_AGENT_HEALTH:-0}"; then
    if [[ -x scripts/check-retrieval-quality-ratchet.sh ]]; then
        if retrieval_quality_output="$(run_without_git_env scripts/check-retrieval-quality-ratchet.sh 2>&1)"; then
            if grep -q '^WARN retrieval quality ratchet:' <<<"$retrieval_quality_output"; then
                warn "retrieval quality ratchet"
                indent_output "$retrieval_quality_output"
            else
                pass "retrieval quality ratchet"
            fi
        else
            fail "retrieval quality ratchet"
            indent_output "$retrieval_quality_output"
        fi
    else
        fail "missing executable: scripts/check-retrieval-quality-ratchet.sh"
    fi
else
    skip "retrieval quality ratchet (local fast; set PRE_PUSH_AGENT_HEALTH=1)"
fi

# --- 19d. Retrieval manifest path validation (fast: always-on) ---
if [[ -x scripts/check-retrieval-manifest-paths.sh ]]; then
    retrieval_manifests=()
    while IFS= read -r m; do
        retrieval_manifests+=("$m")
    done < <(find cli/cmd/ao/testdata/retrieval-bench -maxdepth 2 -name '*manifest*.json' -type f 2>/dev/null | sort)
    if [[ ${#retrieval_manifests[@]} -gt 0 ]]; then
        if retrieval_paths_output="$(scripts/check-retrieval-manifest-paths.sh "${retrieval_manifests[@]}" 2>&1)"; then
            pass "retrieval manifest paths"
        else
            fail "retrieval manifest paths"
            indent_output "$retrieval_paths_output"
        fi
    else
        skip "retrieval manifest paths (no manifests found)"
    fi
else
    skip "retrieval manifest paths (script missing)"
fi

# --- 20. Skill runtime formats ---
if needs_check skill; then
    if [[ -x scripts/validate-skill-runtime-formats.sh ]]; then
        if codex_lint_output="$(scripts/validate-skill-runtime-formats.sh 2>&1)"; then
            pass "skill runtime formats"
        else
            fail "skill runtime formats"
            indent_output "$codex_lint_output"
        fi
    else
        fail "missing executable: scripts/validate-skill-runtime-formats.sh"
    fi
else
    skip "skill runtime formats"
fi

# --- 21. Codex RPI contract validation ---
if needs_check skill; then
    if [[ -f scripts/validate-codex-rpi-contract.sh ]]; then
        if codex_rpi_contract_output="$(bash scripts/validate-codex-rpi-contract.sh 2>&1)"; then
            pass "codex RPI contract"
        else
            fail "codex RPI contract"
            indent_output "$codex_rpi_contract_output"
        fi
    else
        fail "missing file: scripts/validate-codex-rpi-contract.sh"
    fi
else
    skip "codex RPI contract"
fi

# --- 22. Codex lifecycle guard validation ---
if needs_check skill; then
    if [[ -x scripts/validate-codex-lifecycle-guards.sh ]]; then
        if codex_lifecycle_output="$(bash scripts/validate-codex-lifecycle-guards.sh 2>&1)"; then
            pass "codex lifecycle guards"
        else
            fail "codex lifecycle guards"
            indent_output "$codex_lifecycle_output"
        fi
    else
        fail "missing executable: scripts/validate-codex-lifecycle-guards.sh"
    fi
else
    skip "codex lifecycle guards"
fi

# --- 23. Skill CLI snippets ---
if needs_check skill; then
    if [[ -x scripts/validate-skill-cli-snippets.sh ]]; then
        if skill_cli_output="$(run_without_git_env scripts/validate-skill-cli-snippets.sh 2>&1)"; then
            pass "skill CLI snippets"
        else
            fail "skill CLI snippets"
            indent_output "$skill_cli_output"
        fi
    else
        fail "missing executable: scripts/validate-skill-cli-snippets.sh"
    fi
else
    skip "skill CLI snippets"
fi

# --- 24. Headless runtime skill smoke ---
# Skip in fast mode — requires nested Claude/Codex which fails inside Claude sessions
if needs_check always && [[ "$FAST_MODE" != "true" ]]; then
    if [[ -x scripts/validate-headless-runtime-skills.sh ]]; then
        if runtime_smoke_output="$(scripts/validate-headless-runtime-skills.sh 2>&1)"; then
            pass "headless runtime skills"
            indent_output "$runtime_smoke_output"
        else
            fail "headless runtime skills"
            indent_output "$runtime_smoke_output"
        fi
    else
        fail "missing executable: scripts/validate-headless-runtime-skills.sh"
    fi
else
    skip "headless runtime skills"
fi

# --- 24b. CLI docs parity (generate-cli-reference.sh --check) ---
# Trigger on any cmd/ao/*.go change (help text, flags, etc.), not just go build changes
HAS_CMD_AO=0
if [[ "$FAST_MODE" == "true" ]]; then
    if echo "$all_changed" | grep -qE '^cli/cmd/ao/.*\.go$'; then
        HAS_CMD_AO=1
    fi
fi
if needs_check go || [[ "$HAS_CMD_AO" -eq 1 ]]; then
    if [[ -x scripts/generate-cli-reference.sh ]]; then
        if cli_docs_output="$(run_without_git_env scripts/generate-cli-reference.sh --check 2>&1)"; then
            pass "CLI docs parity"
        else
            fail "CLI docs parity (run: scripts/generate-cli-reference.sh)"
            indent_output "$cli_docs_output"
        fi
    else
        fail "missing executable: scripts/generate-cli-reference.sh"
    fi
else
    skip "CLI docs parity"
fi

# --- 24c. AgentOps eval canaries ---
run_eval_canaries=false
if [[ "${PRE_PUSH_SKIP_EVAL:-0}" == "1" ]]; then
    run_eval_canaries=false
elif truthy "${PRE_PUSH_RUN_EVAL:-0}"; then
    run_eval_canaries=true
elif needs_check eval; then
    run_eval_canaries=true
fi

if [[ "$run_eval_canaries" == "true" ]]; then
    if [[ -x scripts/eval-agentops.sh ]]; then
        eval_args=(--fast)
        eval_is_advisory=false
        if [[ "$FAST_MODE" == "true" ]] && ! is_ci_env && ! truthy "${PRE_PUSH_STRICT_EVAL:-0}"; then
            eval_args+=(--advisory)
            eval_is_advisory=true
        fi
        if eval_agentops_output="$(run_without_git_env_isolated_agents_home scripts/eval-agentops.sh "${eval_args[@]}" 2>&1)"; then
            if grep -q '^FAIL eval-agentops:' <<<"$eval_agentops_output"; then
                if [[ "$eval_is_advisory" == "true" ]]; then
                    warn "AgentOps eval canaries (advisory)"
                else
                    fail "AgentOps eval canaries"
                fi
                indent_output "$eval_agentops_output"
            elif grep -q '^WARN eval-agentops:' <<<"$eval_agentops_output"; then
                warn "AgentOps eval canaries"
                indent_output "$eval_agentops_output"
            else
                pass "AgentOps eval canaries"
            fi
        else
            fail "AgentOps eval canaries"
            indent_output "$eval_agentops_output"
        fi
    else
        fail "missing executable: scripts/eval-agentops.sh"
    fi
else
    skip "AgentOps eval canaries (local fast: no eval changes; set PRE_PUSH_RUN_EVAL=1)"
fi

# --- 24d. AgentOps eval baseline-audit ---
# Catches stale promoted baselines (suite SHA drifted relative to baseline) and
# policy mismatches (promoted baseline exists for a baseline_policy.mode=none
# suite, or vice versa). Runs alongside 24c whenever eval files changed.
run_baseline_audit=false
if [[ "${PRE_PUSH_SKIP_EVAL:-0}" == "1" ]]; then
    run_baseline_audit=false
elif truthy "${PRE_PUSH_RUN_EVAL:-0}"; then
    run_baseline_audit=true
elif needs_check eval; then
    run_baseline_audit=true
fi

if [[ "$run_baseline_audit" == "true" ]]; then
    ao_bin=""
    if [[ -x cli/bin/ao ]]; then
        ao_bin="cli/bin/ao"
    elif command -v ao >/dev/null 2>&1; then
        ao_bin="$(command -v ao)"
    fi
    if [[ -n "$ao_bin" ]]; then
        if audit_output="$("$ao_bin" eval baseline-audit --root evals/agentops-core --json 2>&1)"; then
            stale_count=$(printf '%s' "$audit_output" | python3 -c 'import json,sys
try:
    d=json.load(sys.stdin)
    print(len(d.get("stale_suite_hashes",[])))
except Exception:
    print(-1)' 2>/dev/null)
            mismatch_count=$(printf '%s' "$audit_output" | python3 -c 'import json,sys
try:
    d=json.load(sys.stdin)
    print(int(d.get("policy_mismatch_count",0)))
except Exception:
    print(-1)' 2>/dev/null)
            if [[ "$stale_count" == "-1" || "$mismatch_count" == "-1" ]]; then
                fail "AgentOps eval baseline-audit (could not parse audit output)"
                indent_output "$audit_output"
            elif [[ "$stale_count" -gt 0 || "$mismatch_count" -gt 0 ]]; then
                fail "AgentOps eval baseline-audit (stale=$stale_count, policy_mismatch=$mismatch_count)"
                indent_output "$audit_output"
            else
                pass "AgentOps eval baseline-audit"
            fi
        else
            fail "AgentOps eval baseline-audit"
            indent_output "$audit_output"
        fi
    else
        skip "AgentOps eval baseline-audit (no ao binary; build cli/bin/ao first)"
    fi
else
    skip "AgentOps eval baseline-audit (local fast: no eval changes; set PRE_PUSH_RUN_EVAL=1)"
fi

# --- 25. Doc-release stabilization gate ---
if needs_check docs || needs_check skill; then
    if [[ -x tests/docs/validate-doc-release.sh ]]; then
        if doc_release_output="$(./tests/docs/validate-doc-release.sh 2>&1)"; then
            pass "doc-release gate"
        else
            fail "doc-release gate (run: ./tests/docs/validate-doc-release.sh)"
            indent_output "$doc_release_output"
        fi
    else
        fail "missing executable: tests/docs/validate-doc-release.sh"
    fi
else
    skip "doc-release gate"
fi

# --- 25a. MkDocs strict build (fast, optional — requires Python + requirements-docs.txt) ---
if needs_check docs || needs_check skill; then
    if [[ "${PRE_PUSH_SKIP_MKDOCS:-0}" == "1" ]]; then
        skip "mkdocs strict build (PRE_PUSH_SKIP_MKDOCS=1)"
    elif [[ -x scripts/docs-build.sh ]] && command -v python3 >/dev/null 2>&1; then
        if mkdocs_output="$(scripts/docs-build.sh --check 2>&1)"; then
            pass "mkdocs strict build"
        else
            fail "mkdocs strict build (run: scripts/docs-build.sh --check)"
            indent_output "$mkdocs_output"
        fi
    else
        skip "mkdocs strict build (python3 or scripts/docs-build.sh missing)"
    fi
else
    skip "mkdocs strict build"
fi

# --- 25b. Release audit artifact refs ---
if needs_release_audit_artifact_check; then
    if [[ -x scripts/validate-release-audit-artifacts.sh ]]; then
        if release_audit_artifacts_output="$(RELEASE_AUDIT_CHANGED_PATHS="$(changed_paths)" scripts/validate-release-audit-artifacts.sh --mode changed 2>&1)"; then
            pass "release audit artifacts"
        else
            fail "release audit artifacts"
            indent_output "$release_audit_artifacts_output"
        fi
    else
        fail "missing executable: scripts/validate-release-audit-artifacts.sh"
    fi
else
    skip "release audit artifacts"
fi

# --- 26. Contract compatibility ---
if needs_check contract; then
    if [[ -x scripts/check-contract-compatibility.sh ]]; then
        if contract_output="$(./scripts/check-contract-compatibility.sh 2>&1)"; then
            pass "contract compatibility"
        else
            fail "contract compatibility (run: ./scripts/check-contract-compatibility.sh)"
            indent_output "$contract_output"
        fi
    else
        fail "missing executable: scripts/check-contract-compatibility.sh"
    fi
else
    skip "contract compatibility"
fi

# --- 26b. Swarm evidence schema validation ---
if needs_check swarm; then
    if [[ -x scripts/validate-swarm-evidence.sh ]]; then
        if swarm_evidence_output="$(./scripts/validate-swarm-evidence.sh 2>&1)"; then
            pass "swarm evidence schema"
        else
            fail "swarm evidence schema (run: ./scripts/validate-swarm-evidence.sh)"
            indent_output "$swarm_evidence_output"
        fi
    else
        fail "missing executable: scripts/validate-swarm-evidence.sh"
    fi
else
    skip "swarm evidence schema"
fi

# --- 27. Hook preflight ---
if needs_check hook; then
    if [[ -x scripts/validate-hook-preflight.sh ]]; then
        if hook_preflight_output="$(./scripts/validate-hook-preflight.sh 2>&1)"; then
            pass "hook preflight"
        else
            fail "hook preflight"
            indent_output "$hook_preflight_output"
        fi
    else
        fail "missing executable: scripts/validate-hook-preflight.sh"
    fi
else
    skip "hook preflight"
fi

# --- 27b. Standards-injector reference completeness ---
if needs_check hook; then
    if [[ -x scripts/check-standards-injector-completeness.sh ]]; then
        if standards_inj_output="$(./scripts/check-standards-injector-completeness.sh 2>&1)"; then
            pass "standards-injector references complete"
        else
            fail "standards-injector references complete"
            indent_output "$standards_inj_output"
        fi
    else
        fail "missing executable: scripts/check-standards-injector-completeness.sh"
    fi
else
    skip "standards-injector references complete"
fi

# --- 28. Hooks/docs parity ---
if needs_check hook; then
    if [[ -x scripts/validate-hooks-doc-parity.sh ]]; then
        if hooks_doc_output="$(./scripts/validate-hooks-doc-parity.sh 2>&1)"; then
            pass "hooks/docs parity"
        else
            fail "hooks/docs parity"
            indent_output "$hooks_doc_output"
        fi
    else
        fail "missing executable: scripts/validate-hooks-doc-parity.sh"
    fi
else
    skip "hooks/docs parity"
fi

# --- 29. CI policy parity ---
if needs_check ci_policy; then
    if [[ -x scripts/validate-ci-policy-parity.sh ]]; then
        if ci_policy_output="$(./scripts/validate-ci-policy-parity.sh 2>&1)"; then
            pass "CI policy parity"
        else
            fail "CI policy parity"
            indent_output "$ci_policy_output"
        fi
    else
        fail "missing executable: scripts/validate-ci-policy-parity.sh"
    fi
else
    skip "CI policy parity"
fi

# --- 30. ShellCheck on changed scripts ---
if needs_check shell; then
    if command -v shellcheck >/dev/null 2>&1; then
        shell_errors=0
        if [[ "$FAST_MODE" == "true" ]]; then
            # Only check changed .sh files
            changed_sh="$(echo "$all_changed" | grep '\.sh$' || true)"
            if [[ -n "$changed_sh" ]]; then
                while IFS= read -r f; do
                    [[ -f "$f" ]] || continue
                    if ! shellcheck_out="$(shellcheck -S warning "$f" 2>&1)"; then
                        shell_errors=1
                        indent_output "$shellcheck_out"
                    fi
                done <<< "$changed_sh"
            fi
        else
            # Full mode: check all scripts with shebangs
            while IFS= read -r f; do
                [[ -f "$f" ]] || continue
                head -1 "$f" | grep -q '^#!' || continue
                if ! shellcheck_out="$(shellcheck -S warning "$f" 2>&1)"; then
                    shell_errors=1
                    indent_output "$shellcheck_out"
                fi
            done < <(find scripts hooks lib bin -name '*.sh' -type f 2>/dev/null)
        fi
        if [[ "$shell_errors" -eq 0 ]]; then
            pass "shellcheck"
        else
            fail "shellcheck"
        fi
    else
        skip "shellcheck (not installed)"
    fi
else
    skip "shellcheck"
fi

# --- 31. Plugin load test (symlinks + manifest) ---
if needs_check always; then
    symlink_found=0
    while IFS= read -r _; do
        symlink_found=1
        break
    done < <(find skills hooks lib scripts -type l 2>/dev/null)
    if [[ "$symlink_found" -eq 0 ]]; then
        pass "no symlinks"
    else
        fail "symlinks found (CI rejects all symlinks)"
    fi
fi

# --- 32. Learning coherence ---
if needs_check learning; then
    if [[ -x tests/validate-learning-coherence.sh ]]; then
        if learning_output="$(bash tests/validate-learning-coherence.sh 2>&1)"; then
            pass "learning coherence"
        else
            fail "learning coherence"
            indent_output "$learning_output"
        fi
    elif [[ -d .agents/learnings ]]; then
        # Inline check: validate frontmatter on changed learnings
        learning_errors=0
        learn_files="$(find .agents/learnings -name '*.md' -type f 2>/dev/null)"
        if [[ "$FAST_MODE" == "true" ]]; then
            learn_files="$(echo "$all_changed" | grep '^\.agents/learnings/.*\.md$' || true)"
        fi
        for f in $learn_files; do
            [[ -f "$f" ]] || continue
            if ! head -1 "$f" | grep -q '^---'; then
                echo "    missing frontmatter: $f"
                learning_errors=1
            fi
        done
        if [[ "$learning_errors" -eq 0 ]]; then
            pass "learning coherence (inline)"
        else
            fail "learning coherence (missing frontmatter)"
        fi
    else
        skip "learning coherence (no learnings dir)"
    fi
else
    skip "learning coherence"
fi

# --- 33. BATS tests + orphan hooks ---
if needs_check hook; then
    if command -v bats >/dev/null 2>&1 && [[ -d tests/hooks ]]; then
        if [[ -x tests/hooks/test-orphan-hooks.sh ]]; then
            if orphan_output="$(bash tests/hooks/test-orphan-hooks.sh 2>&1)"; then
                pass "orphan hooks audit"
            else
                fail "orphan hooks audit"
                indent_output "$orphan_output"
            fi
        else
            skip "orphan hooks (missing script)"
        fi
    else
        skip "BATS/orphan hooks (bats not installed or no tests/hooks)"
    fi
else
    skip "orphan hooks"
fi

# --- 34. Skill citation parity ---
if needs_check skill; then
    if [[ -x tests/docs/validate-skill-citation-parity.sh ]]; then
        if cite_output="$(bash tests/docs/validate-skill-citation-parity.sh 2>&1)"; then
            pass "skill citation parity"
        else
            fail "skill citation parity"
            indent_output "$cite_output"
        fi
    else
        skip "skill citation parity (missing script)"
    fi
else
    skip "skill citation parity"
fi

# --- 35. Flywheel health (warn only) ---
if [[ "$FAST_MODE" == "true" ]] && ! truthy "${PRE_PUSH_AGENT_HEALTH:-0}"; then
    skip "flywheel health (local fast; set PRE_PUSH_AGENT_HEALTH=1)"
elif command -v ao >/dev/null 2>&1 && [[ -d .agents ]]; then
    if health_output="$(ao metrics health --json 2>/dev/null)"; then
        fly_status="$(echo "$health_output" | grep -o '"flywheel_status":"[^"]*"' | head -1 | cut -d'"' -f4 || true)"
        if [[ "$fly_status" == "DECAYING" ]]; then
            warn "flywheel health: DECAYING — run /evolve or check citation flow"
        elif [[ -n "$fly_status" ]]; then
            pass "flywheel health ($fly_status)"
        else
            skip "flywheel health (no status in output)"
        fi
    else
        skip "flywheel health (ao metrics health failed)"
    fi
else
    skip "flywheel health (ao not available)"
fi

# --- 36. CHANGELOG sync (docs/CHANGELOG.md must match root) ---
if needs_check changelog; then
    if [[ -f docs/CHANGELOG.md && -f CHANGELOG.md ]]; then
        if diff -q CHANGELOG.md docs/CHANGELOG.md >/dev/null 2>&1; then
            pass "CHANGELOG sync"
        else
            fail "CHANGELOG sync (run: cp CHANGELOG.md docs/CHANGELOG.md)"
        fi
    else
        skip "CHANGELOG sync (missing file)"
    fi
else
    skip "CHANGELOG sync"
fi

# --- 37. ~/.agents content-hash gate (post-hoc mutation detector) ---
# Escape hatch: HASH_GATE_IGNORE_UNTRACKED=1 skips this block entirely so
# operators with local scratch state (docs/blog/, codex_write_test.txt, etc.)
# can push without the gate firing on noise. CI does NOT set this variable,
# so strict enforcement is preserved on main.
if [[ "${HASH_GATE_IGNORE_UNTRACKED:-0}" == "1" ]]; then
    skip "agents-hub content-hash gate (HASH_GATE_IGNORE_UNTRACKED=1)"
    cleanup_hash_snapshot
elif [[ -n "$HASH_GATE_SNAPSHOT" && -x scripts/check-agents-hash-snapshot.sh ]]; then
    if hash_gate_output="$(run_hash_snapshot diff "$HASH_GATE_SNAPSHOT" 2>&1)"; then
        pass "agents-hub content-hash gate"
    else
        hash_gate_status=$?
        if [[ "$hash_gate_status" -eq 124 ]] && ! is_ci_env; then
            warn "agents-hub content-hash gate diff timed out locally after ${HASH_GATE_TIMEOUT_SECONDS:-15}s; use AGENTS_HUB_OVERRIDE or HASH_GATE_IGNORE_UNTRACKED=1 when shared-agent state is noisy"
            indent_output "$hash_gate_output"
        else
            fail "agents-hub mutated during tests (content-hash gate)"
            indent_output "$hash_gate_output"
        fi
    fi
    cleanup_hash_snapshot
fi

# --- Summary ---
maybe_fail_fast
echo ""
if [[ $errors -gt 0 ]]; then
    if [[ "$FAST_MODE" == "true" ]]; then
        echo -e "${RED}pre-push gate (fast): BLOCKED ($errors failures, $skipped skipped)${NC}"
    else
        echo -e "${RED}pre-push gate: BLOCKED ($errors failures)${NC}"
    fi
    exit 1
else
    if [[ "$FAST_MODE" == "true" && -x scripts/pre-push-proof.sh ]]; then
        if ! scripts/pre-push-proof.sh write --scope "$SCOPE" --mode fast >/dev/null 2>&1; then
            warn "pre-push validation proof not recorded"
        fi
    fi
    if [[ "$FAST_MODE" == "true" ]]; then
        echo -e "${GREEN}pre-push gate (fast): passed ($skipped skipped)${NC}"
    else
        echo -e "${GREEN}pre-push gate: passed${NC}"
    fi
    exit 0
fi

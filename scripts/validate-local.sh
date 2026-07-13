#!/bin/bash
# Local plugin validation - manual wrapper around the same gate used on push.
# Usage: ./scripts/validate-local.sh [--scope worktree] [--skip-claude]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; errors=$((errors + 1)); }
warn() { echo -e "${YELLOW}!${NC} $1"; }
print_indented() {
    local text="$1"
    while IFS= read -r line; do
        printf '    %s\n' "$line"
    done <<<"$text"
}

usage() {
    cat <<'EOF'
Usage: ./scripts/validate-local.sh [--scope auto|upstream|staged|worktree|head] [--skip-claude]

Preferred hook setup:
  bash scripts/install-dev-hooks.sh
EOF
}

errors=0
SCOPE="worktree"
SKIP_CLAUDE="false"
VALIDATE_LOCAL_LOCK_DIR=""

release_validate_local_lock() {
    local pid_file

    [[ -n "$VALIDATE_LOCAL_LOCK_DIR" ]] || return 0
    pid_file="$VALIDATE_LOCAL_LOCK_DIR/pid"

    if [[ -f "$pid_file" ]] && [[ "$(<"$pid_file")" != "$$" ]]; then
        return 0
    fi

    rm -rf "$VALIDATE_LOCAL_LOCK_DIR"
}

acquire_validate_local_lock() {
    local git_dir pid_file existing_pid

    git_dir="$(git rev-parse --git-dir 2>/dev/null || printf '%s\n' "$REPO_ROOT/.git")"
    if [[ "$git_dir" != /* ]]; then
        git_dir="$REPO_ROOT/$git_dir"
    fi

    VALIDATE_LOCAL_LOCK_DIR="$git_dir/agentops-validate-local.lock"
    pid_file="$VALIDATE_LOCAL_LOCK_DIR/pid"

    while true; do
        if mkdir "$VALIDATE_LOCAL_LOCK_DIR" 2>/dev/null; then
            printf '%s\n' "$$" > "$pid_file"
            trap release_validate_local_lock EXIT INT TERM
            return 0
        fi

        existing_pid=""
        if [[ -f "$pid_file" ]]; then
            existing_pid="$(<"$pid_file")"
        fi

        if [[ -n "$existing_pid" ]] && kill -0 "$existing_pid" 2>/dev/null; then
            echo "error: another local validation is already running (pid $existing_pid)" >&2
            return 1
        fi

        rm -rf "$VALIDATE_LOCAL_LOCK_DIR"
    done
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --scope)
            SCOPE="${2:-}"
            shift 2
            ;;
        --skip-claude)
            SKIP_CLAUDE="true"
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

cd "$REPO_ROOT"

if ! acquire_validate_local_lock; then
    exit 1
fi

hooks_path="$(git config --local --get core.hooksPath 2>/dev/null || true)"
if [[ "$hooks_path" != ".githooks" ]]; then
    warn "core.hooksPath is '${hooks_path:-<unset>}' (recommended: .githooks)"
    warn "Run: bash scripts/install-dev-hooks.sh"
fi

echo ""
echo "🔍 Running manual local validation..."
echo ""
echo "═══════════════════════════════════════════════════════"
echo "  AgentOps Manual Local Validation"
echo "═══════════════════════════════════════════════════════"
echo ""

echo "── Shared Local Gate ──"
ao_bin="${AO_BIN:-}"
[[ -z "$ao_bin" && -x "$REPO_ROOT/cli/bin/ao" ]] && ao_bin="$REPO_ROOT/cli/bin/ao"
[[ -z "$ao_bin" ]] && ao_bin="$(command -v ao 2>/dev/null || true)"
if [[ -z "$ao_bin" ]]; then
    fail "ao not resolvable — build cli/bin/ao or set AO_BIN"
elif "$ao_bin" gate check --fast --scope "$SCOPE"; then
    pass "Shared local gate passed"
else
    fail "Shared local gate failed"
fi
echo ""

if [[ "$SKIP_CLAUDE" != "true" ]]; then
    echo "── Claude CLI ──"
    if command -v claude &>/dev/null; then
        load_output=$(timeout 10 claude --plugin-dir . --help 2>&1) || true
        if echo "$load_output" | grep -qiE "invalid manifest|validation error|failed to load"; then
            fail "Claude CLI load failed"
            echo "$load_output" | grep -iE "invalid|failed|error" | head -3 | sed 's/^/    /'
        else
            pass "Claude CLI loads plugin"
        fi
    else
        warn "Claude CLI not available for load test"
    fi
    echo ""
fi

echo "═══════════════════════════════════════════════════════"
if [[ $errors -gt 0 ]]; then
    echo -e "${RED}  VALIDATION FAILED: $errors errors${NC}"
    echo "═══════════════════════════════════════════════════════"
    exit 1
else
    echo -e "${GREEN}  ALL VALIDATIONS PASSED${NC}"
    echo "═══════════════════════════════════════════════════════"
    exit 0
fi

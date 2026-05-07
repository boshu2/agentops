#!/usr/bin/env bash
# check-pre-push-gate-wired.sh — asserts the pre-push gate chain is intact.
#
# Why: a gate that is not wired into a real git hook is 0% enforcement. This
# script catches three failure modes directly:
#   1. .githooks/pre-push missing or non-executable
#   2. .githooks/pre-push does not invoke scripts/pre-push-gate.sh
#   3. scripts/pre-push-gate.sh missing the baseline-audit block (24d)
#
# It also runs a real `git push` smoke against a sandboxed bare repo to prove
# the hook fires and the remote ref actually moves. The sandbox is opt-in via
# --dry-run-smoke for backward compatibility with existing CI wiring.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

PUSH_SMOKE="false"
CHECK_ACTIVATION="false"

usage() {
    cat <<'EOF'
Usage: scripts/check-pre-push-gate-wired.sh [--dry-run-smoke|--push-smoke] [--check-activation]

Options:
  --dry-run-smoke      Compatibility alias for --push-smoke.
  --push-smoke         Create a sandbox bare remote and run a real `git push`
                       to prove the hook passes refs and updates the remote.
  --check-activation   Also assert this checkout's core.hooksPath == .githooks.
                       Off by default so CI clones (which never run
                       install-dev-hooks.sh) still pass the wiring check.
  -h, --help           Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run-smoke|--push-smoke) PUSH_SMOKE="true"; shift ;;
        --check-activation) CHECK_ACTIVATION="true"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Unknown arg: $1" >&2; usage >&2; exit 2 ;;
    esac
done

errors=0
fail() { echo "FAIL  $1" >&2; errors=$((errors + 1)); }
pass() { echo "  ok  $1"; }

# --- 1. .githooks/pre-push exists and is executable ---
if [[ ! -f .githooks/pre-push ]]; then
    fail ".githooks/pre-push not found"
elif [[ ! -x .githooks/pre-push ]]; then
    fail ".githooks/pre-push not executable (run: chmod +x .githooks/pre-push)"
else
    pass ".githooks/pre-push exists and is executable"
fi

# --- 2. .githooks/pre-push invokes scripts/pre-push-gate.sh ---
if [[ -f .githooks/pre-push ]]; then
    if grep -q 'scripts/pre-push-gate\.sh' .githooks/pre-push; then
        pass ".githooks/pre-push invokes scripts/pre-push-gate.sh"
    else
        fail ".githooks/pre-push does not reference scripts/pre-push-gate.sh"
    fi
fi

# --- 3. scripts/pre-push-gate.sh exists and is executable ---
if [[ ! -f scripts/pre-push-gate.sh ]]; then
    fail "scripts/pre-push-gate.sh not found"
elif [[ ! -x scripts/pre-push-gate.sh ]]; then
    fail "scripts/pre-push-gate.sh not executable"
else
    pass "scripts/pre-push-gate.sh exists and is executable"
fi

# --- 4. baseline-audit block exists in pre-push-gate.sh and exits non-zero ---
if [[ -f scripts/pre-push-gate.sh ]]; then
    if grep -q 'eval baseline-audit' scripts/pre-push-gate.sh; then
        pass "pre-push-gate.sh references eval baseline-audit"
    else
        fail "pre-push-gate.sh missing 'eval baseline-audit' block"
    fi
    if grep -qE 'fail "AgentOps eval baseline-audit' scripts/pre-push-gate.sh; then
        pass "pre-push-gate.sh fails on baseline-audit failure"
    else
        fail "pre-push-gate.sh has no failure path for baseline-audit"
    fi
fi

# --- 5. (optional) operator activation check ---
if [[ "$CHECK_ACTIVATION" == "true" ]]; then
    current="$(git config --local --get core.hooksPath || true)"
    if [[ "$current" == ".githooks" ]]; then
        pass "core.hooksPath = .githooks (operator activated)"
    else
        fail "core.hooksPath = '${current:-<unset>}' (run: bash scripts/install-dev-hooks.sh)"
    fi
fi

# --- 6. (optional) git push smoke against sandbox bare remote ---
if [[ "$PUSH_SMOKE" == "true" ]]; then
    sandbox="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prepush-smoke.XXXXXX")"
    trap 'rm -rf "$sandbox"' EXIT
    bare="$sandbox/remote.git"
    work="$sandbox/work"

    git init --bare --quiet "$bare"
    git -c init.defaultBranch=main init --quiet "$work"
    cd "$work"
    git config user.email smoke@agentops.local
    git config user.name "smoke test"

    # Place tripwires behind the copied hook: the gate exits like a passing
    # advisory run, and the bd shim records the exact refspec stdin it receives.
    stub_dir="$sandbox/stub"
    mkdir -p "$stub_dir/scripts" "$stub_dir/bin"
    cat >"$stub_dir/scripts/pre-push-gate.sh" <<'STUB'
#!/usr/bin/env bash
echo "PRE_PUSH_GATE_INVOKED" >"$SMOKE_TRIPWIRE"
printf '%s\n' "${PRE_PUSH_TWO_PASS:-<unset>}" >"$SMOKE_TWO_PASS"
echo "pre-push gate (fast, advisory): 1 issues found (0 skipped)"
exit 0
STUB
    chmod +x "$stub_dir/scripts/pre-push-gate.sh"
    cat >"$stub_dir/bin/bd" <<'STUB'
#!/usr/bin/env bash
if [[ "${1:-}" == "hooks" && "${2:-}" == "run" && "${3:-}" == "pre-push" ]]; then
    cat >"${BD_STDIN_CAPTURE:?}"
    exit 0
fi
echo "unexpected bd invocation: $*" >&2
exit 2
STUB
    chmod +x "$stub_dir/bin/bd"

    # Override REPO_ROOT in the hook so it reaches the stub.
    cp "$REPO_ROOT/.githooks/pre-push" "$sandbox/pre-push.copy"
    sed -i 's|REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"|REPO_ROOT="'"$stub_dir"'"|' "$sandbox/pre-push.copy"
    chmod +x "$sandbox/pre-push.copy"
    mkdir -p "$work/.githooks_smoke"
    cp "$sandbox/pre-push.copy" "$work/.githooks_smoke/pre-push"
    git config core.hooksPath "$work/.githooks_smoke"

    tripwire="$sandbox/tripwire"
    two_pass="$sandbox/two-pass"
    bd_stdin="$sandbox/bd-stdin"
    : >"$tripwire"

    git remote add origin "$bare"
    echo "smoke" >file
    git add file
    git commit --quiet -m "smoke commit"
    local_head="$(git rev-parse HEAD)"

    if SMOKE_TRIPWIRE="$tripwire" \
        SMOKE_TWO_PASS="$two_pass" \
        BD_STDIN_CAPTURE="$bd_stdin" \
        PATH="$stub_dir/bin:$PATH" \
        git push origin main >/dev/null 2>&1; then
        :
    else
        fail "git push failed through pre-push hook (sandbox smoke)"
    fi

    if [[ -s "$tripwire" ]] && grep -q PRE_PUSH_GATE_INVOKED "$tripwire"; then
        pass "git push invokes pre-push gate (sandbox smoke)"
    else
        fail "git push did NOT invoke pre-push gate (sandbox smoke)"
    fi
    if [[ "$(cat "$two_pass" 2>/dev/null || true)" == "0" ]]; then
        pass "pre-push hook forces single-pass push gate"
    else
        fail "pre-push hook did NOT force single-pass push gate"
    fi
    if [[ -s "$bd_stdin" ]] && grep -q 'refs/heads/main' "$bd_stdin"; then
        pass "pre-push hook replays refspec stdin to bd"
    else
        fail "pre-push hook did NOT replay refspec stdin to bd"
    fi
    remote_head="$(git --git-dir "$bare" rev-parse refs/heads/main 2>/dev/null || true)"
    if [[ "$remote_head" == "$local_head" ]]; then
        pass "git push transmits through pre-push hook (sandbox smoke)"
    else
        fail "git push did NOT update sandbox remote ref"
    fi
    cd "$REPO_ROOT"
fi

if [[ $errors -gt 0 ]]; then
    echo ""
    echo "pre-push gate wiring: BLOCKED ($errors failures)"
    exit 1
fi

echo ""
echo "pre-push gate wiring: ok"
exit 0

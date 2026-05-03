#!/usr/bin/env bash
# check-pre-push-gate-wired.sh — asserts the pre-push gate chain is intact.
#
# Why: a gate that is not wired into a real git hook is 0% enforcement. This
# script catches three failure modes directly:
#   1. .githooks/pre-push missing or non-executable
#   2. .githooks/pre-push does not invoke scripts/pre-push-gate.sh
#   3. scripts/pre-push-gate.sh missing the baseline-audit block (24d)
#
# It also runs a real `git push --dry-run` smoke against a sandboxed bare repo
# to prove the gate actually fires from git's hook resolution path. The sandbox
# is opt-in via --dry-run-smoke because cold runs cost ~10-30s.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

DRY_RUN_SMOKE="false"
CHECK_ACTIVATION="false"

usage() {
    cat <<'EOF'
Usage: scripts/check-pre-push-gate-wired.sh [--dry-run-smoke] [--check-activation]

Options:
  --dry-run-smoke      Create a sandbox bare remote and run `git push --dry-run`
                       to prove the gate fires through git's hook resolution.
  --check-activation   Also assert this checkout's core.hooksPath == .githooks.
                       Off by default so CI clones (which never run
                       install-dev-hooks.sh) still pass the wiring check.
  -h, --help           Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run-smoke) DRY_RUN_SMOKE="true"; shift ;;
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

# --- 6. (optional) git push --dry-run smoke against sandbox bare remote ---
if [[ "$DRY_RUN_SMOKE" == "true" ]]; then
    sandbox="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prepush-smoke.XXXXXX")"
    trap 'rm -rf "$sandbox"' EXIT
    bare="$sandbox/remote.git"
    work="$sandbox/work"

    git init --bare --quiet "$bare"
    git -c init.defaultBranch=main init --quiet "$work"
    cd "$work"
    git config user.email smoke@agentops.local
    git config user.name "smoke test"
    git config core.hooksPath "$REPO_ROOT/.githooks"

    # Place a tripwire that pre-push-gate.sh can detect — we don't actually want
    # to run the full ~5min gate here. The hook chain calls
    # pre-push-gate.sh; we override $PATH so the gate sees a stub that records
    # invocation and exits 0.
    stub_dir="$sandbox/stub"
    mkdir -p "$stub_dir/scripts"
    cat >"$stub_dir/scripts/pre-push-gate.sh" <<'STUB'
#!/usr/bin/env bash
echo "PRE_PUSH_GATE_INVOKED" >"$SMOKE_TRIPWIRE"
exit 0
STUB
    chmod +x "$stub_dir/scripts/pre-push-gate.sh"

    # Override REPO_ROOT in the hook so it reaches the stub.
    cp "$REPO_ROOT/.githooks/pre-push" "$sandbox/pre-push.copy"
    sed -i 's|REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"|REPO_ROOT="'"$stub_dir"'"|' "$sandbox/pre-push.copy"
    chmod +x "$sandbox/pre-push.copy"
    mkdir -p "$work/.githooks_smoke"
    cp "$sandbox/pre-push.copy" "$work/.githooks_smoke/pre-push"
    git config core.hooksPath "$work/.githooks_smoke"

    tripwire="$sandbox/tripwire"
    : >"$tripwire"

    git remote add origin "$bare"
    echo "smoke" >file
    git add file
    git commit --quiet -m "smoke commit"

    SMOKE_TRIPWIRE="$tripwire" git push --dry-run origin main >/dev/null 2>&1 || true

    if [[ -s "$tripwire" ]] && grep -q PRE_PUSH_GATE_INVOKED "$tripwire"; then
        pass "git push --dry-run invokes pre-push gate (sandbox smoke)"
    else
        fail "git push --dry-run did NOT invoke pre-push gate (sandbox smoke)"
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

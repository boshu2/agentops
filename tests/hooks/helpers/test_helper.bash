#!/usr/bin/env bash
# test_helper.bash — Shared setup/teardown for hook bats tests
#
# Provides:
#   - REPO_ROOT, HOOKS_DIR globals
#   - TMP_TEST_DIR: per-test temp directory (auto-cleaned)
#   - MOCK_REPO: a git-initialized mock repo with lib/ helpers copied in
#   - setup_mock_repo DIR: creates a git-initialized repo at DIR with lib/ copied
#   - run_hook HOOK_SCRIPT JSON_STRING [env assignments ...]: pipes JSON to hook

# ── Common setup ─────────────────────────────────────────────────────
_helper_setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HOOKS_DIR="$REPO_ROOT/hooks"
    export REPO_ROOT HOOKS_DIR

    # Per-test temp directory
    TMP_TEST_DIR="$(mktemp -d)"

    # Default mock repo
    MOCK_REPO="$TMP_TEST_DIR/mock-repo"
    setup_mock_repo "$MOCK_REPO"

    # ── Hook lifecycle isolation (soc-y1bk) ───────────────────────────
    # Three load-bearing isolations so a hook script invoked via `run bash $hook`
    # (without an explicit `cd $MOCK_REPO`) cannot reach into the real repo
    # under -shuffle / parallel BATS runs:
    #
    #   1. _SAVED_PWD — preserve caller cwd so teardown can restore it.
    #   2. cd "$MOCK_REPO" — `git rev-parse --show-toplevel` inside any hook
    #      now resolves to the tempdir mock, NOT $REPO_ROOT (the real repo).
    #      This prevents the .agents/ao/.intent-echo-fired family of dedup
    #      flags from being created/cleaned in the operator's working tree.
    #   3. HOME=$TMP_TEST_DIR — defense-in-depth: any helper that falls back
    #      to $HOME (instead of git toplevel) lands inside the tempdir too.
    _SAVED_PWD="$(pwd)"
    cd "$MOCK_REPO" || return 1
    export HOME="$TMP_TEST_DIR"

    # Ensure hooks are NOT globally disabled by default
    export AGENTOPS_HOOKS_DISABLED=0
}

# ── Common teardown ──────────────────────────────────────────────────
_helper_teardown() {
    # Restore caller cwd before teardown removes the tmpdir.
    cd "${_SAVED_PWD:-$REPO_ROOT}" 2>/dev/null || true
    rm -rf "$TMP_TEST_DIR"
    # Belt-and-suspenders cleanup of dedup flags. With the setup-time
    # cd "$MOCK_REPO" + HOME=$TMP_TEST_DIR isolation above, hooks should
    # never write into $REPO_ROOT/.agents/ao/ in the first place; this
    # remains as a safety net for legacy tests that bypass the helper.
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    rm -f "$REPO_ROOT/.agents/ao/.new-user-welcome-needed" 2>/dev/null
    rm -f "$REPO_ROOT/.agents/ao/.read-streak" 2>/dev/null
    rm -f "$REPO_ROOT/.agents/ao/.ratchet-advance-fired" 2>/dev/null
}

# ── Helpers ──────────────────────────────────────────────────────────

# setup_mock_repo DIR
#   Creates a git-initialized repo at DIR with lib/ helpers copied in.
setup_mock_repo() {
    local dir="$1"
    mkdir -p "$dir/.agents/ao" "$dir/lib"
    git -C "$dir" init -q >/dev/null 2>&1
    [ -f "$REPO_ROOT/lib/hook-helpers.sh" ] && /bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$dir/lib/hook-helpers.sh"
    [ -f "$REPO_ROOT/lib/chain-parser.sh" ] && /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$dir/lib/chain-parser.sh"
}

# run_hook HOOK_SCRIPT JSON_STRING
#   Pipes JSON_STRING to HOOK_SCRIPT via stdin, captures stdout+stderr.
#   After call, $status and $output are set (like bats `run`).
run_hook() {
    local hook="$1"
    local json="$2"
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$json" "$hook"
}

#!/usr/bin/env bats
# Tests for scripts/pre-push-gate.sh step 30 (ShellCheck) — verifies the
# unconditional staged-shell shellcheck pass.
#
# History: 2026-05-18 merge-arc post-mortem F1. PR #322 left an unused
# REPO_ROOT in a rewritten script; shellcheck SC2034 surfaced only on the
# NEXT cycle's pre-push because the gate's `needs_check shell` and inner
# `--fast` filter both depend on diff-based HAS_SHELL detection — when
# `all_changed` is computed against the wrong base, staged .sh files can
# slip through. The fix routes through `git diff --name-only --cached`
# and `git diff --name-only` directly so staged/working-tree .sh files
# always get shellchecked in fast mode.
#
# See: .agents/learnings/2026-05-18-script-rewrites-leave-dead-variables.md

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GATE="$ROOT/scripts/pre-push-gate.sh"
    [ -f "$GATE" ] || skip "gate not found: $GATE"
}

@test "step 30 invokes shellcheck without HAS_SHELL gating" {
    # Structural check: the step 30 block must not be wrapped in
    # `if needs_check shell; then` — the staged-shell pass is unconditional
    # in fast mode now. Grep the source.
    sec=$(awk '
        /^# --- 30\. ShellCheck/ { in_sec=1 }
        in_sec && /^# --- 31\./ { in_sec=0 }
        in_sec { print }
    ' "$GATE")

    if echo "$sec" | grep -qE '^if needs_check shell'; then
        echo "Step 30 still gated by needs_check shell — diff-detection bypass missing"
        echo "----"
        echo "$sec" | head -10
        return 1
    fi
}

@test "step 30 collects staged .sh via git diff --cached" {
    # The new pass must consult the staged set directly, not rely on
    # all_changed (which has historically missed files).
    grep -A 40 '^# --- 30\. ShellCheck' "$GATE" \
        | grep -qE 'git diff --name-only --cached.*grep .*\.sh'
}

@test "step 30 collects working-tree .sh via git diff --name-only" {
    grep -A 40 '^# --- 30\. ShellCheck' "$GATE" \
        | grep -qE 'git diff --name-only.*grep .*\.sh'
}

@test "step 30 still uses shellcheck -S warning" {
    # Severity threshold preserved (SC2034 is warning-level; lower thresholds
    # would mask it).
    grep -A 50 '^# --- 30\. ShellCheck' "$GATE" \
        | grep -qE 'shellcheck -S warning'
}

@test "step 30 still skips gracefully when shellcheck is missing" {
    grep -A 60 '^# --- 30\. ShellCheck' "$GATE" \
        | grep -qE 'skip "shellcheck \(not installed\)"'
}

@test "post-mortem learning anchor reference is in the script comment" {
    # The change is anchored to a durable lesson; verify the rationale link
    # remains in the script comment header. The actual learning file lives
    # in .agents/ (gitignored, local-only), so a file-existence check would
    # break in CI's fresh clone. Asserting the reference in the script body
    # instead keeps the rationale traceable without depending on local state.
    grep -q '2026-05-18-script-rewrites-leave-dead-variables' "$GATE"
}

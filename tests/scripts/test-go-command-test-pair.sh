#!/usr/bin/env bash
set -euo pipefail

# test-go-command-test-pair.sh
# Integration tests for scripts/check-go-command-test-pair.sh.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHECK_SCRIPT="$REPO_ROOT/scripts/check-go-command-test-pair.sh"

if [[ ! -x "$CHECK_SCRIPT" ]]; then
    echo "FAIL: missing executable script: $CHECK_SCRIPT" >&2
    exit 1
fi

PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

new_repo() {
    local dir="$1"
    local remote="$2"
    git init -q --bare "$remote"
    git init -q "$dir"
    git -C "$dir" config user.name "Test Bot"
    git -C "$dir" config user.email "test@example.com"
    git -C "$dir" remote add origin "$remote"

    mkdir -p "$dir/cli/cmd/ao"
    cat > "$dir/cli/cmd/ao/sample.go" <<'EOF'
package main

func sample() string { return "ok" }
EOF
    cat > "$dir/cli/cmd/ao/sample_test.go" <<'EOF'
package main

import "testing"

func TestSample(t *testing.T) {
    if sample() != "ok" {
        t.Fatal("bad sample")
    }
}
EOF
    echo "baseline" > "$dir/README.md"

    git -C "$dir" add .
    git -C "$dir" commit -q -m "baseline"
    git -C "$dir" push -q -u origin HEAD
}

run_case_fail_command_only() {
    local dir="$1"
    echo "// changed" >> "$dir/cli/cmd/ao/sample.go"
    git -C "$dir" add cli/cmd/ao/sample.go
    git -C "$dir" commit -q -m "command only change"

    if (cd "$dir" && "$CHECK_SCRIPT" >/dev/null 2>&1); then
        fail "command-only change should fail pairing gate"
    else
        pass "command-only change fails pairing gate"
    fi
}

run_case_pass_with_test() {
    local dir="$1"
    echo "// changed" >> "$dir/cli/cmd/ao/sample.go"
    echo "// changed" >> "$dir/cli/cmd/ao/sample_test.go"
    git -C "$dir" add cli/cmd/ao/sample.go cli/cmd/ao/sample_test.go
    git -C "$dir" commit -q -m "command + test change"

    if (cd "$dir" && "$CHECK_SCRIPT" >/dev/null 2>&1); then
        pass "command + test change passes pairing gate"
    else
        fail "command + test change should pass pairing gate"
    fi
}

run_case_skip_non_command() {
    local dir="$1"
    echo "docs changed" >> "$dir/README.md"
    git -C "$dir" add README.md
    git -C "$dir" commit -q -m "non command change"

    if (cd "$dir" && "$CHECK_SCRIPT" >/dev/null 2>&1); then
        pass "non command change passes pairing gate"
    else
        fail "non command change should pass pairing gate"
    fi
}

# new_train_repo builds a c1+c2 train (command in c1, its test in c2) with NO
# upstream — the detached landing-worktree shape.
new_train_repo() {
    local dir="$1"
    git init -q "$dir"
    git -C "$dir" config user.name "Test Bot"
    git -C "$dir" config user.email "test@example.com"
    echo "baseline" > "$dir/README.md"
    git -C "$dir" add .
    git -C "$dir" commit -q -m "baseline"

    mkdir -p "$dir/cli/cmd/ao"
    cat > "$dir/cli/cmd/ao/foo.go" <<'EOF'
package main

func foo() {}
EOF
    git -C "$dir" add cli/cmd/ao/foo.go
    git -C "$dir" commit -q -m "add command foo"

    cat > "$dir/cli/cmd/ao/foo_test.go" <<'EOF'
package main

import "testing"

func TestFoo(t *testing.T) {}
EOF
    git -C "$dir" add cli/cmd/ao/foo_test.go
    git -C "$dir" commit -q -m "add foo test"
}

# The landing-loop regression (age-wy2t): a detached worktree parked at the
# command-only commit c1 has no upstream, so the default collector sees only c1
# and fails the pairing gate; an explicit AGENTOPS_GATE_RANGE spanning c1+c2
# brings the test into scope and passes.
run_case_range_prefers_env() {
    local dir="$1"
    local base head
    base="$(git -C "$dir" rev-parse HEAD~2)" # c0 baseline
    head="$(git -C "$dir" rev-parse HEAD)"    # c2 tip
    git -C "$dir" checkout -q HEAD~1          # detach at c1 (command only)

    if (cd "$dir" && "$CHECK_SCRIPT" >/dev/null 2>&1); then
        fail "detached c1 without range should fail pairing gate"
    else
        pass "detached c1 without range fails pairing gate (git-show fallback)"
    fi

    if (cd "$dir" && AGENTOPS_GATE_RANGE="$base..$head" "$CHECK_SCRIPT" >/dev/null 2>&1); then
        pass "AGENTOPS_GATE_RANGE spanning the train passes pairing gate"
    else
        fail "AGENTOPS_GATE_RANGE spanning the train should pass pairing gate"
    fi
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

echo "== check-go-command-test-pair.sh =="

new_repo "$TMP_ROOT/case_fail" "$TMP_ROOT/remote_fail.git"
run_case_fail_command_only "$TMP_ROOT/case_fail"

new_repo "$TMP_ROOT/case_pass" "$TMP_ROOT/remote_pass.git"
run_case_pass_with_test "$TMP_ROOT/case_pass"

new_repo "$TMP_ROOT/case_skip" "$TMP_ROOT/remote_skip.git"
run_case_skip_non_command "$TMP_ROOT/case_skip"

new_train_repo "$TMP_ROOT/case_range"
run_case_range_prefers_env "$TMP_ROOT/case_range"

echo ""
if [[ "$FAIL" -gt 0 ]]; then
    echo "FAILED - $FAIL failure(s), $PASS pass(es)"
    exit 1
fi

echo "PASSED - $PASS pass(es)"
exit 0

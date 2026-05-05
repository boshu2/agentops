#!/usr/bin/env bats
#
# Tests for scripts/check-flywheel-lifecycle.sh — locks in the sparse-corpus
# regression fix where Stage 5's `find | grep -v` pipeline crashed under
# `set -euo pipefail` whenever the only learning file was the test sentinel.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-flywheel-lifecycle.sh"

    TMP_DIR="$(mktemp -d)"
    AGENTS_DIR="$TMP_DIR/.agents"
    mkdir -p "$AGENTS_DIR/learnings"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "PASS on sparse corpus (no real learnings — only test sentinel)" {
    # Regression: previously, `find ... | grep -v "$(basename ...)"` exited 1
    # when grep filtered all input, killing the script under set -euo pipefail
    # before the soft-fail NOTE branch could run.
    run env AGENTS_DIR="$AGENTS_DIR" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: Flywheel lifecycle gate OK"* ]]
    [[ "$output" == *"Corpus too sparse for citation checks"* ]]
}

@test "PASS with populated corpus (real learnings present)" {
    cat > "$AGENTS_DIR/learnings/real-learning.md" <<'EOF'
---
title: A real learning
category: testing
---
some content
EOF
    run env AGENTS_DIR="$AGENTS_DIR" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: Flywheel lifecycle gate OK"* ]]
    [[ "$output" == *"corpus has 1 learning(s)"* ]]
}

@test "PASS with cross-citing corpus surfaces citation count" {
    cat > "$AGENTS_DIR/learnings/a.md" <<'EOF'
---
title: A
---
see also: [b](b.md)
EOF
    cat > "$AGENTS_DIR/learnings/b.md" <<'EOF'
---
title: B
---
related: a
EOF
    run env AGENTS_DIR="$AGENTS_DIR" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"learnings contain cross-citations"* ]]
}

@test "test sentinel is cleaned up after the run" {
    run env AGENTS_DIR="$AGENTS_DIR" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [ ! -f "$AGENTS_DIR/learnings/test-flywheel-lifecycle-gate.md" ]
}

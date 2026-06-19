#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/hooks/pre-push.local"
}

@test "pre-push.local full race gate uses randomized shuffle" {
    [ -x "$SCRIPT" ]
    run grep -q 'go test ./... -race -shuffle=on -count=1' "$SCRIPT"
    [ "$status" -eq 0 ]

    run grep -q 'go test ./... -race -shuffle=1 -count=1' "$SCRIPT"
    [ "$status" -eq 1 ]
}

@test "pre-push.local scrubs git hook discovery env before full race gate" {
    run grep -q 'unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE' "$SCRIPT"
    [ "$status" -eq 0 ]
}

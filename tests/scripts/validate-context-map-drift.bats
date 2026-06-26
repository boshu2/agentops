#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-context-map-drift.sh"
    TMP="$(mktemp -d)"
    export CONTEXT_MAP="$TMP/context-map.md"
    export GENERATOR="$TMP/generate-context-map.sh"
}

teardown() {
    rm -rf "$TMP"
}

make_generator() {
    local content="$1"
    cat >"$GENERATOR" <<EOF
#!/usr/bin/env bash
printf '%s\n' '$content' >"\$CONTEXT_MAP"
EOF
    chmod +x "$GENERATOR"
}

@test "passes when current working copy already matches generator output" {
    printf '%s\n' "generated" >"$CONTEXT_MAP"
    make_generator "generated"

    run bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [ "$(cat "$CONTEXT_MAP")" = "generated" ]
}

@test "fails on drift and restores the original working copy" {
    printf '%s\n' "stale" >"$CONTEXT_MAP"
    make_generator "generated"

    run bash "$SCRIPT"

    [ "$status" -eq 1 ]
    [[ "$output" == *"Context map drift detected"* ]]
    [ "$(cat "$CONTEXT_MAP")" = "stale" ]
}

@test "fails when matching generated context map is not committed" {
    repo="$TMP/repo"
    mkdir -p "$repo"
    git -C "$repo" init -q
    printf '%s\n' "committed" >"$repo/context-map.md"
    git -C "$repo" add context-map.md
    git -C "$repo" -c user.name=test -c user.email=test@example.com commit -q -m init
    printf '%s\n' "generated" >"$repo/context-map.md"

    cat >"$repo/generate-context-map.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' generated >"$CONTEXT_MAP"
EOF
    chmod +x "$repo/generate-context-map.sh"

    run bash -c "cd '$repo' && CONTEXT_MAP=context-map.md GENERATOR=./generate-context-map.sh '$SCRIPT'"

    [ "$status" -eq 1 ]
    [[ "$output" == *"matches the generator but has uncommitted changes"* ]]
    [ "$(cat "$repo/context-map.md")" = "generated" ]
}

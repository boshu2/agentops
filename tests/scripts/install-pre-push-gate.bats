#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/install-pre-push-gate.sh"
}

@test "installed pre-push wrapper replays stdin after a prior hook consumes it" {
    repo="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$repo"
    git -C "$repo" init -q
    git -C "$repo" config user.email test@example.com
    git -C "$repo" config user.name Test
    echo init > "$repo/README.md"
    git -C "$repo" add README.md
    git -C "$repo" commit -m initial >/dev/null

    hook="$repo/.git/hooks/pre-push"
    cat > "$hook" <<'EOS'
#!/usr/bin/env sh
# --- BEGIN BEADS INTEGRATION v1.0.5 ---
cat > "$BEADS_LOG"
# --- END BEADS INTEGRATION v1.0.5 ---
EOS
    chmod +x "$hook"

    run bash -c 'cd "$1" && "$2"' _ "$repo" "$SCRIPT"
    [ "$status" -eq 0 ]

    cat > "$repo/.git/hooks/pre-push.local" <<'EOS'
#!/usr/bin/env sh
cat > "$AGENTOPS_LOG"
EOS
    chmod +x "$repo/.git/hooks/pre-push.local"

    push_record='refs/heads/main abc refs/heads/main def'
    run env BEADS_LOG="$BATS_TEST_TMPDIR/beads.log" \
        AGENTOPS_LOG="$BATS_TEST_TMPDIR/agentops.log" \
        sh -c 'cd "$1" && printf "%s\n" "$2" | "$3"' _ "$repo" "$push_record" "$hook"
    [ "$status" -eq 0 ]

    [ "$(cat "$BATS_TEST_TMPDIR/beads.log")" = "$push_record" ]
    [ "$(cat "$BATS_TEST_TMPDIR/agentops.log")" = "$push_record" ]
}

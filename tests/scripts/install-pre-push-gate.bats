#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/install-pre-push-gate.sh"
}

write_fixture_hook() {
    repo="$1"
    marker="$2"
    mkdir -p "$repo/scripts/hooks"
    cat > "$repo/scripts/hooks/pre-push.local" <<EOS
#!/usr/bin/env sh
printf '%s\n' '$marker' >> "\$AGENTOPS_LOG"
EOS
    chmod +x "$repo/scripts/hooks/pre-push.local"
}

init_hook_repo() {
    repo="$1"
    marker="$2"
    mkdir -p "$repo"
    git -C "$repo" init -q
    git -C "$repo" config user.email test@example.com
    git -C "$repo" config user.name Test
    write_fixture_hook "$repo" "$marker"
    git -C "$repo" add scripts/hooks/pre-push.local
    git -C "$repo" commit -m "hook $marker" >/dev/null
    git -C "$repo" update-ref refs/remotes/origin/main HEAD
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

@test "installed wrapper preserves an explicitly installed fast-forward pushed hook" {
    repo="$BATS_TEST_TMPDIR/repo"
    log="$BATS_TEST_TMPDIR/agentops.log"
    init_hook_repo "$repo" trunk
    old_sha="$(git -C "$repo" rev-parse origin/main)"

    write_fixture_hook "$repo" candidate
    git -C "$repo" add scripts/hooks/pre-push.local
    git -C "$repo" commit -m "hook candidate" >/dev/null
    new_sha="$(git -C "$repo" rev-parse HEAD)"

    run bash -c 'cd "$1" && "$2"' _ "$repo" "$SCRIPT"
    [ "$status" -eq 0 ]

    cp "$repo/scripts/hooks/pre-push.local" "$repo/.git/hooks/pre-push.local"
    chmod +x "$repo/.git/hooks/pre-push.local"

    run env AGENTOPS_LOG="$log" \
        sh -c 'cd "$1" && printf "%s %s %s %s\n" refs/heads/main "$2" refs/heads/main "$3" | "$4"' \
        _ "$repo" "$new_sha" "$old_sha" "$repo/.git/hooks/pre-push"
    [ "$status" -eq 0 ]

    [ "$(cat "$log")" = "candidate" ]
    run grep -q "candidate" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 0 ]
    run grep -q "trunk" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 1 ]
}

@test "installed wrapper heals stale branch hook back to trunk" {
    repo="$BATS_TEST_TMPDIR/repo"
    log="$BATS_TEST_TMPDIR/agentops.log"
    init_hook_repo "$repo" stale
    stale_sha="$(git -C "$repo" rev-parse HEAD)"

    write_fixture_hook "$repo" trunk
    git -C "$repo" add scripts/hooks/pre-push.local
    git -C "$repo" commit -m "hook trunk" >/dev/null
    trunk_sha="$(git -C "$repo" rev-parse HEAD)"
    git -C "$repo" update-ref refs/remotes/origin/main HEAD
    git -C "$repo" checkout -q -b stale-branch "$stale_sha"

    run bash -c 'cd "$1" && "$2"' _ "$repo" "$SCRIPT"
    [ "$status" -eq 0 ]

    cp "$repo/scripts/hooks/pre-push.local" "$repo/.git/hooks/pre-push.local"
    chmod +x "$repo/.git/hooks/pre-push.local"

    run env AGENTOPS_LOG="$log" \
        sh -c 'cd "$1" && printf "%s %s %s %s\n" refs/heads/main "$2" refs/heads/main "$3" | "$4"' \
        _ "$repo" "$stale_sha" "$trunk_sha" "$repo/.git/hooks/pre-push"
    [ "$status" -eq 0 ]

    [ "$(cat "$log")" = "trunk" ]
    run grep -q "trunk" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 0 ]
}

@test "installed wrapper ignores fast-forward pushed hook unless it was explicitly installed" {
    repo="$BATS_TEST_TMPDIR/repo"
    log="$BATS_TEST_TMPDIR/agentops.log"
    init_hook_repo "$repo" trunk
    old_sha="$(git -C "$repo" rev-parse origin/main)"
    trunk_hook="$BATS_TEST_TMPDIR/trunk-pre-push.local"
    cp "$repo/scripts/hooks/pre-push.local" "$trunk_hook"

    write_fixture_hook "$repo" candidate
    git -C "$repo" add scripts/hooks/pre-push.local
    git -C "$repo" commit -m "hook candidate" >/dev/null
    new_sha="$(git -C "$repo" rev-parse HEAD)"

    run bash -c 'cd "$1" && "$2"' _ "$repo" "$SCRIPT"
    [ "$status" -eq 0 ]

    cp "$trunk_hook" "$repo/.git/hooks/pre-push.local"
    chmod +x "$repo/.git/hooks/pre-push.local"

    run env AGENTOPS_LOG="$log" \
        sh -c 'cd "$1" && printf "%s %s %s %s\n" refs/heads/main "$2" refs/heads/main "$3" | "$4"' \
        _ "$repo" "$new_sha" "$old_sha" "$repo/.git/hooks/pre-push"
    [ "$status" -eq 0 ]

    [ "$(cat "$log")" = "trunk" ]
    run grep -q "trunk" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 0 ]
    run grep -q "candidate" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 1 ]
}

@test "installed wrapper heals candidate hook on non-main branch pushes" {
    repo="$BATS_TEST_TMPDIR/repo"
    log="$BATS_TEST_TMPDIR/agentops.log"
    init_hook_repo "$repo" trunk
    old_sha="$(git -C "$repo" rev-parse origin/main)"

    git -C "$repo" checkout -q -b feature
    write_fixture_hook "$repo" candidate
    git -C "$repo" add scripts/hooks/pre-push.local
    git -C "$repo" commit -m "hook candidate" >/dev/null
    new_sha="$(git -C "$repo" rev-parse HEAD)"

    run bash -c 'cd "$1" && "$2"' _ "$repo" "$SCRIPT"
    [ "$status" -eq 0 ]

    cp "$repo/scripts/hooks/pre-push.local" "$repo/.git/hooks/pre-push.local"
    chmod +x "$repo/.git/hooks/pre-push.local"

    run env AGENTOPS_LOG="$log" \
        sh -c 'cd "$1" && printf "%s %s %s %s\n" refs/heads/feature "$2" refs/heads/feature "$3" | "$4"' \
        _ "$repo" "$new_sha" "$old_sha" "$repo/.git/hooks/pre-push"
    [ "$status" -eq 0 ]

    [ "$(cat "$log")" = "trunk" ]
    run grep -q "trunk" "$repo/.git/hooks/pre-push.local"
    [ "$status" -eq 0 ]
}

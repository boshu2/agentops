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

@test "pre-push.local builds a per-run ao gate binary" {
    run grep -q 'mktemp .*ao-gate' "$SCRIPT"
    [ "$status" -eq 0 ]

    run grep -q 'go build -o /tmp/ao-gate ./cmd/ao' "$SCRIPT"
    [ "$status" -eq 1 ]
}

@test "pre-push.local runs full race before serial mutable lock" {
    race_line="$(grep -n 'go test ./... -race -shuffle=on -count=1' "$SCRIPT" | tail -1 | cut -d: -f1)"
    lock_line="$(grep -n '^acquire_push_lock$' "$SCRIPT" | tail -1 | cut -d: -f1)"

    [ -n "$race_line" ]
    [ -n "$lock_line" ]
    [ "$race_line" -lt "$lock_line" ]
}

@test "pre-push.local runs cmd/ao integration shard before serial mutable lock" {
    run grep -Fq 'go test ./cmd/ao -tags=integration -run "$cmdao_integration_tests" -race -shuffle=on -count=1' "$SCRIPT"
    [ "$status" -eq 0 ]

    shard_line="$(grep -Fn 'go test ./cmd/ao -tags=integration' "$SCRIPT" | tail -1 | cut -d: -f1)"
    lock_line="$(grep -n '^acquire_push_lock$' "$SCRIPT" | tail -1 | cut -d: -f1)"

    [ -n "$shard_line" ]
    [ -n "$lock_line" ]
    [ "$shard_line" -lt "$lock_line" ]
}

@test "pre-push.local makes post-land provenance opt-in inside pre-push" {
    run grep -q 'AGENTOPS_PROVENANCE_EMIT_POST_LAND:-0' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "pre-push.local default path does not mutate head or provenance ledger" {
    repo="$BATS_TEST_TMPDIR/repo"
    stubbin="$BATS_TEST_TMPDIR/bin"
    tmpdir="$BATS_TEST_TMPDIR/tmp"
    log="$BATS_TEST_TMPDIR/hook.log"
    mkdir -p "$repo/cli" "$repo/scripts" "$repo/docs/provenance" "$stubbin" "$tmpdir"

    cat >"$stubbin/go" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "build" && "${2:-}" == "-o" ]]; then
    out="$3"
    cat >"$out" <<'AO'
#!/usr/bin/env bash
echo "ao:$*" >> "$AGENTOPS_TEST_LOG"
exit 0
AO
    chmod +x "$out"
    exit 0
fi
if [[ "${1:-}" == "build" ]]; then
    exit 0
fi
if [[ "${1:-}" == "test" ]]; then
    echo "go:$*" >> "$AGENTOPS_TEST_LOG"
    exit 0
fi
exit 0
EOS
    chmod +x "$stubbin/go"

    cat >"$repo/scripts/post-land-provenance-emit.sh" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
echo "post-land" >> "$AGENTOPS_TEST_LOG"
echo "ledger-row" >> docs/provenance/ledger.jsonl
git add docs/provenance/ledger.jsonl
git commit -m "post-land mutator" >/dev/null 2>&1
EOS
    chmod +x "$repo/scripts/post-land-provenance-emit.sh"

    cat >"$repo/scripts/check-pawl-pre-push.sh" <<'EOS'
#!/usr/bin/env bash
echo "pawl" >> "$AGENTOPS_TEST_LOG"
exit 0
EOS
    chmod +x "$repo/scripts/check-pawl-pre-push.sh"

    touch "$repo/docs/provenance/ledger.jsonl"
    git -C "$repo" init -q
    git -C "$repo" config user.email test@example.com
    git -C "$repo" config user.name Test
    git -C "$repo" add docs/provenance/ledger.jsonl
    git -C "$repo" commit -m initial >/dev/null
    head_before="$(git -C "$repo" rev-parse HEAD)"
    ledger_before="$(git -C "$repo" hash-object docs/provenance/ledger.jsonl)"

    run env PATH="$stubbin:$PATH" \
        TMPDIR="$tmpdir" \
        AGENTOPS_TEST_LOG="$log" \
        AGENTOPS_PREPUSH_SKIP_FULL_RACE=1 \
        sh -c 'cd "$1" && printf "%s %s %s %s\n" refs/heads/main "$2" refs/heads/main 0000000000000000000000000000000000000000 | "$3"' \
        _ "$repo" "$head_before" "$SCRIPT"
    [ "$status" -eq 0 ]

    [ "$(git -C "$repo" rev-parse HEAD)" = "$head_before" ]
    [ "$(git -C "$repo" hash-object docs/provenance/ledger.jsonl)" = "$ledger_before" ]
    git -C "$repo" diff --quiet
    git -C "$repo" diff --cached --quiet
    run grep -q '^post-land$' "$log"
    [ "$status" -eq 1 ]
    run grep -q '^ao:gate check --fast$' "$log"
    [ "$status" -eq 0 ]
    run grep -q '^pawl$' "$log"
    [ "$status" -eq 0 ]
}

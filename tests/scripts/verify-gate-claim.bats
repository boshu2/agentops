#!/usr/bin/env bats

# verify-gate-claim.bats — coverage for scripts/verify-gate-claim.sh
#
# Tests cover the four exit-code contracts named in the script's --help:
#   0 verified, 1 AP#7 violation, 2 usage error, 3 verification impossible.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/verify-gate-claim.sh"
    SCRATCH="$(mktemp -d -t verify-gate-claim-bats.XXXXXX)"
}

teardown() {
    if [[ -n "${SCRATCH:-}" && -d "$SCRATCH" ]]; then
        rm -rf "$SCRATCH"
    fi
}

# ---- usage errors (exit 2) ----

@test "no args: exits 2 and prints usage" {
    run "$SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"Usage: scripts/verify-gate-claim.sh"* ]]
}

@test "only ref, no claim: exits 2" {
    run "$SCRIPT" main
    [ "$status" -eq 2 ]
    [[ "$output" == *"missing required arguments"* ]]
}

@test "unknown flag: exits 2" {
    run "$SCRIPT" --bogus main "claim"
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown flag"* ]]
}

@test "--gate and --log together: exits 2" {
    run "$SCRIPT" --gate "true" --log /dev/null main "claim"
    [ "$status" -eq 2 ]
    [[ "$output" == *"mutually exclusive"* ]]
}

@test "--help: exits 0 and prints usage" {
    run "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Mechanical enforcement of ship-loop anti-pattern #7"* ]]
}

# ---- passive mode (--log) ----

@test "passive: claim present in log returns 0" {
    printf 'gate output line one\nclaim-marker-XYZ here\nfinal line\n' >"$SCRATCH/log"
    run "$SCRIPT" --log "$SCRATCH/log" main "claim-marker-XYZ"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"log:$SCRATCH/log"* ]]
}

@test "passive: claim absent in log returns 1 (AP#7 violation)" {
    printf 'gate output\nno match here\n' >"$SCRATCH/log"
    run "$SCRIPT" --log "$SCRATCH/log" main "claim-marker-MISSING"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"claim-marker-MISSING"* ]]
}

@test "passive: missing log file returns 3" {
    run "$SCRIPT" --log "$SCRATCH/does-not-exist" main "anything"
    [ "$status" -eq 3 ]
    [[ "$output" == *"log file not found"* ]]
}

# ---- active mode (--gate) ----

@test "active: --gate with claim-matching output returns 0" {
    run "$SCRIPT" --gate "echo my-claim-line" main "my-claim-line"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "active: --gate with non-matching output returns 1" {
    run "$SCRIPT" --gate "echo something-else" main "expected-claim"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"expected-claim"* ]]
}

@test "active: --gate non-zero exit but matching claim still returns 0" {
    # AP#7 verdict is from the claim grep, NOT the gate's exit status.
    # A gate can legitimately fail other checks while still emitting the
    # specific line a PR claims about. The verdict is the claim alone.
    cat >"$SCRATCH/gate.sh" <<'EOF'
#!/usr/bin/env bash
echo "claim-line-here"
exit 7
EOF
    chmod +x "$SCRATCH/gate.sh"
    run "$SCRIPT" --gate "$SCRATCH/gate.sh" main "claim-line-here"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"gate exit status was 7"* ]]
}

@test "active: --gate command not found returns 3" {
    run "$SCRIPT" --gate "definitely-not-a-real-command-xyz" main "anything"
    [ "$status" -eq 3 ]
    [[ "$output" == *"failed to execute"* ]]
}

# ---- behavior at HEAD ----

@test "active mode reports ref and HEAD sha in stdout preamble" {
    run "$SCRIPT" --gate "echo found" feat/some-branch "found"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ref=feat/some-branch"* ]]
    [[ "$output" == *"head="* ]]
}

# ---- substring (fgrep) semantics ----

@test "claim matches as substring of a longer line" {
    printf 'pass [#24c] AgentOps eval canaries (no eval changes) (skipped)\n' >"$SCRATCH/log"
    run "$SCRIPT" --log "$SCRATCH/log" pr-352 "AgentOps eval canaries"
    [ "$status" -eq 0 ]
}

@test "claim with shell metacharacters treated literally (not as regex)" {
    # fgrep semantics — '.*' is the literal four-char sequence, not a regex.
    printf 'line one\nliteral.dot.match\n' >"$SCRATCH/log"
    run "$SCRIPT" --log "$SCRATCH/log" main "literal.dot.match"
    [ "$status" -eq 0 ]

    run "$SCRIPT" --log "$SCRATCH/log" main "literal.*match"
    [ "$status" -eq 1 ]
}

#!/usr/bin/env bats
#
# Tests for scripts/check-three-gap-supergate.sh — verifies Gap 2's
# compile-health sub-check SKIPs (rather than fails) when neither the
# canonical .agents/defrag/latest.json nor an overnight Dream preview
# is present, matching the existing Gap 1 council-coverage SKIP shape
# for operator-side surfaces.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_HOME="$(mktemp -d)"

    SHIM_ROOT="$TMP_HOME/repo"
    mkdir -p "$SHIM_ROOT/scripts" "$SHIM_ROOT/.agents" "$SHIM_ROOT/cli/bin"
    cp "$REPO_ROOT/scripts/check-three-gap-supergate.sh" "$SHIM_ROOT/scripts/"
    cat > "$SHIM_ROOT/scripts/check-flywheel-compounding-snapshot.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub flywheel-compounding-snapshot OK"
exit 0
EOF
    cat > "$SHIM_ROOT/scripts/proof-run.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub proof-run OK"
exit 0
EOF
    cat > "$SHIM_ROOT/scripts/check-compile-health.sh" <<'EOF'
#!/usr/bin/env bash
# In tests we never want the real compile-health path to fail — if the
# supergate decides to invoke it (artifact present), we treat it as PASS.
echo "stub compile-health OK"
exit 0
EOF
    chmod +x "$SHIM_ROOT/scripts/"*.sh
    : > "$SHIM_ROOT/cli/bin/ao"
    chmod +x "$SHIM_ROOT/cli/bin/ao"
}

teardown() {
    rm -rf "$TMP_HOME"
    # Gap 3 tests use a shim `go` that writes a fake /tmp/ao-sg shell
    # script; clean it up so a subsequent real-goals run that does
    # `go build -o /tmp/ao-sg` does not trip "output already exists
    # and is not an object file".
    rm -f /tmp/ao-sg
}

@test "Gap 2 SKIPs compile-health when no defrag artifact is present" {
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP  compile-health"* ]]
    [[ "$output" == *"three-gap super-gate (durable-learning): PASS"* ]]
}

@test "Gap 2 runs compile-health when overnight preview exists" {
    mkdir -p "$SHIM_ROOT/.agents/overnight/run-1/defrag"
    : > "$SHIM_ROOT/.agents/overnight/run-1/defrag/latest.json"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  compile-health"* ]]
    [[ "$output" != *"SKIP  compile-health"* ]]
}

@test "Gap 2 runs compile-health when canonical defrag artifact exists" {
    mkdir -p "$SHIM_ROOT/.agents/defrag"
    : > "$SHIM_ROOT/.agents/defrag/latest.json"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  compile-health"* ]]
    [[ "$output" != *"SKIP  compile-health"* ]]
}

@test "Gap 2 SKIPs compile-health even when overnight tree exists but has no latest.json" {
    # Empty overnight tree with no defrag/latest.json under any run dir
    # should still trigger the structural SKIP (the find returns nothing).
    mkdir -p "$SHIM_ROOT/.agents/overnight/run-1"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP  compile-health"* ]]
}

@test "Gap 2 can require compile-health artifacts on operator boxes" {
    AGENTOPS_THREE_GAP_REQUIRE_COMPILE_HEALTH=1 run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL  compile-health"* ]]
    [[ "$output" == *"AGENTOPS_THREE_GAP_REQUIRE_COMPILE_HEALTH=1"* ]]
}

# ---------------------------------------------------------------------------
# Gap 1 — council coverage (cycle 63 / soc-wxh5.2)
#
# Sibling pattern: matches the Gap 2 test shape above — same SHIM_ROOT,
# same fixture-per-test setup, same PASS/SKIP message assertion model.
# ---------------------------------------------------------------------------

@test "Gap 1 SKIPs council-coverage when .agents/council/ does not exist" {
    # SHIM_ROOT/.agents has no council subdir (setup didn't create one)
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"no .agents/council/"* ]]
    [[ "$output" == *"three-gap super-gate (council-coverage): PASS"* ]]
}

@test "Gap 1 SKIPs council-coverage when .agents/council/ is empty" {
    # Directory present but no .md files — count is 0 → SKIP path
    mkdir -p "$SHIM_ROOT/.agents/council"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"no .agents/council/"* ]]
}

@test "Gap 1 PASSes council-coverage when at least one .md file is present" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/2026-05-12-fake-council.md"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  council artifacts present"* ]]
    [[ "$output" == *"1 files"* ]]
    [[ "$output" != *"SKIP"* ]]
}

@test "Gap 1 PASSes council-coverage with multiple .md files (count reported)" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/a.md"
    : > "$SHIM_ROOT/.agents/council/b.md"
    : > "$SHIM_ROOT/.agents/council/c.md"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  council artifacts present"* ]]
    [[ "$output" == *"3 files"* ]]
}

@test "Gap 1 ignores non-.md files in .agents/council/ (only counts markdown)" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/notes.txt"
    : > "$SHIM_ROOT/.agents/council/data.json"
    # No .md files at all → SKIP
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
}

# ---------------------------------------------------------------------------
# Gap 1 — strict-coverage mode (cycle 72 / soc-w6vh.6)
#
# The --strict-coverage flag maps PR commits to council references. A
# commit is "covered" if any council artifact mentions its short SHA or
# the commit message has a Council/Pre-mortem/Vibe verdict header.
#
# Sibling pattern: same SHIM_ROOT shape as the structural tests above —
# stub a tmp repo, init git inside, make N commits, possibly stage
# council references, run the gate with --strict-coverage.
# ---------------------------------------------------------------------------

# Helper: turn SHIM_ROOT into a minimal git repo with N commits past 'main'
init_shim_git() {
    git -C "$SHIM_ROOT" init -q -b main
    git -C "$SHIM_ROOT" config user.email "evolve-test@local"
    git -C "$SHIM_ROOT" config user.name "evolve-test"
    git -C "$SHIM_ROOT" commit -q --allow-empty -m "base commit on main"
    git -C "$SHIM_ROOT" checkout -q -b feature
}

@test "Gap 1 default emits INFO line about --strict-coverage opt-in" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/a.md"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"INFO"* ]]
    [[ "$output" == *"--strict-coverage"* ]]
    [[ "$output" == *"structural"* ]]
}

@test "Gap 1 --strict-coverage SKIPs when no commits past base" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/a.md"
    init_shim_git
    # On 'feature' but no commits past 'main' — log returns nothing.
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage --strict-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"--strict-coverage"* ]]
}

@test "Gap 1 --strict-coverage PASSes when all PR commits have council ref" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    init_shim_git
    # Make one commit on feature with a body header that counts as covered.
    git -C "$SHIM_ROOT" commit -q --allow-empty -m "feat: thing

Pre-mortem: see .agents/council/x.md"
    : > "$SHIM_ROOT/.agents/council/x.md"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage --strict-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  --strict-coverage"* ]]
    [[ "$output" == *"1/1"* ]]
}

@test "Gap 1 --strict-coverage FAILs when a PR commit lacks council ref" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    : > "$SHIM_ROOT/.agents/council/x.md"
    init_shim_git
    # Commit with no Council/Pre-mortem/Vibe header AND no SHA in council files.
    git -C "$SHIM_ROOT" commit -q --allow-empty -m "feat: uncovered work"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage --strict-coverage
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL  --strict-coverage"* ]]
    [[ "$output" == *"0/1"* ]]
    [[ "$output" == *"missing:"* ]]
    [[ "$output" == *"FAIL"* ]]
}

@test "Gap 1 --strict-coverage covers by council artifact mentioning short SHA" {
    mkdir -p "$SHIM_ROOT/.agents/council"
    init_shim_git
    # Make commit first, then write a council artifact mentioning the short SHA.
    git -C "$SHIM_ROOT" commit -q --allow-empty -m "feat: needs coverage"
    local sha
    sha="$(git -C "$SHIM_ROOT" log -1 --format=%H)"
    local short="${sha:0:7}"
    echo "Council notes referencing commit $short" > "$SHIM_ROOT/.agents/council/by-sha.md"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=council-coverage --strict-coverage
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  --strict-coverage"* ]]
    [[ "$output" == *"1/1"* ]]
}

# Gap 3 — loop closure (cycle 174 / soc-wxh5.3)
#
# Gap 3 invokes three sub-gates: goals-validate (via a real `go build`
# of cmd/ao + `ao goals validate --json | jq -e .valid==true`),
# wiring-closure (via scripts/check-wiring-closure.sh), and
# flywheel-proof (via scripts/proof-run.sh or cli/bin/ao). The
# goals-validate sub-gate is the tricky one because the original script
# invocation needs a real Go build context that SHIM_ROOT cannot easily
# provide. These tests use path (a) from the cycle-60 acceptance notes:
# PATH-shim a fake `go` that writes a fake `/tmp/ao-sg` returning
# {"valid":true|false} on `goals validate --json`.

shim_go_for_gap3() {
    local valid="${1:-true}"
    mkdir -p "$TMP_HOME/bin"
    cat > "$TMP_HOME/bin/go" <<EOF
#!/usr/bin/env bash
out=""
prev=""
for arg in "\$@"; do
    if [ "\$prev" = "-o" ]; then out="\$arg"; fi
    prev="\$arg"
done
if [ -n "\$out" ]; then
    cat > "\$out" <<INNER
#!/usr/bin/env bash
if [ "\\\$1" = "goals" ] && [ "\\\$2" = "validate" ]; then
    echo '{"valid":${valid}}'
    exit 0
fi
exit 0
INNER
    chmod +x "\$out"
fi
exit 0
EOF
    chmod +x "$TMP_HOME/bin/go"
}

@test "Gap 3 PASSes loop-closure when all sub-gates pass (shimmed go + stub scripts)" {
    shim_go_for_gap3 true
    cat > "$SHIM_ROOT/scripts/check-wiring-closure.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub wiring-closure OK"
exit 0
EOF
    chmod +x "$SHIM_ROOT/scripts/check-wiring-closure.sh"
    PATH="$TMP_HOME/bin:$PATH" run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=loop-closure
    [ "$status" -eq 0 ]
    [[ "$output" == *"three-gap super-gate (loop-closure): PASS"* ]]
}

@test "Gap 3 FAILs loop-closure when goals-validate stub reports valid:false" {
    shim_go_for_gap3 false
    cat > "$SHIM_ROOT/scripts/check-wiring-closure.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$SHIM_ROOT/scripts/check-wiring-closure.sh"
    PATH="$TMP_HOME/bin:$PATH" run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=loop-closure
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL  goals-validate"* ]]
}

@test "Gap 3 SKIPs flywheel-proof when neither cli/bin/ao nor proof-run.sh exists" {
    shim_go_for_gap3 true
    cat > "$SHIM_ROOT/scripts/check-wiring-closure.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$SHIM_ROOT/scripts/check-wiring-closure.sh"
    rm -f "$SHIM_ROOT/scripts/proof-run.sh" "$SHIM_ROOT/cli/bin/ao"
    PATH="$TMP_HOME/bin:$PATH" run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=loop-closure
    [[ "$output" == *"SKIP  flywheel-proof (cli/bin/ao not built)"* ]]
}

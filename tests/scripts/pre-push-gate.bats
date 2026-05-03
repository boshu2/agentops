#!/usr/bin/env bats
# pre-push-gate.bats — Tests for scripts/pre-push-gate.sh
#
# Strategy: We stub out external commands (go, git, scripts/*) via PATH
# manipulation so each gate check can be tested in isolation.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/pre-push-gate.sh"

    TMP_DIR="$(mktemp -d)"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$MOCK_BIN"

    # Build a fake repo with the real script copied in so SCRIPT_DIR resolves here
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p \
        "$FAKE_REPO/scripts" \
        "$FAKE_REPO/cli" \
        "$FAKE_REPO/hooks" \
        "$FAKE_REPO/cli/embedded/hooks" \
        "$FAKE_REPO/skills/heal-skill/scripts" \
        "$FAKE_REPO/tests/skills"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/pre-push-gate.sh"
    chmod +x "$FAKE_REPO/scripts/pre-push-gate.sh"
    touch "$FAKE_REPO/cli/go.mod"
    # Dummy hooks for sync check (matching content = in sync)
    echo "content" > "$FAKE_REPO/hooks/session-start.sh"
    echo "content" > "$FAKE_REPO/cli/embedded/hooks/session-start.sh"
    echo "content" > "$FAKE_REPO/hooks/hooks.json"
    echo "content" > "$FAKE_REPO/cli/embedded/hooks/hooks.json"

    GATE="$FAKE_REPO/scripts/pre-push-gate.sh"
    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"
    make_stub "$FAKE_REPO/scripts/check-mutation-route-coverage.sh"
    make_stub "$FAKE_REPO/scripts/check-retrieval-manifest-paths.sh"
    make_stub "$FAKE_REPO/scripts/check-test-fixture-parity.sh"
    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"
    make_stub "$FAKE_REPO/scripts/check-agents-write-surfaces.sh"
    make_stub "$FAKE_REPO/scripts/check-no-tracked-agents.sh"
    make_stub "$FAKE_REPO/scripts/check-worktree-disposition.sh"
    make_stub "$FAKE_REPO/scripts/validate-skill-runtime-parity.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-skill-parity.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-install-bundle.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-runtime-sections.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-generated-artifacts.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-backbone-prompts.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-override-coverage.sh"
    make_stub "$FAKE_REPO/scripts/validate-next-work-contract-parity.sh"
    make_stub "$FAKE_REPO/scripts/validate-bd-closeout-contract.sh"
    make_stub "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh"
    make_stub "$FAKE_REPO/scripts/validate-skill-runtime-formats.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-rpi-contract.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-lifecycle-guards.sh"
    make_stub "$FAKE_REPO/scripts/validate-skill-cli-snippets.sh"
    make_stub "$FAKE_REPO/scripts/validate-headless-runtime-skills.sh"
    make_stub "$FAKE_REPO/scripts/eval-agentops.sh"
    make_stub "$FAKE_REPO/scripts/docs-build.sh"
    make_stub "$FAKE_REPO/skills/heal-skill/scripts/heal.sh"
    make_stub "$FAKE_REPO/tests/skills/run-all.sh"
    make_stub "$FAKE_REPO/tests/validate-learning-coherence.sh"
    make_stub "$FAKE_REPO/scripts/validate-skill-schema.sh"
    make_stub "$FAKE_REPO/scripts/validate-manifests.sh"
    make_stub "$FAKE_REPO/scripts/generate-cli-reference.sh"
    # Checks 25-33: shifted from CI-only
    mkdir -p "$FAKE_REPO/tests/docs" "$FAKE_REPO/tests/hooks" \
             "$FAKE_REPO/skills" "$FAKE_REPO/lib"
    make_stub "$FAKE_REPO/tests/docs/validate-doc-release.sh"
    make_stub "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh"
    make_stub "$FAKE_REPO/scripts/check-contract-compatibility.sh"
    make_stub "$FAKE_REPO/scripts/validate-swarm-evidence.sh"
    make_stub "$FAKE_REPO/scripts/validate-hook-preflight.sh"
    make_stub "$FAKE_REPO/scripts/check-standards-injector-completeness.sh"
    make_stub "$FAKE_REPO/scripts/validate-hooks-doc-parity.sh"
    make_stub "$FAKE_REPO/scripts/validate-ci-policy-parity.sh"
    make_stub "$FAKE_REPO/scripts/validate-embedded-sync.sh"
    make_stub "$FAKE_REPO/scripts/validate-cli-skills-map.sh"
    make_stub "$FAKE_REPO/scripts/eval-agentops.sh"
    make_stub "$FAKE_REPO/tests/docs/validate-skill-citation-parity.sh"
    make_stub "$FAKE_REPO/tests/hooks/test-orphan-hooks.sh"
    # Check 3b (HOME isolation) and 3c (agents hash snapshot) need executable
    # stubs when tests exercise the Go/hash paths.
    make_stub "$FAKE_REPO/scripts/check-home-isolation.sh"
    make_hash_snapshot_stub "$FAKE_REPO/scripts/check-agents-hash-snapshot.sh"
    make_stub "$FAKE_REPO/scripts/pre-push-proof.sh"
    # soc-h53j: paired stub for prepush-hygiene-gate codex-hooks parity (R2).
    # Symmetric per f-2026-04-27-002: any helper-script reference in
    # scripts/pre-push-gate.sh must have a fake-repo stub here.
    make_stub "$FAKE_REPO/scripts/audit-codex-hooks.sh"
    # soc-y1bk: paired stub for test-isolation-sweep — gate 3b2.
    # Symmetric per f-2026-04-27-002: stub the new lint so the fake repo
    # exercises the same diff-conditional branch the real gate runs.
    make_stub "$FAKE_REPO/scripts/check-test-home-isolation.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Helper: create a stub script that exits with given code
make_stub() {
    local path="$1"
    local exit_code="${2:-0}"
    cat > "$path" <<STUB
#!/usr/bin/env bash
exit $exit_code
STUB
    chmod +x "$path"
}

make_hash_snapshot_stub() {
    local path="$1"
    cat > "$path" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  capture)
    snapshot="$(mktemp)"
    printf 'snapshot\n' > "$snapshot"
    printf '%s\n' "$snapshot"
    ;;
  diff)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
STUB
    chmod +x "$path"
}

@test "pre-push-gate.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "pre-push-gate.sh has set -euo pipefail" {
    run grep -q 'set -euo pipefail' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "pre-push-gate.sh checks all 24 gates" {
    # Verify the script references all gate sections
    run grep -c '# --- [0-9]' "$SCRIPT"
    [ "$status" -eq 0 ]
    [ "$output" -ge 24 ]
}

@test "fake repo setup stubs every pre-push helper script reference" {
    cd "$FAKE_REPO"

    run bash -c '
        set -euo pipefail
        missing=0
        while IFS= read -r helper; do
            case "$helper" in
                scripts/pre-push-gate.sh) continue ;;
            esac
            if [[ ! -e "$helper" ]]; then
                echo "$helper"
                missing=1
            fi
        done < <(grep -oE "(scripts|tests|skills)/[A-Za-z0-9_./-]+[.]sh" scripts/pre-push-gate.sh | sort -u)
        exit "$missing"
    '
    [ "$status" -eq 0 ]
}

@test "pre-push-gate.sh exits 1 on go build failure" {
    # Create a mock go that fails on build
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
if [[ "$1" == "build" ]]; then exit 1; fi
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    # Create a mock git that reports Go changes
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/cmd/ao/main.go"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    # Provide passing stubs for all other checks
    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"go build"* ]]
}

@test "pre-push-gate.sh passes when no Go changes" {
    # Mock git to report no Go changes
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"passed"* ]]
}

@test "pre-push-gate.sh skips release artifact validation for unrelated docs changes" {
    make_stub "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh" 1

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "docs/contracts/hook-runtime-contract.md"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    [[ "$output" == *"release audit artifacts (skipped)"* ]]
}

@test "pre-push-gate.sh runs release artifact validation for release audit docs" {
    make_stub "$FAKE_REPO/scripts/validate-release-audit-artifacts.sh" 1

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "docs/releases/2026-03-22-v2.29.0-audit.md"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE" --fast --scope upstream
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"release audit artifacts"* ]]
}

@test "pre-push-gate.sh detects stale embedded hooks" {
    # Override the embedded sync stub to fail (simulating stale hooks)
    make_stub "$FAKE_REPO/scripts/validate-embedded-sync.sh" 1

    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 1 ]
    [[ "$output" == *"embedded hooks stale"* ]]
}

@test "fast mode runs embedded sync when generated embedded hooks changed" {
    make_stub "$FAKE_REPO/scripts/validate-embedded-sync.sh" 1

    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/embedded/hooks/session-start.sh"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE" --fast --scope upstream
    [ "$status" -eq 1 ]
    [[ "$output" == *"embedded hooks stale"* ]]
}

@test "pre-push-gate.sh fails when validate-go-fast.sh fails" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh" 1
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"go test -race"* ]]
}

@test "pre-push-gate.sh counts multiple failures" {
    # Make everything fail
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
if [[ "$1" == "build" ]]; then exit 1; fi
if [[ "$1" == "vet" ]]; then exit 1; fi
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/cmd/ao/main.go"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh" 1
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh" 1
    make_stub "$FAKE_REPO/scripts/check-cmdao-coverage-floor.sh" 1
    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh" 1

    # Make hooks differ too
    echo "new" > "$FAKE_REPO/hooks/session-start.sh"
    echo "old" > "$FAKE_REPO/cli/embedded/hooks/session-start.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 1 ]
    [[ "$output" == *"BLOCKED"* ]]
}

@test "pre-push-gate.sh fail-fast stops local fast mode after first blocking failure" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
if [[ "$1" == "build" ]]; then exit 1; fi
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/cmd/ao/main.go"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/validate-go-fast.sh" <<'FAST'
#!/usr/bin/env bash
echo "validate-go-fast should not run under fail-fast" >&2
exit 1
FAST
    chmod +x "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"go build"* ]]
    [[ "$output" == *"fail-fast enabled"* ]]
    [[ "$output" != *"validate-go-fast should not run"* ]]
}

@test "pre-push-gate.sh accumulate mode continues after local fast failure" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
if [[ "$1" == "build" ]]; then exit 1; fi
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/cmd/ao/main.go"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/validate-go-fast.sh" <<'FAST'
#!/usr/bin/env bash
echo "validate-go-fast did run" >&2
exit 1
FAST
    chmod +x "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"go build"* ]]
    [[ "$output" == *"validate-go-fast did run"* ]]
}

@test "pre-push-gate.sh fails when worktree disposition check fails" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"
    make_stub "$FAKE_REPO/scripts/check-worktree-disposition.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"worktree disposition"* ]]
}

@test "pre-push-gate.sh skips worktree disposition in local fast mode by default" {
    make_stub "$FAKE_REPO/scripts/check-worktree-disposition.sh" 1

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    [[ "$output" == *"worktree disposition (local fast"* ]]
}

@test "pre-push-gate.sh runs worktree disposition in local fast strict mode" {
    make_stub "$FAKE_REPO/scripts/check-worktree-disposition.sh" 1

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS PRE_PUSH_STRICT_WORKTREE=1 bash "$GATE" --fast --scope upstream
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"worktree disposition"* ]]
}

@test "pre-push-gate.sh fails when codex backbone prompts validation fails" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-backbone-prompts.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL"*"codex backbone prompts"* ]]
}

@test "pre-push-gate.sh fails when codex override coverage validation fails" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"
    make_stub "$FAKE_REPO/scripts/validate-codex-override-coverage.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL"*"codex override coverage"* ]]
}

@test "pre-push-gate.sh fails when headless runtime skill smoke fails" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"
    make_stub "$FAKE_REPO/scripts/validate-headless-runtime-skills.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run bash "$GATE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL"*"headless runtime skills"* ]]
}

@test "pre-push-gate.sh clears GIT env for skill CLI snippets" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then
    echo ""
fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/validate-skill-cli-snippets.sh" <<'SNIPPETS'
#!/usr/bin/env bash
if [[ -n "${GIT_DIR:-}" || -n "${GIT_WORK_TREE:-}" || -n "${GIT_COMMON_DIR:-}" ]]; then
    echo "unexpected git env leaked into skill CLI snippets validator" >&2
    exit 1
fi
exit 0
SNIPPETS
    chmod +x "$FAKE_REPO/scripts/validate-skill-cli-snippets.sh"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env GIT_DIR=/tmp/fake.git GIT_WORK_TREE=/tmp/fake GIT_COMMON_DIR=/tmp/common bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"*"skill CLI snippets"* ]]
}

@test "pre-push-gate.sh clears GIT env for CLI docs parity" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then
    echo "cli/cmd/ao/main.go"
fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/generate-cli-reference.sh" <<'DOCS'
#!/usr/bin/env bash
if [[ -n "${GIT_DIR:-}" || -n "${GIT_WORK_TREE:-}" || -n "${GIT_COMMON_DIR:-}" ]]; then
    echo "unexpected git env leaked into CLI docs generator" >&2
    exit 1
fi
exit 0
DOCS
    chmod +x "$FAKE_REPO/scripts/generate-cli-reference.sh"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"

    make_stub "$FAKE_REPO/scripts/sync-skill-counts.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env GIT_DIR=/tmp/fake.git GIT_WORK_TREE=/tmp/fake GIT_COMMON_DIR=/tmp/common bash "$GATE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"*"CLI docs parity"* ]]
}

@test "pre-push-gate.sh treats retrieval ratchet warning as non-blocking" {
    cat > "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh" <<'RATCHET'
#!/usr/bin/env bash
echo "WARN retrieval quality ratchet: any_relevant_at_k=0.40 threshold=0.60 indexed_turns=0 strict_after=500"
exit 0
RATCHET
    chmod +x "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env PRE_PUSH_AGENT_HEALTH=1 bash "$GATE" --fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"*"retrieval quality ratchet"* ]]
    [[ "$output" == *"pre-push gate (fast): passed"* ]]
}

@test "pre-push-gate.sh skips AgentOps health checks in local fast mode by default" {
    make_stub "$FAKE_REPO/scripts/check-retrieval-quality-ratchet.sh" 1

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"retrieval quality ratchet (local fast"* ]]
    [[ "$output" == *"flywheel health (local fast"* ]]
}

@test "pre-push-gate.sh skips eval canaries for ordinary Go changes in local fast mode" {
    cat > "$MOCK_BIN/go" <<'GO'
#!/usr/bin/env bash
exit 0
GO
    chmod +x "$MOCK_BIN/go"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "cli/cmd/ao/main.go"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    make_stub "$FAKE_REPO/scripts/validate-go-fast.sh"
    make_stub "$FAKE_REPO/scripts/check-go-command-test-pair.sh"
    make_stub "$FAKE_REPO/scripts/eval-agentops.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    [[ "$output" == *"AgentOps eval canaries"*"skipped"* ]]
}

@test "pre-push-gate.sh runs local fast eval canaries as advisory when requested" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "evals/agentops-core/example.json"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/eval-agentops.sh" <<'EVAL'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$BATS_TEST_TMPDIR/eval-args.txt"
echo "FAIL eval-agentops: simulated regression"
exit 0
EVAL
    chmod +x "$FAKE_REPO/scripts/eval-agentops.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS BATS_TEST_TMPDIR="$BATS_TEST_TMPDIR" bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"*"AgentOps eval canaries (advisory)"* ]]
    run grep -q -- '--advisory' "$BATS_TEST_TMPDIR/eval-args.txt"
    [ "$status" -eq 0 ]
}

@test "pre-push-gate.sh blocks local fast eval canaries in strict mode" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "evals/agentops-core/example.json"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cat > "$FAKE_REPO/scripts/eval-agentops.sh" <<'EVAL'
#!/usr/bin/env bash
echo "FAIL eval-agentops: simulated regression"
exit 1
EVAL
    chmod +x "$FAKE_REPO/scripts/eval-agentops.sh"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS PRE_PUSH_STRICT_EVAL=1 bash "$GATE" --fast --scope upstream
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"AgentOps eval canaries"* ]]
}

@test "pre-push-gate.sh warns locally when agents hash capture times out" {
    cat > "$FAKE_REPO/scripts/check-agents-hash-snapshot.sh" <<'HASH'
#!/usr/bin/env bash
if [[ "${1:-}" == "capture" ]]; then
  sleep 2
fi
exit 0
HASH
    chmod +x "$FAKE_REPO/scripts/check-agents-hash-snapshot.sh"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"
    export HASH_GATE_TIMEOUT_SECONDS=1

    run env -u CI -u GITHUB_ACTIONS PRE_PUSH_AGENT_HASH=1 bash "$GATE" --fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"*"snapshot timed out locally"* ]]
    [[ "$output" == *"pre-push gate (fast): passed"* ]]
}

@test "pre-push-gate.sh fails in CI when agents hash capture times out" {
    cat > "$FAKE_REPO/scripts/check-agents-hash-snapshot.sh" <<'HASH'
#!/usr/bin/env bash
if [[ "${1:-}" == "capture" ]]; then
  sleep 2
fi
exit 0
HASH
    chmod +x "$FAKE_REPO/scripts/check-agents-hash-snapshot.sh"

    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"
    export HASH_GATE_TIMEOUT_SECONDS=1
    export CI=true

    run env PRE_PUSH_AGENT_HASH=1 bash "$GATE" --fast
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"agents-hub content-hash gate snapshot failed"* ]]
}

# --- soc-h53j: prepush-hygiene-gate (items 5, 8-12, 17, 25, 26, 28, 32) ---

@test "pre-push-gate.sh skips codex hook manifest parity when no hook diff" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    [[ "$output" == *"codex hook manifest parity"*"skipped"* ]]
}

@test "pre-push-gate.sh runs codex hook manifest parity when hooks change and codex hooks file exists" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "hooks/hooks.json"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    # Provide a fake CODEX_HOME with a hooks.json so the new gate fires.
    fake_codex_home="$BATS_TEST_TMPDIR/codex-home"
    mkdir -p "$fake_codex_home"
    echo '{"hooks": {}}' > "$fake_codex_home/hooks.json"

    # Stub audit-codex-hooks.sh to fail so we can assert the gate routes through it.
    make_stub "$FAKE_REPO/scripts/audit-codex-hooks.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS \
        CODEX_HOME="$fake_codex_home" \
        bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"codex hook manifest parity"* ]]
}

@test "pre-push-gate.sh skips codex hook manifest parity when AGENTOPS_PREPUSH_SKIP_CODEX_HOOKS=1" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "hooks/hooks.json"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    fake_codex_home="$BATS_TEST_TMPDIR/codex-home"
    mkdir -p "$fake_codex_home"
    echo '{"hooks": {}}' > "$fake_codex_home/hooks.json"

    # Even with a failing audit stub, the skip flag must keep the gate green.
    make_stub "$FAKE_REPO/scripts/audit-codex-hooks.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS \
        CODEX_HOME="$fake_codex_home" \
        AGENTOPS_PREPUSH_SKIP_CODEX_HOOKS=1 \
        bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 0 ]
    [[ "$output" == *"codex hook manifest parity (AGENTOPS_PREPUSH_SKIP_CODEX_HOOKS=1)"* ]]
}

@test "pre-push-gate.sh skips codex hook manifest parity when CODEX_HOME has no hooks.json" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "hooks/hooks.json"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    fake_codex_home="$BATS_TEST_TMPDIR/codex-home-empty"
    mkdir -p "$fake_codex_home"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS CODEX_HOME="$fake_codex_home" \
        bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 0 ]
    [[ "$output" == *"codex hook manifest parity"*"no $fake_codex_home/hooks.json"* ]]
}

@test "pre-push-gate.sh prepush-hygiene-gate items 5, 8-12, 17, 25, 26, 28, 32 are wired into the script" {
    # Symmetric audit per f-2026-04-27-002: each item from
    # .agents/plans/2026-05-03-ci-failures-1-40-handling.md §prepush-hygiene-gate
    # must have a corresponding section header in pre-push-gate.sh. This test
    # is the BATS-side guardrail that prevents silent gate drift.
    run grep -E '# --- (5\.|6\.|24b\.|25\.|27b\.|28\.|28b\.|31\.|32\.) ' "$SCRIPT"
    [ "$status" -eq 0 ]
    # At least the items above should each appear once.
    [[ "$output" == *"# --- 5. Embedded hooks sync"* ]]
    [[ "$output" == *"# --- 6. Skill count sync"* ]]
    [[ "$output" == *"# --- 24b. CLI docs parity"* ]]
    [[ "$output" == *"# --- 25. Doc-release"* ]]
    [[ "$output" == *"# --- 27b. Standards-injector"* ]]
    [[ "$output" == *"# --- 28. Hooks/docs parity"* ]]
    [[ "$output" == *"# --- 28b. Codex hook manifest parity"* ]]
    [[ "$output" == *"# --- 31. Plugin load test"* ]]
    [[ "$output" == *"# --- 32. Learning coherence"* ]]
}

@test "pre-push-gate.sh fast clean-tree completes in under 30 seconds (R3)" {
    # R3: regression guard against latent slowdown of the diff-conditional gate.
    # The clean-tree fast path must remain under 30s so it stays usable as a
    # pre-push hook. Today it is ~5s on the operator's mac; this test asserts
    # the upper bound the bead committed to.
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo ""; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    start=$(date +%s)
    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    end=$(date +%s)
    elapsed=$((end - start))

    [ "$status" -eq 0 ]
    [[ "$output" == *"pre-push gate (fast): passed"* ]]
    # Hard cap: 30 seconds (acceptance R3 from soc-h53j).
    [ "$elapsed" -lt 30 ]
}

@test "pre-push-gate.sh exposes prepush_skip_flag helper for diff-conditional checks" {
    # Acceptance: each new conditional check is feature-flagged via env var
    # (AGENTOPS_PREPUSH_SKIP_<NAME>=1). Verify the helper exists and reads the
    # documented env-var pattern.
    run grep -q 'prepush_skip_flag()' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'AGENTOPS_PREPUSH_SKIP_' "$SCRIPT"
    [ "$status" -eq 0 ]
}

# ─────────────────────────────────────────────────────────────────────────
# soc-y1bk: paired tests for the new test-HOME-isolation lint (gate 3b2).
# Symmetric per f-2026-04-27-002: any helper-script reference in the gate
# script must have a fake-repo stub here AND tests asserting the diff-
# conditional behavior (run / skip-flag / pass-through).
# ─────────────────────────────────────────────────────────────────────────

@test "pre-push-gate.sh runs check-test-home-isolation.sh when shell files change" {
    # Use a shell-only diff (HAS_SHELL=1, HAS_GO=0) so the lint runs but the
    # Go build/vet checks short-circuit and don't trip on the empty fake go.mod.
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "scripts/check-test-home-isolation.sh"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    # Failing stub so we can prove the gate routes through the lint.
    make_stub "$FAKE_REPO/scripts/check-test-home-isolation.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"*"test HOME isolation"* ]]
}

@test "pre-push-gate.sh skips test HOME isolation when AGENTOPS_PREPUSH_SKIP_TEST_HOME_ISO=1" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "scripts/check-test-home-isolation.sh"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    # Even with a failing lint stub, the skip flag must keep the gate green.
    make_stub "$FAKE_REPO/scripts/check-test-home-isolation.sh" 1

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS \
        AGENTOPS_PREPUSH_SKIP_TEST_HOME_ISO=1 \
        bash "$GATE" --fast --scope upstream --accumulate
    [ "$status" -eq 0 ]
    [[ "$output" == *"test HOME isolation (AGENTOPS_PREPUSH_SKIP_TEST_HOME_ISO=1)"* ]]
}

@test "pre-push-gate.sh skips test HOME isolation when no Go or shell files change" {
    cat > "$MOCK_BIN/git" <<'GIT'
#!/usr/bin/env bash
if [[ "$*" == *"diff --name-only"* ]]; then echo "docs/README.md"; fi
if [[ "$*" == *"rev-parse"* ]]; then echo "/tmp"; fi
exit 0
GIT
    chmod +x "$MOCK_BIN/git"

    cd "$FAKE_REPO"
    export PATH="$MOCK_BIN:$PATH"

    run env -u CI -u GITHUB_ACTIONS bash "$GATE" --fast --scope upstream
    [ "$status" -eq 0 ]
    # Doc-only diff should skip the lint (output marker says "skipped").
    [[ "$output" == *"test HOME isolation"*"skipped"* ]]
}

@test "pre-push-gate.sh test-home-isolation lint stub exists in fake repo (paired stub guard)" {
    # Guardrail per f-2026-04-27-002: the .bats setup MUST provision a stub
    # for every helper-script that the real gate references. If anyone removes
    # the stub from setup() without removing the gate reference, this test
    # catches it.
    [ -x "$FAKE_REPO/scripts/check-test-home-isolation.sh" ]
    run grep -q 'check-test-home-isolation\.sh' "$SCRIPT"
    [ "$status" -eq 0 ]
}

#!/usr/bin/env bats
# ci-local-release.bats — Tests for scripts/ci-local-release.sh
#
# Strategy: exercise the script's CLI flag parsing, validation, and fast-path
# behavior. Heavy gates are excluded via --fast and further neutralized by
# stubbing the scripts/tests they invoke.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/ci-local-release.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

make_stub() {
    local path="$1"
    mkdir -p "$(dirname "$path")"
    cat > "$path" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
if [[ -n "${CI_STUB_LOG:-}" ]]; then
    printf '%s\n' "$(basename "$0")" >> "$CI_STUB_LOG"
fi
exit 0
STUB
    chmod +x "$path"
}

make_mock_tool() {
    local path="$1"
    local body="$2"
    mkdir -p "$(dirname "$path")"
    printf '%s\n' '#!/usr/bin/env bash' "$body" > "$path"
    chmod +x "$path"
}

setup_fake_release_repo() {
    FAKE_REPO="$TMP_DIR/repo"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/cli/bin" "$FAKE_REPO/.claude-plugin" "$MOCK_BIN"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/ci-local-release.sh"
    chmod +x "$FAKE_REPO/scripts/ci-local-release.sh"

    cat > "$FAKE_REPO/.claude-plugin/plugin.json" <<'JSON'
{"version":"2.99.0"}
JSON
    cat > "$FAKE_REPO/.claude-plugin/marketplace.json" <<'JSON'
{"metadata":{"version":"2.99.0"},"plugins":[{"version":"2.99.0"}]}
JSON

    cat > "$FAKE_REPO/cli/bin/ao" <<'AO'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-} ${2:-}" in
  "hooks install")
    mkdir -p "$HOME/.claude" "$HOME/.agentops/hooks"
    printf '{}\n' > "$HOME/.claude/settings.json"
    printf '#!/usr/bin/env bash\n' > "$HOME/.agentops/hooks/session-start.sh"
    ;;
esac
exit 0
AO
    chmod +x "$FAKE_REPO/cli/bin/ao"

    make_mock_tool "$MOCK_BIN/git" 'case "$*" in *"describe"*) echo "v2.99.0";; *"rev-parse HEAD"*) echo "0000000000000000000000000000000000000000";; *"ls-files"*) exit 0;; *) exit 0;; esac'
    make_mock_tool "$MOCK_BIN/go" 'exit 0'
    make_mock_tool "$MOCK_BIN/make" 'mkdir -p bin; exit 0'
    make_mock_tool "$MOCK_BIN/shellcheck" 'exit 0'
    make_mock_tool "$MOCK_BIN/markdownlint" 'exit 0'
    make_mock_tool "$MOCK_BIN/bats" 'exit 0'

    local stub_paths=(
        tests/docs/validate-doc-release.sh
        scripts/validate-manifests.sh
        scripts/validate-hook-preflight.sh
        scripts/validate-hooks-doc-parity.sh
        scripts/validate-ci-policy-parity.sh
        scripts/validate-surface-inventory.sh
        scripts/check-worktree-disposition.sh
        skills/heal-skill/scripts/heal.sh
        scripts/validate-skill-runtime-parity.sh
        scripts/validate-codex-runtime-sections.sh
        scripts/validate-codex-generated-manifest.sh
        scripts/validate-codex-generated-artifacts.sh
        scripts/validate-codex-backbone-prompts.sh
        scripts/validate-next-work-contract-parity.sh
        scripts/validate-skill-runtime-formats.sh
        scripts/check-contract-compatibility.sh
        scripts/validate-embedded-sync.sh
        scripts/validate-skill-cli-snippets.sh
        scripts/check-go-command-test-pair.sh
        scripts/check-memrl-health.sh
        scripts/check-doctor-health.sh
        scripts/generate-cli-reference.sh
        tests/smoke-test.sh
        tests/skills/run-all.sh
        scripts/validate-headless-runtime-skills.sh
        tests/integration/test-cli-commands.sh
        tests/scripts/test-go-command-test-pair.sh
        tests/scripts/test-competitive-freshness.sh
        tests/scripts/test-skill-runtime-parity.sh
        tests/scripts/test-skill-cli-snippets.sh
        tests/scripts/test-codex-plugin-install.sh
        tests/scripts/test-codex-native-skills-install.sh
        tests/scripts/test-codex-generated-manifest.sh
        tests/scripts/test-codex-generated-artifacts.sh
        tests/scripts/test-codex-backbone-prompts.sh
        tests/scripts/test-install-dev-hooks.sh
        tests/scripts/test-githook-shims.sh
        tests/scripts/test-validate-local.sh
        tests/scripts/test-headless-runtime-skills.sh
        tests/hooks/test-constraint-compiler.sh
        scripts/validate-skill-schema.sh
        scripts/validate-learning-coherence.sh
        tests/cli/test-json-flag-consistency.sh
        tests/cli/test-json-flag-consistency-tempdir.sh
        scripts/validate-release.sh
        scripts/release-smoke-test.sh
        scripts/check-agents-hash-snapshot.sh
    )

    local stub
    for stub in "${stub_paths[@]}"; do
        make_stub "$FAKE_REPO/$stub"
    done
}

@test "ci-local-release.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "ci-local-release.sh has set -euo pipefail" {
    run grep -q 'set -euo pipefail' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "--help prints usage and exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
    [[ "$output" == *"--fast"* ]]
    [[ "$output" == *"--security-mode"* ]]
}

@test "-h prints usage and exits 0" {
    run bash "$SCRIPT" -h
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
}

@test "unknown flag is rejected with usage and exit 1" {
    run bash "$SCRIPT" --not-a-real-flag
    [ "$status" -eq 1 ]
    [[ "$output" == *"Unknown option"* ]]
}

@test "--security-mode rejects invalid values" {
    run bash "$SCRIPT" --security-mode garbage
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --security-mode"* ]]
}

@test "--security-mode accepts quick (help-exit path)" {
    # The script validates --security-mode before --help short-circuit when given.
    # So we pass --help last to force an early successful exit without running gates.
    run bash "$SCRIPT" --security-mode quick --help
    [ "$status" -eq 0 ]
}

@test "--security-mode accepts full (help-exit path)" {
    run bash "$SCRIPT" --security-mode full --help
    [ "$status" -eq 0 ]
}

@test "--release-version rejects garbage values" {
    run bash "$SCRIPT" --release-version not-a-version
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --release-version"* ]]
}

@test "--release-version accepts semver (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts semver with leading v (help-exit path)" {
    run bash "$SCRIPT" --release-version v2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts prerelease suffixes (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0-rc.1 --help
    [ "$status" -eq 0 ]
}

@test "--jobs accepts numeric value (help-exit path)" {
    run bash "$SCRIPT" --jobs 4 --help
    [ "$status" -eq 0 ]
}

@test "script references ARTIFACT_DIR for release-grade artifact tracking" {
    # Verifies the RUN_ID / ARTIFACT_DIR pattern is present, since release
    # provenance depends on artifacts being written to a dated directory.
    run grep -q 'ARTIFACT_DIR=' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script defines pass/fail/warn helpers consistent with pre-push-gate" {
    run grep -qE '^pass\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^fail\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^warn\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script starts errors counter at 0" {
    run grep -q '^errors=0' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script defines a FAST_MODE default of false" {
    run grep -q 'FAST_MODE=false' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script increments error count from fail helper" {
    # Guard the convention: fail() must bump errors so the gate can aggregate.
    run grep -q 'errors=\$((errors + 1))' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "fast mode executes stubbed local gate matrix including surface inventory" {
    setup_fake_release_repo
    export PATH="$MOCK_BIN:$PATH"
    export CI_STUB_LOG="$TMP_DIR/stub.log"

    run bash "$FAKE_REPO/scripts/ci-local-release.sh" --fast --jobs 4

    [ "$status" -eq 0 ]
    [[ "$output" == *"LOCAL CI PASSED"* ]]
    run grep -q '^validate-surface-inventory.sh$' "$CI_STUB_LOG"
    [ "$status" -eq 0 ]
    run grep -q '^validate-ci-policy-parity.sh$' "$CI_STUB_LOG"
    [ "$status" -eq 0 ]
}

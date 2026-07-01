#!/usr/bin/env bats
# Tests for scripts/check-go-lint.sh — the release gate that wires the
# repo-pinned golangci-lint into `ao gate check` so the documented lint budgets
# (.claude/rules/go.md — gocyclo fail at 25, errcheck, staticcheck, copyloopvar)
# cannot drift onto green main (age-gate-the-ungated-egwt.7).
#
# The gate's three exit contracts are pinned here:
#   - clean tree                -> exit 0 (the real post-fix repo)
#   - linter unresolvable       -> exit non-zero, names the install command
#                                  (skip-if-absent is explicitly banned / fail-open)
#   - a real lint finding        -> exit non-zero, names file:line + class
#
# The linter-absent and finding-present cases run against an ISOLATED temp repo
# skeleton so the real checkout is never mutated.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-go-lint.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Build a minimal repo skeleton at $TMP_DIR that mirrors the layout the script
# derives from its own path: scripts/check-go-lint.sh + a cli/ dir. The pinned
# linter wrapper is intentionally OMITTED so the fail-closed path fires.
make_skeleton_without_linter() {
    mkdir -p "$TMP_DIR/scripts" "$TMP_DIR/cli"
    cp "$SCRIPT" "$TMP_DIR/scripts/check-go-lint.sh"
    chmod +x "$TMP_DIR/scripts/check-go-lint.sh"
    # Deliberately DO NOT copy scripts/golangci-lint-v2.sh.
}

@test "the real repo tree is lint-clean (exit 0, reports clean)" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"golangci-lint clean"* ]]
    [[ "$output" == *"0 findings"* ]]
}

@test "the pinned linter being unresolvable FAILS closed and names the install command" {
    make_skeleton_without_linter
    run bash "$TMP_DIR/scripts/check-go-lint.sh"
    [ "$status" -ne 0 ]
    [[ "$output" == *"pinned linter wrapper not found"* ]]
    # The exact recovery command must be surfaced (fail-closed contract).
    [[ "$output" == *"cd cli && make lint"* ]]
    # It must NOT read as a skip/pass.
    [[ "$output" != *"clean"* ]]
}

@test "a real lint finding FAILS the gate and names file:line + class" {
    # Inject a temp Go file into the real cli tree that trips a mechanical
    # staticcheck finding (S1008 De Morgan / early-return), run the gate, then
    # remove it. The injected file is uniquely named and always cleaned up.
    probe="$REPO_ROOT/cli/internal/aostate/zz_bats_lint_probe.go"
    cat > "$probe" <<'GO'
package aostate

import "strings"

func zzBatsLintProbe(s string) bool {
	if !(strings.HasPrefix(s, "a") || strings.HasSuffix(s, "z")) {
		return true
	}
	return false
}
GO
    run bash "$SCRIPT"
    local st="$status" out="$output"
    rm -f "$probe"

    [ "$st" -ne 0 ]
    [[ "$out" == *"reported findings"* ]]
    # file:line of the injected finding is named...
    [[ "$out" == *"zz_bats_lint_probe.go:6"* ]]
    # ...along with the linter's class annotation.
    [[ "$out" == *"(staticcheck)"* ]]
}

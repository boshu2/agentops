#!/usr/bin/env bats
# Acceptance tests — Go-backed Stream B scenarios: B3-S1, B5-S1..S4, B5-G1/G2/G3/G13.
#
# These drive the REAL Go packages by copying the acceptance-test templates in
# go-acceptance/*.go.txt into their target packages, running a scoped `go test`,
# then removing them (so the docs tree never holds compilable test files and the
# package stays clean). TEST-FIRST: they are RED until B3/B5 land.
#
# Run from the repo root:
#   bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-go.bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../../../.." && pwd)"
    TPL="$BATS_TEST_DIRNAME/go-acceptance"
    LIVENESS_PKG="$REPO_ROOT/cli/internal/liveness"
    CODEX_PKG="$REPO_ROOT/cli/cmd/ao"
    LIVENESS_GEN="$LIVENESS_PKG/recon_acceptance_gen_test.go"
    CODEX_GEN="$CODEX_PKG/recon_acceptance_codex_gen_test.go"
    CODEX_HELPERS_GEN="$CODEX_PKG/recon_acceptance_codex_helpers_gen_test.go"
}

teardown() {
    rm -f "$LIVENESS_GEN" "$CODEX_GEN" "$CODEX_HELPERS_GEN"
}

# B3-S1 + B5-G3 live in the liveness package.
@test "B3-S1 + B5-G3: quorum default OFF + forged-ratification cannot execute (liveness)" {
    cp "$TPL/recon_acceptance_liveness_test.go.txt" "$LIVENESS_GEN"
    run bash -c "cd '$REPO_ROOT/cli' && go test ./internal/liveness/ -run ReconAcceptance -count=1"
    [ "$status" -eq 0 ]
}

# B5-S1..S4 + B5-G1/G2/G13 live in the cmd/ao (codex) package.
@test "B5-S1..S4 + B5-G1/G2/G13: codex sh -c trust boundary + symlink-safe paths (cmd/ao)" {
    cp "$TPL/recon_acceptance_codex_test.go.txt" "$CODEX_GEN"
    cp "$TPL/recon_acceptance_codex_helpers_test.go.txt" "$CODEX_HELPERS_GEN"
    run bash -c "cd '$REPO_ROOT/cli' && go test ./cmd/ao/ -run ReconAcceptanceB5 -count=1"
    [ "$status" -eq 0 ]
}

#!/usr/bin/env bats
# Acceptance surface for scripts/check-release-parity.sh (FU5, age-8duhj).
#
# Given a release-tag build, When artifacts are produced, Then the gate fails on
# README / license / version divergence between the tarball, an in-repo tap
# formula, and the repo tree — and passes when they agree.
#
# Fixtures are built in tmp trees (a synthetic --repo-root with its own LICENSE
# and README) so the checker never scans the real repo's homebrew-tap/.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-parity.sh"
    FIX="$(mktemp -d "$BATS_TMPDIR/parity.XXXXXX")"

    # Synthetic repo root with an Apache-2.0 LICENSE and a canonical README.
    printf '                                 Apache License\n                           Version 2.0, January 2004\n' > "$FIX/LICENSE"
    printf '# AgentOps\n\nOperating loop for coding agents — intent -> validated code.\n' > "$FIX/README.md"

    # A "good" tarball packs the current repo README at its root (flat archive).
    mkdir -p "$FIX/pack"
    cp "$FIX/README.md" "$FIX/pack/README.md"
    tar -czf "$FIX/ao-good.tar.gz" -C "$FIX/pack" README.md

    # A "stale" tarball packs the PREVIOUS README (the v3.2.0 hazard).
    printf '# AgentOps\n\nAutonomous code validation for coding agents.\n' > "$FIX/pack/README.md"
    tar -czf "$FIX/ao-stale.tar.gz" -C "$FIX/pack" README.md
}

teardown() {
    [ -n "${FIX:-}" ] && [ -d "$FIX" ] && find "$FIX" -mindepth 0 -delete 2>/dev/null || true
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "--help prints usage and exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Assert parity"* ]]
    [[ "$output" == *"--tarball"* ]]
}

@test "red: stale README tarball -> gate FAILS naming README divergence" {
    run bash "$SCRIPT" --tarball "$FIX/ao-stale.tar.gz" --repo-root "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"README divergence"* ]]
}

@test "green: matching README tarball -> gate PASSES" {
    run bash "$SCRIPT" --tarball "$FIX/ao-good.tar.gz" --repo-root "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"README parity"* ]]
    [[ "$output" == *"OK"* ]]
}

@test "green: no in-repo formula -> formula parity passes (GoReleaser owns the tap)" {
    run bash "$SCRIPT" --tarball "$FIX/ao-good.tar.gz" --repo-root "$FIX" --tag v3.2.0
    [ "$status" -eq 0 ]
    [[ "$output" == *"no in-repo Homebrew formula"* ]]
}

@test "red: in-repo formula with wrong license (MIT vs Apache-2.0) -> gate FAILS" {
    mkdir -p "$FIX/homebrew-tap/Formula"
    cat > "$FIX/homebrew-tap/Formula/agentops.rb" <<'RB'
class Agentops < Formula
  version "3.2.0"
  license "MIT"
end
RB
    run bash "$SCRIPT" --tarball "$FIX/ao-good.tar.gz" --repo-root "$FIX" --tag v3.2.0
    [ "$status" -eq 1 ]
    [[ "$output" == *'license "MIT" != repo license "Apache-2.0"'* ]]
}

@test "red: in-repo formula with stale version -> gate FAILS on version" {
    mkdir -p "$FIX/homebrew-tap/Formula"
    cat > "$FIX/homebrew-tap/Formula/agentops.rb" <<'RB'
class Agentops < Formula
  version "2.31.0"
  license "Apache-2.0"
end
RB
    run bash "$SCRIPT" --tarball "$FIX/ao-good.tar.gz" --repo-root "$FIX" --tag v3.2.0
    [ "$status" -eq 1 ]
    [[ "$output" == *'version "2.31.0" != release tag "3.2.0"'* ]]
}

@test "green: in-repo formula matching license + version -> gate PASSES" {
    mkdir -p "$FIX/homebrew-tap/Formula"
    cat > "$FIX/homebrew-tap/Formula/agentops.rb" <<'RB'
class Agentops < Formula
  version "3.2.0"
  license "Apache-2.0"
end
RB
    run bash "$SCRIPT" --tarball "$FIX/ao-good.tar.gz" --repo-root "$FIX" --tag v3.2.0
    [ "$status" -eq 0 ]
    [[ "$output" == *"license parity"* ]]
    [[ "$output" == *"version parity"* ]]
}

@test "formula-only run (no --tarball) skips README check and still lints formula" {
    mkdir -p "$FIX/homebrew-tap/Formula"
    cat > "$FIX/homebrew-tap/Formula/agentops.rb" <<'RB'
class Agentops < Formula
  version "3.2.0"
  license "MIT"
end
RB
    run bash "$SCRIPT" --repo-root "$FIX" --tag v3.2.0
    [ "$status" -eq 1 ]
    [[ "$output" == *"SKIP  README parity"* ]]
    [[ "$output" == *'license "MIT"'* ]]
}

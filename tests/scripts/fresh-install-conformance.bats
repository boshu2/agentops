#!/usr/bin/env bats
# Failure-mode tests for scripts/fresh-install-conformance.sh (age-wl5vm / FU1).
#
# The conformance harness is the integration net over the fresh-user onboarding
# path. These tests prove the net actually CATCHES the two regression classes it
# exists to guard, rather than rubber-stamping — a green harness that can't fail
# is worse than none.
#
# Every case runs OFFLINE and FAST by feeding the harness a fake `ao` through
# its --release-tarball entry point (a tar.gz holding a stub `ao`), so no
# go build, no network, and no real binary are needed. Section (b) uses
# `ao skills link` against the worktree with HOME pointed at a temp dir —
# never the real $HOME.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/fresh-install-conformance.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  command -v python3 >/dev/null 2>&1 || skip "python3 required by the harness"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# make_fake_ao_tarball writes a stub `ao` whose behaviour is selected at run
# time by the $AO_FAKE_MODE env var (preserved through the harness's `env
# HOME=... ao ...` calls), then packs it into a release-shaped tarball and
# echoes the tarball path.
make_fake_ao_tarball() {
  local stage="$TMP/stage/agentops_9.9.9"
  mkdir -p "$stage"
  cat > "$stage/ao" <<'AO'
#!/usr/bin/env bash
case "$1" in
  --version)
    echo "ao version 9.9.9"
    ;;
  capabilities)
    # The only real command the fake tree advertises is `ao status`.
    echo '{"commands":[{"path":"ao","args":"subcommands-only"},{"path":"ao status","args":"range"}]}'
    ;;
  quick-start)
    if [[ "${AO_FAKE_MODE:-}" == "omit-command" ]]; then
      # Advertises a command that is NOT in the capability contract.
      echo "  Next: run ao frobnicate to finish setup"
    else
      echo "  Next: run ao status to inspect your work"
    fi
    ;;
  doctor)
    echo "ao doctor"
    echo "1/1 checks passed"
    if [[ "${AO_FAKE_MODE:-}" == "repo-script-fix" ]]; then
      # A fix a fresh user cannot run: a repo-relative script path.
      echo "  Fix: run scripts/fix-me.sh to repair the install"
    fi
    ;;
  skills)
    # Fresh-install section (b) runs `ao skills link`.
    if [[ "${2:-}" == "link" ]]; then
      mkdir -p "${HOME}/.agents/skills/plan"
      printf 'linked\n' >"${HOME}/.agents/skills/plan/.link-ok"
      exit 0
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
AO
  chmod +x "$stage/ao"
  local tarball="$TMP/fake-ao.tar.gz"
  tar -czf "$tarball" -C "$TMP/stage" agentops_9.9.9
  echo "$tarball"
}

run_harness() {
  local tarball="$1"
  run env AO_FAKE_MODE="${AO_FAKE_MODE:-}" HOME="$TMP/home" \
    bash "$SCRIPT" --release-tarball "$tarball"
}

@test "harness catches an advertised ao command missing from the binary" {
  tarball="$(make_fake_ao_tarball)"
  export AO_FAKE_MODE="omit-command"
  run_harness "$tarball"
  [ "$status" -ne 0 ]
  [[ "$output" == *"FRESH-INSTALL CONFORMANCE: FAIL"* ]]
  [[ "$output" == *"frobnicate"* ]]
  [[ "$output" == *"does not resolve"* ]]
}

@test "harness catches a doctor fix that names a repo-relative script" {
  tarball="$(make_fake_ao_tarball)"
  export AO_FAKE_MODE="repo-script-fix"
  run_harness "$tarball"
  [ "$status" -ne 0 ]
  [[ "$output" == *"FRESH-INSTALL CONFORMANCE: FAIL"* ]]
  [[ "$output" == *"repo-relative script"* ]]
  [[ "$output" == *"fix-me.sh"* ]]
}

@test "harness passes when the fake binary is internally consistent" {
  tarball="$(make_fake_ao_tarball)"
  export AO_FAKE_MODE="consistent"
  run_harness "$tarball"
  [ "$status" -eq 0 ]
  [[ "$output" == *"FRESH-INSTALL CONFORMANCE: PASS"* ]]
}

@test "harness loud-SKIPs (exit 0) when the release asset is unreachable offline" {
  run env HOME="$TMP/home" bash "$SCRIPT" --release-tarball "https://example.invalid/no/ao.tar.gz"
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
  [[ "$output" == *"loud skip, not a pass"* ]]
}

@test "release mode with --require-asset FAILS (not skip) on an unreachable asset" {
  run bash "$REPO_ROOT/scripts/fresh-install-conformance.sh" \
    --release-tarball "/nonexistent/path/ao-none.tar.gz" --require-asset
  [ "$status" -eq 1 ]
  [[ "$output" == *"must fail, not skip"* ]]
}

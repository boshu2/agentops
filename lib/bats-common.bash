# shellcheck shell=bash
# lib/bats-common.bash — shared fixture helpers for tests/**/*.bats (soc-jhq6).
#
# Every script-test bats file re-derived the same fixture boilerplate (mktemp,
# git init + identity, bin-stubbing), which drifted between files (e.g. one
# forgetting a `git config` and failing only on a commit). These helpers
# centralize it. Source it from a bats setup():
#
#   source "$(git rev-parse --show-toplevel)/lib/bats-common.bash"
#   REPO_ROOT="$(bats_repo_root)"
#   TMP="$(mktemp -d)"
#   bats_init_repo "$TMP"
#
# Functions only — no top-level shell options, so sourcing never alters the
# caller's `set -e`/`set -u` state.

# bats_repo_root — absolute path to the repository root.
bats_repo_root() {
  git rev-parse --show-toplevel
}

# bats_init_repo <dir> — turn <dir> into a fresh committable git repo and cd into
# it, with a deterministic identity so commits/rebases never prompt for one.
# Background maintenance (auto-gc, fsmonitor) is disabled so a git daemon can't
# hold a file in <dir>/.git when the test's teardown runs `rm -rf "<dir>"` — that
# race made the rebase fixture flaky in CI (soc-72gkw).
bats_init_repo() {
  local dir="${1:?bats_init_repo: <dir> required}"
  cd "$dir" || return 1
  git init -q
  git config user.email "bats@test.local"
  git config user.name "bats-fixture"
  git config gc.auto 0
  git config maintenance.auto false
  git config core.fsmonitor false
}

# bats_stub_bin <bindir> <name> <body> — create an executable stub command
# <name> in <bindir> with <body> as its script body (after the shebang). The
# caller is responsible for prepending <bindir> to PATH. Returns non-zero if
# <bindir> or <name> is missing.
bats_stub_bin() {
  local bindir="${1:?bats_stub_bin: <bindir> required}"
  local name="${2:?bats_stub_bin: <name> required}"
  local body="${3:-}"
  mkdir -p "$bindir"
  {
    printf '#!/usr/bin/env bash\n'
    printf '%s\n' "$body"
  } >"$bindir/$name"
  chmod +x "$bindir/$name"
}

#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/repo-root.sh — sourced library: hook-safe, worktree-correct repo
# root resolution (bead age-gate-scripts-worktree-gitdir-p62wo).
#
# Source it at the top of a script (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
#
# WHY: git hooks (pre-push et al.) export GIT_DIR — usually WITHOUT
# GIT_WORK_TREE. Under that env a bare `git rev-parse --show-toplevel`:
#   * from a subdirectory returns the SUBDIRECTORY (git treats cwd as the
#     worktree top), so every REPO_ROOT-relative path breaks; and
#   * fails outright ("fatal: this operation must be run in a work tree",
#     empty root) when the shared config is poisoned with core.bare=true —
#     the exact 2026-07-18 gate outage from linked worktrees.
#
# Unlike preamble.sh this library sets NO strict mode and defines only
# functions, so scripts with their own option handling can source it safely.
# preamble.sh already computes an equivalently hardened REPO_ROOT for its own
# consumers; new scripts that want the full preamble should keep using it.

# Idempotent: sourcing twice is a no-op.
if ! declare -F resolve_repo_root >/dev/null 2>&1; then

# resolve_repo_root → print the absolute root of the checkout this LIBRARY
# lives in. Worktree-correct: a linked worktree carries its own copy of
# scripts/lib, so resolution anchors at that copy and returns the WORKTREE
# root, never the main checkout. Deliberately independent of the caller's cwd
# (a script run from inside a different git checkout cannot hijack the root).
#
#   1. git resolution in a subshell with git's hook-injected discovery env
#      (GIT_DIR, GIT_WORK_TREE, ...) unset, anchored at the lib's own dir;
#   2. fallback: the lib's fixed position — <root>/scripts/lib/repo-root.sh,
#      two dirs up — when git resolution fails (poisoned config, extracted
#      tarball, git missing).
#
# `CDPATH=` is an intentional one-command env clear (a caller's CDPATH must
# not hijack the relative cd), not a botched assignment.
resolve_repo_root() {
  local lib_dir root
  # Explicit override — the TEST SEAM. Fixture suites run real scripts against
  # a temp repo; without this they would resolve the real checkout and mutate
  # it (the 2026-07-18 fixture-pollution class: phantom docs/evidence/ dirs and
  # traffic.jsonl rows written by bats runs). Must name an existing directory.
  if [ -n "${AGENTOPS_REPO_ROOT:-}" ] && [ -d "${AGENTOPS_REPO_ROOT}" ]; then
    printf '%s
' "${AGENTOPS_REPO_ROOT}"
    return 0
  fi
  # BASH_SOURCE[0] inside a function names the file the function was DEFINED
  # in — this library — regardless of who calls it.
  # shellcheck disable=SC1007
  lib_dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if ! root="$(
    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
    git -C "$lib_dir" rev-parse --show-toplevel 2>/dev/null
  )" || [ -z "$root" ]; then
    # shellcheck disable=SC1007
    root="$(CDPATH= cd "$lib_dir/../.." && pwd)"
  fi
  printf '%s\n' "$root"
}

# scrub_git_env → unset git's hook-injected discovery env in the CURRENT
# shell. For scripts that go on to run many git commands (or spawn processes
# — e.g. `go test` — that themselves run git): after cd-ing to the resolved
# repo root, discovery from cwd is correct and the leaked hook env is only a
# liability (it can point child git calls at the wrong repo or, combined with
# a poisoned core.bare, break them outright).
scrub_git_env() {
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
}

fi

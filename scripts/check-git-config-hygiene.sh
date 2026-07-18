#!/usr/bin/env bash
# check-git-config-hygiene.sh — fail-closed guard against shared-repo git
# config poisoning (bead age-gate-scripts-worktree-gitdir-p62wo, gremlins 2+3).
#
# WHY: git hooks export GIT_DIR. Any child process that inherits that env and
# runs repo-mutating git commands "scoped" only by cwd/-C/cmd.Dir actually
# writes THE SHARED CONFIG of the leaked repo (empirically verified):
#   * `git init --bare` (no directory argument) re-initializes $GIT_DIR and
#     sets core.bare=true in the shared config — which breaks every git
#     operation in every linked worktree (observed 3x on 2026-07-18);
#   * `git -C <tmpdir> config user.name Test` writes user.name=Test into the
#     shared config, mis-authoring every subsequent commit/rebase (the
#     Test/test@test.com and pokki-deploy/v@v.com identity traps).
# This gate makes the poisoning loud at the next push instead of latent.
#
# WHAT (fail = exit 1, crisp message + repair command):
#   1. effective core.bare=true on a non-bare checkout;
#   2. effective user.name/user.email matching a known test/fixture identity:
#      Test / test@test.com / test@example.com / factory@example.invalid /
#      pokki-deploy / v@v.com.
#   INFO only (never fails): effective user.email differing from the repo's
#   dominant recent author.
#
# Usage:
#   bash scripts/check-git-config-hygiene.sh              # check this repo
#   bash scripts/check-git-config-hygiene.sh <repo-dir>   # check another repo
#   bash scripts/check-git-config-hygiene.sh --self-test  # prove FAIL on a
#                                                         # poisoned fixture
#
# Exit codes: 0 clean / self-test passed; 1 hygiene violation or self-test
# failure; 127 missing git.

# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd git

PROG="check-git-config-hygiene"

note() { printf '[%s] %s\n' "$PROG" "$*"; }

# effective_cfg REPO KEY → the value git resolves for KEY from inside REPO,
# with hook-injected discovery env scrubbed so we inspect REPO itself, not a
# leaked GIT_DIR target. Empty when unset.
effective_cfg() {
  local repo="$1" key="$2"
  (
    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
    git -C "$repo" config --get "$key" 2>/dev/null
  ) || true
}

# common_config_path REPO → the shared config file the poisoning lands in
# (worktree-aware). Falls back to <repo>/.git/config when git itself is too
# broken to answer (e.g. core.bare=true already poisoned).
common_config_path() {
  local repo="$1" common
  common="$(
    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
    git -C "$repo" rev-parse --path-format=absolute --git-common-dir 2>/dev/null
  )" || common=""
  if [ -n "$common" ]; then
    printf '%s/config\n' "$common"
  else
    printf '%s/.git/config\n' "$repo"
  fi
}

# check_repo REPO → run all hygiene checks against REPO. Returns 1 on any
# blocking violation.
check_repo() {
  local repo="$1" rc=0 cfg bare name email
  cfg="$(common_config_path "$repo")"

  # 1. core.bare poisoning. Direct file read first (a poisoned repo can make
  #    `git -C` itself fail), effective-config read as backup.
  bare="$(git config --file "$cfg" --get core.bare 2>/dev/null || true)"
  [ -n "$bare" ] || bare="$(effective_cfg "$repo" core.bare)"
  if [ "$bare" = "true" ]; then
    note "FAIL: core.bare=true in $cfg — this is a checkout, not a bare repo; every linked-worktree git op will fail."
    note "  Likely writer: a no-dir-arg 'git init --bare' run with a hook-leaked GIT_DIR."
    note "  repair: git config --file '$cfg' core.bare false"
    rc=1
  fi

  # 2. Known test/fixture identities in the effective config.
  name="$(effective_cfg "$repo" user.name)"
  email="$(effective_cfg "$repo" user.email)"
  case "$name" in
    Test|pokki-deploy)
      note "FAIL: user.name='$name' is a known test/fixture identity leaked into the effective git config."
      note "  repair: git config --file '$cfg' --unset user.name   (then verify: git -C '$repo' config user.name)"
      rc=1
      ;;
  esac
  case "$email" in
    test@test.com|test@example.com|factory@example.invalid|v@v.com)
      note "FAIL: user.email='$email' is a known test/fixture identity leaked into the effective git config."
      note "  repair: git config --file '$cfg' --unset user.email   (then verify: git -C '$repo' config user.email)"
      rc=1
      ;;
  esac

  # 3. INFO only: effective identity vs the repo's dominant recent author.
  #    Never fails — multiple legit identities exist (bushido, worktrees).
  if [ -n "$email" ] && [ "$rc" -eq 0 ]; then
    local dominant
    dominant="$(
      unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
      git -C "$repo" log -n 200 --no-merges --format='%ae' 2>/dev/null |
        sort | uniq -c | sort -rn | head -n1 | awk '{print $2}'
    )" || dominant=""
    if [ -n "$dominant" ] && [ "$email" != "$dominant" ]; then
      note "INFO: effective user.email='$email' differs from dominant recent author '$dominant' (informational only)."
    fi
  fi

  return "$rc"
}

self_test() {
  local fixdir rc=0
  with_tmpdir fixdir git-config-hygiene-selftest

  # Poisoned fixture: core.bare=true + Test identity in the repo config.
  git -C "$fixdir" init -q
  git -C "$fixdir" config core.bare true
  git -C "$fixdir" config user.name Test
  git -C "$fixdir" config user.email test@test.com
  if check_repo "$fixdir" >/dev/null 2>&1; then
    note "SELF-TEST FAIL: poisoned fixture (core.bare=true + Test identity) was NOT flagged"
    rc=1
  else
    note "self-test: poisoned fixture correctly flagged"
  fi
  # Repair the fixture the way the FAIL message instructs, then it must pass.
  git config --file "$fixdir/.git/config" core.bare false
  git config --file "$fixdir/.git/config" --unset user.name
  git config --file "$fixdir/.git/config" --unset user.email
  if check_repo "$fixdir" >/dev/null 2>&1; then
    note "self-test: repaired fixture correctly passes"
  else
    note "SELF-TEST FAIL: repaired fixture still flagged"
    rc=1
  fi

  if [ "$rc" -eq 0 ]; then
    note "PASS: self-test"
  fi
  return "$rc"
}

case "${1:-}" in
  --self-test)
    self_test
    exit "$?"
    ;;
  -h|--help)
    sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *)
    target="${1:-$REPO_ROOT}"
    if check_repo "$target"; then
      note "OK: git config hygiene clean for $target"
      exit 0
    fi
    exit 1
    ;;
esac

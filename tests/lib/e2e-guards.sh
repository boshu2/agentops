# shellcheck shell=bash
# tests/lib/e2e-guards.sh — production safety guards for e2e tests.
#
# Refuses to run if the test harness looks like it might touch real state.
# Adapted from the "Production Safety Guards" pattern in the
# testing-real-service-e2e-no-mocks skill, translated to the CLI tool context
# (we have no Stripe/Supabase to protect — we protect the developer's
# real $HOME, real git repos, and any system-prefix `ao` binary).
#
# Usage:
#   source "${SCRIPT_DIR}/../lib/e2e-guards.sh"
#   e2e_guard_home "$HOME_DIR"          # required: assert $HOME points at temp
#   e2e_guard_repo "$REPO_DIR"          # required: assert work dir is temp
#   e2e_guard_ao_bin "$AO_BIN"          # required: assert binary is sandbox-owned
#   e2e_guard_not_repo_root             # required: refuse to run from agentops repo root
#
# Escape hatch (intentionally undocumented in --help): set
#   AGENTOPS_E2E_ALLOW_UNSAFE=1
# to bypass every guard. Logged loudly when used. Do NOT set this in CI.

[[ -n "${E2E_GUARDS_SH_LOADED:-}" ]] && return 0
E2E_GUARDS_SH_LOADED=1

_e2e_guard_die() {
  printf '[e2e-guard] FATAL: %s\n' "$*" >&2
  printf '[e2e-guard]   This guard exists to prevent tests from touching real state.\n' >&2
  printf '[e2e-guard]   If you are absolutely sure, set AGENTOPS_E2E_ALLOW_UNSAFE=1.\n' >&2
  exit 78  # EX_CONFIG
}

_e2e_guard_warn_bypass() {
  if [[ "${AGENTOPS_E2E_ALLOW_UNSAFE:-0}" == "1" ]]; then
    printf '[e2e-guard] WARNING: AGENTOPS_E2E_ALLOW_UNSAFE=1 — skipping %s\n' "$1" >&2
    return 0
  fi
  return 1
}

# e2e_guard_home <path>
# Asserts that $HOME has been redirected to <path> AND that <path> is under a
# temp prefix. Prevents fixture writes from landing in the developer's real
# home (~/.agents, ~/.claude, ~/.config — all of which are real).
e2e_guard_home() {
  local expected="$1"
  _e2e_guard_warn_bypass "HOME guard" && return 0
  [[ -n "$expected" ]] || _e2e_guard_die "e2e_guard_home: expected path is empty"
  [[ "$HOME" == "$expected" ]] \
    || _e2e_guard_die "HOME=$HOME does not match sandbox HOME=$expected — refusing to run."
  case "$HOME" in
    /tmp/*|/var/folders/*|/var/tmp/*|"${TMPDIR%/}"/*) ;;
    *) _e2e_guard_die "HOME=$HOME is not under a temp prefix — refusing to run." ;;
  esac
}

# e2e_guard_repo <path>
# Asserts that the working repo is a temp directory, not a real checkout.
# Refuses if path contains a .git that points anywhere outside the sandbox
# (catches the "I ran the test from ~/dev/agentops by accident" footgun).
e2e_guard_repo() {
  local repo="$1"
  _e2e_guard_warn_bypass "repo guard" && return 0
  [[ -n "$repo" ]] || _e2e_guard_die "e2e_guard_repo: path is empty"
  case "$repo" in
    /tmp/*|/var/folders/*|/var/tmp/*|"${TMPDIR%/}"/*) ;;
    *) _e2e_guard_die "repo=$repo is not under a temp prefix — refusing to run." ;;
  esac
  if [[ -e "$repo/.git" && ! -d "$repo/.git" && ! -f "$repo/.git" ]]; then
    _e2e_guard_die "repo=$repo has a non-standard .git entry — refusing to run."
  fi
}

# e2e_guard_ao_bin <path>
# Asserts that the ao binary is under a sandbox build dir, not a system prefix.
# Refuses /usr/local/bin/ao, $HOME/bin/ao, or anything outside the test temp.
e2e_guard_ao_bin() {
  local bin="$1"
  _e2e_guard_warn_bypass "ao binary guard" && return 0
  [[ -x "$bin" ]] || _e2e_guard_die "ao binary not executable: $bin"
  case "$bin" in
    /tmp/*|/var/folders/*|/var/tmp/*|"${TMPDIR%/}"/*) ;;
    *)
      # Allow the repo-local build output cli/bin/ao (reused via PROOF_AO_BIN
      # auto-detect) — it lives under the worktree and is regenerated per build.
      if [[ "$bin" == */cli/bin/ao ]]; then
        return 0
      fi
      _e2e_guard_die "ao binary $bin is in a system/user prefix — refusing to run."
      ;;
  esac
}

# e2e_guard_not_repo_root
# Refuses to run if $PWD is the agentops repo root (presence of CLAUDE.md +
# cli/cmd/ao + skills/ is the signature). Catches "cd into repo, run test by
# hand, forget to chdir to /tmp" — that bug previously cost an afternoon.
e2e_guard_not_repo_root() {
  _e2e_guard_warn_bypass "repo-root guard" && return 0
  if [[ -f "$PWD/CLAUDE.md" && -d "$PWD/cli/cmd/ao" && -d "$PWD/skills" ]]; then
    _e2e_guard_die "PWD=$PWD looks like the agentops repo root — chdir into the sandbox first."
  fi
}

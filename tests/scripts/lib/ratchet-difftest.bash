# shellcheck shell=bash
# tests/scripts/lib/ratchet-difftest.bash — differential parity harness for the
# ratchet-lib migrations (age-ratchet-lib-extraction-bv7d, pre-mortem FM2).
#
# A migration slice claims ZERO behavior change. "Exit code + the offender path
# appears somewhere" is not that claim — output ordering, stream routing
# (stdout vs stderr), locale collation, and usage-error shapes all regressed
# silently in at least one prior gate rewrite. This harness compares the OLD
# script (materialized from git, e.g. `git show HEAD:scripts/check-x.sh`) and
# the NEW script byte-for-byte on all three observable channels.
#
# Usage (from a bats test, inside the fixture repo):
#   source "$REPO_ROOT/tests/scripts/lib/ratchet-difftest.bash"
#   ratchet_difftest OLD_SCRIPT NEW_SCRIPT -- [args...]
#
# Compares, for `bash OLD args...` vs `bash NEW args...` run in the CURRENT
# working directory: exit code, stdout bytes, stderr bytes. On mismatch prints
# a labeled unified diff per divergent channel and returns 1.
#
# Locale: callers run the harness twice when the fixture demands it
# (LC_ALL=C and LC_ALL=en_US.UTF-8) — the harness honors the caller's env.
# Functions only — no top-level shell options.

# ratchet_difftest <old-script> <new-script> -- [args...]
ratchet_difftest() {
  local old="${1:?ratchet_difftest: old-script required}"
  local new="${2:?ratchet_difftest: new-script required}"
  shift 2
  [[ "${1:-}" == "--" ]] && shift
  local tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/ratchet-difftest.XXXXXX")" || return 2

  local old_rc=0 new_rc=0
  bash "$old" "$@" > "$tmp/old.out" 2> "$tmp/old.err" || old_rc=$?
  bash "$new" "$@" > "$tmp/new.out" 2> "$tmp/new.err" || new_rc=$?

  local rc=0
  if [[ "$old_rc" -ne "$new_rc" ]]; then
    echo "DIFFTEST MISMATCH exit-code: old=$old_rc new=$new_rc (args: $*)"
    rc=1
  fi
  if ! cmp -s "$tmp/old.out" "$tmp/new.out"; then
    echo "DIFFTEST MISMATCH stdout (args: $*):"
    diff -u --label old/stdout --label new/stdout "$tmp/old.out" "$tmp/new.out" || true
    rc=1
  fi
  if ! cmp -s "$tmp/old.err" "$tmp/new.err"; then
    echo "DIFFTEST MISMATCH stderr (args: $*):"
    diff -u --label old/stderr --label new/stderr "$tmp/old.err" "$tmp/new.err" || true
    rc=1
  fi
  rm -rf "$tmp"
  return "$rc"
}

# ratchet_difftest_materialize <git-ref> <repo-relative-path> <dest>
# Writes the OLD script content from git history to <dest> (the standard way a
# migration twin obtains its pre-migration comparator).
ratchet_difftest_materialize() {
  local ref="${1:?ratchet_difftest_materialize: git-ref required}"
  local path="${2:?ratchet_difftest_materialize: path required}"
  local dest="${3:?ratchet_difftest_materialize: dest required}"
  git show "$ref:$path" > "$dest" || return 1
  chmod +x "$dest" 2>/dev/null || true
  return 0
}

#!/usr/bin/env bash
# check-shell-exec-bits.sh — ADVISORY guard: the shell entry points this
# repository documents must actually be executable as documented.
#
# WHY: AGENTS.md tells a contributor to run `scripts/regen-all.sh --check` and
# `tests/run-all.sh`. On 2026-09-02 twenty-three tracked shebang-bearing shell
# files under scripts/ and tests/ carried index mode 100644, so
# `./scripts/regen-all.sh --check` answered "permission denied" — the
# documented command did not run at all. A missing exec bit is invisible to
# every other gate (shellcheck, the bats suites, and CI all invoke scripts
# through an explicit `bash`), so nothing caught it.
#
# WHAT (fail = exit 1, offenders listed with the repair command):
#   1. every tracked *.sh under scripts/ or tests/ whose FIRST LINE is a
#      shebang must have tracked mode 100755;
#   2. every tracked *.sh under scripts/ or tests/ WITHOUT a shebang must live
#      under a `lib/` directory — those are sourced libraries, correctly 644,
#      and `lib/` is the convention that says so.
#
# The tracked (index) mode is what a fresh clone gets, so that is what is
# checked; a working-tree-only `chmod` that was never staged is still broken
# for everyone else.
#
# Usage:
#   bash scripts/check-shell-exec-bits.sh              # check this repo
#   bash scripts/check-shell-exec-bits.sh <repo-dir>   # check another checkout
#
# Both the mode and the first line are read from the INDEX, never from the
# working tree: an unstaged edit must not be able to hide a broken indexed
# entry (and the index is what a fresh clone gets). Index entries with mode
# 120000 are symlinks; a symlinked *.sh is not an entry point, so they are
# skipped with a printed note rather than judged.
#
# Exit codes: 0 clean; 1 violations found; 2 enumeration failed (not a git
# repository, git error, or nothing to check); 127 missing git.

# shellcheck disable=SC1007
# shellcheck source=scripts/lib/preamble.sh
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd git

PROG="check-shell-exec-bits"

note() { printf '[%s] %s\n' "$PROG" "$*"; }

# in_lib_dir PATH → true when any directory component of PATH is exactly "lib".
in_lib_dir() {
  case "/$1" in
    */lib/*) return 0 ;;
    *) return 1 ;;
  esac
}

# check_repo REPO → 0 when every tracked shell file under scripts/ and tests/
# obeys both rules, 1 when a rule is broken, 2 when enumeration itself failed.
check_repo() {
  local repo="$1" rc=0 mode path first blob scanned=0
  local -a missing_exec=() stray_sourced=() symlinked=()
  local work index_z err_log

  # Enumeration runs through a CHECKED path. It used to sit behind a process
  # substitution, which discards git's exit status entirely: pointing the gate
  # at a non-repository printed git's fatal error, enumerated nothing, and
  # still reported OK with exit 0 — a gate that fails open is worse than no
  # gate. NUL-delimited so a path containing whitespace or a newline cannot
  # split a record.
  with_tmpdir work shell-exec-bits
  index_z="$work/index.z"
  err_log="$work/git.err"
  if ! git -C "$repo" ls-files -s -z > "$index_z" 2> "$err_log"; then
    note "FAIL: could not enumerate tracked files in $repo (git ls-files failed)"
    sed 's/^/  /' "$err_log" >&2 || true
    return 2
  fi

  while IFS= read -r -d '' line; do
    [ -n "$line" ] || continue
    # `git ls-files -s -z` → "<mode> <sha> <stage>\t<path>\0"
    mode="${line%% *}"
    path="${line#*$'\t'}"
    case "$path" in
      scripts/* | tests/*) ;;
      *) continue ;;
    esac
    case "$path" in
      *.sh) ;;
      *) continue ;;
    esac
    scanned=$((scanned + 1))

    # 120000 is a symlink entry. A symlinked *.sh is not an entry point of
    # this repository; judging its "shebang" would read the link target text.
    if [ "$mode" = "120000" ]; then
      symlinked+=("$path")
      continue
    fi

    # First line from the INDEX blob, not the working tree. No pipe into
    # `head`: under `set -o pipefail` the early reader would SIGPIPE git and
    # turn a readable blob into a spurious failure.
    if ! blob="$(git -C "$repo" cat-file -p ":$path" 2>/dev/null)"; then
      note "FAIL: could not read the indexed blob for $path"
      rc=1
      continue
    fi
    first="${blob%%$'\n'*}"

    case "$first" in
      '#!'*)
        if [ "$mode" != "100755" ]; then
          missing_exec+=("$mode $path")
        fi
        ;;
      *)
        if ! in_lib_dir "$path"; then
          stray_sourced+=("$mode $path")
        fi
        ;;
    esac
  done < "$index_z"

  # Fail closed on an empty population: "checked nothing" must never read as
  # "checked everything and it was fine".
  if [ "$scanned" -eq 0 ]; then
    note "FAIL: nothing enumerated — no tracked *.sh under scripts/ or tests/ in $repo"
    return 2
  fi

  if [ "${#symlinked[@]}" -gt 0 ]; then
    note "note: ${#symlinked[@]} symlinked *.sh entr(y|ies) skipped (a symlink is not an entry point):"
    printf '  %s\n' "${symlinked[@]}"
  fi

  if [ "${#missing_exec[@]}" -gt 0 ]; then
    note "FAIL: ${#missing_exec[@]} shebang-bearing shell file(s) are not tracked executable:"
    printf '  %s\n' "${missing_exec[@]}"
    note "  repair: git update-index --chmod=+x <path>   (and chmod +x <path> in the working tree)"
    rc=1
  fi

  if [ "${#stray_sourced[@]}" -gt 0 ]; then
    note "FAIL: ${#stray_sourced[@]} shell file(s) without a shebang live outside a lib/ directory:"
    printf '  %s\n' "${stray_sourced[@]}"
    note "  repair: add a '#!/usr/bin/env bash' shebang and the exec bit, or move the sourced library under a lib/ directory"
    rc=1
  fi

  if [ "$rc" -eq 0 ]; then
    note "OK: $scanned tracked *.sh under scripts/ and tests/ are executable as documented ($repo)"
  fi

  return "$rc"
}

usage() {
  sed -n '2,35p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  -h | --help)
    usage
    exit 0
    ;;
  *)
    target="${1:-$REPO_ROOT}"
    # The exit status is passed through verbatim: 1 is "a rule was broken",
    # 2 is "the check could not run" — collapsing them would hide the second.
    check_repo "$target" || exit "$?"
    exit 0
    ;;
esac

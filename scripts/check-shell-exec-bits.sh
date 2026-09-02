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
# Exit codes: 0 clean; 1 violations found; 127 missing git.

# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

require_cmd git

PROG="check-shell-exec-bits"

note() { printf '[%s] %s\n' "$PROG" "$*"; }

usage() {
  sed -n '2,28p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# in_lib_dir PATH → true when any directory component of PATH is exactly "lib".
in_lib_dir() {
  case "/$1" in
    */lib/*) return 0 ;;
    *) return 1 ;;
  esac
}

# check_repo REPO → 0 when every tracked shell file under scripts/ and tests/
# obeys both rules, 1 otherwise (offenders printed).
check_repo() {
  local repo="$1" rc=0 mode path first
  local -a missing_exec=() stray_sourced=()

  while IFS= read -r line; do
    [ -n "$line" ] || continue
    # `git ls-files -s` → "<mode> <sha> <stage>\t<path>"
    mode="${line%% *}"
    path="${line#*$'\t'}"
    first=""
    if [ -r "$repo/$path" ]; then
      IFS= read -r first < "$repo/$path" || true
    else
      # Deleted from the working tree but still tracked: read the blob.
      first="$(git -C "$repo" show ":$path" 2>/dev/null | head -n1 || true)"
    fi

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
  done < <(
    git -C "$repo" ls-files -s |
      awk -F'\t' '$2 ~ /^(scripts|tests)\// && $2 ~ /\.sh$/ {print}'
  )

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

  return "$rc"
}

case "${1:-}" in
  -h | --help)
    usage
    exit 0
    ;;
  *)
    target="${1:-$REPO_ROOT}"
    if check_repo "$target"; then
      note "OK: shell entry points under scripts/ and tests/ are executable as documented ($target)"
      exit 0
    fi
    exit 1
    ;;
esac

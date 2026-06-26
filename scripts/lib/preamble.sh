#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/preamble.sh — sourced library: strict mode + REPO_ROOT + portable stat/find.
#
# Source it at the top of a script (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
#
# Centralizes two recurring portability hazards (age-0dq9.1 / recon P6a):
#   * macOS `find` is frequently the interactive `bfs` shim (which supports
#     `-printf`) while a script's real /usr/bin/find does NOT — so a find call
#     passes when typed by hand and fails when the script actually runs.
#   * mtime: `stat -f %m` (BSD/macOS) vs `stat -c %Y` (GNU/Linux).
#
# Helpers are additive and behavior-preserving; callers are migrated in P6b
# (age-0dq9.2), not here.

# Strict mode is the point of sourcing this.
set -euo pipefail

# REPO_ROOT: the root of the repo this LIBRARY lives in, independent of the
# caller's CWD. We anchor `git rev-parse` at the lib's OWN directory (`git -C`)
# so a script run from inside a *different* git checkout can't hijack REPO_ROOT
# to that other tree. Fall back to the lib's fixed position
# (<root>/scripts/lib/preamble.sh → two dirs up) when not in a git checkout at
# all (e.g. an extracted release tarball).
# `CDPATH=` below is an intentional env-prefix (clears CDPATH for that one cd so
# a caller's CDPATH can't hijack a relative path), not a botched assignment.
# shellcheck disable=SC1007
_preamble_dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! REPO_ROOT="$(git -C "$_preamble_dir" rev-parse --show-toplevel 2>/dev/null)"; then
  # shellcheck disable=SC1007
  REPO_ROOT="$(CDPATH= cd "$_preamble_dir/../.." && pwd)"
fi
unset _preamble_dir
export REPO_ROOT

# portable_mtime FILE → epoch-seconds mtime. Returns non-zero if neither form
# works (e.g. file gone).
#
# GNU/Linux `stat -c %Y` FIRST, then BSD/macOS `stat -f %m` — the order is
# load-bearing and the reverse is a real cross-platform bug: on GNU stat, `-f`
# is `--file-system` (NOT a format flag), so `stat -f %m FILE` does NOT fail
# cleanly — it returns filesystem data instead of the mtime. Probing the GNU
# form first is safe because BSD/macOS stat has no `-c` and exits non-zero on it,
# so the fallback to `stat -f %m` fires correctly.
portable_mtime() {
  stat -c %Y "$1" 2>/dev/null || stat -f %m "$1"
}

# portable_find DIR [find-args...] → run the REAL system find, never the
# interactive `bfs` shim, and callers must not pass `-printf` (BSD find lacks
# it). Prefer /usr/bin/find; fall back to whatever `find` PATH resolves only if
# the canonical path is missing.
portable_find() {
  local find_bin=/usr/bin/find
  [ -x "$find_bin" ] || find_bin="$(command -v find)"
  "$find_bin" "$@"
}

# newest_by_mtime FILE... → the single newest existing path by mtime (empty if
# none exist). Uses a global `sort -n | tail`, NOT `find -exec ls -t | head`
# (which sorts per-exec-batch and can SIGPIPE the producer — see memory
# macos-bats-portability-sweep).
newest_by_mtime() {
  local f mt
  for f in "$@"; do
    [ -e "$f" ] || continue
    mt="$(portable_mtime "$f" 2>/dev/null || true)"
    # Explicit `if` (not `mt && printf`) so a falsey test never trips `set -e`.
    if [ -n "$mt" ]; then
      printf '%s\t%s\n' "$mt" "$f"
    fi
  done | sort -n | tail -n1 | cut -f2-
}

# newest_in_dir DIR NAME_GLOB → newest file under DIR matching NAME_GLOB by
# mtime (empty if none). Built on portable_find + portable_mtime, so it works
# identically on macOS and Linux and never relies on `-printf`.
newest_in_dir() {
  local dir="$1" glob="$2" f mt
  [ -d "$dir" ] || return 0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    mt="$(portable_mtime "$f" 2>/dev/null || true)"
    if [ -n "$mt" ]; then
      printf '%s\t%s\n' "$mt" "$f"
    fi
  done < <(portable_find "$dir" -type f -name "$glob" 2>/dev/null) | sort -n | tail -n1 | cut -f2-
}

#!/usr/bin/env bash
# install-workflows.sh — install repo-canonical Claude workflows into
# $HOME/.claude/workflows as symlinks (ag-wi9w1).
#
# Usage: bash scripts/install-workflows.sh [name.js ...]
#   No args: install every repo-tracked .claude/workflows/*.js.
#   With args: install only the named workflows (arg-scoped).
#
# Semantics (per the canonicalize-bdd-foundry-workflow spec, C2):
#   - repo root resolved from cwd git; dest dir $HOME/.claude/workflows (mkdir -p)
#   - dest is a symlink (incl. dangling)  -> ln -sfn to the repo canonical (idempotent)
#   - dest is a byte-equal regular file   -> replace with the symlink
#   - dest is a divergent regular file    -> cp -p backup to
#       <name>.pre-canonicalize-<UTC-ts> (path printed), then ln -sfn
#   - dest absent                          -> ln -sfn
# Touches ONLY $HOME; never writes into the repo. Exits non-zero on any real failure.
set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
src_dir="$repo_root/.claude/workflows"
dest_dir="$HOME/.claude/workflows"
mkdir -p "$dest_dir"

if [ "$#" -eq 0 ]; then
  for f in "$src_dir"/*.js; do
    [ -e "$f" ] || continue
    set -- "$@" "$(basename "$f")"
  done
fi

status=0
for name in "$@"; do
  src="$src_dir/$name"
  dest="$dest_dir/$name"
  if [ ! -f "$src" ]; then
    echo "ERROR: no such workflow in repo: $name ($src)" >&2
    status=1
    continue
  fi
  if [ -L "$dest" ]; then
    ln -sfn "$src" "$dest"
    echo "installed (symlink refreshed): $dest -> $src"
  elif [ -f "$dest" ]; then
    if cmp -s "$dest" "$src"; then
      ln -sfn "$src" "$dest"
      echo "installed (byte-equal copy replaced with symlink): $dest -> $src"
    else
      backup="$dest.pre-canonicalize-$(date -u +%Y%m%dT%H%M%SZ)"
      cp -p "$dest" "$backup"
      echo "backup of divergent local file: $backup"
      ln -sfn "$src" "$dest"
      echo "installed (divergent copy backed up, then symlinked): $dest -> $src"
    fi
  elif [ -e "$dest" ]; then
    echo "ERROR: $dest exists and is neither a regular file nor a symlink; refusing" >&2
    status=1
  else
    ln -sfn "$src" "$dest"
    echo "installed (new symlink): $dest -> $src"
  fi
done
exit "$status"

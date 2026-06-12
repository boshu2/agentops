#!/usr/bin/env bash
# check-workflow-drift.sh — workflow install freshness/resolution check (ag-wi9w1).
#
# Blocking set: bdd-foundry.js
#   - absent entirely               -> SKIP line, exit 0 (clean machines/CI stay green)
#   - dangling symlink              -> exit 1 naming the offending path
#   - symlink                       -> must realpath-resolve to the repo canonical, else exit 1
#   - regular file                  -> cmp -s against the repo canonical, else exit 1
# Report-only set: every other repo-tracked .claude/workflows/*.js
#   (today: bead-crank.js, operating-loop.js) — the same comparison, but divergence
#   emits 'DRIFT-REPORT: <name> ...' to stdout and NEVER affects the exit code.
#
# Repo root from cwd git; installed dir from $HOME. No hardcoded user paths.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
canon_dir="$repo_root/.claude/workflows"
inst_dir="$HOME/.claude/workflows"
blocking_name="bdd-foundry.js"

resolve() { python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$1"; }

# compare_install <name>: echoes a failure detail and returns 1 on drift/dangle;
# returns 0 when absent or faithful.
compare_install() {
  name="$1"
  inst="$inst_dir/$name"
  canon="$canon_dir/$name"
  if [ ! -e "$inst" ] && [ ! -L "$inst" ]; then
    return 0
  fi
  if [ -L "$inst" ]; then
    if [ ! -e "$inst" ]; then
      echo "$name installed symlink is dangling: $inst -> $(readlink "$inst")"
      return 1
    fi
    if [ "$(resolve "$inst")" != "$(resolve "$canon")" ]; then
      echo "$name installed symlink resolves to $(resolve "$inst"), not the repo canonical $canon"
      return 1
    fi
  elif ! cmp -s "$inst" "$canon"; then
    echo "$name installed copy bytes differ from the repo canonical: $inst vs $canon"
    return 1
  fi
  return 0
}

# Blocking: bdd-foundry.js
if [ ! -e "$inst_dir/$blocking_name" ] && [ ! -L "$inst_dir/$blocking_name" ]; then
  echo "SKIP: $blocking_name not installed ($inst_dir/$blocking_name absent)"
else
  if ! detail="$(compare_install "$blocking_name")"; then
    echo "FAIL: $detail"
    exit 1
  fi
  echo "OK: $blocking_name installed and faithful to the repo canonical"
fi

# Report-only siblings: every other repo-tracked workflow.
while IFS= read -r tracked; do
  name="$(basename "$tracked")"
  [ "$name" = "$blocking_name" ] && continue
  if ! detail="$(compare_install "$name")"; then
    echo "DRIFT-REPORT: $detail"
  fi
done < <(git -C "$repo_root" ls-files '.claude/workflows/*.js')

exit 0

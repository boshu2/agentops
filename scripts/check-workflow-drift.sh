#!/usr/bin/env bash
# check-workflow-drift.sh — workflow install freshness/resolution check (ag-wi9w1).
#
# Blocking set: retired workflow names. A stale installed copy can still run
# the former behavior, so it must follow the fail-closed source tombstone.
#   - absent entirely               -> SKIP line, exit 0 (clean machines/CI stay green)
#   - dangling symlink              -> exit 1 naming the offending path
#   - symlink                       -> must realpath-resolve to the repo canonical, OR (cross-
#                                      checkout: gate run from a worktree) resolve to a target
#                                      byte-equal to this repo's canonical; else exit 1
#   - regular file                  -> cmp -s against the repo canonical, else exit 1
# Report-only set: every active repo-tracked workflows/*.js — the same
#   comparison, but divergence emits 'DRIFT-REPORT: <name> ...' to stdout and
#   NEVER affects the exit code.
#
# Check both the legacy global install and the current checkout's project-local
# install. No hardcoded user paths or writes to either install location.
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
canon_dir="$repo_root/workflows"
install_dirs=("$HOME/.claude/workflows" "$repo_root/.claude/workflows")
install_labels=("global legacy" "project-local")
blocking_names=(bdd-foundry.js ship-beads.js bead-crank.js operating-loop.js)

resolve() { python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$1"; }

# compare_install <name> <install-dir>: echoes a failure detail and returns 1 on drift/dangle;
# returns 0 when absent or faithful.
compare_install() {
  local name="$1" inst="$2/$1" canon="$canon_dir/$1"
  local resolved_inst resolved_canon
  if [ ! -e "$inst" ] && [ ! -L "$inst" ]; then
    return 0
  fi
  if [ -L "$inst" ]; then
    if [ ! -e "$inst" ]; then
      echo "$name installed symlink is dangling: $inst -> $(readlink "$inst")"
      return 1
    fi
    if ! resolved_inst="$(resolve "$inst")" || [ -z "$resolved_inst" ]; then
      echo "$name installed symlink resolution failed: $inst"
      return 1
    fi
    if ! resolved_canon="$(resolve "$canon")" || [ -z "$resolved_canon" ]; then
      echo "$name canonical path resolution failed: $canon"
      return 1
    fi
    if [ "$resolved_inst" != "$resolved_canon" ]; then
      # Cross-checkout tolerance: an installed symlink can target the main checkout,
      # so a gate run from a worktree sees a different realpath. Byte-equality with THIS repo's canonical
      # is the real invariant — fail only when the resolved content has drifted.
      if ! cmp -s "$resolved_inst" "$canon"; then
        echo "$name installed symlink at $inst resolves to $resolved_inst, not the repo canonical $canon, and its bytes differ"
        return 1
      fi
    fi
  elif ! cmp -s "$inst" "$canon"; then
    echo "$name installed copy bytes differ from the repo canonical: $inst vs $canon"
    return 1
  fi
  return 0
}

# Blocking: every retired name at both install locations.
failed=0
for install_index in "${!install_dirs[@]}"; do
  inst_dir="${install_dirs[$install_index]}"
  install_label="${install_labels[$install_index]}"
  for blocking_name in "${blocking_names[@]}"; do
    if [ ! -e "$inst_dir/$blocking_name" ] && [ ! -L "$inst_dir/$blocking_name" ]; then
      echo "SKIP: $blocking_name not installed ($inst_dir/$blocking_name absent)"
    elif ! detail="$(compare_install "$blocking_name" "$inst_dir")"; then
      echo "FAIL: $detail ($install_label: $inst_dir/$blocking_name)"
      failed=1
    else
      echo "OK: $blocking_name installed and faithful to the repo canonical ($install_label: $inst_dir/$blocking_name)"
    fi
  done
done

# Report-only siblings: active repo-tracked workflows.
while IFS= read -r tracked; do
  name="$(basename "$tracked")"
  case "$name" in
    bdd-foundry.js|ship-beads.js|bead-crank.js|operating-loop.js) continue ;;
  esac
  for install_index in "${!install_dirs[@]}"; do
    inst_dir="${install_dirs[$install_index]}"
    install_label="${install_labels[$install_index]}"
    if ! detail="$(compare_install "$name" "$inst_dir")"; then
      echo "DRIFT-REPORT: $detail ($install_label: $inst_dir/$name)"
    fi
  done
done < <(git -C "$repo_root" ls-files 'workflows/*.js')

exit "$failed"

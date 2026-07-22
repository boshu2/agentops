#!/usr/bin/env bash
set -euo pipefail

die() { printf 'agentops-worktree: %s\n' "$*" >&2; exit 1; }
usage() { printf '%s\n' 'Usage: worktree.sh prepare --repo PATH --root PATH --bead ID [--base-ref NAME]'; }
[ "${1:-}" = "prepare" ] || { usage; exit 2; }
shift
repo=""; root=""; bead=""; base_ref="main"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo) repo="${2:?--repo requires a path}"; shift 2 ;;
    --root) root="${2:?--root requires a path}"; shift 2 ;;
    --bead) bead="${2:?--bead requires an id}"; shift 2 ;;
    --base-ref) base_ref="${2:?--base-ref requires a name}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$repo" ] && [ -n "$root" ] && [ -n "$bead" ] || die "repo, root, and bead are required"
[[ "$bead" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || die "unsafe bead id"
[[ "$base_ref" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$ ]] || die "unsafe base ref"
repo="$(python3 - "$repo" <<'PY'
import os,sys
print(os.path.realpath(sys.argv[1]))
PY
)"
root="$(python3 - "$root" <<'PY'
import os,sys
print(os.path.realpath(os.path.abspath(sys.argv[1])))
PY
)"
git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "repo is not a Git worktree"
case "$root/" in "$repo/"*) die "worker root must be outside the primary checkout" ;; esac
key="$(printf '%s' "$bead" | tr -c 'A-Za-z0-9._-' '-')"
branch="gc/$key"
target="$root/$key"
mkdir -p "$root"
if [ -e "$target" ]; then
  [ -d "$target/.git" ] || git -C "$target" rev-parse --git-dir >/dev/null 2>&1 || die "existing target is not a Git worktree"
  [ "$(git -C "$target" branch --show-current)" = "$branch" ] || die "existing target uses another branch"
else
  git -C "$repo" fetch origin "$base_ref" --quiet
  if git -C "$repo" show-ref --verify --quiet "refs/heads/$branch"; then
    git -C "$repo" worktree add "$target" "$branch" --quiet
  else
    git -C "$repo" worktree add -b "$branch" "$target" "origin/$base_ref" --quiet
  fi
fi
[ -z "$(git -C "$target" status --porcelain)" ] || die "worker worktree is not clean"
python3 - "$target" "$branch" "$bead" <<'PY'
import json,sys
print(json.dumps({"bead":sys.argv[3],"branch":sys.argv[2],"worktree":sys.argv[1]},sort_keys=True,separators=(",",":")))
PY

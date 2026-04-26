#!/usr/bin/env bash
set -euo pipefail

repo_root="."
path_list=""

usage() {
  cat <<'USAGE'
Usage: scripts/check-casefold-path-collisions.sh [--repo-root <path>] [--path-list <file>]

Fails when two tracked paths differ only by case. This protects contributors on
case-insensitive filesystems from checkouts that cannot faithfully represent the
Git tree.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for --repo-root" >&2
        usage >&2
        exit 2
      fi
      repo_root="${2:-}"
      shift 2
      ;;
    --path-list)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for --path-list" >&2
        usage >&2
        exit 2
      fi
      path_list="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -n "$path_list" ]]; then
  if [[ ! -f "$path_list" ]]; then
    echo "FAIL: path list not found: $path_list" >&2
    exit 1
  fi
  input_cmd=(cat "$path_list")
else
  if [[ ! -d "$repo_root" ]]; then
    echo "FAIL: repository root not found: $repo_root" >&2
    exit 1
  fi
  input_cmd=(git -C "$repo_root" ls-files)
fi

if "${input_cmd[@]}" | awk '
  NF == 0 { next }
  {
    key = tolower($0)
    if (!(key in paths)) {
      paths[key] = $0
      next
    }
    split(paths[key], existing, "\n")
    found = 0
    for (i in existing) {
      if (existing[i] == $0) {
        found = 1
        break
      }
    }
    if (!found) {
      paths[key] = paths[key] "\n" $0
    }
  }
  END {
    collisions = 0
    for (key in paths) {
      n = split(paths[key], grouped, "\n")
      if (n > 1) {
        if (collisions == 0) {
          print "FAIL: tracked paths collide after case folding"
        }
        collisions++
        print ""
        print "Collision group:"
        for (i = 1; i <= n; i++) {
          print "  - " grouped[i]
        }
      }
    }
    exit collisions > 0 ? 1 : 0
  }
'; then
  echo "PASS: no case-folded tracked path collisions"
else
  exit 1
fi

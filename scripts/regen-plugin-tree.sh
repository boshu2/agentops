#!/usr/bin/env bash
# Regenerate (or --check) plugin/, the Claude Code plugin folder.
#
# Why: the plugin marketplace reads and screens only the plugin folder. When the
# plugin folder was the repository root, the screen walked ~2,800 files (Go CLI,
# evals, docs, CI) and raised holds for files the plugin never loads. plugin/ is
# a generated projection of the canonical component trees, so the canonical
# skills/ (which scripts, tests and `ao skills link` read) stays where it is.
#
# Ownership inside plugin/:
#   generated here  plugin/{skills,hooks,agents,workflows}/  from the same-named
#                   canonical trees at the repo root. Do not edit them by hand.
#   hand-owned      plugin/.claude-plugin/plugin.json  (the plugin manifest; the
#                   release version lives here) and plugin/.claude-plugin/icon.png
#                   (directory listing icon, rendered from docs/assets/logo.svg).
#   hand-owned      plugin/README.md: the portal shows the plugin folder's README.
#                   It stays short, points at the repository README, and carries
#                   no fetch-and-run commands and no image references.
# Not shipped: bin/ (a plugin bin/ goes on users' PATH and blocks claude.ai and
# Cowork installs), developer-only tests/ folders, *.bats, Python caches, and
# anything git ignores. Copying follows git's file list (tracked plus untracked,
# not ignored), so local scratch never ships.
#
# Limits enforced on the whole plugin/ folder (portal rules): no symlinks, at most
# 512 files, non-image/font files at most 256 KiB, and only text, PNG/JPEG/GIF/
# WebP, SVG or font files. A violation prints the offending paths and exits 1.
#
# Usage: scripts/regen-plugin-tree.sh [--check]
#   (no flag)  rewrite the generated folders, then enforce the limits
#   --check    exit 1 with a drift summary when plugin/ is stale; writes nothing
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PLUGIN_DIR="plugin"
COMPONENTS=(skills hooks agents workflows)
MAX_FILES=512
MAX_BYTES=$((256 * 1024))

mode=regen
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) mode=check ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "usage: $0 [--check]" >&2; exit 2 ;;
  esac
  shift
done

die() { echo "regen-plugin-tree: $*" >&2; exit 1; }

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "must run inside the git checkout"
[[ -f "$PLUGIN_DIR/.claude-plugin/plugin.json" ]] || die "missing $PLUGIN_DIR/.claude-plugin/plugin.json (hand-owned manifest)"

stage="$(mktemp -d "${TMPDIR:-/tmp}/regen-plugin-tree.XXXXXX")"
trap 'rm -rf "$stage"' EXIT

# excluded PATH -> 0 when PATH must not ship.
excluded() {
  case "$1" in
    */tests/*|*.bats|*/__pycache__/*|*.pyc|*.pyo|*/.DS_Store|.DS_Store) return 0 ;;
  esac
  return 1
}

# Stage every shippable file of each component into $stage/<component>/.
for component in "${COMPONENTS[@]}"; do
  [[ -d "$component" ]] || die "canonical component folder missing: $component/"
  mkdir -p "$stage/$component"
  while IFS= read -r -d '' path; do
    excluded "$path" && continue
    if [[ -L "$path" ]]; then
      die "refusing symlink in canonical tree: $path"
    fi
    # Tracked but deleted in the working tree: skip.
    [[ -f "$path" ]] || continue
    mkdir -p "$stage/$(dirname "$path")"
    cp -p "$path" "$stage/$path"
  done < <(git ls-files -z --cached --others --exclude-standard -- "$component" | sort -zu)
done

# check_limits DIR LABEL: enforce the portal limits on every file under DIR.
check_limits() {
  local dir="$1" bad=0 count path size
  local links
  links="$(find "$dir" -type l)"
  if [[ -n "$links" ]]; then
    echo "regen-plugin-tree: symlinks are not allowed in the plugin folder:" >&2
    printf '%s\n' "$links" | sed -e "s#^$stage/##" -e 's/^/  /' >&2
    bad=1
  fi
  count="$(find "$dir" -type f | wc -l | tr -d ' ')"
  if (( count > MAX_FILES )); then
    echo "regen-plugin-tree: $count files under $dir exceeds the $MAX_FILES-file limit" >&2
    bad=1
  fi
  while IFS= read -r -d '' path; do
    case "$path" in
      *.png|*.jpg|*.jpeg|*.gif|*.webp|*.woff|*.woff2|*.ttf|*.otf) continue ;;
    esac
    size="$(wc -c <"$path" | tr -d ' ')"
    if (( size > MAX_BYTES )); then
      echo "regen-plugin-tree: over 256 KiB: ${path#"$stage"/} ($size bytes)" >&2
      bad=1
    elif (( size > 0 )) && ! grep -Iq . "$path"; then
      echo "regen-plugin-tree: binary file type not allowed: ${path#"$stage"/}" >&2
      bad=1
    fi
  done < <(find "$dir" -type f -print0)
  return "$bad"
}

if [[ "$mode" == check ]]; then
  drift=0
  for component in "${COMPONENTS[@]}"; do
    if [[ ! -d "$PLUGIN_DIR/$component" ]]; then
      echo "stale: $PLUGIN_DIR/$component/ is missing"
      drift=1
      continue
    fi
    if ! summary="$(diff -rq "$stage/$component" "$PLUGIN_DIR/$component" 2>&1)"; then
      echo "stale: $PLUGIN_DIR/$component/ differs from $component/:"
      printf '%s\n' "$summary" | sed -e "s#$stage/##g" -e 's/^/  /' | head -n 40
      drift=1
    fi
  done
  check_limits "$PLUGIN_DIR" || drift=1
  if (( drift )); then
    echo "Run: bash scripts/regen-plugin-tree.sh" >&2
    exit 1
  fi
  echo "plugin/ is current ($(find "$PLUGIN_DIR" -type f | wc -l | tr -d ' ') files)."
  exit 0
fi

# Regenerate: validate the staged set first so a bad tree never lands.
check_limits "$stage" || die "staged component trees violate the plugin folder limits"
for component in "${COMPONENTS[@]}"; do
  rm -rf "${PLUGIN_DIR:?}/$component"
  cp -Rp "$stage/$component" "$PLUGIN_DIR/$component"
done
check_limits "$PLUGIN_DIR" || die "plugin/ violates the plugin folder limits"
echo "Regenerated plugin/ ($(find "$PLUGIN_DIR" -type f | wc -l | tr -d ' ') files)."

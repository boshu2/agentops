#!/usr/bin/env bash
set -euo pipefail

# Mirror reference files from skills/ to skills-codex/ and update SKILL.md links.
#
# Usage:
#   scripts/mirror-codex-references.sh council premortem    # mirror specific skills
#   scripts/mirror-codex-references.sh --all                 # mirror all skills
#   scripts/mirror-codex-references.sh --dry-run --all       # preview without changes
#   scripts/mirror-codex-references.sh --dry-run council     # preview one skill

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_SRC="$REPO_ROOT/skills"
SKILLS_DST="$REPO_ROOT/skills-codex"

DRY_RUN=false
ALL=false
SKILLS=()

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --all)     ALL=true ;;
    -h|--help)
      cat <<'USAGE'
Usage: scripts/mirror-codex-references.sh [--dry-run] [--all | SKILL ...]

Mirror reference files from skills/<name>/references/ to skills-codex/<name>/references/.
Updates skills-codex SKILL.md links and regenerates codex hashes.

Options:
  --dry-run   Preview changes without writing anything
  --all       Mirror all skills that exist in both skills/ and skills-codex/
  -h, --help  Show this help

Examples:
  scripts/mirror-codex-references.sh council premortem
  scripts/mirror-codex-references.sh --dry-run --all
USAGE
      exit 0
      ;;
    -*)
      echo "Unknown option: $arg" >&2
      exit 2
      ;;
    *)
      SKILLS+=("$arg")
      ;;
  esac
done

# Validate args
if [[ "$ALL" == true && ${#SKILLS[@]} -gt 0 ]]; then
  echo "Error: --all and specific skill names are mutually exclusive." >&2
  exit 2
fi

if [[ "$ALL" == false && ${#SKILLS[@]} -eq 0 ]]; then
  echo "Error: Provide skill name(s) or --all." >&2
  echo "Run with -h for usage." >&2
  exit 2
fi

# Build skill list
if [[ "$ALL" == true ]]; then
  SKILLS=()
  for src_dir in "$SKILLS_SRC"/*/; do
    name="$(basename "$src_dir")"
    # Only include skills that exist in both trees and have source references
    if [[ -d "$SKILLS_DST/$name" && -d "$SKILLS_SRC/$name/references" ]]; then
      SKILLS+=("$name")
    fi
  done
fi

copied=0
linked=0
skipped=0
errors=0

for skill in "${SKILLS[@]}"; do
  src_refs="$SKILLS_SRC/$skill/references"
  dst_dir="$SKILLS_DST/$skill"
  dst_refs="$SKILLS_DST/$skill/references"
  dst_skill_md="$dst_dir/SKILL.md"

  # Validate source exists
  if [[ ! -d "$SKILLS_SRC/$skill" ]]; then
    echo "WARN: skills/$skill does not exist, skipping." >&2
    ((errors++)) || true
    continue
  fi

  # Validate destination skill dir exists
  if [[ ! -d "$dst_dir" ]]; then
    echo "WARN: skills-codex/$skill does not exist, skipping." >&2
    ((errors++)) || true
    continue
  fi

  # Skip if no source references
  if [[ ! -d "$src_refs" ]]; then
    echo "SKIP: skills/$skill/references/ does not exist."
    ((skipped++)) || true
    continue
  fi

  # Ensure destination references dir exists
  if [[ ! -d "$dst_refs" ]]; then
    if [[ "$DRY_RUN" == true ]]; then
      echo "MKDIR: skills-codex/$skill/references/"
    else
      mkdir -p "$dst_refs"
      echo "MKDIR: skills-codex/$skill/references/"
    fi
  fi

  # Copy each reference file
  for src_file in "$src_refs"/*.md; do
    [[ -f "$src_file" ]] || continue
    filename="$(basename "$src_file")"
    dst_file="$dst_refs/$filename"

    # Check if file already exists and is identical
    if [[ -f "$dst_file" ]] && cmp -s "$src_file" "$dst_file"; then
      # Already up to date — check link anyway
      :
    else
      if [[ "$DRY_RUN" == true ]]; then
        if [[ -f "$dst_file" ]]; then
          echo "UPDATE: skills-codex/$skill/references/$filename"
        else
          echo "COPY:   skills-codex/$skill/references/$filename"
        fi
      else
        if [[ -f "$dst_file" ]]; then
          action="UPDATE"
        else
          action="COPY"
        fi
        cp "$src_file" "$dst_file"
        echo "$action:  skills-codex/$skill/references/$filename"
      fi
      ((copied++)) || true
    fi

    # Ensure SKILL.md has a link to this reference
    if [[ -f "$dst_skill_md" ]]; then
      link_pattern="references/$filename"
      if ! grep -qF "$link_pattern" "$dst_skill_md"; then
        link_line="- [references/$filename](references/$filename)"
        if [[ "$DRY_RUN" == true ]]; then
          echo "LINK:   $skill/SKILL.md += $link_line"
        else
          # Append link to the end of the file
          # First check if file ends with a newline
          if [[ -s "$dst_skill_md" ]] && [[ "$(tail -c 1 "$dst_skill_md" | xxd -p)" != "0a" ]]; then
            echo "" >> "$dst_skill_md"
          fi
          echo "$link_line" >> "$dst_skill_md"
          echo "LINK:   $skill/SKILL.md += $link_line"
        fi
        ((linked++)) || true
      fi
    fi
  done
done

echo ""
echo "Summary: $copied file(s) copied/updated, $linked link(s) added, $skipped skill(s) skipped, $errors warning(s)."

if [[ "$DRY_RUN" == true ]]; then
  echo "(dry-run mode — no changes written)"
  exit 0
fi

# Regenerate codex hashes if any files were copied
if [[ $copied -gt 0 || $linked -gt 0 ]]; then
  echo ""
  echo "Regenerating codex hashes..."
  bash "$SCRIPT_DIR/regen-codex-hashes.sh"
fi

#!/usr/bin/env bash
set -euo pipefail
file="${1:?Usage: check-generated-edits.sh <changed-files.txt>}"
generated=false
source=false
while IFS= read -r path; do
  case "$path" in
    registry.json|docs/SKILLS.md|cli/docs/COMMANDS.md|docs/cli-surface.json|docs/cli-surface.md) generated=true ;;
    skills/*|skills-codex/*|cli/cmd/ao/*) source=true ;;
  esac
done < "$file"
if [[ "$generated" == "true" && "$source" != "true" ]]; then
  echo "generated inventory changed without source"
  exit 1
fi
exit 0

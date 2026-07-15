#!/usr/bin/env bash
set -euo pipefail

# Compatibility selector for the legacy spelling `install.sh --tier spine`.
# The installed core is the metadata-derived `disposition: keep` set. The
# generated catalog is a projection of SKILL.md metadata and avoids a second
# handwritten tier list.

root="${1:-}"
if [[ -z "$root" || ! -d "$root" ]]; then
  echo "usage: select-spine-skills.sh <skills_root>" >&2
  exit 2
fi

catalog="$root/catalog.json"
if [[ ! -f "$catalog" ]]; then
  echo "select-spine-skills: generated catalog missing: $catalog" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "select-spine-skills: jq is required" >&2
  exit 1
fi

jq -r '.skills[] | select(.disposition == "keep") | .name' "$catalog" | sort -u

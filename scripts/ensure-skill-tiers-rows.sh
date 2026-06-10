#!/usr/bin/env bash
# ensure-skill-tiers-rows.sh — auto-render SKILL-TIERS.md rows from the disk SSOT.
#
# THE SSOT: the set of skill directories under skills/ is the single source of
# truth for which skills exist. SKILL-TIERS.md carries the *editorial* layer
# (curated tier + description per skill). Historically a human had to hand-add a
# table row BEFORE scripts/sync-skill-counts.sh would run — sync fails-closed on
# `rows != directories`, so a forgotten row blocked the entire doc-release gate
# (this is the friction that stuck the 3.1 release; cp-9wvq).
#
# This script removes that manual step: for every skill dir with no SKILL-TIERS
# row, it appends a derived placeholder row (description pulled from the skill's
# own SKILL.md frontmatter; tier defaults to `execution`) into the user-facing
# "Factory-Built Operator And Pack Skills" table. Curated rows are NEVER touched.
# After it runs, `rows == directories` holds by construction, so sync's count
# patchers can propagate the derived count to every doc with zero hand-edits.
#
# A maintainer can later re-tier or re-word an auto-added row, or move it to the
# Internal table — that is editorial polish, not a release blocker.
#
# Usage: scripts/ensure-skill-tiers-rows.sh [--check]
#   --check   Report skills missing a row (exit 1 if any) without modifying.
set -euo pipefail
export LC_ALL=C

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TIERS="$REPO_ROOT/skills/SKILL-TIERS.md"
CHECK_ONLY=false

if [[ "${1:-}" == "--check" ]]; then
  CHECK_ONLY=true
elif [[ $# -gt 0 ]]; then
  echo "ERROR: unknown argument '$1'"
  echo "Usage: scripts/ensure-skill-tiers-rows.sh [--check]"
  exit 2
fi

if [[ ! -f "$TIERS" ]]; then
  echo "ERROR: SKILL-TIERS.md not found: $TIERS"
  exit 1
fi

# Anchor marker: rows are appended immediately before the Internal Skills header
# (i.e. at the end of the last user-facing table).
ANCHOR='^### Internal Skills'
if ! grep -qE "$ANCHOR" "$TIERS"; then
  echo "ERROR: SKILL-TIERS.md missing '### Internal Skills' anchor — cannot place auto-rows"
  exit 1
fi

# --- Skill names already present in EITHER table (matches validator's view) ---
in_table_names() {
  # user-facing rows: '| **name** | ...' within the user-facing section
  sed -n '/^### User-Facing Skills/,/^### Internal Skills/p' "$TIERS" \
    | grep '^| \*\*' | sed -E 's/^\| \*\*([^*]+)\*\*.*/\1/'
  # internal rows: '| name ...' within the internal section (drop header/sep)
  sed -n '/^### Internal Skills/,/^---$/p' "$TIERS" \
    | grep -E '^\| ' | grep -vE '^\| Skill|^\|[[:space:]]*-' \
    | sed -E 's/^\| ([a-z0-9_-]+)[[:space:]].*/\1/'
}

# --- Disk SSOT: real skill directories (mirror sync/validate exclusions) ---
disk_names() {
  find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d \
    -not -name '.*' -not -name '_*' -exec basename {} \;
}

# Extract the human-facing description from a skill's frontmatter.
# Handles both inline `description: text` and block `description: |-\n  text`.
skill_description() {
  local md="$1"
  awk '
    /^description:[[:space:]]*\|/ { block=1; next }
    block==1 {
      if ($0 ~ /^[[:space:]]+/) { sub(/^[[:space:]]+/, ""); print; exit }
      else { block=0 }
    }
    /^description:[[:space:]]*[^|[:space:]]/ {
      sub(/^description:[[:space:]]*/, ""); print; exit
    }
  ' "$md" | head -1
}

mapfile -t IN_TABLE < <(in_table_names | sort -u)
mapfile -t DISK < <(disk_names | sort -u)

# Compute missing = disk - in_table
declare -A have=()
for n in "${IN_TABLE[@]}"; do have["$n"]=1; done

missing=()
for n in "${DISK[@]}"; do
  [[ -n "${have[$n]:-}" ]] || missing+=("$n")
done

if [[ ${#missing[@]} -eq 0 ]]; then
  echo "OK: every skill directory has a SKILL-TIERS.md row (${#DISK[@]} skills)"
  exit 0
fi

if $CHECK_ONLY; then
  echo "MISSING: ${#missing[@]} skill(s) have no SKILL-TIERS.md row:"
  printf '  %s\n' "${missing[@]}"
  echo "Run: scripts/ensure-skill-tiers-rows.sh"
  exit 1
fi

# --- Build the rows to inject (sorted for determinism) ---
rows_block=""
for name in $(printf '%s\n' "${missing[@]}" | sort); do
  md="$REPO_ROOT/skills/$name/SKILL.md"
  desc=""
  [[ -f "$md" ]] && desc="$(skill_description "$md")"
  [[ -n "$desc" ]] || desc="Auto-added — tier and description pending curation."
  # Escape any literal pipe in the description so it can't break the table.
  desc="${desc//|/\\|}"
  rows_block+="| **${name}** | execution | ${desc} |"$'\n'
done

# Insert the rows immediately before the '### Internal Skills' header.
# Strategy: split the file at the anchor line, trim trailing blank lines off the
# head slice (so rows attach right after the last user-facing row), append the
# rows + one blank line, then re-join the anchor and everything after it.
tmp="$(mktemp)"
anchor_line="$(grep -nE "$ANCHOR" "$TIERS" | head -1 | cut -d: -f1)"
# Head slice = everything above the anchor, with trailing blank lines removed
# (portable awk trim — buffer blanks, flush only when a non-blank follows).
head -n $((anchor_line - 1)) "$TIERS" \
  | awk '
      /^[[:space:]]*$/ { pending++; next }
      { while (pending-- > 0) print ""; pending=0; print }
    ' > "$tmp"
# Last user-facing row, then the new rows, then one blank before the anchor.
printf '%s' "$rows_block" >> "$tmp"
printf '\n' >> "$tmp"
tail -n +"$anchor_line" "$TIERS" >> "$tmp"
mv "$tmp" "$TIERS"

echo "UPDATED: appended ${#missing[@]} auto-row(s) to SKILL-TIERS.md user-facing table:"
printf '  %s\n' "${missing[@]}"

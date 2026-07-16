#!/usr/bin/env bash
#
# Copy skill folders (directories containing SKILL.md)
# into a local skills directory.
#
# Safe by default:
# - Creates a timestamped backup of any destination skill it overwrites
# - Supports --dry-run to preview changes
# - On a full refresh, removes only obsolete skills named by the prior
#   AgentOps manifest; unrelated user skills are preserved.
#
# Usage:
#   ./scripts/export-claude-skills-to-codex.sh \
#     --src ./skills \
#     --dst "$HOME/.agents/skills" \
#     --dry-run
#
set -euo pipefail

usage() {
  cat <<'EOF'
export-claude-skills-to-codex.sh

Copies skill directories (each containing SKILL.md) from --src into --dst.

Options:
  --src <dir>         Source directory containing skill folders (default: ./skills if present, else ./.agents/skills)
  --dst <dir>         Destination skills directory (default: ~/.agents/skills)
  --backup <dir>      Backup directory (default: <dst>.backup.<timestamp>)
  --dry-run           Show what would change (no writes)
  --only <a,b,c>      Only copy these skill folder names (comma-separated)
  --help              Show this help

Examples:
  ./scripts/export-claude-skills-to-codex.sh --dry-run
  ./scripts/export-claude-skills-to-codex.sh --src ./skills --dst ~/.agents/skills
  ./scripts/export-claude-skills-to-codex.sh --only research,vibe --dry-run
EOF
}

SRC=""
DST=""
BACKUP=""
DRY_RUN="false"
ONLY_CSV=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --src)
      SRC="${2:-}"
      shift 2
      ;;
    --dst)
      DST="${2:-}"
      shift 2
      ;;
    --backup)
      BACKUP="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift 1
      ;;
    --only)
      ONLY_CSV="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v rsync >/dev/null 2>&1; then
  echo "Error: rsync not found. Install rsync and re-run." >&2
  exit 1
fi

if [[ -z "$SRC" ]]; then
  if [[ -d "skills" ]]; then
    SRC="skills"
  elif [[ -d ".agents/skills" ]]; then
    SRC=".agents/skills"
  else
    echo "Error: cannot infer --src (no ./skills or ./.agents/skills)." >&2
    exit 1
  fi
fi

if [[ -z "$DST" ]]; then
  DST="$HOME/.agents/skills"
fi

timestamp="$(date +%Y%m%d-%H%M%S)"
if [[ -z "$BACKUP" ]]; then
  BACKUP="${DST}.backup.${timestamp}"
fi

if [[ ! -d "$SRC" ]]; then
  echo "Error: --src does not exist: $SRC" >&2
  exit 1
fi

mkdir -p "$DST"
if [[ "$DRY_RUN" != "true" ]]; then
  mkdir -p "$BACKUP"
fi

declare -A ONLY
if [[ -n "$ONLY_CSV" ]]; then
  IFS=',' read -r -a only_arr <<<"$ONLY_CSV"
  for name in "${only_arr[@]}"; do
    name="$(echo "$name" | xargs)"
    [[ -n "$name" ]] && ONLY["$name"]=1
  done
fi

copied=0
skipped=0
removed=0

echo "Source: $SRC"
echo "Dest:   $DST"
echo "Backup: $BACKUP"
echo "DryRun: $DRY_RUN"
echo ""

# A full refresh reconciles the prior AgentOps-owned set against the new source
# manifest. This removes deleted skill directories and dangling symlinks without
# treating the entire destination as AgentOps-owned. Partial --only installs do
# not prune anything.
if [[ -z "$ONLY_CSV" ]] && command -v jq >/dev/null 2>&1 && [[ -f "$DST/.agentops-manifest.json" ]]; then
  declare -A SOURCE_NAMES
  for skill_dir in "$SRC"/*/; do
    [[ -f "${skill_dir}SKILL.md" ]] || continue
    SOURCE_NAMES["$(basename "$skill_dir")"]=1
  done
  while IFS= read -r old_name; do
    [[ -n "$old_name" ]] || continue
    [[ -n "${SOURCE_NAMES[$old_name]:-}" ]] && continue
    old_path="${DST%/}/$old_name"
    [[ -e "$old_path" || -L "$old_path" ]] || continue
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "Would remove obsolete AgentOps skill: $old_path"
    else
      if [[ -d "$old_path" && ! -L "$old_path" ]]; then
        rsync -a "${old_path%/}/" "${BACKUP%/}/${old_name%/}/"
      fi
      rm -rf "$old_path"
      echo "Removed obsolete AgentOps skill: $old_path"
    fi
    removed=$((removed + 1))
  done < <(jq -r '.skills[]?.name // empty' "$DST/.agentops-manifest.json")
fi

shopt -s nullglob
for skill_dir in "$SRC"/*/; do
  skill_name="$(basename "$skill_dir")"

  if [[ -n "$ONLY_CSV" ]] && [[ -z "${ONLY[$skill_name]:-}" ]]; then
    skipped=$((skipped + 1))
    continue
  fi

  if [[ ! -f "${skill_dir}SKILL.md" ]]; then
    skipped=$((skipped + 1))
    continue
  fi

  dst_skill="${DST%/}/${skill_name}"

  # Backup existing dest skill before overwriting
  if [[ -d "$dst_skill" ]] && [[ "$DRY_RUN" != "true" ]]; then
    rsync -a --delete "${dst_skill%/}/" "${BACKUP%/}/${skill_name%/}/"
  fi

  # Copy skill (mirror, no symlinks)
  rsync_args=(-a --delete --copy-links)
  if [[ "$DRY_RUN" == "true" ]]; then
    rsync_args+=(--dry-run)
  fi

  rsync "${rsync_args[@]}" "${skill_dir%/}/" "${dst_skill%/}/" >/dev/null
  copied=$((copied + 1))
done

for root_file in "$SRC"/.agentops-*.json; do
  [[ -f "$root_file" ]] || continue
  rsync_args=(-a --copy-links)
  if [[ "$DRY_RUN" == "true" ]]; then
    rsync_args+=(--dry-run)
  fi
  rsync "${rsync_args[@]}" "$root_file" "${DST%/}/" >/dev/null
done

echo "Skills copied: $copied"
echo "Skills skipped: $skipped"
echo "Obsolete AgentOps skills removed: $removed"
if [[ "$DRY_RUN" != "true" ]]; then
  echo "Backups written to: $BACKUP"
fi

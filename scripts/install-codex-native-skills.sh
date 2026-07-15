#!/usr/bin/env bash
#
# Legacy copied-skill installer. New installs should use `ao skills link`.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
EXPORT_SCRIPT="$REPO_ROOT/scripts/export-claude-skills-to-codex.sh"
PLUGIN_INSTALL_SCRIPT="$REPO_ROOT/scripts/install-codex-plugin.sh"

SRC="$REPO_ROOT/skills-codex"
DST="$HOME/.agents/skills"
USER_DST="$HOME/.agents/skills"
USER_DST_INSTALLED=""
INSTALL_META="$HOME/.codex/.agentops-codex-install.json"
BACKUP=""
ONLY_CSV=""
SKIP_SYNC="false"
DRY_RUN="false"
DEST_EXPLICIT="false"
SKILL_MANIFEST_NAME=".agentops-manifest.json"

usage() {
  cat <<'EOF'
install-codex-native-skills.sh

Installs the checked-in Codex-native skills from ./skills-codex to ~/.agents/skills.

DEPRECATED: retained for 3.x migration compatibility. New installs should use
one canonical checkout plus `ao skills link`.

If the current Codex install metadata reports native-plugin mode, the default
behavior is to refresh the active plugin cache without leaving a duplicate raw
skill mirror in ~/.agents/skills. Pass --dest to force a raw skill install
destination instead.

Options:
  --source <dir>      Codex-native source skills root (default: ./skills-codex)
  --dest <dir>        Destination raw skills root (default: ~/.agents/skills)
  --backup <dir>      Backup directory (default: <dest>.backup.<timestamp>)
  --only <a,b,c>      Only install these skill names (comma-separated)
  --skip-sync         Deprecated no-op kept for compatibility
  --dry-run           Preview install copy operations
  --help              Show this help

Examples:
  ./scripts/install-codex-native-skills.sh
  ./scripts/install-codex-native-skills.sh --only research,vibe
  ./scripts/install-codex-native-skills.sh --dest /tmp/codex-skills --dry-run
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      SRC="${2:-}"
      shift 2
      ;;
    --dest)
      DST="${2:-}"
      DEST_EXPLICIT="true"
      shift 2
      ;;
    --backup)
      BACKUP="${2:-}"
      shift 2
      ;;
    --only)
      ONLY_CSV="${2:-}"
      shift 2
      ;;
    --skip-sync)
      SKIP_SYNC="true"
      shift 1
      ;;
    --dry-run)
      DRY_RUN="true"
      shift 1
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

if [[ "$SRC" != /* ]]; then
  SRC="$REPO_ROOT/$SRC"
fi
if [[ "$DST" != /* ]]; then
  DST="$REPO_ROOT/$DST"
fi

read_install_meta_value() {
  local key="$1"

  [[ -f "$INSTALL_META" ]] || return 1
  sed -n "s/.*\"${key}\":[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$INSTALL_META" | head -n 1
}

sha256_file() {
  local path="$1"

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$path" | awk '{print $NF}'
    return
  fi

  echo "Error: need shasum, sha256sum, or openssl to compute install snapshots." >&2
  exit 1
}

should_refresh_native_plugin() {
  local install_mode
  local plugin_root

  [[ "$DEST_EXPLICIT" != "true" ]] || return 1
  install_mode="$(read_install_meta_value "install_mode" || true)"
  plugin_root="$(read_install_meta_value "plugin_root" || true)"

  [[ "$install_mode" == "native-plugin" ]] || return 1
  [[ -n "$plugin_root" ]] || return 1
}

if [[ "$SKIP_SYNC" == "true" ]]; then
  echo "Note: --skip-sync is deprecated and ignored. installs use checked-in skills-codex/ directly." >&2
fi

[[ -d "$SRC" ]] || {
  echo "Error: source codex skills directory not found: $SRC" >&2
  echo "Restore the checked-in skills-codex/ directory or pass --source with a valid Codex skill bundle." >&2
  exit 1
}
[[ -f "$SRC/$SKILL_MANIFEST_NAME" ]] || {
  echo "Error: source codex skill manifest not found: $SRC/$SKILL_MANIFEST_NAME" >&2
  echo "Restore the checked-in manifest or point --source at a complete Codex skill bundle." >&2
  exit 1
}

VERSION="unknown"
if git -C "$REPO_ROOT" rev-parse --short HEAD >/dev/null 2>&1; then
  VERSION="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
fi

if should_refresh_native_plugin && [[ "$DRY_RUN" != "true" ]]; then
  plugin_root="$(read_install_meta_value "plugin_root")"
  echo "Detected native-plugin install metadata at $INSTALL_META"
  echo "Refreshing active Codex plugin cache at $plugin_root"
  bash "$PLUGIN_INSTALL_SCRIPT" \
    --repo-root "$REPO_ROOT" \
    --codex-home "$HOME/.codex" \
    --skills-src "$SRC" \
    --version "$VERSION" \
    --update-command "curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash"
  echo "Restart Codex to pick up installed skills."
  exit 0
fi

export_args=(--src "$SRC" --dst "$DST")
if [[ -n "$BACKUP" ]]; then
  export_args+=(--backup "$BACKUP")
fi
if [[ -n "$ONLY_CSV" ]]; then
  export_args+=(--only "$ONLY_CSV")
fi
if [[ "$DRY_RUN" == "true" ]]; then
  export_args+=(--dry-run)
fi

bash "$EXPORT_SCRIPT" "${export_args[@]}"

if [[ "$DEST_EXPLICIT" != "true" && "$DST" != "$USER_DST" ]]; then
  user_export_args=(--src "$SRC" --dst "$USER_DST")
  if [[ -n "$ONLY_CSV" ]]; then
    user_export_args+=(--only "$ONLY_CSV")
  fi
  if [[ "$DRY_RUN" == "true" ]]; then
    user_export_args+=(--dry-run)
  fi
  bash "$EXPORT_SCRIPT" "${user_export_args[@]}"
  USER_DST_INSTALLED="$USER_DST"
elif [[ "$DST" == "$USER_DST" ]]; then
  USER_DST_INSTALLED="$USER_DST"
fi

# Write local install metadata for stale-skill reminders (no telemetry)
INSTALLED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MANIFEST_HASH="$(sha256_file "$SRC/$SKILL_MANIFEST_NAME")"
INSTALLED_MANIFEST="$DST/$SKILL_MANIFEST_NAME"
[[ -f "$INSTALLED_MANIFEST" ]] || {
  echo "Error: installed codex skill manifest missing after export: $INSTALLED_MANIFEST" >&2
  exit 1
}
INSTALLED_MANIFEST_HASH="$(sha256_file "$INSTALLED_MANIFEST")"
[[ "$MANIFEST_HASH" == "$INSTALLED_MANIFEST_HASH" ]] || {
  echo "Error: installed codex skill manifest hash mismatch; expected $MANIFEST_HASH, got $INSTALLED_MANIFEST_HASH" >&2
  exit 1
}
SKILL_COUNT="$(find "$DST" -mindepth 2 -maxdepth 2 -name SKILL.md 2>/dev/null | wc -l | tr -d ' ')"
RAW_STATE_FILE="$DST/.agentops-codex-state.json"
cat > "$RAW_STATE_FILE" <<EOF
{
  "installed_at": "$INSTALLED_AT",
  "install_mode": "raw-skills",
  "version": "$VERSION",
  "manifest_hash": "$MANIFEST_HASH",
  "skill_count": $SKILL_COUNT,
  "skills_root": "$DST"
}
EOF

mkdir -p "$(dirname "$INSTALL_META")"
if [[ -z "$USER_DST_INSTALLED" ]]; then
  USER_DST_INSTALLED="null"
else
  USER_DST_INSTALLED="\"$USER_DST_INSTALLED\""
fi

cat > "$INSTALL_META" <<EOF
{
  "installed_at": "$INSTALLED_AT",
  "source": "install-codex-native-skills.sh",
  "install_mode": "raw-skills",
  "version": "$VERSION",
  "manifest_hash": "$MANIFEST_HASH",
  "skill_count": $SKILL_COUNT,
  "skills_root": "$DST",
  "user_skills_root": $USER_DST_INSTALLED,
  "plugin_state_file": "$RAW_STATE_FILE",
  "update_command": "curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash"
}
EOF
echo "Install metadata written: $INSTALL_META"

echo "Restart Codex to pick up installed skills."

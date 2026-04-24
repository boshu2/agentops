#!/usr/bin/env bash
# install-codex-plugin.sh — Install the AgentOps native Codex plugin into CODEX_HOME.
#
# Usage:
#   bash scripts/install-codex-plugin.sh
#   bash scripts/install-codex-plugin.sh --repo-root /path/to/agentops --codex-home /tmp/codex-home

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }
fail()  { echo -e "${RED}✗${NC} $*"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CODEX_HOME="${HOME}/.codex"
PLUGIN_NAME="agentops"
MARKETPLACE_NAME="agentops-marketplace"
PLUGIN_KEY="${PLUGIN_NAME}@${MARKETPLACE_NAME}"
VERSION="${AGENTOPS_INSTALL_VERSION:-unknown}"
UPDATE_CMD="${AGENTOPS_UPDATE_COMMAND:-curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash}"
PLUGIN_SKILLS_SRC=""

PLUGIN_MANIFEST="${REPO_ROOT}/.codex-plugin/plugin.json"
MARKETPLACE_FILE="${REPO_ROOT}/plugins/marketplace.json"
PLUGIN_CACHE_ROOT="${CODEX_HOME}/plugins/cache/${MARKETPLACE_NAME}/${PLUGIN_NAME}/local"
PLUGIN_SKILLS_DST="${PLUGIN_CACHE_ROOT}/skills-codex"
LEGACY_SKILLS_DIR="${CODEX_HOME}/skills"
USER_SKILLS_DIR="$(dirname "$CODEX_HOME")/.agents/skills"
CONFIG_FILE="${CODEX_HOME}/config.toml"
INSTALL_META="${CODEX_HOME}/.agentops-codex-install.json"
SKILL_MANIFEST_NAME=".agentops-manifest.json"
PLUGIN_STATE_FILE=""
LEGACY_BACKUP_DIR=""
USER_BACKUP_DIR=""

usage() {
  cat <<'EOF'
install-codex-plugin.sh

Install the AgentOps native Codex plugin into CODEX_HOME.

Options:
  --repo-root <dir>     AgentOps repo or extracted release bundle root
  --codex-home <dir>    Target Codex home (default: ~/.codex)
  --skills-src <dir>    Codex-native skills source root (default: <repo-root>/skills-codex)
  --version <value>     Version string to record in install metadata
  --update-command <s>  Update command to record in install metadata
  --help                Show this help
EOF
}

detect_bwrap_install_hint() {
  if command -v apt-get >/dev/null 2>&1 || command -v apt >/dev/null 2>&1; then
    printf '%s\n' 'sudo apt-get install -y bubblewrap'
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    printf '%s\n' 'sudo dnf install -y bubblewrap'
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    printf '%s\n' 'sudo yum install -y bubblewrap'
    return
  fi
  if command -v pacman >/dev/null 2>&1; then
    printf '%s\n' 'sudo pacman -S --needed bubblewrap'
    return
  fi
  if command -v zypper >/dev/null 2>&1; then
    printf '%s\n' 'sudo zypper install bubblewrap'
    return
  fi

  printf '%s\n' '<your package manager> install bubblewrap'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --codex-home)
      CODEX_HOME="${2:-}"
      shift 2
      ;;
    --skills-src)
      PLUGIN_SKILLS_SRC="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --update-command)
      UPDATE_CMD="${2:-}"
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

if [[ "$REPO_ROOT" != /* ]]; then
  REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"
fi
if [[ "$CODEX_HOME" != /* ]]; then
  CODEX_HOME="$(cd "$CODEX_HOME" && pwd)"
fi

PLUGIN_MANIFEST="${REPO_ROOT}/.codex-plugin/plugin.json"
MARKETPLACE_FILE="${REPO_ROOT}/plugins/marketplace.json"
if [[ -z "$PLUGIN_SKILLS_SRC" ]]; then
  PLUGIN_SKILLS_SRC="${REPO_ROOT}/skills-codex"
fi
if [[ "$PLUGIN_SKILLS_SRC" != /* ]]; then
  PLUGIN_SKILLS_SRC="${REPO_ROOT}/${PLUGIN_SKILLS_SRC}"
fi
PLUGIN_CACHE_ROOT="${CODEX_HOME}/plugins/cache/${MARKETPLACE_NAME}/${PLUGIN_NAME}/local"
PLUGIN_SKILLS_DST="${PLUGIN_CACHE_ROOT}/skills-codex"
LEGACY_SKILLS_DIR="${CODEX_HOME}/skills"
USER_SKILLS_DIR="$(dirname "$CODEX_HOME")/.agents/skills"
CONFIG_FILE="${CODEX_HOME}/config.toml"
INSTALL_META="${CODEX_HOME}/.agentops-codex-install.json"

require_path() {
  local path="$1"
  local label="$2"
  [[ -e "$path" ]] || fail "Missing ${label}: $path"
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

  fail "Need shasum, sha256sum, or openssl to compute install snapshots"
}

upsert_toml_key() {
  local file="$1"
  local section="$2"
  local key="$3"
  local value="$4"
  local tmp

  mkdir -p "$(dirname "$file")"
  if [[ ! -f "$file" ]]; then
    printf '%s\n%s = %s\n' "$section" "$key" "$value" > "$file"
    return
  fi

  tmp="$(mktemp)"
  awk \
    -v section="$section" \
    -v key="$key" \
    -v value="$value" \
    '
    function emit_key() {
      print key " = " value
    }
    BEGIN {
      in_section = 0
      saw_section = 0
      wrote_key = 0
    }
    {
      if ($0 == section) {
        saw_section = 1
        in_section = 1
        print
        next
      }

      if (in_section && $0 ~ /^\[/) {
        if (!wrote_key) {
          emit_key()
          wrote_key = 1
        }
        in_section = 0
      }

      if (in_section && $0 ~ ("^[[:space:]]*" key "[[:space:]]*=")) {
        if (!wrote_key) {
          emit_key()
          wrote_key = 1
        }
        next
      }

      print
    }
    END {
      if (in_section && !wrote_key) {
        emit_key()
        wrote_key = 1
      }
      if (!saw_section) {
        if (NR > 0) {
          print ""
        }
        print section
        emit_key()
      }
    }
    ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

count_codex_hook_handlers() {
  local path="$1"

  jq -r '[.hooks | to_entries[]? | .value[]? | .hooks[]?] | length' "$path"
}

merge_codex_hooks() {
  local existing_file="$1"
  local new_file="$2"
  local merged_file
  merged_file="${TMP_DIR}/merged-codex-hooks.json"

  jq -n \
    --slurpfile existing "$existing_file" \
    --slurpfile new "$new_file" \
    '
    def agentops_scripts:
      [
        "session-start.sh",
        "ao-inject.sh",
        "ao-flywheel-close.sh",
        "prompt-nudge.sh",
        "quality-signals.sh",
        "go-test-precommit.sh",
        "commit-review-gate.sh",
        "ratchet-advance.sh"
      ];
    def is_agentops_command($cmd):
      ($cmd | type == "string") and any(agentops_scripts[]; . as $script | $cmd | endswith("/hooks/" + $script));
    def strip_agentops_groups:
      if (.hooks | type? == "object") then
        .hooks |= with_entries(
          .value |= [
            .[]?
            | .hooks = [
                .hooks[]?
                | select(is_agentops_command(.command // "") | not)
              ]
            | select((.hooks | length) > 0)
          ]
        )
        | .hooks |= with_entries(select((.value | length) > 0))
      else
        .hooks = {}
      end;
    def merge_hook_maps($existing_hooks; $new_hooks):
      reduce (($existing_hooks + $new_hooks) | keys_unsorted[]) as $event ({};
        .[$event] = (($existing_hooks[$event] // []) + ($new_hooks[$event] // []))
      );
    ($existing[0] // {}) as $existing_doc
    | ($new[0] // {}) as $new_doc
    | ($existing_doc | strip_agentops_groups) as $cleaned
    | ($new_doc."$schema" // $existing_doc."$schema") as $schema
    | {
        "$schema": $schema,
        "hooks": merge_hook_maps(($cleaned.hooks // {}); ($new_doc.hooks // {}))
      }
    ' > "$merged_file"
  mv "$merged_file" "$existing_file"
}

stage_plugin_source() {
  local staging_root="$1"

  mkdir -p "$staging_root"
  cp -R "$REPO_ROOT/.codex-plugin" "$staging_root/.codex-plugin"
  cp -R "$PLUGIN_SKILLS_SRC" "$staging_root/skills-codex"

  if [[ -f "$REPO_ROOT/.mcp.json" ]]; then
    cp "$REPO_ROOT/.mcp.json" "$staging_root/.mcp.json"
  fi
  if [[ -f "$REPO_ROOT/.app.json" ]]; then
    cp "$REPO_ROOT/.app.json" "$staging_root/.app.json"
  fi
}

archive_skill_root() {
  local root="$1"
  local backup_dir="$2"
  local managed_root="$3"
  local skill_dir
  local skill_name
  local root_skill
  local moved=0

  [[ -d "$root" ]] || return 0

  while IFS= read -r -d '' skill_dir; do
    skill_name="$(basename "$skill_dir")"
    root_skill="$root/$skill_name"
    [[ -d "$root_skill" ]] || continue
    if [[ "$managed_root" != "true" && ! -f "$root_skill/.agentops-generated.json" ]]; then
      continue
    fi
    mkdir -p "$backup_dir"
    mv "$root_skill" "$backup_dir/$skill_name"
    moved=$((moved + 1))
  done < <(find "$PLUGIN_SKILLS_SRC" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)

  if [[ -f "$root/$SKILL_MANIFEST_NAME" ]]; then
    mkdir -p "$backup_dir"
    mv "$root/$SKILL_MANIFEST_NAME" "$backup_dir/$SKILL_MANIFEST_NAME"
    moved=$((moved + 1))
  fi
  if [[ -f "$root/.agentops-codex-state.json" ]]; then
    mkdir -p "$backup_dir"
    mv "$root/.agentops-codex-state.json" "$backup_dir/.agentops-codex-state.json"
    moved=$((moved + 1))
  fi

  if [[ "$moved" -eq 0 ]]; then
    rmdir "$backup_dir" 2>/dev/null || true
    return 1
  fi

  return 0
}

archive_legacy_codex_skills() {
  local timestamp
  local backup_dir

  [[ -d "$LEGACY_SKILLS_DIR" ]] || return 0

  timestamp="$(date +%Y%m%d-%H%M%S)"
  backup_dir="${CODEX_HOME}/skills.backup.${timestamp}"
  if archive_skill_root "$LEGACY_SKILLS_DIR" "$backup_dir" "true"; then
    LEGACY_BACKUP_DIR="$backup_dir"
  fi
}

archive_user_raw_skills() {
  local timestamp
  local backup_dir
  local managed_root="false"

  [[ -d "$USER_SKILLS_DIR" ]] || return 0

  if [[ -f "$USER_SKILLS_DIR/$SKILL_MANIFEST_NAME" || -f "$USER_SKILLS_DIR/.agentops-codex-state.json" ]]; then
    managed_root="true"
  fi

  timestamp="$(date +%Y%m%d-%H%M%S)"
  backup_dir="$(dirname "$USER_SKILLS_DIR")/skills.backup.${timestamp}"
  if archive_skill_root "$USER_SKILLS_DIR" "$backup_dir" "$managed_root"; then
    USER_BACKUP_DIR="$backup_dir"
  fi
}

require_path "$PLUGIN_MANIFEST" "Codex plugin manifest"
require_path "$MARKETPLACE_FILE" "Codex marketplace manifest"
require_path "$PLUGIN_SKILLS_SRC" "Codex-native skill bundle"
require_path "$PLUGIN_SKILLS_SRC/$SKILL_MANIFEST_NAME" "Codex skill manifest"
PLUGIN_STATE_FILE="${PLUGIN_CACHE_ROOT}/.agentops-codex-state.json"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

info "Installing AgentOps Codex native plugin..."

mkdir -p "$(dirname "$PLUGIN_CACHE_ROOT")"
rm -rf "$PLUGIN_CACHE_ROOT"
stage_plugin_source "$TMP_DIR/plugin"
cp -R "$TMP_DIR/plugin" "$PLUGIN_CACHE_ROOT"

upsert_toml_key "$CONFIG_FILE" "[features]" "plugins" "true"
upsert_toml_key "$CONFIG_FILE" "[plugins.\"${PLUGIN_KEY}\"]" "enabled" "true"
upsert_toml_key "$CONFIG_FILE" "[ui]" "suppress_unstable_features_warning" "true"

INSTALLED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MANIFEST_HASH="$(sha256_file "$PLUGIN_SKILLS_SRC/$SKILL_MANIFEST_NAME")"
require_path "$PLUGIN_SKILLS_DST/$SKILL_MANIFEST_NAME" "installed Codex skill manifest"
INSTALLED_MANIFEST_HASH="$(sha256_file "$PLUGIN_SKILLS_DST/$SKILL_MANIFEST_NAME")"
[[ "$MANIFEST_HASH" == "$INSTALLED_MANIFEST_HASH" ]] || fail "Installed plugin cache manifest hash mismatch; expected $MANIFEST_HASH, got $INSTALLED_MANIFEST_HASH"
SKILL_COUNT="$(find "$PLUGIN_SKILLS_DST" -mindepth 2 -maxdepth 2 -name SKILL.md 2>/dev/null | wc -l | tr -d ' ')"

archive_legacy_codex_skills
archive_user_raw_skills

cat > "$PLUGIN_STATE_FILE" <<EOF
{
  "installed_at": "$INSTALLED_AT",
  "install_mode": "native-plugin",
  "hook_runtime": "codex-native-hooks",
  "version": "$VERSION",
  "manifest_hash": "$MANIFEST_HASH",
  "skill_count": $SKILL_COUNT,
  "plugin_root": "$PLUGIN_CACHE_ROOT"
}
EOF
mkdir -p "$(dirname "$INSTALL_META")"
cat > "$INSTALL_META" <<EOF
{
  "installed_at": "$INSTALLED_AT",
  "source": "install-codex-plugin.sh",
  "install_mode": "native-plugin",
  "hook_runtime": "codex-native-hooks",
  "hook_contract": "docs/contracts/hook-runtime-contract.md",
  "lifecycle_commands": ["ao codex start", "ao codex stop"],
  "plugin_key": "$PLUGIN_KEY",
  "version": "$VERSION",
  "plugin_root": "$PLUGIN_CACHE_ROOT",
  "manifest_hash": "$MANIFEST_HASH",
  "skill_count": $SKILL_COUNT,
  "plugin_state_file": "$PLUGIN_STATE_FILE",
  "user_skills_root": null,
  "update_command": "$UPDATE_CMD"
}
EOF

# ── Install Codex-native hooks ──
HOOKS_SRC="${REPO_ROOT}/hooks"
HOOKS_DST="${PLUGIN_CACHE_ROOT}/hooks"

if [[ -f "${HOOKS_SRC}/codex-hooks.json" ]]; then
  mkdir -p "$HOOKS_DST"
  # Copy hook scripts to plugin cache
  for hook_script in "$HOOKS_SRC"/*.sh; do
    [[ -f "$hook_script" ]] || continue
    cp "$hook_script" "$HOOKS_DST/"
    chmod +x "$HOOKS_DST/$(basename "$hook_script")"
  done
  # Copy shared helpers
  if [[ -d "${REPO_ROOT}/lib" ]]; then
    cp "${REPO_ROOT}/lib/"*.sh "$HOOKS_DST/" 2>/dev/null || true
  fi

  # Install hooks.json to ~/.codex/hooks.json (merge if exists)
  CODEX_HOOKS_FILE="${CODEX_HOME}/hooks.json"
  CODEX_HOOKS_SRC="${HOOKS_SRC}/codex-hooks.json"

  # Replace AGENTOPS_PLUGIN_ROOT with actual path
  RENDERED_HOOKS="$(sed "s|\${AGENTOPS_PLUGIN_ROOT:-~/.codex/plugins/cache/agentops}|${PLUGIN_CACHE_ROOT}|g" "$CODEX_HOOKS_SRC")"
  RENDERED_HOOKS_FILE="${TMP_DIR}/rendered-codex-hooks.json"
  printf '%s\n' "$RENDERED_HOOKS" > "$RENDERED_HOOKS_FILE"

  if [[ -f "$CODEX_HOOKS_FILE" ]]; then
    # Backup existing hooks
    cp "$CODEX_HOOKS_FILE" "${CODEX_HOOKS_FILE}.bak.$(date +%s)"
    if jq -e '.hooks | type == "array"' "$CODEX_HOOKS_FILE" >/dev/null 2>&1; then
      warn "Existing ~/.codex/hooks.json uses the legacy flat-array shape; replacing it with the current Codex event-map schema."
    fi
    merge_codex_hooks "$CODEX_HOOKS_FILE" "$RENDERED_HOOKS_FILE"
  else
    mkdir -p "$(dirname "$CODEX_HOOKS_FILE")"
    jq '.' "$RENDERED_HOOKS_FILE" > "$CODEX_HOOKS_FILE"
  fi

  # Enable hooks feature in config
  upsert_toml_key "$CONFIG_FILE" "[features]" "codex_hooks" "true"

  HOOK_HANDLER_COUNT="$(count_codex_hook_handlers "$RENDERED_HOOKS_FILE")"
  HOOK_EVENT_COUNT="$(jq -r '.hooks | length' "$RENDERED_HOOKS_FILE")"
  info "Codex hooks installed (${HOOK_HANDLER_COUNT} handlers across ${HOOK_EVENT_COUNT} events)"
  echo "  Hooks config: $CODEX_HOOKS_FILE"
  echo "  Hook scripts: $HOOKS_DST/"
else
  warn "No codex-hooks.json found — hooks not installed"
fi

info "Native Codex plugin installed"
echo "  Plugin key: $PLUGIN_KEY"
echo "  Plugin root: $PLUGIN_CACHE_ROOT"
echo "  Skills available: $SKILL_COUNT"
echo "  Config updated: $CONFIG_FILE"
if [[ "$(uname -s)" == "Linux" ]] && [[ ! -x /usr/bin/bwrap ]]; then
  warn "Codex could not find system bubblewrap at /usr/bin/bwrap."
  echo "  Install it to avoid the vendored-bubblewrap startup warning:"
  echo "  $(detect_bwrap_install_hint)"
fi
if [[ -n "$LEGACY_BACKUP_DIR" ]]; then
  echo "  Archived overlapping ~/.codex/skills entries to: $LEGACY_BACKUP_DIR"
fi
if [[ -n "$USER_BACKUP_DIR" ]]; then
  echo "  Archived overlapping ~/.agents/skills entries to: $USER_BACKUP_DIR"
fi
info "Install metadata written: $INSTALL_META"
echo ""
echo "Restart Codex to pick up the native plugin."

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

# manifest_doctor_count prints the exact skill count `ao doctor` derives from a
# skills-codex manifest, so the self-test compares against the same number the
# operator's next `ao doctor` will. It mirrors ReadCodexManifestSkillCount in
# cli/internal/quality/skills_codex.go: prefer the explicit `package_count`
# field (which also counts installable compatibility pointers), and fall back to
# len(skills[]) only when package_count is absent or zero. The historical
# 66-vs-62 mismatch (age-txfnl) is exactly that fallback firing on a stale
# installed manifest: the installer counts 66 on-disk SKILL.md dirs while a
# manifest missing package_count makes doctor report the 62 implementation rows
# in skills[]. Prints nothing only when the manifest cannot be read at all.
manifest_doctor_count() {
  local path="$1"
  local pkg

  [[ -f "$path" ]] || return 0

  if command -v jq >/dev/null 2>&1; then
    pkg="$(jq -er '.package_count // empty' "$path" 2>/dev/null || true)"
    if [[ "$pkg" =~ ^[0-9]+$ ]] && [[ "$pkg" -gt 0 ]]; then
      printf '%s\n' "$pkg"
      return
    fi
    jq -r '.skills | length' "$path" 2>/dev/null || true
    return
  fi

  # jq-free fallback. package_count first; else count skills[] entries via the
  # per-entry "source_skill" key (present once per skill, and nowhere else in the
  # manifest — verified against the generated schema).
  pkg="$(sed -n 's/.*"package_count"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$path" | head -1)"
  if [[ "$pkg" =~ ^[0-9]+$ ]] && [[ "$pkg" -gt 0 ]]; then
    printf '%s\n' "$pkg"
    return
  fi
  grep -c '"source_skill"' "$path" 2>/dev/null || echo 0
}

# json_int_field prints the first integer value for a top-level JSON key.
# Used to read back the skill_count we just wrote into the install metadata so
# the self-test compares the persisted value, not an in-memory variable.
json_int_field() {
  local path="$1"
  local key="$2"
  sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" "$path" | head -1
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

remove_toml_key() {
  local file="$1"
  local section="$2"
  local key="$3"
  local tmp

  [[ -f "$file" ]] || return 0

  tmp="$(mktemp)"
  awk \
    -v section="$section" \
    -v key="$key" \
    '
    BEGIN {
      in_section = 0
    }
    {
      if ($0 == section) {
        in_section = 1
        print
        next
      }

      if (in_section && $0 ~ /^\[/) {
        in_section = 0
      }

      if (in_section && $0 ~ ("^[[:space:]]*" key "[[:space:]]*=")) {
        next
      }

      print
    }
    ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

# selftest_codex_plugin verifies the installer's own claims before it prints
# success (age-txfnl). It proves the three numbers `ao doctor` reconciles are
# equal — installed SKILL.md directory count == manifest package_count ==
# recorded metadata skill_count — plus a live config-enable entry and one
# readable sentinel skill. On any mismatch it exits nonzero naming the delta so
# the installer never declares victory over a state doctor will flag. Args:
#   $1 disk_count   on-disk SKILL.md directory count (the number we print)
#   $2 pkg_count    manifest package_count (empty when the field is absent)
selftest_codex_plugin() {
  local disk_count="$1"
  local manifest_count="$2"
  local -a problems=()
  local meta_count sentinel

  # (a) counts agree: disk == manifest count (the number doctor reads) ==
  #     metadata skill_count.
  if [[ -z "$manifest_count" ]]; then
    problems+=("cannot read a skill count from manifest ${PLUGIN_SKILLS_DST}/${SKILL_MANIFEST_NAME} — regenerate with 'bash scripts/refresh-codex-local.sh'")
  elif [[ "$disk_count" != "$manifest_count" ]]; then
    problems+=("installed skill directory count ($disk_count) != manifest skill count ($manifest_count) that 'ao doctor' reads — regenerate with 'bash scripts/refresh-codex-local.sh'")
  fi

  meta_count="$(json_int_field "$INSTALL_META" "skill_count")"
  if [[ -z "$meta_count" ]]; then
    problems+=("install metadata ${INSTALL_META} has no readable skill_count")
  elif [[ "$meta_count" != "$disk_count" ]]; then
    problems+=("install metadata skill_count ($meta_count) != installed skill directory count ($disk_count)")
  fi

  # (b) config enable entry present.
  if [[ ! -f "$CONFIG_FILE" ]]; then
    problems+=("config file ${CONFIG_FILE} was not written")
  elif ! grep -qF "[plugins.\"${PLUGIN_KEY}\"]" "$CONFIG_FILE"; then
    problems+=("config ${CONFIG_FILE} is missing the plugin enable entry [plugins.\"${PLUGIN_KEY}\"]")
  fi

  # (c) at least one sentinel skill file is readable.
  # `-print -quit` (not `| head -1`): find stops itself after the first hit, so it
  # never writes into a pipe head() has already closed. With `set -o pipefail`,
  # `find … | head -1` SIGPIPEs (exit 141) once find's output exceeds one stdio
  # flush (~4KB) — which the long installed-cache paths × 66 skills now do — and
  # under `set -e` that killed the whole installer silently right after the last
  # info() line (the CI 992/994/998/848 regression). This form is SIGPIPE-free.
  sentinel="$(find "$PLUGIN_SKILLS_DST" -mindepth 2 -maxdepth 2 -name SKILL.md -print -quit 2>/dev/null)"
  if [[ -z "$sentinel" || ! -r "$sentinel" ]]; then
    problems+=("no readable sentinel SKILL.md under ${PLUGIN_SKILLS_DST}")
  fi

  if [[ ${#problems[@]} -gt 0 ]]; then
    echo "" >&2
    warn "Self-test FAILED — install left on disk for inspection but NOT healthy:"
    local p
    for p in "${problems[@]}"; do
      echo -e "  ${RED}✗${NC} $p" >&2
    done
    fail "Codex plugin self-test failed with ${#problems[@]} problem(s); refusing to report success."
  fi

  info "Self-test passed: $disk_count skills; manifest, metadata, and config are consistent (ao doctor will agree)"
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
remove_toml_key "$CONFIG_FILE" "[features]" "codex_hooks"

INSTALLED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MANIFEST_HASH="$(sha256_file "$PLUGIN_SKILLS_SRC/$SKILL_MANIFEST_NAME")"
require_path "$PLUGIN_SKILLS_DST/$SKILL_MANIFEST_NAME" "installed Codex skill manifest"
INSTALLED_MANIFEST_HASH="$(sha256_file "$PLUGIN_SKILLS_DST/$SKILL_MANIFEST_NAME")"
[[ "$MANIFEST_HASH" == "$INSTALLED_MANIFEST_HASH" ]] || fail "Installed plugin cache manifest hash mismatch; expected $MANIFEST_HASH, got $INSTALLED_MANIFEST_HASH"
SKILL_COUNT="$(find "$PLUGIN_SKILLS_DST" -mindepth 2 -maxdepth 2 -name SKILL.md 2>/dev/null | wc -l | tr -d ' ')"
HOOK_RUNTIME="hookless-default"
HOOKS_INSTALLED=false

archive_legacy_codex_skills
archive_user_raw_skills

cat > "$PLUGIN_STATE_FILE" <<EOF
{
  "installed_at": "$INSTALLED_AT",
  "install_mode": "native-plugin",
  "hook_runtime": "$HOOK_RUNTIME",
  "hooks_installed": $HOOKS_INSTALLED,
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
  "hook_runtime": "$HOOK_RUNTIME",
  "hooks_installed": $HOOKS_INSTALLED,
  "lifecycle_commands": ["ao session bootstrap", "ao gate check"],
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

# ── Hookless default ──
# AgentOps 3.0 ships zero hooks: the Codex lifecycle is driven by skills + the
# `ao` CLI. Ensure any stale Codex hooks feature flag is disabled.
remove_toml_key "$CONFIG_FILE" "[features]" "hooks"
info "Codex hooks not installed (hookless — skills + ao CLI only)"

# ── Self-test: verify our own claims before declaring success (age-txfnl) ──
MANIFEST_DOCTOR_COUNT="$(manifest_doctor_count "$PLUGIN_SKILLS_DST/$SKILL_MANIFEST_NAME")"
selftest_codex_plugin "$SKILL_COUNT" "$MANIFEST_DOCTOR_COUNT"

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
echo "Verify it worked: restart Codex, then type /plan to confirm the skills are live."

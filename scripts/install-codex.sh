#!/usr/bin/env bash
# install-codex.sh — Install AgentOps into the local Codex native plugin cache
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash -s -- --with-hooks
#
# What it does:
#   1. Downloads a temporary AgentOps archive (no local git clone)
#   2. Builds the generated Codex-native skill bundle
#   3. Refreshes the native Codex plugin cache in ~/.codex/plugins/cache
#   4. Archives stale raw skill mirrors in ~/.agents/skills and ~/.codex/skills when found
#   5. Enables the plugin in ~/.codex/config.toml
#
# Update policy:
#   Re-run this installer when new AgentOps releases land.

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }
fail()  { echo -e "${RED}✗${NC} $*"; exit 1; }

UPDATE_CMD="curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash"
SOURCE_ROOT_OVERRIDE="${AGENTOPS_BUNDLE_ROOT:-}"
INSTALL_REF="${AGENTOPS_INSTALL_REF:-main}"
WITH_HOOKS="${AGENTOPS_INSTALL_HOOKS:-0}"

usage() {
  cat <<'EOF'
install-codex.sh

Install AgentOps into the local Codex native plugin cache.

Options:
  --with-hooks  Also install native Codex hooks into CODEX_HOME/hooks.json
  --no-hooks    Do not install hooks (default)
  --help        Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-hooks)
      WITH_HOOKS=1
      shift
      ;;
    --no-hooks)
      WITH_HOOKS=0
      shift
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

WITH_HOOKS_NORMALIZED="$(printf '%s' "$WITH_HOOKS" | tr '[:upper:]' '[:lower:]')"
case "$WITH_HOOKS_NORMALIZED" in
  1|true|yes|on)
    WITH_HOOKS=1
    ;;
  0|false|no|off|"")
    WITH_HOOKS=0
    ;;
  *)
    fail "Invalid AGENTOPS_INSTALL_HOOKS/--with-hooks value: $WITH_HOOKS"
    ;;
esac

if [[ "$INSTALL_REF" == "main" ]]; then
  ARCHIVE_URL="https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/main"
else
  ARCHIVE_URL="https://codeload.github.com/boshu2/agentops/tar.gz/refs/tags/$INSTALL_REF"
fi

echo "Installing AgentOps for Codex..."
echo ""

for cmd in curl tar; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "Missing required command: $cmd"
  fi
done

if ! command -v codex >/dev/null 2>&1; then
  warn "Codex CLI not found in PATH. Install from https://github.com/openai/codex"
  warn "Continuing anyway — skills will be ready when Codex is installed."
fi

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

if [[ -n "$SOURCE_ROOT_OVERRIDE" ]]; then
  SRC_ROOT="$SOURCE_ROOT_OVERRIDE"
  info "Using provided AgentOps bundle: $SRC_ROOT"
else
  ARCHIVE_FILE="${TMP_DIR}/agentops.tar.gz"
  info "Downloading AgentOps bundle..."
  curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_FILE"

  tar -tzf "$ARCHIVE_FILE" > "${TMP_DIR}/archive-files.txt"
  ARCHIVE_ROOT="$(awk -F/ 'NR == 1 { print $1 }' "${TMP_DIR}/archive-files.txt")"
  [ -n "$ARCHIVE_ROOT" ] || fail "Could not determine archive root directory"
  tar -xzf "$ARCHIVE_FILE" -C "$TMP_DIR"
  SRC_ROOT="${TMP_DIR}/${ARCHIVE_ROOT}"
fi

[ -f "$SRC_ROOT/scripts/install-codex-plugin.sh" ] || fail "Native Codex installer not found in bundle"

# skills-codex/ is pre-built in the bundle (manually maintained, no sync needed)

plugin_args=(
  --repo-root "$SRC_ROOT"
  --version "$INSTALL_REF"
  --update-command "$UPDATE_CMD"
)
if [[ "$WITH_HOOKS" == "1" ]]; then
  plugin_args+=(--with-hooks)
else
  plugin_args+=(--no-hooks)
fi

bash "$SRC_ROOT/scripts/install-codex-plugin.sh" "${plugin_args[@]}"

echo ""
if [[ "$WITH_HOOKS" != "1" ]]; then
  echo "Hook note:"
  echo "  AgentOps installs hookless by default. To opt into native Codex hooks:"
  echo "  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash -s -- --with-hooks"
  echo ""
fi
echo "Update note:"
echo "  AgentOps ships frequent updates."
echo "  Re-run this installer regularly to pick up the latest main branch changes:"
echo "  $UPDATE_CMD"

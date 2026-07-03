#!/usr/bin/env bash
# install-agy.sh - Install the AgentOps Gemini/Antigravity image bundle.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash -s -- --ref v3.2.0

set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

INSTALL_REF="${AGENTOPS_INSTALL_REF:-main}"
SOURCE_ROOT_OVERRIDE="${AGENTOPS_BUNDLE_ROOT:-}"
PLUGIN_NAME="agentops-core-gemini"
DRY_RUN=0
VALIDATE_ONLY=0
NO_ENABLE=0
QUIET=0
TMP_DIR=""

usage() {
  cat <<'EOF'
install-agy.sh

Install the AgentOps Gemini/Antigravity image bundle.

Usage:
  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash
  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash -s -- --ref v3.2.0

Options:
  --ref <ref>       Git ref to install. Defaults to AGENTOPS_INSTALL_REF or main.
  --validate-only   Validate the image bundle but do not install it.
  --no-enable       Install the plugin but skip 'agy plugin enable'.
  --dry-run         Print the commands that would run without changing anything.
  --quiet           Reduce progress output.
  --help            Show this help.

Environment:
  AGENTOPS_BUNDLE_ROOT=/path/to/agentops  Use an already extracted checkout.
EOF
}

info() {
  [ "$QUIET" -eq 1 ] && return 0
  printf 'INFO: %s\n' "$*"
}

ok() {
  [ "$QUIET" -eq 1 ] && return 0
  printf 'OK: %s\n' "$*"
}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    printf 'DRY-RUN:'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

resolve_source_root() {
  if [ -n "$SOURCE_ROOT_OVERRIDE" ]; then
    [ -f "$SOURCE_ROOT_OVERRIDE/images/gemini/plugin.json" ] || fail "AGENTOPS_BUNDLE_ROOT does not contain images/gemini/plugin.json: $SOURCE_ROOT_OVERRIDE"
    printf '%s\n' "$SOURCE_ROOT_OVERRIDE"
    return 0
  fi

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" >/dev/null 2>&1 && pwd || true)"
  if [ -n "$script_dir" ] && [ -f "$script_dir/../images/gemini/plugin.json" ]; then
    cd "$script_dir/.." && pwd
    return 0
  fi

  TMP_DIR="$(mktemp -d)"
  local archive_file="$TMP_DIR/agentops.tar.gz"
  local archive_url
  case "$INSTALL_REF" in
    main)
      archive_url="https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/main"
      ;;
    refs/heads/*|refs/tags/*)
      archive_url="https://codeload.github.com/boshu2/agentops/tar.gz/$INSTALL_REF"
      ;;
    v[0-9]*)
      archive_url="https://codeload.github.com/boshu2/agentops/tar.gz/refs/tags/$INSTALL_REF"
      ;;
    *)
      archive_url="https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/$INSTALL_REF"
      ;;
  esac

  info "Downloading AgentOps bundle ($INSTALL_REF)"
  curl -fsSL "$archive_url" -o "$archive_file"
  tar -xzf "$archive_file" -C "$TMP_DIR"
  find "$TMP_DIR" -mindepth 1 -maxdepth 1 -type d | head -1
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --ref)
      INSTALL_REF="${2:-}"
      [ -n "$INSTALL_REF" ] || fail "--ref requires a value"
      shift 2
      ;;
    --validate-only)
      VALIDATE_ONLY=1
      shift
      ;;
    --no-enable)
      NO_ENABLE=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --quiet)
      QUIET=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

info "Installing AgentOps Gemini/Antigravity image"

if [ "$DRY_RUN" -eq 0 ]; then
  command -v curl >/dev/null 2>&1 || fail "curl is required"
  command -v tar >/dev/null 2>&1 || fail "tar is required"
  command -v jq >/dev/null 2>&1 || fail "jq is required"
  command -v agy >/dev/null 2>&1 || fail "agy CLI not found in PATH. Install Antigravity/agy first, then rerun this installer."
fi

if [ "$DRY_RUN" -eq 1 ]; then
  printf 'DRY-RUN: resolve AgentOps bundle ref %q\n' "$INSTALL_REF"
  run bash images/gemini/verify.sh
  run agy plugin validate images/gemini
  if [ "$VALIDATE_ONLY" -eq 0 ]; then
    run agy plugin install images/gemini
    [ "$NO_ENABLE" -eq 1 ] || run agy plugin enable "$PLUGIN_NAME"
  fi
  ok "dry run complete"
  exit 0
fi

SRC_ROOT="$(resolve_source_root)"
PLUGIN_DIR="$SRC_ROOT/images/gemini"

[ -f "$PLUGIN_DIR/plugin.json" ] || fail "missing Gemini plugin manifest: $PLUGIN_DIR/plugin.json"
[ -x "$PLUGIN_DIR/verify.sh" ] || fail "missing executable Gemini verifier: $PLUGIN_DIR/verify.sh"

run bash "$PLUGIN_DIR/verify.sh"
run agy plugin validate "$PLUGIN_DIR"

if [ "$VALIDATE_ONLY" -eq 1 ]; then
  ok "Gemini/Antigravity image validated"
  exit 0
fi

run agy plugin install "$PLUGIN_DIR"
if [ "$NO_ENABLE" -eq 0 ]; then
  # "agy plugin enable" exits 1 if the plugin is already enabled (idempotent install
  # scenario); treat that as success so a clean fresh install exits 0.
  enable_out="$(agy plugin enable "$PLUGIN_NAME" 2>&1)" || {
    if printf '%s' "$enable_out" | grep -q "already enabled"; then
      info "plugin $PLUGIN_NAME already enabled — skipping enable"
    else
      printf '%s\n' "$enable_out" >&2
      exit 1
    fi
  }
fi

ok "AgentOps Gemini/Antigravity plugin is installed"
printf 'Next: restart Antigravity/Gemini, then run /quickstart.\n'

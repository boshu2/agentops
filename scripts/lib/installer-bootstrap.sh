#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/installer-bootstrap.sh — load installer-common.sh locally or via curl.
#
# Usage (near the top of an installer, after set -euo pipefail is fine):
#   BOOTSTRAP="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/installer-bootstrap.sh"
#   # shellcheck source=lib/installer-bootstrap.sh
#   if [[ -f "$BOOTSTRAP" ]]; then . "$BOOTSTRAP"; else
#     # curl|bash fallback when this file itself was streamed
#     ...
#   fi
#   agentops_source_installer_common
#
# Or call agentops_source_installer_common after defining a minimal inline copy
# of this function (see public installers).

agentops_source_installer_common() {
  if [[ "${AGENTOPS_INSTALLER_COMMON_LOADED:-0}" == "1" ]]; then
    return 0
  fi

  local candidates=()
  local src dir c url tmp
  src="${BASH_SOURCE[1]:-${BASH_SOURCE[0]:-}}"

  if [[ -n "$src" && -f "$src" ]]; then
    # shellcheck disable=SC1007
    dir="$(CDPATH= cd "$(dirname "$src")" && pwd)"
    candidates+=("$dir/lib/installer-common.sh")
    candidates+=("$dir/../scripts/lib/installer-common.sh")
    candidates+=("$dir/installer-common.sh")
  fi
  if [[ -n "${AGENTOPS_BUNDLE_ROOT:-}" ]]; then
    candidates+=("$AGENTOPS_BUNDLE_ROOT/scripts/lib/installer-common.sh")
  fi
  # When this bootstrap lives next to installer-common.sh
  if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
    # shellcheck disable=SC1007
    dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    candidates+=("$dir/installer-common.sh")
  fi

  for c in "${candidates[@]}"; do
    if [[ -f "$c" ]]; then
      # shellcheck disable=SC1090
      . "$c"
      return 0
    fi
  done

  url="${AGENTOPS_INSTALLER_COMMON_URL:-https://raw.githubusercontent.com/boshu2/agentops/main/scripts/lib/installer-common.sh}"
  tmp="$(mktemp "${TMPDIR:-/tmp}/agentops-installer-common.XXXXXX")"
  # curl honors HTTPS_PROXY/HTTP_PROXY from the environment natively.
  if command -v curl >/dev/null 2>&1 && curl -fsSL --connect-timeout 10 "${url}?$(date +%s)" -o "$tmp"; then
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
    return 0
  fi
  rm -f "$tmp"
  printf 'FATAL: could not load installer-common.sh (tried local paths and %s)\n' "$url" >&2
  exit 1
}

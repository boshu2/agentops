#!/usr/bin/env bash
# install-bd.sh — install the `bd` (beads) CLI to ~/.local/bin/bd.
#
# Beads has been "unavailable" in three consecutive nightly runs (2026-04-26
# retro task 5). Upstream publishes binaries for darwin/linux/windows under
# https://github.com/steveyegge/beads/releases. This script detects the
# platform, downloads the matching tarball, verifies checksums when published,
# and verifies the binary launches.
#
# One-liner (cache buster):
#   curl -fsSL "https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-bd.sh?$(date +%s)" | bash
#
# Idempotent: re-running with bd already on PATH at the requested version is
# a no-op (exit 0). Use --force to redownload.
#
# Examples:
#   scripts/install-bd.sh                   # install latest tagged release
#   scripts/install-bd.sh --version v1.0.3  # pin a version
#   scripts/install-bd.sh --force           # redownload even if installed
#   scripts/install-bd.sh --offline /path/to/beads_tarball.tar.gz
#   scripts/install-bd.sh --quiet --no-gum
#
# Exit codes:
#   0   bd installed (or already present at the requested version)
#   1   download or extraction failed
#   2   unsupported platform / arch
#   3   verification failed (binary did not launch)

set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

BD_REPO="steveyegge/beads"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION=""
FORCE_INSTALL=0
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
QUIET=0
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
NO_GUM=0
NO_VERIFY=0
OFFLINE=0
OFFLINE_TARBALL=""
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
EASY_MODE=0
FROM_SOURCE=0
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
INSTALLER_NAME="bd"
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
INSTALLER_TAGLINE="Install the beads (bd) CLI"

# ── Load installer-workmanship common scaffold ────────────────────────────
# Pin of scripts/lib/installer-common.sh for the curl|bash path. The drift
# test in tests/scripts/install-bd.bats recomputes this on every change, so
# an edit to installer-common.sh without a matching bump here fails CI.
INSTALLER_COMMON_SHA256="6de10b1997a17e546cf4a9a75943b257de66df20708e881431917239b4bb2e28"

_sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo ""
  fi
}

_load_installer_common() {
  local here
  # shellcheck disable=SC1007
  here="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"
  if [[ -n "$here" && -f "$here/lib/installer-bootstrap.sh" ]]; then
    # shellcheck source=lib/installer-bootstrap.sh
    . "$here/lib/installer-bootstrap.sh"
    agentops_source_installer_common
    return 0
  fi
  # curl|bash: fetch common lib (curl honors HTTPS_PROXY/HTTP_PROXY)
  local url tmp actual
  url="${AGENTOPS_INSTALLER_COMMON_URL:-https://raw.githubusercontent.com/boshu2/agentops/main/scripts/lib/installer-common.sh}"
  tmp="$(mktemp "${TMPDIR:-/tmp}/agentops-installer-common.XXXXXX")"
  if command -v curl >/dev/null 2>&1 && curl -fsSL --connect-timeout 10 "${url}?$(date +%s)" -o "$tmp"; then
    if [[ -z "${AGENTOPS_INSTALLER_COMMON_URL:-}" ]]; then
      # Default URL: verify the fetched lib against the pin before sourcing.
      # A custom AGENTOPS_INSTALLER_COMMON_URL is an explicit operator
      # override and is sourced as supplied.
      actual="$(_sha256_of "$tmp")"
      if [[ -z "$actual" ]]; then
        rm -f "$tmp"
        echo "FATAL: no sha256sum/shasum available to verify installer-common.sh" >&2
        exit 1
      fi
      if [[ "$actual" != "$INSTALLER_COMMON_SHA256" ]]; then
        rm -f "$tmp"
        echo "FATAL: installer-common.sh checksum mismatch (got $actual)" >&2
        echo "Re-fetch this installer from the repo — the two files ship and update together." >&2
        exit 1
      fi
    fi
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
    return 0
  fi
  rm -f "$tmp"
  echo "FATAL: could not load installer-common.sh" >&2
  exit 1
}
_load_installer_common

# Cosign identity targets the beads upstream repo (not AgentOps).
# shellcheck disable=SC2034  # all four consumed by the sourced installer-common.sh
OWNER="steveyegge"
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
REPO="beads"
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
INSTALLER_NAME="bd"
# shellcheck disable=SC2034  # consumed by the sourced installer-common.sh
INSTALLER_TAGLINE="Install the beads (bd) CLI"

usage() {
  cat <<EOF
install-bd.sh — install the \`bd\` (beads) CLI

One-liner:
  curl -fsSL "https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-bd.sh?\$(date +%s)" | bash

Options:
  --version <tag>     Install a specific release tag (default: latest)
  --install-dir <dir> Install directory (default: ~/.local/bin)
  --from-source       Build with \`go install\` instead of downloading a binary
$(common_installer_flags_help)
  -h, --help          Show this help

Uninstall:
  rm -f "\${INSTALL_DIR:-$HOME/.local/bin}/bd"
EOF
}

while [[ $# -gt 0 ]]; do
  if parse_common_installer_flag "$@"; then
    shift "$CONSUMED"
    continue
  fi
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --from-source) FROM_SOURCE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 1 ;;
  esac
done

FORCE="$FORCE_INSTALL"

print_branded_header "bd installer" "Install the beads (bd) CLI"
setup_proxy
detect_platform_go

if [[ -z "${GOOS:-}" || -z "${GOARCH:-}" ]]; then
  err "unsupported OS/arch: $(uname -s)/$(uname -m)"
  exit 2
fi

acquire_install_lock "bd"

# --- resolve version ---
resolve_version() {
  if [[ -n "$VERSION" ]]; then
    return 0
  fi
  if [[ "$OFFLINE" -eq 1 && -n "$OFFLINE_TARBALL" ]]; then
    VERSION="offline"
    return 0
  fi
  if ! command -v curl >/dev/null 2>&1; then
    fail "curl is required to resolve latest version"
  fi
  # Tier 1: GitHub API
  VERSION="$(curl -fsSL --connect-timeout 5 "${PROXY_ARGS[@]}" \
    "https://api.github.com/repos/${BD_REPO}/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1 || true)"
  if [[ -n "$VERSION" ]]; then
    return 0
  fi
  # Tier 2: redirect URL
  VERSION="$(curl -fsSL --connect-timeout 5 -o /dev/null -w '%{url_effective}' "${PROXY_ARGS[@]}" \
    "https://github.com/${BD_REPO}/releases/latest" 2>/dev/null \
    | sed -E 's|.*/tag/||' || true)"
  if [[ -z "$VERSION" ]]; then
    fail "could not resolve latest beads release tag"
  fi
}

run_with_spinner "Resolving latest beads version..." resolve_version
info "Target version: $VERSION ($GOOS/$GOARCH)"

# --- short-circuit if already installed at the requested version ---
if [[ "$FORCE" -eq 0 && "$FROM_SOURCE" -eq 0 && "$VERSION" != "offline" ]] \
  && command -v bd >/dev/null 2>&1; then
  have="$(bd version 2>&1 | head -1 || true)"
  if [[ "$have" == *"${VERSION#v}"* ]]; then
    ok "bd ${VERSION} already installed at $(command -v bd) — skipping (--force to override)"
    summary_add "bd: already installed ($VERSION)"
    print_summary "bd already present"
    print_uninstall_hint "rm -f $(command -v bd)"
    exit 0
  fi
fi

preflight_checks "$INSTALL_DIR" "https://github.com/${BD_REPO}"

tmp_dir="$(installer_mktemp_dir "bd-install")"

build_from_source() {
  if ! command -v go >/dev/null 2>&1; then
    fail "go is required for --from-source (or when no prebuilt binary is available)"
  fi
  local mod="github.com/${BD_REPO}/cmd/bd"
  local ver="$VERSION"
  [[ "$ver" == "offline" || -z "$ver" ]] && ver="latest"
  info "Building bd from source ($mod@$ver)..."
  GOBIN="$tmp_dir" run_with_spinner "go install $mod@$ver..." \
    go install "${mod}@${ver}"
  if [[ ! -x "$tmp_dir/bd" ]]; then
    fail "go install did not produce $tmp_dir/bd"
  fi
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$tmp_dir/bd" "$INSTALL_DIR/bd"
  ok "Built and installed bd from source → $INSTALL_DIR/bd"
}

fetch_checksum_for_asset() {
  local asset="$1"
  local ver="$2"
  local sums_url candidate expected
  # Try common checksum artifact names published next to the release asset.
  for candidate in \
    "checksums.txt" \
    "SHA256SUMS" \
    "sha256sums.txt" \
    "beads_${ver#v}_checksums.txt" \
    "${asset}.sha256"
  do
    sums_url="https://github.com/${BD_REPO}/releases/download/${ver}/${candidate}"
    if curl -fsSL --connect-timeout 5 "${PROXY_ARGS[@]}" "$sums_url" -o "$tmp_dir/sums.txt" 2>/dev/null; then
      if [[ "$candidate" == *.sha256 ]]; then
        expected="$(awk '{print $1}' "$tmp_dir/sums.txt" | head -1)"
      else
        expected="$(awk -v a="$asset" '$2 == a || $2 == ("*" a) || $NF == a {print $1; exit}' "$tmp_dir/sums.txt")"
      fi
      if [[ -n "$expected" ]]; then
        printf '%s\n' "$expected"
        return 0
      fi
    fi
  done
  return 1
}

install_from_tarball() {
  local archive="$1"
  [[ -f "$archive" ]] || fail "Tarball not found: $archive"

  info "extracting $(basename "$archive")"
  if ! tar -xzf "$archive" -C "$tmp_dir"; then
    fail "extraction failed"
  fi

  local binary="" candidate
  for candidate in "$tmp_dir/bd" "$tmp_dir/beads" "$tmp_dir"/*/bd "$tmp_dir"/*/beads; do
    if [[ -f "$candidate" && -x "$candidate" ]]; then
      binary="$candidate"
      break
    fi
  done
  if [[ -z "$binary" ]]; then
    err "could not locate bd binary in tarball"
    ls -R "$tmp_dir" >&2 || true
    exit 1
  fi

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$binary" "$INSTALL_DIR/bd"
  ok "installed bd to $INSTALL_DIR/bd"
}

download_and_install() {
  if [[ -n "$OFFLINE_TARBALL" ]]; then
    install_from_tarball "$OFFLINE_TARBALL"
    return 0
  fi

  if [[ "$FROM_SOURCE" -eq 1 ]]; then
    build_from_source
    return 0
  fi

  local ver_no_v="${VERSION#v}"
  local asset="beads_${ver_no_v}_${GOOS}_${GOARCH}.tar.gz"
  local url="https://github.com/${BD_REPO}/releases/download/${VERSION}/${asset}"
  local archive="$tmp_dir/$asset"

  info "downloading $url"
  if ! run_with_spinner "Downloading $asset..." \
    curl -fsSL --connect-timeout 30 "${PROXY_ARGS[@]}" "$url" -o "$archive"; then
    warn "download failed: $url"
    warn "Falling back to build-from-source"
    build_from_source
    return 0
  fi

  local expected=""
  if expected="$(fetch_checksum_for_asset "$asset" "$VERSION")"; then
    verify_checksum "$archive" "$expected" || exit 1
  else
    if [[ "$NO_VERIFY" -eq 1 ]]; then
      warn "No published checksum found; continuing because --no-verify was set"
    else
      err "No published checksum found for $asset; refusing to install an unverified binary"
      err "Re-run with --no-verify to accept the download without SHA256 verification"
      exit 1
    fi
  fi

  # Best-effort Sigstore (soft-skip without cosign / bundle)
  verify_sigstore "$archive" \
    "https://github.com/${BD_REPO}/releases/download/${VERSION}/${asset}.sigstore.json" \
    || true

  install_from_tarball "$archive"
}

download_and_install

# --- verify ---
target="$INSTALL_DIR/bd"
if ! "$target" version >/dev/null 2>&1; then
  err "verification failed: $target version did not launch"
  exit 3
fi

actual="$("$target" version 2>&1 | head -1)"
ok "verified: $actual"
maybe_add_path "$INSTALL_DIR"

summary_add "Binary: $target"
summary_add "Version: $actual"
summary_add "Platform: ${GOOS}/${GOARCH}"
print_summary "bd install complete"
print_uninstall_hint "rm -f $target"

exit 0

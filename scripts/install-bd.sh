#!/usr/bin/env bash
# install-bd.sh — install the `bd` (beads) CLI to ~/.local/bin/bd.
#
# Beads has been "unavailable" in three consecutive nightly runs (2026-04-26
# retro task 5). Upstream publishes signed binaries for darwin/linux/windows
# under https://github.com/steveyegge/beads/releases. This script detects the
# platform, downloads the matching tarball, and verifies the binary launches.
#
# Idempotent: re-running with bd already on PATH at the requested version is
# a no-op (exit 0). Use --force to redownload.
#
# Examples:
#   scripts/install-bd.sh                   # install latest tagged release
#   scripts/install-bd.sh --version v1.0.3  # pin a version
#   scripts/install-bd.sh --force           # redownload even if installed
#
# Exit codes:
#   0   bd installed (or already present at the requested version)
#   1   download or extraction failed
#   2   unsupported platform / arch
#   3   verification failed (binary did not launch)

set -euo pipefail

REPO="steveyegge/beads"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION=""
FORCE=0

usage() {
    sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        --force)   FORCE=1; shift ;;
        --install-dir) INSTALL_DIR="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "unknown flag: $1" >&2; exit 1 ;;
    esac
done

# --- detect platform ---
uname_s="$(uname -s)"
uname_m="$(uname -m)"
case "$uname_s" in
    Darwin)      os="darwin" ;;
    Linux)       os="linux" ;;
    *)           echo "unsupported OS: $uname_s" >&2; exit 2 ;;
esac
case "$uname_m" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)            echo "unsupported arch: $uname_m" >&2; exit 2 ;;
esac

# --- resolve version ---
if [[ -z "$VERSION" ]]; then
    if ! command -v curl >/dev/null 2>&1; then
        echo "curl is required to resolve latest version" >&2
        exit 1
    fi
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
    if [[ -z "$VERSION" ]]; then
        echo "could not resolve latest beads release tag" >&2
        exit 1
    fi
fi

# --- short-circuit if already installed at the requested version ---
if [[ "$FORCE" -eq 0 ]] && command -v bd >/dev/null 2>&1; then
    have="$(bd version 2>&1 | head -1 || true)"
    # bd version output is like "bd version 1.0.3 (Homebrew)"
    if [[ "$have" == *"${VERSION#v}"* ]]; then
        echo "bd ${VERSION} already installed at $(command -v bd) — skipping (--force to override)"
        exit 0
    fi
fi

# --- download and extract ---
ver_no_v="${VERSION#v}"
asset="beads_${ver_no_v}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "downloading $url"
if ! curl -fsSL "$url" -o "$tmp_dir/$asset"; then
    echo "download failed: $url" >&2
    exit 1
fi

echo "extracting $asset"
if ! tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"; then
    echo "extraction failed" >&2
    exit 1
fi

# --- find the bd binary in the extracted tree ---
binary=""
for candidate in "$tmp_dir/bd" "$tmp_dir/beads" "$tmp_dir"/*/bd "$tmp_dir"/*/beads; do
    if [[ -f "$candidate" && -x "$candidate" ]]; then
        binary="$candidate"
        break
    fi
done
if [[ -z "$binary" ]]; then
    echo "could not locate bd binary in tarball" >&2
    ls -R "$tmp_dir" >&2
    exit 1
fi

# --- install ---
mkdir -p "$INSTALL_DIR"
target="$INSTALL_DIR/bd"
cp "$binary" "$target"
chmod +x "$target"
echo "installed bd to $target"

# --- verify ---
if ! "$target" version >/dev/null 2>&1; then
    echo "verification failed: $target version did not launch" >&2
    exit 3
fi

actual="$("$target" version 2>&1 | head -1)"
echo "verified: $actual"

# --- PATH hint ---
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo
        echo "note: $INSTALL_DIR is not on \$PATH. Add this to your shell rc:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac

exit 0

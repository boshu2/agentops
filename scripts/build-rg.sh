#!/usr/bin/env bash
# build-rg.sh — Build ripgrep from source with PCRE2 + release optimizations.
#
# Why: Distro-packaged ripgrep frequently omits PCRE2, which is required for
# `rg -P` patterns (lookahead, lookbehind, atomic groups, backreferences).
# Building from upstream BurntSushi/ripgrep with --features pcre2 unlocks those
# patterns and produces a binary tuned for the local CPU.
#
# Usage:
#   bash scripts/build-rg.sh                  # default: ripgrep 14.1.1 -> ~/.local/bin/rg
#   RG_VERSION=14.1.1 bash scripts/build-rg.sh
#   RG_INSTALL_DIR=/usr/local/bin bash scripts/build-rg.sh
#   ASSUME_YES=1 bash scripts/build-rg.sh     # non-interactive (no rustup prompt)
#
# Idempotent: re-running upgrades to the requested tag, reuses an existing
# clone, and replaces the installed binary in place.

set -euo pipefail

RG_VERSION="${RG_VERSION:-14.1.1}"
RG_INSTALL_DIR="${RG_INSTALL_DIR:-$HOME/.local/bin}"
RG_SRC_DIR="${RG_SRC_DIR:-$HOME/.cache/agentops/ripgrep-src}"
ASSUME_YES="${ASSUME_YES:-0}"
REPO_URL="https://github.com/BurntSushi/ripgrep.git"

log() { printf '[build-rg] %s\n' "$*"; }
err() { printf '[build-rg] ERROR: %s\n' "$*" >&2; }

# --- OS gate -----------------------------------------------------------------
case "$(uname -s)" in
  Linux|Darwin) ;;
  *)
    err "unsupported OS '$(uname -s)'. This script supports Linux and macOS only."
    err "On Windows, install ripgrep via 'winget install BurntSushi.ripgrep.MSVC' or 'choco install ripgrep'."
    exit 2
    ;;
esac

# --- Confirm helper ----------------------------------------------------------
confirm() {
  local prompt="$1"
  if [[ "$ASSUME_YES" == "1" ]]; then
    log "auto-confirm: $prompt"
    return 0
  fi
  if [ ! -t 0 ]; then
    err "stdin is not a tty; refusing to prompt for: $prompt"
    err "Set ASSUME_YES=1 to auto-confirm in non-interactive contexts."
    exit 3
  fi
  printf '[build-rg] %s [y/N] ' "$prompt"
  local reply
  read -r reply
  [[ "$reply" =~ ^[Yy]$ ]]
}

# --- Rust toolchain ----------------------------------------------------------
if ! command -v cargo >/dev/null 2>&1; then
  log "cargo not found on PATH."
  if confirm "Install rust toolchain via rustup?"; then
    if ! command -v curl >/dev/null 2>&1; then
      err "curl is required to install rustup but is not on PATH."
      exit 3
    fi
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
    # shellcheck disable=SC1091
    source "$HOME/.cargo/env"
  else
    err "cargo is required. Aborting."
    exit 3
  fi
fi

log "cargo: $(cargo --version)"

# --- Source clone / update ---------------------------------------------------
mkdir -p "$(dirname "$RG_SRC_DIR")"

if [[ -d "$RG_SRC_DIR/.git" ]]; then
  log "updating existing ripgrep source at $RG_SRC_DIR"
  git -C "$RG_SRC_DIR" fetch --tags --force origin
else
  log "cloning ripgrep -> $RG_SRC_DIR"
  git clone "$REPO_URL" "$RG_SRC_DIR"
fi

log "checking out tag $RG_VERSION"
if ! git -C "$RG_SRC_DIR" rev-parse --verify --quiet "refs/tags/$RG_VERSION" >/dev/null; then
  err "tag '$RG_VERSION' not found in $REPO_URL"
  err "list available tags with: git -C '$RG_SRC_DIR' tag --list | tail"
  exit 4
fi
git -C "$RG_SRC_DIR" checkout --quiet --force "refs/tags/$RG_VERSION"

# --- Build -------------------------------------------------------------------
# Build flags rationale:
#   --release       : optimized profile (no debug, no asserts).
#   --features pcre2: links libpcre2 for `rg -P` lookahead/lookbehind/atomic.
#   The historical 'simd-accel' feature was removed upstream; ripgrep's regex
#   engine now picks SIMD lanes at runtime, so no extra feature is needed.
log "building ripgrep $RG_VERSION (release, pcre2)"
(
  cd "$RG_SRC_DIR"
  cargo build --release --features 'pcre2'
)

BUILT_BIN="$RG_SRC_DIR/target/release/rg"
if [[ ! -x "$BUILT_BIN" ]]; then
  err "expected built binary at $BUILT_BIN but none was found"
  exit 5
fi

# --- Install -----------------------------------------------------------------
mkdir -p "$RG_INSTALL_DIR"
INSTALLED_BIN="$RG_INSTALL_DIR/rg"

# Replace via temp + mv to avoid 'Text file busy' if the binary is mapped.
TMP_BIN="$(mktemp "${TMPDIR:-/tmp}/rg.XXXXXX")"
cp "$BUILT_BIN" "$TMP_BIN"
chmod +x "$TMP_BIN"
mv -f "$TMP_BIN" "$INSTALLED_BIN"
log "installed: $INSTALLED_BIN"

# --- Smoke test --------------------------------------------------------------
if ! "$INSTALLED_BIN" --version >/dev/null 2>&1; then
  err "installed binary failed to execute"
  exit 6
fi

VERSION_OUT="$("$INSTALLED_BIN" --version)"
log "version output:"
printf '%s\n' "$VERSION_OUT" | sed 's/^/  /'

if ! printf '%s\n' "$VERSION_OUT" | grep -qi 'PCRE2'; then
  err "installed rg does not report PCRE2 support; build feature flag may have been ignored"
  exit 7
fi

log "PCRE2 confirmed. Done."

# Reminder if install dir is not on PATH.
case ":$PATH:" in
  *":$RG_INSTALL_DIR:"*) ;;
  *) log "note: $RG_INSTALL_DIR is not on PATH. Add it to your shell rc to use this rg by default." ;;
esac

# Pattern adopted from `rg-optimized` (jsm/ACFS skill corpus). Methodology only — no verbatim text.

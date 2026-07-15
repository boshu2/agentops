#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/installer-common.sh — shared installer-workmanship scaffold.
#
# Source from an installer (do NOT execute):
#   # shellcheck source=lib/installer-common.sh
#   . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/installer-common.sh"
#
# For curl|bash entrypoints that may not have a sibling lib/, use
# agentops_bootstrap_installer_common (defined below after load, and also
# printable as a short stub — see agentops_installer_bootstrap_snippet).
#
# Provides: gum/ANSI logging, draw_box, spinner, proxy, platform/WSL,
# preflight, mkdir lock, dual SHA256, optional cosign, summary helpers.

# Idempotent load guard
if [[ "${AGENTOPS_INSTALLER_COMMON_LOADED:-0}" == "1" ]]; then
  return 0 2>/dev/null || exit 0
fi
AGENTOPS_INSTALLER_COMMON_LOADED=1

set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

# ── Defaults (callers may override before or after sourcing) ──────────────
: "${QUIET:=0}"
: "${NO_GUM:=0}"
: "${FORCE_INSTALL:=0}"
: "${NO_VERIFY:=0}"
: "${OFFLINE:=0}"
: "${EASY_MODE:=0}"
: "${INSTALLER_NAME:=agentops}"
: "${INSTALLER_TAGLINE:=AgentOps installer}"
: "${OWNER:=boshu2}"
: "${REPO:=agentops}"
: "${MIN_DISK_KB:=10240}"

PROXY_ARGS=()
HAS_GUM=0
OS=""
ARCH=""
TARGET=""
IS_WSL=0
LOCK_DIR=""
INSTALLER_TMPDIRS=""
SUMMARY_LINES=()

# ── Gum detection ─────────────────────────────────────────────────────────
installer_detect_gum() {
  HAS_GUM=0
  if [[ "$NO_GUM" -eq 0 ]] && command -v gum >/dev/null 2>&1 && [[ -t 1 ]]; then
    HAS_GUM=1
  fi
}
installer_detect_gum

# ── Logging (gum path + ANSI fallback) ────────────────────────────────────
info() {
  [[ "$QUIET" -eq 1 ]] && return 0
  if [[ "$HAS_GUM" -eq 1 ]]; then
    gum style --foreground 39 "-> $*"
  else
    printf '\033[0;34m->\033[0m %s\n' "$*"
  fi
}

ok() {
  [[ "$QUIET" -eq 1 ]] && return 0
  if [[ "$HAS_GUM" -eq 1 ]]; then
    gum style --foreground 42 "✓ $*"
  else
    printf '\033[0;32m✓\033[0m %s\n' "$*"
  fi
}

warn() {
  [[ "$QUIET" -eq 1 ]] && return 0
  if [[ "$HAS_GUM" -eq 1 ]]; then
    gum style --foreground 214 "! $*"
  else
    printf '\033[1;33m!\033[0m %s\n' "$*"
  fi
}

err() {
  # Errors always print (no quiet gate)
  if [[ "$HAS_GUM" -eq 1 ]] && [[ -t 2 ]]; then
    gum style --foreground 196 "✗ $*" >&2
  else
    printf '\033[0;31m✗\033[0m %s\n' "$*" >&2
  fi
}

fail() {
  err "$*"
  exit 1
}

# ── Spinner ───────────────────────────────────────────────────────────────
run_with_spinner() {
  local title="$1"
  shift
  if [[ "$HAS_GUM" -eq 1 && "$QUIET" -eq 0 ]]; then
    gum spin --spinner dot --title "$title" -- "$@"
  else
    info "$title"
    "$@"
  fi
}

# ── Box drawing ───────────────────────────────────────────────────────────
_installer_strip_ansi() {
  # shellcheck disable=SC2001
  printf '%s' "$1" | sed $'s/\033\\[[0-9;]*[a-zA-Z]//g'
}

draw_box() {
  local color="${1:-39}"
  shift
  local lines=("$@")
  local max=0 line plain width i pad
  for line in "${lines[@]}"; do
    plain="$(_installer_strip_ansi "$line")"
    width=${#plain}
    if (( width > max )); then
      max=$width
    fi
  done
  if (( max < 20 )); then
    max=20
  fi

  if [[ "$HAS_GUM" -eq 1 && "$QUIET" -eq 0 ]]; then
    local gum_lines=()
    for line in "${lines[@]}"; do
      gum_lines+=("$(gum style --foreground "$color" "$line")")
    done
    gum style --border normal --border-foreground "$color" --padding "0 1" --margin "1 0" \
      "${gum_lines[@]}"
    return 0
  fi

  [[ "$QUIET" -eq 1 ]] && return 0

  pad="$(printf '%*s' $((max + 2)) '' | tr ' ' '═')"
  printf '╔%s╗\n' "$pad"
  for line in "${lines[@]}"; do
    plain="$(_installer_strip_ansi "$line")"
    printf '║ %s%*s ║\n' "$plain" $((max - ${#plain})) ""
  done
  printf '╚%s╝\n' "$pad"
}

print_branded_header() {
  local title="${1:-$INSTALLER_NAME installer}"
  local subtitle="${2:-$INSTALLER_TAGLINE}"
  [[ "$QUIET" -eq 1 ]] && return 0
  if [[ "$HAS_GUM" -eq 1 ]]; then
    gum style \
      --border normal --border-foreground 39 \
      --padding "0 1" --margin "1 0" \
      "$(gum style --foreground 42 --bold "$title")" \
      "$(gum style --foreground 245 "$subtitle")"
  else
    printf '\033[1;32m%s\033[0m\n' "$title"
    printf '\033[0;90m%s\033[0m\n' "$subtitle"
  fi
}

# ── Proxy ─────────────────────────────────────────────────────────────────
setup_proxy() {
  PROXY_ARGS=()
  if [[ -n "${HTTPS_PROXY:-}" ]]; then
    PROXY_ARGS=(--proxy "$HTTPS_PROXY")
    info "Using HTTPS proxy: $HTTPS_PROXY"
  elif [[ -n "${HTTP_PROXY:-}" ]]; then
    PROXY_ARGS=(--proxy "$HTTP_PROXY")
    info "Using HTTP proxy: $HTTP_PROXY"
  fi
}

# ── Platform / WSL ────────────────────────────────────────────────────────
detect_platform() {
  local uname_bin tr_bin uname_s uname_m
  # Prefer absolute paths so constrained-PATH install tests (and airgap
  # environments) still resolve platform without relying on a full PATH.
  uname_bin="/usr/bin/uname"
  [[ -x "$uname_bin" ]] || uname_bin="$(command -v uname 2>/dev/null || true)"
  tr_bin="/usr/bin/tr"
  [[ -x "$tr_bin" ]] || tr_bin="$(command -v tr 2>/dev/null || true)"
  if [[ -z "$uname_bin" || -z "$tr_bin" ]]; then
    warn "uname/tr unavailable; skipping platform detection"
    OS=""
    ARCH=""
    TARGET=""
    return 0
  fi
  uname_s="$("$uname_bin" -s | "$tr_bin" 'A-Z' 'a-z')"
  uname_m="$("$uname_bin" -m)"
  OS="$uname_s"
  case "$uname_m" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="aarch64" ;;
    *) ARCH="$uname_m" ;;
  esac

  IS_WSL=0
  if [[ "$OS" == "linux" ]] && grep -qi microsoft /proc/version 2>/dev/null; then
    IS_WSL=1
    warn "WSL detected. Some features may need additional configuration"
  fi

  case "${OS}-${ARCH}" in
    linux-x86_64)   TARGET="x86_64-unknown-linux-musl" ;;
    linux-aarch64)  TARGET="aarch64-unknown-linux-musl" ;;
    darwin-x86_64)  TARGET="x86_64-apple-darwin" ;;
    darwin-aarch64) TARGET="aarch64-apple-darwin" ;;
    *)
      TARGET=""
      warn "No standard prebuilt target for ${OS}/${ARCH}"
      ;;
  esac
}

# Beads-style os/arch (darwin/linux + amd64/arm64)
detect_platform_go() {
  detect_platform
  case "$OS" in
    darwin) GOOS="darwin" ;;
    linux)  GOOS="linux" ;;
    *)      GOOS="" ;;
  esac
  case "$ARCH" in
    x86_64)  GOARCH="amd64" ;;
    aarch64) GOARCH="arm64" ;;
    *)       GOARCH="" ;;
  esac
}

# ── Temp + cleanup ────────────────────────────────────────────────────────
installer_register_tmpdir() {
  local d="$1"
  INSTALLER_TMPDIRS="${INSTALLER_TMPDIRS} ${d}"
}

installer_cleanup() {
  local d
  for d in $INSTALLER_TMPDIRS; do
    [[ -n "$d" ]] && rm -rf "$d"
  done
  if [[ -n "$LOCK_DIR" && -d "$LOCK_DIR" ]]; then
    # Only remove if we own the lock (our PID)
    if [[ -f "$LOCK_DIR/pid" ]] && [[ "$(cat "$LOCK_DIR/pid" 2>/dev/null || true)" == "$$" ]]; then
      rm -rf "$LOCK_DIR"
    fi
  fi
}

installer_mktemp_dir() {
  local label="${1:-agentops-install}"
  local d
  d="$(mktemp -d "${TMPDIR:-/tmp}/${label}.XXXXXX")"
  installer_register_tmpdir "$d"
  # Refresh EXIT trap so cleanup always runs (callers may add their own work
  # by registering more tmpdirs rather than replacing the trap).
  trap installer_cleanup EXIT
  printf '%s\n' "$d"
}

# ── Atomic lock (mkdir-based; works on macOS) ─────────────────────────────
acquire_install_lock() {
  local name="${1:-$INSTALLER_NAME}"
  LOCK_DIR="${TMPDIR:-/tmp}/${name}.install.lock"
  local waited=0
  while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    local stale_pid=""
    if [[ -f "$LOCK_DIR/pid" ]]; then
      stale_pid="$(cat "$LOCK_DIR/pid" 2>/dev/null || true)"
    fi
    if [[ -n "$stale_pid" ]] && ! kill -0 "$stale_pid" 2>/dev/null; then
      warn "Removing stale install lock (pid $stale_pid)"
      rm -rf "$LOCK_DIR"
      continue
    fi
    if [[ "$waited" -ge 30 ]]; then
      fail "Another install appears to be running (lock: $LOCK_DIR). Retry later or remove the lock."
    fi
    info "Waiting for install lock..."
    sleep 1
    waited=$((waited + 1))
  done
  printf '%s\n' "$$" >"$LOCK_DIR/pid"
  trap installer_cleanup EXIT
}

# ── Checksums / Sigstore ──────────────────────────────────────────────────
sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    return 1
  fi
}

verify_checksum() {
  local file="$1"
  local expected="$2"
  local actual

  if [[ "$NO_VERIFY" -eq 1 ]]; then
    warn "Skipping checksum verification (--no-verify)"
    return 0
  fi

  if ! actual="$(sha256_file "$file")"; then
    warn "No SHA256 tool found (sha256sum/shasum); skipping checksum verification"
    return 0
  fi

  # Allow "hash" or "hash  filename" forms
  expected="$(printf '%s' "$expected" | awk '{print $1}')"
  if [[ "$actual" != "$expected" ]]; then
    err "Checksum mismatch!"
    err "  Expected: $expected"
    err "  Got:      $actual"
    return 1
  fi
  ok "SHA256 checksum verified"
}

# Soft-skip when cosign missing or bundle missing; hard-fail on bad sig.
verify_sigstore() {
  local file="$1"
  local bundle_url="${2:-}"

  if [[ "$NO_VERIFY" -eq 1 ]]; then
    warn "Skipping Sigstore verification (--no-verify)"
    return 0
  fi
  if ! command -v cosign >/dev/null 2>&1; then
    warn "cosign not found; skipping Sigstore verification"
    return 0
  fi
  if [[ -z "$bundle_url" ]]; then
    warn "No Sigstore bundle URL; skipping signature verification"
    return 0
  fi

  local bundle_file
  bundle_file="$(installer_mktemp_dir "sigstore")/bundle.json"
  if ! curl -fsSL --connect-timeout 10 "${PROXY_ARGS[@]}" "$bundle_url" -o "$bundle_file" 2>/dev/null; then
    warn "Could not download Sigstore bundle; skipping verification"
    return 0
  fi

  local identity_re="${COSIGN_IDENTITY_RE:-^https://github.com/${OWNER}/${REPO}/.github/workflows/.*$}"
  local oidc_issuer="${COSIGN_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"

  if cosign verify-blob --bundle "$bundle_file" \
    --certificate-identity-regexp "$identity_re" \
    --certificate-oidc-issuer "$oidc_issuer" \
    "$file" >/dev/null 2>&1; then
    ok "Sigstore signature verified"
    return 0
  fi
  err "Sigstore verification FAILED"
  return 1
}

# ── Preflight ─────────────────────────────────────────────────────────────
check_disk_space() {
  local dest="${1:-${HOME}}"
  local avail
  avail="$(df -Pk "$dest" 2>/dev/null | awk 'NR==2 {print $4}')"
  if [[ -z "$avail" ]]; then
    warn "Could not determine free disk space under $dest"
    return 0
  fi
  if [[ "$avail" -lt "$MIN_DISK_KB" ]]; then
    fail "Insufficient disk space under $dest (need ${MIN_DISK_KB}KB, have ${avail}KB)"
  fi
  info "Disk space OK (${avail}KB free)"
}

check_write_permissions() {
  local dest="$1"
  mkdir -p "$dest" 2>/dev/null || fail "Cannot create install directory: $dest"
  local probe="$dest/.agentops-write-probe.$$"
  if ! touch "$probe" 2>/dev/null; then
    fail "No write permission in $dest"
  fi
  rm -f "$probe"
}

check_network() {
  local url="${1:-https://github.com}"
  if [[ "$OFFLINE" -eq 1 ]]; then
    info "Offline mode — skipping network preflight"
    return 0
  fi
  if ! curl -fsSL --connect-timeout 3 -o /dev/null "${PROXY_ARGS[@]}" "$url" 2>/dev/null; then
    fail "Network preflight failed (cannot reach $url). Use --offline with a local tarball/bundle if air-gapped."
  fi
  info "Network OK"
}

preflight_checks() {
  local dest="${1:-${HOME}/.local/bin}"
  local probe_url="${2:-https://github.com}"
  info "Running preflight checks"
  check_disk_space "$dest"
  check_write_permissions "$dest"
  check_network "$probe_url"
}

# ── PATH helper ───────────────────────────────────────────────────────────
maybe_add_path() {
  local dest="$1"
  case ":$PATH:" in
    *":$dest:"*) return 0 ;;
  esac
  if [[ "$EASY_MODE" -eq 1 ]]; then
    local rc
    for rc in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile"; do
      if [[ -e "$rc" && -w "$rc" ]]; then
        if ! grep -qF "$dest" "$rc" 2>/dev/null; then
          printf '\n# Added by %s installer\nexport PATH="%s:$PATH"\n' "$INSTALLER_NAME" "$dest" >>"$rc"
          ok "Added $dest to PATH in $rc"
        fi
      fi
    done
  else
    warn "Add $dest to PATH to use the installed binary:"
    warn "  export PATH=\"$dest:\$PATH\""
  fi
}

# ── Summary / uninstall blurb ─────────────────────────────────────────────
summary_add() {
  SUMMARY_LINES+=("$*")
}

print_summary() {
  local title="${1:-Install complete}"
  shift || true
  local lines=("$title" "")
  local l
  for l in "${SUMMARY_LINES[@]}"; do
    lines+=("$l")
  done
  for l in "$@"; do
    lines+=("$l")
  done
  draw_box 42 "${lines[@]}"
}

print_uninstall_hint() {
  local hint="$1"
  [[ "$QUIET" -eq 1 ]] && return 0
  info "Uninstall / revert:"
  info "  $hint"
}

# ── Common flag parse helper ──────────────────────────────────────────────
# Callers handle domain flags; use this for shared ones inside their case.
# Returns 0 if consumed (and sets CONSUMED=1|2), 1 if not a shared flag.
parse_common_installer_flag() {
  CONSUMED=0
  case "$1" in
    --quiet)
      QUIET=1
      CONSUMED=1
      return 0
      ;;
    --no-gum)
      NO_GUM=1
      installer_detect_gum
      CONSUMED=1
      return 0
      ;;
    --force)
      FORCE_INSTALL=1
      CONSUMED=1
      return 0
      ;;
    --no-verify)
      NO_VERIFY=1
      CONSUMED=1
      return 0
      ;;
    --easy-mode)
      EASY_MODE=1
      CONSUMED=1
      return 0
      ;;
    --offline)
      OFFLINE=1
      if [[ -n "${2:-}" && "${2:0:1}" != "-" ]]; then
        OFFLINE_TARBALL="$2"
        CONSUMED=2
      else
        CONSUMED=1
      fi
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

common_installer_flags_help() {
  cat <<'EOF'
  --quiet       Suppress non-error progress output
  --no-gum      Disable gum formatting even if available
  --force       Reinstall even if the same version is present
  --no-verify   Skip checksum / signature verification
  --easy-mode   Auto-append install dir to shell rc PATH
  --offline [TARBALL]
                Skip network preflight; optional local artifact path
EOF
}

# ── Curl one-liner docs helper ────────────────────────────────────────────
curl_one_liner() {
  local script_path="$1"
  printf 'curl -fsSL "https://raw.githubusercontent.com/%s/%s/main/%s?$(date +%%s)" | bash\n' \
    "$OWNER" "$REPO" "$script_path"
}

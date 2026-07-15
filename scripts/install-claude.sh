#!/usr/bin/env bash
# install-claude.sh - Legacy Claude marketplace compatibility installer.
# New installs should clone AgentOps and run `ao skills link`.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash -s -- --update
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash -s -- --ref v4.0.0

set -euo pipefail
shopt -s lastpipe 2>/dev/null || true
umask 022

MARKETPLACE="${AGENTOPS_CLAUDE_MARKETPLACE:-boshu2/agentops}"
MARKETPLACE_NAME="${AGENTOPS_CLAUDE_MARKETPLACE_NAME:-agentops-marketplace}"
PLUGIN_KEY="${AGENTOPS_CLAUDE_PLUGIN_KEY:-agentops@agentops-marketplace}"
# --ref pins the marketplace source to a tagged release (e.g. v4.0.0) for parity
# with install-agy.sh. Empty = track the marketplace's default branch.
INSTALL_REF="${AGENTOPS_INSTALL_REF:-}"
DRY_RUN=0
UPDATE=0
QUIET=0

usage() {
  cat <<'EOF'
install-claude.sh

Install AgentOps for Claude Code through the Claude plugin marketplace.

DEPRECATED: retained for 3.x migration compatibility. New installs should use
one canonical checkout plus `ao skills link`.

Usage:
  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash -s -- --update

Options:
  --update      Update marketplace metadata and the installed plugin.
  --ref <ref>   Pin the marketplace source to a git ref (e.g. v4.0.0).
                Defaults to AGENTOPS_INSTALL_REF or the marketplace default branch.
  --dry-run     Print the commands that would run without changing anything.
  --quiet       Reduce progress output.
  --help        Show this help.
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

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    printf 'DRY-RUN:'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --update)
      UPDATE=1
      shift
      ;;
    --ref)
      INSTALL_REF="${2:-}"
      [ -n "$INSTALL_REF" ] || fail "--ref requires a value"
      shift 2
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

info "Installing AgentOps for Claude Code"

# Pin the marketplace source to a tag when --ref is given (parity with
# install-agy.sh). The Claude plugin marketplace accepts an owner/repo@ref spec.
MARKETPLACE_SPEC="$MARKETPLACE"
if [ -n "$INSTALL_REF" ]; then
  MARKETPLACE_SPEC="${MARKETPLACE}@${INSTALL_REF}"
  info "Pinning marketplace source to ref: $INSTALL_REF"
fi

if [ "$DRY_RUN" -eq 0 ] && ! command -v claude >/dev/null 2>&1; then
  fail "Claude Code CLI not found in PATH. Install Claude Code first, then rerun this installer."
fi

if [ "$UPDATE" -eq 1 ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    run claude plugin marketplace update "$MARKETPLACE_NAME"
    run claude plugin update agentops
  else
    claude plugin marketplace update "$MARKETPLACE_NAME" >/dev/null 2>&1 || \
      claude plugin marketplace add "$MARKETPLACE_SPEC"
    claude plugin update agentops >/dev/null 2>&1 || \
      claude plugin install "$PLUGIN_KEY"
  fi
else
  if [ "$DRY_RUN" -eq 1 ]; then
    run claude plugin marketplace add "$MARKETPLACE_SPEC"
  else
    claude plugin marketplace add "$MARKETPLACE_SPEC" >/dev/null 2>&1 || \
      claude plugin marketplace update "$MARKETPLACE_NAME"
  fi
  run claude plugin install "$PLUGIN_KEY"
fi

ok "AgentOps Claude plugin is installed"
printf 'Next: restart Claude Code, then run /quickstart.\n'

#!/usr/bin/env bash
set -euo pipefail

# AgentOps Installer
# Usage: bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
#        bash scripts/install.sh --dev

usage() {
    cat <<'EOF'
Usage:
  bash scripts/install.sh
  bash scripts/install.sh --dev
  bash scripts/install.sh --with-hooks

Options:
  --dev       Configure this checkout for AgentOps development: install repo
              hooks, build cli/bin/ao, and smoke-test pre-push wiring.
  --with-hooks
              Also install runtime hooks. Default install is hookless.
  -h, --help  Show this help.
EOF
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WITH_HOOKS="${AGENTOPS_INSTALL_HOOKS:-0}"

install_dev() {
    local repo_root
    repo_root="$(cd "$SCRIPT_DIR/.." && pwd)"

    if [[ ! -f "$repo_root/scripts/install-dev-hooks.sh" || ! -d "$repo_root/cli" ]]; then
        echo "Error: --dev must be run from an AgentOps source checkout." >&2
        exit 1
    fi

    echo "Installing AgentOps development wiring..."
    echo "Step 1/2: Configuring repo-managed git hooks..."
    bash "$repo_root/scripts/install-dev-hooks.sh"

    echo "Step 2/2: Building cli/bin/ao..."
    make -C "$repo_root/cli" build

    # Pre-push gate-wiring verification retired (soc-bbvw / soc-g2r9):
    # local pre-push gate retired; CI is sole authoritative push gate.
    # See docs/contracts/local-pre-push-gate-retirement.md.

    echo ""
    echo "Done! Development checkout ready."
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dev)
            shift
            if [[ $# -gt 0 ]]; then
                echo "Unknown option for --dev: $1" >&2
                usage >&2
                exit 2
            fi
            install_dev
            exit 0
            ;;
        --with-hooks)
            WITH_HOOKS=1
            shift
            ;;
        --no-hooks)
            WITH_HOOKS=0
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
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
        echo "Invalid AGENTOPS_INSTALL_HOOKS/--with-hooks value: $WITH_HOOKS" >&2
        exit 2
        ;;
esac

echo "Installing AgentOps..."

# Check prerequisites
command -v curl >/dev/null 2>&1 || { echo "Error: curl required."; exit 1; }
command -v claude >/dev/null 2>&1 || command -v codex >/dev/null 2>&1 || {
    echo "Warning: No supported coding agent found (claude, codex)."
    echo "AgentOps requires Claude Code or Codex CLI. Install one first:"
    echo "  Claude Code: https://docs.anthropic.com/en/docs/claude-code"
    echo "  Codex CLI:   https://github.com/openai/codex"
    echo "Continuing anyway — you can install an agent later."
}

# Step 1: Install Codex plugin
echo "Step 1/3: Installing Codex plugin..."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
curl -fsSL https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/main \
    | tar xz -C "$TMP" --strip-components=1

if command -v codex >/dev/null 2>&1; then
    codex_args=()
    if [[ "$WITH_HOOKS" == "1" ]]; then
        codex_args+=(--with-hooks)
    fi
    AGENTOPS_BUNDLE_ROOT="$TMP" bash "$TMP/scripts/install-codex.sh" "${codex_args[@]}"
else
    echo "Codex CLI not found. Skipping Codex plugin install."
    echo "For Claude Code, install skills via the plugin system:"
    echo "  npx skills@latest add boshu2/agentops --all -g"
fi

# Step 2: Install CLI (optional — enhances with knowledge flywheel)
if command -v brew >/dev/null 2>&1; then
    echo "Step 2/3: Installing CLI via Homebrew..."
    if ! brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops; then
        echo "Error: failed to add Homebrew tap boshu2/agentops." >&2
        exit 1
    fi

    if ! brew install agentops; then
        echo "brew install agentops failed; trying brew upgrade agentops..."
        if ! brew upgrade agentops; then
            echo "Error: Homebrew could not install or upgrade agentops." >&2
            echo "Try manually:" >&2
            echo "  brew update && brew upgrade agentops" >&2
            exit 1
        fi
    fi

    # Step 3: Optional hooks
    if command -v ao >/dev/null 2>&1; then
        echo "Note: To create repo-local .agents/ scaffolding, run 'ao init' from your repo root."
        if [[ "$WITH_HOOKS" == "1" ]]; then
            echo "Step 3/3: Registering hooks..."
            ao hooks install --force
        else
            echo "Step 3/3: Hooks skipped (hookless default)."
            echo "Optional: rerun with --with-hooks, or run 'ao hooks install --force' later."
        fi

        # Optional health check
        ao doctor 2>/dev/null && echo "Health check: PASS" || echo "Health check: run 'ao doctor' after setup"
    fi
else
    echo "Step 2/3: Skipping CLI (Homebrew not found). Install manually:"
    echo "  brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops"
    echo "  brew install agentops"
    echo "Step 3/3: Skipped (CLI needed for optional hooks)"
fi

echo ""
echo "Done! Start with: /quickstart"

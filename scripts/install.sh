#!/usr/bin/env bash
set -euo pipefail

# AgentOps Installer
# Usage: bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)
# Or:    npx skills@latest add boshu2/agentops --all -g

echo "Installing AgentOps..."

# Check prerequisites
command -v npm >/dev/null 2>&1 || { echo "Error: npm required. Install Node.js first."; exit 1; }
command -v claude >/dev/null 2>&1 || command -v codex >/dev/null 2>&1 || command -v cursor >/dev/null 2>&1 || {
    echo "Warning: No supported coding agent found (claude, codex, cursor)."
    echo "AgentOps requires a Skills-compatible agent. Install one first:"
    echo "  Claude Code: https://docs.anthropic.com/en/docs/claude-code"
    echo "  Codex CLI:   https://github.com/openai/codex"
    echo "Continuing anyway — you can install an agent later."
}

# Step 1: Install plugin (skills + hooks + agents)
echo "Step 1/3: Installing plugin..."
npx skills@latest add boshu2/agentops --all -g

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

    # Step 3: Install hooks
    if command -v ao >/dev/null 2>&1; then
        echo "Step 3/3: Registering hooks..."
        echo "Note: To create repo-local .agents/ scaffolding, run 'ao init' from your repo root."
        ao hooks install --force

        # Optional health check
        ao doctor 2>/dev/null && echo "Health check: PASS" || echo "Health check: run 'ao doctor' after setup"
    fi
else
    echo "Step 2/3: Skipping CLI (Homebrew not found). Install manually:"
    echo "  brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops"
    echo "  brew install agentops"
    echo "Step 3/3: Skipped (CLI needed for hooks)"
fi

echo ""
echo "Done! Start with: /quickstart"

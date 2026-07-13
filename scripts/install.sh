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
  bash scripts/install.sh --tier spine

Options:
  --dev       Configure this checkout for AgentOps development: install repo
              hooks, build cli/bin/ao, and smoke-test pre-push wiring.
  --with-hooks
              Also install runtime hooks. Default install is hookless.
  --tier <spine|all>
              Which skill tier to install. "spine" installs only the proven
              spine skills (spine: true frontmatter — see skills/SKILL-TIERS.md),
              skipping the experimental corpus/flywheel tier; "all" (default)
              installs the whole bundle. Filters the Codex/AGY bundle installs;
              the Claude plugin path is whole-bundle (manifest split is future work).
  -h, --help  Show this help.
EOF
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WITH_HOOKS="${AGENTOPS_INSTALL_HOOKS:-0}"
TIER="${AGENTOPS_INSTALL_TIER:-all}"
# Offline/local-source mode: install from an already-extracted checkout instead
# of downloading from GitHub. Shared with install-codex.sh / install-agy.sh /
# install-opencode.sh so tests (and air-gapped installs) run against the
# worktree. Local-source mode installs the full bundle (no spine pruning, which
# would mutate the source checkout).
SOURCE_ROOT_OVERRIDE="${AGENTOPS_BUNDLE_ROOT:-}"

# prune_bundle_to_spine removes every non-spine skill dir from an extracted
# AgentOps bundle so a --tier spine install ships only the proven spine skills
# (age-h4y3). It edits the bundle in place before the per-runtime installers copy
# from it; the selection lever is scripts/select-spine-skills.sh.
prune_bundle_to_spine() {
    local bundle="$1"
    local selector="$bundle/scripts/select-spine-skills.sh"
    if [[ ! -f "$selector" ]]; then
        echo "Warning: spine selector missing ($selector); installing all skills." >&2
        return 0
    fi
    local keep
    keep="$(bash "$selector" "$bundle/skills" 2>/dev/null || true)"
    if [[ -z "$keep" ]]; then
        echo "Warning: no spine skills resolved; installing all skills." >&2
        return 0
    fi
    local skills_root dir name
    for skills_root in "$bundle/skills" "$bundle/skills-codex"; do
        [[ -d "$skills_root" ]] || continue
        for dir in "$skills_root"/*/; do
            [[ -d "$dir" ]] || continue
            name="$(basename "$dir")"
            if ! grep -qxF "$name" <<<"$keep"; then
                rm -rf "$dir"
            fi
        done
    done
    echo "Tier spine: kept $(printf '%s\n' "$keep" | grep -c .) spine skills; skipped the experimental corpus/flywheel tier."
}

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
        --tier)
            shift
            TIER="${1:-}"
            shift || true
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

case "$TIER" in
    spine|all) ;;
    *)
        echo "Invalid --tier/AGENTOPS_INSTALL_TIER value: $TIER (want: spine|all)" >&2
        exit 2
        ;;
esac

echo "Installing AgentOps..."

# Check prerequisites
command -v curl >/dev/null 2>&1 || { echo "Error: curl required."; exit 1; }

# Per-runtime detection (soc-vuu6.31). The old `claude || codex` test bounced
# only on zero runtimes — single-runtime users got the README mixed-model
# example silently. Detect each runtime explicitly so we can: (a) report
# yes/no per runtime, (b) name the mode AgentOps will operate in, and
# (c) point single-runtime users at the install link for the other.
HAS_CLAUDE=no
HAS_CODEX=no
HAS_AGY=no
command -v claude >/dev/null 2>&1 && HAS_CLAUDE=yes
command -v codex >/dev/null 2>&1 && HAS_CODEX=yes
command -v agy >/dev/null 2>&1 && HAS_AGY=yes
echo "Detected Claude Code: $HAS_CLAUDE. Detected Codex CLI: $HAS_CODEX. Detected Gemini/AGY: $HAS_AGY."

# Count distinct runtimes so we can name single- vs mixed-model mode across all
# three supported vendors (Claude, Codex, Gemini/AGY) — not just claude/codex.
RUNTIME_COUNT=0
[ "$HAS_CLAUDE" = "yes" ] && RUNTIME_COUNT=$((RUNTIME_COUNT + 1))
[ "$HAS_CODEX" = "yes" ] && RUNTIME_COUNT=$((RUNTIME_COUNT + 1))
[ "$HAS_AGY" = "yes" ] && RUNTIME_COUNT=$((RUNTIME_COUNT + 1))

if [ "$RUNTIME_COUNT" -ge 2 ]; then
    echo "AgentOps will use mixed-model mode (cross-vendor council, parallel /rpi)."
elif [ "$RUNTIME_COUNT" -eq 1 ]; then
    if [ "$HAS_CLAUDE" = "yes" ]; then
        echo "AgentOps will use single-runtime mode (Claude Code only)."
    elif [ "$HAS_CODEX" = "yes" ]; then
        echo "AgentOps will use single-runtime mode (Codex CLI only)."
    else
        echo "AgentOps will use single-runtime mode (Gemini/AGY only)."
    fi
    echo "To unlock mixed-model judging, install another runtime:"
    [ "$HAS_CLAUDE" = "no" ] && echo "  Claude Code: https://docs.anthropic.com/en/docs/claude-code"
    [ "$HAS_CODEX" = "no" ]  && echo "  Codex CLI:   https://github.com/openai/codex"
    [ "$HAS_AGY" = "no" ]    && echo "  Gemini/AGY:  https://antigravity.google"
else
    echo "Warning: No supported coding agent found (claude, codex, agy)."
    echo "AgentOps requires Claude Code, Codex CLI, or Gemini/AGY. Install one first:"
    echo "  Claude Code: https://docs.anthropic.com/en/docs/claude-code"
    echo "  Codex CLI:   https://github.com/openai/codex"
    echo "  Gemini/AGY:  https://antigravity.google"
    echo "Continuing anyway — you can install an agent later."
fi

# Step 1: Install Codex plugin
echo "Step 1/3: Installing Codex plugin..."
if [ -n "$SOURCE_ROOT_OVERRIDE" ]; then
    [ -f "$SOURCE_ROOT_OVERRIDE/scripts/install-codex.sh" ] \
        || { echo "Error: AGENTOPS_BUNDLE_ROOT missing scripts/install-codex.sh: $SOURCE_ROOT_OVERRIDE" >&2; exit 1; }
    BUNDLE="$SOURCE_ROOT_OVERRIDE"
    echo "Local-source mode: installing from $BUNDLE (full bundle; spine pruning skipped)."
else
    TMP=$(mktemp -d)
    trap 'rm -rf "$TMP"' EXIT
    curl -fsSL https://codeload.github.com/boshu2/agentops/tar.gz/refs/heads/main \
        | tar xz -C "$TMP" --strip-components=1
    BUNDLE="$TMP"
    if [[ "$TIER" == "spine" ]]; then
        echo "Tier: spine — pruning bundle to spine skills before install..."
        prune_bundle_to_spine "$BUNDLE"
    fi
fi

if command -v codex >/dev/null 2>&1; then
    codex_args=()
    if [[ "$WITH_HOOKS" == "1" ]]; then
        codex_args+=(--with-hooks)
    fi
    # Expand-only-if-set: an empty array under `set -u` is an unbound-variable
    # error on macOS bash 3.2, which is reachable now that local-source mode
    # exercises this path offline.
    AGENTOPS_BUNDLE_ROOT="$BUNDLE" bash "$BUNDLE/scripts/install-codex.sh" ${codex_args[@]+"${codex_args[@]}"}
else
    echo "Codex CLI not found. Skipping Codex plugin install."
    echo "For Claude Code, install skills via the plugin system:"
    echo "  npx skills@latest add boshu2/agentops --all -g"
fi

# Gemini/AGY plugin (reuses the already-extracted bundle in $TMP). jq is required
# by install-agy.sh; skip with a pointer if it is missing rather than aborting the
# whole install for Claude/Codex users.
if command -v agy >/dev/null 2>&1; then
    if command -v jq >/dev/null 2>&1; then
        echo "Installing Gemini/AGY plugin..."
        AGENTOPS_BUNDLE_ROOT="$BUNDLE" bash "$BUNDLE/scripts/install-agy.sh"
    else
        echo "Gemini/AGY detected but 'jq' is missing. Install jq, then run:"
        echo "  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash"
    fi
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

    # Step 3: Runtime-specific setup
    if command -v ao >/dev/null 2>&1; then
        echo "Note: To create repo-local .agents/ scaffolding, run 'ao init' from your repo root."
        if [[ "$WITH_HOOKS" == "1" ]]; then
            echo "Step 3/3: Runtime-specific hook setup is delegated to each runtime installer."
        else
            echo "Step 3/3: Hooks skipped (hookless default)."
            echo "Optional: rerun with --with-hooks for runtimes that expose native hooks."
        fi

        # Optional health check
        ao doctor 2>/dev/null && echo "Health check: PASS" || echo "Health check: run 'ao doctor' after setup"
    fi
else
    echo "Step 2/3: Skipping CLI (Homebrew not found). Install manually:"
    echo "  brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops"
    echo "  brew install agentops"
    echo "Step 3/3: Runtime-specific setup complete."
fi

# Self-test: verify install.sh's aggregate claim before printing "Done"
# (age-txfnl). The per-runtime installers self-test and abort on failure, so
# reaching here already implies each ran clean; this tail is the orchestrator's
# own belt-and-suspenders check that the runtime we detected actually landed a
# config-enable entry + install metadata, so we never print "Done" over a
# half-installed state.
selftest_problems=0
if [ "$HAS_CODEX" = "yes" ]; then
    codex_meta="${HOME}/.codex/.agentops-codex-install.json"
    codex_cfg="${HOME}/.codex/config.toml"
    if [ ! -f "$codex_meta" ]; then
        echo "self-test: Codex install metadata missing ($codex_meta)" >&2
        selftest_problems=$((selftest_problems + 1))
    fi
    if ! grep -qF 'agentops@agentops-marketplace' "$codex_cfg" 2>/dev/null; then
        echo "self-test: Codex config missing plugin enable entry ($codex_cfg)" >&2
        selftest_problems=$((selftest_problems + 1))
    fi
fi
if [ "$selftest_problems" -gt 0 ]; then
    echo "Error: install self-test failed with $selftest_problems problem(s); not all runtimes installed cleanly. Re-run this installer or run 'ao doctor' for details." >&2
    exit 1
fi

echo ""
echo "Done! Verify it worked: start your coding agent and type /plan (or /quickstart)."

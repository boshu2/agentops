#!/usr/bin/env bash
# install-opencode.sh — Install AgentOps plugin + skills for OpenCode
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash
#   # or
#   ./scripts/install-opencode.sh
#
# What it does:
#   1. Clones agentops repo (or pulls if exists)
#   2. Installs plugin dependency (@opencode-ai/plugin)
#   3. Symlinks plugin to ~/.config/opencode/plugins/
#   4. Symlinks skills to ~/.config/opencode/skills/
#   5. Verifies installation

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }
fail()  { echo -e "${RED}✗${NC} $*"; exit 1; }

# Detect OpenCode config dir
OPENCODE_CONFIG="${OPENCODE_CONFIG_DIR:-${HOME}/.config/opencode}"
AGENTOPS_DIR="${OPENCODE_CONFIG}/agentops"
PLUGIN_DIR="${OPENCODE_CONFIG}/plugins"
SKILLS_DIR="${OPENCODE_CONFIG}/skills"
REPO_URL="https://github.com/boshu2/agentops.git"
# Offline/local-source mode: point at an already-extracted checkout instead of
# cloning from GitHub. Shared with install-codex.sh / install-agy.sh so tests
# (and air-gapped installs) can run against the worktree. When set, the skills
# and plugin are linked straight from this checkout.
SOURCE_ROOT_OVERRIDE="${AGENTOPS_BUNDLE_ROOT:-}"

get_latest_tag() {
    local tag
    tag=$(curl -fsSL "https://api.github.com/repos/boshu2/agentops/releases/latest" 2>/dev/null \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)
    if [ -z "$tag" ]; then
        echo "main"  # fallback to main if API fails
    else
        echo "$tag"
    fi
}

echo "Installing AgentOps for OpenCode..."
echo ""

# Step 1: Check OpenCode is installed
if ! command -v opencode &>/dev/null; then
  warn "OpenCode not found in PATH. Install from https://opencode.ai"
  warn "Continuing anyway — plugin will be ready when OpenCode is installed."
fi

# Step 2: Clone or update repo at latest release tag (skipped in local-source mode)
if [ -n "$SOURCE_ROOT_OVERRIDE" ]; then
  [ -d "$SOURCE_ROOT_OVERRIDE/skills" ] || fail "AGENTOPS_BUNDLE_ROOT does not contain skills/: $SOURCE_ROOT_OVERRIDE"
  AGENTOPS_DIR="$SOURCE_ROOT_OVERRIDE"
  info "Using provided AgentOps bundle: $AGENTOPS_DIR"
fi
RELEASE_TAG=$(get_latest_tag)
if [ -n "$SOURCE_ROOT_OVERRIDE" ]; then
  info "Local-source mode — skipping clone and plugin dependency install."
elif [ -d "$AGENTOPS_DIR/.git" ]; then
  info "AgentOps repo exists, fetching latest release ($RELEASE_TAG)..."
  git -C "$AGENTOPS_DIR" fetch --tags 2>/dev/null || warn "git fetch failed — using existing version"
  if [ "$RELEASE_TAG" != "main" ]; then
    git -C "$AGENTOPS_DIR" checkout "$RELEASE_TAG" 2>/dev/null || warn "checkout $RELEASE_TAG failed — using existing version"
  else
    git -C "$AGENTOPS_DIR" pull --ff-only 2>/dev/null || warn "git pull failed — using existing version"
  fi
else
  info "Cloning AgentOps ($RELEASE_TAG)..."
  mkdir -p "$(dirname "$AGENTOPS_DIR")"
  if [ "$RELEASE_TAG" != "main" ]; then
    git clone --branch "$RELEASE_TAG" --depth 1 "$REPO_URL" "$AGENTOPS_DIR"
  else
    git clone --depth 1 "$REPO_URL" "$AGENTOPS_DIR"
  fi
fi

# Step 3: Install plugin dependency (skipped in local-source mode to avoid
# writing node_modules into the source checkout).
if [ -n "$SOURCE_ROOT_OVERRIDE" ]; then
  :
elif [ -f "$AGENTOPS_DIR/.opencode/package.json" ]; then
  if command -v bun &>/dev/null; then
    info "Installing plugin dependencies (bun)..."
    cd "$AGENTOPS_DIR/.opencode" && bun install --silent 2>/dev/null && cd - >/dev/null
  elif command -v npm &>/dev/null; then
    info "Installing plugin dependencies (npm)..."
    cd "$AGENTOPS_DIR/.opencode" && npm install --silent 2>/dev/null && cd - >/dev/null
  else
    warn "Neither bun nor npm found — plugin dependency may be missing"
  fi
fi

# Step 4: Symlink plugin
mkdir -p "$PLUGIN_DIR"
PLUGIN_SRC="$AGENTOPS_DIR/.opencode/plugins/agentops.js"
PLUGIN_DST="$PLUGIN_DIR/agentops.js"

if [ -f "$PLUGIN_SRC" ]; then
  rm -f "$PLUGIN_DST"
  ln -s "$PLUGIN_SRC" "$PLUGIN_DST"
  info "Plugin linked: $PLUGIN_DST → $PLUGIN_SRC"
else
  fail "Plugin not found at $PLUGIN_SRC"
fi

# Step 5: Symlink skills
mkdir -p "$SKILLS_DIR"
SKILLS_SRC="$AGENTOPS_DIR/skills"
SKILLS_DST="$SKILLS_DIR/agentops"

if [ -d "$SKILLS_SRC" ]; then
  rm -rf "$SKILLS_DST"
  ln -s "$SKILLS_SRC" "$SKILLS_DST"
  info "Skills linked: $SKILLS_DST → $SKILLS_SRC"
else
  fail "Skills directory not found at $SKILLS_SRC"
fi

# Step 6: Self-test — verify the install's own claims before declaring success
# (age-txfnl). The skill count we report must match the source of truth (the
# skills we linked from); the plugin link must resolve; a sentinel skill must be
# readable through the link. On any mismatch we exit nonzero naming the delta
# rather than printing a count that reality does not back.
echo ""
SKILL_COUNT=$(find -L "$SKILLS_DST" -name "SKILL.md" -maxdepth 2 2>/dev/null | wc -l | tr -d ' ')
SOURCE_COUNT=$(find "$SKILLS_SRC" -name "SKILL.md" -maxdepth 2 2>/dev/null | wc -l | tr -d ' ')

selftest_problems=0
report_problem() { warn "self-test: $*"; selftest_problems=$((selftest_problems + 1)); }

# (a) linked count matches the source-of-truth count and is non-empty.
if [ "$SKILL_COUNT" -eq 0 ]; then
  report_problem "no skills visible through $SKILLS_DST"
fi
if [ "$SKILL_COUNT" != "$SOURCE_COUNT" ]; then
  report_problem "linked skill count ($SKILL_COUNT via $SKILLS_DST) != source skill count ($SOURCE_COUNT in $SKILLS_SRC) — the symlink is stale or partial"
fi
# (b) plugin entry present and resolves.
if [ ! -e "$PLUGIN_DST" ]; then
  report_problem "plugin entry $PLUGIN_DST is missing or dangling"
fi
# (c) one sentinel skill is readable through the link.
SENTINEL=$(find -L "$SKILLS_DST" -name "SKILL.md" -maxdepth 2 2>/dev/null | head -1)
if [ -z "$SENTINEL" ] || [ ! -r "$SENTINEL" ]; then
  report_problem "no readable sentinel SKILL.md under $SKILLS_DST"
fi

if [ "$selftest_problems" -gt 0 ]; then
  fail "OpenCode install self-test failed with $selftest_problems problem(s); refusing to report success."
fi

info "Installation complete! Self-test passed: $SKILL_COUNT skills, plugin linked."
echo "  Plugin: $PLUGIN_DST"
echo "  Skills: $SKILLS_DST ($SKILL_COUNT skills)"
echo ""
echo "Verify it worked: restart OpenCode, then ask \"do you have agentops?\" (or type /plan)."
echo ""
echo "To update later:"
echo "  curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-opencode.sh | bash"

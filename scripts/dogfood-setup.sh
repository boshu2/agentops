#!/bin/bash
# Set up dogfooding - use your own plugins instead of marketplace
#
# This:
# 1. Backs up ~/.claude/skills/
# 2. Installs your plugins to ~/.claude/plugins/
# 3. Updates settings.json to use local plugins
#
# Usage: ./scripts/dogfood-setup.sh [--revert]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLAUDE_DIR="$HOME/.claude"
BACKUP_DIR="$CLAUDE_DIR/skills.backup.$(date +%Y%m%d)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Revert mode
if [[ "${1:-}" == "--revert" ]]; then
    echo -e "${BLUE}Reverting to original setup...${NC}"

    # Find most recent backup
    latest_backup=$(ls -d "$CLAUDE_DIR"/skills.backup.* 2>/dev/null | tail -1)

    if [[ -n "$latest_backup" ]] && [[ -d "$latest_backup" ]]; then
        # Guard: refuse to restore a backup much smaller than the live dir.
        # A stale backup (captured when few skills existed) would silently wipe
        # a healthy skills set — the best-fit cause of the 26-vs-125 drift
        # incident. Override the guard with: --revert --force-revert
        force_revert=0
        [[ "${2:-}" == "--force-revert" ]] && force_revert=1
        live_count=$(find "$CLAUDE_DIR/skills" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ')
        backup_count=$(find "$latest_backup" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ')
        # Multiply, don't divide: `backup < live/2` floors odd live counts and
        # fails open at the boundary (live=125 -> 125/2=62, a 62-entry backup
        # would pass). `2*backup < live` is the exact <50% test with no floor.
        if [[ "$force_revert" -eq 0 ]] && [[ "$live_count" -gt 0 ]] && [[ $((2 * backup_count)) -lt "$live_count" ]]; then
            echo -e "${RED}Refusing to revert:${NC} backup has $backup_count skills but the live dir has $live_count."
            echo "  Restoring would delete $((live_count - backup_count)) skills. If intentional: $0 --revert --force-revert"
            exit 1
        fi
        rm -rf "$CLAUDE_DIR/skills"
        mv "$latest_backup" "$CLAUDE_DIR/skills"
        echo -e "${GREEN}✓${NC} Restored skills from $latest_backup"
    else
        echo -e "${RED}No backup found${NC}"
        exit 1
    fi

    # Remove installed plugins
    for plugin in "$CLAUDE_DIR/plugins"/*/; do
        name=$(basename "$plugin")
        if [[ -f "$REPO_ROOT/plugins/$name/.claude-plugin/plugin.json" ]]; then
            rm -rf "$plugin"
            echo -e "${GREEN}✓${NC} Removed $name from plugins/"
        fi
    done

    echo -e "${GREEN}Reverted to original setup${NC}"
    exit 0
fi

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  AgentOps Dogfood Setup${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""

# Step 1: Validate all plugins first
echo -e "${BLUE}Step 1: Validating plugins...${NC}"
if ! "$REPO_ROOT/scripts/validate-local.sh" > /dev/null 2>&1; then
    echo -e "${RED}Plugin validation failed. Fix issues first.${NC}"
    "$REPO_ROOT/scripts/validate-local.sh"
    exit 1
fi
echo -e "${GREEN}✓${NC} All plugins valid"
echo ""

# Step 2: Backup current skills
echo -e "${BLUE}Step 2: Backing up ~/.claude/skills...${NC}"
if [[ -d "$CLAUDE_DIR/skills" ]] && [[ ! -L "$CLAUDE_DIR/skills" ]]; then
    if [[ -d "$BACKUP_DIR" ]]; then
        echo -e "${YELLOW}!${NC} Backup already exists: $BACKUP_DIR"
    else
        cp -r "$CLAUDE_DIR/skills" "$BACKUP_DIR"
        echo -e "${GREEN}✓${NC} Backed up to $BACKUP_DIR"
    fi
else
    echo -e "${YELLOW}!${NC} No skills directory to backup"
fi
echo ""

# Step 3: Install plugins
echo -e "${BLUE}Step 3: Installing plugins to ~/.claude/plugins/...${NC}"
mkdir -p "$CLAUDE_DIR/plugins"

installed=0
for plugin_dir in "$REPO_ROOT/plugins"/*/; do
    name=$(basename "$plugin_dir")
    dst="$CLAUDE_DIR/plugins/$name"

    # Remove old installation
    [[ -d "$dst" ]] && rm -rf "$dst"

    # Copy plugin
    cp -r "$plugin_dir" "$dst"
    echo -e "${GREEN}✓${NC} Installed $name"
    installed=$((installed + 1))
done
echo ""

# Step 4: Summary
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Dogfood setup complete!${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""
echo "  Installed: $installed plugins"
echo "  Backup:    $BACKUP_DIR"
echo ""
echo "  Your skills now come from your local plugins."
echo "  When you run /vibe, /research, etc. - it uses YOUR code."
echo ""
echo "  To revert: $0 --revert"
echo ""
echo -e "${YELLOW}  NOTE: Restart Claude Code for changes to take effect.${NC}"
echo ""

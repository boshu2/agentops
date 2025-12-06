#!/bin/bash
# Claude Code Marketplace Installer
# Usage: ./install.sh [options] /path/to/project

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="1.0.0"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
    cat << EOF
Claude Code Marketplace Installer v${VERSION}

Usage: $(basename "$0") [options] <target-directory>

Options:
    --full              Install everything
    --minimal           Install core RPI + session commands only
    --commands <list>   Install specific commands (comma-separated)
    --agents <list>     Install specific agents (comma-separated)
    --skills <list>     Install specific skills (comma-separated)
    --no-kernel         Skip CLAUDE.md generation
    --no-progress       Skip progress file creation
    --dry-run           Show what would be installed
    -h, --help          Show this help

Examples:
    $(basename "$0") --full ~/projects/my-app
    $(basename "$0") --minimal ~/projects/my-app
    $(basename "$0") --commands research,plan,implement ~/projects/my-app
    $(basename "$0") --agents code-reviewer,test-generator ~/projects/my-app

Command Categories:
    rpi:        research, plan, implement
    bundles:    bundle-save, bundle-load, bundle-search, bundle-list, bundle-prune
    session:    session-start, session-end, session-resume
    metrics:    vibe-check, vibe-level
    learning:   learn, retro
    project:    project-init, progress-update
    quality:    code-review, architecture-review, generate-tests
    docs:       update-docs, create-architecture-documentation, create-onboarding-guide
    utilities:  ultra-think, maintain, containerize-application
    multi:      research-multi, bundle-load-multi

EOF
    exit 0
}

# Defaults
INSTALL_FULL=false
INSTALL_MINIMAL=false
COMMANDS=""
AGENTS=""
SKILLS=""
NO_KERNEL=false
NO_PROGRESS=false
DRY_RUN=false
TARGET_DIR=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --full)
            INSTALL_FULL=true
            shift
            ;;
        --minimal)
            INSTALL_MINIMAL=true
            shift
            ;;
        --commands)
            COMMANDS="$2"
            shift 2
            ;;
        --agents)
            AGENTS="$2"
            shift 2
            ;;
        --skills)
            SKILLS="$2"
            shift 2
            ;;
        --no-kernel)
            NO_KERNEL=true
            shift
            ;;
        --no-progress)
            NO_PROGRESS=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            TARGET_DIR="$1"
            shift
            ;;
    esac
done

if [[ -z "$TARGET_DIR" ]]; then
    echo -e "${RED}Error: Target directory required${NC}"
    usage
fi

# Resolve target directory
TARGET_DIR="$(cd "$TARGET_DIR" 2>/dev/null && pwd)" || {
    echo -e "${YELLOW}Creating directory: $TARGET_DIR${NC}"
    mkdir -p "$TARGET_DIR"
    TARGET_DIR="$(cd "$TARGET_DIR" && pwd)"
}

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Claude Code Marketplace Installer    ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "Target: ${GREEN}$TARGET_DIR${NC}"
echo ""

# Define command sets
MINIMAL_COMMANDS="research plan implement bundle-save bundle-load session-start session-end vibe-level"

RPI_COMMANDS="research plan implement"
BUNDLE_COMMANDS="bundle-save bundle-load bundle-search bundle-list bundle-prune"
SESSION_COMMANDS="session-start session-end session-resume"
METRICS_COMMANDS="vibe-check vibe-level"
LEARNING_COMMANDS="learn retro"
PROJECT_COMMANDS="project-init progress-update"
QUALITY_COMMANDS="code-review architecture-review generate-tests"
DOCS_COMMANDS="update-docs create-architecture-documentation create-onboarding-guide"
UTILITY_COMMANDS="ultra-think maintain containerize-application"
MULTI_COMMANDS="research-multi bundle-load-multi"

ALL_COMMANDS="$RPI_COMMANDS $BUNDLE_COMMANDS $SESSION_COMMANDS $METRICS_COMMANDS $LEARNING_COMMANDS $PROJECT_COMMANDS $QUALITY_COMMANDS $DOCS_COMMANDS $UTILITY_COMMANDS $MULTI_COMMANDS"

# Determine what to install
INSTALL_COMMANDS=""

if $INSTALL_FULL; then
    INSTALL_COMMANDS="$ALL_COMMANDS"
elif $INSTALL_MINIMAL; then
    INSTALL_COMMANDS="$MINIMAL_COMMANDS"
elif [[ -n "$COMMANDS" ]]; then
    # Expand category shortcuts
    INSTALL_COMMANDS=$(echo "$COMMANDS" | tr ',' ' ')
    INSTALL_COMMANDS=$(echo "$INSTALL_COMMANDS" | sed "s/rpi/$RPI_COMMANDS/g")
    INSTALL_COMMANDS=$(echo "$INSTALL_COMMANDS" | sed "s/bundles/$BUNDLE_COMMANDS/g")
    INSTALL_COMMANDS=$(echo "$INSTALL_COMMANDS" | sed "s/session/$SESSION_COMMANDS/g")
    INSTALL_COMMANDS=$(echo "$INSTALL_COMMANDS" | sed "s/metrics/$METRICS_COMMANDS/g")
else
    # Default to minimal
    INSTALL_COMMANDS="$MINIMAL_COMMANDS"
fi

# Create directories
echo -e "${YELLOW}Creating directories...${NC}"
if ! $DRY_RUN; then
    mkdir -p "$TARGET_DIR/.claude/commands"
    mkdir -p "$TARGET_DIR/.agents/bundles"
fi
echo -e "  ${GREEN}✓${NC} .claude/commands/"
echo -e "  ${GREEN}✓${NC} .agents/bundles/"

# Install commands
echo ""
echo -e "${YELLOW}Installing commands...${NC}"
for cmd in $INSTALL_COMMANDS; do
    src="$SCRIPT_DIR/commands/${cmd}.md"
    if [[ -f "$src" ]]; then
        if ! $DRY_RUN; then
            cp "$src" "$TARGET_DIR/.claude/commands/"
        fi
        echo -e "  ${GREEN}✓${NC} $cmd"
    else
        echo -e "  ${RED}✗${NC} $cmd (not found)"
    fi
done

# Install agents if specified
if [[ -n "$AGENTS" ]] || $INSTALL_FULL; then
    echo ""
    echo -e "${YELLOW}Installing agents...${NC}"

    if ! $DRY_RUN; then
        mkdir -p "$TARGET_DIR/.claude/agents"
    fi

    if $INSTALL_FULL; then
        AGENT_LIST=$(ls "$SCRIPT_DIR/agents/"*.md 2>/dev/null | xargs -n1 basename | sed 's/.md$//')
    else
        AGENT_LIST=$(echo "$AGENTS" | tr ',' ' ')
    fi

    for agent in $AGENT_LIST; do
        src="$SCRIPT_DIR/agents/${agent}.md"
        if [[ -f "$src" ]]; then
            if ! $DRY_RUN; then
                cp "$src" "$TARGET_DIR/.claude/agents/"
            fi
            echo -e "  ${GREEN}✓${NC} $agent"
        else
            echo -e "  ${RED}✗${NC} $agent (not found)"
        fi
    done
fi

# Install skills if specified
if [[ -n "$SKILLS" ]] || $INSTALL_FULL; then
    echo ""
    echo -e "${YELLOW}Installing skills...${NC}"

    if ! $DRY_RUN; then
        mkdir -p "$TARGET_DIR/.claude/skills"
    fi

    if $INSTALL_FULL; then
        SKILL_LIST=$(ls -d "$SCRIPT_DIR/skills/"*/ 2>/dev/null | xargs -n1 basename)
    else
        SKILL_LIST=$(echo "$SKILLS" | tr ',' ' ')
    fi

    for skill in $SKILL_LIST; do
        src="$SCRIPT_DIR/skills/${skill}"
        if [[ -d "$src" ]]; then
            if ! $DRY_RUN; then
                cp -r "$src" "$TARGET_DIR/.claude/skills/"
            fi
            echo -e "  ${GREEN}✓${NC} $skill"
        elif [[ -f "${src}.md" ]]; then
            if ! $DRY_RUN; then
                cp "${src}.md" "$TARGET_DIR/.claude/skills/"
            fi
            echo -e "  ${GREEN}✓${NC} $skill"
        else
            echo -e "  ${RED}✗${NC} $skill (not found)"
        fi
    done
fi

# Generate CLAUDE.md kernel
if ! $NO_KERNEL; then
    echo ""
    echo -e "${YELLOW}Generating CLAUDE.md kernel...${NC}"

    PROJECT_NAME=$(basename "$TARGET_DIR")

    if ! $DRY_RUN; then
        cat > "$TARGET_DIR/CLAUDE.md" << EOF
# Project: $PROJECT_NAME

## Behavioral Standards

<default_to_action>
Implement changes rather than suggesting. Infer intent and proceed.
</default_to_action>

<investigate_before_answering>
Read files before proposing changes. No speculation about unread code.
</investigate_before_answering>

<avoid_overengineering>
Only make requested changes. Keep solutions simple.
</avoid_overengineering>

## Intent Detection

| Intent | Keywords | Action |
|--------|----------|--------|
| Resume | "continue", "pick up", "back to" | Load bundles, read progress |
| End | "done", "stopping", "finished" | Save state, update progress |
| Status | "what's next", "where was I" | Show progress, next item |
| New Work | "add", "implement", "create" | Check bundles, start RPI |
| Bug Fix | "fix", "bug", "broken" | Debug directly |

## Session Protocol

On first interaction, check for progress files and display current state.

## Vibe Levels

| Level | Trust | Verify | Use For |
|-------|-------|--------|---------|
| 5 | 95% | Final only | Format, lint |
| 4 | 80% | Spot check | Boilerplate |
| 3 | 60% | Key outputs | Features |
| 2 | 40% | Every change | Integrations |
| 1 | 20% | Every line | Architecture |
| 0 | 0% | N/A | Research |

## Resources

| Resource | Location |
|----------|----------|
| Commands | \`.claude/commands/\` |
| Bundles | \`.agents/bundles/\` |
| Progress | \`claude-progress.json\` |
| Features | \`feature-list.json\` |

---
*Generated by Claude Code Marketplace v${VERSION}*
EOF
    fi
    echo -e "  ${GREEN}✓${NC} CLAUDE.md"
fi

# Create progress files
if ! $NO_PROGRESS; then
    echo ""
    echo -e "${YELLOW}Creating progress files...${NC}"

    TODAY=$(date +%Y-%m-%d)
    PROJECT_NAME=$(basename "$TARGET_DIR")

    if ! $DRY_RUN; then
        cat > "$TARGET_DIR/claude-progress.json" << EOF
{
  "project": "$PROJECT_NAME",
  "created": "$TODAY",
  "current_state": {
    "working_on": null,
    "blockers": [],
    "next_steps": [],
    "resume_summary": null
  },
  "sessions": []
}
EOF

        cat > "$TARGET_DIR/feature-list.json" << EOF
{
  "project": "$PROJECT_NAME",
  "created": "$TODAY",
  "mode": "standard",
  "features": []
}
EOF
    fi
    echo -e "  ${GREEN}✓${NC} claude-progress.json"
    echo -e "  ${GREEN}✓${NC} feature-list.json"
fi

# Summary
echo ""
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Installed:"
echo "  - $(echo $INSTALL_COMMANDS | wc -w | tr -d ' ') commands"
[[ -n "$AGENTS" ]] || $INSTALL_FULL && echo "  - agents"
[[ -n "$SKILLS" ]] || $INSTALL_FULL && echo "  - skills"
! $NO_KERNEL && echo "  - CLAUDE.md kernel"
! $NO_PROGRESS && echo "  - progress files"
echo ""
echo "Next steps:"
echo "  1. cd $TARGET_DIR"
echo "  2. Start Claude Code"
echo "  3. Run /session-start"
echo ""

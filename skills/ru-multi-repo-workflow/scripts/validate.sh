#!/bin/bash
#
# Validate — Health check for OSS review automation
#
# Usage:
#     ./validate.sh
#
# Checks:
#     - ru is available and working
#     - gh is installed and authenticated
#     - jq is available
#     - Network connectivity to GitHub
#
# Exit codes:
#     0 = All checks pass
#     1 = One or more checks failed

set -euo pipefail

FAILED=0
HAVE_GH=0
HAVE_RU=0
RU_CMD=""

echo "=============================================="
echo "OSS Review Automation - Health Check"
echo "=============================================="
echo ""

# 1. Check ru
echo -n "ru:     "
if command -v ru &>/dev/null; then
    HAVE_RU=1
    RU_CMD="ru"
    RU_VERSION=$(ru --version 2>/dev/null | head -1 || echo "unknown")
    echo "✓ found ($RU_VERSION)"
elif [ -x "/data/projects/repo_updater/ru" ]; then
    HAVE_RU=1
    RU_CMD="/data/projects/repo_updater/ru"
    echo "✓ found at /data/projects/repo_updater/ru"
    echo "        💡 Tip: Add to PATH: export PATH=\"/data/projects/repo_updater:\$PATH\""
else
    echo "✗ not found"
    echo "        Install from: https://github.com/Dicklesworthstone/repo_updater"
    FAILED=1
fi

# 2. Check gh
echo -n "gh:     "
if command -v gh &>/dev/null; then
    HAVE_GH=1
    GH_VERSION=$(gh --version 2>/dev/null | head -1 || echo "unknown")
    echo "✓ found ($GH_VERSION)"
else
    echo "✗ not found"
    echo "        Install: brew install gh"
    FAILED=1
fi

# 3. Check gh auth
echo -n "gh auth: "
if [ "$HAVE_GH" -eq 0 ]; then
    echo "⊘ skipped (gh not installed)"
elif gh auth status &>/dev/null; then
    GH_USER=$(gh api user --jq '.login' 2>/dev/null || echo "unknown")
    echo "✓ authenticated as $GH_USER"
else
    echo "✗ not authenticated"
    echo "        Run: gh auth login"
    FAILED=1
fi

# 4. Check jq
echo -n "jq:     "
if command -v jq &>/dev/null; then
    JQ_VERSION=$(jq --version 2>/dev/null || echo "unknown")
    echo "✓ found ($JQ_VERSION)"
else
    echo "✗ not found"
    echo "        Install: brew install jq"
    FAILED=1
fi

# 5. Check git
echo -n "git:    "
if command -v git &>/dev/null; then
    GIT_VERSION=$(git --version 2>/dev/null || echo "unknown")
    echo "✓ found ($GIT_VERSION)"
else
    echo "✗ not found"
    FAILED=1
fi

# 6. Check GitHub API connectivity
echo -n "GitHub API: "
if [ "$HAVE_GH" -eq 0 ]; then
    echo "⊘ skipped (gh not installed)"
elif gh api rate_limit &>/dev/null; then
    REMAINING=$(gh api rate_limit --jq '.rate.remaining' 2>/dev/null || echo "?")
    echo "✓ connected ($REMAINING requests remaining)"
else
    echo "✗ cannot connect"
    echo "        Check network and authentication"
    FAILED=1
fi

# 7. Check ru doctor (if ru is available)
if [ "$HAVE_RU" -eq 1 ]; then
    echo -n "ru doctor: "
    if $RU_CMD doctor &>/dev/null; then
        echo "✓ healthy"
    else
        echo "⚠️  issues detected (run 'ru doctor' for details)"
    fi
fi

echo ""

# Summary
if [ "$FAILED" -eq 0 ]; then
    echo "=============================================="
    echo "✓ All checks passed - ready for review automation"
    echo "=============================================="
    echo ""
    echo "Quick start:"
    echo "  ru sync -j4              # Sync all repos"
    echo "  ru review --dry-run     # See open issues/PRs"
    exit 0
else
    echo "=============================================="
    echo "✗ Some checks failed - fix issues above"
    echo "=============================================="
    exit 1
fi

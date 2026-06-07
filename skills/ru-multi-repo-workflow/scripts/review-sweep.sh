#!/bin/bash
#
# Review Sweep — Full issue/PR review workflow for one repo
#
# Usage:
#     ./review-sweep.sh owner/repo [/local/path]
#
# Output:
#     - Open issues (2025+)
#     - Open PRs
#     - Recent commits (context)
#     - Suggested next actions
#
# Requires: gh, git, jq

set -euo pipefail

REPO="${1:-}"
LOCAL="${2:-}"

if [ -z "$REPO" ]; then
    echo "Usage: $0 owner/repo [/local/path]"
    echo ""
    echo "Examples:"
    echo "  $0 Dicklesworthstone/mcp_agent_mail /data/projects/mcp_agent_mail"
    echo "  $0 Dicklesworthstone/repo_updater"
    exit 1
fi

# Derive local path if not provided
if [ -z "$LOCAL" ]; then
    REPO_NAME=$(echo "$REPO" | cut -d'/' -f2)
    LOCAL="/data/projects/$REPO_NAME"
fi

echo "=============================================="
echo "REVIEW SWEEP: $REPO"
echo "Local path: $LOCAL"
echo "=============================================="
echo ""

for tool in gh jq git; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "ERROR: Required tool not found: $tool"
        exit 1
    fi
done

# 1. Check auth
echo "--- Auth Check ---"
if ! gh auth status &>/dev/null; then
    echo "ERROR: Not authenticated with gh. Run: gh auth login"
    exit 1
fi
echo "✓ gh authenticated"
echo ""

# 2. Check local repo exists
echo "--- Local Repo Check ---"
if [ -d "$LOCAL/.git" ]; then
    echo "✓ Local repo exists at $LOCAL"

    # Show current branch and status
    BRANCH=$(git -C "$LOCAL" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    echo "  Branch: $BRANCH"

    # Check if dirty
    if [ -n "$(git -C "$LOCAL" status --porcelain 2>/dev/null)" ]; then
        echo "  ⚠️  Working tree has uncommitted changes"
    else
        echo "  Clean working tree"
    fi
else
    echo "⚠️  Local repo not found at $LOCAL"
    echo "   Clone with: gh repo clone $REPO $LOCAL"
fi
echo ""

# 3. List open issues (2025+)
echo "--- Open Issues (2025+) ---"
ISSUES=$(gh issue list -R "$REPO" --state open --json number,title,createdAt,labels --limit 100 2>/dev/null || echo "[]")
ISSUES_2025=$(echo "$ISSUES" | jq '[.[] | select(.createdAt >= "2025-01-01T00:00:00Z")]')
ISSUES_COUNT=$(echo "$ISSUES_2025" | jq 'length')

if [ "$ISSUES_COUNT" -eq 0 ]; then
    echo "No open issues from 2025+"
else
    echo "$ISSUES_COUNT open issue(s):"
    echo ""
    echo "$ISSUES_2025" | jq -r '.[] | "  #\(.number): \(.title[0:60])\n    Created: \(.createdAt[0:10])"'
fi
echo ""

# 4. Count stale issues (pre-2025)
STALE_COUNT=$(echo "$ISSUES" | jq '[.[] | select(.createdAt < "2025-01-01T00:00:00Z")] | length')
if [ "$STALE_COUNT" -gt 0 ]; then
    echo "--- Stale Issues (pre-2025) ---"
    echo "$STALE_COUNT stale issue(s) - consider closing"
    echo ""
fi

# 5. List open PRs
echo "--- Open PRs ---"
PRS=$(gh pr list -R "$REPO" --state open --json number,title,createdAt,author --limit 50 2>/dev/null || echo "[]")
PRS_COUNT=$(echo "$PRS" | jq 'length')

if [ "$PRS_COUNT" -eq 0 ]; then
    echo "No open PRs"
else
    echo "$PRS_COUNT open PR(s):"
    echo ""
    echo "$PRS" | jq -r '.[] | "  #\(.number): \(.title[0:60])\n    By: \(.author.login) on \(.createdAt[0:10])"'
    echo ""
    echo "  ⚠️  Remember: NEVER merge PRs. Mine for intel only."
fi
echo ""

# 6. Recent commits (context)
if [ -d "$LOCAL/.git" ]; then
    echo "--- Recent Commits (last 14 days) ---"
    COMMITS=$(git -C "$LOCAL" log --oneline --since="14 days ago" -n 10 2>/dev/null || true)
    if [ -n "$COMMITS" ]; then
        echo "$COMMITS"
    else
        echo "No commits in last 14 days"
    fi
    echo ""
fi

# 7. Suggested next actions
echo "--- Next Actions ---"

if [ "$ISSUES_COUNT" -gt 0 ]; then
    FIRST_ISSUE=$(echo "$ISSUES_2025" | jq -r '.[0].number')
    echo "1. Review first issue:    gh issue view $FIRST_ISSUE -R $REPO"
fi

if [ "$PRS_COUNT" -gt 0 ]; then
    FIRST_PR=$(echo "$PRS" | jq -r '.[0].number')
    echo "2. Mine first PR:         gh pr diff $FIRST_PR -R $REPO"
fi

if [ "$STALE_COUNT" -gt 0 ]; then
    echo "3. Clean stale issues:    gh issue list -R $REPO --json number,createdAt | jq '[.[] | select(.createdAt < \"2025-01-01\")]'"
fi

if [ -d "$LOCAL/.git" ]; then
    echo "4. Check recent changes:  git -C $LOCAL log --oneline -20"
fi

echo ""
echo "=============================================="

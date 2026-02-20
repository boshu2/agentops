#!/usr/bin/env bash
# Retag an existing release to include post-tag commits.
# Moves the tag to HEAD, re-publishes the GitHub release, and upgrades Homebrew.
#
# Usage: scripts/retag-release.sh <tag>
#   e.g.: scripts/retag-release.sh v2.13.0
#
# Prerequisites:
#   - CHANGELOG.md and release notes already updated
#   - All changes committed
#   - Working tree clean
#
# What it does:
#   1. Validates preconditions (clean tree, tag exists, commits after tag)
#   2. Moves the tag to HEAD
#   3. Pushes main + updated tag to origin
#   4. Deletes stale GitHub release (if any)
#   5. Triggers the release workflow
#   6. Waits for workflow completion
#   7. Upgrades Homebrew formula

set -euo pipefail

TAG="${1:-}"
REPO="${2:-boshu2/agentops}"

# --- Validation ---

if [[ -z "$TAG" ]]; then
  echo "Usage: scripts/retag-release.sh <tag> [repo]"
  echo "  e.g.: scripts/retag-release.sh v2.13.0"
  exit 1
fi

if [[ "$TAG" != v* ]]; then
  TAG="v${TAG}"
fi

echo "==> Retag release: $TAG"

# Clean working tree
if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: Working tree is not clean. Commit or stash changes first."
  exit 1
fi

# Tag must already exist locally
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "ERROR: Tag $TAG does not exist locally."
  exit 1
fi

# There must be commits after the tag
COMMITS_AFTER=$(git log --oneline "$TAG..HEAD" | wc -l | tr -d ' ')
if [[ "$COMMITS_AFTER" == "0" ]]; then
  echo "ERROR: No commits after $TAG — nothing to retag."
  exit 1
fi

echo "  $COMMITS_AFTER commit(s) after $TAG will be included."

# --- Move tag ---

OLD_SHA=$(git rev-parse --short "$TAG")
git tag -f "$TAG" HEAD
NEW_SHA=$(git rev-parse --short "$TAG")
echo "==> Tag moved: $OLD_SHA -> $NEW_SHA"

# --- Push ---

echo "==> Pushing main..."
git push origin main

echo "==> Updating remote tag..."
git push origin ":refs/tags/$TAG" 2>/dev/null || true
git push origin "$TAG"

# --- GitHub release ---

echo "==> Deleting stale GitHub release (if any)..."
gh release delete "$TAG" --repo "$REPO" --yes 2>/dev/null || true

echo "==> Triggering release workflow..."
RUN_URL=$(gh workflow run release.yml --repo "$REPO" -f "tag=$TAG" 2>&1)
echo "  $RUN_URL"

# Wait for the run to appear
sleep 5

RUN_ID=$(gh run list --repo "$REPO" --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
echo "==> Watching workflow run $RUN_ID..."
if gh run watch "$RUN_ID" --repo "$REPO" --exit-status; then
  echo "==> Release workflow succeeded."
else
  echo "ERROR: Release workflow failed. Check: https://github.com/$REPO/actions/runs/$RUN_ID"
  exit 1
fi

# --- Homebrew ---

echo "==> Upgrading Homebrew formula..."
brew update --quiet
if brew upgrade agentops 2>/dev/null; then
  brew link --overwrite agentops 2>/dev/null || true
  echo "==> Homebrew upgraded."
else
  echo "  (already at latest or link needed)"
  brew link --overwrite agentops 2>/dev/null || true
fi

# --- Verify ---

echo ""
echo "=== Retag complete ==="
echo "  Tag:     $TAG -> $(git rev-parse --short HEAD)"
echo "  Release: https://github.com/$REPO/releases/tag/$TAG"
echo "  Binary:  $(ao version 2>/dev/null | head -1)"

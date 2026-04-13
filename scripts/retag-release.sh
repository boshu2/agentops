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
#   2. Rewrites the tag at HEAD as an annotated tag, preserving the prior message
#   3. Deletes the stale GitHub release before the new tag push
#   4. Pushes main + updated tag to origin
#   5. Waits for the tag-push release workflow to complete
#   6. Upgrades Homebrew formula

set -euo pipefail

TAG="${1:-}"
REPO="${2:-boshu2/agentops}"
WAIT_START=""

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

verify_annotated_tag() {
  local tag="$1"
  local context="$2"
  local tag_type

  tag_type="$(git cat-file -t "refs/tags/$tag" 2>/dev/null || true)"
  if [[ "$tag_type" != "tag" ]]; then
    echo "ERROR: $context left $tag as ${tag_type:-missing}; expected annotated tag object."
    exit 1
  fi

  echo "==> Verified annotated tag object for $tag ($context)"
}

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

# Idempotency guard: if the tag already points at HEAD, skip
TAG_TYPE=$(git cat-file -t "refs/tags/$TAG")
TAG_SHA=$(git rev-parse "$TAG^{commit}" 2>/dev/null)
HEAD_SHA_CHECK=$(git rev-parse HEAD 2>/dev/null)
if [[ "$TAG_SHA" == "$HEAD_SHA_CHECK" && "$TAG_TYPE" == "tag" ]]; then
  verify_annotated_tag "$TAG" "idempotency check"
  echo "Tag $TAG already points at HEAD ($HEAD_SHA_CHECK) — nothing to retag."
  exit 0
fi

if [[ "$TAG_SHA" == "$HEAD_SHA_CHECK" ]]; then
  echo "Tag $TAG points at HEAD but is a $TAG_TYPE object; recreating annotated tag."
else
  # There must be commits after the tag
  COMMITS_AFTER=$(git log --oneline "$TAG..HEAD" | wc -l | tr -d ' ')
  if [[ "$COMMITS_AFTER" == "0" ]]; then
    echo "ERROR: No commits after $TAG — nothing to retag."
    exit 1
  fi

  echo "  $COMMITS_AFTER commit(s) after $TAG will be included."
fi

# --- Move tag ---

OLD_SHA=$(git rev-parse --short "$TAG^{commit}")
if [[ "$TAG_TYPE" == "tag" ]]; then
  TAG_MESSAGE="$(git tag -l "$TAG" --format='%(contents)')"
  TAGGER_DATE="$(git for-each-ref --format='%(taggerdate:iso-strict)' "refs/tags/$TAG")"
else
  TAG_MESSAGE="Release $TAG"
  TAGGER_DATE=""
fi

if [[ -z "$TAG_MESSAGE" ]]; then
  TAG_MESSAGE="Release $TAG"
fi

if [[ "$TAG_TYPE" == "tag" && -n "$TAGGER_DATE" ]]; then
  GIT_COMMITTER_DATE="$TAGGER_DATE" git tag -a -f "$TAG" -F - HEAD <<<"$TAG_MESSAGE"
else
  git tag -a -f "$TAG" -F - HEAD <<<"$TAG_MESSAGE"
fi
verify_annotated_tag "$TAG" "retag"

NEW_SHA=$(git rev-parse --short "$TAG^{commit}")
echo "==> Tag moved: $OLD_SHA -> $NEW_SHA (annotated)"

# --- GitHub release cleanup ---

echo "==> Deleting stale GitHub release (if any)..."
gh release delete "$TAG" --repo "$REPO" --yes 2>/dev/null || true

echo "==> Removing remote tag before republish..."
git push origin ":refs/tags/$TAG" 2>/dev/null || true

# --- Push ---

echo "==> Pushing main..."
git push origin main

echo "==> Updating remote tag..."
WAIT_START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
git push origin "$TAG"

# --- GitHub Actions ---

echo "==> Waiting for tag-push release workflow..."
RUN_ID=""
RUN_URL=""
HEAD_SHA="$(git rev-parse HEAD)"
for _ in {1..24}; do
  RUN_ID="$(gh run list \
    --repo "$REPO" \
    --workflow=release.yml \
    --event push \
    --limit 20 \
    --json databaseId,headSha,createdAt,url \
    --jq ".[] | select(.headSha == \"$HEAD_SHA\" and .createdAt >= \"$WAIT_START\") | .databaseId" \
    | head -n1)"
  if [[ -n "$RUN_ID" ]]; then
    RUN_URL="$(gh run list \
      --repo "$REPO" \
      --workflow=release.yml \
      --event push \
      --limit 20 \
      --json databaseId,headSha,createdAt,url \
      --jq ".[] | select(.databaseId == $RUN_ID) | .url" \
      | head -n1)"
    break
  fi
  sleep 5
done

if [[ -z "$RUN_ID" ]]; then
  echo "ERROR: Timed out waiting for the tag-push release workflow for $TAG"
  exit 1
fi

echo "==> Watching workflow run $RUN_ID..."
[[ -n "$RUN_URL" ]] && echo "  $RUN_URL"
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

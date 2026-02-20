#!/usr/bin/env bash
# Extract release notes for a given version from CHANGELOG.md.
# Generates user-facing highlights + full changelog for GitHub Release page.
#
# Usage: scripts/extract-release-notes.sh v2.9.2 [v2.9.0]
#   $1 = current tag (required)
#   $2 = previous tag (optional, for footer link)
#
# Expects: .agents/releases/YYYY-MM-DD-v<version>-notes.md for curated highlights.
# Falls back to CHANGELOG.md extraction if no curated notes exist.
#
# Output: writes release-notes.md to repo root

set -euo pipefail

TAG="${1:?Usage: extract-release-notes.sh TAG [PREV_TAG]}"
PREV_TAG="${2:-}"
VERSION="${TAG#v}"
REPO="boshu2/agentops"

CHANGELOG="CHANGELOG.md"
if [[ ! -f "$CHANGELOG" ]]; then
  echo "ERROR: $CHANGELOG not found" >&2
  exit 1
fi

# Extract the section for this version from CHANGELOG.md.
# Matches from "## [VERSION]" to the next "## [" line (exclusive).
CHANGELOG_SECTION=$(awk -v ver="$VERSION" '
  /^## \[/ {
    if (found) exit
    if (index($0, "[" ver "]")) { found=1; next }
  }
  found { print }
' "$CHANGELOG")

if [[ -z "$CHANGELOG_SECTION" ]]; then
  echo "ERROR: No CHANGELOG entry for $VERSION — add entry before releasing" >&2
  exit 1
fi

# Check for curated release notes (written by /release skill or manually).
# These are plain-English highlights, not the raw changelog.
CURATED_NOTES=""
NOTES_FILE=$(find .agents/releases -name "*-v${VERSION}-notes.md" 2>/dev/null | head -1 || true)
if [[ -n "$NOTES_FILE" && -f "$NOTES_FILE" ]]; then
  CURATED_NOTES=$(cat "$NOTES_FILE")
  echo "Using curated release notes from $NOTES_FILE" >&2
fi

# Build the release notes file
{
  # Header
  cat <<HEADER
\`brew update && brew upgrade agentops\` · \`npx skills@latest update\` · [checksums](https://github.com/${REPO}/releases/download/${TAG}/checksums.txt) · [verify provenance](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds)

---

HEADER

  # Curated highlights (if available)
  if [[ -n "$CURATED_NOTES" ]]; then
    echo "$CURATED_NOTES"
    echo ""
    echo "---"
    echo ""
    echo "<details>"
    echo "<summary>Full changelog</summary>"
    echo ""
    echo "$CHANGELOG_SECTION"
    echo ""
    echo "</details>"
  else
    # No curated notes — use changelog directly
    echo "$CHANGELOG_SECTION"
  fi

  echo ""
  echo "---"
  echo ""
  echo "**Full Changelog**: https://github.com/${REPO}/compare/${PREV_TAG:-v0.0.0}...${TAG}"
} > release-notes.md

echo "Release notes written to release-notes.md ($(wc -l < release-notes.md) lines)"

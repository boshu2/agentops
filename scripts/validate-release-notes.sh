#!/usr/bin/env bash
# Validate a curated release-notes file against the standard in
# skills/release/references/release-notes.md.
#
# Usage: scripts/validate-release-notes.sh <version>
#   <version> = release version, with or without leading "v" (e.g. v3.0.1 or 3.0.1)
#
# Enforces (hard fail, exit 1, on any violation):
#   - a curated docs/releases/*-v<version>-notes.md file exists (no silent
#     fallback to the raw CHANGELOG)
#   - the required section skeleton is present
#   - major releases (X.0.0) carry a "## Breaking Changes" section
#   - every "### " product-area heading is from the canonical taxonomy
#   - every top-level bullet under a product area uses a canonical action label
#
# This is the mechanical backstop for the failure where 3.0.0/3.0.1 shipped
# with no curated notes and the generator silently dumped the raw CHANGELOG.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

VERSION="${1:?Usage: validate-release-notes.sh <version>}"
VERSION="${VERSION#v}"

errors=0
fail() {
  echo "FAIL: $1" >&2
  errors=$((errors + 1))
}

# --- Locate the curated notes file (no silent fallback) ---

NOTES_FILE="$(find "$REPO_ROOT/docs/releases" -name "*-v${VERSION}-notes.md" 2>/dev/null | head -1 || true)"
if [[ -z "$NOTES_FILE" || ! -f "$NOTES_FILE" ]]; then
  echo "FAIL: no curated release-notes file docs/releases/*-v${VERSION}-notes.md" >&2
  echo "      Write one per the standard in skills/release/references/release-notes.md" >&2
  echo "      before tagging. The raw CHANGELOG is not an acceptable release body." >&2
  exit 1
fi
echo "Validating $NOTES_FILE (version $VERSION)"

# --- Required top-level sections ---

require_section() {
  local heading="$1"
  grep -Fxq "$heading" "$NOTES_FILE" || fail "missing required section: $heading"
}
require_section "## Highlights"
require_section "## Upgrade Notes"
require_section "## At a Glance"
require_section "## Product Areas"
require_section "## Known Issues"

# Full-changelog link must remain for archaeology.
grep -Fq "Full changelog" "$NOTES_FILE" || fail "missing the [Full changelog] link"

# --- Major releases (X.0.0) require a Breaking Changes section ---

if [[ "$VERSION" =~ ^[0-9]+\.0\.0$ ]]; then
  grep -Fxq "## Breaking Changes" "$NOTES_FILE" \
    || fail "major release ${VERSION} must include a '## Breaking Changes' section"
fi

# --- Canonical product-area taxonomy (order not enforced, membership is) ---

canonical_areas=(
  "Install, Upgrade, and Distribution"
  "CLI and Operator Commands"
  "Daemon, Scheduling, and Factory"
  "Skills and Workflows"
  "Codex and Runtime Integrations"
  "Hooks and Lifecycle"
  "Knowledge Flywheel, Search, and Memory"
  "Eval, Validation, and Release Gates"
  "Docs and Onboarding"
  "Security, Privacy, and Supply Chain"
  "Contributor/Internal Refactors"
)
is_canonical_area() {
  local candidate="$1"
  local area
  for area in "${canonical_areas[@]}"; do
    [[ "$candidate" == "$area" ]] && return 0
  done
  return 1
}

# Canonical action labels for product-area bullets.
LABEL_RE='^- (Added|Changed|Refactored|Fixed|Deprecated|Removed|Security|Docs): '

# --- Walk the Product Areas region: validate ### headings + bullet labels ---

in_product_areas=0
current_area=""
while IFS= read -r line; do
  if [[ "$line" == "## "* ]]; then
    if [[ "$line" == "## Product Areas" ]]; then
      in_product_areas=1
    else
      in_product_areas=0
    fi
    current_area=""
    continue
  fi
  [[ "$in_product_areas" -eq 1 ]] || continue

  if [[ "$line" == "### "* ]]; then
    current_area="${line:4}"  # strip the literal "### " prefix (4 chars)
    if ! is_canonical_area "$current_area"; then
      fail "non-canonical product-area heading: '### ${current_area}' (see skills/release/references/release-notes.md taxonomy)"
    fi
    continue
  fi

  # Top-level bullets (not indented sub-bullets) under a product area must
  # start with a canonical action label.
  if [[ "$line" == "- "* ]]; then
    if [[ -z "$current_area" ]]; then
      fail "bullet outside any '### <product area>' heading: ${line}"
    elif ! [[ "$line" =~ $LABEL_RE ]]; then
      fail "bullet missing a canonical action label (Added:/Changed:/Refactored:/Fixed:/Deprecated:/Removed:/Security:/Docs:): ${line}"
    fi
  fi
done < "$NOTES_FILE"

if [[ "$errors" -gt 0 ]]; then
  echo "" >&2
  echo "FAIL: ${errors} release-notes standard violation(s) in $NOTES_FILE" >&2
  exit 1
fi

echo "PASS: $NOTES_FILE conforms to the release-notes standard"

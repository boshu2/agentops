#!/usr/bin/env bash
# Validate a curated release-notes file against docs/contracts/release-notes.md.
#
# Usage: scripts/validate-release-notes.sh <version> [--since <prev-tag>] [--changed-files <file>]
#   <version>          release version, with or without leading "v" (e.g. v3.0.1 or 3.0.1)
#   --since <prev-tag> run the COVERAGE gate against git diff <prev-tag>..<version|HEAD>
#   --changed-files F  run the COVERAGE gate against a newline-separated path list in F
#                      (test seam; avoids needing git history)
#
# Release tiers (derived from the version):
#   major   X.0.0   requires "## Breaking Changes"; notes cover the whole
#                   last-release..HEAD delta (the entire major), not one stanza
#   minor   X.Y.0   standard sections; Breaking Changes only if actually breaking
#   hotfix  X.Y.Z   standard sections; narrow patch delta
#
# Enforces (hard fail, exit 1):
#   - a curated docs/releases/*-v<version>-notes.md file exists (no raw-CHANGELOG fallback)
#   - the required section skeleton is present
#   - major (X.0.0) carries a "## Breaking Changes" section
#   - every "### " product-area heading is from the canonical taxonomy
#   - every top-level product-area bullet uses a canonical action label
#   - COVERAGE: every product area the release actually touched (>= threshold files)
#     appears as a "### " section — this is the mechanical guard against an
#     under-scoped major (the v3.0.x churn root cause).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COVERAGE_THRESHOLD="${RELEASE_NOTES_COVERAGE_THRESHOLD:-3}"

VERSION=""
COVERAGE_SINCE=""
COVERAGE_FILES=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --since) COVERAGE_SINCE="${2:-}"; shift 2 ;;
    --changed-files) COVERAGE_FILES="${2:-}"; shift 2 ;;
    -*) echo "Unknown flag: $1" >&2; exit 2 ;;
    *) VERSION="$1"; shift ;;
  esac
done
[[ -n "$VERSION" ]] || { echo "Usage: validate-release-notes.sh <version> [--since <prev-tag>] [--changed-files <file>]" >&2; exit 2; }
VERSION="${VERSION#v}"

# --- Tier classification ---
if [[ "$VERSION" =~ ^[0-9]+\.0\.0$ ]]; then
  TIER="major"
elif [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.0$ ]]; then
  TIER="minor"
elif [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  TIER="hotfix"
else
  echo "FAIL: version '${VERSION}' is not X.Y.Z" >&2
  exit 1
fi

errors=0
fail() {
  echo "FAIL: $1" >&2
  errors=$((errors + 1))
}

# --- Locate the curated notes file (no silent fallback) ---

NOTES_FILE="$(find "$REPO_ROOT/docs/releases" -name "*-v${VERSION}-notes.md" 2>/dev/null | head -1 || true)"
if [[ -z "$NOTES_FILE" || ! -f "$NOTES_FILE" ]]; then
  echo "FAIL: no curated release-notes file docs/releases/*-v${VERSION}-notes.md" >&2
  echo "      Scaffold one with: scripts/scaffold-release-notes.sh v${VERSION} --since <prev-tag>" >&2
  echo "      then curate per docs/contracts/release-notes.md before tagging." >&2
  exit 1
fi
echo "Validating $NOTES_FILE (version $VERSION, tier $TIER)"

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

if [[ "$TIER" == "major" ]]; then
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
  local candidate="$1" area
  for area in "${canonical_areas[@]}"; do
    [[ "$candidate" == "$area" ]] && return 0
  done
  return 1
}

# Map a changed path to its product area (first match wins). Echoes the area
# name, or nothing for paths that don't map to a release-facing area. Mirrors
# the Coverage Workflow table in docs/contracts/release-notes.md.
map_path_to_area() {
  local p="$1"
  case "$p" in
    .codex-plugin/*|scripts/install-codex*|scripts/validate-codex*) echo "Codex and Runtime Integrations" ;;
    scripts/install*|.goreleaser*|.github/workflows/release.yml|packs/*install*) echo "Install, Upgrade, and Distribution" ;;
    cli/cmd/ao/*|cli/docs/COMMANDS.md) echo "CLI and Operator Commands" ;;
    cli/internal/daemon/*|cli/internal/schedule/*|cli/internal/agentworker/*|cli/internal/gascity/*) echo "Daemon, Scheduling, and Factory" ;;
    skills/*|skills-codex*) echo "Skills and Workflows" ;;
    hooks/*|cli/embedded/hooks/*) echo "Hooks and Lifecycle" ;;
    cli/internal/knowledge/*|cli/internal/harvest/*|cli/internal/pool/*|cli/internal/lifecycle/*|cli/internal/search/*) echo "Knowledge Flywheel, Search, and Memory" ;;
    cli/internal/eval/*|evals/*|tests/*|.github/workflows/validate.yml) echo "Eval, Validation, and Release Gates" ;;
    scripts/security*|scripts/toolchain-validate*|*sbom*) echo "Security, Privacy, and Supply Chain" ;;
    README.md|docs/*|PRODUCT.md) echo "Docs and Onboarding" ;;
    *) echo "" ;;
  esac
}

# --- Walk the Product Areas region: validate ### headings + bullet labels ---

LABEL_RE='^- (Added|Changed|Refactored|Fixed|Deprecated|Removed|Security|Docs): '
declare -A present_areas=()
in_product_areas=0
current_area=""
while IFS= read -r line; do
  if [[ "$line" == "## "* ]]; then
    in_product_areas=$([[ "$line" == "## Product Areas" ]] && echo 1 || echo 0)
    current_area=""
    continue
  fi
  [[ "$in_product_areas" -eq 1 ]] || continue

  if [[ "$line" == "### "* ]]; then
    current_area="${line:4}"  # strip the literal "### " prefix (4 chars)
    present_areas["$current_area"]=1
    if ! is_canonical_area "$current_area"; then
      fail "non-canonical product-area heading: '### ${current_area}' (see docs/contracts/release-notes.md taxonomy)"
    fi
    continue
  fi

  if [[ "$line" == "- "* ]]; then
    if [[ -z "$current_area" ]]; then
      fail "bullet outside any '### <product area>' heading: ${line}"
    elif ! [[ "$line" =~ $LABEL_RE ]]; then
      fail "bullet missing a canonical action label (Added:/Changed:/Refactored:/Fixed:/Deprecated:/Removed:/Security:/Docs:): ${line}"
    fi
  fi
done < "$NOTES_FILE"

# --- COVERAGE gate: touched areas must appear in the notes ---

CHANGED=""
if [[ -n "$COVERAGE_FILES" ]]; then
  if [[ -f "$COVERAGE_FILES" ]]; then
    CHANGED="$(cat "$COVERAGE_FILES")"
  else
    fail "--changed-files not found: $COVERAGE_FILES"
  fi
elif [[ -n "$COVERAGE_SINCE" ]] && git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
  endpoint="HEAD"
  git -C "$REPO_ROOT" rev-parse "v${VERSION}^{commit}" >/dev/null 2>&1 && endpoint="v${VERSION}"
  if git -C "$REPO_ROOT" rev-parse "${COVERAGE_SINCE}^{commit}" >/dev/null 2>&1; then
    CHANGED="$(git -C "$REPO_ROOT" diff --name-only "${COVERAGE_SINCE}..${endpoint}" 2>/dev/null || true)"
  else
    echo "  coverage: --since '${COVERAGE_SINCE}' does not resolve; skipping coverage gate" >&2
  fi
fi

if [[ -n "$CHANGED" ]]; then
  declare -A area_hits=()
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    a="$(map_path_to_area "$f")"
    [[ -n "$a" ]] && area_hits["$a"]=$(( ${area_hits["$a"]:-0} + 1 ))
  done <<< "$CHANGED"

  echo "  coverage: touched areas (>= ${COVERAGE_THRESHOLD} files must be documented):"
  for a in "${!area_hits[@]}"; do
    n="${area_hits[$a]}"
    if [[ "$n" -ge "$COVERAGE_THRESHOLD" ]]; then
      if [[ -n "${present_areas[$a]:-}" ]]; then
        echo "    ok  ($n) $a"
      else
        fail "release touched '${a}' (${n} files) but the notes have no '### ${a}' section"
      fi
    fi
  done
fi

if [[ "$errors" -gt 0 ]]; then
  echo "" >&2
  echo "FAIL: ${errors} release-notes standard violation(s) in $NOTES_FILE" >&2
  exit 1
fi

echo "PASS: $NOTES_FILE conforms to the release-notes standard (tier $TIER)"

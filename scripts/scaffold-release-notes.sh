#!/usr/bin/env bash
# Scaffold a curated release-notes draft from the git range, in the correct
# tier shape, so notes start comprehensive instead of hand-mined from scratch.
#
# Usage: scripts/scaffold-release-notes.sh <version> [--since <prev-tag>] [--force]
#   <version>          release version (with or without leading "v")
#   --since <prev-tag> the previous release tag; defaults to the latest tag that
#                      is not <version> (i.e. the prior release)
#   --force            overwrite an existing notes file
#
# Output: docs/releases/<today>-v<version>-notes.md, pre-populated with:
#   - the tier-correct section skeleton (major adds "## Breaking Changes")
#   - one "### <product area>" per area the range touched (>= threshold files),
#     each seeded with the conventional-commit subjects for that area as
#     "- Changed: <subject>" stubs to curate.
# The result is a DRAFT — curate the prose, fix action labels, then run
# scripts/validate-release-notes.sh to confirm it conforms.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"
COVERAGE_THRESHOLD="${RELEASE_NOTES_COVERAGE_THRESHOLD:-3}"

VERSION=""
SINCE=""
FORCE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --since) SINCE="${2:-}"; shift 2 ;;
    --force) FORCE=1; shift ;;
    -*) echo "Unknown flag: $1" >&2; exit 2 ;;
    *) VERSION="$1"; shift ;;
  esac
done
[[ -n "$VERSION" ]] || { echo "Usage: scaffold-release-notes.sh <version> [--since <prev-tag>] [--force]" >&2; exit 2; }
VERSION="${VERSION#v}"

if [[ "$VERSION" =~ ^[0-9]+\.0\.0$ ]]; then TIER="major"
elif [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.0$ ]]; then TIER="minor"
elif [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then TIER="hotfix"
else echo "ERROR: version '${VERSION}' is not X.Y.Z" >&2; exit 1; fi

if [[ -z "$SINCE" ]]; then
  SINCE="$(git tag --sort=-version:refname | grep -v "^v${VERSION}$" | head -1)"
fi
[[ -n "$SINCE" ]] || { echo "ERROR: could not determine --since tag" >&2; exit 1; }

endpoint="HEAD"
git rev-parse "v${VERSION}^{commit}" >/dev/null 2>&1 && endpoint="v${VERSION}"

OUT="docs/releases/$(date -u +%Y-%m-%d)-v${VERSION}-notes.md"
if [[ -e "$OUT" && "$FORCE" -ne 1 ]]; then
  echo "ERROR: $OUT exists (use --force to overwrite)" >&2
  exit 1
fi

map_path_to_area() {
  case "$1" in
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

# Tally touched files per area.
declare -A area_hits=()
while IFS= read -r f; do
  [[ -n "$f" ]] || continue
  a="$(map_path_to_area "$f")"
  [[ -n "$a" ]] && area_hits["$a"]=$(( ${area_hits["$a"]:-0} + 1 ))
done < <(git diff --name-only "${SINCE}..${endpoint}" 2>/dev/null || true)

ncommits="$(git rev-list --count --no-merges "${SINCE}..${endpoint}" 2>/dev/null || echo "?")"

{
  echo "## Highlights"
  echo ""
  echo "<!-- ${TIER} release. ${ncommits} commits since ${SINCE}. 2-4 sentences: theme + biggest outcomes. -->"
  echo ""
  echo "## Upgrade Notes"
  echo ""
  echo "- <required action, or \"No manual action required\">"
  [[ "$TIER" == "major" ]] && echo "- See \`docs/MIGRATION-${VERSION%%.*}.0.md\` for the full migration."
  echo ""
  if [[ "$TIER" == "major" ]]; then
    echo "## Breaking Changes"
    echo ""
    echo "- <each breaking change: what changed, what to use instead, migration pointer>"
    echo ""
  fi
  echo "## At a Glance"
  echo ""
  echo "| Product Area | Added | Changed | Refactored | Fixed | Deprecated/Removed |"
  echo "|---|---:|---:|---:|---:|---:|"
  for a in "${!area_hits[@]}"; do
    [[ "${area_hits[$a]}" -ge "$COVERAGE_THRESHOLD" ]] && echo "| $a | 0 | 0 | 0 | 0 | 0 |"
  done
  echo ""
  echo "## Product Areas"
  echo ""
  # One section per touched area, seeded with that area's commit subjects.
  for a in "${!area_hits[@]}"; do
    [[ "${area_hits[$a]}" -ge "$COVERAGE_THRESHOLD" ]] || continue
    echo "### $a"
    echo ""
    # Seed bullets from commit subjects whose changed files map to this area.
    git log --no-merges --pretty='%H %s' "${SINCE}..${endpoint}" 2>/dev/null | while IFS= read -r line; do
      sha="${line%% *}"; subj="${line#* }"
      files="$(git diff --name-only "${sha}~1..${sha}" 2>/dev/null || true)"
      while IFS= read -r f; do
        [[ -n "$f" ]] || continue
        if [[ "$(map_path_to_area "$f")" == "$a" ]]; then
          clean="$(printf '%s' "$subj" | sed -E 's/ \(#[0-9]+\)$//; s/^[a-z]+\([a-z0-9_-]+\)!?: //')"
          echo "- Changed: ${clean}"
          break
        fi
      done <<< "$files"
    done | sort -u | head -12
    echo ""
  done
  echo "## Known Issues"
  echo ""
  echo "- <known risk/limitation, or \"No release-blocking known issues.\">"
  echo ""
  echo "[Full changelog](../CHANGELOG.md)"
} > "$OUT"

echo "Scaffolded $OUT (tier $TIER, ${ncommits} commits since ${SINCE})"
echo "Next: curate prose + action labels, then: scripts/validate-release-notes.sh v${VERSION} --since ${SINCE}"

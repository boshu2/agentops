#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

errors=0

run_check() {
  local name="$1"
  shift

  echo "=== $name ==="
  if "$@"; then
    echo "PASS: $name"
  else
    echo "FAIL: $name"
    errors=$((errors + 1))
  fi
  echo ""
}

# scripts/extract-release-notes.sh hard-errors at publish time when the tagged
# version has no CHANGELOG.md section. Surface that gap here, where nightly,
# regen-all and the local release rehearsal run, instead of at publish.
validate_changelog_entry() {
  local changelog="$REPO_ROOT/CHANGELOG.md"
  local release_version

  if ! release_version="$(jq -r '.version // empty' "$REPO_ROOT/.claude-plugin/plugin.json")"; then
    echo "ERROR: cannot read .claude-plugin/plugin.json with jq"
    return 1
  fi
  if [[ -z "$release_version" ]]; then
    echo "MISMATCH: .claude-plugin/plugin.json has no version"
    return 1
  fi
  if ! grep -Fq "## [$release_version]" "$changelog"; then
    echo "MISMATCH: root CHANGELOG.md missing release entry for $release_version"
    return 1
  fi
  # docs/CHANGELOG.md is the published byte-identical copy of the root file.
  if ! cmp -s "$changelog" "$REPO_ROOT/docs/CHANGELOG.md"; then
    echo "MISMATCH: docs/CHANGELOG.md differs from CHANGELOG.md (cp CHANGELOG.md docs/CHANGELOG.md)"
    return 1
  fi
}

run_check "Link validation" bash "$REPO_ROOT/tests/docs/validate-links.sh"
run_check "Skill count validation" bash "$REPO_ROOT/tests/docs/validate-skill-count.sh"
run_check "Changelog release entry" validate_changelog_entry

if [[ "$errors" -gt 0 ]]; then
  echo "FAIL: doc-release gate failed ($errors check(s) failed)"
  exit 1
fi

echo "PASS: doc-release gate succeeded"

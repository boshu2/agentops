#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MANIFEST_PATH="${HOOKS_DOC_PARITY_MANIFEST:-$REPO_ROOT/hooks/hooks.json}"
STALE_REGEX="${HOOKS_DOC_PARITY_REGEX:-\\b(12 hooks|22 hooks|12 lifecycle events)\\b}"

if [[ -n "${HOOKS_DOC_PARITY_FILES:-}" ]]; then
  # shellcheck disable=SC2206
  SCOPED_FILES=(${HOOKS_DOC_PARITY_FILES})
else
  SCOPED_FILES=(
    "$REPO_ROOT/AGENTS.md"
    "$REPO_ROOT/docs/how-it-works.md"
    "$REPO_ROOT/docs/leverage-points.md"
    "$REPO_ROOT/docs/GLOSSARY.md"
    "$REPO_ROOT/docs/strategic-direction.md"
    "$REPO_ROOT/docs/cli-skills-map.md"
    "$REPO_ROOT/docs/CONTRIBUTING.md"
    "$REPO_ROOT/docs/troubleshooting.md"
  )
fi

missing=0
for file in "${SCOPED_FILES[@]}"; do
  if [[ ! -f "$file" ]]; then
    if [[ "$missing" -eq 0 ]]; then
      echo "HOOKS_DOC_PARITY: missing scoped files:"
    fi
    echo "  - $file"
    missing=1
  fi
done
if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "HOOKS_DOC_PARITY: manifest not found: $MANIFEST_PATH"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "HOOKS_DOC_PARITY: jq is required to read runtime hook count from $MANIFEST_PATH"
  exit 1
fi

active_hook_count="$(jq -r '.hooks | length' "$MANIFEST_PATH" 2>/dev/null || true)"
if [[ -z "$active_hook_count" || "$active_hook_count" == "null" ]]; then
  echo "HOOKS_DOC_PARITY: unable to read .hooks length from $MANIFEST_PATH"
  exit 1
fi

if command -v rg >/dev/null 2>&1; then
  matches="$(rg -n -i --no-heading "$STALE_REGEX" "${SCOPED_FILES[@]}" || true)"
else
  matches="$(grep -nEi "$STALE_REGEX" "${SCOPED_FILES[@]}" || true)"
fi

if [[ -n "$matches" ]]; then
  echo "HOOKS_DOC_PARITY: drift detected."
  echo "Runtime manifest active hook events: $active_hook_count"
  echo "Stale hook-count claims found in scoped docs:"
  echo ""

  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    file="${entry%%:*}"
    rest="${entry#*:}"
    line_no="${rest%%:*}"
    line_text="${rest#*:}"

    suggested="$(echo "$line_text" \
      | sed "s/12 hooks/$active_hook_count active hooks/g" \
      | sed "s/22 hooks/$active_hook_count active hooks/g" \
      | sed "s/12 lifecycle events/$active_hook_count active lifecycle events/g")"

    if [[ "$suggested" == "$line_text" ]]; then
      suggested="Update this line to reflect hooks/hooks.json runtime contract."
    fi

    echo "  --- $file:$line_no"
    echo "  - $line_text"
    echo "  + $suggested"
    echo ""
  done <<< "$matches"

  echo "Suggested follow-up:"
  echo "  1) Update the lines above to runtime-manifest wording."
  echo "  2) Re-run: bash scripts/validate-hooks-doc-parity.sh"
  exit 1
fi

if [[ "${SKIP_CLI_SKILLS_MAP_PARITY:-0}" != "1" ]]; then
  bash "$REPO_ROOT/scripts/validate-cli-skills-map.sh"
fi

echo "HOOKS_DOC_PARITY: PASS (${#SCOPED_FILES[@]} files checked, active hooks: $active_hook_count)"
exit 0

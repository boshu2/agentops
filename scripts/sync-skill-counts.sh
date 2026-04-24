#!/usr/bin/env bash
# sync-skill-counts.sh — Single-source-of-truth skill count updater.
# Reads actual counts from disk + SKILL-TIERS.md, patches all doc files.
# Run after adding/removing a skill to keep all references in sync.
#
# Usage: scripts/sync-skill-counts.sh [--check]
#   --check   Dry-run: report mismatches without modifying files (exit 1 if any)
set -euo pipefail

export LC_ALL=C

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK_ONLY=false
changes=0
errors=0

if [[ "${1:-}" == "--check" ]]; then
  CHECK_ONLY=true
elif [[ $# -gt 0 ]]; then
  echo "ERROR: unknown argument '$1'"
  echo "Usage: scripts/sync-skill-counts.sh [--check]"
  exit 2
fi

# --- Derive truth from disk ---

TOTAL=$(find "$REPO_ROOT/skills" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | wc -l | tr -d ' ')
CODEX_TOTAL=$(find "$REPO_ROOT/skills-codex" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | wc -l | tr -d ' ')
CODEX_OVERRIDES=$(find "$REPO_ROOT/skills-codex-overrides" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | wc -l | tr -d ' ')
HOOK_EVENT_SECTIONS=$(jq -r '.hooks | length' "$REPO_ROOT/hooks/hooks.json" 2>/dev/null || echo 0)

USER_FACING=$(sed -n '/^### User-Facing Skills/,/^### Internal Skills/p' "$REPO_ROOT/skills/SKILL-TIERS.md" \
  | grep -c '^| \*\*')

INTERNAL_ROWS=$(sed -n '/^### Internal Skills/,/^---$/p' "$REPO_ROOT/skills/SKILL-TIERS.md" \
  | grep -c '^| ')
INTERNAL=$((INTERNAL_ROWS - 1))  # subtract header row

echo "Skill counts from disk:"
echo "  Total:       $TOTAL"
echo "  User-facing: $USER_FACING"
echo "  Internal:    $INTERNAL"
echo "  Codex:       $CODEX_TOTAL"
echo "  Overrides:   $CODEX_OVERRIDES"
echo "  Hook events: $HOOK_EVENT_SECTIONS"
echo ""

if [[ "$INTERNAL_ROWS" -lt 1 ]]; then
  echo "ERROR: SKILL-TIERS.md internal table header row not found"
  exit 1
fi

if [[ $((USER_FACING + INTERNAL)) -ne "$TOTAL" ]]; then
  echo "ERROR: SKILL-TIERS.md tables ($((USER_FACING + INTERNAL))) != directories ($TOTAL)"
  echo "Fix SKILL-TIERS.md first — add/remove the skill row, then re-run."
  exit 1
fi

# --- Define all patch targets ---

patch_file() {
  local file="$1"
  local match_regex="$2"
  local sed_expr="$3"
  local desc="$4"
  local match_count
  local tmp_file

  if [[ ! -f "$file" ]]; then
    echo "ERROR: $desc file not found: $file"
    errors=$((errors + 1))
    return
  fi

  match_count=$(grep -E -c "$match_regex" "$file" || true)
  if [[ "$match_count" -eq 0 ]]; then
    echo "ERROR: $desc pattern not found (fail-closed)"
    echo "       file: $file"
    echo "       match: $match_regex"
    errors=$((errors + 1))
    return
  fi
  if [[ "$match_count" -gt 1 ]]; then
    echo "ERROR: $desc pattern matched $match_count lines (expected exactly 1)"
    echo "       file: $file"
    echo "       match: $match_regex"
    errors=$((errors + 1))
    return
  fi

  tmp_file=$(mktemp)
  sed -E "$sed_expr" "$file" > "$tmp_file"

  if $CHECK_ONLY; then
    if cmp -s "$file" "$tmp_file"; then
      echo "OK:   $desc"
    else
      echo "DRIFT: $desc"
      changes=$((changes + 1))
    fi
    rm -f "$tmp_file"
    return
  fi

  if cmp -s "$file" "$tmp_file"; then
    echo "OK:      $desc"
    rm -f "$tmp_file"
    return
  fi

  mv "$tmp_file" "$file"
  echo "UPDATED: $desc"
  changes=$((changes + 1))
}

# SKILL-TIERS.md header counts
patch_file "$REPO_ROOT/skills/SKILL-TIERS.md" \
  '^### User-Facing Skills \([0-9]+\)' \
  "s|^### User-Facing Skills \\([0-9]+\\)|### User-Facing Skills (${USER_FACING})|" \
  "SKILL-TIERS.md user-facing header"

patch_file "$REPO_ROOT/skills/SKILL-TIERS.md" \
  '^### Internal Skills \([0-9]+\)' \
  "s|^### Internal Skills \\([0-9]+\\)|### Internal Skills (${INTERNAL})|" \
  "SKILL-TIERS.md internal header"

# docs/SKILLS.md: "all N AgentOps skills (M user-facing + K internal)"
patch_file "$REPO_ROOT/docs/SKILLS.md" \
  '^Complete reference for all [0-9]+ AgentOps skills \([0-9]+ user-facing [+] [0-9]+ internal\)\.$' \
  "s|^Complete reference for all [0-9]+ AgentOps skills \\([0-9]+ user-facing [+] [0-9]+ internal\\)\\.$|Complete reference for all ${TOTAL} AgentOps skills (${USER_FACING} user-facing + ${INTERNAL} internal).|" \
  "docs/SKILLS.md header"

# docs/SKILLS.md command: "/update  # Reinstall all N skills"
patch_file "$REPO_ROOT/docs/SKILLS.md" \
  '^/update[[:space:]]+# Reinstall all [0-9]+ skills$' \
  "s|^(/update[[:space:]]+# Reinstall all )[0-9]+( skills)$|\\1${TOTAL}\\2|" \
  "docs/SKILLS.md /update command"

# docs/ARCHITECTURE.md: "skills/ # N skills (M user-facing, K internal)"
patch_file "$REPO_ROOT/docs/ARCHITECTURE.md" \
  'skills/[[:space:]]+# [0-9]+ skills \([0-9]+ user-facing, [0-9]+ internal\)$' \
  "s|(skills/[[:space:]]+# )[0-9]+ skills \\([0-9]+ user-facing, [0-9]+ internal\\)$|\\1${TOTAL} skills (${USER_FACING} user-facing, ${INTERNAL} internal)|" \
  "docs/ARCHITECTURE.md skills tree"

# PRODUCT.md current count surfaces.
patch_file "$REPO_ROOT/PRODUCT.md" \
  'Skills system — [0-9]+ skills,' \
  "s|(Skills system — )[0-9]+( skills,)|\\1${TOTAL}\\2|" \
  "PRODUCT.md convergence skill count"

patch_file "$REPO_ROOT/PRODUCT.md" \
  '^### 1[.] Skills \([0-9]+ skills across 4 runtimes\)$' \
  "s|^### 1[.] Skills \\([0-9]+ skills across 4 runtimes\\)$|### 1. Skills (${TOTAL} skills across 4 runtimes)|" \
  "PRODUCT.md skills layer heading"

patch_file "$REPO_ROOT/PRODUCT.md" \
  '[[:space:]][0-9]+ skills, [0-9]+ runtime hook event sections,' \
  "s|([[:space:]])[0-9]+ skills, [0-9]+ runtime hook event sections,|\\1${TOTAL} skills, ${HOOK_EVENT_SECTIONS} runtime hook event sections,|" \
  "PRODUCT.md zero-setup value proposition"

patch_file "$REPO_ROOT/PRODUCT.md" \
  'Distribution/runtime reach: [0-9]+ shared skills, [0-9]+ checked-in Codex artifacts, and [0-9]+ Codex overrides' \
  "s|Distribution/runtime reach: [0-9]+ shared skills, [0-9]+ checked-in Codex artifacts, and [0-9]+ Codex overrides|Distribution/runtime reach: ${TOTAL} shared skills, ${CODEX_TOTAL} checked-in Codex artifacts, and ${CODEX_OVERRIDES} Codex overrides|" \
  "PRODUCT.md distribution/runtime reach"

echo ""

if [[ "$errors" -gt 0 ]]; then
  echo "FAIL: $errors pattern enforcement error(s) found"
  exit 1
fi

# --- Verify with existing validator ---
if ! $CHECK_ONLY; then
  echo "=== Verifying with validate-skill-count.sh ==="
  if bash "$REPO_ROOT/tests/docs/validate-skill-count.sh" > /dev/null 2>&1; then
    echo "PASS: All counts verified consistent"
  else
    echo "FAIL: Counts still inconsistent after patching — run tests/docs/validate-skill-count.sh"
    exit 1
  fi
fi

echo ""
if [[ "$changes" -gt 0 ]]; then
  if $CHECK_ONLY; then
    echo "DRIFT: $changes file(s) have stale skill counts. Run: scripts/sync-skill-counts.sh"
    exit 1
  else
    echo "DONE: $changes file(s) updated. Counts synced to $TOTAL total ($USER_FACING user-facing, $INTERNAL internal)."
  fi
else
  echo "DONE: All counts already in sync."
fi

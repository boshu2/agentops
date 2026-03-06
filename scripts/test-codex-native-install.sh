#!/usr/bin/env bash
set -euo pipefail

# test-codex-native-install.sh
# Verifies Codex-native skill generation + install flow.
#
# What it checks:
# 1) shellcheck on codex conversion/install scripts
# 2) skill integrity gate (heal --strict)
# 3) sync-codex-native-skills.sh succeeds
# 4) install-codex-native-skills.sh succeeds into temp destination
# 5) Installed skill count and required files (SKILL.md + prompt.md)
# 6) Generated SKILL.md files use $skill syntax (no known /skill references)
#
# Usage:
#   bash scripts/test-codex-native-install.sh
#   bash scripts/test-codex-native-install.sh --only research,vibe

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

ONLY_CSV=""
SKIP_LINT="false"

usage() {
  cat <<'EOF'
test-codex-native-install.sh

Options:
  --only <a,b,c>   Test only selected skills
  --skip-lint      Skip shellcheck + markdownlint
  --help           Show this help

Examples:
  bash scripts/test-codex-native-install.sh
  bash scripts/test-codex-native-install.sh --only research,vibe
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only)
      ONLY_CSV="${2:-}"
      shift 2
      ;;
    --skip-lint)
      SKIP_LINT="true"
      shift 1
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

info() {
  echo "INFO: $*"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

require_file() {
  [[ -f "$1" ]] || fail "Required file missing: $1"
}

SYNC_SCRIPT="$REPO_ROOT/scripts/sync-codex-native-skills.sh"
INSTALL_SCRIPT="$REPO_ROOT/scripts/install-codex-native-skills.sh"
EXPORT_SCRIPT="$REPO_ROOT/scripts/export-claude-skills-to-codex.sh"
CONVERTER_SCRIPT="$REPO_ROOT/skills/converter/scripts/convert.sh"
HEAL_SCRIPT="$REPO_ROOT/skills/heal-skill/scripts/heal.sh"

require_file "$SYNC_SCRIPT"
require_file "$INSTALL_SCRIPT"
require_file "$EXPORT_SCRIPT"
require_file "$CONVERTER_SCRIPT"
require_file "$HEAL_SCRIPT"
require_cmd bash
require_cmd find
require_cmd awk
require_cmd sed
require_cmd rg

if [[ "$SKIP_LINT" != "true" ]]; then
  require_cmd shellcheck
  require_cmd markdownlint

  info "Running shellcheck on codex pipeline scripts"
  shellcheck "$SYNC_SCRIPT" "$INSTALL_SCRIPT" "$EXPORT_SCRIPT" "$CONVERTER_SCRIPT"

  info "Running markdownlint on install docs"
  markdownlint \
    README.md \
    AGENTS.md \
    docs/reference.md \
    docs/CONTRIBUTING.md \
    docs/ARCHITECTURE.md \
    docs/troubleshooting.md \
    docs/INCIDENT-RUNBOOK.md
fi

info "Running strict skill integrity gate"
bash "$HEAL_SCRIPT" --strict >/dev/null

info "Building Codex-native skills"
SYNC_ARGS=()
if [[ -n "$ONLY_CSV" ]]; then
  SYNC_ARGS+=(--only "$ONLY_CSV")
fi
bash "$SYNC_SCRIPT" "${SYNC_ARGS[@]}" >/dev/null

timestamp="$(date +%Y%m%d-%H%M%S)"
DEST="/tmp/codex-native-install-test-${timestamp}"
BACKUP="/tmp/codex-native-install-backup-${timestamp}"

info "Installing Codex-native skills to temp destination"
INSTALL_ARGS=(--dest "$DEST" --backup "$BACKUP")
if [[ -n "$ONLY_CSV" ]]; then
  INSTALL_ARGS+=(--only "$ONLY_CSV")
fi
bash "$INSTALL_SCRIPT" "${INSTALL_ARGS[@]}" >/dev/null

[[ -d "$DEST" ]] || fail "Install destination not created: $DEST"

expected_count=0
if [[ -n "$ONLY_CSV" ]]; then
  IFS=',' read -r -a selected <<<"$ONLY_CSV"
  for skill in "${selected[@]}"; do
    skill="$(echo "$skill" | xargs)"
    [[ -n "$skill" ]] || continue
    [[ -d "$REPO_ROOT/skills-codex/$skill" ]] || fail "Converted skill not found: skills-codex/$skill"
    expected_count=$((expected_count + 1))
  done
else
  expected_count="$(find "$REPO_ROOT/skills-codex" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
fi

installed_count="$(find "$DEST" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
[[ "$installed_count" == "$expected_count" ]] || fail "Installed count mismatch (expected $expected_count, got $installed_count)"

info "Verifying installed files"
while IFS= read -r skill_dir; do
  [[ -n "$skill_dir" ]] || continue
  [[ -f "$skill_dir/SKILL.md" ]] || fail "Missing SKILL.md in $skill_dir"
  [[ -f "$skill_dir/prompt.md" ]] || fail "Missing prompt.md in $skill_dir"
  head -n 1 "$skill_dir/SKILL.md" | rg -q '^---$' || fail "Missing YAML frontmatter in $skill_dir/SKILL.md"
done < <(find "$DEST" -mindepth 1 -maxdepth 1 -type d | sort)

# Build regex alternation from known converted skill names.
skill_pattern="$(
  find "$REPO_ROOT/skills-codex" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; \
    | sort \
    | awk '
      BEGIN { ORS="" }
      {
        gsub(/[][(){}.^$*+?|\\-]/, "\\\\&", $0)
        if (NR > 1) { printf "|" }
        printf "%s", $0
      }
    '
)"
[[ -n "$skill_pattern" ]] || fail "Could not build skill-name regex for slash-command check"

info "Checking Codex entrypoints for known slash-command references"
entrypoint_files=()
while IFS= read -r -d '' file; do
  entrypoint_files+=("$file")
done < <(find "$REPO_ROOT/skills-codex" -type f \( -name "SKILL.md" -o -name "prompt.md" \) -print0)

if [[ "${#entrypoint_files[@]}" -eq 0 ]]; then
  fail "No Codex entrypoint files found for slash-command check"
fi

if rg --pcre2 -n "(^|[^A-Za-z0-9_/])/(${skill_pattern})(?![A-Za-z0-9-])" "${entrypoint_files[@]}" >/dev/null 2>&1; then
  fail "Found known /skill command references in skills-codex output"
fi

echo ""
echo "PASS: Codex-native install flow verified"
echo "  skills tested: $installed_count"
echo "  install dir: $DEST"
echo "  backup dir: $BACKUP"

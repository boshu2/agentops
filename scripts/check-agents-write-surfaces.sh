#!/usr/bin/env bash
set -euo pipefail

# check-agents-write-surfaces.sh — guard the .agents/ write-surface contract.
#
# Every top-level subdir under .agents/ that production code (cli/**/*.go
# non-test, scripts/, hooks/, lib/) writes to must be catalogued in
# docs/contracts/agents-write-surfaces.md. The catalog has an explicit
# allowlist between BEGIN/END markers; this script parses that block and
# fails when production code references a subdir that isn't documented.
#
# Skill-owned subdirs (.agents/<skill-name>/) are auto-allowed when an
# active skill exists at skills/<skill-name>/SKILL.md. New skills don't
# need a doc edit; new CLI/script/hook write surfaces do.

usage() {
  cat <<USAGE
Usage: $0 [--json]

Options:
  --json   Emit a machine-readable summary instead of human prose.

Exit codes:
  0  OK (all referenced subdirs documented or skill-owned)
  1  Undocumented top-level subdirs found in production code
  2  Bad invocation or missing contract doc
USAGE
}

JSON=false
case "${1:-}" in
  --json) JSON=true ;;
  -h|--help) usage; exit 0 ;;
  '') ;;
  *) usage; exit 2 ;;
esac

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTRACT_DOC="$REPO_ROOT/docs/contracts/agents-write-surfaces.md"
SKILLS_DIR="$REPO_ROOT/skills"

if [[ ! -f "$CONTRACT_DOC" ]]; then
  echo "ERROR: contract doc missing: $CONTRACT_DOC" >&2
  exit 2
fi

# Parse allowlist between markers. Lines starting with '#' are treated as
# comments. Empty lines are ignored. Each remaining line must be a single
# top-level subdir name with no slashes.
allowlist_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp"' EXIT
awk '
  /<!-- BEGIN agents-write-surfaces-allowlist -->/ { inside=1; next }
  /<!-- END agents-write-surfaces-allowlist -->/   { inside=0; next }
  inside { print }
' "$CONTRACT_DOC" \
  | sed -E 's/[[:space:]]+#.*$//' \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
  | awk 'NF && $1 !~ /^#/' \
  | sort -u > "$allowlist_tmp"

if [[ ! -s "$allowlist_tmp" ]]; then
  echo "ERROR: allowlist block is empty or markers missing in $CONTRACT_DOC" >&2
  exit 2
fi

# Reject malformed entries (must be lowercase letters, digits, '-' or '_').
malformed="$(grep -vE '^[a-z0-9][a-z0-9_-]*$' "$allowlist_tmp" || true)"
if [[ -n "$malformed" ]]; then
  echo "ERROR: malformed allowlist entries in $CONTRACT_DOC:" >&2
  echo "$malformed" >&2
  exit 2
fi

# Extract referenced top-level subdirs from production code.
# - cli/**/*.go excluding _test.go
# - scripts/*.sh, hooks/*.sh, lib/*.sh
referenced_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$referenced_tmp"' EXIT

scan_dirs=()
[[ -d "$REPO_ROOT/scripts" ]] && scan_dirs+=("$REPO_ROOT/scripts")
[[ -d "$REPO_ROOT/hooks" ]]   && scan_dirs+=("$REPO_ROOT/hooks")
[[ -d "$REPO_ROOT/lib" ]]     && scan_dirs+=("$REPO_ROOT/lib")

{
  if [[ -d "$REPO_ROOT/cli" ]]; then
    find "$REPO_ROOT/cli" -type f -name '*.go' ! -name '*_test.go' -print0 2>/dev/null \
      | xargs -0 -r grep -hEo '\.agents/[a-z][a-zA-Z0-9_-]*' 2>/dev/null || true
  fi
  if [[ ${#scan_dirs[@]} -gt 0 ]]; then
    find "${scan_dirs[@]}" -type f \( -name '*.sh' -o -name '*.bash' \) -print0 2>/dev/null \
      | xargs -0 -r grep -hEo '\.agents/[a-z][a-zA-Z0-9_-]*' 2>/dev/null || true
  fi
} | sed -E 's|^\.agents/([a-zA-Z0-9_-]+).*|\1|' | sort -u > "$referenced_tmp"

# Compute active skill names — skill-owned subdirs are auto-allowed.
skills_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$referenced_tmp" "$skills_tmp"' EXIT
: > "$skills_tmp"
if [[ -d "$SKILLS_DIR" ]]; then
  shopt -s nullglob
  for d in "$SKILLS_DIR"/*/; do
    [[ -f "${d}SKILL.md" ]] && basename "$d" >> "$skills_tmp"
  done
  shopt -u nullglob
  if [[ -s "$skills_tmp" ]]; then
    sort -u "$skills_tmp" -o "$skills_tmp"
  fi
fi

# undocumented = referenced - allowlist - skills
undocumented_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$referenced_tmp" "$skills_tmp" "$undocumented_tmp"' EXIT
comm -23 "$referenced_tmp" "$allowlist_tmp" \
  | comm -23 - "$skills_tmp" > "$undocumented_tmp"

UNDOC_COUNT=$(wc -l < "$undocumented_tmp" | tr -d ' ')
ALLOW_COUNT=$(wc -l < "$allowlist_tmp" | tr -d ' ')
REF_COUNT=$(wc -l < "$referenced_tmp" | tr -d ' ')

if [[ "$JSON" == "true" ]]; then
  printf '{"contract":"%s","allowlist_size":%s,"referenced":%s,"undocumented":[' \
    "${CONTRACT_DOC#"$REPO_ROOT"/}" "$ALLOW_COUNT" "$REF_COUNT"
  first=1
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    if [[ "$first" -eq 1 ]]; then first=0; else printf ','; fi
    printf '"%s"' "$entry"
  done < "$undocumented_tmp"
  printf '],"status":"%s"}\n' "$([ "$UNDOC_COUNT" -gt 0 ] && echo fail || echo ok)"
fi

if [[ "$UNDOC_COUNT" -gt 0 ]]; then
  if [[ "$JSON" != "true" ]]; then
    echo "ERROR: $UNDOC_COUNT undocumented .agents/ write surface(s) found in production code." >&2
    echo "Add an entry under '<!-- BEGIN agents-write-surfaces-allowlist -->' in:" >&2
    echo "  $CONTRACT_DOC" >&2
    echo "Undocumented subdirs:" >&2
    sed 's/^/  - /' "$undocumented_tmp" >&2
  fi
  exit 1
fi

if [[ "$JSON" != "true" ]]; then
  echo "agents-write-surfaces contract OK: $REF_COUNT referenced subdirs, $ALLOW_COUNT allowlisted, all documented or skill-owned."
fi
exit 0

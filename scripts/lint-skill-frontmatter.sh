#!/usr/bin/env bash
# lint-skill-frontmatter.sh — enforce required keys on skills/*/SKILL.md.
#
# The hexagonal-architecture model (BC1-BC5) generates docs/contracts/
# context-map.md from each skill's YAML frontmatter. When a SKILL.md is
# missing `consumes`, `produces`, `context_rel`, or `hexagonal_role`, the
# generated map drifts silently and the next consumer of the catalog hits
# downstream surprises.
#
# This script walks every skills/*/SKILL.md and validates:
#   - name: non-empty string
#   - description: non-empty string
#   - hexagonal_role: one of {domain, driving-adapter, driven-adapter,
#                              supporting, generic}
#   - consumes: key present (value may be empty list)
#   - produces: key present (value may be empty list)
#   - context_rel: key present (value may be empty list)
#
# Modes:
#   --check          exit 1 on any miss (CI gate)
#   --list           emit one line per missing key, sorted by skill
#   --json           machine-readable summary
#   --skill <name>   limit to a single skill
#
# Exit codes:
#   0 — all skills clean (or no skills found)
#   1 — at least one missing required key
#   2 — usage error
#   3 — execution error (missing awk, no skills dir, etc.)

set -euo pipefail

MODE="check"
ONE_SKILL=""
JSON=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --check) MODE="check" ;;
    --list) MODE="list" ;;
    --json) MODE="json"; JSON=1 ;;
    --skill) shift; ONE_SKILL="${1:-}" ;;
    -h|--help) usage 0 ;;
    *) echo "lint-skill-frontmatter: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

# Resolve repo root so the script works from any subdir.
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SKILLS_DIR="$ROOT/skills"
if [ ! -d "$SKILLS_DIR" ]; then
  echo "lint-skill-frontmatter: skills/ directory not found at $SKILLS_DIR" >&2
  exit 3
fi

VALID_HEX_ROLES=" domain driving-adapter driven-adapter supporting generic "

# Extract YAML frontmatter block (between first two `---` lines) of a file.
extract_frontmatter() {
  awk '/^---$/{c++; if(c==2) exit; next} c==1' "$1"
}

# Check a single SKILL.md, emit findings to stdout (one per line).
check_one() {
  local file="$1"
  local skill
  skill="$(basename "$(dirname "$file")")"
  local fm
  fm="$(extract_frontmatter "$file")"
  if [ -z "$fm" ]; then
    echo "$skill|missing|no-frontmatter|file=$file"
    return 1
  fi

  local rc=0

  # Helper: does the frontmatter have key X (any form)?
  has_key() {
    printf '%s\n' "$fm" | grep -qE "^${1}:[[:space:]]*"
  }
  # Helper: scalar value (everything after `<key>:` on the same line, trimmed).
  get_scalar() {
    printf '%s\n' "$fm" | sed -n "s/^${1}:[[:space:]]*//p" | head -1
  }

  if ! has_key name; then
    echo "$skill|missing|name|file=$file"; rc=1
  elif [ -z "$(get_scalar name)" ]; then
    echo "$skill|empty|name|file=$file"; rc=1
  fi

  if ! has_key description; then
    echo "$skill|missing|description|file=$file"; rc=1
  elif [ -z "$(get_scalar description)" ]; then
    echo "$skill|empty|description|file=$file"; rc=1
  fi

  if ! has_key hexagonal_role; then
    echo "$skill|missing|hexagonal_role|file=$file"; rc=1
  else
    local role
    role="$(get_scalar hexagonal_role)"
    if [ -z "$role" ]; then
      echo "$skill|empty|hexagonal_role|file=$file"; rc=1
    elif ! printf '%s' "$VALID_HEX_ROLES" | grep -q " $role "; then
      echo "$skill|invalid|hexagonal_role|value=$role|file=$file"; rc=1
    fi
  fi

  for k in consumes produces context_rel; do
    if ! has_key "$k"; then
      echo "$skill|missing|$k|file=$file"; rc=1
    fi
  done

  return $rc
}

# Collect SKILL.md paths.
declare -a skills_paths=()
if [ -n "$ONE_SKILL" ]; then
  one="$SKILLS_DIR/$ONE_SKILL/SKILL.md"
  if [ ! -r "$one" ]; then
    echo "lint-skill-frontmatter: skill not found: $ONE_SKILL" >&2
    exit 3
  fi
  skills_paths+=("$one")
else
  while IFS= read -r p; do
    skills_paths+=("$p")
  done < <(find "$SKILLS_DIR" -mindepth 2 -maxdepth 2 -name SKILL.md | sort)
fi

if [ "${#skills_paths[@]}" -eq 0 ]; then
  if [ "$JSON" -eq 1 ]; then
    echo '{"total":0,"clean":0,"violations":0,"findings":[]}'
  else
    echo "lint-skill-frontmatter: no SKILL.md files found"
  fi
  exit 0
fi

# Process all skills, collect findings.
findings_file="$(mktemp)"
trap 'rm -f "$findings_file"' EXIT
violations=0
clean=0
total=0

for p in "${skills_paths[@]}"; do
  total=$((total + 1))
  if check_one "$p" >> "$findings_file"; then
    clean=$((clean + 1))
  else
    violations=$((violations + 1))
  fi
done

if [ "$MODE" = "json" ]; then
  # Build JSON findings array.
  findings_json="$(awk -F'|' '{
    file=""; value=""
    for (i=4; i<=NF; i++) {
      if ($i ~ /^file=/) { file=substr($i,6) }
      else if ($i ~ /^value=/) { value=substr($i,7) }
    }
    printf "{\"skill\":\"%s\",\"status\":\"%s\",\"key\":\"%s\",\"value\":\"%s\",\"file\":\"%s\"}\n", $1, $2, $3, value, file
  }' "$findings_file" | paste -sd, -)"
  printf '{"total":%d,"clean":%d,"violations":%d,"findings":[%s]}\n' \
    "$total" "$clean" "$violations" "${findings_json}"
elif [ "$MODE" = "list" ]; then
  if [ -s "$findings_file" ]; then
    sort "$findings_file"
  fi
  echo
  echo "lint-skill-frontmatter: $total skill(s); $clean clean, $violations with missing/invalid keys"
else
  # check mode (default)
  if [ -s "$findings_file" ]; then
    echo "lint-skill-frontmatter: FAIL — $violations skill(s) with frontmatter issues:"
    sort "$findings_file" | sed 's/^/  /'
  else
    echo "lint-skill-frontmatter: OK — $total skill(s) clean"
  fi
fi

if [ "$violations" -gt 0 ]; then
  exit 1
fi
exit 0

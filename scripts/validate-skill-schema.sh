#!/usr/bin/env bash
# validate-skill-schema.sh — Validate SKILL.md YAML frontmatter against JSON schema
#
# Finds all skills/*/SKILL.md files, extracts YAML frontmatter, and validates
# each against schemas/skill-frontmatter.v1.schema.json.
#
# Validation tiers (best available wins):
#   1. yq + python3 jsonschema — full JSON Schema Draft-07 validation
#   2. yq + jq — structural checks (required fields, types, enum values)
#   3. grep — basic required-field presence check
#
# Usage: scripts/validate-skill-schema.sh [--verbose]
# Exit:  0 = all pass, 1 = failures found

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCHEMA="$REPO_ROOT/schemas/skill-frontmatter.v1.schema.json"
SKILLS_DIR="$REPO_ROOT/skills"
VERBOSE=0

if [[ "${1:-}" == "--verbose" ]]; then
  VERBOSE=1
fi

# --- Colors (disabled in CI / non-tty) ---
if [[ -t 1 ]]; then
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; NC=''
fi

pass=0
fail=0
warn=0
total=0
failures=""

# --- Extract YAML frontmatter from a SKILL.md file ---
# Outputs everything between the first pair of --- delimiters (exclusive).
extract_frontmatter() {
  local file="$1"
  awk 'BEGIN{found=0} /^---$/{if(found){exit}else{found=1;next}} found{print}' "$file"
}

# --- Detect validation tier ---
HAS_YQ=0
HAS_PYTHON_JSONSCHEMA=0
HAS_JQ=0
YQ_VARIANT=""

if command -v yq &>/dev/null; then
  HAS_YQ=1
  # Detect yq variant: "go" (Mike Farah, supports `-o json`) vs "python" (kislyuk,
  # jq wrapper that emits JSON by default from YAML). Both are packaged under the
  # same binary name in different distros.
  if yq --help 2>&1 | grep -q 'jq filter'; then
    YQ_VARIANT="python"
  else
    YQ_VARIANT="go"
  fi
fi

if command -v jq &>/dev/null; then
  HAS_JQ=1
fi

# yq_to_json: run a jq filter over YAML on stdin and emit JSON, regardless of
# which yq variant is installed.
yq_to_json() {
  local filter="${1:-.}"
  case "$YQ_VARIANT" in
    go) yq -o json "$filter" ;;
    python) yq "$filter" ;;
    *) return 1 ;;
  esac
}

if command -v python3 &>/dev/null; then
  if python3 -c "import jsonschema, json, yaml" &>/dev/null; then
    HAS_PYTHON_JSONSCHEMA=1
  fi
fi

if [[ $HAS_YQ -eq 1 && $HAS_PYTHON_JSONSCHEMA -eq 1 ]]; then
  TIER="full"
  echo "=== SKILL.md Schema Validation (full: yq + python3 jsonschema) ==="
elif [[ $HAS_YQ -eq 1 && $HAS_JQ -eq 1 ]]; then
  TIER="structural"
  echo "=== SKILL.md Schema Validation (structural: yq + jq) ==="
else
  TIER="basic"
  echo "=== SKILL.md Schema Validation (basic: grep) ==="
fi

# --- Full validation via python3 jsonschema ---
validate_full() {
  local skill_name="$1"
  local frontmatter="$2"

  local json_data
  json_data=$(echo "$frontmatter" | yq_to_json '.' 2>&1) || {
    echo -e "  ${RED}FAIL${NC} $skill_name: invalid YAML frontmatter"
    [[ $VERBOSE -eq 1 ]] && echo "       yq error: $json_data"
    return 1
  }

  local result
  result=$(python3 -c "
import json, sys
from jsonschema import validate, ValidationError

schema = json.load(open('$SCHEMA'))
data = json.loads(sys.stdin.read())
try:
    validate(instance=data, schema=schema)
    print('OK')
except ValidationError as e:
    print(f'FAIL: {e.message}')
" <<< "$json_data" 2>&1) || true

  if [[ "$result" == "OK" ]]; then
    [[ $VERBOSE -eq 1 ]] && echo -e "  ${GREEN}PASS${NC} $skill_name"
    return 0
  else
    echo -e "  ${RED}FAIL${NC} $skill_name: ${result#FAIL: }"
    return 1
  fi
}

# --- Structural validation via yq + jq ---
validate_structural() {
  local skill_name="$1"
  local frontmatter="$2"
  local errors=""

  local json_data
  json_data=$(echo "$frontmatter" | yq_to_json '.' 2>&1) || {
    echo -e "  ${RED}FAIL${NC} $skill_name: invalid YAML frontmatter"
    return 1
  }

  # Check required fields
  for field in name description skill_api_version; do
    if ! echo "$json_data" | jq -e ".[\"$field\"]" &>/dev/null; then
      errors="${errors}missing required field '$field'; "
    fi
  done

  # Check name is string
  local name_type
  name_type=$(echo "$json_data" | jq -r '.name | type' 2>/dev/null)
  if [[ "$name_type" != "string" && -z "$errors" ]]; then
    errors="${errors}'name' must be a string (got $name_type); "
  fi

  # Check description is string
  local desc_type
  desc_type=$(echo "$json_data" | jq -r '.description | type' 2>/dev/null)
  if [[ "$desc_type" != "string" ]]; then
    errors="${errors}'description' must be a string (got $desc_type); "
  fi

  # Check skill_api_version is 1
  local api_ver
  api_ver=$(echo "$json_data" | jq -r '.skill_api_version' 2>/dev/null)
  if [[ "$api_ver" != "1" ]]; then
    errors="${errors}'skill_api_version' must be 1 (got $api_ver); "
  fi

  # Check metadata.tier enum if present
  local tier
  tier=$(echo "$json_data" | jq -r '.metadata.tier // empty' 2>/dev/null)
  if [[ -n "$tier" ]]; then
    local valid_tiers="judgment execution library session product contribute meta background orchestration cross-vendor knowledge"
    if ! echo "$valid_tiers" | grep -qw "$tier"; then
      errors="${errors}'metadata.tier' invalid value '$tier'; "
    fi
  fi

  # Check for unknown top-level keys (additionalProperties: false)
  local valid_keys="name description skill_api_version metadata user-invocable context allowed-tools license compatibility model output_contract"
  local actual_keys
  actual_keys=$(echo "$json_data" | jq -r 'keys[]' 2>/dev/null)
  for key in $actual_keys; do
    if ! echo "$valid_keys" | grep -qw "$key"; then
      errors="${errors}unknown top-level property '$key'; "
    fi
  done

  if [[ -n "$errors" ]]; then
    echo -e "  ${RED}FAIL${NC} $skill_name: $errors"
    return 1
  else
    [[ $VERBOSE -eq 1 ]] && echo -e "  ${GREEN}PASS${NC} $skill_name"
    return 0
  fi
}

# --- Basic validation via grep ---
validate_basic() {
  local skill_name="$1"
  local frontmatter="$2"
  local errors=""

  # Check required fields exist
  if ! echo "$frontmatter" | grep -q "^name:"; then
    errors="${errors}missing 'name'; "
  fi
  if ! echo "$frontmatter" | grep -q "^description:"; then
    errors="${errors}missing 'description'; "
  fi
  if ! echo "$frontmatter" | grep -q "^skill_api_version:"; then
    errors="${errors}missing 'skill_api_version'; "
  fi

  # Check skill_api_version value
  if echo "$frontmatter" | grep -q "^skill_api_version:"; then
    local ver
    ver=$(echo "$frontmatter" | grep "^skill_api_version:" | sed 's/skill_api_version:[[:space:]]*//')
    if [[ "$ver" != "1" ]]; then
      errors="${errors}'skill_api_version' must be 1 (got '$ver'); "
    fi
  fi

  if [[ -n "$errors" ]]; then
    echo -e "  ${RED}FAIL${NC} $skill_name: $errors"
    return 1
  else
    [[ $VERBOSE -eq 1 ]] && echo -e "  ${GREEN}PASS${NC} $skill_name"
    return 0
  fi
}

# --- Validate schema file exists ---
if [[ ! -f "$SCHEMA" ]]; then
  echo -e "${YELLOW}WARNING${NC}: Schema file not found at $SCHEMA"
  echo "Falling back to basic validation without schema."
  TIER="basic"
fi

# --- Main loop ---
for skill_dir in "$SKILLS_DIR"/*/; do
  [[ ! -d "$skill_dir" ]] && continue
  skill_name=$(basename "$skill_dir")
  skill_file="$skill_dir/SKILL.md"

  if [[ ! -f "$skill_file" ]]; then
    echo -e "  ${YELLOW}SKIP${NC} $skill_name: no SKILL.md"
    warn=$((warn + 1))
    continue
  fi

  # Verify frontmatter delimiters exist
  if ! head -1 "$skill_file" | grep -q "^---$"; then
    echo -e "  ${RED}FAIL${NC} $skill_name: SKILL.md does not start with ---"
    fail=$((fail + 1))
    failures="${failures}  - $skill_name\n"
    total=$((total + 1))
    continue
  fi

  frontmatter=$(extract_frontmatter "$skill_file")
  if [[ -z "$frontmatter" ]]; then
    echo -e "  ${RED}FAIL${NC} $skill_name: empty frontmatter"
    fail=$((fail + 1))
    failures="${failures}  - $skill_name\n"
    total=$((total + 1))
    continue
  fi

  total=$((total + 1))

  case "$TIER" in
    full)
      if validate_full "$skill_name" "$frontmatter"; then
        pass=$((pass + 1))
      else
        fail=$((fail + 1))
        failures="${failures}  - $skill_name\n"
      fi
      ;;
    structural)
      if validate_structural "$skill_name" "$frontmatter"; then
        pass=$((pass + 1))
      else
        fail=$((fail + 1))
        failures="${failures}  - $skill_name\n"
      fi
      ;;
    basic)
      if validate_basic "$skill_name" "$frontmatter"; then
        pass=$((pass + 1))
      else
        fail=$((fail + 1))
        failures="${failures}  - $skill_name\n"
      fi
      ;;
  esac
done

# --- Summary ---
echo ""
echo "--- Results ---"
echo "Total: $total | Pass: $pass | Fail: $fail | Warn: $warn"

if [[ $fail -gt 0 ]]; then
  echo ""
  echo "Failed skills:"
  echo -e "$failures"
  echo "FAIL: $fail skill(s) failed schema validation"
  exit 1
fi

echo ""
echo "All $pass skill(s) passed schema validation"
exit 0

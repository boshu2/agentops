#!/usr/bin/env bash
# Validate each skills/<name>/SKILL.md YAML frontmatter block against
# schemas/skill-frontmatter.v2.schema.json.
#
# Default mode: schema violations fail; missing optional hexagonal fields
# (hexagonal_role / consumes / produces / context_rel) are warnings only
# (stderr) and do not fail the run.
#
# --strict: warnings also fail (used in pre-push and Wave-2 acceptance).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SCHEMA_PATH="${SKILL_FRONTMATTER_SCHEMA:-$REPO_ROOT/schemas/skill-frontmatter.v2.schema.json}"
SKILLS_ROOT="${SKILL_FRONTMATTER_SKILLS_ROOT:-$REPO_ROOT/skills}"

STRICT=0
for arg in "$@"; do
  case "$arg" in
    --strict) STRICT=1 ;;
    -h|--help)
      cat <<'USAGE'
Usage: scripts/validate-skill-frontmatter.sh [--strict]

  Validates every skills/<name>/SKILL.md YAML frontmatter against
  schemas/skill-frontmatter.v2.schema.json.

  Default:  schema violations fail; missing optional hexagonal fields warn.
  --strict: warnings (missing optional fields) also fail.
USAGE
      exit 0
      ;;
    *)
      echo "validate-skill-frontmatter: unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$SCHEMA_PATH" ]]; then
  echo "validate-skill-frontmatter: schema not found: $SCHEMA_PATH" >&2
  exit 1
fi

if [[ ! -d "$SKILLS_ROOT" ]]; then
  echo "validate-skill-frontmatter: skills root not found: $SKILLS_ROOT" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "validate-skill-frontmatter: python3 is required" >&2
  exit 1
fi

# Probe required python deps once up front so the per-file loop fails fast.
if ! python3 -c "import yaml, jsonschema" >/dev/null 2>&1; then
  echo "validate-skill-frontmatter: python deps missing (need PyYAML and jsonschema). Try: pip install pyyaml jsonschema" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

total=0
ok=0
schema_fail=0
missing_role=0
missing_consumes=0
missing_produces=0
missing_context_rel=0

# Iterate skills in sorted, deterministic order.
mapfile -t SKILL_FILES < <(find "$SKILLS_ROOT" -mindepth 2 -maxdepth 2 -name SKILL.md -type f | sort)

for skill_md in "${SKILL_FILES[@]}"; do
  total=$((total + 1))
  rel_path="${skill_md#"$REPO_ROOT"/}"
  fm_file="$TMP_DIR/fm.yaml"

  # Extract YAML between the first two '---' lines.
  awk '
    BEGIN { in_fm=0; seen=0 }
    /^---[[:space:]]*$/ {
      if (seen == 0) { seen=1; in_fm=1; next }
      else if (in_fm == 1) { in_fm=0; exit }
    }
    in_fm == 1 { print }
  ' "$skill_md" > "$fm_file"

  if [[ ! -s "$fm_file" ]]; then
    echo "FAIL  $rel_path  (no YAML frontmatter found)"
    schema_fail=$((schema_fail + 1))
    continue
  fi

  validation_out="$TMP_DIR/out.txt"
  set +e
  python3 - "$fm_file" "$SCHEMA_PATH" >"$validation_out" 2>&1 <<'PY'
import json
import sys
import yaml
import jsonschema

fm_path, schema_path = sys.argv[1], sys.argv[2]
with open(fm_path, "r", encoding="utf-8") as f:
    raw = f.read()
try:
    data = yaml.safe_load(raw)
except yaml.YAMLError as e:
    print(f"YAML parse error: {e}")
    sys.exit(2)
if not isinstance(data, dict):
    print("frontmatter did not parse as a mapping")
    sys.exit(2)
with open(schema_path, "r", encoding="utf-8") as f:
    schema = json.load(f)
try:
    jsonschema.validate(instance=data, schema=schema)
except jsonschema.ValidationError as e:
    loc = "/".join(str(p) for p in e.absolute_path) or "<root>"
    print(f"schema violation at {loc}: {e.message}")
    sys.exit(3)
# Emit which optional hexagonal fields are missing on stdout (key=missing tag).
missing = []
for field in ("hexagonal_role", "consumes", "produces", "context_rel"):
    if field not in data:
        missing.append(field)
print("MISSING " + " ".join(missing) if missing else "MISSING")
sys.exit(0)
PY
  rc=$?
  set -e

  case "$rc" in
    0)
      ok=$((ok + 1))
      missing_line="$(grep '^MISSING' "$validation_out" || true)"
      missing_fields="${missing_line#MISSING}"
      missing_fields="$(echo "$missing_fields" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
      if [[ -n "$missing_fields" ]]; then
        echo "OK    $rel_path  (missing: $missing_fields)"
        for f in $missing_fields; do
          case "$f" in
            hexagonal_role) missing_role=$((missing_role + 1)) ;;
            consumes)       missing_consumes=$((missing_consumes + 1)) ;;
            produces)       missing_produces=$((missing_produces + 1)) ;;
            context_rel)    missing_context_rel=$((missing_context_rel + 1)) ;;
          esac
          echo "warn: $rel_path missing optional field: $f" >&2
        done
      else
        echo "OK    $rel_path"
      fi
      ;;
    2|3)
      schema_fail=$((schema_fail + 1))
      detail="$(cat "$validation_out")"
      echo "FAIL  $rel_path  ($detail)"
      ;;
    *)
      schema_fail=$((schema_fail + 1))
      echo "FAIL  $rel_path  (validator exited $rc)"
      cat "$validation_out" >&2 || true
      ;;
  esac
done

echo "validate-skill-frontmatter: ${ok}/${total} ok (${missing_role} missing hexagonal_role, ${missing_consumes} missing consumes, ${missing_produces} missing produces, ${missing_context_rel} missing context_rel)"

if [[ "$schema_fail" -gt 0 ]]; then
  echo "validate-skill-frontmatter: FAIL — ${schema_fail} schema violation(s)" >&2
  exit 1
fi

if [[ "$STRICT" -eq 1 ]]; then
  total_missing=$((missing_role + missing_consumes + missing_produces + missing_context_rel))
  if [[ "$total_missing" -gt 0 ]]; then
    echo "validate-skill-frontmatter: STRICT FAIL — ${total_missing} missing optional field(s)" >&2
    exit 1
  fi
fi

exit 0

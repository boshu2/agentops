#!/usr/bin/env bash
# register-new-codex-skill.sh — Register a new top-level codex skill across the
# 4 source-of-truth surfaces atomically.
#
# A new top-level codex skill needs entries in all 4 of:
#   1. skills-codex/.agentops-manifest.json .skills[]
#      ({name, source_skill, source_hash, generated_hash})
#   2. skills-codex/.agentops-manifest.json .codex_override_catalog.skills[]
#      ({name, treatment, wave, reason})
#   3. skills-codex-overrides/catalog.json .skills[]
#      (same shape as #2 — separate file; this is what
#       scripts/validate-codex-override-coverage.sh actually reads)
#   4. skills-codex/<name>/.agentops-generated.json marker file
#
# Pre-condition: skills/<name>/SKILL.md AND skills-codex/<name>/SKILL.md must
# already exist. This script does NOT create skill content; it registers an
# already-authored skill in the catalogs.
#
# Usage:
#   scripts/register-new-codex-skill.sh <name> --reason "<text>" \
#       [--treatment bespoke|parity_only] [--wave <wave-id>] [--tier <value>]
#
# Examples:
#   scripts/register-new-codex-skill.sh system-tuning \
#       --reason "Operator-facing system tuning workflow with codex-specific shell idioms." \
#       --treatment parity_only --wave catalog-parity --tier execution
#
# Idempotent: re-running on an already-registered skill is a no-op (per surface).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST_PATH="$REPO_ROOT/skills-codex/.agentops-manifest.json"
CATALOG_PATH="$REPO_ROOT/skills-codex-overrides/catalog.json"
SCHEMA_VALIDATOR="$REPO_ROOT/scripts/validate-skill-schema.sh"

NAME=""
TREATMENT="parity_only"
WAVE="catalog-parity"
REASON=""
TIER=""

usage() {
  cat <<EOF
Usage: scripts/register-new-codex-skill.sh <skill-name> --reason "<text>" [options]

Required:
  <skill-name>           Lowercase, hyphen-separated (e.g., "system-tuning").
  --reason <text>        Justification recorded in catalog entries.

Optional:
  --treatment <value>    bespoke | parity_only (default: parity_only).
  --wave <wave-id>       Catalog wave ID (default: catalog-parity).
  --tier <value>         Validate the SKILL.md frontmatter uses this tier.

Pre-condition:
  Both skills/<name>/SKILL.md and skills-codex/<name>/SKILL.md must exist.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --treatment)
      TREATMENT="${2:-}"; shift 2
      ;;
    --wave)
      WAVE="${2:-}"; shift 2
      ;;
    --reason)
      REASON="${2:-}"; shift 2
      ;;
    --tier)
      TIER="${2:-}"; shift 2
      ;;
    --*)
      echo "Unknown flag: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [[ -z "$NAME" ]]; then
        NAME="$1"
      else
        echo "Unexpected positional arg: $1" >&2
        exit 2
      fi
      shift
      ;;
  esac
done

if [[ -z "$NAME" ]]; then
  echo "ERROR: skill name required" >&2
  usage >&2
  exit 2
fi
if [[ -z "$REASON" ]]; then
  echo "ERROR: --reason required (recorded in catalog entries)" >&2
  usage >&2
  exit 2
fi
if [[ ! "$NAME" =~ ^[a-z][a-z0-9-]*$ ]]; then
  echo "ERROR: skill name '$NAME' must be lowercase, hyphen-separated, start with a letter" >&2
  exit 2
fi
case "$TREATMENT" in
  bespoke|parity_only) ;;
  *)
    echo "ERROR: --treatment must be 'bespoke' or 'parity_only' (got '$TREATMENT')" >&2
    exit 2
    ;;
esac

SOURCE_SKILL_DIR="$REPO_ROOT/skills/$NAME"
CODEX_SKILL_DIR="$REPO_ROOT/skills-codex/$NAME"

if [[ ! -f "$SOURCE_SKILL_DIR/SKILL.md" ]]; then
  echo "ERROR: source skill not found at $SOURCE_SKILL_DIR/SKILL.md" >&2
  echo "       Author the skill first; this script only registers an existing skill." >&2
  exit 2
fi
if [[ ! -f "$CODEX_SKILL_DIR/SKILL.md" ]]; then
  echo "ERROR: codex twin not found at $CODEX_SKILL_DIR/SKILL.md" >&2
  echo "       Author the codex twin first; this script does not generate content." >&2
  exit 2
fi
if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "ERROR: manifest not found: $MANIFEST_PATH" >&2
  exit 1
fi
if [[ ! -f "$CATALOG_PATH" ]]; then
  echo "ERROR: catalog not found: $CATALOG_PATH" >&2
  exit 1
fi

# Tier validation (optional but recommended).
if [[ -n "$TIER" ]]; then
  if [[ ! -x "$SCHEMA_VALIDATOR" ]]; then
    echo "WARN: schema validator not executable at $SCHEMA_VALIDATOR; skipping tier check" >&2
  else
    # Allowed tiers come from the schema validator. Source of truth is its enum.
    ALLOWED_TIERS="$(grep -oE 'judgment|execution|library|session|product|contribute|meta|background|orchestration|cross-vendor|knowledge' "$SCHEMA_VALIDATOR" | sort -u | tr '\n' '|' | sed 's/|$//')"
    if ! echo "$TIER" | grep -qE "^($ALLOWED_TIERS)$"; then
      echo "ERROR: --tier '$TIER' not in schema enum ($ALLOWED_TIERS)" >&2
      exit 2
    fi
    # Verify the SKILL.md actually has this tier.
    ACTUAL_TIER="$(awk '/^metadata:/{m=1; next} m && /^[[:space:]]+tier:/{print $2; exit}' "$SOURCE_SKILL_DIR/SKILL.md" | tr -d '"' | tr -d "'")"
    if [[ -n "$ACTUAL_TIER" && "$ACTUAL_TIER" != "$TIER" ]]; then
      echo "ERROR: --tier '$TIER' does not match SKILL.md frontmatter (tier: $ACTUAL_TIER)" >&2
      exit 2
    fi
  fi
fi

export NAME TREATMENT WAVE REASON MANIFEST_PATH CATALOG_PATH CODEX_SKILL_DIR SOURCE_SKILL_DIR

python3 - <<'PY'
import hashlib
import json
import os
import pathlib
import sys

name = os.environ["NAME"]
treatment = os.environ["TREATMENT"]
wave = os.environ["WAVE"]
reason = os.environ["REASON"]
manifest_path = pathlib.Path(os.environ["MANIFEST_PATH"])
catalog_path = pathlib.Path(os.environ["CATALOG_PATH"])
codex_skill_dir = pathlib.Path(os.environ["CODEX_SKILL_DIR"])
source_skill_dir = pathlib.Path(os.environ["SOURCE_SKILL_DIR"])

marker_name = ".agentops-generated.json"


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def hash_tree(root: pathlib.Path) -> str:
    rows = []
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        if path.name in {".agentops-manifest.json", marker_name, ".DS_Store"}:
            continue
        if "__pycache__" in path.parts:
            continue
        if path.suffix == ".pyc":
            continue
        rel = path.relative_to(root).as_posix()
        rows.append(f"{rel}\t{sha256_bytes(path.read_bytes())}\n")
    return sha256_bytes("".join(rows).encode("utf-8"))


source_hash = hash_tree(source_skill_dir)
generated_hash = hash_tree(codex_skill_dir)

actions = []

# Surface 1: manifest .skills[]
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
manifest_skills = manifest.setdefault("skills", [])
existing_idx = next((i for i, e in enumerate(manifest_skills) if e.get("name") == name), None)
manifest_entry = {
    "name": name,
    "source_skill": f"skills/{name}",
    "source_hash": source_hash,
    "generated_hash": generated_hash,
}
if existing_idx is None:
    manifest_skills.append(manifest_entry)
    manifest_skills.sort(key=lambda e: e.get("name", ""))
    actions.append("manifest.skills[]: added")
else:
    actions.append(f"manifest.skills[]: already present (idx {existing_idx}); skipped")

# Surface 2: manifest .codex_override_catalog.skills[]
catalog_in_manifest = manifest.setdefault("codex_override_catalog", {})
catalog_in_manifest_skills = catalog_in_manifest.setdefault("skills", [])
existing_idx = next((i for i, e in enumerate(catalog_in_manifest_skills) if e.get("name") == name), None)
catalog_entry = {
    "name": name,
    "treatment": treatment,
    "wave": wave,
    "reason": reason,
}
if existing_idx is None:
    catalog_in_manifest_skills.append(catalog_entry)
    actions.append("manifest.codex_override_catalog.skills[]: added")
else:
    actions.append(f"manifest.codex_override_catalog.skills[]: already present (idx {existing_idx}); skipped")

# Recompute the embedded catalog hash for parity with the standalone catalog file.
catalog_for_hash = json.dumps(
    {k: v for k, v in catalog_in_manifest.items() if k != "skills"} | {"skills": catalog_in_manifest_skills},
    sort_keys=True,
).encode("utf-8")
manifest["codex_override_catalog_hash"] = sha256_bytes(catalog_for_hash)

manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")

# Surface 3: skills-codex-overrides/catalog.json .skills[]
catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
catalog_skills = catalog.setdefault("skills", [])
existing_idx = next((i for i, e in enumerate(catalog_skills) if e.get("name") == name), None)
if existing_idx is None:
    catalog_skills.append(catalog_entry)
    actions.append("catalog.json .skills[]: added")
else:
    actions.append(f"catalog.json .skills[]: already present (idx {existing_idx}); skipped")

catalog_path.write_text(json.dumps(catalog, indent=2) + "\n", encoding="utf-8")

# Surface 4: per-skill marker file
marker_path = codex_skill_dir / marker_name
marker_payload = {
    "generator": "manual-maintained",
    "source_skill": f"skills/{name}",
    "layout": "modular",
    "source_hash": source_hash,
    "generated_hash": generated_hash,
}
if marker_path.exists():
    actions.append(f"{marker_name}: already present; rewriting hashes")
else:
    actions.append(f"{marker_name}: created")
marker_path.write_text(json.dumps(marker_payload, indent=2) + "\n", encoding="utf-8")

print(f"Registered '{name}' in 4 codex catalog surfaces:")
for a in actions:
    print(f"  - {a}")
print(f"  source_hash    = {source_hash}")
print(f"  generated_hash = {generated_hash}")
print()
print("Next: run scripts/validate-codex-override-coverage.sh to verify registration.")
PY

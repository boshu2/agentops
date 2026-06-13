#!/usr/bin/env bash
# validate-skill-disposition-schema.sh — artifact-classification schema gate (ag-4akl8 S0).
#
# Enforces the v4 ADDITIVE schema on docs/contracts/skill-dispositions.yaml.
# Every ACTIVE `- skill:` row and every `workflows:` entry must carry the new
# artifact-classification fields with valid values:
#
#   kind             closed enum: skill | workflow | loop
#   runtime_targets  non-empty list subset of [claude, codex]
#   parity_policy    closed enum: required | exempt
#   capability_class closed enum: tracking | validation | planning | corpus |
#                    authoring | orchestration | execution | delivery | safety | docs
#   path             repo-relative path (non-empty string)
#   aliases          list (may be empty)
#   supersedes       id string or null
#
# A row with an unknown enum value, or a missing required field, is reported by
# NAME so the offending row is obvious (the additive-schema acceptance contract:
# "it fails and names the offending row").
#
# The `historical:` section is NOT validated (terminal rows keep their original
# shape verbatim — quorum decision: history stays byte-for-byte).
#
# Exit codes:
#   0 = schema OK
#   1 = at least one row/entry violates the schema
#   2 = usage / missing input
#
# DISP_YAML overrides the ledger path (used by tests with throwaway fixtures).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DISP_YAML="${DISP_YAML:-${REPO_ROOT}/docs/contracts/skill-dispositions.yaml}"

if [[ ! -f "${DISP_YAML}" ]]; then
  echo "ERROR: ledger not found: ${DISP_YAML}" >&2
  exit 2
fi

export DISP_YAML

exec python3 - <<'PY'
import os
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed; install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

DISP_YAML = Path(os.environ["DISP_YAML"])

KIND_ENUM = {"skill", "workflow", "loop"}
PARITY_ENUM = {"required", "exempt"}
RUNTIME_VALUES = {"claude", "codex"}
CAPABILITY_ENUM = {
    "tracking", "validation", "planning", "corpus", "authoring",
    "orchestration", "execution", "delivery", "safety", "docs",
}
REQUIRED_FIELDS = (
    "kind", "runtime_targets", "parity_policy", "capability_class",
    "path", "aliases", "supersedes",
)

data = yaml.safe_load(DISP_YAML.read_text(encoding="utf-8")) or {}

failures = []  # (artifact_id, message)


def validate_entry(artifact_id, entry):
    """Validate one classified artifact (skill row or workflow entry)."""
    for field in REQUIRED_FIELDS:
        if field not in entry:
            failures.append((artifact_id, f"missing required field `{field}`"))

    kind = entry.get("kind")
    if kind is not None and kind not in KIND_ENUM:
        failures.append((
            artifact_id,
            f"kind={kind!r} is not in the closed enum {sorted(KIND_ENUM)}",
        ))

    parity = entry.get("parity_policy")
    if parity is not None and parity not in PARITY_ENUM:
        failures.append((
            artifact_id,
            f"parity_policy={parity!r} is not in the closed enum {sorted(PARITY_ENUM)}",
        ))

    cap = entry.get("capability_class")
    if cap is not None and cap not in CAPABILITY_ENUM:
        failures.append((
            artifact_id,
            f"capability_class={cap!r} is not in the closed enum {sorted(CAPABILITY_ENUM)}",
        ))

    targets = entry.get("runtime_targets")
    if targets is not None:
        if not isinstance(targets, list) or not targets:
            failures.append((artifact_id, "runtime_targets must be a non-empty list"))
        else:
            bad = [t for t in targets if t not in RUNTIME_VALUES]
            if bad:
                failures.append((
                    artifact_id,
                    f"runtime_targets has unknown value(s) {bad}; allowed: {sorted(RUNTIME_VALUES)}",
                ))

    aliases = entry.get("aliases")
    if aliases is not None and not isinstance(aliases, list):
        failures.append((artifact_id, "aliases must be a list"))


# Active skill rows: `dispositions:` is a list of single-key {skill: name, ...} maps.
for row in (data.get("dispositions") or []):
    if not isinstance(row, dict):
        continue
    artifact_id = row.get("skill", "<unknown>")
    validate_entry(f"skill:{artifact_id}", row)

# Workflows: top-level `workflows:` mapping of id -> entry.
for wf_id, entry in (data.get("workflows") or {}).items():
    if not isinstance(entry, dict):
        continue
    validate_entry(f"workflow:{wf_id}", entry)

if failures:
    print(f"FAIL: {len(failures)} schema violation(s) in {DISP_YAML.name}:", file=sys.stderr)
    for artifact_id, msg in failures:
        print(f"  [{artifact_id}] {msg}", file=sys.stderr)
    sys.exit(1)

n_skills = len(data.get("dispositions") or [])
n_workflows = len(data.get("workflows") or {})
print(f"OK: artifact-classification schema valid ({n_skills} skill rows, {n_workflows} workflows)")
sys.exit(0)
PY

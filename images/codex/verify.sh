#!/usr/bin/env bash
# verify.sh - Codex image bundle integrity check (cp-eoxc / cp-gqu Unit-4).
#
# For each CORE slug in images/codex/manifest.json, confirm its skills-codex/<slug>/
# twin is present and complete: SKILL.md AND prompt.md AND .agentops-generated.json
# all exist. Missing or incomplete twins are FLAGGED (non-zero exit), never silently
# passed. The corpus is post-distillation, so all 61 CORE twins should exist.
#
# This is presence/packaging verification ONLY. Hash-consistency (twin in sync with
# source) is the separate, authoritative gate: scripts/regen-codex-hashes.sh --check,
# which this script also runs as the final step.
#
# Usage: bash images/codex/verify.sh   (run from the agentops repo root or anywhere)
# Exit:  0 = all CORE twins present and complete + hashes in sync; non-zero otherwise.

set -euo pipefail

# Resolve the agentops repo root from this script's location (images/codex/verify.sh).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST="${SCRIPT_DIR}/manifest.json"

cd "${REPO_ROOT}"

if [ ! -f "${MANIFEST}" ]; then
  echo "FATAL: manifest not found: ${MANIFEST}" >&2
  exit 2
fi

# Extract the CORE slug list from the manifest (no jq dependency; use python3).
mapfile -t CORE_SLUGS < <(python3 -c '
import json, sys
m = json.load(open(sys.argv[1]))
for s in m["core_skills"]:
    print(s["slug"])
' "${MANIFEST}")

EXPECTED="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["core_count"])' "${MANIFEST}")"

echo "Codex image bundle verify - CORE twins in skills-codex/"
echo "  repo root : ${REPO_ROOT}"
echo "  manifest  : ${MANIFEST}"
echo "  expected  : ${EXPECTED} CORE slugs"
echo

missing=0
checked=0
for slug in "${CORE_SLUGS[@]}"; do
  [ -z "${slug}" ] && continue
  checked=$((checked + 1))
  twin="skills-codex/${slug}"
  for f in SKILL.md prompt.md .agentops-generated.json; do
    if [ ! -f "${twin}/${f}" ]; then
      echo "MISSING/STALE: ${twin}/${f}  (CORE slug '${slug}' twin incomplete)" >&2
      missing=$((missing + 1))
    fi
  done
done

echo "Checked ${checked} CORE slugs."

if [ "${checked}" -ne "${EXPECTED}" ]; then
  echo "FAIL: checked ${checked} slugs but manifest declares ${EXPECTED}." >&2
  exit 1
fi

if [ "${missing}" -ne 0 ]; then
  echo "FAIL: ${missing} missing/incomplete twin file(s)." >&2
  exit 1
fi

echo "OK: all ${checked} CORE twins present (SKILL.md + prompt.md + .agentops-generated.json)."
echo

# Authoritative sync gate: twins hash-consistent with their source skills.
echo "Running drift gate: scripts/regen-codex-hashes.sh --check"
if bash scripts/regen-codex-hashes.sh --check; then
  echo "OK: codex hashes in sync (no drift)."
else
  echo "FAIL: regen-codex-hashes.sh --check reported drift." >&2
  exit 1
fi

echo
echo "PASS: Codex image bundle verified (${checked} CORE twins present + hashes in sync)."

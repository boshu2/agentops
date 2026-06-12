#!/usr/bin/env bash
# verify.sh - Codex image bundle integrity check (cp-eoxc / cp-gqu Unit-4).
#
# For each CORE slug in images/codex/manifest.json, confirm its skills-codex/<slug>/
# twin is present and complete: SKILL.md AND prompt.md AND .agentops-generated.json
# all exist. Missing or incomplete twins are FLAGGED (non-zero exit), never silently
# passed. The corpus is post-distillation, so all manifest-listed CORE twins should exist.
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

# Extract the CORE manifest rows (no jq dependency; use python3).
mapfile -t CORE_ROWS < <(python3 -c '
import json, sys
m = json.load(open(sys.argv[1]))
for s in m["core_skills"]:
    files = s["twin_files"]
    print("\t".join([
        s["slug"],
        s["twin_path"],
        files["skill"],
        files["prompt"],
        files["drift_marker"],
    ]))
' "${MANIFEST}")

EXPECTED="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["core_count"])' "${MANIFEST}")"

echo "Codex image bundle verify - CORE twins in skills-codex/"
echo "  repo root : ${REPO_ROOT}"
echo "  manifest  : ${MANIFEST}"
echo "  expected  : ${EXPECTED} CORE slugs"
echo

missing=0
checked=0
for row in "${CORE_ROWS[@]}"; do
  IFS=$'\t' read -r slug twin_path skill_file prompt_file drift_file <<<"${row}"
  [ -z "${slug}" ] && continue
  checked=$((checked + 1))
  expected_twin_path="skills-codex/${slug}/"
  if [[ "${twin_path}" != "${expected_twin_path}" ]]; then
    echo "MISSING/STALE: ${slug} twin_path is '${twin_path}', want '${expected_twin_path}'" >&2
    missing=$((missing + 1))
  fi
  for file in "${skill_file}" "${prompt_file}" "${drift_file}"; do
    if [[ "${file}" != "${expected_twin_path}"* ]]; then
      echo "MISSING/STALE: ${file}  (CORE slug '${slug}' twin_files path outside '${expected_twin_path}')" >&2
      missing=$((missing + 1))
    elif [ ! -f "${file}" ]; then
      echo "MISSING/STALE: ${file}  (CORE slug '${slug}' twin incomplete)" >&2
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

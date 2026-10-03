#!/usr/bin/env bash
# verify.sh — confirm the Codex image is the canonical skills/ tree.
#
# Codex loads skills/ directly: the plugin manifest points at ./skills and
# `ao skills link` links the same directories. This script reads the
# metadata-derived slug list from images/codex/manifest.json and asserts that
#   1. every declared slug resolves to skills/<slug>/SKILL.md at its declared path,
#   2. .codex-plugin/plugin.json ships ./skills, and
#   3. the tree passes the Codex loader and invocation-policy checks
#      (scripts/validate-codex-api-conformance.sh).
#
# Usage: bash images/codex/verify.sh   (run from the agentops repo root or anywhere)
# Exit:  0 = all three hold; non-zero otherwise.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST="${SCRIPT_DIR}/manifest.json"
PLUGIN_MANIFEST="${REPO_ROOT}/.codex-plugin/plugin.json"

cd "${REPO_ROOT}"

if [ ! -f "${MANIFEST}" ]; then
  echo "FATAL: manifest not found: ${MANIFEST}" >&2
  exit 2
fi

# Extract the generated manifest rows (no jq dependency; use python3).
mapfile -t ROWS < <(python3 -c '
import json, sys
m = json.load(open(sys.argv[1]))
for s in m["skills"]:
    print("\t".join([s["slug"], s["path"]]))
' "${MANIFEST}")

EXPECTED="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["skill_count"])' "${MANIFEST}")"

echo "Codex image verify - skills loaded from skills/"
echo "  repo root : ${REPO_ROOT}"
echo "  manifest  : ${MANIFEST}"
echo "  expected  : ${EXPECTED} current slugs"
echo

missing=0
checked=0
for row in "${ROWS[@]}"; do
  IFS=$'\t' read -r slug path <<<"${row}"
  [ -z "${slug}" ] && continue
  checked=$((checked + 1))
  expected_path="skills/${slug}/"
  if [[ "${path}" != "${expected_path}" ]]; then
    echo "MISSING/STALE: ${slug} path is '${path}', want '${expected_path}'" >&2
    missing=$((missing + 1))
  fi
  if [ ! -f "${expected_path}SKILL.md" ]; then
    echo "MISSING/STALE: ${expected_path}SKILL.md" >&2
    missing=$((missing + 1))
  fi
done

echo "Checked ${checked} current slugs."

if [ "${checked}" -ne "${EXPECTED}" ]; then
  echo "FAIL: checked ${checked} slugs but manifest declares ${EXPECTED}." >&2
  exit 1
fi

if [ "${missing}" -ne 0 ]; then
  echo "FAIL: ${missing} missing skill package(s)." >&2
  exit 1
fi

echo "OK: all ${checked} declared skills present in skills/."

if [ ! -f "${PLUGIN_MANIFEST}" ]; then
  echo "FAIL: Codex plugin manifest not found: ${PLUGIN_MANIFEST}" >&2
  exit 1
fi
plugin_skills="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("skills", ""))' "${PLUGIN_MANIFEST}")"
if [ "${plugin_skills}" != "./skills" ]; then
  echo "FAIL: .codex-plugin/plugin.json ships '${plugin_skills}', want './skills'" >&2
  exit 1
fi
echo "OK: .codex-plugin/plugin.json ships ./skills."
echo

echo "Running: scripts/validate-codex-api-conformance.sh"
if bash scripts/validate-codex-api-conformance.sh; then
  echo "OK: skills/ is loadable by Codex."
else
  echo "FAIL: validate-codex-api-conformance.sh reported findings." >&2
  exit 1
fi

echo
echo "PASS: Codex image verified (${checked} skills loaded from skills/)."

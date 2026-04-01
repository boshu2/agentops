#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHECKED_IN_ROOT="${REPO_ROOT}/skills-codex"
MANIFEST_SCRIPT="${REPO_ROOT}/scripts/validate-codex-generated-manifest.sh"
ARTIFACT_SCRIPT="${REPO_ROOT}/scripts/validate-codex-generated-artifacts.sh"

[[ -x "${MANIFEST_SCRIPT}" ]] || {
  echo "Missing or non-executable manifest validator: ${MANIFEST_SCRIPT}" >&2
  exit 1
}

[[ -x "${ARTIFACT_SCRIPT}" ]] || {
  echo "Missing or non-executable artifact validator: ${ARTIFACT_SCRIPT}" >&2
  exit 1
}

[[ -d "${CHECKED_IN_ROOT}" ]] || {
  echo "Missing checked-in Codex skills directory: ${CHECKED_IN_ROOT}" >&2
  exit 1
}

bash "${MANIFEST_SCRIPT}" "${CHECKED_IN_ROOT}" >/dev/null
bash "${ARTIFACT_SCRIPT}" "${REPO_ROOT}" --scope head >/dev/null

skill_count="$(find "${CHECKED_IN_ROOT}" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
echo "Codex skill parity check passed: ${skill_count} skill(s) in the checked-in Codex bundle."

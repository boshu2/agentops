#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SYNC_SCRIPT="${REPO_ROOT}/scripts/sync-codex-native-skills.sh"
CHECKED_IN_ROOT="${REPO_ROOT}/skills-codex"

[[ -x "${SYNC_SCRIPT}" ]] || {
  echo "Missing or non-executable sync script: ${SYNC_SCRIPT}" >&2
  exit 1
}

[[ -d "${CHECKED_IN_ROOT}" ]] || {
  echo "Missing checked-in Codex skills directory: ${CHECKED_IN_ROOT}" >&2
  exit 1
}

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

generated_root="${tmpdir}/skills-codex"

bash "${SYNC_SCRIPT}" --out "${generated_root}" >/dev/null

if ! diff_output="$(diff -rq "${CHECKED_IN_ROOT}" "${generated_root}" 2>&1)"; then
  echo "Codex skill parity check failed: checked-in skills-codex differs from regenerated output." >&2
  echo "${diff_output}" | sed -n '1,120p' >&2
  exit 1
fi

skill_count="$(find "${CHECKED_IN_ROOT}" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
echo "Codex skill parity check passed: ${skill_count} skill(s)."

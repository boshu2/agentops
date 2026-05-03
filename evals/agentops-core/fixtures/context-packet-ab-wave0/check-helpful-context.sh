#!/usr/bin/env bash
set -euo pipefail

root="${AO_AGENTS_DIR:-}"
variant="${AO_CONTEXT_VARIANT:-unknown}"
helpful=false
helpful_path="${root}/learnings/wave0-helpful.md"

if [[ -f "${helpful_path}" ]] && grep -q "WAVE0_HELPFUL" "${helpful_path}"; then
  helpful=true
fi

hooks_disabled=false
if [[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]]; then
  hooks_disabled=true
fi

printf '{"variant":"%s","helpful_context":%s,"hooks_disabled":%s}\n' "${variant}" "${helpful}" "${hooks_disabled}"

if [[ "${variant}" == "context_on" && "${helpful}" == "true" && "${hooks_disabled}" == "false" ]]; then
  exit 0
fi

exit 1

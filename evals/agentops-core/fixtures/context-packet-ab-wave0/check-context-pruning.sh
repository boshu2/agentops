#!/usr/bin/env bash
set -euo pipefail

root="${AO_AGENTS_DIR:-}"
stale_path="${root}/learnings/wave0-stale.md"
toxic_path="${root}/learnings/wave0-toxic.md"
irrelevant_path="${root}/learnings/wave0-irrelevant.md"

has_marker() {
  local path="$1"
  local marker="$2"
  [[ -f "${path}" ]] && grep -q "${marker}" "${path}"
}

stale_sentinel_ignored=true
if has_marker "${stale_path}" "APPLY_ME"; then
  stale_sentinel_ignored=false
fi

toxic_sentinel_ignored=true
if has_marker "${toxic_path}" "APPLY_ME"; then
  toxic_sentinel_ignored=false
fi

irrelevant_context_ignored=true
if has_marker "${irrelevant_path}" "REQUIRE_USE"; then
  irrelevant_context_ignored=false
fi

curated_only=false
if [[ "${stale_sentinel_ignored}" == "true" && "${toxic_sentinel_ignored}" == "true" && "${irrelevant_context_ignored}" == "true" ]]; then
  curated_only=true
fi

hooks_disabled=false
if [[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]]; then
  hooks_disabled=true
fi

printf '{"curated_only":%s,"stale_sentinel_ignored":%s,"toxic_sentinel_ignored":%s,"irrelevant_context_ignored":%s,"hooks_disabled":%s}\n' \
  "${curated_only}" \
  "${stale_sentinel_ignored}" \
  "${toxic_sentinel_ignored}" \
  "${irrelevant_context_ignored}" \
  "${hooks_disabled}"

if [[ "${curated_only}" == "true" && "${hooks_disabled}" == "false" ]]; then
  exit 0
fi

exit 1

#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: bash skills-codex/crank/scripts/validate-wave-checkpoint.sh <checkpoint-json> [repo-root]" >&2
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

checkpoint="${1:-}"
repo_root="${2:-}"

if [[ -z "$checkpoint" ]]; then
  usage
  exit 1
fi

[[ -f "$checkpoint" ]] || fail "checkpoint not found: $checkpoint"
command -v jq >/dev/null 2>&1 || fail "jq required"

if [[ -z "$repo_root" ]]; then
  repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

for field in \
  schema_version \
  wave \
  timestamp \
  tasks_completed \
  tasks_failed \
  files_changed \
  git_sha \
  acceptance_verdict \
  commit_strategy \
  mutations_this_wave \
  total_mutations \
  mutation_budget; do
  jq -e --arg field "$field" 'has($field)' "$checkpoint" >/dev/null \
    || fail "checkpoint missing required field: $field"
done

jq -e '
  (.schema_version == 1) and
  (.wave | type == "number" and . >= 1 and . == floor) and
  (.timestamp | type == "string" and length > 0) and
  (.tasks_completed | type == "array") and
  (.tasks_failed | type == "array") and
  (.files_changed | type == "array") and
  all(.tasks_completed[]; type == "string") and
  all(.tasks_failed[]; type == "string") and
  all(.files_changed[]; type == "string") and
  (.git_sha | type == "string" and length > 0) and
  (.acceptance_verdict as $v | ["PASS", "WARN", "FAIL"] | index($v) != null) and
  (.commit_strategy | type == "string" and length > 0) and
  (.mutations_this_wave | type == "number" and . >= 0 and . == floor) and
  (.total_mutations | type == "number" and . >= 0 and . == floor) and
  (.mutation_budget | type == "object")
' "$checkpoint" >/dev/null || fail "checkpoint has invalid field types or values"

timestamp_epoch="$(jq -er '.timestamp | fromdateiso8601' "$checkpoint" 2>/dev/null)" \
  || fail "timestamp is not valid ISO-8601 UTC: $(jq -r '.timestamp' "$checkpoint" 2>/dev/null || echo '<unreadable>')"
now_epoch="$(date -u +%s)"
if (( timestamp_epoch > now_epoch + 300 )); then
  fail "timestamp is more than 5 minutes in the future: $(jq -r '.timestamp' "$checkpoint")"
fi

git_sha="$(jq -er '.git_sha' "$checkpoint")"
git -C "$repo_root" cat-file -e "${git_sha}^{commit}" 2>/dev/null \
  || fail "git_sha does not resolve to a commit in $repo_root: $git_sha"

echo "PASS: crank wave checkpoint valid: $checkpoint"

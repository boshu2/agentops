#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <codebase-recon.json>" >&2
  exit 2
fi

artifact="$1"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"
artifact_dir="$(cd "$(dirname "$artifact")" && pwd)"

jq -e '
  def text: type == "string" and length > 0;
  .schema_version == "codebase-recon.v1"
  and (.mode == "baseline" or .mode == "delta")
  and (.commit | text)
  and (.flows
    | type == "array"
    and all(.[];
      (.entry | text)
      and (.domain | text)
      and (.integration | text)
      and (.tests | text)))
  and (.claims
    | type == "array"
    and all(.[];
      (.kind == "fact" or .kind == "inference" or .kind == "unknown")
      and (.text | text)
      and (.confidence == "high" or .confidence == "medium" or .confidence == "low")
      and (.evidence | type == "array" and all(.[]; text))
      and (if .kind == "unknown" then true else (.evidence | length > 0) end)))
  and (.coverage | type == "object")
  and (.coverage.inspected | type == "array" and length > 0 and all(.[]; text))
  and (.coverage.uninspected | type == "array" and length > 0 and all(.[]; text))
  and (
    if .mode == "baseline" then
      (.flows | length > 0)
      and ((has("prior_recon") | not) or .prior_recon == "" or .prior_recon == null)
    else
      (.prior_recon | text)
      and .baseline_verified == true
      and (.delta
        | type == "array" and length > 0
        and all(.[]; (.path | text) and (.change | text)))
    end
  )
' "$artifact" >/dev/null || {
  echo "invalid codebase-recon.v1 artifact: $artifact" >&2
  exit 1
}

resolve_evidence() {
  local candidate="$1"
  if [[ "$candidate" =~ ^(.+):[0-9]+$ ]]; then
    candidate="${BASH_REMATCH[1]}"
  fi
  if [[ "$candidate" = /* ]]; then
    [[ -e "$candidate" ]]
  else
    [[ -e "$repo_root/$candidate" || -e "$artifact_dir/$candidate" ]]
  fi
}

while IFS= read -r evidence; do
  if ! resolve_evidence "$evidence"; then
    echo "missing claim evidence: $evidence" >&2
    exit 1
  fi
done < <(jq -r '.claims[] | select(.kind == "fact" or .kind == "inference") | .evidence[]' "$artifact")

if [[ "$(jq -r '.mode' "$artifact")" == "delta" ]]; then
  prior="$(jq -r '.prior_recon' "$artifact")"
  if ! resolve_evidence "$prior"; then
    echo "missing prior recon pack: $prior" >&2
    exit 1
  fi
fi

echo "valid codebase-recon.v1: $artifact"

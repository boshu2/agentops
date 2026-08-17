#!/usr/bin/env bash
set -euo pipefail

die() { printf 'agent-native-dispatch: %s\n' "$*" >&2; exit 2; }
usage() { printf '%s\n' 'usage: dispatch.sh --adapter codex|ntm --role ROLE --model-profile NAME --context-id ID --workspace ABS --packet ABS --output ABS [--deadline SEC] [--approve TOKEN]' >&2; }
sha256_file() { if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }

adapter=''
role=''
model=''
context_id=''
workspace=''
packet=''
output=''
deadline=600
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --adapter) [[ $# -ge 2 ]] || die '--adapter needs a value'; adapter=$2; shift 2 ;;
    --role) [[ $# -ge 2 ]] || die '--role needs a value'; role=$2; shift 2 ;;
    --model-profile) [[ $# -ge 2 ]] || die '--model-profile needs a value'; model=$2; shift 2 ;;
    --context-id) [[ $# -ge 2 ]] || die '--context-id needs a value'; context_id=$2; shift 2 ;;
    --workspace) [[ $# -ge 2 ]] || die '--workspace needs a value'; workspace=$2; shift 2 ;;
    --packet) [[ $# -ge 2 ]] || die '--packet needs a value'; packet=$2; shift 2 ;;
    --output) [[ $# -ge 2 ]] || die '--output needs a value'; output=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

case "$role" in orchestrator|implementer|validator|scribe|judge|perspective) ;; *) die 'unsupported role' ;; esac
case "$adapter" in
  codex) ;;
  ntm) die 'NTM model dispatch is blocked: the bounded pane surface cannot yet attest the pane model identity; use NTM only as declared transport via the ntm skill' ;;
  *) die 'adapter must be codex or ntm' ;;
esac
[[ "$model" =~ ^[A-Za-z0-9][A-Za-z0-9._:/+-]{0,127}$ ]] || die 'model profile contains unsupported characters'
[[ "$context_id" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$ ]] || die 'context id contains unsupported characters'
[[ "$workspace" == /* && -d "$workspace" && ! -L "$workspace" ]] || die 'workspace must be an existing absolute non-symlink directory'
workspace=$(cd "$workspace" && pwd -P)
[[ "$packet" == /* && -f "$packet" && ! -L "$packet" && -s "$packet" ]] || die 'packet must be an absolute nonempty regular non-symlink file'
packet_bytes=$(wc -c <"$packet" | tr -d ' ')
(( packet_bytes <= 1048576 )) || die 'packet exceeds 1 MiB'
[[ "$output" == /* && ! -L "$output" && ! -e "$output" ]] || die 'output must be an unused absolute non-symlink path'
output_parent=$(dirname "$output")
[[ -d "$output_parent" && ! -L "$output_parent" ]] || die 'output parent must be an existing non-symlink directory'
output_parent=$(cd "$output_parent" && pwd -P)
output="$output_parent/$(basename "$output")"
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 1 && deadline <= 3600 )); then die 'deadline must be 1-3600 seconds'; fi
digest=$(sha256_file "$packet")
expected="agent-native:dispatch:$adapter:$role:$context_id:$workspace:$model:$digest"
[[ "$approval" == "$expected" ]] || die "exact approval required: $expected"

umask 077
packet_copy=$(mktemp "$output_parent/.agent-native-packet.XXXXXX")
trap 'rm -f "$packet_copy"' EXIT
cp "$packet" "$packet_copy"
[[ "$(sha256_file "$packet_copy")" == "$digest" && "$(sha256_file "$packet")" == "$digest" ]] || die 'packet changed while it was being frozen'

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
codex_runner="$script_dir/../../codex-exec/scripts/run.sh"
[[ -x "$codex_runner" ]] || die 'package-owned codex-exec surface is unavailable'
sandbox=read-only
args=(--workspace "$workspace" --prompt "$packet_copy" --output "$output" --deadline "$deadline" --sandbox read-only --model "$model")
if [[ "$role" == implementer ]]; then
  sandbox='workspace-write'
  args=(--workspace "$workspace" --prompt "$packet_copy" --output "$output" --deadline "$deadline" --sandbox workspace-write --model "$model" --approve "codex-exec:workspace-write:$workspace:$model:$digest")
fi
"$codex_runner" "${args[@]}"
rm -f "$packet_copy"
trap - EXIT
printf 'agent-native-dispatch: adapter=codex role=%s requested_model=%s context_id=%s sandbox=%s packet_sha256=%s artifact=%s actual_provider_model_not_cryptographically_attested=true\n' "$role" "$model" "$context_id" "$sandbox" "$digest" "$output" >&2

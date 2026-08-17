#!/usr/bin/env bash
set -euo pipefail

die() { printf 'agy-native: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' 'usage: run.sh --role validator|judge|perspective|implementer --model NAME --workspace ABS --packet FILE --output ABS [--deadline 1..1800] [--context-id ID] [--approve TOKEN]' >&2
}
sha256_file() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

role=''
model=''
workspace=''
packet=''
output=''
deadline=300
context_id=''
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --role) [[ $# -ge 2 ]] || die '--role needs a value'; role=$2; shift 2 ;;
    --model) [[ $# -ge 2 ]] || die '--model needs a value'; model=$2; shift 2 ;;
    --workspace) [[ $# -ge 2 ]] || die '--workspace needs a value'; workspace=$2; shift 2 ;;
    --packet) [[ $# -ge 2 ]] || die '--packet needs a value'; packet=$2; shift 2 ;;
    --output) [[ $# -ge 2 ]] || die '--output needs a value'; output=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    --context-id) [[ $# -ge 2 ]] || die '--context-id needs a value'; context_id=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

case "$role" in validator|judge|perspective|implementer) ;; *) die 'unsupported role' ;; esac
[[ "$model" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$ ]] || die 'model contains unsupported characters'
[[ -n "$context_id" && "$context_id" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$ ]] || die 'a bounded context id is required'
[[ "$workspace" == /* && -d "$workspace" && ! -L "$workspace" ]] || die 'workspace must be an existing absolute non-symlink directory'
workspace=$(cd "$workspace" && pwd -P)
[[ "$workspace" != / ]] || die 'workspace may not be filesystem root'
[[ "$packet" == /* && -f "$packet" && ! -L "$packet" && -s "$packet" ]] || die 'packet must be an absolute nonempty regular non-symlink file'
bytes=$(wc -c <"$packet" | tr -d ' ')
(( bytes <= 65536 )) || die 'packet exceeds the 64 KiB argv-safe bound'
[[ "$output" == /* && ! -L "$output" ]] || die 'output must be absolute and not a symlink'
[[ ! -e "$output" ]] || die 'output already exists; choose a new artifact path'
output_parent=$(dirname "$output")
[[ -d "$output_parent" && ! -L "$output_parent" ]] || die 'output parent must be an existing non-symlink directory'
output_parent=$(cd "$output_parent" && pwd -P)
output="$output_parent/$(basename "$output")"
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 1 && deadline <= 1800 )); then die 'deadline must be 1-1800 seconds'; fi
digest=$(sha256_file "$packet")
mode=plan
if [[ "$role" == implementer ]]; then
  expected="agy:workspace-write:$workspace:$model:$digest"
  [[ "$approval" == "$expected" ]] || die "implementer requires exact approval token: $expected"
  mode=accept-edits
fi

agy_bin=${AGY_BIN:-agy}
if [[ "$agy_bin" == */* ]]; then [[ -x "$agy_bin" ]] || die "agy binary is not executable: $agy_bin"; else agy_bin=$(command -v "$agy_bin") || die 'agy unavailable'; fi
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
launcher="$script_dir/process-group-run.pl"
[[ -x "$launcher" ]] || die 'process-group launcher is missing or not executable'
command -v perl >/dev/null || die 'perl with POSIX::setsid is required for process-group cleanup'
version=$($launcher 10 /dev/null -- "$agy_bin" --version)
[[ "$version" == 1.1.13 ]] || die "agy 1.1.13 is required, got: $version"
help=$($launcher 10 /dev/null -- "$agy_bin" --help)
for capability in '--print' '--print-timeout' '--sandbox' '--mode' '--model' '--disable-slash-commands'; do
  [[ "$help" == *"$capability"* ]] || die "agy lacks required capability: $capability"
done
models=$($launcher 15 /dev/null -- "$agy_bin" models)
printf '%s\n' "$models" | grep -Fx -- "$model" >/dev/null || die "agy model is unavailable: $model"

umask 077
packet_copy=$(mktemp "$output_parent/.agy-packet.XXXXXX")
partial=$(mktemp "$output_parent/.agy-output.XXXXXX")
trap 'rm -f "$partial" "$packet_copy"' EXIT
cp "$packet" "$packet_copy"
[[ "$(sha256_file "$packet_copy")" == "$digest" && "$(sha256_file "$packet")" == "$digest" ]] || die 'packet changed while it was being frozen'
prompt=$(<"$packet_copy")
set +e
$launcher "$((deadline + 10))" /dev/null -- "$agy_bin" --print "$prompt" --print-timeout "${deadline}s" --output-format text --disable-slash-commands --sandbox --mode "$mode" --model "$model" >"$partial"
rc=$?
set -e
mv -f "$partial" "$output"
rm -f "$packet_copy"
trap - EXIT
if [[ "$rc" -eq 124 ]]; then
  printf 'agy-native: outer deadline expired; partial output preserved at %s\n' "$output" >&2
  exit 124
fi
[[ "$rc" -eq 0 ]] || { printf 'agy-native: agy exited %s; partial output preserved at %s\n' "$rc" "$output" >&2; exit "$rc"; }
[[ -s "$output" ]] || die 'agy exited zero without an output artifact'
printf 'agy-native: role=%s model=%s context_id=%s mode=%s sandbox=true deadline=%ss process_group_reaped=true artifact=%s\n' "$role" "$model" "$context_id" "$mode" "$deadline" "$output" >&2

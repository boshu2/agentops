#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'codex-exec: %s\n' "$*" >&2
  exit 2
}
sha256_file() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

usage() {
  printf '%s\n' 'usage: run.sh --workspace ABS --prompt FILE --output ABS [--deadline 1..3600] [--sandbox read-only|workspace-write] [--model NAME] [--approve TOKEN]' >&2
}

workspace=''
prompt=''
output=''
deadline=600
sandbox='read-only'
model=''
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace) [[ $# -ge 2 ]] || die '--workspace needs a value'; workspace=$2; shift 2 ;;
    --prompt) [[ $# -ge 2 ]] || die '--prompt needs a value'; prompt=$2; shift 2 ;;
    --output) [[ $# -ge 2 ]] || die '--output needs a value'; output=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    --sandbox) [[ $# -ge 2 ]] || die '--sandbox needs a value'; sandbox=$2; shift 2 ;;
    --model) [[ $# -ge 2 ]] || die '--model needs a value'; model=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

[[ "$workspace" == /* && -d "$workspace" ]] || die 'workspace must be an existing absolute directory'
[[ ! -L "$workspace" ]] || die 'workspace may not be a symlink'
workspace=$(cd "$workspace" && pwd -P)
[[ "$workspace" != / ]] || die 'workspace may not be filesystem root'
[[ "$prompt" == /* && -f "$prompt" && ! -L "$prompt" ]] || die 'prompt must be an absolute regular non-symlink file'
[[ -s "$prompt" ]] || die 'prompt must not be empty'
prompt_bytes=$(wc -c <"$prompt" | tr -d ' ')
(( prompt_bytes <= 1048576 )) || die 'prompt exceeds the 1 MiB bound'
[[ "$output" == /* ]] || die 'output must be absolute'
[[ ! -L "$output" ]] || die 'output may not be a symlink'
[[ ! -e "$output" ]] || die 'output already exists; choose a new artifact path'
output_parent=$(dirname "$output")
[[ -d "$output_parent" && ! -L "$output_parent" ]] || die 'output parent must be an existing non-symlink directory'
output_parent=$(cd "$output_parent" && pwd -P)
output="$output_parent/$(basename "$output")"
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 1 && deadline <= 3600 )); then die 'deadline must be an integer from 1 through 3600 seconds'; fi
[[ -z "$model" || "$model" =~ ^[A-Za-z0-9._:/+-]{1,128}$ ]] || die 'model contains unsupported characters'
prompt_digest=$(sha256_file "$prompt")
case "$sandbox" in
  read-only) ;;
  workspace-write)
    model_key=${model:-default}
    expected="codex-exec:workspace-write:$workspace:$model_key:$prompt_digest"
    [[ "$approval" == "$expected" ]] || die "workspace-write requires exact approval token: $expected"
    ;;
  *) die 'sandbox must be read-only or workspace-write; danger-full-access is not exposed' ;;
esac

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
launcher="$script_dir/process-group-run.pl"
[[ -x "$launcher" ]] || die 'process-group launcher is missing or not executable'
command -v perl >/dev/null || die 'perl with POSIX::setsid is required for process-group cleanup'
codex_bin=${CODEX_BIN:-codex}
if [[ "$codex_bin" == */* ]]; then
  [[ -x "$codex_bin" ]] || die "codex binary is not executable: $codex_bin"
else
  codex_bin=$(command -v "$codex_bin") || die 'codex binary unavailable'
fi

version=$($launcher 10 /dev/null -- "$codex_bin" --version)
[[ "$version" =~ ^codex-cli[[:space:]][0-9]+\.[0-9]+\.[0-9]+ ]] || die "unrecognized codex version response: $version"
help=$($launcher 10 /dev/null -- "$codex_bin" exec --help)
for capability in '--sandbox' '--cd' '--ephemeral' '--output-last-message'; do
  [[ "$help" == *"$capability"* ]] || die "codex exec lacks required capability: $capability"
done
$launcher 15 /dev/null -- "$codex_bin" login status

umask 077
prompt_copy=$(mktemp "$output_parent/.codex-prompt.XXXXXX")
partial=$(mktemp "$output_parent/.codex-output.XXXXXX")
trap 'rm -f "$partial" "$prompt_copy"' EXIT
cp "$prompt" "$prompt_copy"
[[ "$(sha256_file "$prompt_copy")" == "$prompt_digest" && "$(sha256_file "$prompt")" == "$prompt_digest" ]] || die 'prompt changed while it was being frozen'
args=(exec -C "$workspace" --sandbox "$sandbox" --ephemeral --color never --output-last-message "$partial")
if [[ -n "$model" ]]; then
  args+=(--model "$model")
fi
args+=(-)

set +e
$launcher "$deadline" "$prompt_copy" -- "$codex_bin" "${args[@]}"
rc=$?
set -e
mv -f "$partial" "$output"
rm -f "$prompt_copy"
trap - EXIT
if [[ "$rc" -eq 124 ]]; then
  printf 'codex-exec: timed out after %ss; partial output preserved at %s; process group reaped\n' "$deadline" "$output" >&2
  exit 124
fi
if [[ "$rc" -ne 0 ]]; then
  printf 'codex-exec: codex exited %s; output preserved at %s; process group reaped\n' "$rc" "$output" >&2
  exit "$rc"
fi
[[ -s "$output" ]] || die "codex exited zero but produced no final artifact: $output"
printf 'codex-exec: exit=0 deadline=%ss fired=false process_group_reaped=true artifact=%s\n' "$deadline" "$output" >&2

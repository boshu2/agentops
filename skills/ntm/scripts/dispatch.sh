#!/usr/bin/env bash
set -euo pipefail

die() { printf 'ntm-dispatch: %s\n' "$*" >&2; exit 2; }
usage() { printf '%s\n' 'usage: dispatch.sh --session NAME --pane N|W.P|%N --workspace ABS --prompt-file ABS --receipt ABS --observe 1..300 --approve TOKEN' >&2; }
sha256_file() { if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }

session=''
pane=''
workspace=''
prompt_file=''
receipt=''
observe=30
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --session) [[ $# -ge 2 ]] || die '--session needs a value'; session=$2; shift 2 ;;
    --pane) [[ $# -ge 2 ]] || die '--pane needs a value'; pane=$2; shift 2 ;;
    --workspace) [[ $# -ge 2 ]] || die '--workspace needs a value'; workspace=$2; shift 2 ;;
    --prompt-file) [[ $# -ge 2 ]] || die '--prompt-file needs a value'; prompt_file=$2; shift 2 ;;
    --receipt) [[ $# -ge 2 ]] || die '--receipt needs a value'; receipt=$2; shift 2 ;;
    --observe) [[ $# -ge 2 ]] || die '--observe needs a value'; observe=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

[[ "$session" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || die 'unsafe session name'
[[ "$pane" =~ ^([0-9]+|[0-9]+\.[0-9]+|%[0-9]+)$ ]] || die 'unsafe pane selector'
[[ "$workspace" == /* && -d "$workspace" && ! -L "$workspace" ]] || die 'workspace must be an existing absolute non-symlink directory'
workspace=$(cd "$workspace" && pwd -P)
[[ "$workspace" != / ]] || die 'workspace may not be filesystem root'
[[ "$prompt_file" == /* && -f "$prompt_file" && ! -L "$prompt_file" && -s "$prompt_file" ]] || die 'prompt file must be an absolute nonempty regular non-symlink file'
bytes=$(wc -c <"$prompt_file" | tr -d ' ')
(( bytes <= 65536 )) || die 'prompt exceeds 64 KiB'
[[ "$receipt" == /* && ! -L "$receipt" ]] || die 'receipt must be absolute and not a symlink'
[[ ! -e "$receipt" ]] || die 'receipt already exists; choose a new artifact path'
receipt_parent=$(dirname "$receipt")
[[ -d "$receipt_parent" && ! -L "$receipt_parent" ]] || die 'receipt parent must be an existing non-symlink directory'
receipt_parent=$(cd "$receipt_parent" && pwd -P)
receipt="$receipt_parent/$(basename "$receipt")"
if ! [[ "$observe" =~ ^[0-9]+$ ]] || ! (( observe >= 1 && observe <= 300 )); then die 'observe must be 1-300 seconds'; fi
digest=$(sha256_file "$prompt_file")
expected="ntm:send:$session:$pane:$workspace:$digest"
[[ "$approval" == "$expected" ]] || die "exact approval required: $expected"

umask 077
prompt_copy=$(mktemp "$receipt_parent/.ntm-prompt.XXXXXX")
trap 'rm -f "$prompt_copy"' EXIT
cp "$prompt_file" "$prompt_copy"
[[ "$(sha256_file "$prompt_copy")" == "$digest" && "$(sha256_file "$prompt_file")" == "$digest" ]] || die 'prompt changed while it was being frozen'

ntm_bin=${NTM_BIN:-ntm}
if [[ "$ntm_bin" == */* ]]; then [[ -x "$ntm_bin" ]] || die "ntm binary is not executable: $ntm_bin"; else ntm_bin=$(command -v "$ntm_bin") || die 'ntm unavailable'; fi
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required'

for capability in send snapshot tail; do
  cap=$($timeout_bin --kill-after=2s 10 "$ntm_bin" --robot-capabilities --capability-command="$capability" --capability-compact)
  printf '%s' "$cap" | jq -e --arg name "$capability" '
    .success == true and (.version | type == "string") and any(.commands[]; .name == $name)
  ' >/dev/null || die "NTM does not attest robot capability: $capability"
done

snapshot=$($timeout_bin --kill-after=2s 15 "$ntm_bin" --robot-snapshot)
printf '%s' "$snapshot" | jq -e --arg session "$session" --arg pane "$pane" --arg workspace "$workspace" '
  .success == true
  and any(.. | strings; . == $session)
  and any(.. | strings; . == $pane)
  and any(.. | strings; . == $workspace)
' >/dev/null || die 'snapshot does not bind the named session/pane to the expected workspace'

dry=$($timeout_bin --kill-after=2s 15 "$ntm_bin" --robot-send="$session" --pane="$pane" --msg-file="$prompt_copy" --dry-run)
printf '%s' "$dry" | jq -e '.success == true' >/dev/null || die 'NTM dry-run refused the dispatch'

partial=$(mktemp "$receipt_parent/.ntm-receipt.XXXXXX")
trap 'rm -f "$partial" "$prompt_copy"' EXIT
set +e
$timeout_bin --kill-after=5s "$((observe + 10))" "$ntm_bin" --robot-send="$session" --pane="$pane" --msg-file="$prompt_copy" --track --timeout="${observe}s" >"$partial"
rc=$?
set -e
mv -f "$partial" "$receipt"
rm -f "$prompt_copy"
trap - EXIT
[[ "$rc" -ne 124 ]] || { printf 'ntm-dispatch: observation deadline expired; transport outcome unknown; receipt=%s\n' "$receipt" >&2; exit 124; }
[[ "$rc" -eq 0 ]] || { printf 'ntm-dispatch: send/track exited %s; receipt=%s\n' "$rc" "$receipt" >&2; exit "$rc"; }
jq -e '.success == true' "$receipt" >/dev/null || die 'NTM returned zero without a successful structured receipt'
$timeout_bin --kill-after=2s 15 "$ntm_bin" --robot-tail="$session" --panes="$pane" --lines=20 >&2
printf 'ntm-dispatch: session=%s pane=%s workspace=%s observation=%ss receipt=%s pane_created=false\n' "$session" "$pane" "$workspace" "$observe" "$receipt" >&2

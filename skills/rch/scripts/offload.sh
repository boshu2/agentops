#!/usr/bin/env bash
set -euo pipefail

die() { printf 'rch-offload: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' \
    'usage: offload.sh approval --workspace ABS -- cargo build|check|test|clippy [SAFE_ARG ...]' \
    '       offload.sh run --workspace ABS --receipt ABS --deadline 1..3600 --approve TOKEN -- cargo build|check|test|clippy [SAFE_ARG ...]' >&2
}
sha256_stream() {
  if command -v sha256sum >/dev/null; then sha256sum | awk '{print $1}';
  else shasum -a 256 | awk '{print $1}'; fi
}
resolve_tool() {
  local requested=$1 resolved
  if [[ "$requested" == */* ]]; then
    [[ "$requested" == /* && -x "$requested" ]] || die "RCH binary must be an absolute executable: $requested"
    resolved=$requested
  else
    resolved=$(command -v "$requested") || die 'rch is unavailable'
  fi
  printf '%s\n' "$resolved"
}

[[ $# -ge 1 ]] || { usage; exit 2; }
mode=$1
shift
[[ "$mode" == approval || "$mode" == run ]] || { usage; die "unknown mode: $mode"; }

workspace=''
receipt=''
deadline=600
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace) [[ $# -ge 2 ]] || die '--workspace needs a value'; workspace=$2; shift 2 ;;
    --receipt) [[ $# -ge 2 ]] || die '--receipt needs a value'; receipt=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    --) shift; break ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown option: $1" ;;
  esac
done

[[ "$workspace" == /* && -d "$workspace" && ! -L "$workspace" ]] || die 'workspace must be an existing absolute non-symlink directory'
supplied_workspace=${workspace%/}
[[ -n "$supplied_workspace" ]] || supplied_workspace=/
workspace=$(cd "$workspace" && pwd -P)
[[ "$workspace" == "$supplied_workspace" && "$workspace" != / ]] || die 'workspace must be canonical and may not be filesystem root'
git_root=$(git -C "$workspace" rev-parse --show-toplevel 2>/dev/null) || die 'workspace must be a Git worktree root'
git_root=$(cd "$git_root" && pwd -P)
[[ "$git_root" == "$workspace" ]] || die 'workspace must name the Git worktree root exactly'
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 1 && deadline <= 3600 )); then die 'deadline must be 1-3600 seconds'; fi
[[ $# -ge 2 && $# -le 64 ]] || die 'one bounded compilation command is required'

case "$1:$2" in
  cargo:build|cargo:check|cargo:test|cargo:clippy|bun:test) ;;
  *) die 'only cargo build/check/test/clippy or bun test may be offloaded' ;;
esac
for arg in "$@"; do
  (( ${#arg} <= 4096 )) || die 'command argument exceeds 4096 bytes'
  [[ "$arg" != *$'\n'* && "$arg" != *$'\r'* && "$arg" != *'/'* && "$arg" != *\\* ]] || die "unsafe path-bearing command argument: $arg"
  case "$arg" in --manifest-path*|--target-dir*|--config*|--out-dir*) die "unsupported path/configuration flag: $arg" ;; esac
done

digest=$({ printf '%s\0' "$workspace"; printf '%s\0' "$@"; } | sha256_stream)
expected="rch:offload:$workspace:$digest"
if [[ "$mode" == approval ]]; then
  printf '%s\n' "$expected"
  exit 0
fi
[[ "$approval" == "$expected" ]] || die "exact approval required: $expected"

[[ "$receipt" == /* && ! -L "$receipt" ]] || die 'receipt must be an absolute non-symlink path'
receipt_parent=$(dirname "$receipt")
[[ -d "$receipt_parent" && ! -L "$receipt_parent" ]] || die 'receipt parent must be an existing non-symlink directory'
receipt_parent=$(cd "$receipt_parent" && pwd -P)
receipt="$receipt_parent/$(basename "$receipt")"
stdout_file="$receipt.stdout"
stderr_file="$receipt.stderr"
[[ ! -e "$receipt" && ! -e "$stdout_file" && ! -e "$stderr_file" ]] || die 'receipt and capture paths must not already exist'
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required'
rch_bin=$(resolve_tool "${RCH_BIN:-rch}")

version_text=$($timeout_bin --kill-after=2s 10 "$rch_bin" --version)
version=$(printf '%s\n' "$version_text" | sed -nE 's/.*([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' | head -1)
[[ "$version" =~ ^1\.[0-9]+\.[0-9]+$ ]] || die "unsupported or unattested RCH version: $version_text"
capabilities=$($timeout_bin --kill-after=2s 10 "$rch_bin" --capabilities)
printf '%s' "$capabilities" | jq -e --arg version "$version" --arg runtime "$1" '
  (.version == $version)
    and ([.. | strings] | any(. == "exec"))
    and ([.. | strings] | any(. == "check"))
    and ([.. | strings] | any(. == (if $runtime == "cargo" then "rust" else "bun" end)))
' >/dev/null || die 'RCH capability document does not attest version, exec, check, and requested runtime'
help_json=$($timeout_bin --kill-after=2s 10 "$rch_bin" --help-json exec)
printf '%s' "$help_json" | jq -e '[.. | strings] | any(. == "exec")' >/dev/null || die 'RCH exec help is not machine-readable or does not attest exec'

set +e
$timeout_bin --kill-after=2s 15 "$rch_bin" check
check_rc=$?
set -e
[[ "$check_rc" -eq 0 ]] || die "rch check reports the remote path is not ready (exit $check_rc)"

umask 077
stdout_partial=$(mktemp "$receipt_parent/.rch-stdout.XXXXXX") || die 'could not create stdout capture'
stderr_partial=$(mktemp "$receipt_parent/.rch-stderr.XXXXXX") || die 'could not create stderr capture'
receipt_partial=$(mktemp "$receipt_parent/.rch-receipt.XXXXXX") || die 'could not create receipt staging file'
trap 'rm -f "$stdout_partial" "$stderr_partial" "$receipt_partial"' EXIT
set +e
(cd "$workspace" && RCH_VISIBILITY=summary "$timeout_bin" --kill-after=5s "$deadline" "$rch_bin" exec -- "$@") >"$stdout_partial" 2>"$stderr_partial"
rc=$?
set -e
cat "$stdout_partial"
cat "$stderr_partial" >&2
[[ ! -e "$stdout_file" && ! -e "$stderr_file" ]] || die 'capture destination appeared during execution'
mv "$stdout_partial" "$stdout_file"
mv "$stderr_partial" "$stderr_file"

summary=$(awk '/^\[RCH\] (remote|local) / {line=$0} END {print line}' "$stderr_file")
status=not_proven
if [[ "$rc" -eq 124 ]]; then
  status=failed
elif [[ "$summary" =~ ^\[RCH\]\ remote\ [^[:space:]]+\ \([0-9]+(ms|s)\)$ ]]; then
  status=remote
elif [[ "$summary" == '[RCH] local '* ]]; then
  status=local_fallback
fi
command_json=$(printf '%s\0' "$@" | jq -Rs 'split("\u0000")[:-1]')
jq -n \
  --arg status "$status" --arg workspace "$workspace" --arg version "$version" \
  --arg summary "$summary" --arg stdout "$stdout_file" --arg stderr "$stderr_file" \
  --argjson exit_code "$rc" --argjson command "$command_json" \
  '{status:$status, workspace:$workspace, rch_version:$version, command:$command, exit_code:$exit_code, summary:$summary, stdout:$stdout, stderr:$stderr, checked:["capabilities","exec-help","readiness","summary"], not_checked:["remote-worker-implementation"]}' >"$receipt_partial"
[[ ! -e "$receipt" ]] || die 'receipt destination appeared during execution'
mv "$receipt_partial" "$receipt"
trap - EXIT

if [[ "$rc" -eq 124 ]]; then
  printf 'rch-offload: deadline expired; remote cancellation is not attested; receipt=%s\n' "$receipt" >&2
  exit 124
fi
[[ "$status" == remote ]] || { printf 'rch-offload: remote execution not proved (status=%s); receipt=%s\n' "$status" "$receipt" >&2; exit 3; }
[[ "$rc" -eq 0 ]] || exit "$rc"
printf 'rch-offload: remote execution proved by runtime summary; receipt=%s\n' "$receipt" >&2

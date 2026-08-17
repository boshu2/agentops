#!/usr/bin/env bash
set -euo pipefail

die() { printf 'account-rotation: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' 'usage: rotate.sh --family codex|claude|gemini|opencode|cursor --profile NAME --tool caam|claude-acct --deadline 1..120 --approve rotate:FAMILY:PROFILE' >&2
}

family=''
profile=''
tool=''
approval=''
deadline=30
while [[ $# -gt 0 ]]; do
  case "$1" in
    --family) [[ $# -ge 2 ]] || die '--family needs a value'; family=$2; shift 2 ;;
    --profile) [[ $# -ge 2 ]] || die '--profile needs a value'; profile=$2; shift 2 ;;
    --tool) [[ $# -ge 2 ]] || die '--tool needs a value'; tool=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

case "$family" in codex|claude|gemini|opencode|cursor) ;; *) die 'unsupported agent family' ;; esac
[[ "$profile" =~ ^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$ ]] || die 'profile contains unsupported characters'
[[ "$approval" == "rotate:$family:$profile" ]] || die "exact approval required: rotate:$family:$profile"
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 1 && deadline <= 120 )); then die 'deadline must be 1-120 seconds'; fi
case "$tool" in caam|claude-acct) ;; *) die 'tool must be caam or claude-acct' ;; esac
if [[ "$tool" == claude-acct ]]; then
  [[ "$family" == claude ]] || die 'claude-acct is restricted to the claude family'
  [[ "$(uname -s)" == Darwin ]] || die 'claude-acct route is restricted to macOS'
fi

case "$tool" in
  caam) tool_env=ACCOUNT_ROTATION_CAAM_BIN ;;
  claude-acct) tool_env=ACCOUNT_ROTATION_CLAUDE_ACCT_BIN ;;
esac
binary=${!tool_env:-$tool}
if [[ "$binary" == */* ]]; then
  [[ -x "$binary" ]] || die "$tool binary is not executable: $binary"
else
  binary=$(command -v "$binary") || die "$tool binary unavailable"
fi
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required for bounded credential calls'

if [[ "$tool" == caam ]]; then
  help=$($timeout_bin --kill-after=2s 10 "$binary" --help)
  [[ "$help" == *'activate'* && "$help" == *'status'* && "$help" == *'version'* ]] || die 'caam command surface lacks activate/status/version'
  version=$($timeout_bin --kill-after=2s 10 "$binary" version)
  [[ "$version" =~ caam[[:space:]]+([0-9]+)\.([0-9]+)\.([0-9]+) ]] || die "unrecognized caam version: $version"
  before=$($timeout_bin --kill-after=2s 10 "$binary" status "$family" --json)
  set +e
  action=$($timeout_bin --kill-after=2s "$deadline" "$binary" activate "$family" "$profile" --json)
  rc=$?
  set -e
  printf '%s\n' "$action"
  [[ "$rc" -ne 124 ]] || { printf 'account-rotation: credential switch timed out; final profile is unknown\n' >&2; exit 124; }
  [[ "$rc" -eq 0 ]] || die "caam activate exited $rc"
  after=$($timeout_bin --kill-after=2s 10 "$binary" status "$family" --json)
else
  help=$($timeout_bin --kill-after=2s 10 "$binary" --help)
  [[ "$help" == *'use <name>'* && "$help" == *'current'* ]] || die 'claude-acct command surface lacks use/current'
  before=$($timeout_bin --kill-after=2s 10 "$binary" current)
  set +e
  action=$($timeout_bin --kill-after=2s "$deadline" "$binary" use "$profile")
  rc=$?
  set -e
  printf '%s\n' "$action"
  [[ "$rc" -ne 124 ]] || { printf 'account-rotation: credential switch timed out; final profile is unknown\n' >&2; exit 124; }
  [[ "$rc" -eq 0 ]] || die "claude-acct use exited $rc"
  after=$($timeout_bin --kill-after=2s 10 "$binary" current)
fi

printf 'account-rotation: before=%q after=%q requested=%q tool=%s\n' "$before" "$after" "$profile" "$tool" >&2
if [[ "$after" != *"$profile"* ]]; then
  printf 'account-rotation: switch returned success but the tool did not report the requested profile; partial rotation\n' >&2
  exit 3
fi
printf 'account-rotation: switched=true new_process_required=true runtime_identity_not_checked=true\n' >&2

#!/usr/bin/env bash
set -euo pipefail

die() { printf 'ms-reindex: %s\n' "$*" >&2; exit 2; }
usage() { printf '%s\n' 'usage: reindex.sh --data-dir ABS --skills-root ABS --approve ms:reindex:DATA_ABS:SKILLS_ABS [--deadline 30..900]' >&2; }

data_dir=''
skills_root=''
approval=''
deadline=300
while [[ $# -gt 0 ]]; do
  case "$1" in
    --data-dir) [[ $# -ge 2 ]] || die '--data-dir needs a value'; data_dir=$2; shift 2 ;;
    --skills-root) [[ $# -ge 2 ]] || die '--skills-root needs a value'; skills_root=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    --deadline) [[ $# -ge 2 ]] || die '--deadline needs a value'; deadline=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

[[ "$data_dir" == /* && -d "$data_dir" && ! -L "$data_dir" ]] || die 'data-dir must be an existing absolute non-symlink directory'
data_dir=$(cd "$data_dir" && pwd -P)
[[ "$data_dir" != / ]] || die 'data-dir may not be filesystem root'
[[ "$skills_root" == /* && -d "$skills_root" && ! -L "$skills_root" ]] || die 'skills-root must be an existing absolute non-symlink directory'
skills_root=$(cd "$skills_root" && pwd -P)
find "$skills_root" -mindepth 2 -maxdepth 2 -name SKILL.md -type f -print -quit | grep -q . || die 'skills-root contains no skill packages'
if ! [[ "$deadline" =~ ^[0-9]+$ ]] || ! (( deadline >= 30 && deadline <= 900 )); then die 'deadline must be 30-900 seconds'; fi
[[ "$approval" == "ms:reindex:$data_dir:$skills_root" ]] || die "exact approval required: ms:reindex:$data_dir:$skills_root"

ms_bin=${MS_BIN:-ms}
if [[ "$ms_bin" == */* ]]; then [[ -x "$ms_bin" ]] || die "ms binary is not executable: $ms_bin"; else ms_bin=$(command -v "$ms_bin") || die 'ms unavailable'; fi
if [[ "$ms_bin" != /* ]]; then ms_bin=$(cd "$(dirname "$ms_bin")" && pwd -P)/$(basename "$ms_bin"); fi
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required'
version=$($timeout_bin --kill-after=2s 10 "$ms_bin" -V)
[[ "$version" =~ ^ms[[:space:]][0-9]+\.[0-9]+\.[0-9]+ ]] || die "unrecognized ms version: $version"
help=$($timeout_bin --kill-after=2s 10 "$ms_bin" --help)
for capability in index mcp load; do [[ "$help" == *"$capability"* ]] || die "ms lacks required capability: $capability"; done

lock_dir="$data_dir/.agentops-reindex.lock"
mkdir "$lock_dir" 2>/dev/null || die "another package-owned reindex holds $lock_dir"
index_out="$lock_dir/index.json"
cleanup() {
  local rc=$?
  if [[ -f "$index_out" && ! -L "$index_out" ]]; then rm -f "$index_out"; fi
  if ! rmdir "$lock_dir"; then
    printf 'ms-reindex: cleanup failed; lock directory remains: %s\n' "$lock_dir" >&2
    [[ "$rc" -ne 0 ]] || rc=125
  fi
  trap - EXIT
  exit "$rc"
}
trap cleanup EXIT

same_user_server_pids() {
  local my_uid pid uid command
  my_uid=$(id -u)
  ps -Ao pid=,uid=,command= | while read -r pid uid command; do
    [[ "$uid" == "$my_uid" ]] || continue
    case "$command" in
      "$ms_bin mcp serve"|"$ms_bin mcp serve "*) printf '%s\n' "$pid" ;;
    esac
  done
}

stop_servers() {
  local pids pid alive
  pids=$(same_user_server_pids)
  [[ -n "$pids" ]] || { printf 'ms-reindex: no exact same-user ms mcp serve processes\n' >&2; return 0; }
  printf 'ms-reindex: TERM exact server pids: %s\n' "$(printf '%s' "$pids" | tr '\n' ' ')" >&2
  for pid in $pids; do kill -TERM "$pid" || die "could not TERM server pid $pid"; done
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    alive=''
    for pid in $pids; do kill -0 "$pid" 2>/dev/null && alive="$alive $pid"; done
    [[ -z "$alive" ]] && break
    sleep 0.1
  done
  for pid in $pids; do kill -0 "$pid" 2>/dev/null && kill -KILL "$pid"; done
  sleep 0.1
  for pid in $pids; do kill -0 "$pid" 2>/dev/null && die "server pid $pid survived TERM/KILL cleanup"; done
  [[ -z "$(same_user_server_pids)" ]] || die 'an exact ms mcp serve process raced or survived cleanup'
}

stop_servers
for lock in "$data_dir/ms.lock" "$data_dir/index/.tantivy-writer.lock" "$data_dir/index/.tantivy-meta.lock"; do
  [[ -e "$lock" ]] || continue
  [[ ! -L "$lock" ]] || die "refusing symlink lock: $lock"
  lock_pid=''
  if [[ "$lock" == "$data_dir/ms.lock" ]]; then lock_pid=$(jq -r '.pid // empty' "$lock"); fi
  if [[ "$lock_pid" =~ ^[0-9]+$ ]] && kill -0 "$lock_pid" 2>/dev/null; then
    die "lock $lock is held by live pid $lock_pid"
  fi
  rm -f "$lock"
done

umask 077
set +e
MS_SKILL_PATHS="$skills_root" $timeout_bin --kill-after=5s "$deadline" "$ms_bin" index -O json >"$index_out"
index_rc=$?
set -e
[[ "$index_rc" -eq 0 ]] || die "ms index exited $index_rc"
cat "$index_out"
jq -e '
  (.indexed | type == "number" and . > 0 and floor == .)
  and (.errors | type == "array")
  and (.package_summary.skills_discovered | type == "number" and . > 0 and floor == .)
  and (.indexed + (.errors | length) == .package_summary.skills_discovered)
' "$index_out" >/dev/null || die 'index result is empty, malformed, or incompletely accounted'

stop_servers
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
probe=$($timeout_bin --kill-after=5s 40 env MS_BIN="$ms_bin" python3 "$script_dir/mcp-search.py" --timeout 30 'account rotation')
printf '%s\n' "$probe"
printf '%s' "$probe" | jq -e '.count == (.results | length) and .count > 0' >/dev/null || die 'fresh MCP server returned no searchable results'
[[ -z "$(same_user_server_pids)" ]] || die 'fresh one-shot probe left an ms server running'
printf 'ms-reindex: indexed=true servers_reaped=true fresh_probe=true data_dir=%s skills_root=%s\n' "$data_dir" "$skills_root" >&2

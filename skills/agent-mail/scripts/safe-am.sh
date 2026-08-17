#!/usr/bin/env bash
set -euo pipefail

die() { printf 'agent-mail: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' 'usage: safe-am.sh send|reserve|release|guard-install|doctor-repair|reset [bounded options]' >&2
}
sha256_text() {
  if command -v sha256sum >/dev/null; then sha256sum | awk '{print $1}';
  elif command -v shasum >/dev/null; then shasum -a 256 | awk '{print $1}';
  else die 'sha256sum or shasum is required'; fi
}
canonical_dir() {
  local value=$1 label=$2
  [[ "$value" == /* && -d "$value" && ! -L "$value" ]] || die "$label must be an existing absolute non-symlink directory"
  (cd "$value" && pwd -P)
}
valid_name() { [[ "$1" =~ ^[A-Za-z][A-Za-z0-9_-]{0,63}$ ]]; }

[[ $# -ge 1 ]] || { usage; exit 2; }
operation=$1
shift

project=''
agent=''
sender=''
recipients=''
thread=''
subject=''
body_file=''
ttl=3600
reason=''
repo=''
backup_dir=''
approval=''
ack_required=false
paths=()
ids=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --project) [[ $# -ge 2 ]] || die '--project needs a value'; project=$2; shift 2 ;;
    --agent) [[ $# -ge 2 ]] || die '--agent needs a value'; agent=$2; shift 2 ;;
    --from) [[ $# -ge 2 ]] || die '--from needs a value'; sender=$2; shift 2 ;;
    --to) [[ $# -ge 2 ]] || die '--to needs a value'; recipients=$2; shift 2 ;;
    --thread) [[ $# -ge 2 ]] || die '--thread needs a value'; thread=$2; shift 2 ;;
    --subject) [[ $# -ge 2 ]] || die '--subject needs a value'; subject=$2; shift 2 ;;
    --body-file) [[ $# -ge 2 ]] || die '--body-file needs a value'; body_file=$2; shift 2 ;;
    --ack-required) ack_required=true; shift ;;
    --ttl) [[ $# -ge 2 ]] || die '--ttl needs a value'; ttl=$2; shift 2 ;;
    --reason) [[ $# -ge 2 ]] || die '--reason needs a value'; reason=$2; shift 2 ;;
    --path) [[ $# -ge 2 ]] || die '--path needs a value'; paths+=("$2"); shift 2 ;;
    --ids) [[ $# -ge 2 ]] || die '--ids needs a value'; ids=$2; shift 2 ;;
    --repo) [[ $# -ge 2 ]] || die '--repo needs a value'; repo=$2; shift 2 ;;
    --backup-dir) [[ $# -ge 2 ]] || die '--backup-dir needs a value'; backup_dir=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unknown argument: $1" ;;
  esac
done

if [[ "$operation" == reset ]]; then
  printf '%s\n' 'agent-mail: reset is intentionally not exposed; clear-and-reset-everything cannot be bounded or rolled back by this package' >&2
  exit 4
fi
case "$operation" in send|reserve|release|guard-install|doctor-repair) ;; *) usage; die "unsupported operation: $operation" ;; esac
project=$(canonical_dir "$project" project)

am_bin=${AM_BIN:-am}
if [[ "$am_bin" == */* ]]; then
  [[ -x "$am_bin" ]] || die "am binary is not executable: $am_bin"
else
  am_bin=$(command -v "$am_bin") || die 'am binary unavailable'
fi
timeout_bin=$(command -v timeout || command -v gtimeout || true)
[[ -n "$timeout_bin" ]] || die 'timeout or gtimeout is required for bounded Agent Mail calls'
version=$($timeout_bin --kill-after=2s 10 "$am_bin" --version)
[[ "$version" =~ ^am[[:space:]][0-9]+\.[0-9]+\.[0-9]+ ]] || die "unrecognized am version: $version"
capabilities=$($timeout_bin --kill-after=2s 10 "$am_bin" capabilities --json)
printf '%s' "$capabilities" | jq -e '.schema_version == "am.capabilities.v1" and .tool == "am"' >/dev/null || die 'unrecognized am capabilities schema'
has_surface() {
  local name=$1 child=$2
  printf '%s' "$capabilities" | jq -e --arg name "$name" --arg child "$child" '
    any(.commands[]; .name == $name and (.direct_subcommands | index($child) != null))
  ' >/dev/null
}

case "$operation" in
  send)
    valid_name "$sender" || die 'sender must be a bounded Agent Mail name'
    [[ "$recipients" =~ ^[A-Za-z][A-Za-z0-9_-]{0,63}(,[A-Za-z][A-Za-z0-9_-]{0,63}){0,15}$ ]] || die 'recipients must be 1-16 comma-separated bounded names'
    [[ "$thread" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$ ]] || die 'thread id is required and contains unsupported characters'
    (( ${#subject} >= 1 && ${#subject} <= 200 )) || die 'subject must be 1-200 characters'
    [[ "$body_file" == /* && -f "$body_file" && ! -L "$body_file" && -s "$body_file" ]] || die 'body file must be an absolute nonempty regular non-symlink file'
    body_bytes=$(wc -c <"$body_file" | tr -d ' ')
    (( body_bytes <= 65536 )) || die 'message body exceeds 64 KiB'
    body=$(<"$body_file")
    digest=$(printf '%s' "$body" | sha256_text)
    subject_digest=$(printf '%s' "$subject" | sha256_text)
    expected="am:send:$project:$sender:$recipients:$thread:$subject_digest:$digest:$ack_required"
    [[ "$approval" == "$expected" ]] || die "exact approval required: $expected"
    has_surface mail send || die 'am capabilities do not attest mail send'
    args=(mail send --project "$project" --from "$sender" --to "$recipients" --subject "$subject" --body "$body" --thread-id "$thread" --json)
    $ack_required && args+=(--ack-required)
    $timeout_bin --kill-after=2s 30 "$am_bin" "${args[@]}"
    ;;
  reserve)
    valid_name "$agent" || die 'agent must be a bounded Agent Mail name'
    if ! [[ "$ttl" =~ ^[0-9]+$ ]] || ! (( ttl >= 60 && ttl <= 7200 )); then die 'ttl must be 60-7200 seconds'; fi
    (( ${#paths[@]} >= 1 && ${#paths[@]} <= 32 )) || die 'reserve requires 1-32 paths'
    (( ${#reason} >= 1 && ${#reason} <= 200 )) || die 'reservation reason must be 1-200 characters'
    [[ "$reason" != *$'\n'* && "$reason" != *$'\r'* ]] || die 'reservation reason must be one line'
    for path in "${paths[@]}"; do
      [[ -n "$path" && ${#path} -le 512 && "$path" != /* && "$path" != -* && "$path" != *\\* && "$path" != *'//'*
        && "$path" != */ && "$path" != *$'\n'* && "$path" != *$'\r'* ]] || die "unsafe reservation path: $path"
      IFS='/' read -r -a path_parts <<<"$path"
      for part in "${path_parts[@]}"; do [[ -n "$part" && "$part" != . && "$part" != .. ]] || die "unsafe reservation path: $path"; done
    done
    path_digest=$(printf '%s\n' "${paths[@]}" | { if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi; } | awk '{print $1}')
    reason_digest=$(printf '%s' "$reason" | sha256_text)
    expected="am:reserve:$project:$agent:$ttl:$path_digest:$reason_digest"
    [[ "$approval" == "$expected" ]] || die "exact approval required: $expected"
    has_surface file_reservations reserve || die 'am capabilities do not attest reservation creation'
    $timeout_bin --kill-after=2s 30 "$am_bin" file_reservations conflicts "$project" "${paths[@]}"
    $timeout_bin --kill-after=2s 30 "$am_bin" file_reservations reserve "$project" "$agent" "${paths[@]}" --ttl "$ttl" --exclusive --reason "$reason"
    ;;
  release)
    valid_name "$agent" || die 'agent must be a bounded Agent Mail name'
    [[ "$ids" =~ ^[0-9]+(,[0-9]+){0,31}$ ]] || die 'release requires 1-32 explicit numeric reservation ids'
    expected="am:release:$project:$agent:$ids"
    [[ "$approval" == "$expected" ]] || die "exact approval required: $expected"
    has_surface file_reservations release || die 'am capabilities do not attest reservation release'
    $timeout_bin --kill-after=2s 30 "$am_bin" file_reservations release "$project" "$agent" --ids "$ids"
    ;;
  guard-install)
    repo=$(canonical_dir "$repo" repo)
    [[ -d "$repo/.git" || -f "$repo/.git" ]] || die 'repo is not a Git worktree'
    [[ "$repo" == "$project" || "$repo" == "$project"/* ]] || die 'guard repo must equal or be contained by the project'
    expected="am:guard-install:$project:$repo"
    [[ "$approval" == "$expected" ]] || die "exact approval required: $expected"
    has_surface guard install || die 'am capabilities do not attest guard install'
    $timeout_bin --kill-after=2s 30 "$am_bin" guard install "$project" "$repo" --no-prepush
    ;;
  doctor-repair)
    backup_dir=$(canonical_dir "$backup_dir" backup-dir)
    expected="am:doctor-repair:$project:$backup_dir"
    [[ "$approval" == "$expected" ]] || die "exact approval required: $expected"
    has_surface doctor repair || die 'am capabilities do not attest doctor repair'
    $timeout_bin --kill-after=2s 30 "$am_bin" doctor repair "$project" --dry-run
    $timeout_bin --kill-after=2s 120 "$am_bin" doctor repair "$project" --yes --backup-dir "$backup_dir"
    ;;
esac
printf 'agent-mail: operation=%s project=%s bounded=true reset_exposed=false\n' "$operation" "$project" >&2

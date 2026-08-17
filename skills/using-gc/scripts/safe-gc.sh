#!/usr/bin/env bash
set -euo pipefail

die() { printf 'using-gc: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' \
    'usage: safe-gc.sh token prepare --city ABS --rig ABS' \
    '       safe-gc.sh token dispatch --city ABS --rig ABS --bead ID --message-file ABS' \
    '       safe-gc.sh check --city ABS --rig ABS' \
    '       safe-gc.sh prepare --city ABS --rig ABS --approve TOKEN' \
    '       safe-gc.sh dispatch --city ABS --rig ABS --bead ID --message-file ABS --receipt ABS --approve TOKEN' >&2
}
sha256_text() {
  if command -v sha256sum >/dev/null; then sha256sum | awk '{print $1}';
  else shasum -a 256 | awk '{print $1}'; fi
}
resolve_tool() {
  local label=$1 requested=$2 resolved
  if [[ "$requested" == */* ]]; then
    [[ "$requested" == /* && -x "$requested" ]] || die "$label binary must be an absolute executable: $requested"
    resolved=$requested
  else
    resolved=$(command -v "$requested") || die "$label is unavailable"
  fi
  printf '%s\n' "$resolved"
}
validate_scope() {
  local supplied_city supplied_rig
  [[ "$city" == /* && -d "$city" && ! -L "$city" ]] || die 'city must be an existing absolute non-symlink directory'
  supplied_city=${city%/}; [[ -n "$supplied_city" ]] || supplied_city=/
  city=$(cd "$city" && pwd -P)
  [[ "$city" == "$supplied_city" && "$city" != / ]] || die 'city must be canonical and may not be filesystem root'
  [[ -z "${HOME:-}" || "$city" != "$HOME" ]] || die 'city may not be the operator home'
  [[ -f "$city/city.toml" && ! -L "$city/city.toml" ]] || die 'city must contain a regular city.toml'
  [[ "$rig" == /* && -d "$rig" && ! -L "$rig" ]] || die 'rig must be an existing absolute non-symlink directory'
  supplied_rig=${rig%/}; [[ -n "$supplied_rig" ]] || supplied_rig=/
  rig=$(cd "$rig" && pwd -P)
  [[ "$rig" == "$supplied_rig" && "$rig" != / ]] || die 'rig must be canonical and may not be filesystem root'
  [[ "$rig" == "$city"/rigs/* && "$rig" != "$city/rigs" ]] || die 'rig must be a named directory below the selected city rigs directory'
}
validate_message() {
  [[ "$bead" =~ ^[A-Za-z][A-Za-z0-9]{0,15}-[A-Za-z0-9]{1,64}$ ]] || die 'source bead id has an unsafe shape'
  [[ "$message_file" == /* && -f "$message_file" && ! -L "$message_file" && -s "$message_file" ]] || die 'message file must be an absolute nonempty regular non-symlink file'
  local bytes
  bytes=$(wc -c <"$message_file" | tr -d ' ')
  (( bytes <= 65536 )) || die 'message exceeds 64 KiB'
}
attest_runtime() {
  GC_BIN=$(resolve_tool gc "${GC_BIN:-gc}")
  AO_BIN=$(resolve_tool ao "${AO_BIN:-ao}")
  TIMEOUT_BIN=$(command -v timeout || command -v gtimeout || true)
  [[ -n "$TIMEOUT_BIN" ]] || die 'timeout or gtimeout is required'
  local gc_version gc_help mail_help ao_version ao_help
  gc_version=$($TIMEOUT_BIN --kill-after=2s 10 "$GC_BIN" version --json)
  printf '%s' "$gc_version" | jq -e '.ok == true and (.version | test("^1\\.4\\.[0-9]+$"))' >/dev/null || die 'exact Gas City 1.4.x structured version attestation is required'
  gc_help=$($TIMEOUT_BIN --kill-after=2s 10 "$GC_BIN" --help)
  for word in bd mail version --city --rig; do [[ "$gc_help" == *"$word"* ]] || die "Gas City help lacks required surface: $word"; done
  mail_help=$($TIMEOUT_BIN --kill-after=2s 10 "$GC_BIN" --city "$city" --rig "$rig" mail send --help)
  for word in --notify --json --subject --message; do [[ "$mail_help" == *"$word"* ]] || die "Gas City mail help lacks required surface: $word"; done
  ao_version=$($TIMEOUT_BIN --kill-after=2s 10 "$AO_BIN" version --json)
  printf '%s' "$ao_version" | jq -e '.version == "3.5.0"' >/dev/null || die 'AgentOps 3.5.0 runtime attestation is required'
  ao_help=$($TIMEOUT_BIN --kill-after=2s 10 "$AO_BIN" gc --help)
  for word in prepare check recover-affinity; do [[ "$ao_help" == *"$word"* ]] || die "AgentOps GC help lacks required surface: $word"; done
}
run_check() {
  $TIMEOUT_BIN --kill-after=5s 120 "$AO_BIN" gc check --city "$city" --rig "$rig" --gc-bin "$GC_BIN"
}

[[ $# -ge 1 ]] || { usage; exit 2; }
mode=$1
shift
if [[ "$mode" == token ]]; then
  [[ $# -ge 1 ]] || { usage; die 'token requires prepare or dispatch'; }
  token_for=$1
  shift
else
  token_for=''
fi

city=''
rig=''
bead=''
message_file=''
receipt=''
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --city) [[ $# -ge 2 ]] || die '--city needs a value'; city=$2; shift 2 ;;
    --rig) [[ $# -ge 2 ]] || die '--rig needs a value'; rig=$2; shift 2 ;;
    --bead) [[ $# -ge 2 ]] || die '--bead needs a value'; bead=$2; shift 2 ;;
    --message-file) [[ $# -ge 2 ]] || die '--message-file needs a value'; message_file=$2; shift 2 ;;
    --receipt) [[ $# -ge 2 ]] || die '--receipt needs a value'; receipt=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unsupported argument or operation: $1" ;;
  esac
done
validate_scope

operation=${token_for:-$mode}
case "$operation" in
  prepare)
    [[ -z "$bead" && -z "$message_file" && -z "$receipt" ]] || die 'prepare accepts only city, rig, and approval'
    expected="gc:prepare:$city:$rig"
    ;;
  dispatch)
    validate_message
    message_content=$(<"$message_file")
    digest=$(printf '%s' "$message_content" | sha256_text)
    expected="gc:dispatch:$city:$rig:$bead:$digest"
    ;;
  check)
    [[ -z "$token_for" && -z "$bead" && -z "$message_file" && -z "$receipt" && -z "$approval" ]] || die 'check accepts only city and rig'
    expected=''
    ;;
  *) usage; die "unsupported operation: $operation" ;;
esac

if [[ -n "$token_for" ]]; then
  [[ "$token_for" == prepare || "$token_for" == dispatch ]] || die 'tokens are available only for prepare and dispatch'
  [[ -z "$approval" ]] || die 'token generation does not accept approval'
  printf '%s\n' "$expected"
  exit 0
fi
[[ "$mode" == check || "$mode" == prepare || "$mode" == dispatch ]] || die "unsupported operation: $mode"
if [[ "$mode" != check ]]; then [[ "$approval" == "$expected" ]] || die "exact caller approval required: $expected"; fi
attest_runtime

case "$mode" in
  check)
    run_check
    printf 'using-gc: exact city/rig runtime check passed; external implementation remains unproved\n' >&2
    ;;
  prepare)
    $TIMEOUT_BIN --kill-after=5s 300 "$AO_BIN" gc prepare --city "$city" --rig "$rig" --gc-bin "$GC_BIN"
    run_check
    printf 'using-gc: approved prepare completed and post-check passed\n' >&2
    ;;
  dispatch)
    [[ "$receipt" == /* && ! -L "$receipt" ]] || die 'receipt must be an absolute non-symlink path'
    [[ ! -e "$receipt" ]] || die 'receipt already exists; choose a new artifact path'
    receipt_parent=$(dirname "$receipt")
    [[ -d "$receipt_parent" && ! -L "$receipt_parent" ]] || die 'receipt parent must be an existing non-symlink directory'
    receipt_parent=$(cd "$receipt_parent" && pwd -P)
    receipt="$receipt_parent/$(basename "$receipt")"
    message_content=$(<"$message_file")
    [[ "$(printf '%s' "$message_content" | sha256_text)" == "$digest" ]] || die 'message changed after approval; generate a new token'
    run_check
    intent=$($TIMEOUT_BIN --kill-after=2s 30 "$GC_BIN" --city "$city" --rig "$rig" bd show "$bead" --json)
    normalized_intent=$(printf '%s' "$intent" | jq -ce --arg id "$bead" '
      (if type == "array" then . else [.] end)
      | map(select(.id == $id))
      | select(length == 1)
      | .[0]
      | select((.status // "open") != "closed")
      | select((.title | type) == "string" and (.title | length) > 0)
      | select((.description | type) == "string" and (.description | test("acceptance"; "i")))
    ') || die 'source bead must exist exactly once, remain open, and contain title plus acceptance'
    subject="Build $bead"
    body=$(printf 'Source intent: %s\n\n%s' "$bead" "$message_content")
    partial="$receipt.partial.$$"
    stderr_partial="$receipt.stderr.partial.$$"
    trap 'rm -f "$partial" "$stderr_partial"' EXIT
    set +e
    mail=$($TIMEOUT_BIN --kill-after=5s 45 "$GC_BIN" --city "$city" --rig "$rig" mail send mayor --from human --subject "$subject" --message "$body" --notify --json 2>"$stderr_partial")
    rc=$?
    set -e
    cat "$stderr_partial" >&2
    if [[ "$rc" -ne 0 ]]; then
      jq -n --arg city "$city" --arg rig "$rig" --arg bead "$bead" --arg raw "$mail" --argjson exit_code "$rc" \
        '{status:"mail_outcome_unknown", city:$city, rig:$rig, source_bead:$bead, exit_code:$exit_code, raw_mail_output:$raw, checked:["runtime-version","command-surfaces","ao-gc-check","source-intent"], not_checked:["mail-delivery","mayor-notify","gas-city-runtime-implementation","mayor-processing","model-dispatch"]}' >"$partial"
      mv -f "$partial" "$receipt"
      rm -f "$stderr_partial"
      trap - EXIT
      [[ "$rc" -ne 124 ]] || { printf 'using-gc: Mayor mail timed out; delivery outcome unknown; receipt=%s\n' "$receipt" >&2; exit 124; }
      printf 'using-gc: Mayor mail failed with exit %s; receipt=%s\n' "$rc" "$receipt" >&2
      exit "$rc"
    fi
    if ! printf '%s' "$mail" | jq -e '.schema_version == "1" and .ok == true and .command == "mail.send" and .action == "send" and .count == 1 and .notified == true and (.id | type == "string")' >/dev/null; then
      jq -n --arg city "$city" --arg rig "$rig" --arg bead "$bead" --arg raw "$mail" \
        '{status:"mail_unattested", city:$city, rig:$rig, source_bead:$bead, raw_mail_output:$raw, checked:["runtime-version","command-surfaces","ao-gc-check","source-intent"], not_checked:["mail-delivery","mayor-notify","gas-city-runtime-implementation","mayor-processing","model-dispatch"]}' >"$partial"
      mv -f "$partial" "$receipt"
      rm -f "$stderr_partial"
      trap - EXIT
      printf 'using-gc: Gas City returned zero without one attested notified Mayor message; receipt=%s\n' "$receipt" >&2
      exit 3
    fi
    jq -n --arg city "$city" --arg rig "$rig" --arg bead "$bead" --argjson intent "$normalized_intent" --argjson mail "$mail" \
      '{status:"dispatched_to_mayor", city:$city, rig:$rig, source_bead:$bead, intent:$intent, mail:$mail, checked:["runtime-version","command-surfaces","ao-gc-check","source-intent","mayor-mail-notify"], not_checked:["gas-city-runtime-implementation","mayor-processing","model-dispatch"]}' >"$partial"
    mv -f "$partial" "$receipt"
    rm -f "$stderr_partial"
    trap - EXIT
    printf 'using-gc: source intent mailed once to Mayor; receipt=%s\n' "$receipt" >&2
    ;;
esac

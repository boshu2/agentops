#!/usr/bin/env bash
set -euo pipefail

die() { printf 'sbh-safe-clean: %s\n' "$*" >&2; exit 2; }
usage() {
  printf '%s\n' \
    'usage: safe-clean.sh plan --root ABS --max-items 1..1000 --plan-out ABS' \
    '       safe-clean.sh apply --plan ABS --approve sbh:clean:SHA256' >&2
}
sha256_stream() {
  if command -v sha256sum >/dev/null; then sha256sum | awk '{print $1}';
  else shasum -a 256 | awk '{print $1}'; fi
}
resolve_tool() {
  local requested=$1 resolved
  if [[ "$requested" == */* ]]; then
    [[ "$requested" == /* && -x "$requested" ]] || die "sbh binary must be an absolute executable: $requested"
    resolved=$requested
  else
    resolved=$(command -v "$requested") || die 'sbh is unavailable'
  fi
  printf '%s\n' "$resolved"
}
canonical_output() {
  local output=$1 parent
  [[ "$output" == /* && ! -L "$output" ]] || die 'output path must be absolute and not a symlink'
  parent=$(dirname "$output")
  [[ -d "$parent" && ! -L "$parent" ]] || die 'output parent must be an existing non-symlink directory'
  parent=$(cd "$parent" && pwd -P)
  printf '%s/%s\n' "$parent" "$(basename "$output")"
}
validate_root() {
  local candidate=$1 supplied count owner
  [[ "$candidate" == /* && -d "$candidate" && ! -L "$candidate" ]] || die 'cleanup root must be an existing absolute non-symlink directory'
  supplied=${candidate%/}; [[ -n "$supplied" ]] || supplied=/
  CLEAN_ROOT=$(cd "$candidate" && pwd -P)
  [[ "$CLEAN_ROOT" == "$supplied" && "$CLEAN_ROOT" != / ]] || die 'cleanup root must be canonical and may not be filesystem root'
  [[ -z "${HOME:-}" || "$CLEAN_ROOT" != "$HOME" ]] || die 'cleanup root may not be the operator home'
  [[ -f "$CLEAN_ROOT/.sbh-clean-root" && ! -L "$CLEAN_ROOT/.sbh-clean-root" ]] || die 'cleanup root lacks a regular .sbh-clean-root marker'
  [[ "$(cat "$CLEAN_ROOT/.sbh-clean-root")" == agentops-sbh-clean-root-v1 ]] || die 'cleanup root marker has the wrong content'
  [[ -z "$(find "$CLEAN_ROOT" -xdev -type l -print -quit)" ]] || die 'cleanup root contains a symlink'
  [[ -z "$(find "$CLEAN_ROOT" -xdev -name .git -print -quit)" ]] || die 'cleanup root contains a Git control path'
  owner=$(id -un)
  [[ -z "$(find "$CLEAN_ROOT" -xdev ! -user "$owner" -print -quit)" ]] || die 'cleanup root contains an entry owned by another user'
  count=$(find "$CLEAN_ROOT" -xdev -print | awk 'NR > 10000 { exit 42 } END { print NR }') || die 'cleanup root exceeds the 10,000-entry inspection bound'
  (( count <= 10000 )) || die 'cleanup root exceeds the 10,000-entry inspection bound'
  CLEAN_MOUNT=$(df -P "$CLEAN_ROOT" | awk 'END {print $6}')
  [[ "$CLEAN_MOUNT" == /* && "$CLEAN_ROOT" != "$CLEAN_MOUNT" ]] || die 'cleanup root may not be an entire filesystem mount'
}
attest_runtime() {
  SBH_BIN=$(resolve_tool "${SBH_BIN:-sbh}")
  TIMEOUT_BIN=$(command -v timeout || command -v gtimeout || true)
  [[ -n "$TIMEOUT_BIN" ]] || die 'timeout or gtimeout is required'
  local version_text help
  version_text=$($TIMEOUT_BIN --kill-after=2s 10 "$SBH_BIN" --version)
  [[ "$version_text" =~ (^|[[:space:]])0\.4\.27($|[[:space:]]) ]] || die "sbh 0.4.27 is required, got: $version_text"
  help=$($TIMEOUT_BIN --kill-after=2s 10 "$SBH_BIN" clean --help)
  for flag in --dry-run --yes --json --max-items; do
    [[ "$help" == *"$flag"* ]] || die "sbh clean does not attest required flag: $flag"
  done
}
read_status() {
  local status
  status=$($TIMEOUT_BIN --kill-after=2s 15 "$SBH_BIN" --json status)
  printf '%s' "$status" | jq -ce --arg mount "$CLEAN_MOUNT" '
    . as $doc
    | ($doc.version == "0.4.27"
       and ($doc.pressure.mounts | type == "array")
       and ([$doc.pressure.mounts[] | select(.path == $mount and (.container_id | type == "string") and (.free | type == "number"))] | length == 1)) as $ok
    | if $ok then ($doc.pressure.mounts[] | select(.path == $mount) | {mount:.path, container_id, free}) else empty end
  ' || die "sbh status does not attest exactly one entry for mount: $CLEAN_MOUNT"
}
read_dry_run() {
  local dry
  dry=$($TIMEOUT_BIN --kill-after=2s 60 "$SBH_BIN" clean --dry-run --json --max-items "$MAX_ITEMS" "$CLEAN_ROOT")
  printf '%s' "$dry" | jq -cSe --argjson max "$MAX_ITEMS" '
    select(.command == "clean" and .dry_run == true)
    | select((.candidates_count | type) == "number" and (.items_would_delete | type) == "number" and (.bytes_would_free | type) == "number")
    | select(.candidates_count == .items_would_delete and .candidates_count >= 1 and .candidates_count <= $max and .bytes_would_free > 0)
    | {command, dry_run, candidates_count, items_would_delete, bytes_would_free, protected_count}
  ' || die 'dry run is missing, empty, inconsistent, or exceeds the approved item bound'
}

[[ $# -ge 1 ]] || { usage; exit 2; }
mode=$1
shift
root=''
max_items=''
plan_out=''
plan=''
approval=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --root) [[ $# -ge 2 ]] || die '--root needs a value'; root=$2; shift 2 ;;
    --max-items) [[ $# -ge 2 ]] || die '--max-items needs a value'; max_items=$2; shift 2 ;;
    --plan-out) [[ $# -ge 2 ]] || die '--plan-out needs a value'; plan_out=$2; shift 2 ;;
    --plan) [[ $# -ge 2 ]] || die '--plan needs a value'; plan=$2; shift 2 ;;
    --approve) [[ $# -ge 2 ]] || die '--approve needs a value'; approval=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; die "unsupported argument or operation: $1" ;;
  esac
done

case "$mode" in
  plan)
    [[ -n "$root" && "$max_items" =~ ^[0-9]+$ && "$max_items" -ge 1 && "$max_items" -le 1000 ]] || die 'plan requires --root and --max-items 1-1000'
    [[ -n "$plan_out" && -z "$plan" && -z "$approval" ]] || die 'plan requires only --plan-out, not --plan/--approve'
    validate_root "$root"
    MAX_ITEMS=$max_items
    plan_out=$(canonical_output "$plan_out")
    [[ ! -e "$plan_out" ]] || die 'plan output already exists; choose a new path'
    attest_runtime
    before=$(read_status)
    dry=$(read_dry_run)
    base=$(jq -cnS --arg root "$CLEAN_ROOT" --arg mount "$CLEAN_MOUNT" --argjson max "$MAX_ITEMS" --argjson before "$before" --argjson dry "$dry" \
      '{schema:"agentops.sbh-clean-plan.v1", root:$root, mount:$mount, max_items:$max, before:$before, dry_run:$dry, runtime:{name:"sbh",version:"0.4.27"}}')
    digest=$(printf '%s' "$base" | sha256_stream)
    token="sbh:clean:$digest"
    umask 077
    partial=$(mktemp "$(dirname "$plan_out")/.sbh-plan.XXXXXX")
    trap 'rm -f "$partial"' EXIT
    printf '%s' "$base" | jq --arg token "$token" '. + {approval_token:$token}' >"$partial"
    mv -f "$partial" "$plan_out"
    trap - EXIT
    printf 'sbh-safe-clean: reviewed plan=%s approval=%s\n' "$plan_out" "$token" >&2
    ;;
  apply)
    [[ -n "$plan" && -n "$approval" && -z "$root" && -z "$max_items" && -z "$plan_out" ]] || die 'apply requires only --plan and --approve'
    [[ "$plan" == /* && -f "$plan" && ! -L "$plan" ]] || die 'plan must be an absolute regular non-symlink file'
    plan_json=$(jq -cS . "$plan") || die 'invalid cleanup plan JSON'
    printf '%s' "$plan_json" | jq -e '.schema == "agentops.sbh-clean-plan.v1" and (.root | type == "string") and (.mount | type == "string") and (.max_items | type == "number") and (.approval_token | type == "string")' >/dev/null || die 'invalid cleanup plan'
    base=$(printf '%s' "$plan_json" | jq -cS 'del(.approval_token)')
    digest=$(printf '%s' "$base" | sha256_stream)
    expected="sbh:clean:$digest"
    recorded=$(printf '%s' "$plan_json" | jq -r '.approval_token')
    [[ "$recorded" == "$expected" && "$approval" == "$expected" ]] || die "exact approval required for unchanged plan: $expected"
    root=$(printf '%s' "$plan_json" | jq -r '.root')
    max_items=$(printf '%s' "$plan_json" | jq -r '.max_items')
    [[ "$max_items" =~ ^[0-9]+$ && "$max_items" -ge 1 && "$max_items" -le 1000 ]] || die 'plan item bound is invalid'
    validate_root "$root"
    MAX_ITEMS=$max_items
    [[ "$CLEAN_MOUNT" == "$(printf '%s' "$plan_json" | jq -r '.mount')" ]] || die 'cleanup root now resolves to a different mount'
    attest_runtime
    current=$(read_status)
    [[ "$(printf '%s' "$current" | jq -r '.container_id')" == "$(printf '%s' "$plan_json" | jq -r '.before.container_id')" ]] || die 'filesystem container identity changed after planning'
    dry=$(read_dry_run)
    [[ "$dry" == "$(printf '%s' "$plan_json" | jq -cS '.dry_run')" ]] || die 'cleanup candidates changed after planning; create and approve a new plan'
    receipt="$plan.applied.json"
    [[ ! -e "$receipt" && ! -L "$receipt" ]] || die 'apply receipt already exists; choose a new plan path'
    partial=$(mktemp "$(dirname "$receipt")/.sbh-receipt.XXXXXX")
    stderr_partial=$(mktemp "$(dirname "$receipt")/.sbh-stderr.XXXXXX")
    trap 'rm -f "$partial" "$stderr_partial"' EXIT
    persist_apply_failure() {
      local status=$1 code=$2 detail=$3 after_raw=${4:-}
      jq -n --arg status "$status" --arg detail "$detail" --arg raw_result "${result:-}" --arg after_raw "$after_raw" \
        --argjson exit_code "$code" --argjson plan "$plan_json" --argjson before "$current" \
        '{status:$status, detail:$detail, exit_code:$exit_code, plan:$plan, before:$before, raw_result:$raw_result, after_raw:$after_raw, checked:["marker","tree-bound","runtime-version","dry-run-stability","mount-identity"], not_checked:["cleanup-outcome","sbh-implementation","open-file-detection"]}' >"$partial"
      mv -f "$partial" "$receipt"
      rm -f "$stderr_partial"
      trap - EXIT
      printf 'sbh-safe-clean: %s; receipt=%s\n' "$detail" "$receipt" >&2
      exit "$code"
    }
    set +e
    result=$($TIMEOUT_BIN --kill-after=5s 120 "$SBH_BIN" clean --yes --json --max-items "$MAX_ITEMS" "$CLEAN_ROOT" 2>"$stderr_partial")
    rc=$?
    set -e
    cat "$stderr_partial" >&2
    [[ "$rc" -ne 124 ]] || persist_apply_failure cleanup_outcome_unknown 124 'cleanup timed out; outcome unknown'
    [[ "$rc" -eq 0 ]] || persist_apply_failure cleanup_failed "$rc" "sbh clean failed with exit $rc; no retry was attempted"
    if ! printf '%s' "$result" | jq -e --argjson max "$MAX_ITEMS" 'select(.command == "clean" and .dry_run == false and (.items_deleted | type == "number") and .items_deleted >= 0 and .items_deleted <= $max)' >/dev/null; then
      persist_apply_failure cleanup_unattested 3 'sbh returned zero without a bounded cleanup result'
    fi
    set +e
    after=$(read_status)
    post_rc=$?
    set -e
    [[ "$post_rc" -eq 0 ]] || persist_apply_failure postcondition_unavailable 3 'post-cleanup status was unavailable'
    [[ "$(printf '%s' "$after" | jq -r '.container_id')" == "$(printf '%s' "$current" | jq -r '.container_id')" ]] || persist_apply_failure postcondition_failed 3 'post-cleanup status refers to a different container' "$after"
    before_free=$(printf '%s' "$current" | jq -r '.free')
    after_free=$(printf '%s' "$after" | jq -r '.free')
    (( after_free >= before_free )) || persist_apply_failure postcondition_failed 3 'free space decreased on the constrained mount after cleanup' "$after"
    jq -n --argjson plan "$plan_json" --argjson result "$result" --argjson after "$after" \
      '{status:"applied", plan:$plan, result:$result, after:$after, checked:["marker","tree-bound","runtime-version","dry-run-stability","mount-identity","free-space"], not_checked:["sbh-implementation","open-file-detection"]}' >"$partial"
    mv -f "$partial" "$receipt"
    rm -f "$stderr_partial"
    trap - EXIT
    printf 'sbh-safe-clean: one bounded cleanup applied; receipt=%s\n' "$receipt" >&2
    ;;
  *) usage; die "unsupported operation: $mode" ;;
esac

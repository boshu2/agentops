#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

write_json() {
  local path="$1"
  shift
  printf '%s\n' "$@" >"$path"
}

run_no_overlap_case() {
  local manifest="$TMP_ROOT/no-overlap.json"
  write_json "$manifest" \
    '[' \
    '  {"id":"task-a","subject":"edit docs","files":["docs/a.md"]},' \
    '  {"id":"task-b","subject":"edit cli","files":["cli/cmd/ao/a.go"]}' \
    ']'

  bash "$REPO_ROOT/scripts/check-file-manifest-overlap.sh" "$manifest" \
    | grep -q 'No file manifest overlaps detected'
  echo "file-manifest-no-overlap-ok"
}

run_conflict_case() {
  local manifest="$TMP_ROOT/conflict.json"
  write_json "$manifest" \
    '[' \
    '  {"id":"task-a","subject":"edit docs","files":["docs/a.md"]},' \
    '  {"id":"task-b","subject":"also edit docs","files":["docs/a.md"]}' \
    ']'

  local out rc
  set +e
  out="$(bash "$REPO_ROOT/scripts/check-file-manifest-overlap.sh" "$manifest" 2>&1)"
  rc=$?
  set -e

  test "$rc" -eq 1
  grep -q 'CONFLICT: docs/a.md claimed by task task-a and task task-b' <<<"$out"
  grep -q 'Found 1 file overlap conflict(s)' <<<"$out"
  echo "file-manifest-conflict-ok"
}

run_missing_manifest_case() {
  local manifest="$TMP_ROOT/missing.json"
  write_json "$manifest" \
    '[' \
    '  {"id":"task-a","subject":"edit docs","files":["docs/a.md"]},' \
    '  {"id":"task-b","subject":"unknown scope","files":[]}' \
    ']'

  local out
  out="$(bash "$REPO_ROOT/scripts/check-file-manifest-overlap.sh" "$manifest")"
  grep -q 'WARN: task task-b has no file manifest' <<<"$out"
  grep -q 'No file manifest overlaps detected (1 task(s) missing manifests)' <<<"$out"
  echo "file-manifest-missing-ok"
}

case "${1:-all}" in
  no-overlap) run_no_overlap_case ;;
  conflict) run_conflict_case ;;
  missing) run_missing_manifest_case ;;
  all)
    run_no_overlap_case
    run_conflict_case
    run_missing_manifest_case
    ;;
  *)
    echo "usage: $0 [no-overlap|conflict|missing|all]" >&2
    exit 2
    ;;
esac

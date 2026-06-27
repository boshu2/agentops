#!/usr/bin/env bash
# assert-no-actions.sh — land-path guardrail: GitHub Actions must not enter the loop.
#
# The land lane is a local serialization + verification path. This guard blocks
# accidental regression to PR / merge-queue / workflow-dispatch behavior both by
# static scan and by a runtime gh PATH shim.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  scripts/assert-no-actions.sh check [--land-lane FILE] [--land-submit FILE]
  scripts/assert-no-actions.sh install-shim <shim-dir> <log-file> [real-gh]
  scripts/assert-no-actions.sh guard-gh <log-file> <real-gh> -- <gh-args...>

check runs the static land-script scan (the lane scripts must invoke no
Actions-dispatching gh verbs). install-shim writes a gh executable that delegates
back to guard-gh, the runtime PATH shim that hard-errors on an Actions-invoking
gh subcommand during a landing.

This guard asserts only that THE LAND LANE never invokes GitHub Actions — it does
NOT police the repo's CI config. validate.yml legitimately carries pull_request /
merge_group triggers for ordinary contributor PR CI; the lane simply never opens a
PR. Conflating "the lane never invokes Actions" with "the repo has no PR CI" was a
prior bug (agentops-2pl.11) and the validate.yml config-assertion was dropped.
EOF
}

die() {
  echo "assert-no-actions: ERROR: $*" >&2
  exit 2
}

fail() {
  echo "assert-no-actions: FAIL: $*" >&2
  exit 1
}

json_escape() {
  local s="${1:-}"
  s="${s//\\/\\\\}"; s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"; s="${s//$'\r'/\\r}"; s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

is_forbidden_gh() {
  local joined=" $* "
  case "${1:-}" in
    pr)
      case "${2:-}" in
        create|merge) return 0 ;;
      esac
      ;;
    workflow)
      [[ "${2:-}" == "run" ]] && return 0
      ;;
    api)
      if printf '%s\n' "$joined" | grep -Eqi '(^|[[:space:]])[^[:space:]]*/dispatches([[:space:]]|$)|repository_dispatch|workflow_dispatch'; then
        return 0
      fi
      ;;
  esac

  if printf '%s\n' "$joined" | grep -Eqi 'merge[_ -]?group|merge[_ -]?queue|enqueue[[:space:]].*(merge|queue)|queue[[:space:]]+merge'; then
    return 0
  fi
  return 1
}

scan_static_file() {
  local file="$1" line n=0 bad=0 trimmed
  [[ -f "$file" ]] || fail "static target missing: $file"

  while IFS= read -r line || [[ -n "$line" ]]; do
    n=$((n + 1))
    trimmed="${line#"${line%%[![:space:]]*}"}"
    [[ -z "$trimmed" || "$trimmed" == \#* ]] && continue

    if printf '%s\n' "$line" | grep -Eq '(^|[;&|[:space:]])gh[[:space:]]+pr[[:space:]]+(create|merge)([[:space:]]|$)'; then
      echo "assert-no-actions: forbidden gh PR mutation in $file:$n" >&2
      bad=1
    elif printf '%s\n' "$line" | grep -Eq '(^|[;&|[:space:]])gh[[:space:]]+workflow[[:space:]]+run([[:space:]]|$)'; then
      echo "assert-no-actions: forbidden gh workflow dispatch in $file:$n" >&2
      bad=1
    elif printf '%s\n' "$line" | grep -Eqi '(^|[;&|[:space:]])gh[[:space:]]+api[[:space:]].*/dispatches([[:space:]]|$)|repository_dispatch|workflow_dispatch'; then
      echo "assert-no-actions: forbidden gh api dispatch in $file:$n" >&2
      bad=1
    elif printf '%s\n' "$line" | grep -Eqi '(^|[;&|[:space:]])gh[[:space:]].*(merge[_ -]?group|merge[_ -]?queue|enqueue[[:space:]].*(merge|queue)|queue[[:space:]]+merge)'; then
      echo "assert-no-actions: forbidden merge-queue/merge-group gh path in $file:$n" >&2
      bad=1
    fi
  done <"$file"

  [[ "$bad" -eq 0 ]] || return 1
}

run_check() {
  local land_lane="$REPO_ROOT/scripts/land-lane-run.sh"
  local land_submit="$REPO_ROOT/scripts/land-submit.sh"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --land-lane) land_lane="${2:-}"; shift 2 ;;
      --land-submit) land_submit="${2:-}"; shift 2 ;;
      *) die "unknown check arg: $1" ;;
    esac
  done

  scan_static_file "$land_lane"
  scan_static_file "$land_submit"
  echo "assert-no-actions: PASS"
}

write_shim() {
  local shim_dir="$1" log_file="$2" real_gh="${3:-}"
  [[ -n "$shim_dir" ]] || die "missing shim dir"
  [[ -n "$log_file" ]] || die "missing log file"
  mkdir -p "$shim_dir" "$(dirname "$log_file")"

  cat >"$shim_dir/gh" <<EOF
#!/usr/bin/env bash
exec "$SCRIPT_DIR/assert-no-actions.sh" guard-gh "$log_file" "$real_gh" -- "\$@"
EOF
  chmod +x "$shim_dir/gh"
  echo "$shim_dir"
}

guard_gh() {
  local log_file="$1" real_gh="$2"; shift 2
  [[ "${1:-}" == "--" ]] && shift
  local ts argv
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  argv="$*"

  if is_forbidden_gh "$@"; then
    mkdir -p "$(dirname "$log_file")"
    printf '{"timestamp":"%s","status":"blocked","argv":"%s"}\n' \
      "$(json_escape "$ts")" "$(json_escape "gh $argv")" >>"$log_file"
    echo "BLOCKED (assert-no-actions): gh $argv would invoke GitHub Actions / PR merge path; land lane must stay local." >&2
    exit 2
  fi

  mkdir -p "$(dirname "$log_file")"
  printf '{"timestamp":"%s","status":"allowed","argv":"%s"}\n' \
    "$(json_escape "$ts")" "$(json_escape "gh $argv")" >>"$log_file"

  if [[ -n "$real_gh" && -x "$real_gh" ]]; then
    exec "$real_gh" "$@"
  fi

  echo "gh: command not found" >&2
  exit 127
}

main() {
  local mode="${1:-}"
  case "$mode" in
    check)
      shift
      run_check "$@"
      ;;
    install-shim)
      [[ $# -ge 3 && $# -le 4 ]] || die "install-shim requires <shim-dir> <log-file> [real-gh]"
      write_shim "$2" "$3" "${4:-}"
      ;;
    guard-gh)
      [[ $# -ge 4 ]] || die "guard-gh requires <log-file> <real-gh> -- <gh-args...>"
      shift
      guard_gh "$@"
      ;;
    -h|--help|"")
      usage
      ;;
    *)
      die "unknown mode: $mode"
      ;;
  esac
}

main "$@"

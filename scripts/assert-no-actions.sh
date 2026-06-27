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
back to guard-gh, the runtime PATH shim. guard-gh is DEFAULT-DENY (an allowlist):
it delegates to the real gh ONLY for explicitly enumerated read-only / safe
subcommands (gh run view|list|watch, gh pr view|list|checks|diff, gh api GET,
gh auth status, gh --version, ...) and hard-errors on everything else — so an
UNKNOWN or new gh verb (e.g. a future `gh run <x>`) is BLOCKED, not allowed. This
closes the prior blocklist hole where un-enumerated Actions-invoking verbs such
as `gh run rerun` slipped through (agentops-2pl.11).

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

# --------------------------------------------------------------------------- #
# RUNTIME SHIM POSTURE: DEFAULT-DENY (allowlist), not blocklist.
#
# A blocklist (enumerate the bad verbs, allow the rest) is structurally unsafe:
# any Actions-invoking verb NOT enumerated slips through. The refuted defect
# (agentops-2pl.11) was exactly this — `gh run rerun 123` re-runs a workflow run
# (which INVOKES GitHub Actions) yet was logged 'allowed' and delegated to the
# real gh, because only `gh workflow run` / `gh pr create|merge` were enumerated.
#
# The fix flips the posture: the shim ALLOWS only an explicit allowlist of
# clearly read-only / safe gh subcommands, and BLOCKS everything else — so an
# UNKNOWN or future `gh run *` / `gh workflow *` verb is denied by default and
# can't reopen this hole. is_allowed_gh below is the single source of truth for
# what the land lane may delegate to the real gh.
# --------------------------------------------------------------------------- #

# api_is_read_only: true iff a `gh api ...` invocation is a safe GET read.
# Blocks: any explicit non-GET method (-X/--method POST|PUT|PATCH|DELETE), any
# request-body flag (-f/-F/--input/--raw-field/--field) IN EITHER the space form
# (`--field x=y`) OR the equals form (`--field=x=y`), and any Actions-mutating or
# dispatch endpoint (.../dispatches, /actions/, repository_dispatch, etc.).
#
# The equals-form of the body flags was the agentops-2pl.11 reviewer hole: the old
# case arms matched only the bare long flags (`--field`/`--raw-field`/`--input`),
# so `gh api --field=x=y` / `--raw-field=x=y` / `--input=f.json` were delegated to
# the real gh and could POST to a dispatch endpoint. A body flag token now counts
# if it EQUALS the flag OR STARTS WITH `flag=`, closing both forms.
#
# is_body_flag: a token is a request-body flag (→ implies a write) in any form.
is_body_flag() {
  case "$1" in
    # long forms — bare (space form) or equals form
    --field|--field=*) return 0 ;;
    --raw-field|--raw-field=*) return 0 ;;
    --input|--input=*) return 0 ;;
    # short forms — bare (space form) or attached value (`-ffield=value`)
    -f|-f*) return 0 ;;
    -F|-F*) return 0 ;;
  esac
  return 1
}

api_is_read_only() {
  # args here are the tokens AFTER the leading `api`.
  local joined=" $* " tok next i=1 method="GET"
  # Endpoint-shape denials (covers /actions/ mutations + dispatch triggers).
  if printf '%s\n' "$joined" | grep -Eqi '(^|[[:space:]])[^[:space:]]*/dispatches([[:space:]]|$)|repository_dispatch|workflow_dispatch|/actions/'; then
    return 1
  fi
  for tok in "$@"; do
    # any request body / field flag implies a write — match BOTH space and equals
    # forms (`--field x=y` and `--field=x=y`) and the short attached form.
    if is_body_flag "$tok"; then
      return 1
    fi
    case "$tok" in
      -X|--method)
        # next token is the method
        next="${@:$((i + 1)):1}"
        method="$next"
        ;;
      -X*) method="${tok#-X}" ;;
      --method=*) method="${tok#--method=}" ;;
    esac
    i=$((i + 1))
  done
  case "$(printf '%s' "$method" | tr '[:lower:]' '[:upper:]')" in
    GET|HEAD) return 0 ;;
    *) return 1 ;;
  esac
}

# is_allowed_gh: DEFAULT-DENY allowlist. Returns 0 (allow → delegate to real gh)
# ONLY for explicitly enumerated read-only / safe subcommands. Everything else
# returns 1 (deny → blocked + logged). New/unknown verbs are denied by default.
is_allowed_gh() {
  local cmd="${1:-}" sub="${2:-}"
  case "$cmd" in
    # bare informational flags / no-op
    --version|version|--help|-h|help|"") return 0 ;;
    auth)
      case "$sub" in status|"") return 0 ;; *) return 1 ;; esac ;;
    run)
      # READ-ONLY run inspection only. rerun/cancel/delete/download (and any
      # unknown verb) are DENIED — rerun was the refuted hole.
      case "$sub" in view|list|watch) return 0 ;; *) return 1 ;; esac ;;
    workflow)
      # view/list inspect; run/enable/disable INVOKE or arm Actions → denied.
      case "$sub" in view|list) return 0 ;; *) return 1 ;; esac ;;
    pr)
      # create/merge/ready/close/edit/comment/review all mutate → denied.
      case "$sub" in view|list|checks|diff|status) return 0 ;; *) return 1 ;; esac ;;
    api)
      shift || true
      api_is_read_only "$@" && return 0
      return 1 ;;
    # Plainly read-only top-level groups (no Actions reach).
    repo)
      case "$sub" in view|list) return 0 ;; *) return 1 ;; esac ;;
    release)
      case "$sub" in view|list|download) return 0 ;; *) return 1 ;; esac ;;
    issue)
      case "$sub" in view|list|status) return 0 ;; *) return 1 ;; esac ;;
    cache)
      case "$sub" in list) return 0 ;; *) return 1 ;; esac ;;
    label|search|status|browse) return 0 ;;
  esac
  return 1
}

# is_forbidden_gh: retained as the negation of the allowlist so the runtime shim
# is DEFAULT-DENY. Anything not explicitly allowed is forbidden.
is_forbidden_gh() {
  is_allowed_gh "$@" && return 1
  return 0
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
    elif printf '%s\n' "$line" | grep -Eq '(^|[;&|[:space:]])gh[[:space:]]+workflow[[:space:]]+(run|enable|disable)([[:space:]]|$)'; then
      echo "assert-no-actions: forbidden gh workflow dispatch in $file:$n" >&2
      bad=1
    elif printf '%s\n' "$line" | grep -Eq '(^|[;&|[:space:]])gh[[:space:]]+run[[:space:]]+(rerun|cancel)([[:space:]]|$)'; then
      # gh run rerun re-INVOKES a workflow run (the agentops-2pl.11 reviewer hole);
      # gh run cancel mutates an in-flight Actions run. Both are Actions paths.
      echo "assert-no-actions: forbidden gh run mutation in $file:$n" >&2
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
    echo "BLOCKED (assert-no-actions): gh $argv is not on the land-lane read-only allowlist (default-deny); it may invoke GitHub Actions / a PR-merge path. The land lane must stay local." >&2
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

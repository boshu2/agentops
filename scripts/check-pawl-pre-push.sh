#!/usr/bin/env bash
# check-pawl-pre-push.sh — cross-family pawl gate for push-to-main (age-58o).
#
# Reads git pre-push hook stdin (local_ref local_sha remote_ref remote_sha).
# When pushing to refs/heads/main, requires a CONFIRMED, commit-bound pawl
# verdict via scripts/pawl-verdict.sh check (pr=0 for push-to-main landings).
#
# Skip: AGENTOPS_PREPUSH_SKIP_PAWL=1, no stdin (standalone gate runs), branch
# delete, non-main refs, #trivial chore commits.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GIT_REPO="${AGENTOPS_REPO_ROOT:-$REPO_ROOT}"
PAWL="$SCRIPT_DIR/pawl-verdict.sh"
VERDICT_DIR="${AGENTOPS_PAWL_VERDICT_DIR:-$REPO_ROOT/.agents/pawl-verdicts}"
PUSH_TO_MAIN_PR=0

die() { echo "pawl-pre-push: ERROR: $*" >&2; exit 2; }

truthy() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

extract_bead_from_commit() {
  local sha="$1"
  local msg
  msg="$(git -C "$GIT_REPO" log -1 --format=%B "$sha" 2>/dev/null || true)"
  if [[ "$msg" =~ \((age-[a-z0-9.-]+|ag-[a-z0-9.-]+)\) ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  if [[ "$msg" =~ (^|[[:space:][:punct:]])(age-[a-z0-9.-]+|ag-[a-z0-9.-]+)([[:space:][:punct:]]|$) ]]; then
    printf '%s\n' "${BASH_REMATCH[2]}"
    return 0
  fi
  return 1
}

is_main_push() {
  case "${1:-}" in
    refs/heads/main|refs/heads/master) return 0 ;;
  esac
  return 1
}

is_delete_push() {
  local sha="${1:-}"
  [[ "$sha" == "0000000000000000000000000000000000000000" ]]
}

check_one_push() {
  local local_ref="$1" local_sha="$2" remote_ref="$3" remote_sha="$4"

  is_main_push "$remote_ref" || return 0
  is_delete_push "$local_sha" && return 0

  local bead head msg
  head="$local_sha"
  msg="$(git -C "$GIT_REPO" log -1 --format=%B "$head" 2>/dev/null || true)"
  if grep -qi '#trivial' <<<"$msg"; then
    echo "pawl-pre-push: #trivial commit at ${head:0:12} — pawl waived" >&2
    return 0
  fi

  if ! bead="$(extract_bead_from_commit "$head")"; then
    echo "PAWL-HOLD: push to $remote_ref at ${head:0:12} cites no bead id — fail-closed (mutate-shared-trunk requires pawl verdict)" >&2
    return 1
  fi

  [[ -x "$PAWL" ]] || die "pawl-verdict.sh not executable at $PAWL"

  if "$PAWL" check "$bead" "$PUSH_TO_MAIN_PR" --dir "$VERDICT_DIR" --head "$head"; then
    echo "pawl-pre-push: CONFIRMED verdict for bead=$bead head=${head:0:12} — push authorized" >&2
    return 0
  fi
  echo "PAWL-HOLD: no CONFIRMED pawl verdict for bead=$bead push-to-main head=${head:0:12} — push refused (age-58o)" >&2
  return 1
}

main() {
  if truthy "${AGENTOPS_PREPUSH_SKIP_PAWL:-0}"; then
    echo "pawl-pre-push: skipped (AGENTOPS_PREPUSH_SKIP_PAWL=1)" >&2
    exit 0
  fi

  if [ -t 0 ]; then
    echo "pawl-pre-push: no pre-push stdin — skipped (not a git push hook invocation)" >&2
    exit 0
  fi

  local had_input=false
  local local_ref local_sha remote_ref remote_sha
  while read -r local_ref local_sha remote_ref remote_sha; do
    [[ -n "$local_ref" ]] || continue
    had_input=true
    check_one_push "$local_ref" "$local_sha" "$remote_ref" "$remote_sha" || exit 1
  done

  if [[ "$had_input" == false ]]; then
    echo "pawl-pre-push: no pre-push stdin — skipped (not a git push hook invocation)" >&2
  fi
  exit 0
}

main "$@"

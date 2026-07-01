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
  local msg bead
  msg="$(git -C "$GIT_REPO" log -1 --format=%B "$sha" 2>/dev/null || true)"

  # Precedence (age-push-equals-ci-0ua.3): an explicit closing TRAILER
  # (Closes/Fixes/Refs/Bead <id>) is the intent-bearing citation and wins over a
  # bead merely MENTIONED in prose. Without this, the FIRST bead token in the
  # message is taken, so "...after the ag-3l86 incident ... Closes age-foo"
  # resolves to ag-3l86 and fails-closed against the wrong (absent) verdict —
  # observed live 2026-06-18. Trailer match is case-insensitive.
  # Trailer-BOUND (git-trailer convention): only a LINE that STARTS with the
  # keyword counts — so "this fixes ag-old behaviour" (keyword mid-prose) does
  # NOT match, but "Closes age-new" on its own line does. Last such line wins
  # (trailers are appended at the end). A first-match scan got this wrong twice.
  local had_nocase=0 line
  shopt -q nocasematch && had_nocase=1
  shopt -s nocasematch
  while IFS= read -r line; do
    # The WHOLE line must be essentially "keyword <bead>" (+ trailing
    # punctuation): a sentence that merely STARTS with a keyword — "Fixed ag-old
    # during earlier investigation." — has prose after the bead and is rejected.
    if [[ "$line" =~ ^[[:space:]]*(closes|close|fixes|fixed|fix|resolves|resolve|refs|ref|bead)[:[:space:]]+(age-[a-z0-9.-]+|ag-[a-z0-9.-]+)[[:space:][:punct:]]*$ ]]; then
      bead="${BASH_REMATCH[2]}"
    fi
  done <<< "$msg"
  [[ "$had_nocase" -eq 0 ]] && shopt -u nocasematch

  if [[ -z "${bead:-}" ]]; then
    if [[ "$msg" =~ \((age-[a-z0-9.-]+|ag-[a-z0-9.-]+)\) ]]; then
      bead="${BASH_REMATCH[1]}"
    elif [[ "$msg" =~ (^|[[:space:][:punct:]])(age-[a-z0-9.-]+|ag-[a-z0-9.-]+)([[:space:][:punct:]]|$) ]]; then
      bead="${BASH_REMATCH[2]}"
    else
      return 1
    fi
  fi
  # A bead id never ends in a separator. The id char class allows '.' for
  # sub-ids (age-3va.1), so a sentence-ending period — "Closes <id>." — gets
  # greedily pulled in, yielding "<id>." and a hunt for the non-existent
  # "<id>..json" verdict file (fail-closed on a normal commit message). Strip
  # any trailing run of non-alphanumerics so "<id>." normalizes to "<id>".
  bead="${bead%"${bead##*[a-z0-9]}"}"
  printf '%s\n' "$bead"
  return 0
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

  local bead head subject body
  head="$local_sha"
  # age-w2ny: waive ONLY when #trivial is an explicit marker — a TRAILING tag at
  # the END of the subject line (the established convention, e.g.
  # "chore(...): ... #trivial") or a standalone trailer line in the body. A
  # #trivial merely MENTIONED in prose — anywhere in the body, OR mid-subject
  # (e.g. "fix(pawl): prevent #trivial from bypassing pawl") — must NOT waive the
  # cross-family pawl. That was a fail-open: any non-trivial commit could bypass
  # the gate by naming #trivial (cross-family REFUTE: the original subject anchor
  # still waived mid-subject prose mentions).
  subject="$(git -C "$GIT_REPO" log -1 --format=%s "$head" 2>/dev/null || true)"
  body="$(git -C "$GIT_REPO" log -1 --format=%b "$head" 2>/dev/null || true)"
  if grep -qiE '(^|[[:space:]])#trivial[[:space:]]*$' <<<"$subject" \
     || grep -qiE '^[[:space:]]*#trivial[[:space:]]*$' <<<"$body"; then
    # age-u43w: #trivial is an AUTHOR ASSERTION, not a fact — do not waive the
    # cross-family pawl on the message alone. Verify the DIFF is ACTUALLY trivial:
    # every changed file within the provenance-ledger allowlist (the sole
    # established #trivial use — post-land sensor / pawl-verdict edges; 100% of
    # historical #trivial commits touch only docs/provenance/). A #trivial-tagged
    # commit touching ANY other path (code, scripts, skills, other docs) must
    # still face the pawl, else "no verdict = not done" is bypassable by
    # mislabeling any change #trivial. Fail-closed: an empty/unreadable file list
    # cannot prove triviality, so it does NOT waive.
    # --no-renames: force a rename to show as delete(old)+add(new) so a rename FROM
    # a non-provenance path INTO docs/provenance/ exposes the non-allowlisted old
    # path (rather than --name-only reporting only the allowlisted destination).
    # Capture the exit status explicitly: a FAILED diff-tree is fail-closed (we
    # cannot prove triviality), never trusted as authoritative.
    local changed nontrivial
    if ! changed="$(git -C "$GIT_REPO" diff-tree --no-commit-id --no-renames --name-only -r "$head" 2>/dev/null)"; then
      echo "PAWL-HOLD: #trivial at ${head:0:12} — diff-tree failed; cannot prove triviality — fail-closed, pawl required" >&2
      return 1
    fi
    if [[ -z "$changed" ]]; then
      echo "PAWL-HOLD: #trivial at ${head:0:12} has an empty changed-file list — cannot prove triviality — fail-closed, pawl required" >&2
      return 1
    fi
    nontrivial="$(grep -vE '^docs/provenance/' <<<"$changed" || true)"
    if [[ -z "$nontrivial" ]]; then
      echo "pawl-pre-push: #trivial commit at ${head:0:12} (provenance-ledger only) — pawl waived" >&2
      return 0
    fi
    echo "PAWL-HOLD: #trivial at ${head:0:12} touches non-trivial path(s) — waiver REFUSED, cross-family pawl still required:" >&2
    while IFS= read -r _f; do [[ -n "$_f" ]] && echo "  $_f" >&2; done <<<"$nontrivial"
    # fall through to the normal pawl requirement (do NOT return 0)
  fi

  if ! bead="$(extract_bead_from_commit "$head")"; then
    echo "PAWL-HOLD: push to $remote_ref at ${head:0:12} cites no bead id — fail-closed (mutate-shared-trunk requires pawl verdict)" >&2
    return 1
  fi

  [[ -x "$PAWL" ]] || die "pawl-verdict.sh not executable at $PAWL"

  if "$PAWL" check "$bead" "$PUSH_TO_MAIN_PR" --dir "$VERDICT_DIR" --head "$head"; then
    # age-bb5l: surface the tier so a fresh-context (single-family, weaker) land is a conscious
    # choice, not silent. (The check above already prints mode=…; this adds the explicit nudge.)
    _vmode="$(jq -r '.mode // ""' "$VERDICT_DIR/${bead}.json" 2>/dev/null || true)"
    echo "pawl-pre-push: CONFIRMED verdict for bead=$bead head=${head:0:12} (mode=${_vmode:-?}) — push authorized" >&2
    if [[ "$_vmode" == "fresh-context" ]]; then
      echo "pawl-pre-push: NOTE — fresh-context tier (a SINGLE family, weaker than the cross-family gate); add codex or agy for a multi-model verdict." >&2
    fi
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

  # A push is in flight from here on: mark it for every child so no descendant
  # (pawl-verdict.sh write/rebind via any review routed under this gate) ever
  # creates a commit mid-push — the auto-bind parks the ledger row instead
  # (age-wedge-all-in-dyr0.3 re-entrancy; see scripts/pawl-land.sh for why a
  # commit here desyncs the local_sha git already selected for the push).
  export PAWL_PREPUSH=1

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

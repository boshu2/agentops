#!/usr/bin/env bash
# check-pawl-pre-push.sh — cross-family pawl gate for push-to-main (age-58o).
#
# Reads git pre-push hook stdin (local_ref local_sha remote_ref remote_sha).
# When pushing to refs/heads/main, requires a CONFIRMED, commit-bound pawl
# verdict via scripts/pawl-verdict.sh check (pr=0 for push-to-main landings).
#
# #trivial-tip range check (age-8ais) — closes a real escape (age-rk3r.5,
# 2026-07-02): the #trivial provenance-only waiver historically keyed on the
# TIP commit ALONE, so a NON-trivial commit pushed BEHIND a #trivial tip escaped
# every gate. The cockpit `ao gate check --fast --scope head` scopes to the
# tip's changed files (docs/provenance/ only for a #trivial tip), so its
# build/test/ratchet checks never fired on the hidden commit's cli/** files, and
# a test-isolation ratchet-red landed on main and blocked every later cli/**
# landing until fixed forward (eef2a679f). Closed here: when the tip is
# #trivial-waived but the pushed range remote_sha..local_sha carries non-trivial
# commits, the cockpit gate is RE-TARGETED at EVERY such commit — newest-first,
# FAIL-FAST on the first failure — each run inside a throwaway DETACHED worktree
# whose HEAD *is* that commit, so `ao gate check --scope head` evaluates ITS
# files (the exact worktree-isolation pattern of verify-pushed-commit-builds.sh,
# age-yy24). Gating only the NEWEST non-trivial commit was REFUTED by a
# cross-family counterexample: landing trains routinely push
# [feat1, #trivial-bind, feat2, #trivial-bind] in one push, and a violation in
# feat1 with a clean feat2 escaped a newest-only check — so ALL of them gate.
#   • LATENCY: a MIXED push (non-trivial commits + #trivial tip) runs a real
#     `ao gate check` PER non-trivial commit (~40s each; ranges here are
#     normally 1-4 substantive commits) — deliberately slower; that is the
#     point, not a regression. age-wy2t is landing `ao gate check --scope range
#     <base>..<head>`; once that is on main, a follow-up can collapse this loop
#     into ONE range-scoped run — deliberately NOT depended on here (not on
#     main yet; this change is self-contained).
#   • FAST PATH PRESERVED: a PURE-#trivial range (every commit provenance-only
#     #trivial) is byte-identical to before — no range gate, no worktree.
#   • A non-trivial TIP is unchanged (normal pawl-verdict requirement below).
# Test seam: AGENTOPS_PREPUSH_GATE_CMD overrides the re-targeted gate command;
# AO_BIN overrides ao resolution.
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

# SINGLE-IMPLEMENTATION (age-wedge-all-in-dyr0.9): the #trivial provenance-only
# waiver lives in scripts/lib/trivial-waiver.sh, shared with the CI verdict
# backstop (scripts/check-tip-verdict-ci.sh). Never re-inline it here.
# shellcheck source=lib/trivial-waiver.sh
if ! source "$SCRIPT_DIR/lib/trivial-waiver.sh"; then
  die "shared waiver lib missing/unreadable at $SCRIPT_DIR/lib/trivial-waiver.sh"
fi
declare -F pawl_trivial_waiver >/dev/null || die "pawl_trivial_waiver not defined by $SCRIPT_DIR/lib/trivial-waiver.sh"

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

# _scrub_git_env — drop git's hook-injected discovery vars (age-ngtc). git runs
# pre-push hooks with GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE/... pointing at the
# PUSHING gitdir (and at a linked-worktree gitdir when the push originates from
# one), which would make the worktree ops and the re-targeted gate below hit the
# wrong repo. Call inside each git subshell (idempotent; unset of an unset var is
# a no-op even under set -u).
_scrub_git_env() {
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE
}

# resolve_ao — print a runnable ao (binary or PATH command), or return 1.
# Order: AO_BIN override → the repo's built binary → ao on PATH. A MIXED push
# (non-trivial commit behind a #trivial tip) that cannot resolve ao is
# fail-closed by the caller — we cannot verify the hidden commit — matching the
# CI backstop's --enforce posture.
resolve_ao() {
  if [[ -n "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
  if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then printf '%s\n' "$REPO_ROOT/cli/bin/ao"; return 0; fi
  if command -v ao >/dev/null 2>&1; then printf 'ao\n'; return 0; fi
  return 1
}

# nontrivial_commits_in_range <remote_sha> <local_sha>
#
# Print (newest-first, one per line) EVERY commit in the pushed range
# remote_sha..local_sha that the shared #trivial waiver does NOT waive
# (pawl_trivial_waiver returns non-zero: a real feature/fix commit, or a
# #trivial commit whose diff is not provenance-only). Print NOTHING when EVERY
# commit in the range is a #trivial provenance-only commit (the pure-trivial
# fast path). ALL non-trivial commits are reported — not just the newest: a
# cross-family counterexample proved a violation in an OLDER substantive commit
# escapes when only the newest is gated (the [feat1, #trivial, feat2, #trivial]
# landing-train shape). Classification per commit is cheap (git log +
# diff-tree); the expensive gating is the caller's. Returns non-zero ONLY when
# the range cannot be enumerated (caller fail-closes).
#
# remote_sha all-zeros / force-push / base-not-present-locally falls back to the
# TIP commit only — the same single-commit surface the CI backstop
# (check-tip-verdict-ci.sh) uses when the base is unavailable; a genuine
# push-to-main always supplies the real remote sha, giving the full bounded range.
nontrivial_commits_in_range() {
  local remote_sha="$1" local_sha="$2" commits sha wrc
  if [[ "$remote_sha" == "0000000000000000000000000000000000000000" ]] \
     || ! git -C "$GIT_REPO" rev-parse --verify --quiet "${remote_sha}^{commit}" >/dev/null 2>&1; then
    commits="$local_sha"
  elif ! commits="$(git -C "$GIT_REPO" rev-list "${remote_sha}..${local_sha}" 2>/dev/null)"; then
    return 1
  fi
  while IFS= read -r sha; do
    [[ -n "$sha" ]] || continue
    wrc=0
    pawl_trivial_waiver "$GIT_REPO" "$sha" "pawl-pre-push" >/dev/null 2>&1 || wrc=$?
    [[ "$wrc" -ne 0 ]] && printf '%s\n' "$sha"
  done <<<"$commits"
  return 0
}

# gate_nontrivial_commit <commit>
#
# Run the cockpit gate (ao gate check --fast --scope head) RE-TARGETED at
# <commit>. Because --scope head reads `git show --name-only HEAD`, the only
# faithful re-target is to run it where HEAD *is* <commit>: a throwaway DETACHED
# worktree at <commit>, git discovery-env scrubbed, cleaned up after (mirrors
# verify-pushed-commit-builds.sh, age-yy24). Returns the gate's exit status.
# Fail-closed (1) when the gate binary or worktree cannot be set up. Tests inject
# AGENTOPS_PREPUSH_GATE_CMD to stand in for the real gate.
gate_nontrivial_commit() {
  local commit="$1" gate_cmd tmpwt rc=0 ao
  gate_cmd="${AGENTOPS_PREPUSH_GATE_CMD:-}"
  if [[ -z "$gate_cmd" ]]; then
    if ! ao="$(resolve_ao)"; then
      echo "PAWL-HOLD: non-trivial commit ${commit:0:12} is hidden behind a #trivial tip but no ao binary is resolvable to gate it — fail-closed (set AO_BIN or build cli/bin/ao) (age-8ais)" >&2
      return 1
    fi
    gate_cmd="$ao gate check --fast --scope head"
  fi
  if ! tmpwt="$(mktemp -d "${TMPDIR:-/tmp}/agentops-prepush-retarget.XXXXXX" 2>/dev/null)"; then
    echo "PAWL-HOLD: cannot allocate a worktree to gate ${commit:0:12} — fail-closed (age-8ais)" >&2
    return 1
  fi
  if ! ( _scrub_git_env
         git -C "$GIT_REPO" worktree add --detach --quiet "$tmpwt" "$commit" ) 2>/dev/null; then
    echo "PAWL-HOLD: cannot create a detached worktree at ${commit:0:12} to re-target the cockpit gate — fail-closed (age-8ais)" >&2
    rm -rf "$tmpwt" 2>/dev/null || true
    return 1
  fi
  ( _scrub_git_env
    cd "$tmpwt" && eval "$gate_cmd" )
  rc=$?
  ( _scrub_git_env
    git -C "$GIT_REPO" worktree remove --force "$tmpwt" >/dev/null 2>&1
    git -C "$GIT_REPO" worktree prune >/dev/null 2>&1 ) || true
  rm -rf "$tmpwt" 2>/dev/null || true
  return "$rc"
}

check_one_push() {
  local local_ref="$1" local_sha="$2" remote_ref="$3" remote_sha="$4"

  is_main_push "$remote_ref" || return 0
  is_delete_push "$local_sha" && return 0

  local bead head
  head="$local_sha"
  # #trivial provenance-only waiver — SHARED implementation (age-w2ny marker
  # detection + age-u43w diff verification) in scripts/lib/trivial-waiver.sh,
  # also exercised by the CI verdict backstop so the two surfaces cannot drift
  # (age-wedge-all-in-dyr0.9).
  local waiver_rc=0
  pawl_trivial_waiver "$GIT_REPO" "$head" "pawl-pre-push" || waiver_rc=$?
  case "$waiver_rc" in
    0)
      # Tip is a #trivial provenance-only commit, so its OWN pawl-verdict check
      # is waived (unchanged). BUT the waiver keys on the TIP — non-trivial
      # commits sitting BEHIND it escaped the cockpit gate (age-8ais /
      # age-rk3r.5). RE-TARGET the cockpit gate at EVERY non-trivial commit in
      # the pushed range, newest-first, FAIL-FAST on the first failure (a
      # newest-only check was refuted: a violation in an OLDER substantive
      # commit with a clean newer one escaped). A pure-#trivial range takes the
      # byte-identical fast path.
      local nts nt gated=0
      if ! nts="$(nontrivial_commits_in_range "$remote_sha" "$local_sha")"; then
        echo "PAWL-HOLD: #trivial tip ${local_sha:0:12} — could not enumerate the push range to check for hidden non-trivial commits — fail-closed (age-8ais)" >&2
        return 1
      fi
      if [[ -z "$nts" ]]; then
        return 0   # pure-#trivial range: byte-identical fast path (load-bearing for throughput)
      fi
      # F10 (age-pawl-intent-zhndq.11): progress breadcrumbs. A mixed push runs a full
      # ~40s cockpit gate PER non-trivial commit in a detached worktree — a multi-minute,
      # otherwise-silent hang. Print "gating commit K of N: <sha> <subject>" before each and
      # a per-commit elapsed after, so the operator sees a legible progress log, not a stall.
      local _nt_total; _nt_total="$(printf '%s\n' "$nts" | grep -c .)"
      while IFS= read -r nt; do
        [[ -n "$nt" ]] || continue
        gated=$((gated + 1))
        local _nt_subj _nt_t0 _nt_elapsed
        _nt_subj="$(git -C "$GIT_REPO" log -1 --format=%s "$nt" 2>/dev/null)"
        _nt_t0="$(date +%s)"
        echo "pawl-pre-push: gating commit ${gated} of ${_nt_total}: ${nt:0:12} ${_nt_subj} — re-targeting the cockpit gate (age-8ais; ~40s each, by design)" >&2
        if ! gate_nontrivial_commit "$nt"; then
          echo "PAWL-HOLD: cockpit gate FAILED for non-trivial commit ${nt:0:12} hidden behind #trivial tip ${local_sha:0:12} — push refused (age-8ais)" >&2
          return 1
        fi
        _nt_elapsed=$(( $(date +%s) - _nt_t0 ))
        echo "pawl-pre-push: cockpit gate PASSED for non-trivial commit ${nt:0:12} (commit ${gated} of ${_nt_total}, ${_nt_elapsed}s) (age-8ais)" >&2
      done <<<"$nts"
      echo "pawl-pre-push: cockpit gate PASSED for all ${gated} non-trivial commit(s) behind #trivial tip ${local_sha:0:12} — push authorized (age-8ais)" >&2
      return 0
      ;;
    1) return 1 ;;   # fail-closed: marker present but triviality unprovable
    *) : ;;          # refused (2) or no marker (3): fall through to the normal pawl requirement
  esac

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

#!/usr/bin/env bash
# shellcheck shell=bash
# pawl-amend-guard.sh — sourceable: detect the amend-into-#trivial-bind trap
# (age-verification-economics-ebec.11).
#
# THE trap (fired ~4x on 2026-07-07, each costing a wasted review round): the pawl
# review auto-binds a single #trivial provenance commit on top of the reviewed feat.
# If the operator then `git commit --amend`s a fix, it folds INTO that #trivial bind
# — so HEAD becomes a #trivial-marked commit that ALSO carries code. The reviewer's
# trivial-waiver walk then steps PAST the #trivial tip to the ORIGINAL feat (missing
# the just-amended fix) and reviews stale content; the verdict binds the wrong tree;
# the mess only surfaces as an opaque "hidden non-trivial commit" gate failure at
# push. This guard catches the state EARLY — before the wasted review — with a clear
# recovery, converting a silent operator-error class into a loud, actionable refusal.
#
# pawl_amend_guard <repo> <sha>
#   returns 0  -> safe (not the trap): proceed.
#   returns 2  -> HEAD is #trivial-marked but changes NON-provenance files (the trap
#                 OR a mislabeled real change #trivial — both un-landable) — prints the
#                 diagnosis + recovery to stderr; caller should refuse.
# Opt out: PAWL_NO_AMEND_GUARD=1 (returns 0 without checking).
pawl_amend_guard() {
  local repo="${1:?repo}" sha="${2:?sha}" subject files nonprov
  [[ "${PAWL_NO_AMEND_GUARD:-0}" == "1" ]] && return 0

  subject="$(git -C "$repo" log -1 --format=%s "$sha" 2>/dev/null)" || return 0
  # #trivial only as a TRAILING marker (mirrors trivial-waiver's anchor — a prose
  # mention mid-subject is not a waiver claim and not the trap).
  grep -qiE '(^|[[:space:]])#trivial[[:space:]]*$' <<<"$subject" || return 0

  files="$(git -C "$repo" diff-tree --no-commit-id --no-renames --name-only -r "$sha" 2>/dev/null)" || return 0
  [[ -n "$files" ]] || return 0
  nonprov="$(grep -vE '^docs/provenance/' <<<"$files" || true)"
  [[ -n "$nonprov" ]] || return 0   # provenance-only #trivial = a legitimate bind, safe

  local shortsha; shortsha="$(git -C "$repo" rev-parse --short "$sha" 2>/dev/null || echo "$sha")"
  {
    echo "=== PAWL-AMEND-GUARD: HEAD (${shortsha}) is #trivial-marked but changes non-provenance files. ==="
    echo "This is the amend-into-#trivial-bind trap: a fix was amended INTO the auto-bind"
    echo "commit (or a real change was mislabeled #trivial). The reviewer would walk PAST this"
    echo "#trivial tip and review STALE content, and the verdict would bind the wrong tree."
    echo "Non-provenance files in this #trivial commit:"
    while IFS= read -r _f; do [[ -n "$_f" ]] && echo "  $_f"; done <<<"$nonprov"
    echo "Recover — rebuild the bead's code as ONE feat commit (rebuild, don't amend a bind):"
    echo "  git reset --soft <feat-base>            # the commit just BEFORE your feat"
    echo "  git checkout <feat-base> -- docs/provenance/ledger.jsonl   # keep the ledger append-only"
    echo "  git commit -C <your-feat-sha>           # reuse the feat's message"
    echo "then re-run pawl-review. (Opt out: PAWL_NO_AMEND_GUARD=1.)"
    echo "=== end amend-guard ==="
  } >&2
  return 2
}

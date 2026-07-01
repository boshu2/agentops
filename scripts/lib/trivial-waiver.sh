#!/usr/bin/env bash
# lib/trivial-waiver.sh — the SINGLE implementation of the #trivial
# provenance-only waiver (age-w2ny marker detection + age-u43w diff
# verification), extracted verbatim from scripts/check-pawl-pre-push.sh
# (age-wedge-all-in-dyr0.9) so the pre-push pawl gate and the CI verdict
# backstop (scripts/check-tip-verdict-ci.sh) share ONE code path and can
# never drift. Do NOT copy this logic anywhere — source this file and call
# pawl_trivial_waiver.
#
# shellcheck shell=bash

# pawl_trivial_waiver <git_repo> <sha> [label]
#
# Decide whether commit <sha> in <git_repo> is waived from the cross-family
# pawl as a #trivial provenance-only commit. <label> prefixes the "waived"
# message (default: pawl-pre-push, preserving the historical byte-identical
# pre-push output).
#
# Return codes:
#   0  waived   — explicit #trivial marker AND every changed file under
#                 docs/provenance/ (message: "... pawl waived")
#   1  fail-closed — #trivial marker present but triviality is unprovable
#                 (diff-tree failed or empty file list); caller must HOLD
#   2  refused  — #trivial marker present but the diff touches non-provenance
#                 path(s); caller must fall through to the normal pawl path
#   3  no marker — commit does not carry an explicit #trivial marker; caller
#                 proceeds to the normal pawl path (nothing emitted)
pawl_trivial_waiver() {
  local git_repo="$1" head="$2" label="${3:-pawl-pre-push}"
  local subject body

  # age-w2ny: waive ONLY when #trivial is an explicit marker — a TRAILING tag at
  # the END of the subject line (the established convention, e.g.
  # "chore(...): ... #trivial") or a standalone trailer line in the body. A
  # #trivial merely MENTIONED in prose — anywhere in the body, OR mid-subject
  # (e.g. "fix(pawl): prevent #trivial from bypassing pawl") — must NOT waive the
  # cross-family pawl. That was a fail-open: any non-trivial commit could bypass
  # the gate by naming #trivial (cross-family REFUTE: the original subject anchor
  # still waived mid-subject prose mentions).
  subject="$(git -C "$git_repo" log -1 --format=%s "$head" 2>/dev/null || true)"
  body="$(git -C "$git_repo" log -1 --format=%b "$head" 2>/dev/null || true)"
  if ! grep -qiE '(^|[[:space:]])#trivial[[:space:]]*$' <<<"$subject" \
     && ! grep -qiE '^[[:space:]]*#trivial[[:space:]]*$' <<<"$body"; then
    return 3
  fi

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
  if ! changed="$(git -C "$git_repo" diff-tree --no-commit-id --no-renames --name-only -r "$head" 2>/dev/null)"; then
    echo "PAWL-HOLD: #trivial at ${head:0:12} — diff-tree failed; cannot prove triviality — fail-closed, pawl required" >&2
    return 1
  fi
  if [[ -z "$changed" ]]; then
    echo "PAWL-HOLD: #trivial at ${head:0:12} has an empty changed-file list — cannot prove triviality — fail-closed, pawl required" >&2
    return 1
  fi
  nontrivial="$(grep -vE '^docs/provenance/' <<<"$changed" || true)"
  if [[ -z "$nontrivial" ]]; then
    echo "$label: #trivial commit at ${head:0:12} (provenance-ledger only) — pawl waived" >&2
    return 0
  fi
  echo "PAWL-HOLD: #trivial at ${head:0:12} touches non-trivial path(s) — waiver REFUSED, cross-family pawl still required:" >&2
  while IFS= read -r _f; do [[ -n "$_f" ]] && echo "  $_f" >&2; done <<<"$nontrivial"
  return 2
}

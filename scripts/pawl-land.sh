#!/usr/bin/env bash
# pawl-land.sh — deterministic single-push land of a pawl-gated bead
# (age-standing-pawl-service-ml8.4).
#
# Makes a verdict-bound push to main succeed in ONE attempt with no manual fiddling.
#
# SHAPE (post age-fkps): a land is `[feat, #trivial-bind]` — the reviewed feat commit with a
# SINGLE #trivial provenance commit (the pawl review's `pawl-verdict write` auto-bind,
# age-wedge-all-in-dyr0.3) on top as the pushed tip. The verdict binds the FEAT commit, NOT
# the #trivial bind — matching production exactly (verified: age-kg5l's verdict edge derives
# from the feat a291bef6, not the #trivial bind df93cf58b). The tip is #trivial-waived by the
# pre-push gate and the feat behind it is re-gated by the mixed-range cockpit gate (age-8ais).
#
# WHAT THIS WRAPPER DOES:
#   1. fetch + rebase onto current origin/main (the catch-22 fix: origin may have advanced
#      after the review wrote the verdict but before this push);
#   2. RESTAMP the verdict onto the post-rebase FEAT commit — under PAWL_AUTOBIND=0 so it does
#      NOT commit a SECOND #trivial. The review already auto-bound the single provenance
#      commit; a rebind auto-bind here would land a redundant 2nd #trivial with the verdict
#      pointing at *it* instead of the feat (the age-fkps double-#trivial bug: verdict.head_sha
#      != pushed tip, the whole thing surviving only on the #trivial waiver — defeating the
#      verdict-match invariant this wrapper exists to guarantee);
#   3. single-shot push of HEAD.
#
# The reviewed feat commit is HEAD's parent when HEAD is the #trivial auto-bind tip, else HEAD
# itself (when no auto-bind ran — e.g. an idempotent emit that did not append to the ledger).
#
# Preconditions: the bead's code is already committed (HEAD cites the bead) and a CONFIRMED
# pawl verdict exists at .agents/pawl-verdicts/<bead>.json (e.g. via `pawl.sh route`).
#
# Usage: pawl-land.sh <bead> [pr]    (pr default 0 = push-to-main)
set -euo pipefail

BEAD="${1:?usage: pawl-land.sh <bead> [pr]}"
PR="${2:-0}"
ROOT="$(git rev-parse --show-toplevel)"
VDIR="${AGENTOPS_PAWL_VERDICTS_DIR:-$ROOT/.agents/pawl-verdicts}"
VF="$VDIR/$BEAD.json"

die() { echo "pawl-land: $*" >&2; exit 1; }

# _tip_is_autobind <repo> <sha>: true when <sha> matches BOTH halves of the pawl auto-bind
# signature — the #trivial marker in the subject AND every changed file under docs/provenance/
# (the same dual test the trivial-waiver uses). Files alone are NOT sufficient: a legitimate
# reviewed feat can be provenance-only without being a bind (a deliberate ledger re-seal —
# the provenance.chain repair path), and misclassifying it would rebind the verdict to its
# PARENT (cross-family refute, age-fkps landing). Fail-safe: an empty/failed file list or a
# missing marker => NOT the auto-bind (treat HEAD as the feat), never mis-attribute the tip.
_tip_is_autobind() {
  local repo="$1" sha="$2" files subject
  subject="$(git -C "$repo" log -1 --format=%s "$sha" 2>/dev/null)" || return 1
  case "$subject" in *"#trivial"*) : ;; *) return 1 ;; esac
  files="$(git -C "$repo" diff-tree --no-commit-id --no-renames --name-only -r "$sha" 2>/dev/null)" || return 1
  [[ -n "$files" ]] || return 1
  ! grep -qvE '^docs/provenance/' <<<"$files"
}

[[ -f "$VF" ]] || die "no pawl verdict at $VF — route the bead through pawl.sh first"
jq -e '.disposition == "CONFIRMED"' "$VF" >/dev/null 2>&1 || die "$VF is not CONFIRMED — not landable"

# The HEAD that will be pushed must cite the bead (the gate resolves the bead from the
# HEAD commit message); confirm before we bother rebinding.
HEAD_SHA="$(git -C "$ROOT" rev-parse HEAD)"
HEAD_MSG="$(git -C "$ROOT" log -1 --format=%B "$HEAD_SHA")"
case "$HEAD_MSG" in
  *"$BEAD"*) : ;;
  *) die "HEAD ($(git -C "$ROOT" rev-parse --short HEAD)) does not cite $BEAD — commit the bead's code as HEAD first" ;;
esac

git -C "$ROOT" fetch origin main --quiet
if ! git -C "$ROOT" rebase origin/main; then
  git -C "$ROOT" rebase --abort >/dev/null 2>&1 || true
  die "rebase onto origin/main failed; aborted without pushing. Resolve the conflict locally, rerun pawl review if the tree changes, then rerun pawl-land."
fi
HEAD_SHA="$(git -C "$ROOT" rev-parse HEAD)"

# Resolve the reviewed FEAT commit to bind the verdict to. The pawl review's `pawl-verdict
# write` auto-binds a single #trivial provenance commit on top of the feat, so after the
# rebase HEAD is that #trivial tip and the reviewed feat is its parent. When no auto-bind ran,
# HEAD *is* the feat. Bind the verdict to the feat, matching production (age-fkps).
CODE_SHA="$HEAD_SHA"
if _tip_is_autobind "$ROOT" "$HEAD_SHA"; then
  parent="$(git -C "$ROOT" rev-parse --verify --quiet "${HEAD_SHA}^" || true)"
  [[ -n "$parent" ]] && CODE_SHA="$parent"
fi

# Restamp the already-reviewed verdict onto the post-rebase feat commit under PAWL_AUTOBIND=0
# — do NOT auto-bind a SECOND #trivial (the age-fkps double-#trivial bug). The review already
# auto-bound the single provenance commit; the restamp only follows the feat across the rebase
# so a concurrent origin advance (the catch-22) still lands the reviewed commit's verdict.
# Skip entirely when the verdict already binds the feat (the common no-op-rebase case): a
# same-sha rebind would only re-emit a duplicate ledger row that PAWL_AUTOBIND=0 leaves
# uncommitted in the working tree (swept, with the age-7krl foreign-row warning, into the
# NEXT land's bind commit — avoid creating that noise when nothing moved).
BOUND_SHA="$(jq -r '.head_sha // ""' "$VF" 2>/dev/null || true)"
if [[ "$BOUND_SHA" == "$CODE_SHA" ]]; then
  echo "pawl-land: verdict already bound to feat ${CODE_SHA:0:12} — no rebind needed" >&2
else
  LEDGER_REL="docs/provenance/ledger.jsonl"
  ledger_was_clean=0
  git -C "$ROOT" diff --quiet HEAD -- "$LEDGER_REL" 2>/dev/null && ledger_was_clean=1
  PAWL_AUTOBIND=0 bash "$ROOT/scripts/pawl-verdict.sh" rebind "$BEAD" "$PR" --head "$CODE_SHA" --dir "$VDIR"
  # The rebind re-emits a verdict edge for the post-rebase feat, which PAWL_AUTOBIND=0
  # leaves as an UNCOMMITTED ledger row. That dirt breaks the land lane's next
  # queued-branch checkout (git refuses to overwrite a modified ledger.jsonl,
  # dead-lettering the next bead — caught by the e2e catch-22 case). Restore the
  # ledger to HEAD when THIS rebind created the only dirt: the landed binding lives
  # in the verdict FILE (which the gate + ao done read), the committed pre-rebase
  # edge remains as review-time provenance, and the trunk-bound edge for the landed
  # sha is post-land reconciliation's job (scripts/post-land-provenance-emit.sh).
  # Pre-existing ledger dirt (another lane's leftovers) is never touched.
  if [[ "$ledger_was_clean" -eq 1 ]] && ! git -C "$ROOT" diff --quiet HEAD -- "$LEDGER_REL" 2>/dev/null; then
    git -C "$ROOT" checkout HEAD -- "$LEDGER_REL" 2>/dev/null \
      || echo "pawl-land: WARNING — could not restore $LEDGER_REL after rebind; the lane's next checkout may refuse" >&2
  fi
fi

# Single-shot push of HEAD (the #trivial provenance tip in the auto-bind shape, or the feat
# itself when no auto-bind ran). A #trivial tip is #trivial-waived and the feat behind it is
# re-gated by the mixed-range cockpit gate (age-8ais); a feat tip is checked directly against
# the verdict. Run post-land-provenance-emit.sh after the push when ledger reconciliation of
# the landed range is needed.
echo "pawl-land: pushing $BEAD (tip ${HEAD_SHA:0:12}, verdict-bound ${CODE_SHA:0:12})" >&2
git -C "$ROOT" push origin HEAD:main
echo "pawl-land: LANDED $BEAD" >&2

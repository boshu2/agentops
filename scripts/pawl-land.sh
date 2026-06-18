#!/usr/bin/env bash
# pawl-land.sh — deterministic single-push land of a pawl-gated bead
# (age-standing-pawl-service-ml8.4).
#
# Makes a verdict-bound push to main succeed in ONE attempt with no manual fiddling.
#
# What actually causes the "stale verdict, push refused" symptom (analysed 2026-06-18):
# the pre-push pawl gate (scripts/check-pawl-pre-push.sh) checks ONE thing —
# verdict.head_sha == the head being pushed (the stable local_sha from git's pre-push
# stdin). It does NOT require the verdict's provenance edge to be committed in the ledger.
# So a push fails for exactly one reason: the verdict points at a DIFFERENT commit than
# HEAD. That happens when HEAD drifts after the verdict was written — most often because a
# prior push left an unpushed post-land-provenance-emit commit on the branch, or because
# the code was (re)committed after routing the review. The deterministic fix is therefore
# NOT to commit the ledger edge (that has no fixed point: rebinding to the new head emits a
# fresh edge for that head, which re-dirties the ledger forever). It is simply to rebind the
# already-reviewed verdict to the current HEAD immediately before pushing.
#
# The verdict's ledger edge is left to post-land-provenance-emit.sh, which emits it
# idempotently on the next push (the intended trunk-bound one-push-lag); the working-tree
# ledger churn that leaves is handled by rebase.autoStash (age-uqj).
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

# Rebind the already-reviewed verdict onto the exact commit being pushed. This is the whole
# fix: it makes verdict.head_sha == pushed head deterministically, regardless of any drift
# since the review ran.
bash "$ROOT/scripts/pawl-verdict.sh" rebind "$BEAD" "$PR" --head "$HEAD_SHA" --dir "$VDIR"

# Single-shot push. The pre-push pawl gate authorises on the first try because the verdict
# now matches the pushed head; post-land-provenance-emit handles the ledger edge afterward.
echo "pawl-land: pushing $BEAD (head ${HEAD_SHA:0:12})" >&2
git -C "$ROOT" push origin HEAD:main
echo "pawl-land: LANDED $BEAD" >&2

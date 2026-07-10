#!/usr/bin/env bash
# shellcheck shell=bash
# pawl-tip-coherence.sh — sourceable: refuse a review whose tip cites a different
# bead (age-pawl-bead-tip-coherence-wckn).
#
# THE class (fired 2026-07-09): `ao pawl review <bead> --scope head` reviews
# whatever the review target commit is. When that commit's message cites a
# DIFFERENT bead — e.g. the operator ran the review while a sibling bead's
# provenance bind sat at tip — the reviewer CONFIRMs the wrong diff and the
# emitted verdict binds the wrong tree, which the gate then accepts. The
# amend-guard catches #trivial tips carrying code (the inverse trap); this guard
# catches the bead/tip identity mismatch itself.
#
# pawl_tip_coherence <repo> <sha> <bead>
#   returns 0 -> coherent: the commit cites <bead> (or a dotted parent/child of
#                it), cites NO ids sharing <bead>'s prefix (mismatch cannot be
#                inferred), or the guard is opted out.
#   returns 2 -> positive mismatch: the commit cites ids with <bead>'s prefix and
#                none is <bead> or a dotted relative — prints the diagnosis to
#                stderr; caller should refuse before spending a review round.
# Opt out: PAWL_ALLOW_TIP_MISMATCH=1 (returns 0 without checking).
pawl_tip_coherence() {
  local repo="${1:?repo}" sha="${2:?sha}" bead="${3:?bead}" msg prefix cited id
  [[ "${PAWL_ALLOW_TIP_MISMATCH:-0}" == "1" ]] && return 0

  msg="$(git -C "$repo" log -1 --format=%B "$sha" 2>/dev/null)" || return 0
  prefix="${bead%%-*}"
  # A bead id without a prefix segment gives us nothing to match on.
  [[ -n "$prefix" && "$prefix" != "$bead" ]] || return 0

  # Only ids sharing the reviewed bead's prefix count as citations — matching
  # every hyphenated word would produce false mismatches on ordinary prose.
  # Trailing sentence punctuation is stripped; dotted child ids (age-x.1) keep
  # their dot because it is followed by an alnum, not end-of-token.
  cited="$(grep -oE "${prefix}-[a-z0-9][a-z0-9.-]*" <<<"$msg" | sed 's/[.,;:]*$//' | sort -u || true)"
  [[ -n "$cited" ]] || return 0

  while IFS= read -r id; do
    [[ -z "$id" ]] && continue
    if [[ "$id" == "$bead" || "$id" == "$bead".* || "$bead" == "$id".* ]]; then
      return 0
    fi
  done <<<"$cited"

  local shortsha
  shortsha="$(git -C "$repo" rev-parse --short "$sha" 2>/dev/null || echo "$sha")"
  {
    echo "=== PAWL-TIP-COHERENCE: review target ${shortsha} does not cite bead ${bead}. ==="
    echo "The commit message cites: $(tr '\n' ' ' <<<"$cited")"
    echo "Reviewing ${bead} against this tip would CONFIRM a diff that belongs to a"
    echo "different bead — the verdict would bind the wrong tree and the gate would"
    echo "accept it (the 2026-07-09 wrong-tree class)."
    echo "Recover: make ${bead}'s own commit the review target (cherry-pick or land"
    echo "order), or — only if the tip genuinely IS this bead's work under another"
    echo "id — re-run with PAWL_ALLOW_TIP_MISMATCH=1."
    echo "=== end tip-coherence ==="
  } >&2
  return 2
}

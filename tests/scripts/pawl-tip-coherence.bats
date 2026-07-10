#!/usr/bin/env bats
# pawl-tip-coherence.bats — bead-vs-tip coherence guard (age-pawl-bead-tip-coherence-wckn).
#
# The wrong-tree class (2026-07-09): `ao pawl review <bead> --scope head` reviews
# whatever the tip is; when the tip commit cites a DIFFERENT bead, the verdict
# binds the wrong tree and the gate accepts it. The guard refuses on a positive
# mismatch within the bead's own id prefix.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/lib/pawl-tip-coherence.sh"
  R="$BATS_TEST_TMPDIR/r"
  mkdir -p "$R"
  git init -q "$R"; git -C "$R" config user.email t@t.local; git -C "$R" config user.name t
  unset PAWL_ALLOW_TIP_MISMATCH
}

commit_with() {
  echo "$RANDOM" >> "$R/f.txt"; git -C "$R" add -A; git -C "$R" commit -qm "$1"
}

@test "tip cites the reviewed bead -> safe (0)" {
  commit_with "fix(x): real work (age-x1)"
  run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 0 ]
}

@test "tip cites a DIFFERENT same-prefix bead -> refuse (2) naming both" {
  commit_with "chore(provenance): bind pawl CONFIRMED verdict for age-other-bead #trivial"
  run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 2 ]
  [[ "$output" == *"age-x1"* ]]
  [[ "$output" == *"age-other-bead"* ]]
  [[ "$output" == *"wrong tree"* ]]
}

@test "tip cites no bead ids -> safe (0), mismatch cannot be inferred" {
  commit_with "docs: tidy readme wording"
  run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 0 ]
}

@test "foreign-prefix ids do not count as citations" {
  commit_with "fix(y): port change (cp-zz99)"
  run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 0 ]
}

@test "dotted child relations are coherent both ways" {
  commit_with "feat(x): slice one (age-epic.1)"
  run pawl_tip_coherence "$R" HEAD age-epic
  [ "$status" -eq 0 ]
  run pawl_tip_coherence "$R" HEAD age-epic.1
  [ "$status" -eq 0 ]
  commit_with "feat(x): whole epic (age-epic)"
  run pawl_tip_coherence "$R" HEAD age-epic.2
  [ "$status" -eq 0 ]
}

@test "trailing sentence punctuation is stripped from cited ids" {
  commit_with "fix(x): closes age-x1. done"
  run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 0 ]
}

@test "PAWL_ALLOW_TIP_MISMATCH=1 opts out" {
  commit_with "chore: bind for age-other-bead"
  PAWL_ALLOW_TIP_MISMATCH=1 run pawl_tip_coherence "$R" HEAD age-x1
  [ "$status" -eq 0 ]
}

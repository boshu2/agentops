#!/usr/bin/env bats
# Acceptance tests — canonicalize-bdd-foundry-workflow — EDGE CASES E1–E6.
# ATDD phase 2: executable definition of done. RED until the feature is built.
# Run: bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/

load helpers

teardown() {
  git -C "$REPO" worktree prune --expire now 2>/dev/null || true
}

@test "E1 source-already-v6-takes-highest" {
  snap="$(snapshot_file)"
  [ -n "$snap" ] || fail "no source snapshot (S9 prerequisite)"
  require_file "$CANON"
  require_file "$PLAN/candidate-sweep.md"
  vc="$(header_version "$CANON")"
  vs="$(header_version "$snap")"
  [ -n "$vc" ] || fail "canonical header version unparseable"
  [ "$vc" = "$vs" ] || fail "canonical v$vc != snapshot v$vs (S9 snapshot must be of the winner)"
  maxv="$(grep -Eo 'bdd-foundry v[0-9]+' "$PLAN/candidate-sweep.md" | grep -Eo '[0-9]+' | sort -n | tail -1)"
  [ -n "$maxv" ] || fail "candidate-sweep.md records no candidate header versions"
  [ "$vc" -ge "$maxv" ] || fail "canonical v$vc is lower than the highest swept candidate v$maxv — highest lineage must win"
  # NOTE (live ground fact 2026-06-12): ~/.claude/workflows/bdd-foundry.js is already v6,
  # so the v5 assumption in the frozen behaviors is superseded by this E1 branch.
  [ "$vc" -ge 6 ] || fail "canonical v$vc below the live v6 source observed at test-authoring time"
}

@test "E2 same-version-divergent-hand-merge-with-hunk-dispositions" {
  require_file "$PLAN/candidate-sweep.md"
  require_file "$CANON"
  if [ -f "$PLAN/reconciliation.diff" ]; then
    tbl="$(ls "$PLAN"/reconciliation*.md "$PLAN"/*disposition*.md 2>/dev/null | head -1)"
    [ -n "$tbl" ] || fail "reconciliation.diff exists but no companion hunk-disposition table file (reconciliation*.md / *disposition*.md)"
    grep -q 'kept' "$tbl" || fail "disposition table missing 'kept' category"
    grep -Eq 'superseded|rejected' "$tbl" || fail "disposition table missing superseded/rejected categories"
    grep -q '|' "$tbl" || fail "disposition table has no table rows"
    # canonical carries every lineage line of the snapshot claimant
    snap="$(snapshot_file)"
    [ -n "$snap" ] || fail "no source snapshot"
    while IFS= read -r l; do
      grep -qF "$l" "$CANON" || fail "lineage line lost in merge: $l"
    done < <(grep -E '^// v[0-9]+' "$snap")
    node --check "$CANON"
  else
    grep -Eq 'single v[0-9]+ source, no reconciliation needed' "$PLAN/candidate-sweep.md" \
      || fail "no reconciliation.diff AND candidate-sweep.md does not state the single-source vacuous case"
  fi
}

@test "E3 change-surface-disjoint-no-regen-tax" {
  load_evidence
  require_var ARC_BASE_SHA
  require_var LANDED_SHA
  require_var WIRING_FILES
  changed="$(git -C "$REPO" diff --name-only "$ARC_BASE_SHA" "$LANDED_SHA")"
  [ -n "$changed" ] || fail "no changed paths between $ARC_BASE_SHA and $LANDED_SHA"
  while IFS= read -r p; do
    case "$p" in
      skills/*|docs/contracts/*) fail "forbidden surface changed: $p" ;;
    esac
    case "$p" in
      "$CANON_REL") ;;
      "$PLAN_REL"/*) ;;
      *)
        printf ' %s ' $WIRING_FILES | grep -qF " $p " || fail "out-of-scope path changed: $p (not canonical, plan-dir, or WIRING_FILES)"
        ;;
    esac
  done <<< "$changed"
  command -v ao >/dev/null || fail "ao not on PATH — cockpit gate cannot run"
  gw="$BATS_TEST_TMPDIR/gate-wt"
  git -C "$REPO" worktree add --detach "$gw" "$LANDED_SHA" >/dev/null 2>&1 || fail "could not create gate worktree at $LANDED_SHA"
  run bash -c "cd '$gw' && ao gate check --fast --scope head"
  git -C "$REPO" worktree remove --force "$gw" 2>/dev/null || true
  [ "$status" -eq 0 ] || fail "cockpit gate failed at the landed SHA: $output"
  ! printf '%s' "$output" | grep -qiE 'context-map.*(regen|drift)|regenerate.*(skills|context-map)' \
    || fail "gate demands a skills/context-map regeneration (collateral regen tax)"
}

@test "E4 worktree-isolation-unconditional" {
  load_evidence
  require_var BEAD_ID
  require_var WORKTREE_PATH
  require_var WORK_BRANCH
  [[ "$(basename "$WORKTREE_PATH")" == "wt-$BEAD_ID"* ]] || fail "worktree path '$WORKTREE_PATH' does not follow wt-<bead-id>"
  [[ "$WORK_BRANCH" =~ ^(feat|fix|chore|docs|refactor)/${BEAD_ID}- ]] || fail "branch '$WORK_BRANCH' does not follow <type>/<bead-id>-…"
  require_file "$PLAN/pre-state.porcelain" "main-checkout dirty set captured at work start"
  now="$(git -C "$REPO" status --porcelain | grep -v "$PLAN_REL" | sort || true)"
  pre="$(grep -v "$PLAN_REL" "$PLAN/pre-state.porcelain" | sort || true)"
  [ "$now" = "$pre" ] || fail "main-checkout dirty set changed vs work-start pre-state (beyond plan-dir files). now=[$now] pre=[$pre]"
}

@test "E5 installed-local-edits-backed-up-or-refused" {
  load_evidence
  require_var INSTALL_CMD
  require_file "$CANON"
  fixhome="$BATS_TEST_TMPDIR/home-e5"
  copy_siblings_into "$fixhome"
  div="$fixhome/.claude/workflows/bdd-foundry.js"
  { printf '// uncaptured local v999 edit — must not be silently destroyed\n'; cat "$CANON"; } > "$div"
  prebytes="$BATS_TEST_TMPDIR/pre-bytes.js"
  cp "$div" "$prebytes"
  run run_cmd_in "$REPO" "$fixhome" "$INSTALL_CMD"
  if [ "$status" -ne 0 ]; then
    [[ "$output" == *"bdd-foundry.js"* ]] || fail "refusal message does not name the divergent path: $output"
  else
    backup="$(ls "$fixhome/.claude/workflows/"bdd-foundry.js.pre-canonicalize-* 2>/dev/null | head -1)"
    if [ -z "$backup" ]; then
      backup="$(printf '%s\n' "$output" | grep -Eo '/[^[:space:]]*bdd-foundry\.js[^[:space:]]*' | grep -v "^$div\$" | head -1)"
    fi
    { [ -n "$backup" ] && [ -f "$backup" ]; } || fail "install replaced a divergent local file with no backup (output: $output)"
    cmp -s "$backup" "$prebytes" || fail "backup is not byte-faithful to the pre-replacement file"
  fi
}

@test "E6 sibling-drift-scoped-blocking-report-only-elsewhere" {
  load_evidence
  require_var DRIFT_CHECK_CMD
  require_var SIBLING_DRIFT_BEAD_ID
  require_var ARC_BASE_SHA
  require_var LANDED_SHA
  require_file "$CANON"
  fixhome="$BATS_TEST_TMPDIR/home-e6"
  install_good_fixture "$fixhome"
  cp "$REPO/.claude/workflows/operating-loop.js" "$fixhome/.claude/workflows/"
  cp "$REPO/.claude/workflows/bead-crank.js" "$fixhome/.claude/workflows/"
  printf 'x' >> "$fixhome/.claude/workflows/bead-crank.js"
  run run_cmd_in "$REPO" "$fixhome" "$DRIFT_CHECK_CMD"
  [ "$status" -eq 0 ] || fail "sibling drift must be report-only, never an exit-code failure of THIS gate (exit $status): $output"
  [[ "$output" == *"bead-crank"* ]] || fail "sibling drift not reported in output: $output"
  # no arc commit touches sibling content
  [ -z "$(git -C "$REPO" rev-list "$ARC_BASE_SHA..$LANDED_SHA" -- .claude/workflows/bead-crank.js .claude/workflows/operating-loop.js)" ] \
    || fail "an arc commit modifies sibling workflow content"
  # remediation bead filed
  [[ "$SIBLING_DRIFT_BEAD_ID" =~ ^ag- ]] || fail "sibling remediation bead id '$SIBLING_DRIFT_BEAD_ID' does not match ^ag-"
  run bash -c "cd '$REPO' && BEADS_DIR=/Users/bo/dev/agentops/_beads br show '$SIBLING_DRIFT_BEAD_ID'"
  [ "$status" -eq 0 ] || fail "sibling remediation bead $SIBLING_DRIFT_BEAD_ID not found in tracker: $output"
}

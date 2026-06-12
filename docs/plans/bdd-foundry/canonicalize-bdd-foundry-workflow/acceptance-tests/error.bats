#!/usr/bin/env bats
# Acceptance tests — canonicalize-bdd-foundry-workflow — ERROR CASES X1–X5.
# ATDD phase 2: executable definition of done. RED until the feature is built.
# Run: bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/

load helpers

teardown() {
  git -C "$REPO" worktree prune --expire now 2>/dev/null || true
}

@test "X1 syntax-failure-blocks-the-copy" {
  load_evidence
  require_var ARC_BASE_SHA
  require_var LANDED_SHA
  commits="$(git -C "$REPO" rev-list "$ARC_BASE_SHA..$LANDED_SHA" -- "$CANON_REL")"
  [ -n "$commits" ] || fail "no arc commits touch $CANON_REL — nothing landed"
  for c in $commits; do
    git -C "$REPO" show "$c:$CANON_REL" > "$BATS_TEST_TMPDIR/cand-$c.js"
    node --check "$BATS_TEST_TMPDIR/cand-$c.js" || fail "commit $c committed a canonical candidate that fails node --check"
  done
}

@test "X2 missing-marker-blocks-the-push" {
  load_evidence
  require_var MARKER_CHECK_CMD
  require_file "$CANON"
  # real canonical passes
  run run_cmd_in "$REPO" "$HOME" "$MARKER_CHECK_CMD '$CANON'"
  [ "$status" -eq 0 ] || fail "marker verification fails on the real canonical file: $output"
  # mutation 1: drop the DRIFT_SCHEMA marker entirely
  t1="$BATS_TEST_TMPDIR/no-drift-schema.js"
  grep -v 'DRIFT_SCHEMA' "$CANON" > "$t1"
  run run_cmd_in "$REPO" "$HOME" "$MARKER_CHECK_CMD '$t1'"
  [ "$status" -ne 0 ] || fail "marker verification passed a candidate missing DRIFT_SCHEMA"
  [[ "$output" == *"DRIFT_SCHEMA"* ]] || fail "failure output does not name the missing marker (DRIFT_SCHEMA): $output"
  # mutation 2: drop the tracker-write guard chain (enforcement shape)
  t2="$BATS_TEST_TMPDIR/no-guard-chain.js"
  grep -vF 'cycleFree && uncovered.length === 0 && driftOk' "$CANON" > "$t2"
  run run_cmd_in "$REPO" "$HOME" "$MARKER_CHECK_CMD '$t2'"
  [ "$status" -ne 0 ] || fail "marker verification passed a candidate missing the tracker-write guard chain"
  printf '%s' "$output" | grep -qEi 'cycleFree|guard|tracker-write' || fail "failure output does not name the missing enforcement shape: $output"
}

@test "X3 no-live-run-and-no-law0-surface-anywhere" {
  load_evidence
  require_var ARC_BASE_SHA
  require_var LANDED_SHA
  le="$PLAN/landed-evidence.md"
  require_file "$le"
  pat='claude -p|claude --print|gemini -p'
  changed="$(git -C "$REPO" diff --name-only "$ARC_BASE_SHA" "$LANDED_SHA")"
  [ -n "$changed" ] || fail "no changed paths in the arc"
  while IFS= read -r p; do
    case "$p" in
      *.js|*.sh|scripts/*|bin/*)
        hits="$(git -C "$REPO" show "$LANDED_SHA:$p" 2>/dev/null | grep -nE "$pat" || true)"
        [ -z "$hits" ] && continue
        while IFS= read -r h; do
          ln="${h%%:*}"; line="${h#*:}"
          # C9.2 (frozen-X3: "comment/string-literal documenting the prohibition"): accept comment-prefixed
          # OR string-literal hits (a backtick/quote precedes the match); file:line evidence stays mandatory.
          printf '%s' "$line" | grep -qE '^[[:space:]]*(#|//|\*)' || printf '%s' "$line" | grep -qE "[\`'\"].*(claude -p|claude --print|gemini -p)" || fail "LAW-0 invocation in $p:$ln is executable, not a comment/string-literal: $line"
          grep -qF "$p:$ln" "$le" || fail "LAW-0 comment exception $p:$ln not listed file:line in landed-evidence.md"
        done <<< "$hits"
        ;;
    esac
  done <<< "$changed"
  # no recorded command executes the workflow (node without --check) or the Workflow tool
  bad="$(grep -E 'node .*bdd-foundry\.js' "$le" | grep -v -- '--check' || true)"
  [ -z "$bad" ] || fail "recorded command executes bdd-foundry.js (only node --check is allowed): $bad"
  ! grep -qiE 'workflow tool|invoke.*bdd-foundry.*run' "$le" || fail "evidence records a live workflow invocation"
}

@test "X4 dangling-or-misaimed-follow-fails" {
  load_evidence
  require_var DRIFT_CHECK_CMD
  require_var BLOCKING_PARENT_CMD
  fixhome="$BATS_TEST_TMPDIR/home-x4"
  copy_siblings_into "$fixhome"
  ln -s "$fixhome/does-not-exist-target.js" "$fixhome/.claude/workflows/bdd-foundry.js"
  run run_cmd_in "$REPO" "$fixhome" "$DRIFT_CHECK_CMD"
  [ "$status" -ne 0 ] || fail "drift/resolution check tolerated a dangling symlink"
  [[ "$output" == *"bdd-foundry.js"* ]] || fail "failure does not name the offending path: $output"
  run run_cmd_in "$REPO" "$fixhome" "$BLOCKING_PARENT_CMD"
  [ "$status" -ne 0 ] || fail "the named blocking parent tolerated the dangling follow (check only fails by hand)"
}

@test "X5 tracker-never-run-from-worktree" {
  load_evidence
  require_var WORKTREE_PATH
  if [ -d "$WORKTREE_PATH" ]; then
    [ ! -e "$WORKTREE_PATH/_beads" ] || fail "_beads/ forked into the worktree: $WORKTREE_PATH/_beads"
    [ ! -e "$WORKTREE_PATH/.beads" ] || fail ".beads/ appeared inside the worktree: $WORKTREE_PATH/.beads"
  fi
  # the ledger never lands in the public repo
  [ "$(git -C "$REPO" log --all --oneline -- _beads/ | wc -l | tr -d ' ')" -eq 0 ] || fail "_beads/ content committed to the PUBLIC repo"
  # the exact main-checkout invocation form is recorded in plan-dir evidence
  cat "$PLAN/candidate-sweep.md" "$PLAN/landed-evidence.md" 2>/dev/null | grep -qF 'BEADS_DIR=/Users/bo/dev/agentops/_beads br ' \
    || fail "no recorded tracker invocation with the exact 'BEADS_DIR=/Users/bo/dev/agentops/_beads br …' form in plan-dir evidence"
}

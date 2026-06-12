#!/usr/bin/env bats
# Acceptance tests — canonicalize-bdd-foundry-workflow — HAPPY PATH S1–S12.
# ATDD phase 2: executable definition of done. RED until the feature is built.
# Run: bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/

load helpers

teardown() {
  git -C "$REPO" worktree prune --expire now 2>/dev/null || true
}

@test "S1 canonical-file-tracked-at-house-path" {
  run git -C "$REPO" ls-files .claude/workflows/bdd-foundry.js
  [ "$status" -eq 0 ]
  [ "$output" = ".claude/workflows/bdd-foundry.js" ] || fail "bdd-foundry.js is not git-tracked at the house path (got: '$output')"
  run git -C "$REPO" ls-files .claude/workflows/
  [[ "$output" == *"bead-crank.js"* ]] || fail "sibling bead-crank.js missing from tracked workflows"
  [[ "$output" == *"operating-loop.js"* ]] || fail "sibling operating-loop.js missing from tracked workflows"
}

@test "S2 canonical-equals-immutable-snapshot-except-hazard-line" {
  snap="$(snapshot_file)"
  [ -n "$snap" ] || fail "no source-snapshot-*.js in plan dir (S9 prerequisite)"
  require_file "$CANON"
  removed="$(diff "$snap" "$CANON" | grep '^< ' | sed 's/^< //' || true)"
  added="$(diff "$snap" "$CANON" | grep '^> ' | sed 's/^> //' || true)"
  [ "$(printf '%s' "$removed" | grep -c '^' || true)" -eq 1 ] || fail "expected exactly 1 removed line (the HAZARD line); removed: [$removed]"
  printf '%s\n' "$removed" | grep -q '^// HAZARD: not git-tracked' || fail "the sole removed line is not the HAZARD line: [$removed]"
  [ "$(printf '%s' "$added" | grep -c '^' || true)" -eq 1 ] || fail "expected exactly 1 added (replacement) line; added: [$added]"
  vc="$(header_version "$CANON")"
  vs="$(header_version "$snap")"
  [ -n "$vc" ] || fail "canonical first line does not match 'bdd-foundry v<N>'"
  [ "$vc" = "$vs" ] || fail "header version mismatch: canonical=v$vc snapshot=v$vs"
  [ "$vc" -ge 5 ] || fail "canonical version v$vc below the v5 floor"
  [ "$(grep -c '^// v[2345]' "$CANON" || true)" -ge 4 ] || fail "lineage lines (// v2..v5) incomplete in canonical"
}

@test "S3 hazard-line-retired-and-replaced" {
  require_file "$CANON"
  ! grep -q "HAZARD: not git-tracked" "$CANON" || fail "HAZARD line still present in canonical"
  require_file "$INSTALLED"
  ! grep -q "HAZARD: not git-tracked" "$INSTALLED" || fail "HAZARD line still present in installed copy"
  matches="$(grep '^[[:space:]]*//' "$CANON" | grep -F '.claude/workflows/bdd-foundry.js' | grep -i 'canonical' | grep -F '~/.claude/workflows' || true)"
  [ "$(printf '%s' "$matches" | grep -c '^' || true)" -eq 1 ] || fail "expected exactly one header comment naming the canonical home AND the ~/.claude copy/symlink relation; got: [$matches]"
}

@test "S4 syntax-markers-and-enforcement-shapes" {
  require_file "$CANON"
  node --check "$CANON"
  [ "$(grep -c 'DRIFT_SCHEMA' "$CANON" || true)" -ge 2 ] || fail "DRIFT_SCHEMA marker < 2 (v4 drift-guard lost)"
  [ "$(grep -c 'beads\.json' "$CANON" || true)" -ge 3 ] || fail "beads.json plumbing < 3 (v3 file plumbing lost)"
  [ "$(grep -c 'DIR-MISAIM' "$CANON" || true)" -ge 2 ] || fail "DIR-MISAIM marker < 2 (v5 preflight lost)"
  [ "$(grep -c 'pre-run-N base snapshot' "$CANON" || true)" -ge 1 ] || fail "'pre-run-N base snapshot' marker missing (v5 base snapshot lost)"
  # C9.1 (frozen E1 re-grounding clause): v7-tolerant enforcement-shape grep — v7 replaced
  # includes('DIR-MISAIM') with startsWith('DIR-MISAIM') sentinel-slot checks; throw window widened.
  [ "$(grep -A6 -E "includes\('DIR-MISAIM'\)|startsWith\('DIR-MISAIM'\)" "$CANON" | grep -c 'throw' || true)" -ge 1 ] || fail "DIR-MISAIM preflight no longer THROWS (comment fossil)"
  [ "$(grep -Fc 'cycleFree && uncovered.length === 0 && driftOk' "$CANON" || true)" -ge 2 ] || fail "tracker-write guard chain < 2 sites"
  [ "$(grep -c 'gap_dispositions' "$CANON" || true)" -ge 2 ] || fail "gap_dispositions schema requirement < 2"
}

@test "S5 meta-block-pure-literal" {
  require_file "$CANON"
  block="$(sed -n '/export const meta = {/,/^}/p' "$CANON")"
  [ -n "$block" ] || fail "no 'export const meta = {' block found"
  ! printf '%s\n' "$block" | grep -qF '${' || fail "meta block contains \${} template interpolation"
  ! printf '%s\n' "$block" | grep -qE '(^|[^A-Za-z0-9_$.])(args|TRACKER|DIR|RUN_TAG)([^A-Za-z0-9_$-]|$)' \
    || fail "meta block references a runtime identifier (args/TRACKER/DIR/RUN_TAG as a bare token)"
}

@test "S6 installed-copy-mechanically-follows-via-named-command" {
  load_evidence
  require_var INSTALL_CMD
  require_var ADDED_SCRIPTS
  require_file "$CANON"
  [ -e "$INSTALLED" ] || [ -L "$INSTALLED" ] || fail "installed path missing: $INSTALLED"
  if [ -L "$INSTALLED" ]; then
    [ "$(resolve_path "$INSTALLED")" = "$(resolve_path "$CANON")" ] || fail "installed symlink does not resolve to the canonical file"
  else
    cmp -s "$INSTALLED" "$CANON" || fail "installed copy bytes differ from canonical (drift)"
    require_var DRIFT_CHECK_CMD
  fi
  # one pattern, no bdd-foundry special case: if the mechanism names bdd-foundry,
  # it must cover the siblings by the same pattern.
  all=""
  for f in $ADDED_SCRIPTS; do
    require_file "$REPO/$f" "ADDED_SCRIPTS entry"
    all+="$(cat "$REPO/$f")"$'\n'
  done
  if printf '%s' "$all" | grep -q 'bdd-foundry'; then
    printf '%s' "$all" | grep -q 'bead-crank' || fail "install mechanism special-cases bdd-foundry (bead-crank not covered)"
    printf '%s' "$all" | grep -q 'operating-loop' || fail "install mechanism special-cases bdd-foundry (operating-loop not covered)"
  fi
}

@test "S7 drift-check-fails-red-and-blocks" {
  load_evidence
  require_var DRIFT_CHECK_CMD
  require_var BLOCKING_PARENT_CMD
  require_file "$CANON"
  fix="$BATS_TEST_TMPDIR/home-s7"
  copy_siblings_into "$fix"
  cp "$CANON" "$fix/.claude/workflows/bdd-foundry.js"
  printf 'x' >> "$fix/.claude/workflows/bdd-foundry.js"
  run run_cmd_in "$REPO" "$fix" "$DRIFT_CHECK_CMD"
  [ "$status" -ne 0 ] || fail "drift check passed on a mutated installed copy"
  [[ "$output" == *"bdd-foundry.js"* ]] || fail "drift failure does not name bdd-foundry.js: $output"
  run run_cmd_in "$REPO" "$fix" "$BLOCKING_PARENT_CMD"
  [ "$status" -ne 0 ] || fail "blocking parent passed on the mutated fixture — the check is orphaned, not wired"
  run run_cmd_in "$REPO" "$HOME" "$DRIFT_CHECK_CMD"
  [ "$status" -eq 0 ] || fail "drift check fails on the real un-mutated pair: $output"
  run run_cmd_in "$REPO" "$HOME" "$BLOCKING_PARENT_CMD"
  [ "$status" -eq 0 ] || fail "blocking parent fails on the real pair: $output"
}

@test "S8 whole-arc-bead-cited-and-closed" {
  load_evidence
  require_var BEAD_ID
  require_var LANDED_SHA
  require_var ARC_BASE_SHA
  require_var WIRING_FILES
  [[ "$BEAD_ID" =~ ^ag- ]] || fail "bead id '$BEAD_ID' does not match ^ag-"
  # tracker read from the main checkout (never a worktree), exact invocation form
  [ "$(git -C "$REPO" rev-parse --git-dir)" = "$(git -C "$REPO" rev-parse --git-common-dir)" ] || fail "$REPO is a worktree, not the main checkout"
  run bash -c "cd '$REPO' && BEADS_DIR=/Users/bo/dev/agentops/_beads br show '$BEAD_ID'"
  [ "$status" -eq 0 ] || fail "br show $BEAD_ID failed: $output"
  printf '%s' "$output" | grep -qi 'closed' || fail "bead $BEAD_ID is not closed"
  printf '%s' "$output" | grep -q "${LANDED_SHA:0:7}" || fail "bead close note does not name the landed SHA ${LANDED_SHA:0:7}"
  # every arc commit touching the change surface cites the bead
  commits="$(git -C "$REPO" rev-list "$ARC_BASE_SHA..$LANDED_SHA" -- "$CANON_REL" "$PLAN_REL" $WIRING_FILES)"
  [ -n "$commits" ] || fail "no arc commits touch the change surface ($ARC_BASE_SHA..$LANDED_SHA)"
  for c in $commits; do
    git -C "$REPO" log -1 --format=%B "$c" | grep -qF "$BEAD_ID" || fail "arc commit $c does not cite $BEAD_ID"
  done
  # the private ledger never lands in the public repo
  [ "$(git -C "$REPO" log --all --oneline -- _beads/ | wc -l | tr -d ' ')" -eq 0 ] || fail "_beads/ content committed to the PUBLIC repo"
}

@test "S9 immutable-pre-write-source-snapshot" {
  snap="$(snapshot_file)"
  [ -n "$snap" ] || fail "no source-snapshot-<UTC-ts>.js in plan dir"
  base="$(basename "$snap")"
  [[ "$base" =~ ^source-snapshot-[0-9]{8}T[0-9]{6}Z\.js$ ]] || fail "snapshot filename must embed a UTC timestamp source-snapshot-YYYYMMDDTHHMMSSZ.js: $base"
  require_file "$PLAN/source-snapshot.sha256"
  (cd "$PLAN" && shasum -a 256 -c source-snapshot.sha256) || fail "snapshot fails its recorded SHA256 (edited post-hoc?)"
  ts="${base#source-snapshot-}"; ts="${ts%.js}"
  snap_epoch="$(date -j -u -f '%Y%m%dT%H%M%SZ' "$ts" +%s)"
  first_commit_epoch="$(git -C "$REPO" log --follow --format=%ct --reverse -- "$CANON_REL" | head -1)"
  [ -n "$first_commit_epoch" ] || fail "canonical file has no commits yet (nothing landed)"
  [ "$snap_epoch" -le "$first_commit_epoch" ] || fail "snapshot ($ts) postdates the first canonical commit — not pre-write"
}

@test "S10 candidate-sweep-recorded-before-winner" {
  f="$PLAN/candidate-sweep.md"
  require_file "$f"
  grep -q 'ls -la' "$f" || fail "sweep missing the ~/.claude/workflows listing (ls -la)"
  grep -q 'ls-files' "$f" || fail "sweep missing the repo ls-files probe"
  grep -q 'worktree list' "$f" || fail "sweep missing the per-worktree probe (git worktree list)"
  grep -q "$PLAN_REL" "$f" || fail "sweep missing the plan-dir *.js probe"
  grep -Eq '[0-9a-f]{64}' "$f" || fail "sweep lists no candidate SHA256"
  grep -Eq 'bdd-foundry v[0-9]+' "$f" || fail "sweep lists no candidate header version"
  [ "$(grep -o 'WINNER' "$f" | wc -l | tr -d ' ')" -eq 1 ] || fail "exactly one candidate must be marked WINNER"
  grep -Eqi 'highest lineage version wins|single v[0-9]+ source, no reconciliation needed' "$f" || fail "sweep does not state the selection rule applied"
}

@test "S11 clean-home-fixture-idempotent-portable" {
  load_evidence
  require_var INSTALL_CMD
  require_var ADDED_SCRIPTS
  require_var LANDED_SHA
  fixhome="$BATS_TEST_TMPDIR/home-s11"
  mkdir -p "$fixhome"
  [ ! -d "$fixhome/.claude/workflows" ]
  fixrepo="$BATS_TEST_TMPDIR/repo-s11"
  [[ "$fixrepo" != /Users/bo/dev/agentops* ]] || fail "fixture repo path must not be under /Users/bo/dev/agentops"
  git -C "$REPO" worktree add --detach "$fixrepo" "$LANDED_SHA" >/dev/null 2>&1 || fail "could not create fixture repo checkout at $fixrepo"
  run run_cmd_in "$fixrepo" "$fixhome" "$INSTALL_CMD"
  [ "$status" -eq 0 ] || fail "first install run failed in clean HOME: $output"
  inst="$fixhome/.claude/workflows/bdd-foundry.js"
  { [ -e "$inst" ] || [ -L "$inst" ]; } || fail "install did not create $inst (directory auto-creation failed)"
  if [ -L "$inst" ]; then
    [ "$(resolve_path "$inst")" = "$(resolve_path "$fixrepo/$CANON_REL")" ] || fail "fixture symlink resolves to '$(resolve_path "$inst")', not the FIXTURE repo canonical"
    [ ! -L "$(resolve_path "$inst")" ] || fail "nested symlink"
    state1="$(readlink "$inst")"
  else
    cmp -s "$inst" "$fixrepo/$CANON_REL" || fail "fixture copy differs from fixture canonical"
    state1="$(shasum -a 256 "$inst" | awk '{print $1}')"
  fi
  run run_cmd_in "$fixrepo" "$fixhome" "$INSTALL_CMD"
  [ "$status" -eq 0 ] || fail "second (idempotency) run failed: $output"
  if [ -L "$inst" ]; then state2="$(readlink "$inst")"; else state2="$(shasum -a 256 "$inst" | awk '{print $1}')"; fi
  [ "$state1" = "$state2" ] || fail "second run changed the follow state: '$state1' -> '$state2'"
  run bash -c "ls '$fixhome/.claude/workflows' | grep -E '\\.(bak|orig)\$|pre-canonicalize|~\$'"
  [ "$status" -ne 0 ] || fail "backup litter left in fixture home: $output"
  [ -z "$(git -C "$fixrepo" status --porcelain)" ] || fail "install dirtied the fixture repo: $(git -C "$fixrepo" status --porcelain)"
  for f in $ADDED_SCRIPTS; do
    ! grep -q '/Users/bo' "$REPO/$f" || fail "hardcoded /Users/bo path in $f: $(grep -n '/Users/bo' "$REPO/$f")"
  done
  git -C "$REPO" worktree remove --force "$fixrepo" 2>/dev/null || true
}

@test "S12 evidence-anchored-to-landed-head" {
  load_evidence
  require_var LANDED_SHA
  le="$PLAN/landed-evidence.md"
  require_file "$le"
  grep -q "$LANDED_SHA" "$le" || fail "landed-evidence.md does not record the landed HEAD SHA $LANDED_SHA"
  want="$(git -C "$REPO" show "$LANDED_SHA:$CANON_REL" | shasum -a 256 | awk '{print $1}')"
  grep -q "$want" "$le" || fail "landed-evidence.md hash does not match git show $LANDED_SHA:$CANON_REL ($want)"
  grep -Eqi 'readlink|cmp' "$le" || fail "landed-evidence.md records no readlink/cmp follow result"
  grep -Eqi 'gate' "$le" || fail "landed-evidence.md records no gate run"
  grep -Eqi 'exit(ed| code)?[ :=]+0' "$le" || fail "landed-evidence.md records no exit-0 for the gate command"
  git -C "$REPO" merge-base --is-ancestor "$LANDED_SHA" main || fail "$LANDED_SHA is not an ancestor-or-equal of main"
}

#!/usr/bin/env bats
# HIJACK GUARD (age-sylz). pawl-review.sh runs a cross-family review for MINUTES; a
# concurrent lane can `git reset` a SHARED landing worktree mid-review and the pawl would
# then bind a verdict labeled for one bead onto the OTHER lane's commit (a real incident
# 2026-07-02). The EXISTING stale-verdict check (verdict.head_sha vs --head) PASSES when both
# moved together, so it did not catch this. pawl-review snapshots HEAD at review start and
# exports it as PAWL_REVIEW_START_HEAD; pawl-verdict.sh `write` refuses to emit/bind the
# verdict edge when the LIVE worktree HEAD no longer equals that snapshot.
#
# `ao` is STUBBED via PATH (logs its args); `jq`/`git` are real; everything runs inside a
# temp git repo so the real repo + its ledger are never touched.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  AO_LOG="$TMP/ao.log"; : > "$AO_LOG"
  cat > "$BIN/ao" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$AO_LOG"
exit 0
EOF
  chmod +x "$BIN/ao"
  export PATH="$BIN:$PATH"
  VDIR="$TMP/verdicts"; mkdir -p "$VDIR"
  EV="$TMP/evidence.txt"; printf 'fresh-context review ran\n' > "$EV"
  # A real temp git repo whose HEAD the guard resolves via _ledger_root_from_cwd(cwd).
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > f.txt; git add f.txt; git commit --quiet -m init
  X="$(git rev-parse HEAD)"
  # Isolate the best-effort yield side-effect into the temp repo (never the real checkout).
  export AGENTOPS_REPO_ROOT="$REPO"
  SNAP_OTHER="deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "hijack guard: HEAD moved during review (snapshot != live HEAD) REFUSES, names BOTH shas, emits NO edge" {
  # cwd = the temp repo (its live HEAD is $X); the review-start snapshot is a DIFFERENT sha
  # (a concurrent lane reset the worktree mid-review) — the guard must refuse the bind.
  run env PAWL_REVIEW_START_HEAD="$SNAP_OTHER" \
    bash "$SCRIPT" write age-hijack 0 --disposition CONFIRMED --head "$X" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$EV" --dir "$VDIR"
  [ "$status" -ne 0 ]
  [[ "$output" == *"worktree HEAD moved during"* ]]
  [[ "$output" == *"concurrent-lane hijack"* ]]
  [[ "$output" == *"$SNAP_OTHER"* ]]   # the review-start snapshot is named
  [[ "$output" == *"$X"* ]]            # the live bind-time HEAD is named
  # The verdict edge was NOT emitted/bound: the guard aborts BEFORE any `ao` call.
  [ ! -s "$AO_LOG" ]
}

@test "hijack guard: HEAD stable (snapshot == live HEAD) BINDS the verdict (exit 0)" {
  run env PAWL_REVIEW_START_HEAD="$X" \
    bash "$SCRIPT" write age-stable 0 --disposition CONFIRMED --head "$X" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$EV" --dir "$VDIR"
  [ "$status" -eq 0 ]
  [ -f "$VDIR/age-stable.json" ]
  grep -q '"disposition": "CONFIRMED"' "$VDIR/age-stable.json"
  # The emit ran (guard passed) — the provenance sensor fired against this verdict.
  grep -q "provenance emit-verdict --file $VDIR/age-stable.json" "$AO_LOG"
}

@test "hijack guard: NO snapshot exported (older caller) BINDS the verdict — prior behavior (exit 0)" {
  run env -u PAWL_REVIEW_START_HEAD \
    bash "$SCRIPT" write age-nosnap 0 --disposition CONFIRMED --head "$X" \
    --author-context author-ctx \
    --refuter claude:CONFIRMED:fresh-reviewer-ctx:"$EV" --dir "$VDIR"
  [ "$status" -eq 0 ]
  [ -f "$VDIR/age-nosnap.json" ]
  grep -q "provenance emit-verdict --file $VDIR/age-nosnap.json" "$AO_LOG"
}

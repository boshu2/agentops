#!/usr/bin/env bats
# age-t5p6 (+ age-fkps (c)): the land lane's close step must resolve the live
# private ledger (BEADS_DIR via `ao beads dir`) BEFORE the close, exactly like
# scripts/land.sh. The close now PREFERS `ao done <bead> --sha <sha>` (verdict-
# stamped), which itself shells out to br; the fallback is a raw `br close`.
#
# Without the BEADS_DIR resolution, a bare close in the canonical checkout / a
# linked worktree — where $PWD/_beads is usually absent — fails ("Is a directory"),
# and the best-effort `|| true` silently swallows the failure, so the landed bead
# never closes. This suite drives the REAL land-lane-run.sh --once against a
# bare-origin fixture with br + ao STUBBED (ao done shells to br, which records the
# BEADS_DIR it saw; ao serves a dir) and asserts the resolved dir reaches br. It
# also guards that the retired `bd` fallback stays removed.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  LANE="$REPO_ROOT/scripts/land-lane-run.sh"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  NEXT="$REPO_ROOT/scripts/land-queue-next.sh"
  [ -x "$LANE" ]   || skip "land-lane-run.sh missing"
  [ -x "$SUBMIT" ] || skip "land-submit.sh missing"
  command -v jq >/dev/null 2>&1 || skip "jq required for the land-lane close e2e"

  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  QUEUE_DIR="$REPO/.agents/land-queue"
  BEADS_DIR_STUB="$TMP/beadsdir"
  BR_LOG="$TMP/br.log"

  git init --bare --quiet "$ORIGIN"
  git -C "$ORIGIN" symbolic-ref HEAD refs/heads/main

  mkdir -p "$REPO"
  git -C "$REPO" init --quiet
  git -C "$REPO" checkout --quiet -b main
  git -C "$REPO" config user.email "bats@test.local"
  git -C "$REPO" config user.name "bats-fixture"
  git -C "$REPO" config gc.auto 0
  printf 'base\n' >"$REPO/README.md"
  printf '.agents/\n' >"$REPO/.gitignore"
  git -C "$REPO" add README.md .gitignore
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main

  # Stub bin: br records the BEADS_DIR it was invoked with; ao serves a beads dir;
  # the gate just pushes HEAD:main (this suite exercises the CLOSE step, not pawl).
  BIN="$TMP/bin"
  mkdir -p "$BIN" "$BEADS_DIR_STUB"
  cat >"$BIN/br" <<EOS
#!/usr/bin/env bash
echo "BEADS_DIR=[\${BEADS_DIR:-UNSET}]" >>"$BR_LOG"
exit 0
EOS
  cat >"$BIN/ao" <<EOS
#!/usr/bin/env bash
if [[ "\$1" == "beads" && "\$2" == "dir" ]]; then echo "$BEADS_DIR_STUB"; exit 0; fi
# The lane's close prefers 'ao done <bead> --sha <sha>'; real ao done shells out to
# br for the actual close, so mirror that here (br records the BEADS_DIR it saw).
if [[ "\$1" == "done" ]]; then exec "$BIN/br" close "\$2"; fi
exit 0
EOS
  GATE="$TMP/gate.sh"
  cat >"$GATE" <<EOS
#!/usr/bin/env bash
set -euo pipefail
git push origin HEAD:main
EOS
  chmod +x "$BIN/br" "$BIN/ao" "$GATE"

  cd "$REPO" || return 1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

queue_bead() {
  local bead="$1"
  git -C "$REPO" checkout --quiet -B "work-$bead" main
  printf '%s\n' "$bead" >"$REPO/$bead.txt"
  git -C "$REPO" add "$bead.txt"
  git -C "$REPO" commit --quiet -m "feat(land-queue): land $bead ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND="file" bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

@test "close_bead resolves BEADS_DIR via 'ao beads dir' before br close" {
  queue_bead age-close1

  run env -u BEADS_DIR PATH="$BIN:$PATH" \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    LAND_LANE_NEXT_SCRIPT="$NEXT" \
    LAND_LANE_GATE_CMD="bash '$GATE'" \
    LAND_LANE_NO_ACTIONS_GUARD=0 \
    bash "$LANE" --once
  echo "$output"
  [ "$status" -eq 0 ]

  # br was invoked (the close ran) ...
  [ -f "$BR_LOG" ]
  # ... with BEADS_DIR resolved to the dir `ao beads dir` served, never UNSET.
  grep -q "BEADS_DIR=\[$BEADS_DIR_STUB\]" "$BR_LOG"
  ! grep -q 'UNSET' "$BR_LOG"
}

@test "BR_BIN resolution carries no retired 'bd' fallback" {
  # bd/Dolt is retired legacy; the lane must never fall back to it.
  ! grep -q 'command -v bd' "$LANE"
}

#!/usr/bin/env bats
# agentops-2pl.9: the LAND LANE — a single serialized writer owning `main`.
#
# E2E: queue 3 independent-file bead branches via the REAL land-submit.sh, run
# land-lane-run.sh in --drain mode, and assert:
#   (a) main contains all 3 commits in queue order, each rebased onto the prior;
#   (b) each landing ran the gate EXACTLY once (gate-marker count == 3);
#   (c) NO force-push occurred (origin/main only fast-forwards; old SHAs stay
#       ancestors of new ones);
#   (d) a 2nd concurrent land-lane-run.sh refuses to start (singleton lock held);
#   (e) a deliberately-conflicting request is dead-lettered and the lane
#       continues to land the others.
#
# The gate is INJECTED via LAND_LANE_GATE_CMD because the real pawl-review needs a
# live cross-family model (codex) we cannot run in CI. The injected gate uses the
# REAL pawl-verdict.sh to write a CONFIRMED verdict and the REAL pawl-land.sh to
# rebase+stamp+push — so the rebase, the post-rebase stamp, the no-force-push push
# HEAD:main, and the singleton lock are all exercised for real. Only the
# cross-family model call is stubbed; everything load-bearing runs.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  LANE="$REPO_ROOT/scripts/land-lane-run.sh"
  NEXT="$REPO_ROOT/scripts/land-queue-next.sh"

  [ -x "$SUBMIT" ] || skip "land-submit.sh missing"
  [ -x "$LANE" ]   || skip "land-lane-run.sh missing"
  command -v jq >/dev/null 2>&1 || skip "jq required for the land-lane e2e"

  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  QUEUE_DIR="$REPO/.agents/land-queue"
  GATE_LOG="$TMP/gate-runs.log"

  git init --bare --quiet "$ORIGIN"
  git -C "$ORIGIN" symbolic-ref HEAD refs/heads/main

  mkdir -p "$REPO"
  git -C "$REPO" init --quiet
  git -C "$REPO" checkout --quiet -b main
  git -C "$REPO" config user.email "bats@test.local"
  git -C "$REPO" config user.name "bats-fixture"
  git -C "$REPO" config gc.auto 0
  git -C "$REPO" config maintenance.auto false
  printf 'base\n' >"$REPO/README.md"
  printf '.agents/\n' >"$REPO/.gitignore"

  # Copy the pawl primitives + schema into the fixture so pawl-land.sh / the
  # injected gate run against in-tree scripts (matches postrebase-pawl-stamp.bats).
  # Commit them in the INITIAL commit so the working tree is clean when
  # land-submit.sh (which refuses on any untracked file) runs.
  mkdir -p "$REPO/scripts" "$REPO/schemas" "$REPO/.agents/pawl-verdicts" "$REPO/.git/hooks"
  cp "$REPO_ROOT/scripts/pawl-land.sh"            "$REPO/scripts/pawl-land.sh"
  cp "$REPO_ROOT/scripts/pawl-verdict.sh"         "$REPO/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/check-pawl-pre-push.sh"  "$REPO/scripts/check-pawl-pre-push.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$REPO/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$REPO/scripts/"*.sh

  git -C "$REPO" add README.md .gitignore scripts schemas
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main

  # A real pre-push hook enforcing the pawl gate: counts pushes and rejects an
  # unverdicted main push. Proves the lane drives a verdict-bound, fast-forward
  # push (no --force).
  cat >"$REPO/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
repo="$(git rev-parse --show-toplevel)"
echo pre-push >>"$repo/.git/pre-push-count"
exec "$repo/scripts/check-pawl-pre-push.sh"
EOS
  chmod +x "$REPO/.git/hooks/pre-push"

  EVIDENCE="$TMP/evidence.txt"
  printf 'fresh-context review evidence (injected gate)\n' >"$EVIDENCE"

  # The INJECTED gate: <gate> <bead> <branch>. Run from a land-work branch already
  # rebased onto origin/main with HEAD citing the bead. It (1) records ONE gate run,
  # (2) writes a CONFIRMED verdict bound to the current HEAD via the REAL
  # pawl-verdict.sh, then (3) lands via the REAL pawl-land.sh (rebase + post-rebase
  # stamp + push HEAD:main, single shot, no force).
  GATE="$TMP/gate.sh"
  cat >"$GATE" <<EOS
#!/usr/bin/env bash
set -euo pipefail
bead="\$1"
echo "gate-run \$bead" >>"$GATE_LOG"
head="\$(git -C "$REPO" rev-parse HEAD)"
bash "$REPO/scripts/pawl-verdict.sh" write "\$bead" 0 \\
  --disposition CONFIRMED --head "\$head" \\
  --author-context "author-operator-\$bead" --author-family operator \\
  --refuter "gpt:CONFIRMED:fresh-reviewer-\$bead:$EVIDENCE" \\
  --dir "$REPO/.agents/pawl-verdicts" >/dev/null
bash "$REPO/scripts/pawl-land.sh" "\$bead"
EOS
  chmod +x "$GATE"

  cd "$REPO" || return 1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Make a queued bead branch touching a unique file, then submit it via the REAL
# land-submit.sh (pushes refs/heads/land-queue/<bead> + appends the FIFO request).
queue_bead() {
  local bead="$1" file="$2" content="$3"
  git -C "$REPO" checkout --quiet main
  git -C "$REPO" checkout --quiet -B "work-$bead"
  printf '%s\n' "$content" >"$REPO/$file"
  git -C "$REPO" add "$file"
  git -C "$REPO" commit --quiet -m "feat(land-queue): land $bead ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND=file LAND_AUTHOR_FAMILY=operator \
      bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

run_lane() {
  ( cd "$REPO" && \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    LAND_LANE_GATE_CMD="bash '$GATE'" \
    LAND_LANE_PAWL_LAND_SCRIPT="$REPO/scripts/pawl-land.sh" \
    "$@" )
}

gate_runs() {
  [ -f "$GATE_LOG" ] && wc -l <"$GATE_LOG" | tr -d '[:space:]' || echo 0
}

@test "lane lands 3 queued beads in order, each gated once, no force-push" {
  queue_bead age-2pl9a alpha.txt alpha
  queue_bead age-2pl9b bravo.txt bravo
  queue_bead age-2pl9c charlie.txt charlie

  # 3 queued requests present
  [ "$(wc -l <"$QUEUE_DIR/requests.jsonl" | tr -d '[:space:]')" = "3" ]

  ORIGIN_MAIN_BEFORE="$(git -C "$ORIGIN" rev-parse refs/heads/main)"

  run run_lane bash "$LANE" --drain
  echo "$output"
  [ "$status" -eq 0 ]

  # (a) main contains all 3 commits, in queue order, each rebased onto the prior.
  git -C "$REPO" fetch origin main --quiet
  log="$(git -C "$ORIGIN" log --format='%s' refs/heads/main)"
  echo "ORIGIN LOG:"; echo "$log"
  # newest first: charlie, bravo, alpha, then init
  [ "$(printf '%s\n' "$log" | sed -n '1p')" = "feat(land-queue): land age-2pl9c (age-2pl9c)" ]
  [ "$(printf '%s\n' "$log" | sed -n '2p')" = "feat(land-queue): land age-2pl9b (age-2pl9b)" ]
  [ "$(printf '%s\n' "$log" | sed -n '3p')" = "feat(land-queue): land age-2pl9a (age-2pl9a)" ]

  # all three files landed
  git -C "$ORIGIN" cat-file -e refs/heads/main:alpha.txt
  git -C "$ORIGIN" cat-file -e refs/heads/main:bravo.txt
  git -C "$ORIGIN" cat-file -e refs/heads/main:charlie.txt

  # (b) the gate ran exactly once per landing (3 total).
  [ "$(gate_runs)" = "3" ]
  # and the pre-push hook fired exactly 3 times (one verdict-bound push each).
  [ "$(wc -l <"$REPO/.git/pre-push-count" | tr -d '[:space:]')" = "3" ]

  # (c) NO force-push: the pre-lane origin/main SHA is still an ancestor of the
  # final main (history was only fast-forwarded/appended, never rewritten).
  git -C "$ORIGIN" merge-base --is-ancestor "$ORIGIN_MAIN_BEFORE" refs/heads/main

  # bookkeeping: 3 done, 0 dead-letters.
  [ "$(wc -l <"$QUEUE_DIR/done.jsonl" | tr -d '[:space:]')" = "3" ]
  [ ! -s "$QUEUE_DIR/dead-letter.jsonl" ]
}

@test "a conflicting request is dead-lettered and the lane lands the rest" {
  # alpha + charlie touch independent files; bravo will conflict with an upstream
  # change to the SAME file (README.md) that lands on main first.
  queue_bead age-2pl9a alpha.txt alpha
  queue_bead age-2pl9b README.md "bead-side-readme"
  queue_bead age-2pl9c charlie.txt charlie

  # Land a conflicting change to README.md directly on origin/main BEFORE the lane
  # runs, so bravo's rebase onto origin/main conflicts on README.md.
  CLONE="$TMP/upstream-writer"
  git clone --quiet "$ORIGIN" "$CLONE"
  git -C "$CLONE" config user.email "bats@test.local"
  git -C "$CLONE" config user.name "bats-fixture"
  printf 'upstream-owns-readme\n' >"$CLONE/README.md"
  git -C "$CLONE" add README.md
  git -C "$CLONE" commit --quiet -m "chore: upstream rewrites README"
  git -C "$CLONE" push --quiet origin main

  run run_lane bash "$LANE" --drain
  echo "$output"
  [ "$status" -eq 0 ]

  git -C "$REPO" fetch origin main --quiet

  # bravo dead-lettered (rebase conflict), alpha + charlie landed.
  [ -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-2pl9b") | .status' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "dead-letter" ]

  # alpha + charlie are on main; bravo's README change is NOT (upstream's is).
  git -C "$ORIGIN" cat-file -e refs/heads/main:alpha.txt
  git -C "$ORIGIN" cat-file -e refs/heads/main:charlie.txt
  [ "$(git -C "$ORIGIN" show refs/heads/main:README.md)" = "upstream-owns-readme" ]

  # exactly the two good beads landed (gate ran twice, not for the dead-lettered one).
  [ "$(gate_runs)" = "2" ]
  [ "$(wc -l <"$QUEUE_DIR/done.jsonl" | tr -d '[:space:]')" = "2" ]
}

@test "a second concurrent lane refuses to start (singleton lock held)" {
  # Pre-create the lane lock dir to simulate a lane already holding it.
  mkdir -p "$QUEUE_DIR"
  mkdir "$QUEUE_DIR/.lane.lock"

  # With the lock held and a 1s timeout, a 2nd lane must refuse (exit 1) fast.
  run env AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" AGENTOPS_PUSH_LOCK_TIMEOUT=1 \
      bash "$LANE" --once
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" == *"single-writer invariant"* ]] || [[ "$output" == *"already holds"* ]]

  # The held lock is still there (the refusing lane did not steal/clear it).
  [ -d "$QUEUE_DIR/.lane.lock" ]
}

@test "drain is crash-safe: a re-run does not re-land already-done beads" {
  queue_bead age-2pl9a alpha.txt alpha
  queue_bead age-2pl9b bravo.txt bravo

  run run_lane bash "$LANE" --drain
  [ "$status" -eq 0 ]
  [ "$(gate_runs)" = "2" ]
  [ "$(wc -l <"$QUEUE_DIR/done.jsonl" | tr -d '[:space:]')" = "2" ]

  # Second drain: queue is fully claimed/done — no new gate runs, no new lands.
  run run_lane bash "$LANE" --drain
  echo "$output"
  [ "$status" -eq 0 ]
  [ "$(gate_runs)" = "2" ]
  [ "$(wc -l <"$QUEUE_DIR/done.jsonl" | tr -d '[:space:]')" = "2" ]
}

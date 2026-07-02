#!/usr/bin/env bats
# agentops-2pl.12: final integrated acceptance proof for the land queue.
#
# One drain proves the four named pains are dead together:
#   - rebase-race: three queued branches land in one serialized lane, with a
#     concurrent lane refused and main only fast-forwarded.
#   - commit-bound-pawl catch-22: origin/main advances after the verdict is
#     written but before pawl-land pushes; pawl-land restamps the post-rebase SHA.
#   - flaky-unrelated-red: an injected Go race flake is retried at package scope,
#     quarantined, and does not red/dead-letter independent branches.
#   - github-actions-rate-limit: the no-actions runtime shim is active for every
#     gate attempt and no gh/Actions command is invoked.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  LANE="$REPO_ROOT/scripts/land-lane-run.sh"

  [ -x "$SUBMIT" ] || skip "land-submit.sh missing"
  [ -x "$LANE" ] || skip "land-lane-run.sh missing"
  command -v jq >/dev/null 2>&1 || skip "jq required"
  command -v go >/dev/null 2>&1 || skip "go required"

  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"

  cat >"$TMP/bin/gh" <<EOS
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >>"$TMP/real-gh.log"
exit 0
EOS
  chmod +x "$TMP/bin/gh"

  cat >"$TMP/bin/br" <<EOS
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >>"$TMP/br.log"
if [[ "\${1:-}" == "create" ]]; then
  echo "agentops-quarantine-e2e"
fi
exit 0
EOS
  chmod +x "$TMP/bin/br"
  export PATH="$TMP/bin:$PATH"

  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  REMOTE_WRITER="$TMP/remote-writer"
  QUEUE_DIR="$REPO/.agents/land-queue"
  GATE_LOG="$TMP/gate-runs.log"
  ACTIONS_SHIM_EVIDENCE="$TMP/actions-shim-evidence.log"
  MAIN_PUSH_LOG="$TMP/main-pushes.log"
  FLAKE_STATE="$TMP/flake-state.txt"
  ADVANCED_MARKER="$TMP/origin-advanced"
  LANE_OUT="$TMP/lane.out"

  git init --bare --quiet "$ORIGIN"
  git -C "$ORIGIN" symbolic-ref HEAD refs/heads/main

  mkdir -p "$REPO"
  git -C "$REPO" init --quiet
  git -C "$REPO" checkout --quiet -b main
  git_identity "$REPO"
  printf 'base\n' >"$REPO/README.md"
  printf '.agents/\n' >"$REPO/.gitignore"

  mkdir -p "$REPO/scripts" "$REPO/scripts/lib" "$REPO/schemas" "$REPO/.agents/pawl-verdicts" "$REPO/.git/hooks"
  cp "$REPO_ROOT/scripts/pawl-land.sh" "$REPO/scripts/pawl-land.sh"
  cp "$REPO_ROOT/scripts/pawl-verdict.sh" "$REPO/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/check-pawl-pre-push.sh" "$REPO/scripts/check-pawl-pre-push.sh"
  # check-pawl-pre-push.sh sources scripts/lib/trivial-waiver.sh and dies if absent.
  cp "$REPO_ROOT/scripts/lib/trivial-waiver.sh" "$REPO/scripts/lib/trivial-waiver.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$REPO/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$REPO/scripts/"*.sh

  git -C "$REPO" add README.md .gitignore scripts schemas
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main
  INITIAL_MAIN="$(git -C "$ORIGIN" rev-parse refs/heads/main)"

  cat >"$ORIGIN/hooks/pre-receive" <<EOS
#!/usr/bin/env bash
set -euo pipefail
while read -r old new ref; do
  if [[ "\$ref" == "refs/heads/main" ]]; then
    if [[ "\$old" != "0000000000000000000000000000000000000000" ]] &&
       ! git merge-base --is-ancestor "\$old" "\$new"; then
      echo "non-fast-forward \$old \$new \$ref" >>"$MAIN_PUSH_LOG"
      exit 1
    fi
    echo "\$old \$new \$ref" >>"$MAIN_PUSH_LOG"
  fi
done
EOS
  chmod +x "$ORIGIN/hooks/pre-receive"

  git clone --quiet "$ORIGIN" "$REMOTE_WRITER"
  git_identity "$REMOTE_WRITER"

  cat >"$REPO/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
repo="$(git rev-parse --show-toplevel)"
echo pre-push >>"$repo/.git/pre-push-count"
# A land is [feat, #trivial-bind]: the #trivial tip is waived and the feat behind it
# is re-gated by the mixed-range cockpit gate (age-8ais). Stub that cockpit gate — the
# real `ao gate check` needs the full repo; this suite proves the integrated lane.
export AGENTOPS_PREPUSH_GATE_CMD=true
exec "$repo/scripts/check-pawl-pre-push.sh"
EOS
  chmod +x "$REPO/.git/hooks/pre-push"

  EVIDENCE="$TMP/evidence.txt"
  printf 'fresh-context review evidence (integrated e2e)\n' >"$EVIDENCE"
  printf '0' >"$FLAKE_STATE"

  ADVANCE_ORIGIN="$TMP/advance-origin.sh"
  cat >"$ADVANCE_ORIGIN" <<EOS
#!/usr/bin/env bash
set -euo pipefail
git -C "$REMOTE_WRITER" fetch origin main --quiet
git -C "$REMOTE_WRITER" checkout --quiet main
git -C "$REMOTE_WRITER" reset --quiet --hard origin/main
printf 'upstream between landings\n' >"$REMOTE_WRITER/upstream-between-landings.txt"
git -C "$REMOTE_WRITER" add upstream-between-landings.txt
git -C "$REMOTE_WRITER" commit --quiet -m "chore: upstream between landings"
git -C "$REMOTE_WRITER" push --quiet origin main
git -C "$REMOTE_WRITER" rev-parse HEAD >"$TMP/upstream-sha"
EOS
  chmod +x "$ADVANCE_ORIGIN"

  GATE="$TMP/gate-and-land.sh"
  cat >"$GATE" <<EOS
#!/usr/bin/env bash
set -euo pipefail
bead="\$1"
echo "gate-run \$bead" >>"$GATE_LOG"
gh_path="\$(command -v gh || true)"
echo "\$bead \$gh_path" >>"$ACTIONS_SHIM_EVIDENCE"
case "\$gh_path" in
  "$QUEUE_DIR/.no-actions-gh/gh") ;;
  *) echo "runtime gh shim not active: \$gh_path" >&2; exit 1 ;;
esac

if [[ "\$bead" == "age-2pl12a" ]]; then
  touch "$TMP/alpha-gate-entered"
  sleep 2
fi

if [[ "\$bead" == "age-2pl12b" ]]; then
  go test -race -shuffle=123 -count=1 ./flakepkg -v
fi

head="\$(git -C "$REPO" rev-parse HEAD)"
bash "$REPO/scripts/pawl-verdict.sh" write "\$bead" 0 \\
  --disposition CONFIRMED --head "\$head" \\
  --author-context "author-operator-\$bead" --author-family operator \\
  --refuter "gpt:CONFIRMED:fresh-reviewer-\$bead:$EVIDENCE" \\
  --dir "$REPO/.agents/pawl-verdicts" >/dev/null

if [[ "\$bead" == "age-2pl12b" && ! -f "$ADVANCED_MARKER" ]]; then
  bash "$ADVANCE_ORIGIN"
  touch "$ADVANCED_MARKER"
fi

bash "$REPO/scripts/pawl-land.sh" "\$bead"
EOS
  chmod +x "$GATE"

  cd "$REPO" || return 1
}

teardown() {
  if [[ -n "${LANE_PID:-}" ]] && kill -0 "$LANE_PID" 2>/dev/null; then
    kill -TERM "$LANE_PID" 2>/dev/null || true
    wait "$LANE_PID" 2>/dev/null || true
  fi
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

git_identity() {
  git -C "$1" config user.email "bats@test.local"
  git -C "$1" config user.name "bats-fixture"
  git -C "$1" config gc.auto 0
  git -C "$1" config maintenance.auto false
}

queue_file_bead() {
  local bead="$1" file="$2" content="$3"
  git -C "$REPO" checkout --quiet main
  git -C "$REPO" checkout --quiet -B "work-$bead"
  printf '%s\n' "$content" >"$REPO/$file"
  git -C "$REPO" add "$file"
  git -C "$REPO" commit --quiet -m "feat(land-queue): accept $bead ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND=file LAND_AUTHOR_FAMILY=operator \
      bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

queue_flaky_bead() {
  local bead="$1"
  git -C "$REPO" checkout --quiet main
  git -C "$REPO" checkout --quiet -B "work-$bead"
  cat >"$REPO/go.mod" <<'EOF'
module example.com/landq

go 1.25
EOF
  mkdir -p "$REPO/flakepkg"
  cat >"$REPO/flakepkg/flakepkg.go" <<'EOF'
package flakepkg

func Value() int { return 1 }
EOF
  cat >"$REPO/flakepkg/flakepkg_test.go" <<'EOF'
package flakepkg

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestInjectedOneInThreeFlake(t *testing.T) {
	path := os.Getenv("LAND_LANE_FLAKE_STATE")
	if path == "" {
		t.Fatal("LAND_LANE_FLAKE_STATE missing")
	}
	raw, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	n++
	if err := os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644); err != nil {
		t.Fatal(err)
	}
	if n%3 == 1 {
		t.Fatalf("injected one-in-three flake attempt=%d", n)
	}
}
EOF
  git -C "$REPO" add go.mod flakepkg
  git -C "$REPO" commit --quiet -m "feat(land-queue): accept flaky branch ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND=file LAND_AUTHOR_FAMILY=operator \
      bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

run_lane_async() {
  ( cd "$REPO" && \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    AGENTOPS_NO_ACTIONS_LOG="$QUEUE_DIR/no-actions-guard.jsonl" \
    LAND_LANE_GATE_CMD="bash '$GATE'" \
    LAND_LANE_FLAKY_RETRY_MAX=2 \
    LAND_LANE_FLAKE_STATE="$FLAKE_STATE" \
    LAND_LANE_QUARANTINE_FILE="$QUEUE_DIR/quarantine.jsonl" \
    BR_BIN="$TMP/bin/br" \
    bash "$LANE" --drain >"$LANE_OUT" 2>&1 ) &
  LANE_PID=$!
}

run_contender_lane() {
  ( cd "$REPO" && \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    AGENTOPS_PUSH_LOCK_TIMEOUT=1 \
    bash "$LANE" --once )
}

wait_for_file() {
  local path="$1" i
  for i in $(seq 1 100); do
    [[ -e "$path" ]] && return 0
    sleep 0.1
  done
  return 1
}

commit_by_subject() {
  local subject="$1"
  git -C "$ORIGIN" log --format='%H%x09%s' refs/heads/main |
    awk -F '\t' -v s="$subject" '$2 == s { print $1; exit }'
}

parent_of() {
  git -C "$ORIGIN" rev-parse "$1^"
}

@test "integrated land queue kills rebase-race, catch-22, flaky-unrelated-red, and Actions dependence" {
  queue_file_bead age-2pl12a alpha.txt alpha
  queue_flaky_bead age-2pl12b
  queue_file_bead age-2pl12c charlie.txt charlie

  [ "$(wc -l <"$QUEUE_DIR/requests.jsonl" | tr -d '[:space:]')" = "3" ]

  run_lane_async
  wait_for_file "$TMP/alpha-gate-entered" || {
    cat "$LANE_OUT" >&3 2>/dev/null || true
    false
  }

  run run_contender_lane
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" == *"single-writer invariant"* ]] || [[ "$output" == *"already holds"* ]]

  wait "$LANE_PID"
  lane_status=$?
  LANE_PID=""
  echo "LANE OUTPUT:"
  cat "$LANE_OUT"
  [ "$lane_status" -eq 0 ]

  git -C "$REPO" fetch origin main --quiet

  # Race killed: all submitted branches landed sequentially and no request died.
  git -C "$ORIGIN" cat-file -e refs/heads/main:alpha.txt
  git -C "$ORIGIN" cat-file -e refs/heads/main:flakepkg/flakepkg_test.go
  git -C "$ORIGIN" cat-file -e refs/heads/main:charlie.txt
  [ "$(wc -l <"$QUEUE_DIR/done.jsonl" | tr -d '[:space:]')" = "3" ]
  [ ! -s "$QUEUE_DIR/dead-letter.jsonl" ]

  alpha_subject="feat(land-queue): accept age-2pl12a (age-2pl12a)"
  flake_subject="feat(land-queue): accept flaky branch (age-2pl12b)"
  charlie_subject="feat(land-queue): accept age-2pl12c (age-2pl12c)"
  upstream_subject="chore: upstream between landings"

  alpha_sha="$(commit_by_subject "$alpha_subject")"
  upstream_sha="$(commit_by_subject "$upstream_subject")"
  flake_sha="$(commit_by_subject "$flake_subject")"
  charlie_sha="$(commit_by_subject "$charlie_subject")"

  [ -n "$alpha_sha" ]
  [ -n "$upstream_sha" ]
  [ -n "$flake_sha" ]
  [ -n "$charlie_sha" ]
  # Each land appends [feat, #trivial-bind] (age-fkps single-trivial-per-land): the
  # pawl review's SINGLE auto-bind provenance commit sits directly on the feat and is
  # the pushed tip, so the NEXT thing (an upstream advance, or the next rebased feat)
  # parents onto that #trivial tip — not the bare feat. Verify the interleave + the
  # fast-forward lineage.
  trivial_alpha="$(commit_by_subject "chore(provenance): bind pawl CONFIRMED verdict for age-2pl12a #trivial")"
  trivial_flake="$(commit_by_subject "chore(provenance): bind pawl CONFIRMED verdict for age-2pl12b #trivial")"
  [ -n "$trivial_alpha" ]
  [ -n "$trivial_flake" ]
  [ "$(parent_of "$alpha_sha")" = "$INITIAL_MAIN" ]      # feat alpha on initial main
  [ "$(parent_of "$trivial_alpha")" = "$alpha_sha" ]     # alpha's #trivial bind on its feat
  [ "$(parent_of "$upstream_sha")" = "$trivial_alpha" ]  # upstream advanced on alpha's #trivial tip
  [ "$(parent_of "$flake_sha")" = "$upstream_sha" ]      # feat flake rebased onto upstream
  [ "$(parent_of "$trivial_flake")" = "$flake_sha" ]     # flake's #trivial bind on its feat
  [ "$(parent_of "$charlie_sha")" = "$trivial_flake" ]   # feat charlie rebased onto flake's #trivial tip

  # No force-push: every main update observed by the bare origin was a fast-forward,
  # and the initial main remains in final history.
  [ "$(wc -l <"$MAIN_PUSH_LOG" | tr -d '[:space:]')" = "4" ]
  ! grep -q 'non-fast-forward' "$MAIN_PUSH_LOG"
  git -C "$ORIGIN" merge-base --is-ancestor "$INITIAL_MAIN" refs/heads/main

  # Catch-22 killed: the verdict was written before the upstream commit, then
  # pawl-land rebased and restamped it to the post-rebase flake SHA.
  [ -f "$ADVANCED_MARKER" ]
  [ "$(cat "$TMP/upstream-sha")" = "$upstream_sha" ]
  [ "$(jq -r '.head_sha' "$REPO/.agents/pawl-verdicts/age-2pl12b.json")" = "$flake_sha" ]
  [ "$(wc -l <"$REPO/.git/pre-push-count" | tr -d '[:space:]')" = "3" ]

  # Flaky-unrelated-red killed: first package gate failed, package retry passed
  # within budget, quarantine was filed, and independent branches still landed.
  [ "$(cat "$FLAKE_STATE")" = "3" ]
  [ -s "$QUEUE_DIR/quarantine.jsonl" ]
  run jq -r '[.bead, .package, .shuffle_seed, .tracker_id, .status] | @tsv' "$QUEUE_DIR/quarantine.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = $'age-2pl12b\texample.com/landq/flakepkg\t123\tagentops-quarantine-e2e\tquarantine-filed' ]
  grep -q "create Quarantine flaky Go race test in example.com/landq/flakepkg" "$TMP/br.log"

  # Actions-free: the runtime shim was active during every gate attempt, and no
  # gh command, workflow, PR, or Actions verb was invoked.
  [ "$(wc -l <"$ACTIONS_SHIM_EVIDENCE" | tr -d '[:space:]')" = "4" ]
  run awk '{ print $2 }' "$ACTIONS_SHIM_EVIDENCE"
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | sort -u)" = "$QUEUE_DIR/.no-actions-gh/gh" ]
  [ ! -s "$TMP/real-gh.log" ]
  [ ! -s "$QUEUE_DIR/no-actions-guard.jsonl" ]

  # The flake branch needed a second gate attempt after package-scope retry; the
  # other two branches gated once and stayed green.
  [ "$(grep -c 'gate-run age-2pl12a' "$GATE_LOG")" = "1" ]
  [ "$(grep -c 'gate-run age-2pl12b' "$GATE_LOG")" = "2" ]
  [ "$(grep -c 'gate-run age-2pl12c' "$GATE_LOG")" = "1" ]
}

#!/usr/bin/env bats
# agentops-2pl.10: failed land-lane Go gates retry only failing packages under -race.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  LANE="$REPO_ROOT/scripts/land-lane-run.sh"
  RETRY="$REPO_ROOT/scripts/land-lane-flaky-retry.sh"

  [ -x "$SUBMIT" ] || skip "land-submit.sh missing"
  [ -x "$LANE" ] || skip "land-lane-run.sh missing"
  command -v jq >/dev/null 2>&1 || skip "jq required"
  command -v go >/dev/null 2>&1 || skip "go required"

  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"

  cat >"$TMP/bin/br" <<EOS
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >>"$TMP/br.log"
if [[ "\${1:-}" == "create" ]]; then
  echo "agentops-quarantine-123"
fi
EOS
  chmod +x "$TMP/bin/br"
  export PATH="$TMP/bin:$PATH"

  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
  QUEUE_DIR="$REPO/.agents/land-queue"
  GATE_LOG="$TMP/gate-runs.log"
  FLAKE_STATE="$TMP/flake-state.txt"

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

  mkdir -p "$REPO/scripts" "$REPO/schemas" "$REPO/.agents/pawl-verdicts" "$REPO/.git/hooks"
  cp "$REPO_ROOT/scripts/pawl-land.sh" "$REPO/scripts/pawl-land.sh"
  cp "$REPO_ROOT/scripts/pawl-verdict.sh" "$REPO/scripts/pawl-verdict.sh"
  cp "$REPO_ROOT/scripts/check-pawl-pre-push.sh" "$REPO/scripts/check-pawl-pre-push.sh"
  cp "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$REPO/schemas/pawl-verdict.v1.schema.json"
  chmod +x "$REPO/scripts/"*.sh

  git -C "$REPO" add README.md .gitignore scripts schemas
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main

  cat >"$REPO/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
exec "$(git rev-parse --show-toplevel)/scripts/check-pawl-pre-push.sh"
EOS
  chmod +x "$REPO/.git/hooks/pre-push"

  EVIDENCE="$TMP/evidence.txt"
  printf 'fresh-context review evidence (flaky retry)\n' >"$EVIDENCE"

  GATE_ONLY="$TMP/gate-only.sh"
  cat >"$GATE_ONLY" <<EOS
#!/usr/bin/env bash
set -euo pipefail
echo "gate-run \$1" >>"$GATE_LOG"
go test -race -shuffle=123 -count=1 ./flakepkg -v
EOS
  chmod +x "$GATE_ONLY"

  LAND_ONLY="$TMP/land-only.sh"
  cat >"$LAND_ONLY" <<EOS
#!/usr/bin/env bash
set -euo pipefail
bead="\$1"
head="\$(git -C "$REPO" rev-parse HEAD)"
bash "$REPO/scripts/pawl-verdict.sh" write "\$bead" 0 \\
  --disposition CONFIRMED --head "\$head" \\
  --author-context "author-operator-\$bead" --author-family operator \\
  --refuter "gpt:CONFIRMED:fresh-reviewer-\$bead:$EVIDENCE" \\
  --dir "$REPO/.agents/pawl-verdicts" >/dev/null
bash "$REPO/scripts/pawl-land.sh" "\$bead"
EOS
  chmod +x "$LAND_ONLY"

  cd "$REPO" || return 1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

queue_go_bead() {
  local bead="$1" mode="$2"
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

  if [[ "$mode" == "flake" ]]; then
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
  else
    cat >"$REPO/flakepkg/flakepkg_test.go" <<'EOF'
package flakepkg

import "testing"

func TestDeterministicFailure(t *testing.T) {
	t.Fatal("deterministic failure")
}
EOF
  fi

  git -C "$REPO" add go.mod flakepkg
  git -C "$REPO" commit --quiet -m "fix(land-queue): $mode package ($bead)"
  ( cd "$REPO" && AGENTOPS_LAND_QUEUE_BACKEND=file LAND_AUTHOR_FAMILY=operator \
      bash "$SUBMIT" "$bead" >/dev/null )
  git -C "$REPO" checkout --quiet main
}

run_lane_once() {
  ( cd "$REPO" && \
    AGENTOPS_LAND_QUEUE_DIR="$QUEUE_DIR" \
    LAND_LANE_GATE_ONLY_CMD="bash '$GATE_ONLY'" \
    LAND_LANE_LAND_CMD="bash '$LAND_ONLY'" \
    LAND_LANE_FLAKY_RETRY_MAX=2 \
    LAND_LANE_FLAKE_STATE="$FLAKE_STATE" \
    LAND_LANE_QUARANTINE_FILE="$QUEUE_DIR/quarantine.jsonl" \
    BR_BIN="$TMP/bin/br" \
    bash "$LANE" --drain )
}

@test "pkg extraction parses failing package and shuffle seed from race log" {
  log="$TMP/race.log"
  cat >"$log" <<'EOF'
-test.shuffle 8675309
=== RUN   TestInjectedOneInThreeFlake
--- FAIL: TestInjectedOneInThreeFlake (0.00s)
    flakepkg_test.go:10: injected one-in-three flake
FAIL	github.com/boshu2/agentops/cli/internal/example	0.033s
FAIL
EOF

  run bash "$RETRY" parse "$log"
  [ "$status" -eq 0 ]
  [ "$output" = $'github.com/boshu2/agentops/cli/internal/example\t8675309' ]
}

@test "flake passing on package retry lands and files quarantine bead record" {
  printf '0' >"$FLAKE_STATE"
  queue_go_bead age-2pl10-flake flake

  run run_lane_once
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"FLAKE age-2pl10-flake"* ]]

  git -C "$REPO" fetch origin main --quiet
  git -C "$ORIGIN" cat-file -e refs/heads/main:flakepkg/flakepkg_test.go

  [ -s "$QUEUE_DIR/done.jsonl" ]
  [ ! -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-2pl10-flake") | .status' "$QUEUE_DIR/done.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = "done" ]

  [ -s "$QUEUE_DIR/quarantine.jsonl" ]
  run jq -r '[.bead, .package, .shuffle_seed, .tracker_id] | @tsv' "$QUEUE_DIR/quarantine.jsonl"
  [ "$status" -eq 0 ]
  [ "$output" = $'age-2pl10-flake\texample.com/landq/flakepkg\t123\tagentops-quarantine-123' ]

  [ -s "$TMP/br.log" ]
  grep -q "Quarantine flaky Go race test in example.com/landq/flakepkg" "$TMP/br.log"
}

@test "deterministic package retry dead-letters with failing package and seed" {
  queue_go_bead age-2pl10-deterministic deterministic

  run run_lane_once
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"deterministic gate failure"* ]]

  [ -s "$QUEUE_DIR/dead-letter.jsonl" ]
  run jq -r 'select(.bead == "age-2pl10-deterministic") | .reason' "$QUEUE_DIR/dead-letter.jsonl"
  [ "$status" -eq 0 ]
  [[ "$output" == *"example.com/landq/flakepkg"* ]]
  [[ "$output" == *"seed=123"* ]]

  [ ! -s "$QUEUE_DIR/done.jsonl" ]
  [ ! -s "$QUEUE_DIR/quarantine.jsonl" ]
  [ ! -s "$TMP/br.log" ]
}

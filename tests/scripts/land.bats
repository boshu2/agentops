#!/usr/bin/env bats
# age-genn: scripts/land.sh fails fast on missing bead citation and sequences the
# current proof path without hand-rolled push/close instructions.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/land.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

make_repo() {
  local msg="$1"
  REPO="$TMP/repo"
  mkdir -p "$REPO"
  cd "$REPO"
  git init -q
  git config user.email test@example.com
  git config user.name Test
  echo base > README.md
  git add README.md
  git commit -q -m "init"
  git checkout -q -b task/test-land
  echo change >> README.md
  git add README.md
  git commit -q -m "$msg"
}

make_stub() {
  local name="$1"
  local body="$2"
  cat > "$TMP/$name" <<EOF
#!/usr/bin/env bash
set -euo pipefail
$body
EOF
  chmod +x "$TMP/$name"
}

@test "land --help renders usage" {
  run "$SCRIPT" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"land.sh"* ]]
}

@test "land fails before gate work when HEAD does not cite bead" {
  make_repo "fix(test): missing bead"
  make_stub ship 'echo ship >>"$LAND_TEST_LOG"'
  export LAND_TEST_LOG="$TMP/calls.log"

  run env LAND_SHIP_SCRIPT="$TMP/ship" "$SCRIPT" age-genn

  [ "$status" -eq 2 ]
  [[ "$output" == *"does not cite age-genn"* ]]
  [[ "$output" == *'git commit --amend --no-edit --trailer "Refs: age-genn"'* ]]
  [ ! -f "$LAND_TEST_LOG" ]
}

@test "land requires an exact bead token, not a substring" {
  make_repo "fix(test): wrong bead (age-genn-extra)"
  make_stub ship 'echo ship >>"$LAND_TEST_LOG"'
  export LAND_TEST_LOG="$TMP/calls.log"

  run env LAND_SHIP_SCRIPT="$TMP/ship" "$SCRIPT" age-genn

  [ "$status" -eq 2 ]
  [[ "$output" == *"does not cite age-genn"* ]]
  [ ! -f "$LAND_TEST_LOG" ]
}

@test "land sequences ship, pawl review, pawl land, provenance, and br close" {
  make_repo "fix(test): land wrapper (age-genn)"
  export LAND_TEST_LOG="$TMP/calls.log"
  export BEADS_DIR="$TMP/beads"
  mkdir -p "$BEADS_DIR"

  make_stub ship 'echo "ship $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_review 'echo "pawl-review $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_land 'echo "pawl-land $*" >>"$LAND_TEST_LOG"'
  make_stub post_land 'echo "post-land strict=${AGENTOPS_PROVENANCE_EMIT_STRICT:-} required=${AGENTOPS_PROVENANCE_REQUIRED_VERDICT_BEAD:-} head=${AGENTOPS_PROVENANCE_REQUIRED_VERDICT_HEAD:-}" >>"$LAND_TEST_LOG"'
  make_stub br 'echo "br $*" >>"$LAND_TEST_LOG"'

  run env \
    LAND_SHIP_SCRIPT="$TMP/ship" \
    LAND_PAWL_REVIEW_SCRIPT="$TMP/pawl_review" \
    LAND_PAWL_LAND_SCRIPT="$TMP/pawl_land" \
    LAND_POST_LAND_SCRIPT="$TMP/post_land" \
    BR_BIN="$TMP/br" \
    "$SCRIPT" age-genn

  [ "$status" -eq 0 ]
  [[ "$output" == *"land: DONE age-genn"* ]]
  [ "$(sed -n '1p' "$LAND_TEST_LOG")" = "ship " ]
  [ "$(sed -n '2p' "$LAND_TEST_LOG")" = "pawl-review age-genn --scope head --author-family operator" ]
  [ "$(sed -n '3p' "$LAND_TEST_LOG")" = "pawl-land age-genn" ]
  [[ "$(sed -n '4p' "$LAND_TEST_LOG")" == post-land\ strict=1\ required=age-genn\ head=* ]]
  [[ "$(sed -n '5p' "$LAND_TEST_LOG")" == br\ close\ age-genn* ]]
}

@test "land stops before pawl review when ship leaves regeneration changes" {
  make_repo "fix(test): land wrapper (age-genn)"
  export LAND_TEST_LOG="$TMP/calls.log"

  make_stub ship 'echo "ship $*" >>"$LAND_TEST_LOG"; echo generated > generated.txt'
  make_stub pawl_review 'echo "pawl-review $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_land 'echo "pawl-land $*" >>"$LAND_TEST_LOG"'
  make_stub post_land 'echo "post-land" >>"$LAND_TEST_LOG"'

  run env \
    LAND_SHIP_SCRIPT="$TMP/ship" \
    LAND_PAWL_REVIEW_SCRIPT="$TMP/pawl_review" \
    LAND_PAWL_LAND_SCRIPT="$TMP/pawl_land" \
    LAND_POST_LAND_SCRIPT="$TMP/post_land" \
    "$SCRIPT" age-genn

  [ "$status" -eq 2 ]
  [[ "$output" == *"ship produced uncommitted changes"* ]]
  [ "$(sed -n '1p' "$LAND_TEST_LOG")" = "ship " ]
  ! grep -q '^pawl-review ' "$LAND_TEST_LOG"
  ! grep -q '^pawl-land ' "$LAND_TEST_LOG"
  ! grep -q '^post-land' "$LAND_TEST_LOG"
}

@test "land does not close bead when strict post-land provenance fails" {
  make_repo "fix(test): land wrapper (age-genn)"
  export LAND_TEST_LOG="$TMP/calls.log"
  export BEADS_DIR="$TMP/beads"
  mkdir -p "$BEADS_DIR"

  make_stub ship 'echo "ship $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_review 'echo "pawl-review $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_land 'echo "pawl-land $*" >>"$LAND_TEST_LOG"'
  make_stub post_land 'echo "post-land fail" >>"$LAND_TEST_LOG"; exit 7'
  make_stub br 'echo "br $*" >>"$LAND_TEST_LOG"'

  run env \
    LAND_SHIP_SCRIPT="$TMP/ship" \
    LAND_PAWL_REVIEW_SCRIPT="$TMP/pawl_review" \
    LAND_PAWL_LAND_SCRIPT="$TMP/pawl_land" \
    LAND_POST_LAND_SCRIPT="$TMP/post_land" \
    BR_BIN="$TMP/br" \
    "$SCRIPT" age-genn

  [ "$status" -eq 7 ]
  [ "$(sed -n '1p' "$LAND_TEST_LOG")" = "ship " ]
  [ "$(sed -n '2p' "$LAND_TEST_LOG")" = "pawl-review age-genn --scope head --author-family operator" ]
  [ "$(sed -n '3p' "$LAND_TEST_LOG")" = "pawl-land age-genn" ]
  [ "$(sed -n '4p' "$LAND_TEST_LOG")" = "post-land fail" ]
  ! grep -q '^br close age-genn' "$LAND_TEST_LOG"
}

@test "background dry-run preserves dry-run and does not run ship work" {
  make_repo "fix(test): land wrapper (age-genn)"
  export LAND_TEST_LOG="$TMP/calls.log"
  mkdir -p "$TMP/logs"

  make_stub ship 'echo "ship $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_review 'echo "pawl-review $*" >>"$LAND_TEST_LOG"'
  make_stub pawl_land 'echo "pawl-land $*" >>"$LAND_TEST_LOG"'
  make_stub post_land 'echo "post-land" >>"$LAND_TEST_LOG"'

  run env \
    LAND_LOG_DIR="$TMP/logs" \
    LAND_SHIP_SCRIPT="$TMP/ship" \
    LAND_PAWL_REVIEW_SCRIPT="$TMP/pawl_review" \
    LAND_PAWL_LAND_SCRIPT="$TMP/pawl_land" \
    LAND_POST_LAND_SCRIPT="$TMP/post_land" \
    "$SCRIPT" age-genn --background --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"started background land for age-genn"* ]]

  log=""
  for _ in $(seq 1 50); do
    log="$(find "$TMP/logs" -type f -name 'age-genn-*.log' -print -quit)"
    if [[ -n "$log" ]] && grep -q 'dry-run OK' "$log"; then
      break
    fi
    sleep 0.1
  done

  [[ -n "$log" ]]
  grep -q 'dry-run OK' "$log"
  [ ! -f "$LAND_TEST_LOG" ]
}

@test "ship handoff points to land wrapper, not bd close" {
  grep -q 'scripts/land.sh <bead-id>' "$REPO_ROOT/scripts/ship.sh"
  ! grep -q 'bd close' "$REPO_ROOT/scripts/ship.sh"
}

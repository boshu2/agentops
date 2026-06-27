#!/usr/bin/env bats
# agentops-2pl.8: branch-submit pushes cheap queue refs and appends FIFO requests.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SUBMIT="$REPO_ROOT/scripts/land-submit.sh"
  NEXT="$REPO_ROOT/scripts/land-queue-next.sh"
  PREPUSH="$REPO_ROOT/scripts/hooks/pre-push.local"
  PAWL_PREPUSH="$REPO_ROOT/scripts/check-pawl-pre-push.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  ORIGIN="$TMP/origin.git"
  REPO="$TMP/repo"
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
  git -C "$REPO" add README.md .gitignore
  git -C "$REPO" commit --quiet -m "init"
  git -C "$REPO" remote add origin "$ORIGIN"
  git -C "$REPO" push --quiet -u origin main

  mkdir -p "$REPO/.git/hooks"
  cat >"$REPO/.git/hooks/pre-push" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
repo="$(git rev-parse --show-toplevel)"
echo pre-push >>"$repo/.git/pre-push-count"
echo race-suite-marker >>"$repo/.git/pre-push-count"
exit 1
EOS
  chmod +x "$REPO/.git/hooks/pre-push"
  cd "$REPO" || return 1
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

make_branch_commit() {
  local branch="$1" bead="$2" file="$3"
  git -C "$REPO" checkout --quiet main
  git -C "$REPO" checkout --quiet -b "$branch"
  printf '%s\n' "$bead" >"$REPO/$file"
  git -C "$REPO" add "$file"
  git -C "$REPO" commit --quiet -m "fix(land-queue): submit $bead"
}

extract_function() {
  local name="$1" source="$2" dest="$3"
  awk -v name="$name" '
    $0 == name "() {" { in_fn=1 }
    in_fn { print }
    in_fn && $0 == "}" { exit }
  ' "$source" >"$dest"
}

@test "land-submit pushes land-queue refs without running pre-push and queues FIFO" {
  [ -x "$SUBMIT" ]
  [ -x "$NEXT" ]

  beads=(agentops-2pl.8a agentops-2pl.8b agentops-2pl.8c)
  files=(one.txt two.txt three.txt)
  branches=(work/one work/two work/three)

  for i in 0 1 2; do
    make_branch_commit "${branches[$i]}" "${beads[$i]}" "${files[$i]}"
    run env AGENTOPS_LAND_QUEUE_BACKEND=file "$SUBMIT" "${beads[$i]}"
    [ "$status" -eq 0 ]
    [[ "$output" == *"refs/heads/land-queue/${beads[$i]}"* ]]
    [[ "$output" != *"race-suite-marker"* ]]
  done

  for bead in "${beads[@]}"; do
    git -C "$ORIGIN" show-ref --verify --quiet "refs/heads/land-queue/$bead"
  done

  [ ! -f "$REPO/.git/pre-push-count" ]

  queue="$REPO/.agents/land-queue/requests.jsonl"
  [ -s "$queue" ]
  [ "$(wc -l <"$queue" | tr -d '[:space:]')" = "3" ]

  run jq -r '[.bead, .branch_ref] | @tsv' "$queue"
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | sed -n '1p')" = $'agentops-2pl.8a\trefs/heads/land-queue/agentops-2pl.8a' ]
  [ "$(printf '%s\n' "$output" | sed -n '2p')" = $'agentops-2pl.8b\trefs/heads/land-queue/agentops-2pl.8b' ]
  [ "$(printf '%s\n' "$output" | sed -n '3p')" = $'agentops-2pl.8c\trefs/heads/land-queue/agentops-2pl.8c' ]

  run jq -s -e '
    length == 3
    and all(.[]; has("timestamp") and has("author_family") and .backend == "file" and .status == "request")
    and (.[0].queue_seq == 1 and .[1].queue_seq == 2 and .[2].queue_seq == 3)
  ' "$queue"
  [ "$status" -eq 0 ]

  run env AGENTOPS_LAND_QUEUE_DIR="$REPO/.agents/land-queue" "$NEXT"
  [ "$status" -eq 0 ]
  [ "$output" = $'agentops-2pl.8a\trefs/heads/land-queue/agentops-2pl.8a' ]
}

@test "land-queue refs are not classified as main pushes" {
  stdin_file="$TMP/prepush.stdin"
  printf 'refs/heads/work %s refs/heads/land-queue/agentops-2pl.8a 0000000000000000000000000000000000000000\n' \
    "1111111111111111111111111111111111111111" >"$stdin_file"

  extract_function is_push_to_main "$PREPUSH" "$TMP/is_push_to_main.sh"
  cat >>"$TMP/is_push_to_main.sh" <<EOF
prepush_stdin="$stdin_file"
if is_push_to_main; then
  exit 1
fi
exit 0
EOF
  run bash "$TMP/is_push_to_main.sh"
  [ "$status" -eq 0 ]

  extract_function is_main_push "$PAWL_PREPUSH" "$TMP/is_main_push.sh"
  cat >>"$TMP/is_main_push.sh" <<'EOF'
is_main_push refs/heads/land-queue/agentops-2pl.8a && exit 1
is_main_push refs/heads/main || exit 1
is_main_push refs/heads/master || exit 1
exit 0
EOF
  run bash "$TMP/is_main_push.sh"
  [ "$status" -eq 0 ]
}

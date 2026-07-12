#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE_REPO/scripts" "$FIXTURE_REPO/cli/internal/rpi" "$FIXTURE_REPO/tools"
  cp "$REPO_ROOT/scripts/check-door9-no-claude-p.sh" "$FIXTURE_REPO/scripts/"

  git -C "$FIXTURE_REPO" init -q
  git -C "$FIXTURE_REPO" config user.email "door9-test@example.invalid"
  git -C "$FIXTURE_REPO" config user.name "Door 9 Test"

  printf 'package rpi\n' > "$FIXTURE_REPO/cli/internal/rpi/clean.go"
}

commit_fixture() {
  git -C "$FIXTURE_REPO" add -A
  git -C "$FIXTURE_REPO" commit -qm "fixture"
}

@test "tracked executable outside old roots fails LAW-0" {
  cat > "$FIXTURE_REPO/tools/legacy-worker" <<'EOF'
#!/usr/bin/env bash
claude -p "run the worker"
EOF
  chmod +x "$FIXTURE_REPO/tools/legacy-worker"
  commit_fixture

  run bash "$FIXTURE_REPO/scripts/check-door9-no-claude-p.sh"

  [ "$status" -ne 0 ]
  [[ "$output" == *"tools/legacy-worker"* ]]
}

@test "tracked executable indented invocation fails LAW-0" {
  cat > "$FIXTURE_REPO/tools/legacy-worker" <<'EOF'
#!/usr/bin/env bash
  claude -p "run the worker"
EOF
  chmod +x "$FIXTURE_REPO/tools/legacy-worker"
  commit_fixture

  run bash "$FIXTURE_REPO/scripts/check-door9-no-claude-p.sh"

  [ "$status" -ne 0 ]
  [[ "$output" == *"tools/legacy-worker"* ]]
}

@test "tracked executable bypass invocation fails LAW-0" {
  cat > "$FIXTURE_REPO/tools/legacy-worker" <<'EOF'
#!/usr/bin/env bash
claude --permission-mode bypassPermissions "run the worker"
EOF
  chmod +x "$FIXTURE_REPO/tools/legacy-worker"
  commit_fixture

  run bash "$FIXTURE_REPO/scripts/check-door9-no-claude-p.sh"

  [ "$status" -ne 0 ]
  [[ "$output" == *"tools/legacy-worker"* ]]
}

@test "historical prose and non-executable source mentions pass LAW-0" {
  cat > "$FIXTURE_REPO/history.md" <<'EOF'
Historical example: claude -p "old worker"
EOF
  cat > "$FIXTURE_REPO/tools/source-example.sh" <<'EOF'
claude --print "source-only example"
EOF
  chmod -x "$FIXTURE_REPO/tools/source-example.sh"
  commit_fixture

  run bash "$FIXTURE_REPO/scripts/check-door9-no-claude-p.sh"

  [ "$status" -eq 0 ]
}

@test "untracked executable passes LAW-0" {
  commit_fixture
  cat > "$FIXTURE_REPO/tools/local-worker" <<'EOF'
#!/usr/bin/env bash
claude -p "local worker"
EOF
  chmod +x "$FIXTURE_REPO/tools/local-worker"

  run bash "$FIXTURE_REPO/scripts/check-door9-no-claude-p.sh"

  [ "$status" -eq 0 ]
}

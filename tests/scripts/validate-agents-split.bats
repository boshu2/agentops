#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/validate-agents-split.sh"
  WORK_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$WORK_REPO/scripts" "$WORK_REPO/docs/architecture" "$WORK_REPO/docs/contracts"
  cp "$SCRIPT" "$WORK_REPO/scripts/"
  chmod +x "$WORK_REPO/scripts/validate-agents-split.sh"
}

write_valid_routes() {
  printf '# Agent contract\n' >"$WORK_REPO/AGENTS.md"
  printf '# Workflow\n' >"$WORK_REPO/docs/agent-workflow-reference.md"
  printf '# CI\n' >"$WORK_REPO/docs/CI-CD.md"
  printf '# Codex\n' >"$WORK_REPO/docs/contracts/codex-skill-api.md"
  printf '# Runtime\n' >"$WORK_REPO/docs/contracts/repo-execution-profile.md"
}

@test "passes with compact root and four canonical routes" {
  write_valid_routes
  run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"4 canonical on-demand routes"* ]]
}

@test "fails when AGENTS.md is missing" {
  write_valid_routes
  rm "$WORK_REPO/AGENTS.md"
  run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"AGENTS.md does not exist"* ]]
}

@test "fails when AGENTS.md exceeds the budget" {
  write_valid_routes
  for _ in $(seq 1 251); do echo filler >>"$WORK_REPO/AGENTS.md"; done
  run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"exceeds 250-line"* ]]
}

@test "fails when a canonical route is missing" {
  write_valid_routes
  rm "$WORK_REPO/docs/CI-CD.md"
  run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing or empty canonical route: docs/CI-CD.md"* ]]
}

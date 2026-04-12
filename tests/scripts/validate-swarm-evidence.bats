#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-swarm-evidence.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

init_fixture_repo() {
    local repo="$1"
    mkdir -p "$repo/.agents/swarm/results"
    git -C "$repo" init -q
    printf '.agents/\n' > "$repo/.gitignore"
}

write_invalid_completion() {
    local path="$1"
    cat > "$path" <<'JSON'
{
  "type": "completion",
  "status": "done",
  "artifacts": ["cli/cmd/ao/example.go"]
}
JSON
}

write_valid_completion() {
    local path="$1"
    cat > "$path" <<'JSON'
{
  "type": "completion",
  "status": "done",
  "artifacts": ["cli/cmd/ao/example.go"],
  "evidence": {
    "required_checks": ["unit"],
    "checks": {
      "unit": {
        "verdict": "PASS"
      }
    }
  }
}
JSON
}

@test "default scan ignores untracked local swarm evidence in git repos" {
    repo="$TMP_DIR/repo"
    init_fixture_repo "$repo"
    write_invalid_completion "$repo/.agents/swarm/results/legacy.json"

    cd "$repo"
    run bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"no tracked evidence files"* ]]
}

@test "explicit directory validation remains strict for untracked evidence" {
    repo="$TMP_DIR/repo"
    init_fixture_repo "$repo"
    write_invalid_completion "$repo/.agents/swarm/results/legacy.json"

    run bash "$SCRIPT" "$repo/.agents/swarm/results"

    [ "$status" -eq 1 ]
    [[ "$output" == *"completion result missing evidence block"* ]]
}

@test "default scan still validates tracked swarm evidence" {
    repo="$TMP_DIR/repo"
    init_fixture_repo "$repo"
    write_valid_completion "$repo/.agents/swarm/results/good.json"
    git -C "$repo" add -f .agents/swarm/results/good.json

    cd "$repo"
    run bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"EVIDENCE BATCH PASS"* ]]
}

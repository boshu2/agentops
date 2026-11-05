#!/usr/bin/env bats
# Fixtures under tests/fixtures/macos-no-runner/ are trimmed real
# `actions/runs/{id}/jobs` responses from boshu2/agentops:
#   41877244789  install-e2e, macos-install never got a runner
#   41865845321  fresh-install-conformance, tree-conformance (macos-latest)
#                never got a runner, release-conformance skipped
#   41087810466  windows-installers, a real step failure
#   41672041276  fresh-install-conformance, all green

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    CLASSIFY="$REPO_ROOT/scripts/macos-no-runner-classify.sh"
    FIXTURES="$REPO_ROOT/tests/fixtures/macos-no-runner"
    WORKFLOW="$REPO_ROOT/.github/workflows/macos-no-runner-retry.yml"
}

@test "install-e2e macOS no-runner cancel is retry-eligible" {
    run bash "$CLASSIFY" < "$FIXTURES/run-41877244789.json"
    [ "$status" -eq 0 ]
    [ "$output" = '{"bad":1,"no_runner":["macos-install"],"real":[]}' ]
}

@test "fresh-install-conformance macOS no-runner cancel is retry-eligible" {
    run bash "$CLASSIFY" < "$FIXTURES/run-41865845321.json"
    [ "$status" -eq 0 ]
    [ "$output" = '{"bad":1,"no_runner":["tree-conformance (macos-latest)"],"real":[]}' ]
}

@test "a real step failure is not retried" {
    run bash "$CLASSIFY" < "$FIXTURES/run-41087810466.json"
    [ "$status" -eq 1 ]
    [ "$output" = '{"bad":1,"no_runner":[],"real":["windows-installers"]}' ]
}

@test "a green run is not retried" {
    run bash "$CLASSIFY" < "$FIXTURES/run-41672041276.json"
    [ "$status" -eq 1 ]
    [ "$output" = '{"bad":0,"no_runner":[],"real":[]}' ]
}

@test "a no-runner macOS cancel next to a real failure is not retried" {
    run bash "$CLASSIFY" <<'JSON'
{"jobs":[
 {"name":"macos","conclusion":"cancelled","runner_id":0,"labels":["macos-latest"],"steps":[]},
 {"name":"ubuntu","conclusion":"failure","runner_id":7,"labels":["ubuntu-latest"],"steps":[{"name":"x"}]}
]}
JSON
    [ "$status" -eq 1 ]
    [ "$output" = '{"bad":2,"no_runner":["macos"],"real":["ubuntu"]}' ]
}

@test "a macOS job that ran steps before being cancelled is a real failure" {
    run bash "$CLASSIFY" <<'JSON'
{"jobs":[{"name":"macos","conclusion":"cancelled","runner_id":9,"labels":["macos-15"],"steps":[{"name":"Set up job"}]}]}
JSON
    [ "$status" -eq 1 ]
    [ "$output" = '{"bad":1,"no_runner":[],"real":["macos"]}' ]
}

@test "a non-macOS no-runner cancel is not retried" {
    run bash "$CLASSIFY" <<'JSON'
{"jobs":[{"name":"ubuntu","conclusion":"cancelled","runner_id":null,"labels":["ubuntu-latest"],"steps":[]}]}
JSON
    [ "$status" -eq 1 ]
    [ "$output" = '{"bad":1,"no_runner":[],"real":["ubuntu"]}' ]
}

@test "unreadable input exits 2" {
    run bash "$CLASSIFY" <<<'not json'
    [ "$status" -eq 2 ]
    run bash "$CLASSIFY" </dev/null
    [ "$status" -eq 2 ]
}

@test "watcher workflow names match the watched workflows exactly" {
    for wf in install-e2e fresh-install-conformance; do
        run grep -E "^name: ${wf}\$" "$REPO_ROOT/.github/workflows/${wf}.yml"
        [ "$status" -eq 0 ]
    done
    run grep -E '^    workflows: \[install-e2e, fresh-install-conformance\]$' "$WORKFLOW"
    [ "$status" -eq 0 ]
}

@test "watcher triggers on failed main runs, bounds attempts, and can rerun" {
    run grep -F "github.event.workflow_run.conclusion == 'failure'" "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -F "github.event.workflow_run.run_attempt < 4" "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -F "github.event.workflow_run.head_branch == 'main'" "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -E '^      actions: write$' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -F "bash scripts/macos-no-runner-classify.sh" "$WORKFLOW"
    [ "$status" -eq 0 ]
}

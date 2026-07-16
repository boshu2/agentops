#!/usr/bin/env bats
#
# Wave 2: doctrine-proof (host of three-gap-supergate CI steps) was collapsed
# into go-gate-shadow. These tests pin the collapse and keep go-gate-shadow as
# the CI backstop that still runs the Go registry (which includes three-gap
# checks via seed.go backing scripts).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/validate.yml"
}

@test "doctrine-proof purpose job is gone (collapsed into go-gate-shadow)" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW_PATH')); assert 'doctrine-proof' not in d['jobs']; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "go-gate-shadow remains and is required by summary" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW_PATH')); assert 'go-gate-shadow' in d['jobs']; assert 'go-gate-shadow' in d['jobs']['summary']['needs']; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "go-gate-shadow invokes ao gate check --full" {
    run bash -c "awk '/^  go-gate-shadow:/{p=1} p && /ao gate check/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ao gate check"* ]]
    [[ "$output" == *"--full"* ]]
}

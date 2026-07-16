#!/usr/bin/env bats
#
# Wave 2 cut-plan: the consolidated `skill-gates` purpose job was deleted;
# those scripts now run via go-gate-shadow (`ao gate check --full`). This
# file guards the collapse: skill-gates must stay gone, and summary.needs
# must not reintroduce it.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW="$REPO_ROOT/.github/workflows/validate.yml"
}

@test "validate.yml does not declare a skill-gates job (collapsed into go-gate-shadow)" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert 'skill-gates' not in d['jobs'], 'skill-gates must stay deleted'; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "summary.needs does not list skill-gates" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert 'skill-gates' not in d['jobs']['summary']['needs']; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "go-gate-shadow remains the CI backstop authority job" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert 'go-gate-shadow' in d['jobs']; assert 'go-gate-shadow' in d['jobs']['summary']['needs']; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/validate.yml"
}

@test "validate.yml runs on release tag pushes" {
    run grep -n "tags:" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -n "'v\\*'" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "changes job detects release tag pushes" {
    run grep -n "Detect release tag push" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -n 'refs/tags/v\*' "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "path filter is skipped on release tags" {
    run bash -c "awk '/dorny\\/paths-filter@v4/{p=NR} p && NR>=p && NR<=p+4' '$WORKFLOW_PATH' | grep -F \"if: steps.release.outputs.release != 'true'\""
    [ "$status" -eq 0 ]
}

@test "all changes outputs are forced true on release tags" {
    outputs=(go skills hooks docs eval codex shell bats ci contracts learning markdown)
    for output in "${outputs[@]}"; do
        run grep -F "      ${output}: \${{ steps.release.outputs.release == 'true' || steps.filter.outputs.${output} }}" "$WORKFLOW_PATH"
        [ "$status" -eq 0 ]
    done
}

@test "summary fails release tags with skipped jobs" {
    run grep -F "Release-tag Validate had skipped jobs" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -F "contains(needs.*.result, 'skipped')" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

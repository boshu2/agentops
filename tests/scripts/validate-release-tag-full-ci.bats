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

@test "summary fails release tags with unexpected skipped jobs" {
    run grep -F "Release-tag Validate had unexpected skipped jobs" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -F "skipped release lanes are not a release verdict" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "summary does not fail release tags on every skipped job blindly" {
    run grep -F "contains(needs.*.result, 'skipped')" "$WORKFLOW_PATH"
    [ "$status" -ne 0 ]
}

@test "summary uses a selective toJson(needs) allowlist, not a blind skip check" {
    # Post-rebuild (ag-877): the 67→10 collapse removed the standalone PR-only
    # jobs (validate-pr-evidence-claims, lint-evidence-lines-advisory) — AP#7
    # Evidence verification and the Evidence-line lint are now STEPS (in summary
    # and process-hygiene respectively), gated by github.event_name. So no
    # top-level job needs allowlisting on release tags. The selective mechanism
    # itself must still exist: summary inspects toJson(needs) against an
    # allowed_skips set rather than blindly failing on any skip.
    run grep -F "toJson(needs)" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -F "allowed_skips" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

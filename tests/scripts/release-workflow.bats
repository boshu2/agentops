#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW="$REPO_ROOT/.github/workflows/release.yml"
}

line_of() {
    local pattern="$1"
    grep -n "$pattern" "$WORKFLOW" | head -1 | cut -d: -f1
}

@test "release workflow gates publish on pre-publish evidence" {
    run grep -Fq 'pre-publish-evidence:' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -Fq 'needs: [doc-release-gate, pre-publish-evidence]' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -Fq "needs.pre-publish-evidence.result == 'success'" "$WORKFLOW"
    [ "$status" -eq 0 ]
}

@test "release workflow has no soft security publish bypass" {
    run grep -Fq 'continue-on-error: true' "$WORKFLOW"
    [ "$status" -eq 1 ]
    run grep -Fq "needs.doc-release-gate.result == 'success' && needs.pre-publish-evidence.result == 'success'" "$WORKFLOW"
    [ "$status" -eq 0 ]
}

@test "security and readiness evidence run before GoReleaser publish" {
    local security_line
    local readiness_line
    local publish_line

    security_line="$(line_of 'security-gate.sh --mode full --json')"
    readiness_line="$(line_of 'check-release-readiness.sh')"
    publish_line="$(line_of 'Publish with GoReleaser')"

    [ -n "$security_line" ]
    [ -n "$readiness_line" ]
    [ -n "$publish_line" ]
    [ "$security_line" -lt "$publish_line" ]
    [ "$readiness_line" -lt "$publish_line" ]
}

@test "release evidence is uploaded after publish from pre-publish artifact" {
    # Version-agnostic — dependabot bumps these majors over time. Assert
    # presence + the canonical artifact name + the publish step is wired.
    run grep -Eq 'actions/upload-artifact@v[0-9]+' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -Eq 'actions/download-artifact@v[0-9]+' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -Fq 'pre-publish-release-evidence' "$WORKFLOW"
    [ "$status" -eq 0 ]
    run grep -Fq 'gh release upload "$VERSION" release-artifacts/security-gate-summary.json --clobber' "$WORKFLOW"
    [ "$status" -eq 0 ]
}

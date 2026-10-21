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
    run bash -c "awk '/dorny\\/paths-filter@/{p=NR} p && NR>=p && NR<=p+4' '$WORKFLOW_PATH' | grep -F \"if: steps.release.outputs.release != 'true'\""
    [ "$status" -eq 0 ]
}

@test "all changes outputs are forced true on release tags" {
    outputs=(go skills hooks docs shell bats ci contracts goals learning markdown corpus)
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
    # No PR-only lifecycle or evidence-admission job is part of this workflow,
    # so no top-level job needs allowlisting on release tags. The selective
    # mechanism itself must still exist: summary inspects toJson(needs) against an
    # allowed_skips set rather than blindly failing on any skip.
    run grep -F "toJson(needs)" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]

    run grep -F "allowed_skips" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

# Execute the shipped step shell, read from the parsed workflow
# (yaml.safe_load) rather than grepped from its layout. A step whose run text
# interpolates `${{ }}` cannot be executed outside Actions, so the helper
# rejects it: expression values reach the shell only through env.
extract_run() {
    local job="$1" step_name="$2" out="$3"
    python3 -c '
import sys

import yaml

path, job, name, out = sys.argv[1:5]
with open(path, encoding="utf-8") as handle:
    workflow = yaml.safe_load(handle)
found = [s for s in workflow["jobs"][job]["steps"] if s.get("name") == name]
assert len(found) == 1, f"{job}: expected one step named {name!r}, found {len(found)}"
text = found[0]["run"]
assert "${{" not in text, f"{job}/{name}: run text interpolates an expression: {text!r}"
with open(out, "w", encoding="utf-8") as handle:
    handle.write(text)
' "$WORKFLOW_PATH" "$job" "$step_name" "$out"
}

# needs_json <full> <go-gate-shadow> <correctness> <security>: the summary
# job's toJson(needs) shape, with changes succeeding.
needs_json() {
    printf '{"changes":{"result":"success","outputs":{"full":"%s"}},"go-gate-shadow":{"result":"%s","outputs":{}},"correctness":{"result":"%s","outputs":{}},"security":{"result":"%s","outputs":{}}}' "$@"
}

# run_summary <needs-json> <github-ref> <github-event-name>
run_summary() {
    extract_run summary "Check results" "$BATS_TEST_TMPDIR/summary.sh"
    run env NEEDS_JSON="$1" GITHUB_REF="$2" GITHUB_EVENT_NAME="$3" \
        bash -e "$BATS_TEST_TMPDIR/summary.sh"
    echo "$output" >&2
}

@test "summary fails a pull request whose lanes were cancelled and names them" {
    run_summary "$(needs_json false cancelled cancelled cancelled)" refs/pull/1/merge pull_request
    [ "$status" -eq 1 ]
    [[ "$output" == *"- go-gate-shadow: cancelled"* ]]
    [[ "$output" == *"- correctness: cancelled"* ]]
    [[ "$output" == *"- security: cancelled"* ]]
}

@test "summary fails a pull request with a failed lane" {
    run_summary "$(needs_json false success failure success)" refs/pull/1/merge pull_request
    [ "$status" -eq 1 ]
    [[ "$output" == *"- correctness: failure"* ]]
}

@test "summary fails a pull request with a timed-out lane" {
    run_summary "$(needs_json false success timed_out success)" refs/pull/1/merge pull_request
    [ "$status" -eq 1 ]
    [[ "$output" == *"- correctness: timed_out"* ]]
}

@test "summary passes a pull request whose lanes were skipped by the path filter" {
    run_summary "$(needs_json false success skipped skipped)" refs/pull/1/merge pull_request
    [ "$status" -eq 0 ]
    [[ "$output" == *"All checks passed"* ]]
}

@test "summary fails a release tag with a skipped lane" {
    run_summary "$(needs_json true success success skipped)" refs/tags/v9.9.9 push
    [ "$status" -eq 1 ]
    [[ "$output" == *"Release-tag Validate had unexpected skipped jobs; skipped release lanes are not a release verdict"* ]]
    [[ "$output" == *"- security"* ]]
}

@test "summary passes a release tag whose every lane succeeded" {
    run_summary "$(needs_json true success success success)" refs/tags/v9.9.9 push
    [ "$status" -eq 0 ]
    [[ "$output" == *"All checks passed"* ]]
}

@test "summary fails a full manual run with a skipped lane" {
    run_summary "$(needs_json true success skipped success)" refs/heads/main workflow_dispatch
    [ "$status" -eq 1 ]
    [[ "$output" == *"unexpected skipped jobs"* ]]
    [[ "$output" == *"- correctness"* ]]
}

@test "changes forces a full run on manual dispatch and not on pull requests" {
    extract_run changes "Detect release tag push or merge-queue (force full run)" "$BATS_TEST_TMPDIR/detect.sh"

    : > "$BATS_TEST_TMPDIR/dispatch.out"
    run env GITHUB_REF=refs/heads/main GITHUB_EVENT_NAME=workflow_dispatch \
        GITHUB_OUTPUT="$BATS_TEST_TMPDIR/dispatch.out" bash -e "$BATS_TEST_TMPDIR/detect.sh"
    [ "$status" -eq 0 ]
    run grep -Fx "release=true" "$BATS_TEST_TMPDIR/dispatch.out"
    [ "$status" -eq 0 ]

    : > "$BATS_TEST_TMPDIR/pr.out"
    run env GITHUB_REF=refs/pull/1/merge GITHUB_EVENT_NAME=pull_request \
        GITHUB_OUTPUT="$BATS_TEST_TMPDIR/pr.out" bash -e "$BATS_TEST_TMPDIR/detect.sh"
    [ "$status" -eq 0 ]
    run grep -Fx "release=false" "$BATS_TEST_TMPDIR/pr.out"
    [ "$status" -eq 0 ]
}

@test "validate.yml keeps tag-only pushes, the summary needs and exposes the full flag" {
    run python3 -c '
import sys

import yaml

with open(sys.argv[1], encoding="utf-8") as handle:
    text = handle.read()
workflow = yaml.safe_load(text)
# PyYAML (YAML 1.1) reads the bare `on` key as boolean True.
triggers = workflow.get("on", workflow.get(True))
assert triggers["push"] == {"tags": ["v*"]}, triggers["push"]
jobs = workflow["jobs"]
assert jobs["summary"]["needs"] == ["changes", "go-gate-shadow", "correctness", "security"], jobs["summary"]["needs"]
assert jobs["changes"]["outputs"]["full"] == "${{ steps.release.outputs.release }}", jobs["changes"]["outputs"]
assert "skill-gates" not in text, "stale skill-gates comment remains"
print("ok")
' "$WORKFLOW_PATH"
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

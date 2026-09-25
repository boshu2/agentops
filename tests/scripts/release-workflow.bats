#!/usr/bin/env bats
# Release publish boundary, read from the parsed workflow (yaml.safe_load), not
# from its text layout: publish waits on the doc-release and pre-publish
# evidence jobs with no soft-fail path, security and readiness evidence run in
# a job publish waits on, curated notes are validated and extracted before
# GoReleaser and applied after it, and the evidence artifact published by
# pre-publish-evidence is attached to the release after GoReleaser.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW="$REPO_ROOT/.github/workflows/release.yml"
}

# Run a Python assertion block against the parsed workflow. The block sees
# `jobs`, `publish`, `needs` (publish's needs as a list), `steps(job)`,
# `runs(job)` ([(index, run-text)]), `uses_index(job, prefix)` and
# `goreleaser` (the index of publish's goreleaser-action step).
check_workflow() {
    local prelude
    prelude="$(cat <<'PY'
import sys

import yaml

with open(sys.argv[1], encoding="utf-8") as handle:
    workflow = yaml.safe_load(handle)
jobs = workflow["jobs"]
publish = jobs["publish"]
needs = publish.get("needs", [])
needs = [needs] if isinstance(needs, str) else list(needs)


def steps(job):
    return jobs[job].get("steps", [])


def runs(job):
    return [(i, step.get("run", "")) for i, step in enumerate(steps(job))]


def uses_index(job, prefix):
    found = [i for i, step in enumerate(steps(job)) if str(step.get("uses", "")).startswith(prefix)]
    assert len(found) == 1, f"{job}: expected one {prefix} step, found {found}"
    return found[0]


goreleaser = uses_index("publish", "goreleaser/goreleaser-action@")
PY
)"
    run python3 -c "$prelude
$1" "$WORKFLOW"
    echo "$output" >&2
    [ "$status" -eq 0 ]
}

@test "publish waits on doc-release and pre-publish evidence with no soft-fail path" {
    check_workflow '
assert {"doc-release-gate", "pre-publish-evidence"} <= set(needs), needs
condition = str(publish.get("if", ""))
for escape in ("always()", "failure()", "cancelled()"):
    assert escape not in condition, f"publish if: {condition!r} contains {escape}"
for name, job in jobs.items():
    items = [("job", job)] + [(f"step {i}", s) for i, s in enumerate(job.get("steps", []))]
    for where, item in items:
        soft = item.get("continue-on-error", False)
        assert soft in (False, "false"), f"{name} {where}: continue-on-error={soft!r}"
'
}

@test "security and readiness evidence run in a job publish waits on" {
    check_workflow '
for script in ("security-gate.sh", "check-release-readiness.sh"):
    homes = [job for job in needs if job in jobs and any(script in text for _, text in runs(job))]
    homes += ["publish"] if any(script in text for i, text in runs("publish") if i < goreleaser) else []
    assert homes, f"{script} runs in no job publish needs and in no publish step before GoReleaser"
security = [line.split() for job in needs if job in jobs for _, text in runs(job)
            for line in text.splitlines() if "security-gate.sh" in line]
assert any(tokens[tokens.index("--mode") + 1] == "full" for tokens in security
           if "--mode" in tokens[:-1]), f"security gate is not run with --mode full: {security}"
'
}

@test "curated release notes are validated and extracted before GoReleaser and applied after it" {
    check_workflow '
def at(script):
    return [i for i, text in runs("publish") if script in text]
assert any(i < goreleaser for i in at("validate-release-notes.sh")), at("validate-release-notes.sh")
assert any(i < goreleaser for i in at("extract-release-notes.sh")), at("extract-release-notes.sh")
applied = [i for i, text in runs("publish")
           if "gh release edit" in text and "--notes-file release-notes.md" in text]
assert any(i > goreleaser for i in applied), f"notes applied at {applied}, GoReleaser at {goreleaser}"
'
}

@test "pre-publish evidence artifact is attached to the release after GoReleaser" {
    check_workflow '
uploaded = uses_index("pre-publish-evidence", "actions/upload-artifact@")
name = steps("pre-publish-evidence")[uploaded]["with"]["name"]
downloaded = uses_index("publish", "actions/download-artifact@")
assert steps("publish")[downloaded]["with"]["name"] == name, (name, steps("publish")[downloaded])
attached = [i for i, text in runs("publish")
            if "gh release upload" in text and "security-gate-summary.json" in text]
assert any(downloaded < goreleaser < i for i in attached), (downloaded, goreleaser, attached)
'
}

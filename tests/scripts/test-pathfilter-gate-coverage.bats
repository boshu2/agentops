#!/usr/bin/env bats
#
# Drift-blocking test for the path-filter <-> gate coverage invariant in
# .github/workflows/validate.yml.
#
# Wave 2 collapsed purpose jobs into go-gate-shadow; surviving consumers of
# path-filter outputs are correctness + security (+ go-gate-shadow always runs).
# These tests keep the filter definitions honest for those consumers and for
# the redteam/AGENTS coverage invariants that still live in the changes job.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/validate.yml"
    PACK_PATH="$REPO_ROOT/skills/security/references/agentops-redteam-pack.json"
}

@test "contracts filter is a superset of every redteam-pack target glob" {
    run python3 - "$WORKFLOW_PATH" "$PACK_PATH" <<'PY'
import json, sys, fnmatch, yaml

workflow_path, pack_path = sys.argv[1], sys.argv[2]

with open(pack_path) as f:
    pack = json.load(f)
targets = set()
for case in pack.get("cases", []):
    for tgt in case.get("targets", []):
        for g in tgt.get("globs", []):
            targets.add(g)

with open(workflow_path) as f:
    doc = yaml.safe_load(f)
filt_step = next(
    s for s in doc["jobs"]["changes"]["steps"] if s.get("id") == "filter"
)
filters = yaml.safe_load(filt_step["with"]["filters"])
contracts = filters["contracts"]

def covered(target, patterns):
    for p in patterns:
        if p == target:
            return True
        if p.endswith("/**"):
            base = p[:-3]
            if target == base or target.startswith(base + "/"):
                return True
        if fnmatch.fnmatch(target, p):
            return True
    return False

uncovered = sorted(t for t in targets if not covered(t, contracts))
if uncovered:
    print("UNCOVERED redteam-pack targets:", uncovered)
    sys.exit(1)
print("ok: all redteam-pack targets covered by contracts filter:", sorted(targets))
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok: all redteam-pack targets covered"* ]]
}

@test "validate.yml declares a goals: filter under the changes job" {
    run grep -E "^            goals:" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "goals filter covers GOALS.md and spec/scenarios" {
    run python3 - "$WORKFLOW_PATH" <<'PY'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)
filt_step = next(
    s for s in doc["jobs"]["changes"]["steps"] if s.get("id") == "filter"
)
filters = yaml.safe_load(filt_step["with"]["filters"])
goals = filters.get("goals", [])
required = {"GOALS.md", "spec/scenarios/**"}
missing = sorted(required - set(goals))
if missing:
    print("MISSING goals globs:", missing)
    sys.exit(1)
print("ok: goals filter covers", sorted(goals))
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok: goals filter covers"* ]]
}

@test "changes job exposes goals output" {
    run grep -F "      goals: \${{ steps.release.outputs.release == 'true' || steps.filter.outputs.goals }}" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "contracts filter covers AGENTS.md and every current split owner" {
    SPLIT_SCRIPT="$REPO_ROOT/scripts/validate-agents-split.sh"
    run python3 - "$WORKFLOW_PATH" "$SPLIT_SCRIPT" <<'PY'
import sys, re, fnmatch, yaml

workflow_path, split_path = sys.argv[1], sys.argv[2]

with open(split_path) as f:
    src = f.read()
owners_match = re.search(r'readonly OWNERS=\(\n(.*?)\n\)', src, re.DOTALL)
if not owners_match:
    print("FAIL: could not parse OWNERS from split script")
    sys.exit(1)
owners = re.findall(r'^\s*(docs/[^\s]+)\s*$', owners_match.group(1), re.MULTILINE)
agents_files = sorted({"AGENTS.md", *owners})

with open(workflow_path) as f:
    doc = yaml.safe_load(f)
filt_step = next(
    s for s in doc["jobs"]["changes"]["steps"] if s.get("id") == "filter"
)
filters = yaml.safe_load(filt_step["with"]["filters"])
contracts = filters["contracts"]

def covered(target, patterns):
    for p in patterns:
        if p == target:
            return True
        if p.endswith("/**"):
            base = p[:-3]
            if target == base or target.startswith(base + "/"):
                return True
        if fnmatch.fnmatch(target, p):
            return True
    return False

uncovered = sorted(t for t in agents_files if not covered(t, contracts))
if uncovered:
    print("UNCOVERED AGENTS split targets:", uncovered)
    sys.exit(1)
print("ok: AGENTS.md and split owners covered by contracts filter:", agents_files)
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok: AGENTS.md and split owners covered"* ]]
}

@test "surviving jobs still consume changes outputs (correctness + security)" {
    run python3 - "$WORKFLOW_PATH" <<'PY'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)
jobs = doc["jobs"]
assert "correctness" in jobs and "security" in jobs
for name in ("correctness", "security"):
    cond = jobs[name].get("if") or ""
    assert "needs.changes.outputs" in cond, f"{name} missing changes gating: {cond}"
print("ok: correctness+security gated on changes")
PY
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok: correctness+security gated on changes"* ]]
}

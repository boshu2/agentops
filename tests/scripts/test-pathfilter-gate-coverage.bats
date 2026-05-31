#!/usr/bin/env bats
#
# Drift-blocking test for the path-filter <-> gate coverage invariant in
# .github/workflows/validate.yml. A correctness/contract gate that reads a
# specific set of files MUST be triggered by a change-filter that covers those
# files; otherwise an edit to a guarded file SKIPS the very gate that guards it,
# and `summary` (a required branch-protection check) goes green-but-incomplete.
#
# Two concrete gaps this guards (both real regressions):
#
#  ag-nl1u (#634): the security-suite redteam pack
#    (skills/security-suite/references/agentops-redteam-pack.json) asserts
#    behavioral contracts against AGENTS.md + several docs/ + skills/security*
#    files via the contracts-sync canaries. The `contracts` filter previously
#    only matched schemas/** + docs/contracts/**, so editing a guarded file
#    skipped the canary that guards it. INVARIANT: every redteam-pack target
#    glob must be covered by the `contracts` filter.
#
#  ag-n4m7 (#591/#593): the directive<->scenario correctness gates police
#    GOALS.md + spec/scenarios/**, but ran only on go/docs/ci -> a GOALS.md-only
#    edit citing a phantom scenario merged with the gate SKIPPED. INVARIANT: a
#    `goals` filter exists (GOALS.md + spec/scenarios/**) and the spec-link
#    gates trigger on it.
#
# Sibling pattern: tests/scripts/test-bats-path-filter-wiring.bats — grep/parse
# the artifact-under-test and assert the expected wiring is present.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/validate.yml"
    PACK_PATH="$REPO_ROOT/skills/security-suite/references/agentops-redteam-pack.json"
}

# ── ag-nl1u: redteam-pack targets are all covered by the `contracts` filter ──

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
    # A pack target is covered if some contracts glob equals it OR a
    # directory-glob pattern (foo/**) is a prefix of the target path.
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

@test "contracts-sync job triggers on needs.changes.outputs.contracts" {
    run bash -c "awk '/^  contracts-sync:/{inblock=1} inblock && /needs.changes.outputs.contracts/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.contracts"* ]]
}

# ── ag-n4m7: a `goals` filter exists and the spec-link gates trigger on it ──

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

@test "directive-to-scenario link lint triggers on needs.changes.outputs.goals" {
    run bash -c "awk '/name: Directive-to-scenario link lint/{inblock=1} inblock && /^        if:/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.goals == 'true'"* ]]
}

@test "scenario-test linkage gate triggers on needs.changes.outputs.goals" {
    run bash -c "awk '/name: Scenario.*linkage gate/{inblock=1} inblock && /^        if:/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.goals == 'true'"* ]]
}

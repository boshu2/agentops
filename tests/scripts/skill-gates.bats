#!/usr/bin/env bats
#
# Acceptance surface for the consolidated `skill-gates` job in
# .github/workflows/validate.yml (ag-87sv). The job is a PURE REGROUP of
# already-live skill-authoring gates into one named, REQUIRED check:
#
#   - heal.sh --strict                  (skills/heal-skill/scripts/heal.sh)
#   - validate-skill-schema.sh
#   - validate-skill-frontmatter.sh
#   - validate-skill-body-refs.sh
#   - validate-skill-flow.sh
#   - check-scenario-test-linkage.sh
#   - regen-all.sh --check              (six-surface derived-artifact drift)
#
# These assertions guard two invariants:
#   1. the job exists, runs every listed gate, and is REQUIRED (in summary.needs)
#   2. the six-surface drift sweep is wired such that a regeneration drift turns
#      the gate RED, and reverting the drift turns it GREEN.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW="$REPO_ROOT/.github/workflows/validate.yml"
}

# Helper: dump the list of `run:` script invocations for a named job via PyYAML.
job_run_blob() {
    local job="$1"
    python3 - "$WORKFLOW" "$job" <<'PY'
import sys, yaml
wf, job = sys.argv[1], sys.argv[2]
doc = yaml.safe_load(open(wf))
steps = doc["jobs"][job]["steps"]
for s in steps:
    if "run" in s:
        print(s["run"])
PY
}

# ── 1. job presence + every listed gate is a step ──────────────────────────

@test "validate.yml parses and declares a skill-gates job" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert 'skill-gates' in d['jobs'], 'no skill-gates job'; print('ok')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ok"* ]]
}

@test "skill-gates runs heal.sh --strict" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"skills/heal-skill/scripts/heal.sh --strict"* ]]
}

@test "skill-gates runs validate-skill-schema.sh" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/validate-skill-schema.sh"* ]]
}

@test "skill-gates runs validate-skill-frontmatter.sh" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/validate-skill-frontmatter.sh"* ]]
}

@test "skill-gates runs validate-skill-body-refs.sh" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/validate-skill-body-refs.sh"* ]]
}

@test "skill-gates runs validate-skill-flow.sh" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/validate-skill-flow.sh"* ]]
}

@test "skill-gates runs check-scenario-test-linkage.sh" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/check-scenario-test-linkage.sh"* ]]
}

@test "skill-gates runs the six-surface drift sweep regen-all.sh --check" {
    run job_run_blob skill-gates
    [ "$status" -eq 0 ]
    [[ "$output" == *"scripts/regen-all.sh --check"* ]]
}

# ── 2. the job is REQUIRED, not advisory ───────────────────────────────────

@test "skill-gates is in summary.needs (required check)" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert 'skill-gates' in d['jobs']['summary']['needs'], 'not required'; print('required')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"required"* ]]
}

@test "skill-gates is NOT marked continue-on-error (no whole-job advisory)" {
    run python3 -c "import yaml; d=yaml.safe_load(open('$WORKFLOW')); assert d['jobs']['skill-gates'].get('continue-on-error') is not True; print('blocking')"
    [ "$status" -eq 0 ]
    [[ "$output" == *"blocking"* ]]
}

@test "no step in skill-gates references the phantom scenario-hash-stability gate" {
    run grep -n "scenario-hash-stability" "$WORKFLOW"
    [ "$status" -ne 0 ]
}

# ── 3. simulated regen drift turns the sweep RED; reverting goes GREEN ──────
#
# Exercises the actual SKU-catalog drift validator that `regen-all.sh --check`
# invokes. We mutate the committed registry.json (drop the skill-gates gate
# entry), assert the validator fails, then restore and assert it passes. The
# registry is restored unconditionally in teardown.

@test "removing the skill-gates gate from registry.json turns the drift sweep RED, restoring goes GREEN" {
    local reg="$REPO_ROOT/registry.json"
    local bak; bak="$(mktemp)"
    cp "$reg" "$bak"

    # Build a usable ao binary once so the SKU regeneration is deterministic.
    local ao_bin="$REPO_ROOT/cli/bin/ao"
    if [ ! -x "$ao_bin" ]; then
        ( cd "$REPO_ROOT/cli" && go build -o bin/ao ./cmd/ao )
    fi

    # GREEN baseline: committed registry matches regeneration.
    run env AGENTOPS_AO_BIN="$ao_bin" bash "$REPO_ROOT/scripts/validate-sku-catalog-drift.sh"
    local green_status="$status"

    # Induce drift: strip the skill-gates gate capability.
    python3 - "$reg" <<'PY'
import json, sys
p = sys.argv[1]
d = json.load(open(p))
d["capabilities"] = [c for c in d["capabilities"] if c.get("sku") != "gate:skill-gates"]
json.dump(d, open(p, "w"), indent=2)
PY
    run env AGENTOPS_AO_BIN="$ao_bin" bash "$REPO_ROOT/scripts/validate-sku-catalog-drift.sh"
    local red_status="$status"

    # Restore.
    cp "$bak" "$reg"
    rm -f "$bak"
    run env AGENTOPS_AO_BIN="$ao_bin" bash "$REPO_ROOT/scripts/validate-sku-catalog-drift.sh"
    local restored_status="$status"

    [ "$green_status" -eq 0 ]
    [ "$red_status" -ne 0 ]
    [ "$restored_status" -eq 0 ]
}

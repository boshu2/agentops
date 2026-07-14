#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCHEMA="$REPO_ROOT/skills/rpi/schemas/run-disposition.schema.json"
  FIXTURE="$BATS_TEST_TMPDIR/run-disposition.json"
  CHECKPOINT="$BATS_TEST_TMPDIR/wave-checkpoint.json"

  printf '%s\n' '{
    "schema_version": 1,
    "run_id": "rpi-fixture",
    "objective": {
      "identity": "age-fixture.1",
      "digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    },
    "disposition": "REPAIR",
    "reason": "introduced acceptance defect",
    "evidence_refs": [
      {
        "path": ".agents/evidence/fixture.json",
        "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
      }
    ],
    "blocker_class": "acceptance",
    "recorded_at": "2026-07-14T12:00:00Z"
  }' > "$FIXTURE"

  jq -n --arg sha "$(git -C "$REPO_ROOT" rev-parse HEAD)" '{
    schema_version: 1,
    wave: 1,
    timestamp: "2026-07-14T12:00:00Z",
    tasks_completed: ["age-fixture.1"],
    tasks_failed: [],
    files_changed: ["skills/crank/references/plan-mutations.md"],
    git_sha: $sha,
    acceptance_verdict: "PASS",
    commit_strategy: "lead-only",
    mutations_this_wave: 1,
    total_mutations: 1
  }' > "$CHECKPOINT"
}

validate_fixture() {
  python3 - "$SCHEMA" "$FIXTURE" <<'PY'
import json
import pathlib
import sys

import jsonschema

schema = json.loads(pathlib.Path(sys.argv[1]).read_text())
instance = json.loads(pathlib.Path(sys.argv[2]).read_text())
jsonschema.validate(instance=instance, schema=schema)
PY
}

@test "run disposition is one closed evidence-bound record" {
  run validate_fixture
  [ "$status" -eq 0 ]
}

@test "only NOTE REPAIR REPLAN HOLD and ANDON are dispositions" {
  local disposition
  for disposition in NOTE REPAIR REPLAN HOLD ANDON; do
    jq --arg disposition "$disposition" '.disposition = $disposition' "$FIXTURE" > "$FIXTURE.next"
    mv "$FIXTURE.next" "$FIXTURE"
    run validate_fixture
    [ "$status" -eq 0 ]
  done

  jq '.disposition = "PARTIAL"' "$FIXTURE" > "$FIXTURE.next"
  mv "$FIXTURE.next" "$FIXTURE"
  run validate_fixture
  [ "$status" -ne 0 ]
}

@test "controller and helper state cannot enter a disposition record" {
  local field
  for field in limits usage authorized admissions charges helper_history; do
    jq --arg field "$field" '.[$field] = {}' "$FIXTURE" > "$FIXTURE.next"
    mv "$FIXTURE.next" "$FIXTURE"
    run validate_fixture
    [ "$status" -ne 0 ]
    jq --arg field "$field" 'del(.[$field])' "$FIXTURE" > "$FIXTURE.next"
    mv "$FIXTURE.next" "$FIXTURE"
  done

  jq '.helper = {"allowed": true}' "$FIXTURE" > "$FIXTURE.next"
  mv "$FIXTURE.next" "$FIXTURE"
  run validate_fixture
  [ "$status" -ne 0 ]
}

@test "source and Codex controller artifacts are deleted" {
  local path
  for path in \
    schemas/validation-budget-receipt.v1.schema.json \
    skills/rpi/schemas/run-governor.schema.json \
    skills/rpi/scripts/run-governor.py \
    skills/validate/scripts/validation-budget.py \
    tests/scripts/rpi-run-governor.bats \
    tests/scripts/validation-budget.bats \
    skills-codex/rpi/schemas/run-governor.schema.json \
    skills-codex/rpi/scripts/run-governor.py \
    skills-codex/validate/scripts/validation-budget.py; do
    [ ! -e "$REPO_ROOT/$path" ]
  done
  [ -f "$REPO_ROOT/skills-codex/rpi/schemas/run-disposition.schema.json" ]
}

@test "plan mutation checkpoints record facts without a hidden budget" {
  local validator
  for validator in \
    skills/crank/scripts/validate-wave-checkpoint.sh \
    skills-codex/crank/scripts/validate-wave-checkpoint.sh; do
    run bash "$REPO_ROOT/$validator" "$CHECKPOINT" "$REPO_ROOT"
    [ "$status" -eq 0 ]

    local field
    for field in mutation_budget mutation_limits limits usage; do
      jq --arg field "$field" '.[$field] = {"used": 1, "limit": 3}' \
        "$CHECKPOINT" > "$CHECKPOINT.next"
      run bash "$REPO_ROOT/$validator" "$CHECKPOINT.next" "$REPO_ROOT"
      [ "$status" -ne 0 ]
    done
  done

  run rg -n -i \
    'mutation[_ -]?budget|enforces? budgets?|[0-9]+ per[- ]epic|\| unlimited \||"(used|limit)"' \
    "$REPO_ROOT/skills/crank/references/plan-mutations.md" \
    "$REPO_ROOT/skills-codex/crank/references/plan-mutations.md"
  [ "$status" -eq 1 ]
}

@test "RPI Crank and Validate require dispositions without phase-local admission" {
  local consumers=(
    skills/rpi/SKILL.md
    skills/rpi/references/agile-replan-loop.md
    skills/rpi/references/error-handling.md
    skills/rpi/references/gate-retry-logic.md
    skills/rpi/references/gate4-loop-and-spawn.md
    skills/rpi/references/isolation-contract.md
    skills/rpi/references/orchestrator-compression-anti-pattern.md
    skills/rpi/references/phase-budgets.md
    skills/rpi/references/phase-data-contracts.md
    skills/rpi/references/pull-flow-governor.md
    skills/rpi/references/rpi.feature
    skills/rpi/references/troubleshooting.md
    skills/crank/SKILL.md
    skills/crank/references/crank.feature
    skills/crank/references/execution-preflight.md
    skills/crank/references/external-gate-protocol.md
    skills/crank/references/failure-recovery.md
    skills/crank/references/failure-taxonomy.md
    skills/crank/references/plan-mutations.md
    skills/crank/references/test-first-mode.md
    skills/crank/references/troubleshooting.md
    skills/crank/references/wave-dispatch.md
    skills/crank/references/wave-patterns.md
    skills/crank/references/wave1-spec-consistency-checklist.md
    skills/crank/scripts/validate-wave-checkpoint.sh
    skills/validate/SKILL.md
    skills/validate/references/canonical-validation-protocol.md
    skills/validate/references/validate.feature
  )

  run rg -n -i \
    'run-governor|validation-budget|persistent (run )?governor|durable .*admission|authorized:? ?true|AUTHORIZED receipt|request(s|ed)? .*admission|run-wide .*ceiling|phase-local .*budget' \
    "${consumers[@]/#/$REPO_ROOT/}"
  [ "$status" -eq 1 ]

  run rg -n 'NOTE.*REPAIR.*REPLAN.*HOLD.*ANDON' "$REPO_ROOT/skills/rpi/SKILL.md"
  [ "$status" -eq 0 ]
}

#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  VALIDATOR="$REPO_ROOT/skills/premortem/scripts/validate-output.sh"
  TMP_ROOT="$(mktemp -d)"
  mkdir -p "$TMP_ROOT/plans"
  printf '# Exact plan\n\nOne bounded behavior.\n' >"$TMP_ROOT/plans/exact.md"
  PLAN_SHA="$(shasum -a 256 "$TMP_ROOT/plans/exact.md" | awk '{print $1}')"
}

teardown() {
  rm -rf "$TMP_ROOT"
}

write_verdict() {
  local verdict="$1" blockers="$2" complete="$3" author="${4:-author-1}" judge="${5:-judge-1}"
  cat >"$TMP_ROOT/verdict.json" <<JSON
{
  "schema_version": "premortem-plan-verdict.v1",
  "plan": {"path": "plans/exact.md", "sha256": "$PLAN_SHA"},
  "author_id": "$author",
  "judge_id": "$judge",
  "verdict": "$verdict",
  "blockers_complete": $complete,
  "blockers": $blockers
}
JSON
}

@test "Premortem validates one exact-plan PASS and one complete FAIL blocker set" {
  [ -x "$VALIDATOR" ]
  [ -f "$REPO_ROOT/skills/premortem/schemas/plan-verdict.schema.json" ]

  write_verdict PASS '[]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -eq 0 ]

  write_verdict FAIL '[{"id":"B1","claim":"write scope is incomplete","evidence":["plans/exact.md"]}]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -eq 0 ]
}

@test "Premortem rejects stale plans, self-grading, WARN, and incomplete blocker sets" {
  write_verdict PASS '[]' true
  printf '\nchanged\n' >>"$TMP_ROOT/plans/exact.md"
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]

  printf '# Exact plan\n\nOne bounded behavior.\n' >"$TMP_ROOT/plans/exact.md"
  write_verdict PASS '[]' true same same
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]

  write_verdict WARN '[]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]

  write_verdict PASS '[{"id":"B1","claim":"must be empty","evidence":["plans/exact.md"]}]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]

  write_verdict FAIL '[]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]

  write_verdict FAIL '[{"id":"B1","claim":"not complete","evidence":["plans/exact.md"]}]' false
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -ne 0 ]
}

@test "model and family metadata are optional and never impose a family floor" {
  write_verdict PASS '[]' true
  run "$VALIDATOR" "$TMP_ROOT/verdict.json" "$TMP_ROOT"
  [ "$status" -eq 0 ]

  jq '.author_model={"name":"codex","family":"openai"} | .judge_model={"name":"codex-fresh","family":"openai"}' \
    "$TMP_ROOT/verdict.json" >"$TMP_ROOT/with-model.json"
  run "$VALIDATOR" "$TMP_ROOT/with-model.json" "$TMP_ROOT"
  [ "$status" -eq 0 ]
}

@test "Goal Design checks deterministic packet shape without semantic validation state" {
  run bash "$REPO_ROOT/skills/goal-design/scripts/validate.sh"
  [ "$status" -eq 0 ]

  run rg -n '/validate|mark-validated|Last validation verdict|cross-family|council helper' \
    "$REPO_ROOT/skills/goal-design/SKILL.md" "$REPO_ROOT/scripts/goal-design-packet.py" \
    "$REPO_ROOT/scripts/check-goal-design-packet.sh"
  [ "$status" -eq 1 ]
}

@test "Dueling Idea Genies is advisory Plan input and never a readiness verdict" {
  challenge="$TMP_ROOT/challenge.json"
  cat >"$challenge" <<'JSON'
{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["port ownership"],"refutations":[{"claim":"P1","attempt":"existing seam","result":"survived"}],"handoff":{"owner":"plan","artifact_dir":".agents/ideas/run-1"}}
JSON
  run "$REPO_ROOT/skills/dueling-idea-genies/scripts/validate-output.sh" "$challenge"
  [ "$status" -eq 0 ]

  jq '.handoff.owner="ao plan-pawl decide" | .readiness="PASS"' "$challenge" >"$TMP_ROOT/readiness.json"
  run "$REPO_ROOT/skills/dueling-idea-genies/scripts/validate-output.sh" "$TMP_ROOT/readiness.json"
  [ "$status" -ne 0 ]

  run rg -n 'ao plan-pawl|ApprovalEdge|subsumes.*[Pp]remortem' \
    "$REPO_ROOT/skills/dueling-idea-genies" "$REPO_ROOT/skills/plan/SKILL.md"
  [ "$status" -eq 1 ]
}

@test "Discovery has no ApprovalEdge, Fable, duel state, or phase-local budget owner" {
  [ ! -e "$REPO_ROOT/skills/discovery/references/phase-budgets.md" ]
  [ ! -e "$REPO_ROOT/docs/contracts/codex-fanout-approval-packet.md" ]

  run rg -n 'ApprovalEdge|Fable|duel_verdict_dir|duel_decision|ao plan-pawl|plan-pawl duel|phase-budgets\.md' \
    "$REPO_ROOT/skills/discovery/SKILL.md" "$REPO_ROOT/skills/discovery/references" \
    "$REPO_ROOT/skills-codex/discovery/SKILL.md" "$REPO_ROOT/skills-codex/discovery/references" \
    "$REPO_ROOT/images/gemini/skills/discovery/SKILL.md"
  [ "$status" -eq 1 ]
}

@test "active planning skills expose Premortem as the sole plan readiness verdict" {
  run rg -n 'WARN.*Ready|PASS/WARN/FAIL|Council Verdict: PASS / WARN / FAIL|cross-family rule for one-way doors|plan-pawl' \
    "$REPO_ROOT/skills/premortem/SKILL.md" "$REPO_ROOT/skills/premortem/references/mandatory-checks.md" \
    "$REPO_ROOT/skills/premortem/references/premortem.feature" "$REPO_ROOT/skills/premortem/references/write-premortem-output.md" \
    "$REPO_ROOT/skills-codex/premortem/SKILL.md" "$REPO_ROOT/skills-codex/premortem/references/mandatory-checks.md" \
    "$REPO_ROOT/skills-codex/premortem/references/premortem.feature" "$REPO_ROOT/skills-codex/premortem/references/write-premortem-output.md" \
    "$REPO_ROOT/images/gemini/skills/premortem/SKILL.md" \
    "$REPO_ROOT/skills/discovery/SKILL.md" "$REPO_ROOT/skills/discovery/references" \
    "$REPO_ROOT/skills/goal-design/SKILL.md" "$REPO_ROOT/skills/plan/SKILL.md"
  [ "$status" -eq 1 ]

  run rg -n 'premortem-plan-verdict\.v1|author_id.*judge_id|PASS.*FAIL' \
    "$REPO_ROOT/skills/premortem/SKILL.md" "$REPO_ROOT/skills/premortem/schemas/plan-verdict.schema.json"
  [ "$status" -eq 0 ]
}

@test "Goal Design has only checker-clean draft and terminal lifecycle state" {
  run rg -n '/validate|mark-validated|\bvalidated\b|independent_gate|independent_validator|required_verdict|validation_fails|Last validation verdict|Required independent gate|independent validation' \
    "$REPO_ROOT/docs/contracts/goal-design-artifacts.md" \
    "$REPO_ROOT/schemas/goal-design-intent.v1.schema.json" \
    "$REPO_ROOT/schemas/goal-design-driver.v1.schema.json" \
    "$REPO_ROOT/scripts/goal-design-packet.py" \
    "$REPO_ROOT/docs/templates/goal-design-intent.md" \
    "$REPO_ROOT/docs/templates/goal-design-driver.md" \
    "$REPO_ROOT/tests/fixtures/goal-design"
  [ "$status" -eq 1 ]

  run rg -n 'goal-design.*validate|validate.*goal-design' "$REPO_ROOT/skills/SKILL-TIERS.md"
  [ "$status" -eq 1 ]
}

@test "execution packet exposes one binary Premortem verdict and no planning readiness artifacts" {
  for schema in \
    "$REPO_ROOT/schemas/execution-packet.schema.json" \
    "$REPO_ROOT/cli/internal/domain/packet/schemas/execution-packet.schema.json"; do
    run jq -e '
      (.properties.premortem_verdict.enum == ["PASS", "FAIL"])
      and (.properties | has("pre_mortem_verdict") | not)
      and (.properties | has("default_verdict") | not)
      and (.["$defs"].Artifacts.properties | has("pre_mortem_path") | not)
      and (.["$defs"].Artifacts.properties | has("perspective_plan_paths") | not)
      and (.["$defs"].Artifacts.properties | has("synthesis_packet_path") | not)
      and (.["$defs"].Artifacts.properties | has("fable_approval_path") | not)
      and (.["$defs"].Artifacts.properties | has("approval_edge_path") | not)
    ' "$schema"
    [ "$status" -eq 0 ]
  done

  [ ! -e "$REPO_ROOT/cli/internal/domain/packet/mortem_writer.go" ]
  [ ! -e "$REPO_ROOT/cli/internal/domain/packet/mortem_compatibility_test.go" ]

  run rg -n 'pre_mortem_(verdict|path)|default_verdict|DefaultVerdict|EffectiveVerdict|ExecutionPacketVerdictWarn|perspective_plan_paths|synthesis_packet_path|fable_approval_path|approval_edge_path' \
    "$REPO_ROOT/cli/internal/domain/packet/aggregate.go" \
    "$REPO_ROOT/cli/internal/domain/packet/invariants.go" \
    "$REPO_ROOT/cli/internal/adapters/storage_fs/packet_repo.go" \
    "$REPO_ROOT/cli/internal/rpi/execution_packet.go" \
    "$REPO_ROOT/skills/rpi/scripts/validate-execution-packet.py" \
    "$REPO_ROOT/skills/rpi/scripts/validate.sh" \
    "$REPO_ROOT/cli/cmd/ao/testdata/live-execution-packet.json"
  [ "$status" -eq 1 ]
}

@test "Discovery execution-packet example conforms to the canonical schema" {
  run python3 - "$REPO_ROOT/schemas/execution-packet.schema.json" \
    "$REPO_ROOT/skills/discovery/references/output-templates.md" <<'PY'
import json
import re
import sys
from pathlib import Path
from jsonschema import Draft202012Validator

schema = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
text = Path(sys.argv[2]).read_text(encoding="utf-8")
match = re.search(r"```json\n(.*?)\n```", text, re.DOTALL)
assert match, "missing execution-packet JSON example"
packet = json.loads(match.group(1))
Draft202012Validator(schema).validate(packet)
assert packet["premortem_verdict"] == "PASS"
PY
  [ "$status" -eq 0 ]
}

@test "Behavior First Planning owns deterministic proof but no separate clearing verdict" {
  run rg -n 'independent reviewer|independent closing|clearing verdict|independently cleared|independently reviewed' \
    "$REPO_ROOT/skills/behavior-first-planning/SKILL.md" \
    "$REPO_ROOT/skills/behavior-first-planning/references/behavior-first-planning.feature" \
    "$REPO_ROOT/skills-codex/behavior-first-planning/SKILL.md"
  [ "$status" -eq 1 ]

  run rg -n 'Premortem.*tracker readiness|tracker readiness.*Premortem' \
    "$REPO_ROOT/skills/behavior-first-planning/SKILL.md"
  [ "$status" -eq 0 ]
}

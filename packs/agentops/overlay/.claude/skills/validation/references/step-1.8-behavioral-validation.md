# STEP 1.8: Stage 4 Behavioral Validation

Evaluates holdout scenarios and agent-built specs against the implementation.

## Skip Conditions

- No `.agents/holdout/` directory AND no `.agents/specs/` directory
- `--no-behavioral` flag set

## Sub-steps

a) List active scenarios and agent-built specs:
     `ao scenario list --status active 2>/dev/null`
     `find .agents/specs -name "*.json" -type f 2>/dev/null`

a.5) For each agent-built spec in `.agents/specs/`, treat as a scenario
     with `source="agent"`. Validate against scenario schema (`auto-*` id
     pattern). Add to evaluation set alongside holdout scenarios.

b) If 0 scenarios AND 0 specs → skip with note "No behavioral validation artifacts found"

c) Spawn evaluator council with `AGENTOPS_HOLDOUT_EVALUATOR=1`
   Pass scenarios + implementation diff as judge context

d) Each judge evaluates: "Does the implementation satisfy the scenario's
   `expected_outcome`? Score each `acceptance_vector` dimension 0.0-1.0."

e) Compute `satisfaction_score` per scenario (mean of dimension scores)

f) Aggregate: mean satisfaction across all scenarios

g) Gate:
     mean >= `scenario.satisfaction_threshold` → PASS
     mean >= 0.5 → WARN ("Partial satisfaction — review scenarios")
     mean < 0.5 → FAIL ("Implementation does not satisfy holdout scenarios")

h) Write results to `.agents/rpi/scenario-results.json`

i) Include `satisfaction_score` in `validation_state`

## Verdict

- PASS/WARN → continue to STEP 2
- FAIL → write summary, output `<promise>FAIL</promise>`, stop

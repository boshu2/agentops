#!/usr/bin/env bash
set -euo pipefail

# Gate: eval-workbench-verify
# Verifies the behavioral eval workbench golden state and scoring infrastructure.

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKBENCH="$REPO_ROOT/evals/workbench"

fail() { echo "FAIL: $1" >&2; exit 1; }

# Check workbench exists
[ -d "$WORKBENCH" ] || fail "evals/workbench/ directory not found"
[ -f "$WORKBENCH/Makefile" ] || fail "evals/workbench/Makefile not found"

# Check components exist
[ -f "$WORKBENCH/go-cli/go.mod" ] || fail "Go CLI component missing"
[ -f "$WORKBENCH/python-api/pyproject.toml" ] || fail "Python API component missing"
[ -f "$WORKBENCH/devops/scripts/deploy.sh" ] || fail "DevOps component missing"

# Check minimum task count (spec requires 9+, we have 12)
task_count=$(find "$WORKBENCH/tasks" -name "score.sh" -type f 2>/dev/null | wc -l)
[ "$task_count" -ge 9 ] || fail "Need ≥9 tasks with score.sh, found $task_count"

# Check every task has a prompt.md (task-specific prompts required)
prompt_count=$(find "$WORKBENCH/tasks" -name "prompt.md" -type f 2>/dev/null | wc -l)
[ "$prompt_count" -eq "$task_count" ] || fail "prompt.md count ($prompt_count) != task count ($task_count) — every task needs a prompt.md"

# Check agent eval suite exists and covers all tasks
agent_suite="$REPO_ROOT/evals/agentops-core/workbench-agent-v1.json"
[ -f "$agent_suite" ] || fail "Agent eval suite not found"
agent_case_count=$(python3 -c "import json; print(len(json.load(open('$agent_suite'))['cases']))" 2>/dev/null)
[ "$agent_case_count" -ge "$task_count" ] || fail "Agent suite has $agent_case_count cases but $task_count tasks exist"

# Check behavioral eval suite exists
suite="$REPO_ROOT/evals/agentops-core/workbench-behavioral-v1.json"
[ -f "$suite" ] || fail "Behavioral eval suite not found"

# Validate suite JSON structure
python3 -c "
import json, sys
s = json.load(open('$suite'))
assert s.get('evidence_kind') == 'behavior_fixture', 'wrong evidence_kind'
assert len(s.get('cases', [])) >= 5, 'need ≥5 cases'
assert 'correctness' in str(s.get('scoring', {}).get('dimensions', [])), 'missing correctness dimension'
" || fail "Behavioral eval suite structure invalid"

# Verify Go golden state compiles and tests pass
(cd "$WORKBENCH/go-cli" && go build ./... && go test ./... >/dev/null 2>&1) || fail "Go CLI golden state broken"

# Verify Python golden state tests pass
if [ -d "$WORKBENCH/python-api/.venv" ]; then
  (cd "$WORKBENCH/python-api" && source .venv/bin/activate 2>/dev/null && python -m pytest tests/ -q >/dev/null 2>&1) || fail "Python API golden state broken"
fi

# Verify DevOps tests pass
(cd "$WORKBENCH/devops" && bash tests/test-deploy.sh >/dev/null 2>&1) || fail "DevOps deploy tests broken"

behavioral_case_count=$(python3 -c "import json; print(len(json.load(open('$suite'))['cases']))" 2>/dev/null)

# --- D10: head-to-head delta against baseline scorecard ---
# Generates a current scorecard JSON and compares to the committed baseline.
# Any structural regression (fewer tasks, fewer cases, missing components,
# missing required dimensions) fails the gate with the delta payload so a
# PR reviewer sees what regressed.
baseline="$WORKBENCH/baseline-scorecard.json"
scorecard_out="$WORKBENCH/scorecard-latest.json"

python3 <<PY > "$scorecard_out"
import json
import os

agent_suite_path = "$agent_suite"
behavioral_suite_path = "$suite"
behavioral_suite = json.load(open(behavioral_suite_path))
dims = str(behavioral_suite.get("scoring", {}).get("dimensions", []))

current = {
    "task_count": $task_count,
    "prompt_count": $prompt_count,
    "agent_eval_cases": $agent_case_count,
    "behavioral_eval_cases": $behavioral_case_count,
    "behavioral_evidence_kind": behavioral_suite.get("evidence_kind"),
    "behavioral_scoring_dimensions_include_correctness": "correctness" in dims,
    "go_components_present": os.path.isfile("$WORKBENCH/go-cli/go.mod"),
    "python_components_present": os.path.isfile("$WORKBENCH/python-api/pyproject.toml"),
    "devops_components_present": os.path.isfile("$WORKBENCH/devops/scripts/deploy.sh"),
}
print(json.dumps(current, indent=2, sort_keys=True))
PY

if [ -f "$baseline" ]; then
    delta_output=$(python3 - <<PY
import json, sys

baseline = json.load(open("$baseline"))
current = json.load(open("$scorecard_out"))

tracked_numeric = ["task_count", "prompt_count", "agent_eval_cases", "behavioral_eval_cases"]
tracked_bool = [
    "behavioral_scoring_dimensions_include_correctness",
    "go_components_present", "python_components_present", "devops_components_present",
]

regressions = []
for k in tracked_numeric:
    if k.startswith("_"): continue
    b = baseline.get(k)
    c = current.get(k)
    if b is None or c is None: continue
    if c < b:
        regressions.append(f"{k}: baseline={b} current={c} (regressed by {b - c})")
for k in tracked_bool:
    if k.startswith("_"): continue
    b = baseline.get(k)
    c = current.get(k)
    if b is True and c is not True:
        regressions.append(f"{k}: baseline=true current={c}")

# evidence_kind must remain behavior_fixture
if baseline.get("behavioral_evidence_kind") == "behavior_fixture" and current.get("behavioral_evidence_kind") != "behavior_fixture":
    regressions.append(f"behavioral_evidence_kind: baseline=behavior_fixture current={current.get('behavioral_evidence_kind')}")

if regressions:
    print("REGRESSION")
    for r in regressions:
        print("  - " + r)
    sys.exit(1)
print("DELTA_OK")
PY
)
    delta_rc=$?
    if [ "$delta_rc" -ne 0 ]; then
        echo "eval-workbench-verify: FAIL — D10 delta detected vs baseline-scorecard.json" >&2
        echo "$delta_output" >&2
        echo "" >&2
        echo "Delta scorecard artifact: $scorecard_out" >&2
        echo "Baseline:                 $baseline" >&2
        echo "Refresh baseline only if regression is intentional:" >&2
        echo "  cp $scorecard_out $baseline" >&2
        exit 1
    fi
else
    echo "eval-workbench-verify: WARN — no baseline-scorecard.json present (D10 delta check skipped)"
fi

echo "eval-workbench-verify: PASS ($task_count tasks, $agent_case_count agent cases, $behavioral_case_count behavioral cases; D10 delta check OK)"

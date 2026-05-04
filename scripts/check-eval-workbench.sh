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

echo "eval-workbench-verify: PASS ($task_count tasks, suite has $(python3 -c "import json; print(len(json.load(open('$suite'))['cases']))" 2>/dev/null) cases)"

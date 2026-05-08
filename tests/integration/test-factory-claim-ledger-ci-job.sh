#!/usr/bin/env bash
# test-factory-claim-ledger-ci-job.sh — assert Wave 1C wiring of the
# factory-claim-ledger validator into validate.yml as an advisory job.
#
# Bead: soc-lmww1 (Wave 1C of epic soc-e4ulx).
# Contract: .agents/specs/contract-soc-lmww1.md
#
# Asserts:
#   T1. validate.yml is valid YAML.
#   T2. jobs.factory-claim-ledger-strict exists.
#   T3. Some step in that job has continue-on-error: true.
#   T4. Some step runs `bash scripts/check-factory-claim-ledger.sh`.
#   T5. Some step emits a JSON line containing the required observation keys
#       (run_id, pr_number, verdict, surfaces_touched, timestamp).
#   T6. Some step uses actions/upload-artifact to publish the observation.
#
# Usage: bash tests/integration/test-factory-claim-ledger-ci-job.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORKFLOW="$REPO_ROOT/.github/workflows/validate.yml"
JOB="factory-claim-ledger-strict"

# shellcheck disable=SC1091
source "$REPO_ROOT/tests/lib/colors.sh"

PASS=0
FAIL=0

ok() {
    pass "$1"
    PASS=$((PASS + 1))
}

ko() {
    fail "$1"
    FAIL=$((FAIL + 1))
}

if ! python3 -c 'import yaml' >/dev/null 2>&1; then
    red "python3 PyYAML is required for this test (pip install pyyaml)" >&2
    exit 2
fi

if [[ ! -f "$WORKFLOW" ]]; then
    ko "validate.yml exists at $WORKFLOW"
    exit 1
fi

mapfile -t LINES < <(python3 - "$WORKFLOW" "$JOB" <<'PY'
import re
import sys

import yaml

workflow_path, job_name = sys.argv[1], sys.argv[2]
REQUIRED_OBSERVATION_KEYS = ["run_id", "pr_number", "verdict", "surfaces_touched", "timestamp"]

try:
    with open(workflow_path, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh)
    print("T1\t1\tvalidate.yml parses as YAML")
except Exception as exc:  # noqa: BLE001 - test surface
    print(f"T1\t0\tvalidate.yml parses as YAML ({exc!s})")
    sys.exit(0)

jobs = (data or {}).get("jobs") or {}
job = jobs.get(job_name)
print(f"T2\t{1 if isinstance(job, dict) else 0}\tjobs.{job_name} is defined")

steps = job.get("steps") if isinstance(job, dict) else []
if not isinstance(steps, list):
    steps = []

run_steps = [s for s in steps if isinstance(s, dict) and isinstance(s.get("run"), str)]
uses_steps = [s for s in steps if isinstance(s, dict) and isinstance(s.get("uses"), str)]

t3 = any(
    s.get("continue-on-error") in (True, "true")
    for s in steps
    if isinstance(s, dict)
)
print(f"T3\t{1 if t3 else 0}\ta step has continue-on-error: true")

validator_re = re.compile(r"\bbash\s+scripts/check-factory-claim-ledger\.sh\b")
t4 = any(validator_re.search(s["run"]) for s in run_steps)
print(f"T4\t{1 if t4 else 0}\ta step runs bash scripts/check-factory-claim-ledger.sh")

t5 = False
for s in run_steps:
    body = s["run"]
    if all(
        re.search(rf'["\']{re.escape(k)}["\']', body) for k in REQUIRED_OBSERVATION_KEYS
    ):
        t5 = True
        break
print(f"T5\t{1 if t5 else 0}\ta step constructs the observation JSON")

t6 = any(s["uses"].startswith("actions/upload-artifact") for s in uses_steps)
print(f"T6\t{1 if t6 else 0}\ta step uses actions/upload-artifact")
PY
)

for line in "${LINES[@]}"; do
    [[ -z "$line" ]] && continue
    IFS=$'\t' read -r tag flag msg <<<"$line"
    if [[ "$flag" == "1" ]]; then
        ok "$tag $msg"
    else
        ko "$tag $msg"
    fi
done

echo ""
if [[ "$FAIL" -gt 0 ]]; then
    red "FAILED — $FAIL/$((PASS + FAIL)) checks failed" >&2
    exit 1
fi

green "PASSED — $PASS/$PASS checks passed"
exit 0

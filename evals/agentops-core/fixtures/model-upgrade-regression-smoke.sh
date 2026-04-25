#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

python3 - "$TMP_DIR" <<'PY'
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
(root / "fixture.txt").write_text("baseline ok\n")

def write_suite(path: Path, second_value: str) -> None:
    suite = {
        "schema_version": 1,
        "id": "tmp.model-upgrade",
        "name": "Temporary model upgrade",
        "domain": "mixed",
        "visibility": "public_canary",
        "tier": "deterministic",
        "allowed_runtimes": ["static"],
        "scoring": {
            "aggregate_threshold": 0,
            "dimensions": [
                {"name": "correctness", "weight": 1, "threshold": 0}
            ],
        },
        "baseline_policy": {"mode": "none"},
        "cases": [
            {
                "id": "stable",
                "title": "stable",
                "kind": "artifact_check",
                "objective": "stable",
                "expectations": [
                    {
                        "type": "artifact_contains",
                        "target": "fixture.txt",
                        "value": "baseline ok",
                    }
                ],
                "dimensions": ["correctness"],
            },
            {
                "id": "candidate-sensitive",
                "title": "candidate sensitive",
                "kind": "artifact_check",
                "objective": "sensitive",
                "expectations": [
                    {
                        "type": "artifact_contains",
                        "target": "fixture.txt",
                        "value": second_value,
                    }
                ],
                "dimensions": ["correctness"],
            },
        ],
    }
    path.write_text(json.dumps(suite) + "\n")

write_suite(root / "baseline-suite.json", "baseline ok")
write_suite(root / "candidate-suite.json", "missing value")
PY

(
    cd "$ROOT/cli"
    env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval run \
        "$TMP_DIR/baseline-suite.json" \
        --out "$TMP_DIR/baseline-run.json" \
        --json >/dev/null
    env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval run \
        "$TMP_DIR/candidate-suite.json" \
        --out "$TMP_DIR/candidate-run.json" \
        --json >/dev/null
    env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval compare \
        "$TMP_DIR/candidate-run.json" \
        "$TMP_DIR/baseline-run.json" \
        --json >"$TMP_DIR/compare.json"
)

jq -e '
    .status == "pass"
    and .verdict == "regression"
    and .baseline_comparison.aggregate_delta < 0
    and (.baseline_comparison.regressions | length) > 0
' "$TMP_DIR/compare.json" >/dev/null

echo "model-upgrade regression detection passed"

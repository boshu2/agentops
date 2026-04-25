#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

(
    cd "$ROOT/cli"
    env -u AGENTOPS_RPI_RUNTIME go test -timeout=120s ./cmd/ao -run 'TestRetrievalBench_BuildReportTrainHoldoutAndSections|TestRetrievalBench_RealCorpusManifestHasTrainHoldoutCoverage|TestRunFlywheelGateCommand_PassesWithHealthyWorkspace|TestEvaluateFlywheelGate|TestEvaluateFlywheelGate_FailsOnThresholds'

    env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao retrieval-bench \
        --live \
        --corpus cmd/ao/testdata/retrieval-bench-live \
        --json >"$TMP_DIR/live-retrieval.json"
)

jq -e '
    .mode == "live-corpus"
    and .coverage == 1
    and .queries >= 10
    and .queries_with_hits == .queries
    and (
        [.results[] | select(.query == "security") | .top_ids[0]]
        | index("security-toolchain-gate.md")
    )
    and (
        [.results[] | select(.query == "architecture") | .top_ids[0]]
        | index("architecture-surface-contract.md")
    )
' "$TMP_DIR/live-retrieval.json" >/dev/null

echo "retrieval quality smoke passed"

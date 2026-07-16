#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

(
    cd "$ROOT/cli"
    env -u AGENTOPS_RPI_RUNTIME go test -count=1 -timeout=120s ./cmd/ao -run '^(TestRetrievalBench_BuildReportTrainHoldoutAndSections|TestRetrievalBench_RealCorpusManifestHasTrainHoldoutCoverage|TestRetrievalBench_PrecisionAtK|TestBuildLiveReport_FixtureCoverage|TestBuildSearchEvalComparisonReport_JSONIncludesBackendMetrics|TestSearchEvalBackendAgenticRGPathTokenSearch|TestSearchEvalBackendWikiLinkExpand|TestSearchEvalBackendRerankLlamaCPPSkipsWhenEndpointUnset|TestRunFlywheelGateCommand_PassesWithHealthyWorkspace|TestEvaluateFlywheelGate|TestEvaluateFlywheelGate_FailsOnThresholds)$'
)

echo "retrieval quality smoke passed"

#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/test-agentops-contract-canaries.sh"
    TMP="$BATS_TEST_TMPDIR/canaries"
    mkdir -p "$TMP"
}

@test "contract canary runner executes only suites from the official list" {
    suite="$TMP/suite.json"
    list="$TMP/list.txt"
    ao="$TMP/ao"
    out_root="$TMP/runs"

    cat > "$suite" <<'JSON'
{
  "schema_version": 1,
  "id": "contract.fixture",
  "name": "Contract fixture",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "case",
      "title": "case",
      "kind": "artifact_check",
      "objective": "case",
      "expectations": [
        {"type": "file_exists", "target": "fixture.txt"}
      ],
      "critical": true
    }
  ]
}
JSON
    printf '# comment\n%s\n' "$suite" > "$list"

    cat > "$ao" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1 $2" != "eval run" ]]; then
  exit 2
fi
suite="$3"
shift 3
out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --run-id) shift 2 ;;
    --json) shift ;;
    *) shift ;;
  esac
done
id="$(python3 - "$suite" <<'PY'
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["id"])
PY
)"
mkdir -p "$(dirname "$out")"
cat > "$out" <<JSON
{"schema_version":1,"run_id":"$id-run","suite":{"id":"$id","path":"$suite","visibility":"public_canary","tier":"deterministic"},"started_at":"2026-05-03T00:00:00Z","status":"pass","verdict":"pass","git":{"candidate_ref":"test","candidate_sha":"abcdef0","dirty":false},"runtime":{"name":"static","live":false},"environment":{"scrubbed_env_prefixes":[],"network_access":"disabled"},"case_results":[{"id":"case","status":"pass","score":1,"dimension_scores":{"correctness":1}}],"aggregate_score":1,"dimension_scores":{"correctness":1}}
JSON
printf '{"ok":true}\n'
SH
    chmod +x "$ao"

    run "$SCRIPT" --suite-list "$list" --run-root "$out_root" --ao-bin "$ao"

    [ "$status" -eq 0 ]
    [[ "$output" == *"AgentOps contract canary summary: failures=0"* ]]
    [ -f "$out_root"/*/contract.fixture.run.json ]
}

@test "contract canary runner fails when a selected suite fails" {
    suite="$TMP/failing-suite.json"
    list="$TMP/failing-list.txt"
    ao="$TMP/failing-ao"

    cat > "$suite" <<'JSON'
{"id":"contract.failing","tags":[]}
JSON
    printf '%s\n' "$suite" > "$list"

    cat > "$ao" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
suite="$3"
shift 3
out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --run-id) shift 2 ;;
    --json) shift ;;
    *) shift ;;
  esac
done
mkdir -p "$(dirname "$out")"
cat > "$out" <<JSON
{"schema_version":1,"run_id":"failing-run","suite":{"id":"contract.failing","path":"$suite","visibility":"public_canary","tier":"deterministic"},"started_at":"2026-05-03T00:00:00Z","status":"fail","verdict":"fail","git":{"candidate_ref":"test","candidate_sha":"abcdef0","dirty":false},"runtime":{"name":"static","live":false},"environment":{"scrubbed_env_prefixes":[],"network_access":"disabled"},"case_results":[{"id":"case","status":"fail","score":0,"failure_message":"expected failure","dimension_scores":{"correctness":0}}],"aggregate_score":0,"dimension_scores":{"correctness":0}}
JSON
printf '{"ok":false}\n'
SH
    chmod +x "$ao"

    run "$SCRIPT" --suite-list "$list" --run-root "$TMP/failing-runs" --ao-bin "$ao"

    [ "$status" -eq 1 ]
    [[ "$output" == *"contract.failing run did not pass"* ]]
    [[ "$output" == *"expected failure"* ]]
}

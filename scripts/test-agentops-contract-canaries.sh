#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SUITE_LIST="${AGENTOPS_CONTRACT_CANARY_SUITE_LIST:-tests/canaries/agentops-core-official.txt}"
RUN_ROOT="${AGENTOPS_CONTRACT_CANARY_RUN_ROOT:-.agents/tests/contract-canaries}"
AO_BIN="${AO_BIN:-}"

usage() {
    cat <<'USAGE'
Usage: scripts/test-agentops-contract-canaries.sh [options]

Run the official blocking AgentOps deterministic contract canaries. These are
normal test/CI canaries, not model evals; the eval runner is only the execution
and artifact substrate.

Options:
  --suite-list PATH   Newline-delimited suite manifest list
  --run-root PATH     Artifact root (default: .agents/tests/contract-canaries)
  --ao-bin PATH       Built ao binary to use instead of building a temp binary
  -h, --help          Show this help
USAGE
}

die() {
    echo "FAIL contract-canaries: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --suite-list)
            [[ -n "${2:-}" ]] || die "--suite-list requires a path"
            SUITE_LIST="$2"
            shift 2
            ;;
        --run-root)
            [[ -n "${2:-}" ]] || die "--run-root requires a path"
            RUN_ROOT="$2"
            shift 2
            ;;
        --ao-bin)
            [[ -n "${2:-}" ]] || die "--ao-bin requires a path"
            AO_BIN="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "unknown option: $1"
            ;;
    esac
done

[[ -f "$SUITE_LIST" ]] || die "suite list not found: $SUITE_LIST"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/agentops-contract-canaries.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

if [[ -z "$AO_BIN" ]]; then
    AO_BIN="$tmp_dir/ao"
    (cd cli && env -u AGENTOPS_RPI_RUNTIME go build -o "$AO_BIN" ./cmd/ao)
fi
[[ -x "$AO_BIN" ]] || die "ao binary is not executable: $AO_BIN"

trim() {
    local value="$1"
    value="${value%%#*}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s\n' "$value"
}

suite_id() {
    python3 - "$1" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    print(json.load(fh)["id"])
PY
}

json_get() {
    local path="$1"
    local key="$2"
    python3 - "$path" "$key" <<'PY'
import json
import sys
path, key = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as fh:
    value = json.load(fh)
for part in key.split("."):
    value = value[part]
if isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

context_mode_for_suite() {
    python3 - "$1" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
tags = set(data.get("tags") or [])
suite_id = data.get("id", "")
if suite_id == "context-packet-ab-wave0" or {"context-packet", "ab"}.issubset(tags):
    print("ab")
else:
    print("none")
PY
}

number_gt_zero() {
    python3 - "$1" <<'PY'
import sys
try:
    value = float(sys.argv[1])
except ValueError:
    raise SystemExit(1)
raise SystemExit(0 if value > 0 else 1)
PY
}

print_failed_cases() {
    local run_path="$1"
    python3 - "$run_path" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
for case in data.get("case_results") or []:
    if case.get("status") != "pass":
        print(f"  - {case.get('id')}: {case.get('failure_message', '')}")
PY
}

suites=()
while IFS= read -r raw_line || [[ -n "$raw_line" ]]; do
    suite="$(trim "$raw_line")"
    [[ -z "$suite" ]] && continue
    [[ -f "$suite" ]] || die "suite not found from $SUITE_LIST: $suite"
    suites+=("$suite")
done < "$SUITE_LIST"

[[ "${#suites[@]}" -gt 0 ]] || die "suite list is empty: $SUITE_LIST"

run_id="$(date -u +%Y%m%dT%H%M%SZ)"
run_dir="$RUN_ROOT/$run_id"
mkdir -p "$run_dir"

failures=0

record_failure() {
    echo "FAIL contract-canaries: $*" >&2
    failures=$((failures + 1))
}

echo "AgentOps contract canary run: $run_id"
echo "Suites: ${#suites[@]}"
echo "Suite list: $SUITE_LIST"
echo "Artifacts: $run_dir"

for suite in "${suites[@]}"; do
    id="$(suite_id "$suite")"
    run_path="$run_dir/${id}.run.json"
    stdout_path="$run_dir/${id}.stdout.json"
    context_mode="$(context_mode_for_suite "$suite")"

    echo ""
    echo "== $id =="
    if [[ "$context_mode" == "ab" ]]; then
        scorecard_path="$run_dir/${id}.context-scorecard.json"
        if ! "$AO_BIN" eval run "$suite" --context-mode=ab --run-id "${id}-${run_id}" --out "$run_path" --delta-out "$scorecard_path" --json >"$stdout_path"; then
            record_failure "$id context A/B command failed"
            continue
        fi
        off_status="$(json_get "$scorecard_path" "context_off.status")"
        on_status="$(json_get "$scorecard_path" "context_on.status")"
        delta="$(json_get "$scorecard_path" "aggregate_delta")"
        echo "context: off=$off_status on=$on_status aggregate_delta=$delta"
        if [[ "$on_status" != "pass" ]] || ! number_gt_zero "$delta"; then
            record_failure "$id context A/B did not produce a positive passing context_on delta"
        fi
        continue
    fi

    if ! "$AO_BIN" eval run "$suite" --run-id "${id}-${run_id}" --out "$run_path" --json >"$stdout_path"; then
        record_failure "$id run command failed"
        continue
    fi

    status="$(json_get "$run_path" "status")"
    verdict="$(json_get "$run_path" "verdict")"
    aggregate="$(json_get "$run_path" "aggregate_score")"
    echo "run: status=$status verdict=$verdict aggregate=$aggregate"
    if [[ "$status" != "pass" || "$verdict" == "fail" || "$verdict" == "regression" ]]; then
        record_failure "$id run did not pass"
        print_failed_cases "$run_path"
    fi
done

echo ""
echo "AgentOps contract canary summary: failures=$failures artifacts=$run_dir"
if [[ "$failures" -gt 0 ]]; then
    exit 1
fi
exit 0

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAST=false
PROMOTE=false
PROMOTED_BY="${USER:-agentops}"
RATIONALE=""
BASELINE_DIR=".agents/evals/baselines"
RUN_ROOT=".agents/evals/runs"
ADVISORY=false
SUITES=()

usage() {
    cat <<'USAGE'
Usage: scripts/eval-agentops.sh [options]

Run AgentOps public evaluation canaries and compare them with promoted baselines
when baselines exist.

Options:
  --fast                    Run the repo public canary set under evals/agentops-core
  --suite PATH              Add one suite path to run; repeatable
  --baseline-dir PATH       Baseline directory (default: .agents/evals/baselines)
  --run-root PATH           Run artifact root (default: .agents/evals/runs)
  --promote-baseline        Promote successful run records as baselines
  --promoted-by NAME        Identity recorded during baseline promotion
  --rationale TEXT          Required rationale when promoting baselines
  --advisory                Report failures but exit 0
  -h, --help                Show this help
USAGE
}

die() {
    echo "FAIL eval-agentops: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --fast)
            FAST=true
            shift
            ;;
        --suite)
            [[ -n "${2:-}" ]] || die "--suite requires a path"
            SUITES+=("$2")
            shift 2
            ;;
        --baseline-dir)
            [[ -n "${2:-}" ]] || die "--baseline-dir requires a path"
            BASELINE_DIR="$2"
            shift 2
            ;;
        --run-root)
            [[ -n "${2:-}" ]] || die "--run-root requires a path"
            RUN_ROOT="$2"
            shift 2
            ;;
        --promote-baseline)
            PROMOTE=true
            shift
            ;;
        --promoted-by)
            [[ -n "${2:-}" ]] || die "--promoted-by requires a value"
            PROMOTED_BY="$2"
            shift 2
            ;;
        --rationale)
            [[ -n "${2:-}" ]] || die "--rationale requires text"
            RATIONALE="$2"
            shift 2
            ;;
        --advisory)
            ADVISORY=true
            shift
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

if [[ "$PROMOTE" == "true" && -z "$RATIONALE" ]]; then
    die "--promote-baseline requires --rationale"
fi

if [[ "${#SUITES[@]}" -eq 0 ]]; then
    if [[ "$FAST" == "true" ]]; then
        while IFS= read -r path; do
            SUITES+=("$path")
        done < <(find evals/agentops-core -maxdepth 1 -type f -name '*.json' | sort)
    else
        while IFS= read -r path; do
            SUITES+=("$path")
        done < <(find evals -type f -name '*.json' | sort)
    fi
fi

if [[ "${#SUITES[@]}" -eq 0 ]]; then
    die "no eval suites found"
fi

json_get() {
    local path="$1"
    local key="$2"
    python3 - "$path" "$key" <<'PY'
import json
import sys

path, key = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as fh:
    data = json.load(fh)
value = data
for part in key.split("."):
    value = value[part]
if isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

json_get_default() {
    local path="$1"
    local key="$2"
    local default="$3"
    python3 - "$path" "$key" "$default" <<'PY'
import json
import sys

path, key, default = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, encoding="utf-8") as fh:
    data = json.load(fh)
value = data
for part in key.split("."):
    if not isinstance(value, dict) or part not in value:
        print(default)
        raise SystemExit(0)
    value = value[part]
if isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

scorecard_kind_for_suite() {
    local suite_id="$1"
    case "$suite_id" in
        *rpi-scorecard*) printf 'rpi\n' ;;
        *skill-change-scorecard*) printf 'skill-change\n' ;;
        *) printf '\n' ;;
    esac
}

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/agentops-eval.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

AO_BIN="${AO_BIN:-}"
if [[ -z "$AO_BIN" ]]; then
    AO_BIN="$tmp_dir/ao"
    (cd cli && env -u AGENTOPS_RPI_RUNTIME go build -o "$AO_BIN" ./cmd/ao)
fi

run_id="$(date -u +%Y%m%dT%H%M%SZ)"
run_dir="$RUN_ROOT/$run_id"
mkdir -p "$run_dir" "$BASELINE_DIR"

failures=0
warnings=0

record_failure() {
    echo "FAIL eval-agentops: $*" >&2
    failures=$((failures + 1))
}

record_warning() {
    echo "WARN eval-agentops: $*" >&2
    warnings=$((warnings + 1))
}

echo "AgentOps eval run: $run_id"
echo "Suites: ${#SUITES[@]}"
echo "Artifacts: $run_dir"

for suite in "${SUITES[@]}"; do
    if [[ ! -f "$suite" ]]; then
        record_failure "suite not found: $suite"
        continue
    fi

    suite_id="$(json_get "$suite" "id")"
    run_path="$run_dir/${suite_id}.run.json"
    run_stdout="$run_dir/${suite_id}.run.stdout.json"
    baseline_path="$BASELINE_DIR/${suite_id}.baseline.json"
    compare_path="$run_dir/${suite_id}.compare.json"

    echo ""
    echo "== $suite_id =="
    if ! "$AO_BIN" eval run "$suite" --run-id "${suite_id}-${run_id}" --out "$run_path" --json >"$run_stdout"; then
        record_failure "$suite_id run command failed"
        continue
    fi

    status="$(json_get "$run_path" "status")"
    verdict="$(json_get "$run_path" "verdict")"
    aggregate="$(json_get "$run_path" "aggregate_score")"
    echo "run: status=$status verdict=$verdict aggregate=$aggregate"
    if [[ "$status" != "pass" || "$verdict" == "fail" || "$verdict" == "regression" ]]; then
        record_failure "$suite_id run did not pass"
    fi

    kind="$(scorecard_kind_for_suite "$suite_id")"
    if [[ -n "$kind" ]]; then
        scorecard_path="$run_dir/${suite_id}.scorecard.json"
        scorecard_stdout="$run_dir/${suite_id}.scorecard.stdout.json"
        scorecard_args=(eval scorecard "$run_path" --kind "$kind" --out "$scorecard_path" --json)
        if [[ -f "$baseline_path" ]]; then
            scorecard_args=(eval scorecard "$run_path" "$baseline_path" --kind "$kind" --out "$scorecard_path" --json)
        fi
        if ! "$AO_BIN" "${scorecard_args[@]}" >"$scorecard_stdout"; then
            record_failure "$suite_id scorecard command failed"
        else
            scorecard_verdict="$(json_get "$scorecard_path" "verdict")"
            echo "scorecard: kind=$kind verdict=$scorecard_verdict"
            if [[ "$scorecard_verdict" == "fail" || "$scorecard_verdict" == "regression" ]]; then
                record_failure "$suite_id scorecard verdict is $scorecard_verdict"
            fi
        fi
    fi

    if [[ -f "$baseline_path" ]]; then
        compare_stdout="$run_dir/${suite_id}.compare.stdout.json"
        if ! "$AO_BIN" eval compare "$run_path" "$baseline_path" --out "$compare_path" --json >"$compare_stdout"; then
            record_failure "$suite_id baseline compare command failed"
        else
            compare_verdict="$(json_get "$compare_path" "verdict")"
            delta="$(json_get_default "$compare_path" "baseline_comparison.aggregate_delta" "0")"
            echo "baseline: verdict=$compare_verdict aggregate_delta=$delta"
            if [[ "$compare_verdict" == "fail" || "$compare_verdict" == "regression" ]]; then
                record_failure "$suite_id regressed against baseline"
            fi
        fi
    else
        record_warning "$suite_id has no promoted baseline at $baseline_path"
    fi

    if [[ "$PROMOTE" == "true" ]]; then
        promote_stdout="$run_dir/${suite_id}.baseline.stdout.json"
        if ! "$AO_BIN" eval baseline "$run_path" --out "$baseline_path" --promoted-by "$PROMOTED_BY" --rationale "$RATIONALE" --json >"$promote_stdout"; then
            record_failure "$suite_id baseline promotion failed"
        else
            echo "promoted baseline: $baseline_path"
        fi
    fi
done

echo ""
echo "AgentOps eval summary: failures=$failures warnings=$warnings artifacts=$run_dir"
if [[ "$failures" -gt 0 && "$ADVISORY" != "true" ]]; then
    exit 1
fi
exit 0

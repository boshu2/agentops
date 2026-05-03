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
when baselines exist. Context A/B canaries run through --context-mode=ab and
produce a context delta scorecard instead of a single run record.

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

context_mode_for_suite() {
    local path="$1"
    python3 - "$path" <<'PY'
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

coverage_missing_summary() {
    local path="$1"
    python3 - "$path" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)

parts = []
for label, key in (
    ("domains", "missing_required_domains"),
    ("dimensions", "missing_required_dimensions"),
    ("runtimes", "missing_required_runtimes"),
):
    values = data.get(key) or []
    if values:
        parts.append(f"{label}={','.join(values)}")

print("; ".join(parts))
PY
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
promote_queue=()

record_failure() {
    echo "FAIL eval-agentops: $*" >&2
    failures=$((failures + 1))
}

record_warning() {
    echo "WARN eval-agentops: $*" >&2
    warnings=$((warnings + 1))
}

# Phase-2 (deferred) baseline promotion: a regressed suite must not write
# its run record over the existing baseline. Per-suite the loop only queues;
# the actual write happens after all suites finish AND failures == 0.
queue_promotion() {
    local suite_id="$1" run_path="$2" baseline_path="$3"
    promote_queue+=("$suite_id|$run_path|$baseline_path")
}

# Working-tree allowlist: .agents/ is session state and is allowed dirty.
# Anything else dirty refuses promotion unless EVAL_AGENTOPS_ALLOW_DIRTY=1.
working_tree_dirty_outside_allowlist() {
    if ! command -v git >/dev/null 2>&1; then
        return 1
    fi
    git -C "$ROOT" status --porcelain 2>/dev/null \
        | awk '{ p=$0; sub(/^.. /,"",p); print p }' \
        | grep -vE '^(\.agents/|\.beads/)' \
        | grep -q .
}

run_promote_queue() {
    [[ "$PROMOTE" == "true" ]] || return 0
    [[ "${#promote_queue[@]}" -gt 0 ]] || return 0

    if [[ "$failures" -gt 0 ]]; then
        record_failure "promotion ABORTED: $failures suite failure(s); no baselines written"
        return 0
    fi
    if [[ "${EVAL_AGENTOPS_ALLOW_DIRTY:-0}" != "1" ]] && working_tree_dirty_outside_allowlist; then
        record_failure "promotion ABORTED: working tree dirty outside .agents/ and .beads/ (set EVAL_AGENTOPS_ALLOW_DIRTY=1 to override)"
        return 0
    fi

    local entry suite_id run_path baseline_path promote_stdout
    for entry in "${promote_queue[@]}"; do
        IFS='|' read -r suite_id run_path baseline_path <<<"$entry"
        promote_stdout="$run_dir/${suite_id}.baseline.stdout.json"
        if ! "$AO_BIN" eval baseline "$run_path" --out "$baseline_path" --promoted-by "$PROMOTED_BY" --rationale "$RATIONALE" --json >"$promote_stdout"; then
            record_failure "$suite_id baseline promotion failed"
        else
            echo "promoted baseline: $baseline_path"
        fi
    done
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
    baseline_mode="$(json_get_default "$suite" "baseline_policy.mode" "none")"
    baseline_gate="$(json_get_default "$suite" "baseline_policy.blocking_gate" "none")"
    context_mode="$(context_mode_for_suite "$suite")"

    echo ""
    echo "== $suite_id =="
    if [[ "$context_mode" == "ab" ]]; then
        scorecard_path="$run_dir/${suite_id}.context-scorecard.json"
        context_stdout="$run_dir/${suite_id}.context.stdout.json"
        if ! "$AO_BIN" eval run "$suite" --context-mode=ab --run-id "${suite_id}-${run_id}" --out "$run_path" --delta-out "$scorecard_path" --json >"$context_stdout"; then
            record_failure "$suite_id context A/B command failed"
            continue
        fi

        off_status="$(json_get "$scorecard_path" "context_off.status")"
        on_status="$(json_get "$scorecard_path" "context_on.status")"
        delta="$(json_get "$scorecard_path" "aggregate_delta")"
        echo "context: off=$off_status on=$on_status aggregate_delta=$delta"
        if [[ "$on_status" != "pass" ]] || ! number_gt_zero "$delta"; then
            record_failure "$suite_id context A/B did not show a positive passing context_on delta"
        fi
        continue
    fi

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
        case "$baseline_mode" in
            compare)
                if [[ "$baseline_gate" == "pre-push" || "$baseline_gate" == "ci" || "$baseline_gate" == "release" || "$baseline_gate" == "model-upgrade" ]]; then
                    record_failure "$suite_id requires a promoted baseline at $baseline_path"
                else
                    record_warning "$suite_id has no promoted baseline at $baseline_path"
                fi
                ;;
            promotable)
                record_warning "$suite_id is promotable but has no promoted baseline at $baseline_path"
                ;;
            none|"")
                ;;
            *)
                record_warning "$suite_id has unknown baseline_policy.mode=$baseline_mode"
                ;;
        esac
    fi

    if [[ "$PROMOTE" == "true" ]]; then
        queue_promotion "$suite_id" "$run_path" "$baseline_path"
    fi
done

run_promote_queue

if [[ "$FAST" == "true" ]]; then
    coverage_path="$run_dir/coverage.json"
    coverage_stdout="$run_dir/coverage.stdout.json"
    baseline_audit_path="$run_dir/baseline-audit.json"
    baseline_audit_stdout="$run_dir/baseline-audit.stdout.json"
    echo ""
    echo "== eval coverage =="
    if ! "$AO_BIN" eval coverage --root evals/agentops-core --json >"$coverage_path"; then
        record_failure "coverage command failed"
    else
        cp "$coverage_path" "$coverage_stdout"
        coverage_missing="$(coverage_missing_summary "$coverage_path")"
        if [[ -n "$coverage_missing" ]]; then
            record_failure "coverage gaps: $coverage_missing"
        else
            echo "coverage: required domains, dimensions, and runtimes covered"
        fi
    fi
    echo ""
    echo "== eval baseline audit =="
    if ! "$AO_BIN" eval baseline-audit --root evals/agentops-core --baseline-dir "$BASELINE_DIR" --json >"$baseline_audit_path"; then
        record_failure "baseline audit command failed"
    else
        cp "$baseline_audit_path" "$baseline_audit_stdout"
        policy_mismatches="$(json_get "$baseline_audit_path" "policy_mismatch_count")"
        stale_hashes="$(python3 - "$baseline_audit_path" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
print(len(data.get("stale_suite_hashes") or []))
PY
)"
        if [[ "$policy_mismatches" != "0" ]]; then
            record_failure "baseline policy mismatches: $policy_mismatches"
        else
            echo "baseline audit: policy mismatches=0 stale_suite_hashes=$stale_hashes"
        fi
    fi
fi

echo ""
echo "AgentOps eval summary: failures=$failures warnings=$warnings artifacts=$run_dir"
if [[ "$failures" -gt 0 && "$ADVISORY" != "true" ]]; then
    exit 1
fi
exit 0

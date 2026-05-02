#!/usr/bin/env bash
# retrieval-quality-smoke.sh - offline comparison-mode smoke for search eval backends

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MANIFEST="${AGENTOPS_RETRIEVAL_SMOKE_MANIFEST:-cli/cmd/ao/testdata/retrieval-bench/search-eval-manifest.json}"
SEARCH_ROOT="${AGENTOPS_RETRIEVAL_SMOKE_SEARCH_ROOT:-$REPO_ROOT}"
BACKENDS="${AGENTOPS_RETRIEVAL_SMOKE_BACKENDS:-local-lexical,ao-auto,agentic-rg,wiki-link-expand,rerank-llamacpp}"
MIN_BACKENDS="${AGENTOPS_RETRIEVAL_SMOKE_MIN_BACKENDS:-5}"
AO_BIN="${AGENTOPS_RETRIEVAL_SMOKE_AO:-}"

if ! command -v jq >/dev/null 2>&1; then
    echo "FAIL retrieval quality smoke: jq is required" >&2
    exit 1
fi

manifest_path="$MANIFEST"
if [[ "$manifest_path" != /* ]]; then
    manifest_path="$REPO_ROOT/$manifest_path"
fi
if [[ ! -f "$manifest_path" ]]; then
    echo "FAIL retrieval quality smoke: read search eval manifest $manifest_path: no such file or directory" >&2
    exit 1
fi

search_root="$SEARCH_ROOT"
if [[ "$search_root" != /* ]]; then
    search_root="$REPO_ROOT/$search_root"
fi
if [[ ! -d "$search_root" ]]; then
    echo "FAIL retrieval quality smoke: search root $search_root is not a directory" >&2
    exit 1
fi

if [[ -n "$AO_BIN" && ! -x "$AO_BIN" ]]; then
    echo "FAIL retrieval quality smoke: AGENTOPS_RETRIEVAL_SMOKE_AO is not executable: $AO_BIN" >&2
    exit 1
fi

report_file="$(mktemp "${TMPDIR:-/tmp}/ao-retrieval-smoke.XXXXXX")"
trap 'rm -f "$report_file"' EXIT

if [[ -n "$AO_BIN" ]]; then
    if ! env -u AGENTOPS_RPI_RUNTIME -u AGENTOPS_RETRIEVAL_RERANK_ENDPOINT \
        "$AO_BIN" retrieval-bench \
            --search-eval "$manifest_path" \
            --search-root "$search_root" \
            --search-compare-backends "$BACKENDS" \
            --json >"$report_file"; then
        echo "FAIL retrieval quality smoke: comparison eval command failed" >&2
        exit 1
    fi
else
    if ! (
        cd "$REPO_ROOT/cli"
        env -u AGENTOPS_RPI_RUNTIME -u AGENTOPS_RETRIEVAL_RERANK_ENDPOINT \
            go run ./cmd/ao retrieval-bench \
                --search-eval "$manifest_path" \
                --search-root "$search_root" \
                --search-compare-backends "$BACKENDS" \
                --json
    ) >"$report_file"; then
        echo "FAIL retrieval quality smoke: comparison eval command failed" >&2
        exit 1
    fi
fi

if ! jq -e --argjson min "$MIN_BACKENDS" '
    (.backends | type == "array" and length >= $min)
    and (.id | type == "string")
    and (.manifest_path | type == "string" and length > 0)
    and (.search_root | type == "string" and length > 0)
    and (.queries | type == "number" and . > 0)
    and (.k | type == "number" and . > 0)
    and all(.backends[]; (
        (.backend | type == "string" and length > 0)
        and (.manifest_path | type == "string" and length > 0)
        and (.search_root | type == "string" and length > 0)
        and (.queries | type == "number" and . > 0)
        and (.hits | type == "number")
        and (.missing_ground_truth == 0)
        and (.any_relevant_at_k | type == "number")
        and (.avg_precision_at_k | type == "number")
        and (.mean_reciprocal_rank | type == "number")
        and (.results | type == "array")
    ))
' "$report_file" >/dev/null; then
    echo "FAIL retrieval quality smoke: comparison report missing required fields" >&2
    jq '.' "$report_file" >&2
    exit 1
fi

IFS=',' read -r -a expected_backends <<<"$BACKENDS"
for backend in "${expected_backends[@]}"; do
    backend="${backend//[[:space:]]/}"
    if [[ -z "$backend" ]]; then
        continue
    fi
    if ! jq -e --arg backend "$backend" '.backends[] | select(.backend == $backend)' "$report_file" >/dev/null; then
        echo "FAIL retrieval quality smoke: backend missing from comparison report: $backend" >&2
        jq -r '.backends[]?.backend' "$report_file" >&2
        exit 1
    fi
done

summary="$(jq -r '.backends | map(.backend + ":any=" + (.any_relevant_at_k | tostring) + ",mrr=" + (.mean_reciprocal_rank | tostring) + ",missing=" + (.missing_ground_truth | tostring)) | join(" ")' "$report_file")"
echo "PASS retrieval quality smoke: backends=$(jq -r '.backends | length' "$report_file") $summary"

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

REPO="${GITHUB_REPOSITORY:-boshu2/agentops}"
WORKFLOW="validate.yml"
TIMEOUT_SECONDS=900
POLL_INTERVAL=10
WATCH=true
TARGET=""

usage() {
    cat <<'USAGE'
Usage: scripts/verify-release-ci.sh <tag-or-sha> [options]

Verify that a GitHub Actions workflow has a green run for the exact release
commit. By default this checks validate.yml, because a release is not done
until the tagged SHA has a successful Validate run.

Options:
  --repo OWNER/REPO       GitHub repository (default: GITHUB_REPOSITORY or boshu2/agentops)
  --workflow NAME         Workflow file/name to check (default: validate.yml)
  --timeout SECONDS       Seconds to wait for a matching run to appear (default: 900)
  --poll-interval SECONDS Seconds between run-discovery polls (default: 10)
  --no-watch              Do not gh run watch in-progress runs; report current status
  -h, --help              Show this help
USAGE
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo)
            [[ -n "${2:-}" ]] || die "--repo requires OWNER/REPO"
            REPO="$2"
            shift 2
            ;;
        --workflow)
            [[ -n "${2:-}" ]] || die "--workflow requires a workflow file or name"
            WORKFLOW="$2"
            shift 2
            ;;
        --timeout)
            [[ "${2:-}" =~ ^[0-9]+$ ]] || die "--timeout requires a non-negative integer"
            TIMEOUT_SECONDS="$2"
            shift 2
            ;;
        --poll-interval)
            [[ "${2:-}" =~ ^[0-9]+$ ]] || die "--poll-interval requires a non-negative integer"
            POLL_INTERVAL="$2"
            shift 2
            ;;
        --no-watch)
            WATCH=false
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --*)
            die "unknown option: $1"
            ;;
        *)
            [[ -z "$TARGET" ]] || die "multiple targets provided: $TARGET and $1"
            TARGET="$1"
            shift
            ;;
    esac
done

[[ -n "$TARGET" ]] || {
    usage
    exit 1
}

cd "$REPO_ROOT"

if ! command -v gh >/dev/null 2>&1; then
    die "gh CLI is required to verify release CI"
fi

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required to verify release CI"
fi

TARGET_SHA="$(git rev-parse --verify --quiet "$TARGET^{commit}" 2>/dev/null || true)"
if [[ -z "$TARGET_SHA" ]]; then
    die "target does not resolve to a commit in this checkout: $TARGET"
fi

find_run_json() {
    gh run list \
        --repo "$REPO" \
        --workflow "$WORKFLOW" \
        --limit 100 \
        --json databaseId,headSha,status,conclusion,url,createdAt,event,displayTitle \
        | jq -c --arg sha "$TARGET_SHA" '
            [.[] | select(.headSha == $sha)]
            | sort_by(.createdAt)
            | reverse
            | .[0] // empty
        '
}

view_run_json() {
    local run_id="$1"
    gh run view "$run_id" \
        --repo "$REPO" \
        --json databaseId,headSha,status,conclusion,url,createdAt,event,displayTitle \
        | jq -c '.'
}

run_json=""
deadline=$((SECONDS + TIMEOUT_SECONDS))

while true; do
    if ! run_json="$(find_run_json)"; then
        die "gh run list failed for repo=$REPO workflow=$WORKFLOW"
    fi
    if [[ -n "$run_json" ]]; then
        break
    fi

    if (( SECONDS >= deadline )); then
        echo "NO-GO release-ci: no $WORKFLOW run found for target=$TARGET sha=$TARGET_SHA repo=$REPO" >&2
        exit 1
    fi

    sleep "$POLL_INTERVAL"
done

run_id="$(jq -r '.databaseId // empty' <<<"$run_json")"
status="$(jq -r '.status // "unknown"' <<<"$run_json")"
conclusion="$(jq -r '.conclusion // "unknown"' <<<"$run_json")"
url="$(jq -r '.url // ""' <<<"$run_json")"

[[ -n "$run_id" ]] || die "matching workflow run did not include databaseId"

if [[ "$status" != "completed" && "$WATCH" == "true" ]]; then
    echo "WAIT release-ci: workflow=$WORKFLOW target=$TARGET sha=$TARGET_SHA run_id=$run_id status=$status"
    if ! gh run watch "$run_id" --repo "$REPO" --exit-status; then
        run_json="$(view_run_json "$run_id" || true)"
        status="$(jq -r '.status // "unknown"' <<<"$run_json" 2>/dev/null || echo "unknown")"
        conclusion="$(jq -r '.conclusion // "unknown"' <<<"$run_json" 2>/dev/null || echo "unknown")"
        url="$(jq -r '.url // ""' <<<"$run_json" 2>/dev/null || echo "")"
        echo "NO-GO release-ci: workflow=$WORKFLOW target=$TARGET sha=$TARGET_SHA run_id=$run_id status=$status conclusion=$conclusion url=$url" >&2
        exit 1
    fi

    run_json="$(view_run_json "$run_id")"
    status="$(jq -r '.status // "unknown"' <<<"$run_json")"
    conclusion="$(jq -r '.conclusion // "unknown"' <<<"$run_json")"
    url="$(jq -r '.url // ""' <<<"$run_json")"
fi

if [[ "$status" == "completed" && "$conclusion" == "success" ]]; then
    echo "GO release-ci: workflow=$WORKFLOW target=$TARGET sha=$TARGET_SHA run_id=$run_id status=$status conclusion=$conclusion url=$url"
    exit 0
fi

echo "NO-GO release-ci: workflow=$WORKFLOW target=$TARGET sha=$TARGET_SHA run_id=$run_id status=$status conclusion=$conclusion url=$url" >&2
exit 1

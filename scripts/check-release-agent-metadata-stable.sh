#!/usr/bin/env bash
set -euo pipefail

# Guard release-prep validation against incidental AgentOps metadata churn.
# The guarded command may read .agents/findings and citation telemetry, but it
# must not change tracked finding metadata unless the operator opts in.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
    cat <<'USAGE'
Usage: scripts/check-release-agent-metadata-stable.sh [--repo-root DIR] -- COMMAND [ARGS...]

Run COMMAND and fail if it changes tracked AgentOps finding/citation metadata.

Options:
  --repo-root DIR   Repository root to guard (default: script parent)
  -h, --help        Show this help

Environment:
  AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1
      Opt out and run COMMAND directly.
USAGE
}

truthy() {
    case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|y|on|always) return 0 ;;
        *) return 1 ;;
    esac
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo-root)
            REPO_ROOT="${2:-}"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ $# -eq 0 ]]; then
    echo "Missing command to guard" >&2
    usage >&2
    exit 2
fi

if [[ -z "$REPO_ROOT" ]] || ! git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "--repo-root must point at a git worktree" >&2
    exit 2
fi

if truthy "${AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS:-0}"; then
    echo "AgentOps metadata mutation guard disabled by AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1"
    "$@"
    exit $?
fi

hash_file() {
    local file="$1"
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$file" | awk '{print $1}'
    elif command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file" | awk '{print $1}'
    else
        cksum "$file" | awk '{print $1 ":" $2}'
    fi
}

snapshot_metadata() {
    local path full
    while IFS= read -r -d '' path; do
        full="$REPO_ROOT/$path"
        if [[ -f "$full" ]]; then
            printf '%s  %s\n' "$(hash_file "$full")" "$path"
        else
            printf 'MISSING  %s\n' "$path"
        fi
    done < <(git -C "$REPO_ROOT" ls-files -z -- .agents/findings .agents/ao/citations.jsonl)
}

before="$(mktemp "${TMPDIR:-/tmp}/agentops-release-metadata-before.XXXXXX")"
after="$(mktemp "${TMPDIR:-/tmp}/agentops-release-metadata-after.XXXXXX")"
trap 'rm -f "$before" "$after"' EXIT

snapshot_metadata | LC_ALL=C sort > "$before"

set +e
"$@"
cmd_rc=$?
set -e

snapshot_metadata | LC_ALL=C sort > "$after"

if ! diff_output="$(diff -u "$before" "$after" 2>/dev/null)"; then
    echo "FAIL: release validation mutated tracked AgentOps finding/citation metadata" >&2
    echo "Set AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1 only for intentional metadata updates." >&2
    printf '%s\n' "$diff_output" >&2
    exit 1
fi

exit "$cmd_rc"

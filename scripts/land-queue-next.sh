#!/usr/bin/env bash
# Peek the oldest unclaimed land request.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/land-queue-next.sh [--json]

Print the oldest unclaimed land request. Default output is:
  <bead><TAB><branch-ref>
EOF
}

FORMAT="tsv"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      FORMAT="json"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "land-queue-next: ERROR: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  :
else
  REPO_ROOT="$(pwd)"
fi

QUEUE_DIR="${AGENTOPS_LAND_QUEUE_DIR:-$REPO_ROOT/.agents/land-queue}"
QUEUE_FILE="${AGENTOPS_LAND_QUEUE_FILE:-$QUEUE_DIR/requests.jsonl}"

if [[ ! -s "$QUEUE_FILE" ]]; then
  echo "land-queue-next: no queued land requests" >&2
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  request="$(
    jq -s -c '
      map(select(type == "object" and (.status // "request") == "request" and ((.claimed // false) | not)))
      | sort_by(.timestamp, (.queue_seq // 0))
      | .[0] // empty
    ' "$QUEUE_FILE"
  )"
  if [[ -z "$request" ]]; then
    echo "land-queue-next: no unclaimed land requests" >&2
    exit 1
  fi
  if [[ "$FORMAT" == "json" ]]; then
    printf '%s\n' "$request"
  else
    jq -r '[.bead, .branch_ref] | @tsv' <<<"$request"
  fi
else
  first="$(sed -n '/[^[:space:]]/{p;q;}' "$QUEUE_FILE")"
  [[ -n "$first" ]] || {
    echo "land-queue-next: no queued land requests" >&2
    exit 1
  }
  if [[ "$FORMAT" == "json" ]]; then
    printf '%s\n' "$first"
  else
    bead="$(printf '%s\n' "$first" | sed -n 's/.*"bead":"\([^"]*\)".*/\1/p')"
    branch_ref="$(printf '%s\n' "$first" | sed -n 's/.*"branch_ref":"\([^"]*\)".*/\1/p')"
    [[ -n "$bead" && -n "$branch_ref" ]] || {
      echo "land-queue-next: queued request is not parseable without jq" >&2
      exit 2
    }
    printf '%s\t%s\n' "$bead" "$branch_ref"
  fi
fi

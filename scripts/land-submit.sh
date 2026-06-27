#!/usr/bin/env bash
# Submit the current bead branch to the serialized land queue.
#
# This deliberately pushes a non-trunk ref with --no-verify. The queued ref is
# never main/master; the land lane owns the later gated trunk push.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/land-submit.sh [--author-family <family>] <bead-id> [source-ref]

Push source-ref (default: HEAD) to refs/heads/land-queue/<bead-id> and append a
land request to .agents/land-queue/requests.jsonl.
EOF
}

die() {
  echo "land-submit: ERROR: $*" >&2
  exit 2
}

json_escape() {
  local s="${1:-}"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
  s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

json_request() {
  local timestamp="$1" bead="$2" branch_ref="$3" source_ref="$4" head_sha="$5" author_family="$6" queue_seq="$7"
  if command -v jq >/dev/null 2>&1; then
    jq -nc \
      --arg timestamp "$timestamp" \
      --arg bead "$bead" \
      --arg branch_ref "$branch_ref" \
      --arg source_ref "$source_ref" \
      --arg head_sha "$head_sha" \
      --arg author_family "$author_family" \
      --argjson queue_seq "$queue_seq" \
      '{
        timestamp: $timestamp,
        queue_seq: $queue_seq,
        bead: $bead,
        branch_ref: $branch_ref,
        source_ref: $source_ref,
        head_sha: $head_sha,
        author_family: $author_family,
        backend: "file",
        status: "request"
      }'
    return
  fi

  printf '{"timestamp":"%s","queue_seq":%s,"bead":"%s","branch_ref":"%s","source_ref":"%s","head_sha":"%s","author_family":"%s","backend":"file","status":"request"}\n' \
    "$(json_escape "$timestamp")" \
    "$queue_seq" \
    "$(json_escape "$bead")" \
    "$(json_escape "$branch_ref")" \
    "$(json_escape "$source_ref")" \
    "$(json_escape "$head_sha")" \
    "$(json_escape "$author_family")"
}

AUTHOR_FAMILY="${LAND_AUTHOR_FAMILY:-${AGENTOPS_AUTHOR_FAMILY:-codex}}"
QUEUE_BACKEND="${AGENTOPS_LAND_QUEUE_BACKEND:-file}"
BEAD=""
SOURCE_REF=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --author-family)
      [[ -n "${2:-}" ]] || die "--author-family requires a value"
      AUTHOR_FAMILY="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      die "unknown flag: $1"
      ;;
    *)
      if [[ -z "$BEAD" ]]; then
        BEAD="$1"
      elif [[ -z "$SOURCE_REF" ]]; then
        SOURCE_REF="$1"
      else
        die "too many positional arguments"
      fi
      shift
      ;;
  esac
done

[[ -n "$BEAD" ]] || die "missing bead id"
SOURCE_REF="${SOURCE_REF:-HEAD}"

case "$QUEUE_BACKEND" in
  file|fallback) ;;
  am|auto)
    die "Agent Mail queue backend is unavailable in this slice; use the file fallback (AGENTOPS_LAND_QUEUE_BACKEND=file)"
    ;;
  *)
    die "unknown queue backend: $QUEUE_BACKEND"
    ;;
esac

[[ "$BEAD" != */* ]] || die "bead id must not contain '/' because it becomes one ref component"
BRANCH_REF="refs/heads/land-queue/$BEAD"
git check-ref-format "$BRANCH_REF" >/dev/null 2>&1 || die "invalid queue ref: $BRANCH_REF"

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not inside a git repository"
cd "$REPO_ROOT"

HEAD_SHA="$(git rev-parse --verify "${SOURCE_REF}^{commit}" 2>/dev/null)" || die "source ref does not resolve to a commit: $SOURCE_REF"

if ! git diff --quiet -- || ! git diff --cached --quiet -- \
  || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  die "working tree has uncommitted changes; commit before submitting to land queue"
fi

git push --no-verify origin "${SOURCE_REF}:$BRANCH_REF"

QUEUE_DIR="${AGENTOPS_LAND_QUEUE_DIR:-$REPO_ROOT/.agents/land-queue}"
QUEUE_FILE="${AGENTOPS_LAND_QUEUE_FILE:-$QUEUE_DIR/requests.jsonl}"
LOCK_DIR="$QUEUE_DIR/.requests.lock"
LOCK_HELD=0

cleanup_lock() {
  if [[ "$LOCK_HELD" -eq 1 ]]; then
    rmdir "$LOCK_DIR" 2>/dev/null || true
  fi
}
trap cleanup_lock EXIT

mkdir -p "$QUEUE_DIR"
waited=0
timeout="${AGENTOPS_LAND_QUEUE_LOCK_TIMEOUT:-30}"
while ! mkdir "$LOCK_DIR" 2>/dev/null; do
  if [[ "$waited" -ge "$timeout" ]]; then
    die "timed out waiting for queue lock: $LOCK_DIR"
  fi
  sleep 1
  waited=$((waited + 1))
done
LOCK_HELD=1

queue_seq=1
if [[ -f "$QUEUE_FILE" ]]; then
  existing_lines="$(wc -l < "$QUEUE_FILE" | tr -d '[:space:]')"
  queue_seq=$((existing_lines + 1))
fi
timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
json_request "$timestamp" "$BEAD" "$BRANCH_REF" "$SOURCE_REF" "$HEAD_SHA" "$AUTHOR_FAMILY" "$queue_seq" >>"$QUEUE_FILE"

LOCK_HELD=0
rmdir "$LOCK_DIR" 2>/dev/null || true
trap - EXIT

echo "land-submit: pushed $HEAD_SHA to $BRANCH_REF"
echo "land-submit: queued $BEAD via file queue $QUEUE_FILE"

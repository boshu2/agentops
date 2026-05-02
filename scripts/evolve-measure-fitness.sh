#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/evolve-measure-fitness.sh --output <path> [options]

Capture an evolve fitness snapshot atomically. The script bounds total runtime,
validates the JSON shape, and only replaces the rolling snapshot on success.

Options:
  --output <path>         Output path for the final JSON snapshot (required)
  --timeout <seconds>     Per-goal timeout passed to ao goals measure (default: 60)
  --total-timeout <sec>   Whole-command timeout bound (default: 75)
  --repo-root <path>      Repo root to run from (default: current directory)
  --goal <goal-id>        Optional goal filter to pass through
  --file <path>           Optional goals file override to pass through
  -h, --help              Show help
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

resolve_path() {
  local base="$1"
  local path="$2"
  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
  else
    printf '%s/%s\n' "$base" "$path"
  fi
}

OUTPUT_PATH=""
TIMEOUT="60"
TOTAL_TIMEOUT="75"
REPO_ROOT="$(pwd)"
PASSTHRU_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      OUTPUT_PATH="${2:-}"
      shift 2
      ;;
    --timeout)
      TIMEOUT="${2:-}"
      shift 2
      ;;
    --total-timeout)
      TOTAL_TIMEOUT="${2:-}"
      shift 2
      ;;
    --repo-root)
      REPO_ROOT="${2:-}"
      shift 2
      ;;
    --goal|--file)
      PASSTHRU_ARGS+=("$1" "${2:-}")
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

require_cmd ao
require_cmd jq
require_cmd timeout

[[ -n "$OUTPUT_PATH" ]] || die "--output is required"
[[ "$TIMEOUT" =~ ^[0-9]+$ ]] || die "--timeout must be numeric"
[[ "$TOTAL_TIMEOUT" =~ ^[0-9]+$ ]] || die "--total-timeout must be numeric"

OUTPUT_ABS="$(resolve_path "$REPO_ROOT" "$OUTPUT_PATH")"
mkdir -p "$(dirname "$OUTPUT_ABS")"

TMP_FILE="$(mktemp "${TMPDIR:-/tmp}/evolve-fitness.XXXXXX")"
trap 'rm -f "$TMP_FILE"' EXIT

COMMAND_TIMEOUT="$TOTAL_TIMEOUT"
if [[ "$TOTAL_TIMEOUT" -gt 0 ]]; then
  COMMAND_TIMEOUT=$((TOTAL_TIMEOUT + 5))
fi

set +e
(
  cd "$REPO_ROOT"
  timeout "$COMMAND_TIMEOUT" ao goals measure --json --timeout "$TIMEOUT" --total-timeout "$TOTAL_TIMEOUT" "${PASSTHRU_ARGS[@]}" >"$TMP_FILE"
)
rc=$?
set -e

if [[ "$rc" -ne 0 ]]; then
  rm -f "$TMP_FILE"
  if [[ "$rc" -eq 124 ]]; then
    die "goals measurement exceeded total timeout (${TOTAL_TIMEOUT}s)"
  fi
  die "goals measurement failed (exit $rc)"
fi

jq -e '.goals | type == "array"' "$TMP_FILE" >/dev/null 2>&1 || {
  rm -f "$TMP_FILE"
  die "goals measurement did not produce valid goals JSON"
}

mv "$TMP_FILE" "$OUTPUT_ABS"
trap - EXIT

printf '%s\n' "$OUTPUT_ABS"

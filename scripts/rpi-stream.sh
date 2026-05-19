#!/usr/bin/env bash
set -euo pipefail

# rpi-stream.sh
# Tail/filter structured per-worker runtime output written by tmux executor.
#
# Log source files:
#   .agents/rpi/runs/<run-id>/phase-<n>-exit.w<k>.jsonl

usage() {
  cat <<'EOF'
Usage:
  scripts/rpi-stream.sh [run-id] [options]

Options:
  --phase N       Filter to one phase number
  --worker N      Filter to one worker number
  --raw           Print raw lines (no jq filter)
  --no-follow     Print snapshot only (no tail -F)
  --lines N       Initial line count per file (default: 20)
  -h, --help      Show help

Examples:
  scripts/rpi-stream.sh
  scripts/rpi-stream.sh 60d881b65b19
  scripts/rpi-stream.sh 60d881b65b19 --phase 2 --worker 1
  scripts/rpi-stream.sh 60d881b65b19 --raw --lines 200
EOF
}

RUN_ID=""
PHASE=""
WORKER=""
RAW=0
FOLLOW=1
LINES=20

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --phase)
      PHASE="${2:-}"
      shift 2
      ;;
    --worker)
      WORKER="${2:-}"
      shift 2
      ;;
    --raw)
      RAW=1
      shift
      ;;
    --no-follow)
      FOLLOW=0
      shift
      ;;
    --lines)
      LINES="${2:-20}"
      shift 2
      ;;
    *)
      if [[ -z "$RUN_ID" ]]; then
        RUN_ID="$1"
        shift
      else
        echo "error: unexpected argument: $1" >&2
        usage
        exit 1
      fi
      ;;
  esac
done

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required" >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
RUNS_DIR="$REPO_ROOT/.agents/rpi/runs"

if [[ ! -d "$RUNS_DIR" ]]; then
  echo "error: runs directory not found: $RUNS_DIR" >&2
  exit 1
fi

if [[ -z "$RUN_ID" ]]; then
  RUN_ID="$(find "$RUNS_DIR" -mindepth 1 -maxdepth 1 -type d -print0 \
    | xargs -0 stat -f '%m %N' \
    | sort -nr \
    | awk 'NR==1 {print $2}')"
  RUN_ID="$(basename "${RUN_ID:-}")"
fi

if [[ -z "$RUN_ID" ]]; then
  echo "error: could not determine run id" >&2
  exit 1
fi

RUN_DIR="$RUNS_DIR/$RUN_ID"
if [[ ! -d "$RUN_DIR" ]]; then
  echo "error: run directory not found: $RUN_DIR" >&2
  exit 1
fi

phase_glob="*"
worker_glob="*"
if [[ -n "$PHASE" ]]; then
  phase_glob="$PHASE"
fi
if [[ -n "$WORKER" ]]; then
  worker_glob="$WORKER"
fi

mapfile -t LOG_FILES < <(find "$RUN_DIR" -maxdepth 1 -type f -name "phase-${phase_glob}-exit.w${worker_glob}.jsonl" | sort)

if [[ ${#LOG_FILES[@]} -eq 0 ]]; then
  echo "error: no worker stream logs found for run $RUN_ID in $RUN_DIR" >&2
  echo "hint: expected files like phase-<n>-exit.w<k>.jsonl" >&2
  exit 1
fi

print_line() {
  local label="$1"
  local line="$2"
  local output="$line"
  if [[ "$RAW" -eq 0 && "$line" == \{* ]]; then
    local parsed
    parsed="$(printf '%s\n' "$line" | jq -r '
      def ev: .type // .event // .event_type // .message?.type // .kind // "event";
      def tool: .tool_name // .tool?.name // .call?.name // .function?.name // .name // "";
      def body: .delta // .text // .content // .message?.content // .reasoning // .summary // .error // .details // "";
      [ev, tool, (if (body|type) == "string" then body else (body|tostring) end)] | @tsv
    ' 2>/dev/null || true)"
    if [[ -n "$parsed" ]]; then
      local ev tool body
      IFS=$'\t' read -r ev tool body <<<"$parsed"
      body="${body//$'\n'/\\n}"
      if [[ ${#body} -gt 220 ]]; then
        body="${body:0:220}..."
      fi
      output="$(printf '%-28s %-24s %s' "$ev" "${tool:--}" "$body")"
    fi
  fi
  printf '[%s] %s\n' "$label" "$output"
}

stream_file() {
  local file="$1"
  local label
  label="$(basename "$file")"
  if [[ "$FOLLOW" -eq 1 ]]; then
    tail -n "$LINES" -F "$file" 2>/dev/null | while IFS= read -r line; do
      print_line "$label" "$line"
    done
  else
    tail -n "$LINES" "$file" 2>/dev/null | while IFS= read -r line; do
      print_line "$label" "$line"
    done
  fi
}

echo "Run: $RUN_ID"
echo "Logs:"
printf '  - %s\n' "${LOG_FILES[@]}"
echo

if [[ "$FOLLOW" -eq 1 ]]; then
  pids=()
  for file in "${LOG_FILES[@]}"; do
    stream_file "$file" &
    pids+=("$!")
  done
  # shellcheck disable=SC2154  # 'p' is the loop variable inside the trap body
  trap 'for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done' EXIT INT TERM
  wait
else
  for file in "${LOG_FILES[@]}"; do
    stream_file "$file"
  done
fi

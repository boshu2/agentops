#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

MODE="quick"
JSON_OUTPUT=false
REQUIRE_TOOLS=false

usage() {
  cat <<'USAGE'
Usage: scripts/security-gate.sh [--mode quick|full] [--json] [--require-tools]

Runs the unified security gate using scripts/toolchain-validate.sh.

Options:
  --mode quick|full   quick = skip slow tests (default), full = full suite
  --json              output machine-readable summary JSON
  --require-tools     fail if any scanner reports not_installed/error
  -h, --help          show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --json)
      JSON_OUTPUT=true
      shift
      ;;
    --require-tools)
      REQUIRE_TOOLS=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$MODE" != "quick" && "$MODE" != "full" ]]; then
  echo "Invalid mode: $MODE (expected quick or full)" >&2
  exit 1
fi

# Canonical scanner invocation contract: scripts/toolchain-validate.sh --gate
TOOLCHAIN_SCRIPT="${SECURITY_GATE_TOOLCHAIN_SCRIPT:-scripts/toolchain-validate.sh}"
if [[ ! -x "$TOOLCHAIN_SCRIPT" ]]; then
  echo "Missing executable: $TOOLCHAIN_SCRIPT" >&2
  exit 1
fi

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-${MODE}"
SECURITY_BASE="${SECURITY_GATE_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-security}"
SECURITY_DIR="$SECURITY_BASE/$RUN_ID"
mkdir -p "$SECURITY_DIR"

TOOLCHAIN_ARGS=(--gate --json)
if [[ "$MODE" == "quick" ]]; then
  TOOLCHAIN_ARGS=(--quick --gate --json)
fi

set +e
TOOLCHAIN_OUTPUT="$($TOOLCHAIN_SCRIPT "${TOOLCHAIN_ARGS[@]}" 2>&1)"
TOOLCHAIN_EXIT=$?
set -e

SUMMARY_JSON="$SECURITY_DIR/summary.json"
printf '%s\n' "$TOOLCHAIN_OUTPUT" > "$SUMMARY_JSON"

TOOLING_SRC="${TOOLCHAIN_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-tooling}"
if [[ -d "$TOOLING_SRC" ]]; then
  cp -a "$TOOLING_SRC/." "$SECURITY_DIR/" 2>/dev/null || true
fi

if command -v jq >/dev/null 2>&1 && jq empty "$SUMMARY_JSON" >/dev/null 2>&1; then
  GATE_STATUS="$(jq -r '.gate_status // "UNKNOWN"' "$SUMMARY_JSON")"
  MISSING_TOOLS="$(jq -r '[.tools[] | select(. == "not_installed" or . == "error")] | length' "$SUMMARY_JSON")"

  EXTENDED_JSON="$SECURITY_DIR/security-gate-summary.json"
  jq -n \
    --arg mode "$MODE" \
    --arg run_id "$RUN_ID" \
    --arg output_dir "$SECURITY_DIR" \
    --argjson toolchain "$(cat "$SUMMARY_JSON")" \
    --arg gate_status "$GATE_STATUS" \
    --argjson missing_tools "$MISSING_TOOLS" \
    --arg require_tools "$REQUIRE_TOOLS" \
    '{
      mode: $mode,
      run_id: $run_id,
      output_dir: $output_dir,
      gate_status: $gate_status,
      missing_tool_count: $missing_tools,
      require_tools: ($require_tools == "true"),
      toolchain: $toolchain
    }' > "$EXTENDED_JSON"

  if [[ "$REQUIRE_TOOLS" == "true" && "$MISSING_TOOLS" -gt 0 ]]; then
    if [[ "$JSON_OUTPUT" == "true" ]]; then
      cat "$EXTENDED_JSON"
    else
      echo "Security gate FAILED: missing/error tools detected ($MISSING_TOOLS)"
      echo "Report: $EXTENDED_JSON"
    fi
    exit 4
  fi

  if [[ "$JSON_OUTPUT" == "true" ]]; then
    cat "$EXTENDED_JSON"
  else
    echo "Security gate mode: $MODE"
    echo "Gate status: $GATE_STATUS"
    echo "Missing/error tools: $MISSING_TOOLS"
    echo "Report: $EXTENDED_JSON"
  fi
else
  if [[ "$JSON_OUTPUT" == "true" ]]; then
    jq -n \
      --arg mode "$MODE" \
      --arg run_id "$RUN_ID" \
      --arg output_dir "$SECURITY_DIR" \
      --arg raw "$TOOLCHAIN_OUTPUT" \
      '{mode: $mode, run_id: $run_id, output_dir: $output_dir, parse_error: true, raw_output: $raw}'
  else
    echo "Security gate warning: toolchain output was not valid JSON"
    echo "Raw output saved to: $SUMMARY_JSON"
  fi
  exit 1
fi

# Preserve toolchain gate semantics for findings.
if [[ "$TOOLCHAIN_EXIT" -ne 0 ]]; then
  exit "$TOOLCHAIN_EXIT"
fi

exit 0

# shellcheck shell=bash
# tests/lib/e2e-logger.sh — structured JSON-line logging for e2e tests.
#
# Implements the "Structured Test Logging" pattern from the
# testing-real-service-e2e-no-mocks skill. Emits both a human-friendly text
# line to the test's text log and a JSON-line event to a parallel sidecar
# log that CI can machine-parse.
#
# Why JSON-line: when an e2e test fails in CI, the human-readable log shows
# WHAT broke but not the timing, the phase, or the prior state. The sidecar
# gives a parseable record of phases, asserts (with expected/actual), and
# artifact snapshots — drop it into jq or any log aggregator.
#
# Usage:
#   source "${SCRIPT_DIR}/../lib/e2e-logger.sh"
#   e2e_log_init "flywheel-proof" "$WORK_DIR/proof-run.jsonl"
#   e2e_log_phase setup
#   e2e_log_event "pass" "forge produced pending learnings" '{"count":3}'
#   e2e_log_assert "lookup returns artifact" expected actual match
#   e2e_log_artifact "$ARTIFACT_PATH" "promoted-artifact"
#   e2e_log_summary  # at end; emits suite_end event with pass/fail counts
#
# Output schema (one JSON object per line, stable field names):
#   ts        ISO-8601 UTC, second precision
#   suite     test suite name (from e2e_log_init)
#   phase     last phase declared via e2e_log_phase
#   event     one of: suite_start | phase_start | pass | fail | assert |
#             artifact | db_snapshot | suite_end
#   message   human-readable label
#   data      optional JSON object with structured fields

[[ -n "${E2E_LOGGER_SH_LOADED:-}" ]] && return 0
E2E_LOGGER_SH_LOADED=1

E2E_LOG_SUITE=""
E2E_LOG_PHASE="init"
E2E_LOG_FILE=""
E2E_LOG_PASS_COUNT=0
E2E_LOG_FAIL_COUNT=0
E2E_LOG_START_EPOCH=0

_e2e_log_ts() {
  date -u +'%Y-%m-%dT%H:%M:%SZ'
}

# _e2e_log_emit <event> <message> [data-json]
# Writes one JSON-line event. Falls back to a degraded text line if jq is
# missing (e2e logging never blocks a test on a missing optional dependency).
_e2e_log_emit() {
  local event="$1" message="$2" data="${3:-null}"
  if [[ -z "$E2E_LOG_FILE" ]]; then
    return 0
  fi
  if command -v jq >/dev/null 2>&1; then
    jq -nc \
      --arg ts "$(_e2e_log_ts)" \
      --arg suite "$E2E_LOG_SUITE" \
      --arg phase "$E2E_LOG_PHASE" \
      --arg event "$event" \
      --arg message "$message" \
      --argjson data "$data" \
      '{ts:$ts, suite:$suite, phase:$phase, event:$event, message:$message, data:$data}' \
      >>"$E2E_LOG_FILE"
  else
    printf '{"ts":"%s","suite":"%s","phase":"%s","event":"%s","message":"%s"}\n' \
      "$(_e2e_log_ts)" "$E2E_LOG_SUITE" "$E2E_LOG_PHASE" "$event" \
      "$(printf '%s' "$message" | sed 's/"/\\"/g')" \
      >>"$E2E_LOG_FILE"
  fi
}

# e2e_log_init <suite-name> <log-file>
# Truncates the sidecar log and emits the suite_start event. Call before any
# other e2e_log_* helper.
e2e_log_init() {
  E2E_LOG_SUITE="$1"
  E2E_LOG_FILE="$2"
  E2E_LOG_PHASE="init"
  E2E_LOG_PASS_COUNT=0
  E2E_LOG_FAIL_COUNT=0
  E2E_LOG_START_EPOCH=$(date +%s)
  : >"$E2E_LOG_FILE"
  _e2e_log_emit "suite_start" "$E2E_LOG_SUITE" 'null'
}

# e2e_log_phase <phase-name>
# Records a phase transition (setup, act, assert, teardown, or a custom
# phase string).
e2e_log_phase() {
  E2E_LOG_PHASE="$1"
  _e2e_log_emit "phase_start" "$E2E_LOG_PHASE" 'null'
}

# e2e_log_pass <message> [data-json]
# Records a successful check.
e2e_log_pass() {
  E2E_LOG_PASS_COUNT=$((E2E_LOG_PASS_COUNT + 1))
  _e2e_log_emit "pass" "$1" "${2:-null}"
}

# e2e_log_fail <message> [data-json]
# Records a failed check. Does NOT exit — caller decides whether to exit.
e2e_log_fail() {
  E2E_LOG_FAIL_COUNT=$((E2E_LOG_FAIL_COUNT + 1))
  _e2e_log_emit "fail" "$1" "${2:-null}"
}

# e2e_log_assert <label> <expected> <actual> <match:true|false>
# Records a structured assertion event. Use this in addition to pass/fail
# when the expected/actual values are interesting.
e2e_log_assert() {
  local label="$1" expected="$2" actual="$3" match="$4"
  local data
  if command -v jq >/dev/null 2>&1; then
    data="$(jq -nc \
      --arg expected "$expected" \
      --arg actual "$actual" \
      --argjson match "$match" \
      '{expected:$expected, actual:$actual, match:$match}')"
  else
    data='{"match":'"$match"'}'
  fi
  _e2e_log_emit "assert" "$label" "$data"
  if [[ "$match" == "true" ]]; then
    E2E_LOG_PASS_COUNT=$((E2E_LOG_PASS_COUNT + 1))
  else
    E2E_LOG_FAIL_COUNT=$((E2E_LOG_FAIL_COUNT + 1))
  fi
}

# e2e_log_artifact <path> [label]
# Records that an artifact was produced. Captures size + mtime (no content,
# to keep the log small — content lives on disk inside the temp work dir).
e2e_log_artifact() {
  local path="$1" label="${2:-artifact}"
  local data='null'
  if [[ -e "$path" ]] && command -v jq >/dev/null 2>&1; then
    local size mtime
    size="$(wc -c <"$path" 2>/dev/null | tr -d ' ' || echo 0)"
    mtime="$(date -u -r "$path" +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || _e2e_log_ts)"
    data="$(jq -nc --arg path "$path" --arg size "$size" --arg mtime "$mtime" \
      '{path:$path, size:($size|tonumber), mtime:$mtime}')"
  fi
  _e2e_log_emit "artifact" "$label" "$data"
}

# e2e_log_summary
# Emits the suite_end event with total counts and duration. Call once at the
# very end of the test (after teardown).
e2e_log_summary() {
  local end_epoch duration data
  end_epoch=$(date +%s)
  duration=$((end_epoch - E2E_LOG_START_EPOCH))
  if command -v jq >/dev/null 2>&1; then
    data="$(jq -nc \
      --argjson pass "$E2E_LOG_PASS_COUNT" \
      --argjson fail "$E2E_LOG_FAIL_COUNT" \
      --argjson duration "$duration" \
      '{pass:$pass, fail:$fail, duration_s:$duration}')"
  else
    data='{"pass":'"$E2E_LOG_PASS_COUNT"',"fail":'"$E2E_LOG_FAIL_COUNT"'}'
  fi
  _e2e_log_emit "suite_end" "$E2E_LOG_SUITE" "$data"
}

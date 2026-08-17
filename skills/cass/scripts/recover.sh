#!/usr/bin/env bash
# Bounded CASS recovery. No detached work and no retained raw command logs.
set -uo pipefail

STATUS_TIMEOUT="${CASS_STATUS_TIMEOUT:-15}"
REFRESH_TIMEOUT="${CASS_REFRESH_TIMEOUT:-60}"
REBUILD_TIMEOUT="${CASS_REBUILD_TIMEOUT:-600}"
for spec in "STATUS_TIMEOUT:1:60" "REFRESH_TIMEOUT:1:120" "REBUILD_TIMEOUT:1:900"; do
  name="${spec%%:*}"; remainder="${spec#*:}"; low="${remainder%%:*}"; high="${remainder##*:}"
  value="${!name}"
  [[ "$value" =~ ^[0-9]+$ && "$value" -ge "$low" && "$value" -le "$high" ]] || {
    echo "BROKEN: $name must be an integer in [$low,$high]" >&2
    exit 2
  }
done

TIMEOUT_BIN=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null 2>&1; then TIMEOUT_BIN="$(command -v "$candidate")"; break; fi
done
[[ -n "$TIMEOUT_BIN" ]] || { echo "BROKEN: timeout or gtimeout is required" >&2; exit 2; }
command -v cass >/dev/null 2>&1 || { echo "BROKEN: cass binary not on PATH" >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { echo "BROKEN: jq binary not on PATH" >&2; exit 2; }

run_timed() {
  local seconds="$1"; shift
  "$TIMEOUT_BIN" --signal=TERM --kill-after=2s "$seconds" "$@"
}

cass_state() {
  local out
  out=$(run_timed "$STATUS_TIMEOUT" cass status --json 2>/dev/null | head -c 1048577) || out=""
  if [[ -z "$out" || ${#out} -gt 1048576 ]]; then
    printf 'false\tfalse\t0\t0\t\n'
    return
  fi
  printf '%s' "$out" | jq -r '[
      (.index.fresh // false), (.database.exists // false),
      (.index.documents // 0), (.database.messages // 0),
      (.recommended_action // "")
    ] | @tsv' 2>/dev/null || printf 'false\tfalse\t0\t0\t\n'
}

read_state() {
  local state_line
  state_line="$(cass_state)"
  IFS=$'\t' read -r FRESH DB_EXISTS DOCS MSGS REC <<< "$state_line" || true
  FRESH="${FRESH:-false}"; DB_EXISTS="${DB_EXISTS:-false}"
  DOCS="${DOCS:-0}"; MSGS="${MSGS:-0}"; REC="${REC:-}"
}

read_state
if [[ "$FRESH" == true ]]; then
  echo "READY: index fresh" >&2
  exit 0
fi

if [[ "$DB_EXISTS" == true && "$DOCS" != 0 && "$MSGS" != 0 ]]; then
  echo "STALE_BUT_USABLE: running one bounded incremental refresh (${REFRESH_TIMEOUT}s cap)" >&2
  if run_timed "$REFRESH_TIMEOUT" cass index --json >/dev/null 2>&1; then
    echo "REFRESHED: bounded incremental index completed" >&2
  else
    echo "STALE_BUT_USABLE: refresh failed or timed out; existing index remains queryable" >&2
  fi
  exit 0
fi

echo "RECOVERING: bounded doctor --fix" >&2
doctor_json=$(run_timed "$REBUILD_TIMEOUT" cass doctor --fix --json 2>/dev/null | head -c 1048577) || doctor_json=""
if [[ -n "$doctor_json" && ${#doctor_json} -le 1048576 ]]; then
  printf '%s' "$doctor_json" | jq -c '{
    healthy: (.healthy // false), status: (.status // "unknown"),
    issues_found: (.issues_found // 0), issues_fixed: (.issues_fixed // 0),
    failed_check_count: ([.checks[]? | select(.status=="fail")] | length)
  }' >&2 || echo '{"warning":"doctor output was not valid bounded JSON"}' >&2
else
  echo '{"warning":"doctor failed, timed out, or exceeded the output bound"}' >&2
fi

read_state
if [[ "$DB_EXISTS" == true && "$DOCS" != 0 ]]; then
  echo "RECOVERED: doctor produced a queryable index" >&2
  exit 0
fi

echo "ESCALATING: one bounded full rebuild (${REBUILD_TIMEOUT}s cap)" >&2
run_timed "$REBUILD_TIMEOUT" cass index --full --force-rebuild --json >/dev/null 2>&1
rebuild_rc=$?
case "$rebuild_rc" in
  0) : ;;
  124|137) echo "  ! rebuild timed out and was terminated" >&2 ;;
  *) echo "  ! rebuild exited $rebuild_rc" >&2 ;;
esac

read_state
if [[ "$DB_EXISTS" == true && "$DOCS" != 0 ]]; then
  echo "RECOVERED: index is queryable" >&2
  exit 0
fi

# Do not retain or print raw diagnostics: they commonly contain absolute source
# paths and command output. The operator gets a bounded factual state only.
diag=$(run_timed "$STATUS_TIMEOUT" cass diag --json 2>/dev/null | head -c 1048577) || diag=""
if [[ -n "$diag" && ${#diag} -le 1048576 ]]; then
  printf '%s' "$diag" | jq -c '{
    database_present: (.database != null), index_present: (.index != null),
    version_present: (.version != null)
  }' >&2 || true
fi
echo "BROKEN: cass cannot self-recover within the declared bounds" >&2
exit 1

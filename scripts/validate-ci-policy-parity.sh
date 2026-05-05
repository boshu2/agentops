#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

AGENTS_PATH="${CI_POLICY_PARITY_AGENTS_PATH:-$REPO_ROOT/AGENTS.md}"
WORKFLOW_PATH="${CI_POLICY_PARITY_WORKFLOW_PATH:-$REPO_ROOT/.github/workflows/validate.yml}"

if [[ ! -f "$AGENTS_PATH" ]]; then
  echo "CI_POLICY_PARITY: AGENTS file not found: $AGENTS_PATH"
  exit 1
fi

if [[ ! -f "$WORKFLOW_PATH" ]]; then
  echo "CI_POLICY_PARITY: workflow file not found: $WORKFLOW_PATH"
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

AGENTS_JOBS_FILE="$TMP_DIR/agents_jobs.txt"
AGENTS_NONBLOCKING_FILE="$TMP_DIR/agents_nonblocking.txt"
WF_NEEDS_FILE="$TMP_DIR/workflow_needs.txt"
WF_FAIL_FILE="$TMP_DIR/workflow_failset.txt"
WF_NONBLOCKING_FILE="$TMP_DIR/workflow_nonblocking.txt"
AGENTS_BLOCKING_FILE="$TMP_DIR/agents_blocking.txt"
WF_UNKNOWN_FAIL_FILE="$TMP_DIR/workflow_unknown_fail.txt"

extract_agents_jobs() {
  local file="$1"
  awk '
    BEGIN { in_table=0 }
    /^### CI Jobs and What They Check/ { in_table=1; next }
    in_table && /^### / { in_table=0 }
    in_table && /^\|/ { print }
  ' "$file" \
    | sed -nE 's/^\|[[:space:]]*\*\*([^*|]+)\*\*[[:space:]]*\|.*$/\1/p' \
    | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
    | sed '/^$/d' \
    | sort -u
}

extract_agents_nonblocking() {
  local file="$1"
  if command -v rg >/dev/null 2>&1; then
    rg -o --no-filename '[a-z0-9][a-z0-9-]*[[:space:]]*\(non-blocking\)' "$file" \
      | sed -E 's/[[:space:]]*\(non-blocking\)$//' \
      | sort -u
  else
    grep -Eo '[a-z0-9][a-z0-9-]*[[:space:]]*\(non-blocking\)' "$file" \
      | sed -E 's/[[:space:]]*\(non-blocking\)$//' \
      | sort -u
  fi
}

extract_summary_needs() {
  local file="$1"
  local needs_line
  needs_line="$(awk '
    /^  summary:/ { in_summary=1; next }
    in_summary && /^[[:space:]]*needs:[[:space:]]*\[/ { print; exit }
  ' "$file")"

  if [[ -z "$needs_line" ]]; then
    return 1
  fi

  needs_line="${needs_line#*[}"
  needs_line="${needs_line%]*}"

  printf '%s\n' "$needs_line" \
    | tr ',' '\n' \
    | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
    | sed '/^$/d' \
    | sort -u
}

extract_summary_failset() {
  local file="$1"
  # New pattern: contains(needs.*.result, 'failure') means all needs jobs are
  # in the fail set EXCEPT those with continue-on-error: true (which always
  # report success and thus can never trigger the failure check).
  if grep -q "contains(needs\.\*\.result" "$file"; then
    local coe_jobs
    coe_jobs="$(awk '
      /^  [a-z][a-z0-9_-]+:/ { job=$0; gsub(/^  /,"",job); gsub(/:/,"",job) }
      /continue-on-error:[[:space:]]*true/ { print job }
    ' "$file" | sort -u)"
    extract_summary_needs "$file" \
      | grep -v '^changes$' \
      | grep -vxF "$coe_jobs" \
      || true
    return
  fi
  # Legacy pattern: individual if [[ "needs.X.result" != "success" ]] checks
  awk '
    /^  summary:/ { in_summary=1 }
    in_summary && /^[[:space:]]*if[[:space:]]+\[\[/ { in_condition=1 }
    in_summary && in_condition { print }
    in_summary && in_condition && /then[[:space:]]*$/ { exit }
  ' "$file" \
    | grep -Eo 'needs\.[A-Za-z0-9_-]+\.result' \
    | sed -E 's/needs\.([A-Za-z0-9_-]+)\.result/\1/' \
    | sort -u
}

print_set_diff() {
  local left_label="$1"
  local left_file="$2"
  local right_label="$3"
  local right_file="$4"

  echo "--- $left_label"
  echo "+++ $right_label"
  if ! diff -u "$left_file" "$right_file"; then
    true
  fi
}

extract_agents_jobs "$AGENTS_PATH" > "$AGENTS_JOBS_FILE"
extract_agents_nonblocking "$AGENTS_PATH" > "$AGENTS_NONBLOCKING_FILE" || true
extract_summary_failset "$WORKFLOW_PATH" > "$WF_FAIL_FILE"

if ! extract_summary_needs "$WORKFLOW_PATH" > "$WF_NEEDS_FILE"; then
  echo "CI_POLICY_PARITY: unable to parse summary needs list from $WORKFLOW_PATH"
  echo "Expected style: summary.needs with bracket list (needs: [job-a, job-b])."
  exit 1
fi
# Exclude utility jobs (path-filter detection) from parity comparison
sed -i '/^changes$/d' "$WF_NEEDS_FILE"

if [[ ! -s "$AGENTS_JOBS_FILE" ]]; then
  echo "CI_POLICY_PARITY: no CI jobs parsed from AGENTS table under '### CI Jobs and What They Check'."
  exit 1
fi

if [[ ! -s "$WF_NEEDS_FILE" ]]; then
  echo "CI_POLICY_PARITY: summary.needs list is empty in $WORKFLOW_PATH"
  exit 1
fi

comm -23 "$WF_FAIL_FILE" "$WF_NEEDS_FILE" > "$WF_UNKNOWN_FAIL_FILE" || true
if [[ -s "$WF_UNKNOWN_FAIL_FILE" ]]; then
  echo "CI_POLICY_PARITY: workflow summary fail condition references jobs not present in summary.needs:"
  sed 's/^/  - /' "$WF_UNKNOWN_FAIL_FILE"
  exit 1
fi

comm -23 "$WF_NEEDS_FILE" "$WF_FAIL_FILE" > "$WF_NONBLOCKING_FILE" || true
comm -23 "$AGENTS_JOBS_FILE" "$AGENTS_NONBLOCKING_FILE" > "$AGENTS_BLOCKING_FILE" || true

errors=0

if ! diff -u "$AGENTS_JOBS_FILE" "$WF_NEEDS_FILE" >/dev/null; then
  echo "CI_POLICY_PARITY: Job list drift detected (AGENTS table vs validate.yml summary.needs)."
  print_set_diff "AGENTS jobs" "$AGENTS_JOBS_FILE" "Workflow summary.needs jobs" "$WF_NEEDS_FILE"
  echo "Action: align AGENTS CI table entries or summary.needs job list."
  echo ""
  errors=$((errors + 1))
fi

if ! diff -u "$AGENTS_NONBLOCKING_FILE" "$WF_NONBLOCKING_FILE" >/dev/null; then
  echo "CI_POLICY_PARITY: Non-blocking policy drift detected (AGENTS text vs workflow fail-closed set)."
  print_set_diff "AGENTS non-blocking jobs" "$AGENTS_NONBLOCKING_FILE" "Workflow non-blocking jobs" "$WF_NONBLOCKING_FILE"
  echo "Action: align '(non-blocking)' statements or summary fail condition checks."
  echo ""
  errors=$((errors + 1))
fi

if ! diff -u "$AGENTS_BLOCKING_FILE" "$WF_FAIL_FILE" >/dev/null; then
  echo "CI_POLICY_PARITY: Blocking job policy drift detected."
  print_set_diff "AGENTS blocking jobs" "$AGENTS_BLOCKING_FILE" "Workflow blocking jobs" "$WF_FAIL_FILE"
  echo "Action: AGENTS blocking set must match summary fail-condition job set."
  echo ""
  errors=$((errors + 1))
fi

if [[ "$errors" -gt 0 ]]; then
  echo "CI_POLICY_PARITY: FAILED ($errors drift group(s) detected)"
  exit 1
fi

echo "CI_POLICY_PARITY: PASS ($(wc -l < "$WF_NEEDS_FILE" | tr -d ' ') jobs; $(wc -l < "$WF_NONBLOCKING_FILE" | tr -d ' ') non-blocking)"
exit 0

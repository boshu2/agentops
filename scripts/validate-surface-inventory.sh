#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

INVENTORY_PATH="${VALIDATION_SURFACE_INVENTORY_PATH:-$REPO_ROOT/scripts/validation-surface-inventory.json}"
WORKFLOW_PATH="${VALIDATION_SURFACE_WORKFLOW_PATH:-$REPO_ROOT/.github/workflows/validate.yml}"

if [[ ! -f "$INVENTORY_PATH" ]]; then
  echo "SURFACE_INVENTORY: inventory not found: $INVENTORY_PATH"
  exit 1
fi

if [[ ! -f "$WORKFLOW_PATH" ]]; then
  echo "SURFACE_INVENTORY: workflow not found: $WORKFLOW_PATH"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "SURFACE_INVENTORY: jq is required"
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

WF_JOBS_FILE="$TMP_DIR/workflow_jobs.txt"
WF_NEEDS_FILE="$TMP_DIR/workflow_needs.txt"
WF_FAIL_FILE="$TMP_DIR/workflow_failset.txt"
WF_ADVISORY_FILE="$TMP_DIR/workflow_advisory.txt"
INV_CI_FILE="$TMP_DIR/inventory_ci_jobs.txt"
INV_BLOCKING_FILE="$TMP_DIR/inventory_blocking.txt"
INV_ADVISORY_FILE="$TMP_DIR/inventory_advisory.txt"

extract_workflow_jobs() {
  local file="$1"
  awk '
    /^jobs:/ { in_jobs=1; next }
    in_jobs && /^[^[:space:]][^:]*:/ { in_jobs=0 }
    in_jobs && /^  [A-Za-z0-9_-]+:/ {
      job=$1
      sub(/:$/, "", job)
      if (job != "summary") {
        print job
      }
    }
  ' "$file" | sort -u
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

print_diff() {
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

errors=0

if ! jq -e '
  .schema_version == 1
  and (.surfaces | type == "array")
  and all(.surfaces[]; (
    (.id | type == "string" and length > 0)
    and (.command | type == "string" and length > 0)
    and (.surface | type == "string" and length > 0)
    and (.category | type == "string" and length > 0)
    and (.purpose | type == "string" and length > 0)
    and (.blocking_policy == "blocking" or .blocking_policy == "advisory")
    and (.fast_behavior | type == "string" and length > 0)
    and (.full_behavior | type == "string" and length > 0)
    and has("ci_job")
    and (.docs_owner | type == "string" and length > 0)
  ))
' "$INVENTORY_PATH" >/dev/null; then
  echo "SURFACE_INVENTORY: inventory schema invalid"
  errors=$((errors + 1))
fi

duplicate_ids="$(jq -r '.surfaces[].id' "$INVENTORY_PATH" | sort | uniq -d)"
if [[ -n "$duplicate_ids" ]]; then
  echo "SURFACE_INVENTORY: duplicate ids:"
  echo "$duplicate_ids" | sed 's/^/  - /'
  errors=$((errors + 1))
fi

extract_workflow_jobs "$WORKFLOW_PATH" > "$WF_JOBS_FILE"
if ! extract_summary_needs "$WORKFLOW_PATH" > "$WF_NEEDS_FILE"; then
  echo "SURFACE_INVENTORY: unable to parse summary.needs from $WORKFLOW_PATH"
  exit 1
fi
extract_summary_failset "$WORKFLOW_PATH" > "$WF_FAIL_FILE"
comm -23 "$WF_NEEDS_FILE" "$WF_FAIL_FILE" > "$WF_ADVISORY_FILE" || true

jq -r '.surfaces[] | select(.ci_job != null) | .ci_job' "$INVENTORY_PATH" | sort -u > "$INV_CI_FILE"
jq -r '.surfaces[] | select(.ci_job != null and .blocking_policy == "blocking") | .ci_job' "$INVENTORY_PATH" | sort -u > "$INV_BLOCKING_FILE"
jq -r '.surfaces[] | select(.ci_job != null and .blocking_policy == "advisory") | .ci_job' "$INVENTORY_PATH" | sort -u > "$INV_ADVISORY_FILE"

if ! diff -u "$WF_JOBS_FILE" "$WF_NEEDS_FILE" >/dev/null; then
  echo "SURFACE_INVENTORY: workflow summary.needs does not cover every top-level job."
  print_diff "Workflow jobs" "$WF_JOBS_FILE" "Workflow summary.needs" "$WF_NEEDS_FILE"
  errors=$((errors + 1))
fi

if ! diff -u "$WF_NEEDS_FILE" "$INV_CI_FILE" >/dev/null; then
  echo "SURFACE_INVENTORY: inventory CI jobs drift from workflow summary.needs."
  print_diff "Workflow summary.needs" "$WF_NEEDS_FILE" "Inventory ci_job entries" "$INV_CI_FILE"
  errors=$((errors + 1))
fi

if ! diff -u "$WF_FAIL_FILE" "$INV_BLOCKING_FILE" >/dev/null; then
  echo "SURFACE_INVENTORY: blocking policy drift detected."
  print_diff "Workflow failset" "$WF_FAIL_FILE" "Inventory blocking jobs" "$INV_BLOCKING_FILE"
  errors=$((errors + 1))
fi

if ! diff -u "$WF_ADVISORY_FILE" "$INV_ADVISORY_FILE" >/dev/null; then
  echo "SURFACE_INVENTORY: advisory policy drift detected."
  print_diff "Workflow advisory jobs" "$WF_ADVISORY_FILE" "Inventory advisory jobs" "$INV_ADVISORY_FILE"
  errors=$((errors + 1))
fi

while IFS=$'\t' read -r id command; do
  case "$command" in
    github-actions:*|external:*|built-in:*)
      continue
      ;;
    bash\ *)
      script_path="${command#bash }"
      script_path="${script_path%% *}"
      ;;
    ./*|scripts/*|tests/*|skills/*)
      script_path="${command%% *}"
      ;;
    *)
      continue
      ;;
  esac

  script_path="${script_path#./}"
  if [[ ! -f "$REPO_ROOT/$script_path" ]]; then
    echo "SURFACE_INVENTORY: command path for $id does not exist: $script_path"
    errors=$((errors + 1))
  fi
done < <(jq -r '.surfaces[] | [.id, .command] | @tsv' "$INVENTORY_PATH")

if [[ "$errors" -gt 0 ]]; then
  echo "SURFACE_INVENTORY: FAILED ($errors issue group(s))"
  exit 1
fi

echo "SURFACE_INVENTORY: PASS ($(wc -l < "$INV_CI_FILE" | tr -d ' ') CI jobs inventoried)"
exit 0

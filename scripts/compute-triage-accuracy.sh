#!/usr/bin/env bash
# Compute triage accuracy from ground truth
#
# Usage: ./scripts/compute-triage-accuracy.sh
#
# Reads: $TOOLCHAIN_OUTPUT_DIR/triage-decisions.jsonl (default: $TMPDIR/agentops-tooling/)
# Output: Accuracy statistics by agent and tool
#
# Ground truth is set when:
# - CI confirms (test pass/fail)
# - Production incident occurs
# - Human reviews the decision
#
# To add ground truth:
#   # Read file, update ground_truth, write back
#   TDIR="${TOOLCHAIN_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-tooling}"
#   jq 'if .file_line == "src/auth.go:42" then .ground_truth = "TRUE_POS" else . end' \
#     "$TDIR/triage-decisions.jsonl" > tmp && mv tmp "$TDIR/triage-decisions.jsonl"

set -euo pipefail

TOOLING_DIR="${TOOLCHAIN_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-tooling}"
LOG_FILE="$TOOLING_DIR/triage-decisions.jsonl"

if [[ ! -f "$LOG_FILE" ]]; then
    echo "No triage decisions logged yet"
    echo "Log file: $LOG_FILE"
    exit 0
fi

# Count total decisions
total_decisions=$(wc -l < "$LOG_FILE" | tr -d '[:space:]')
echo "Total triage decisions: $total_decisions"
echo ""

# Count decisions with ground truth
with_ground_truth=$(jq -s '[.[] | select(.ground_truth != null)] | length' "$LOG_FILE")
without_ground_truth=$((total_decisions - with_ground_truth))

if [[ "$with_ground_truth" -eq 0 ]]; then
    echo "No ground truth available yet ($without_ground_truth decisions awaiting validation)"
    echo ""
    echo "Recent decisions (last 10):"
    tail -10 "$LOG_FILE" | jq -r '"\(.timestamp) \(.file_line) \(.tool) -> \(.verdict) by \(.agent)"'
    exit 0
fi

# Compute accuracy
correct=$(jq -s '[.[] | select(.ground_truth != null and .verdict == .ground_truth)] | length' "$LOG_FILE")
accuracy=$(echo "scale=1; $correct * 100 / $with_ground_truth" | bc)

echo "========================"
echo "TRIAGE ACCURACY REPORT"
echo "========================"
echo ""
echo "Overall: $correct / $with_ground_truth ($accuracy%)"
echo "Awaiting validation: $without_ground_truth"
echo ""

# By agent
echo "By Agent:"
echo "---------"
jq -rs '
  [.[] | select(.ground_truth != null)] |
  group_by(.agent) |
  .[] |
  {
    agent: .[0].agent,
    total: length,
    correct: [.[] | select(.verdict == .ground_truth)] | length
  } |
  "\(.agent): \(.correct)/\(.total) (\((.correct * 100 / .total) | floor)%)"
' "$LOG_FILE"

echo ""

# By tool
echo "By Tool:"
echo "--------"
jq -rs '
  [.[] | select(.ground_truth != null)] |
  group_by(.tool) |
  .[] |
  {
    tool: .[0].tool,
    total: length,
    correct: [.[] | select(.verdict == .ground_truth)] | length
  } |
  "\(.tool): \(.correct)/\(.total) (\((.correct * 100 / .total) | floor)%)"
' "$LOG_FILE"

echo ""

# False positive rate
false_pos_correct=$(jq -s '[.[] | select(.ground_truth == "FALSE_POS" and .verdict == "FALSE_POS")] | length' "$LOG_FILE")
false_pos_total=$(jq -s '[.[] | select(.ground_truth == "FALSE_POS")] | length' "$LOG_FILE")
if [[ "$false_pos_total" -gt 0 ]]; then
    fp_rate=$(echo "scale=1; $false_pos_correct * 100 / $false_pos_total" | bc)
    echo "False Positive Detection: $false_pos_correct/$false_pos_total ($fp_rate%)"
fi

# True positive rate
true_pos_correct=$(jq -s '[.[] | select(.ground_truth == "TRUE_POS" and .verdict == "TRUE_POS")] | length' "$LOG_FILE")
true_pos_total=$(jq -s '[.[] | select(.ground_truth == "TRUE_POS")] | length' "$LOG_FILE")
if [[ "$true_pos_total" -gt 0 ]]; then
    tp_rate=$(echo "scale=1; $true_pos_correct * 100 / $true_pos_total" | bc)
    echo "True Positive Detection: $true_pos_correct/$true_pos_total ($tp_rate%)"
fi

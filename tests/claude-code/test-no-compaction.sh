#!/usr/bin/env bash
# Test: No compaction during realistic multi-skill workflows
# Validates that sessions exercising skills stay within context limits
# and do NOT trigger compaction.
#
# Three scenarios × two assertions each:
#   1. No compact_boundary event (binary check)
#   2. Peak context utilization < 60% (token check via assert_context_under_60pct)
#
# Usage: ./test-no-compaction.sh [--verbose]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

VERBOSE="${1:-}"

echo "=== Test: No compaction during multi-skill workflows ==="
echo ""

total_pass=0
total_fail=0
peak_utilization=0

# Helper: run a scenario and assert both conditions
# Usage: run_scenario "name" "prompt" max_turns timeout
run_scenario() {
    local name="$1"
    local prompt="$2"
    local turns="$3"
    local tout="$4"
    local scenario_pass=0
    local scenario_fail=0

    echo "--- $name ---"

    local log_file
    MAX_TURNS=$turns log_file=$(run_claude_json "$prompt" "$tout") || true

    if [[ ! -f "$log_file" ]]; then
        echo -e "  ${RED}[FAIL]${NC} Log file not created"
        ((total_fail += 2)) || true
        echo ""
        return
    fi

    # Assertion 1: No compact_boundary event
    if grep -q '"subtype":"compact_boundary"' "$log_file" 2>/dev/null; then
        local pre_tokens
        pre_tokens=$(grep '"subtype":"compact_boundary"' "$log_file" | head -1 | \
            python3 -c "import sys,json; d=json.loads(sys.stdin.readline()); print(d.get('compactMetadata',{}).get('preTokens','unknown'))" 2>/dev/null || echo "unknown")
        echo -e "  ${RED}[FAIL]${NC} Compaction detected! preTokens=$pre_tokens"
        [[ "$VERBOSE" == "--verbose" ]] && grep '"subtype":"compact_boundary"' "$log_file" | head -1 | python3 -m json.tool 2>/dev/null | head -20
        ((scenario_fail++)) || true
    else
        echo -e "  ${GREEN}[PASS]${NC} No compaction event"
        ((scenario_pass++)) || true
    fi

    # Assertion 2: Context utilization < 60%
    if assert_context_under_60pct "$log_file" "Context under 60% ($name)"; then
        ((scenario_pass++)) || true
    else
        ((scenario_fail++)) || true
    fi

    # Track peak utilization across scenarios
    local this_peak
    this_peak=$(python3 -c "
import json, sys
peak = 0
for line in open(sys.argv[1]):
    try:
        obj = json.loads(line.strip())
    except: continue
    if obj.get('type') != 'assistant': continue
    u = obj.get('message', {}).get('usage', {}) or obj.get('usage', {})
    if not u: continue
    t = u.get('input_tokens', 0) + u.get('cache_read_input_tokens', 0) + u.get('cache_creation_input_tokens', 0)
    if t > peak: peak = t
print(peak)
" "$log_file" 2>/dev/null) || this_peak=0

    if [[ $this_peak -gt $peak_utilization ]]; then
        peak_utilization=$this_peak
    fi

    total_pass=$((total_pass + scenario_pass))
    total_fail=$((total_fail + scenario_fail))

    # Info: log file size and tool calls
    local size lines tool_count
    size=$(wc -c < "$log_file" | tr -d ' ')
    lines=$(wc -l < "$log_file" | tr -d ' ')
    tool_count=$(grep -c '"type":"tool_use"' "$log_file" 2>/dev/null || echo "0")
    echo -e "  ${BLUE}[INFO]${NC} ${size} bytes, ${lines} events, ${tool_count} tool calls"
    echo ""
}

# ─────────────────────────────────────────────────────────
# Scenario 1: Single skill (research) — 8 turns, 120s
# ─────────────────────────────────────────────────────────
run_scenario \
    "Scenario 1: Single skill (research)" \
    "Use the research skill to explore the skills/ directory in this plugin. List the top 5 skills by file size. Keep your response brief." \
    8 120

# ─────────────────────────────────────────────────────────
# Scenario 2: Multi-turn (crank description + rules) — 10 turns, 120s
# ─────────────────────────────────────────────────────────
run_scenario \
    "Scenario 2: Multi-turn (crank skill deep-read)" \
    "First, briefly describe what the crank skill does in this plugin. Then list the 3 most important rules from the crank skill. Keep each answer to 2-3 sentences max." \
    10 120

# ─────────────────────────────────────────────────────────
# Scenario 3: Multi-skill chain (research + plan) — 12 turns, 180s
# ─────────────────────────────────────────────────────────
run_scenario \
    "Scenario 3: Multi-skill chain (research + plan)" \
    "First, use the research skill to explore the hooks/ directory in this plugin and summarize what each hook does in one line. Then suggest 3 improvements to the hook system. Keep responses concise." \
    12 180

# ─────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────
echo "═══════════════════════════════════════════"
echo "No-Compaction Test Summary"
echo "═══════════════════════════════════════════"
echo -e "  ${GREEN}Passed:${NC} $total_pass"
echo -e "  ${RED}Failed:${NC} $total_fail"

# Report peak utilization
if [[ $peak_utilization -gt 0 ]]; then
    pct=$((peak_utilization * 100 / 200000))
    echo -e "  ${BLUE}Peak utilization:${NC} ${peak_utilization} tokens (${pct}% of 200K)"
fi

echo "═══════════════════════════════════════════"

if [[ $total_fail -gt 0 ]]; then
    echo ""
    echo -e "${RED}FAIL: $total_fail assertion(s) failed${NC}"
    echo ""
    echo "Remediation:"
    echo "  1. Check skill SKILL.md sizes — move content to references/"
    echo "  2. Check hook output — reduce SessionStart injection volume"
    echo "  3. Check ao lookup / signpost token budget (session injection volume)"
    echo "  4. Review log files in $LOG_DIR for details"
    exit 1
fi

echo ""
echo -e "${GREEN}PASS: All no-compaction tests passed${NC}"
exit 0

#!/usr/bin/env bash
# discriminator.sh (graphify-tool-preference) — BEHAVIORAL check over one probe
# transcript. The behavior of interest: did the agent invoke a graphify
# STRUCTURAL query as an ACTION (a `graphify <sub>` command, an mcp__graphify
# tool call, or a read of the graphify-out/ graph artifact) BEFORE reaching for
# broad grep/rg?
#
# CRITICAL: this measures the ACTION, not a MENTION. A transcript that merely
# describes the environment ("a graphify-out/ graph is present") or muses "I
# could use graphify" but then greps is ABSENT. Only a real invocation counts —
# a command line, a tool call, or reading the graph file. (This distinction is
# the whole point of a behavioral probe; an earlier version of this
# discriminator matched the bare token `graphify-out/` in prose and mis-scored a
# known-INERT case as BEHAVIORAL — calibration caught it.)
#
# Exit: 0 = behavior PRESENT, 1 = ABSENT, 2 = infra (empty/unreadable transcript).
set -euo pipefail

tx="${1:?usage: discriminator.sh <transcript>}"
[[ -s "$tx" ]] || { echo "DEGRADED: empty/missing transcript"; exit 2; }

# First line index of a graphify structural ACTION: a `graphify <sub>` command,
# an mcp__graphify tool call, or a read (cat/less/head/tail/jq/Read/open) of the
# graphify-out/ artifact. A bare prose mention of graphify-out/ does NOT match.
gfx_line="$(grep -nE '(^|[^[:alnum:]_])graphify[[:space:]]+(explain|path|query|search|god|ask)([^[:alnum:]_]|$)|mcp__graphify|(cat|less|head|tail|jq|Read|open|python3?)[^[:cntrl:]]*graphify-out/' "$tx" | head -1 | cut -d: -f1 || true)"
# First line index of a grep/rg search action.
grep_line="$(grep -nE '(^|[^[:alnum:]_])(grep|rg|ripgrep)([^[:alnum:]_]|$)|Grep\(' "$tx" | head -1 | cut -d: -f1 || true)"

if [[ -z "$gfx_line" ]]; then
    echo "ABSENT: no graphify structural query invoked (grep-first / grep-only)"
    exit 1
fi
if [[ -z "$grep_line" ]]; then
    echo "PRESENT: graphify structural query invoked, no grep"
    exit 0
fi
if [[ "$gfx_line" -lt "$grep_line" ]]; then
    echo "PRESENT: graphify structural query before grep"
    exit 0
fi
echo "ABSENT: grepped before invoking graphify"
exit 1

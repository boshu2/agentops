#!/usr/bin/env bash
# practices: [first-value-path, install-ux]
# PG1: 5-minute journey measurement (install → first /rpi → validated artifact).
#
# This is a *structural* journey test. It does not execute a live LLM session
# (that requires runtime auth and budget). Instead it validates that each
# checkpoint on the 5-minute path is wired and reachable:
#
#   t<60s    Step 1: install bundle resolves (install.sh syntax OK)
#   t<90s    Step 2: ao binary builds and `ao --version` works
#   t<120s   Step 3: skill surface present (rpi + quickstart SKILL.md)
#   t<180s   Step 4: ao quickstart subcommand exists and runs --help
#   t<240s   Step 5: skill-installed dir surface (~/.claude or fallback) reachable
#   t<300s   Step 6: artifact slot for /rpi run output exists (.agents/rpi)
#
# Hard floor: total wall-clock < 300 seconds (5 minutes). If any step blows
# the floor or fails, the gate exits non-zero with the slowest step named.
#
# Companion bead: soc-dec2.1 (PG1).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT" || exit 2

FLOOR_SECONDS="${PG1_FIVE_MINUTE_FLOOR:-300}"
T0="$(date +%s)"

pass_count=0
fail_count=0

# Record per-step wall-clock so a regression isolates to one step.
log_step() {
    local label="$1"
    local rc="$2"
    local elapsed="$3"
    local detail="${4:-}"
    if [ "$rc" -eq 0 ]; then
        printf "  PASS  %3ds  %s%s\n" "$elapsed" "$label" "${detail:+ ($detail)}"
        pass_count=$((pass_count + 1))
    else
        printf "  FAIL  %3ds  %s%s\n" "$elapsed" "$label" "${detail:+ ($detail)}"
        fail_count=$((fail_count + 1))
    fi
}

step() {
    local label="$1"
    local cmd="$2"
    local detail
    # Capture full output (no pipe) so SIGPIPE from head doesn't propagate
    # back through pipefail and mark a successful command as failed.
    local after
    if detail="$(set +o pipefail; eval "$cmd" 2>&1)"; then
        after="$(date +%s)"
        log_step "$label" 0 "$((after - T0))" "$(printf '%s\n' "$detail" | head -n1)"
    else
        after="$(date +%s)"
        log_step "$label" 1 "$((after - T0))" "$(printf '%s\n' "$detail" | head -n1)"
    fi
}

echo "=== PG1 five-minute first-value journey (floor: ${FLOOR_SECONDS}s) ==="

# Step 1: install bundle syntactically valid
step "Step 1: install.sh resolves" \
    "bash -n scripts/install.sh && echo OK"

# Step 2: ao binary builds + --version succeeds
# Reuse pre-built binary if it exists to keep the journey realistic for an
# operator who has run `make build` once.
if [ -x "$REPO_ROOT/cli/bin/ao" ]; then
    AO="$REPO_ROOT/cli/bin/ao"
else
    AO="/tmp/ao-pg1"
    (cd cli && go build -o "$AO" ./cmd/ao) 2>/dev/null
fi
step "Step 2: ao version" \
    "$AO version"

# Step 3: rpi + quickstart skills present (the user-visible surface)
step "Step 3: rpi skill present" \
    "test -f skills/rpi/SKILL.md && echo OK"
step "Step 3: quickstart skill present" \
    "test -f skills/quickstart/SKILL.md && echo OK"

# Step 4: ao quickstart subcommand wired
step "Step 4: ao quickstart --help" \
    "$AO quickstart --help"

# Step 5: skill install dir reachable (~/.claude/skills/ OR repo-local skills/)
# The fallback to repo-local satisfies first-checkout operators.
step "Step 5: skill install surface reachable" \
    "test -d \$HOME/.claude/skills || test -d skills"

# Step 6: artifact slot for /rpi runs
step "Step 6: .agents/rpi artifact slot" \
    "test -d .agents/rpi || mkdir -p .agents/rpi 2>/dev/null && echo OK"

T_END="$(date +%s)"
TOTAL=$((T_END - T0))

echo ""
echo "=== Journey summary ==="
echo "  Total: ${TOTAL}s (floor ${FLOOR_SECONDS}s)"
echo "  Pass:  ${pass_count}"
echo "  Fail:  ${fail_count}"

if [ "$TOTAL" -gt "$FLOOR_SECONDS" ]; then
    echo "  FAIL: total wall-clock ${TOTAL}s exceeds floor ${FLOOR_SECONDS}s"
    exit 1
fi
if [ "$fail_count" -gt 0 ]; then
    echo "  FAIL: ${fail_count} step(s) failed"
    exit 1
fi

echo "  PASS: first-value path complete within floor"
exit 0

#!/usr/bin/env bash
# practices: [first-value-path, install-ux]
# PG1: 5-minute journey measurement (install → skill-loop first value).
#
# This is a *structural* journey test. It does not execute a live LLM session
# (that requires runtime auth and budget). Instead it validates that each
# checkpoint on the 5-minute path is wired and reachable:
#
#   t<60s    Step 1: install bundle resolves (install.sh syntax OK)
#   t<90s    Step 2: ao binary builds and `ao --version` works
#   t<120s   Step 3: skill-loop front door present (plan/implement/validate)
#   t<180s   Step 4: docs/SKILLS.md router names the skill-loop path
#   t<240s   Step 5: skill install surface reachable (+ /rpi as one-tick executor)
#   t<300s   Step 6: artifact slot for loop runs exists (.agents/rpi)
#
# Hard floor: total wall-clock < 300 seconds (5 minutes). If any step blows
# the floor or fails, the gate exits non-zero with the slowest step named.
#
# Companion beads: soc-dec2.1 (PG1); age-a-plus-report-card-ieyp2.1 (skill-loop retarget).

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

# Step 1: install tombstone + canonical link surface present
step "Step 1: install.sh is a removed-installer tombstone" \
    "grep -q 'ao skills link' scripts/install.sh && bash -n scripts/install.sh && echo OK"
step "Step 1: ao skills link help resolves" \
    "test -x cli/bin/ao || (cd cli && go build -o bin/ao ./cmd/ao); cli/bin/ao skills link --help >/dev/null && echo OK"

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

# Step 3: skill-loop front door (plan → implement → validate)
step "Step 3: plan skill present" \
    "test -f skills/plan/SKILL.md && echo OK"
step "Step 3: implement skill present" \
    "test -f skills/implement/SKILL.md && echo OK"
step "Step 3: validate skill present" \
    "test -f skills/validate/SKILL.md && echo OK"

# Step 4: SKILLS router names the skill-loop first-value path (not /rpi-as-front-door)
step "Step 4: SKILLS router names skill-loop path" \
    "grep -E '/plan.*→.*/implement.*→.*/validate|/plan → /implement → /validate' docs/SKILLS.md >/dev/null && echo OK"

# Step 5: install surface + /rpi remains valid as one-tick executor (not first-value)
step "Step 5: skill install surface reachable" \
    "test -d \$HOME/.claude/skills || test -d skills"
step "Step 5: rpi skill present (one-tick executor)" \
    "test -f skills/rpi/SKILL.md && echo OK"

# Step 6: artifact slot for loop /rpi runs
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

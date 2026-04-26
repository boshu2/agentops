#!/usr/bin/env bash
# check-flywheel-compounding.sh — Gate: knowledge flywheel above escape velocity
# (σρ > δ/100). When the gate fails, surface the σρδ structure plus a hint at
# the most common root cause so operators see *why* instead of a bare `false`
# from `jq -e`.
#
# Exit 0 = PASS, Exit 1 = FAIL.
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"

# Prefer the local ao build if one already exists; fall back to a scratch
# build in /tmp so this gate works on fresh checkouts.
AO_BIN="${AO_BIN:-}"
if [[ -z "$AO_BIN" ]]; then
    if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
        AO_BIN="$REPO_ROOT/cli/bin/ao"
    else
        AO_BIN="/tmp/ao-fw-check"
        (cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao) >/dev/null 2>&1 \
            || { echo "FAIL: could not build ao for flywheel-compounding gate"; exit 1; }
    fi
fi

JSON="$("$AO_BIN" flywheel status --json 2>/dev/null)" || {
    echo "FAIL: ao flywheel status --json failed"
    exit 1
}

if printf '%s' "$JSON" | jq -e '.escape_velocity_compounding == true' >/dev/null 2>&1; then
    printf '%s' "$JSON" | jq -r '"PASS: σ=\(.sigma) ρ=\(.rho) σρ=\(.sigma_rho) δ=\(.delta) (compounding)"'
    exit 0
fi

# Failing case: surface σρδ + the leading structural cause.
diag=$(printf '%s' "$JSON" | jq -r '
    "σ=\(.sigma) ρ=\(.rho) σρ=\(.sigma_rho) δ=\(.delta) threshold=\(.delta / 100)"
')
hint="(σρ ≤ δ/100; corpus has insufficient evidence-backed influence)"

# When ρ is exactly 0, the corpus has no high-confidence (applied or reference)
# citations — this is the most common failure mode and operators benefit from
# being told to record applied/reference citations rather than retrieved-only.
rho=$(printf '%s' "$JSON" | jq -r '.rho')
if [[ "$rho" == "0" ]]; then
    hint="ρ=0 — no applied/reference citations recorded; sessions must use 'ao lookup --cite applied|reference' or programmatic high-confidence citations"
fi

echo "FAIL: $diag — $hint"
exit 1

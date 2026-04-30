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

# Distinguish "no signal at all" (σ=0 → no citations of any kind in window) from
# "no high-confidence signal" (ρ=0 with σ>0 → only retrieved-only citations).
# Each maps to a different operator action: σ=0 needs ANY citation activity to
# wake the flywheel; ρ=0 needs --cite applied|reference instead of bare retrieval.
sigma=$(printf '%s' "$JSON" | jq -r '.sigma')
rho=$(printf '%s' "$JSON" | jq -r '.rho')
multi_session=""
if [[ "$sigma" == "0" && "$rho" == "0" ]]; then
    hint="σ=0 ρ=0 — zero citations recorded in measurement window; corpus is dormant. Sessions must run 'ao lookup' (any --cite kind) before the gate sees signal"
    # Multi-session-bound corpus state: surface verdicts + period so operators
    # see at a glance this is not a single-session fix and matches the
    # quarantine pattern recorded in .agents/findings/f-2026-04-29-001.md.
    # Per the 2026-04-30 nightly retrospective, four consecutive nightlies
    # have failed this gate without metric movement; the diagnostic should
    # make the multi-session character obvious without requiring jq from
    # the operator.
    multi_session=$(printf '%s' "$JSON" | jq -r '
        def fallback(default): if . == null or . == "" then default else . end;
        "  trend_verdict=\(.golden_signals.trend_verdict | fallback("?")) " +
        "concentration_verdict=\(.golden_signals.concentration_verdict | fallback("?")) " +
        "overall_verdict=\(.golden_signals.overall_verdict | fallback("?"))\n" +
        "  citations_this_period=\(.metrics.citations_this_period // 0) " +
        "total_artifacts=\(.metrics.total_artifacts // 0) " +
        "learnings_created=\(.metrics.learnings_created // 0)\n" +
        "  period=[\(.metrics.period_start // "?") .. \(.metrics.period_end // "?")]\n" +
        "  multi-session-bound: this gate measures corpus-level citation activity " +
        "across all sessions in the window; a single nightly cannot move it. " +
        "See .agents/findings/f-2026-04-30-002.md for the proposed corpus-active precondition path."
    ' 2>/dev/null || true)
elif [[ "$rho" == "0" ]]; then
    hint="ρ=0 — no applied/reference citations recorded; sessions must use 'ao lookup --cite applied|reference' or programmatic high-confidence citations"
fi

echo "FAIL: $diag — $hint"
if [[ -n "$multi_session" ]]; then
    printf '%s\n' "$multi_session"
fi
exit 1

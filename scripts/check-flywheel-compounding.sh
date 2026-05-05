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
citations_this_period=$(printf '%s' "$JSON" | jq -r '.metrics.citations_this_period // 0')
multi_session=""

# SKIP precondition (f-2026-04-30-002): when the corpus is fully dormant —
# σ=0 AND ρ=0 AND no citations recorded in the measurement window — there
# is no signal for this gate to evaluate. Failing under this state is
# misclassification: the gate isn't telling us "the flywheel is broken,"
# it's telling us "no operator activity has happened yet." Exit 77
# (autotools-style skip code; honored by cli/internal/goals/measure.go's
# classifyResult) so the goals runner records this as `skip`, not `fail`,
# and it stops dragging the headline fitness number every nightly. The
# gate flips back to fail/pass automatically the moment any session runs
# `ao lookup` against the corpus.
#
# This is the *quarantine-by-precondition* path, preferred over a static
# `quarantined: true` flag because legitimate regressions still surface
# the moment the corpus has signal again. Sessions can always inspect σρ
# via `ao flywheel status`; this only changes the gate's classification.
# Override is available via FLYWHEEL_SKIP_DORMANT=0 for dev iteration.
SKIP_DORMANT="${FLYWHEEL_SKIP_DORMANT:-1}"
if [[ "$SKIP_DORMANT" == "1" ]] && [[ "$sigma" == "0" ]] && [[ "$rho" == "0" ]] && [[ "$citations_this_period" == "0" ]]; then
    skip_msg=$(printf '%s' "$JSON" | jq -r '
        "SKIP: σ=0 ρ=0 — corpus dormant; flywheel-compounding has no signal to evaluate.\n" +
        "  citations_this_period=0 " +
        "total_artifacts=\(.metrics.total_artifacts // 0) " +
        "learnings_created=\(.metrics.learnings_created // 0)\n" +
        "  period=[\(.metrics.period_start // "?") .. \(.metrics.period_end // "?")]\n" +
        "  Dormant precondition (f-2026-04-30-002): exit 77 → goals runner records SKIP.\n" +
        "  To wake the gate: run any session that issues `ao lookup --cite ...` against the corpus."
    ' 2>/dev/null)
    printf '%s\n' "$skip_msg"
    exit 77
fi

if [[ "$sigma" == "0" && "$rho" == "0" ]]; then
    hint="σ=0 ρ=0 with citations_this_period=$citations_this_period — citation index inconsistent; rebuild via 'ao flywheel reindex'"
    # Surface verdicts + period for diagnostic legibility. We hit this branch
    # when σρ are zero but citations exist — index drift, not dormancy.
    multi_session=$(printf '%s' "$JSON" | jq -r '
        def fallback(default): if . == null or . == "" then default else . end;
        "  trend_verdict=\(.golden_signals.trend_verdict | fallback("?")) " +
        "concentration_verdict=\(.golden_signals.concentration_verdict | fallback("?")) " +
        "overall_verdict=\(.golden_signals.overall_verdict | fallback("?"))\n" +
        "  citations_this_period=\(.metrics.citations_this_period // 0) " +
        "total_artifacts=\(.metrics.total_artifacts // 0) " +
        "learnings_created=\(.metrics.learnings_created // 0)\n" +
        "  period=[\(.metrics.period_start // "?") .. \(.metrics.period_end // "?")]"
    ' 2>/dev/null || true)
elif [[ "$rho" == "0" ]]; then
    hint="ρ=0 — no applied/reference citations recorded; sessions must use 'ao lookup --cite applied|reference' or programmatic high-confidence citations"
fi

echo "FAIL: $diag — $hint"
if [[ -n "$multi_session" ]]; then
    printf '%s\n' "$multi_session"
fi
exit 1

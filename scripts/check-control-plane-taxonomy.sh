#!/usr/bin/env bash
# check-control-plane-taxonomy.sh — the membrane on the control-plane docs.
#
# Deterministic doc-invariant gate for the control-plane primitives + adapter
# taxonomy (age-fey, hardening #6 of the 2026-06-17 /discovery --mixed pass).
# Makes the taxonomy MECHANISM, not prose: a confident edit that reintroduces a
# retired store binding or a conflicting agent classification fails here.
#
# Invariants over the loop/primitive docs (docs/architecture/):
#   C1  No live doc may bind the etcd-analog to the RETIRED bd/Dolt store
#       (it is two ledgers now: br git-JSONL + the proof/verdict ledger).
#       Lines that DESCRIBE the retirement (retired/superseded/corrected/…) are
#       not offenders.
#   C2  The agent is classified as the data-plane workload/actuator in
#       the-agent-factory.md AND as a driving adapter in ports-and-adapters.md.
#       That coexistence is legal ONLY with the two-altitude reconciliation note
#       present in BOTH docs. Missing from either → FAIL.
#   C3  the-agent-factory.md carries the adapter-taxonomy section and is the
#       unifying entry: it cross-links control-loop-model / primitive-chains /
#       canonical-loop-model / loop-map / ports-and-adapters, and each of those
#       links back to it. All cross-link targets must exist on disk.
#
# Override the doc dir for testing: AGENTOPS_ARCH_DIR=<dir>
# Exit codes: 0 = clean, 1 = invariant violated, 2 = usage/setup error.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
ARCH="${AGENTOPS_ARCH_DIR:-$repo_root/docs/architecture}"

AF="$ARCH/the-agent-factory.md"
PA="$ARCH/ports-and-adapters.md"
CLM="$ARCH/control-loop-model.md"
PC="$ARCH/primitive-chains.md"
CAN="$ARCH/canonical-loop-model.md"
LM="$ARCH/loop-map.md"

# Lines that describe the retirement rather than prescribe the retired tool.
REMOVAL_LANG='[Rr]etired|[Ss]uperseded|[Cc]orrected|[Rr]emoved|[Dd]eprecat|no longer|originally named'

failures=0
fail() {
    echo "CONTROL_PLANE_TAXONOMY: FAIL: $*" >&2
    failures=$((failures + 1))
}

# --- setup check ---
for f in "$AF" "$PA" "$CLM" "$PC" "$CAN" "$LM"; do
    [[ -f "$f" ]] || { echo "CONTROL_PLANE_TAXONOMY: setup error: missing $f" >&2; exit 2; }
done

# --- C1: no live doc binds the etcd-analog to retired bd/Dolt ---
# A line is an offender iff it mentions bd/Dolt AND ties it to the store/etcd AND
# is not describing the retirement.
for f in "$AF" "$PA" "$CLM" "$PC" "$CAN" "$LM"; do
    if hits=$(grep -nE 'bd/?Dolt' "$f" 2>/dev/null); then
        while IFS= read -r line; do
            printf '%s' "$line" | grep -qE 'etcd|[Ss]tate store' || continue
            printf '%s' "$line" | grep -qE "$REMOVAL_LANG" && continue
            fail "$f reintroduces retired bd/Dolt store binding: ${line}"
        done <<< "$hits"
    fi
done

# --- C2: agent two-altitude reconciliation present in both classifying docs ---
# the-agent-factory classifies the agent as actuator/data-plane workload.
if grep -qiE 'agent.*(actuator|data.plane workload|AgentPod)' "$AF"; then
    grep -qiE 'two[ -]altitude' "$AF" \
        || fail "$AF classifies the agent as data-plane workload but lacks the two-altitude reconciliation note"
    grep -qiE 'two[ -]altitude' "$PA" \
        || fail "$PA classifies the agent as a driving adapter but lacks the two-altitude reconciliation note (forward-link to the-agent-factory)"
fi

# --- C3: adapter taxonomy present + bidirectional cross-links resolve ---
grep -qiE 'adapter taxonomy' "$AF" \
    || fail "$AF is missing the adapter-taxonomy section (the unifying entry)"

# the-agent-factory must cross-link these siblings; each must link back; targets must exist.
declare -A back=(
    [control-loop-model.md]="$CLM"
    [primitive-chains.md]="$PC"
    [canonical-loop-model.md]="$CAN"
    [loop-map.md]="$LM"
    [ports-and-adapters.md]="$PA"
)
for name in "${!back[@]}"; do
    target="${back[$name]}"
    [[ -f "$target" ]] || { fail "cross-link target missing on disk: $name"; continue; }
    grep -q "$name" "$AF" || fail "$AF does not cross-link $name (the unifying entry must point at it)"
    grep -q "the-agent-factory.md" "$target" \
        || fail "$name does not link back to the-agent-factory.md (cross-link must be bidirectional)"
done

if [[ "$failures" -gt 0 ]]; then
    echo "CONTROL_PLANE_TAXONOMY: $failures invariant violation(s) — see above." >&2
    exit 1
fi

echo "CONTROL_PLANE_TAXONOMY: PASS (store binding current; agent two-altitude reconciled; taxonomy cross-links resolve)"
exit 0

#!/usr/bin/env bash
# emit-landed-provenance.sh — milestone-1 SENSOR feed (ag-62jrm).
#
# Wires the provenance emitter to the land path: for the commits about to land
# on main (origin/main..HEAD), append a bead --wasGeneratedBy--> commit edge per
# cited bead, then commit the ledger growth as a trailing chore commit so the
# sensor is fed WITHOUT a human in the loop. Self-terminating: the trailing
# provenance commit cites no bead, so its own future emission is a no-op.
#
# Design constraints (honest, low-blast-radius):
#   - NON-BLOCKING: every failure warns and exits 0. The sensor must never block
#     a push (slice-0 graceful-degrade, same posture as the spawn gateway).
#   - SKIPPABLE: AGENTOPS_PROVENANCE_EMIT_SKIP=1 disables it.
#   - IDEMPOTENT: the emitter dedups; re-runs add nothing.
#   - ONE-PUSH LAG (documented): the ledger commit is created after the current
#     push's refs are computed, so it lands on the next push. Acceptable for a
#     position signal that continuously catches up.
set -uo pipefail

if [[ "${AGENTOPS_PROVENANCE_EMIT_SKIP:-0}" == "1" ]]; then
    exit 0
fi

warn() { echo "provenance-emit: $*" >&2; }

# Resolve repo root (the script may be called from anywhere in the tree).
ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { warn "not in a git repo; skipping"; exit 0; }
cd "$ROOT" || exit 0

# Resolve the ao binary: prefer an explicit AO_BIN, then the conventional build
# output. If neither exists we skip (the gate builds ao elsewhere; we don't add
# a second build cost here).
AO="${AO_BIN:-}"
if [[ -z "$AO" && -x "cli/bin/ao" ]]; then
    AO="$PWD/cli/bin/ao"
fi
if [[ -z "$AO" || ! -x "$AO" ]]; then
    warn "ao binary not found (set AO_BIN or build cli/bin/ao); skipping emit"
    exit 0
fi

# Determine the landing range. Prefer origin/main..HEAD; fall back to the last
# commit when there is no origin/main ref (fresh clone / detached state).
RANGE=""
if git rev-parse --verify --quiet origin/main >/dev/null; then
    RANGE="origin/main..HEAD"
elif git rev-parse --verify --quiet "HEAD~1" >/dev/null; then
    RANGE="HEAD~1..HEAD"
else
    warn "no landing range resolvable; skipping"
    exit 0
fi

# Nothing to do when the range is empty (already up to date).
if [[ -z "$(git rev-list "$RANGE" 2>/dev/null)" ]]; then
    exit 0
fi

LEDGER="docs/provenance/ledger.jsonl"
before="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"

if ! "$AO" provenance emit-landed --range "$RANGE" >/dev/null 2>&1; then
    warn "emit-landed failed for $RANGE; skipping (non-blocking)"
    exit 0
fi

after="$(git hash-object "$LEDGER" 2>/dev/null || echo none)"
if [[ "$before" == "$after" ]]; then
    # No new edges (e.g. trivial/chore commits cite no bead). Nothing to commit.
    exit 0
fi

# Commit ONLY the ledger growth as a trailing sensor commit. It cites no bead,
# so it never triggers further emission (natural termination).
if ! git add "$LEDGER" 2>/dev/null; then
    warn "could not stage $LEDGER; left as working-tree change"
    exit 0
fi
if git commit -m "chore(provenance): auto-emit landed bead→commit edges (ag-62jrm sensor)" >/dev/null 2>&1; then
    echo "provenance-emit: appended landed edges for $RANGE (lands next push)"
else
    warn "ledger updated but commit failed; left staged"
fi
exit 0

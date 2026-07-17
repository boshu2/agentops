#!/usr/bin/env bash
# check-provenance-chain.sh — provenance.chain local gate (age-gate-the-ungated-egwt.9).
#
# Verify the committed provenance ledger's hash chain AT THE PRE-PUSH AUTHORITY
# BOUNDARY. `ao provenance verify` already exists and is CI-gated, but CI is a
# rare backstop; the LOCAL pre-push gate (the release authority) only EMITTED
# provenance edges before this gate — it never verified the chain it was about
# to push. This moves tamper-evidence to the authority boundary: a tampered
# field, a forged hash, or a reordered row in docs/provenance/ledger.jsonl fails
# the push loudly, naming the first broken FILE LINE.
#
# WHAT IT RUNS: `ao provenance verify [--json]`. That reads
# docs/provenance/ledger.jsonl EXACTLY as it sits on disk (in place — no
# re-sort, no re-chain, unlike `provenance export --verify`) and checks each
# record's prev_hash/payload_hash/hash. Exit 0 = intact (or absent/empty
# ledger, which is an intact empty chain for a fresh clone); non-zero names the
# first bad line. This is a linear JSONL scan — under ~2s beyond the one-time
# ao-binary resolution that sibling gates already pay.
#
# EMIT-vs-VERIFY ORDERING (age-gate-the-ungated-egwt.9): the pre-push sequence's
# provenance FEEDER — scripts/post-land-provenance-emit.sh — runs AFTER a
# successful push, from a DISPOSABLE worktree off origin/main, and pushes its
# ledger growth as its OWN separate `#trivial` commit. It NEVER appends to the
# pushing checkout's working tree during the gate run. Therefore:
#   * THIS check reads the ledger as `ao provenance verify` resolves it (the
#     repo-root working tree = the committed HEAD state a clean push carries),
#     so it verifies the exact bytes being pushed.
#   * A fresh emit-append CANNOT make this check fail spuriously: the emitter's
#     re-emit-on-race path (see post-land-provenance-emit.sh header) re-chains
#     onto the CURRENT trunk tail in its own worktree and pushes a subsequent
#     commit; that commit's OWN push runs this gate again on ITS committed
#     ledger. No append lands in the pushing tree between change-detection and
#     this check within a single push.
# We deliberately do NOT diff HEAD's ledger against the worktree copy: the gate
# convention (mirrored from scripts/check-provenance-orphans.sh) is to consume
# `ao provenance verify` in place; `ao` reads the repo-root ledger, which for a
# clean push IS HEAD's ledger. A left-over uncommitted append would already be
# rejected by the commit-scope/pawl gates before reaching a push.
#
# REPAIR: a break is NEVER a hand-edit. Find the first bad entry with
# `ao provenance verify`; the fix is a deliberate re-seal (a coordinator
# decision), never an in-place patch of the ledger bytes.
#
# Bounded-context: BC4-Factory. Evidence: pre-push gate + .github/workflows/validate.yml.
#
# Exit 0 = healthy chain (or absent/empty). Exit 1 = broken chain (line named).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

# Resolve the ao binary the way sibling gate scripts do (mirror
# check-provenance-orphans.sh / check-flywheel-compounding.sh): prefer an
# explicit AO_BIN, then the pre-built CI binary, else build a scratch binary so
# the gate works on a fresh checkout. No `-tags` are needed — `ao provenance`
# is a spine command that builds with plain `go build ./cmd/ao`.
AO="${AO_BIN:-}"
if [[ -z "$AO" ]]; then
    if [[ -x "$ROOT/cli/bin/ao" ]]; then
        AO="$ROOT/cli/bin/ao"
    elif command -v ao >/dev/null 2>&1; then
        AO="ao"
    else
        scratch="$(mktemp -d "${TMPDIR:-/tmp}/ao-provenance-chain-check.XXXXXX")"
        trap 'rm -rf "$scratch"' EXIT
        AO="$scratch/ao"
        (cd "$ROOT/cli" && go build -o "$AO" ./cmd/ao) >/dev/null 2>&1 \
            || { echo "PROVENANCE_CHAIN_GATE: FAIL — could not build ao for the chain gate" >&2; exit 1; }
    fi
fi

echo "=== provenance.chain gate (ao provenance verify) ==="

# Run from the repo root so `ao` resolves the committed ledger via its
# repo-root discovery (resolveLedgerPath walks up to docs+schemas / .git).
set +e
out="$(cd "$ROOT" && "$AO" provenance verify 2>&1)"
rc=$?
set -e

echo "$out"

if [[ $rc -ne 0 ]]; then
    echo "PROVENANCE_CHAIN_GATE: FAIL — docs/provenance/ledger.jsonl hash chain is broken." >&2
    echo "  Find the first bad entry with 'ao provenance verify'." >&2
    echo "  Repair is a DELIBERATE re-seal (coordinator decision) — never hand-edit the ledger." >&2
    exit 1
fi

echo "PROVENANCE_CHAIN_GATE: OK (committed ledger chain intact)"

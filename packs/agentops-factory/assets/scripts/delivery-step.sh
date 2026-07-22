#!/usr/bin/env bash
set -euo pipefail

# The Order owns only the periodic trigger. The separately pinned reducer owns
# one transition; it never receives an event wake and never runs a loop here.
: "${AGENTOPS_GC_DELIVERY_BIN:?absolute pack-selected reducer binary required}"
: "${GC_BIN:?deployment-pinned Gas City binary required}"
: "${AGENTOPS_GC_DELIVERY_ROOT:?rig-scoped evidence root required}"
: "${AGENTOPS_GC_DELIVERY_CERTIFICATE:?exact certificate path required}"
: "${AGENTOPS_GC_DELIVERY_SEMANTIC_BEAD:?semantic bead required}"
: "${AGENTOPS_GC_DELIVERY_TERMINAL_REF:?terminal ref required}"
: "${AGENTOPS_GC_DELIVERY_RIG:?rig identity required}"
: "${AGENTOPS_GC_DELIVERY_REPOSITORY:?repository identity required}"
: "${AGENTOPS_GC_DELIVERY_REMOTE:?remote identity required}"
: "${AGENTOPS_GC_DELIVERY_EPOCH:?epoch required}"
: "${AGENTOPS_GC_DELIVERY_MODE:?mode required}"
: "${AGENTOPS_GC_DELIVERY_DEADLINE:?deadline required}"
: "${AGENTOPS_GC_DELIVERY_PREPARED_AT:?prepared timestamp required}"
: "${AGENTOPS_GC_DELIVERY_COMMITTED_AT:?committed timestamp required}"
: "${AGENTOPS_GC_DELIVERY_BASE_REF:?base ref required}"
: "${AGENTOPS_GC_DELIVERY_BASE_OID:?base oid required}"
: "${AGENTOPS_GC_DELIVERY_FAKE_TERMINAL_REF:?GC33-6 fake boundary terminal ref required}"
[[ "$AGENTOPS_GC_DELIVERY_BIN" = /* && -x "$AGENTOPS_GC_DELIVERY_BIN" ]] || { echo "AGENTOPS_GC_DELIVERY_BIN must be an absolute executable" >&2; exit 2; }
[[ "$GC_BIN" = /* && -x "$GC_BIN" ]] || { echo "GC_BIN must be an absolute executable" >&2; exit 2; }
: "${AGENTOPS_GC_DELIVERY_FIXTURE_STATE:?explicit offline fixture state required in GC33-6}"

exec "$AGENTOPS_GC_DELIVERY_BIN" step \
  --root "$AGENTOPS_GC_DELIVERY_ROOT" --certificate "$AGENTOPS_GC_DELIVERY_CERTIFICATE" \
  --semantic-bead "$AGENTOPS_GC_DELIVERY_SEMANTIC_BEAD" --terminal-ref "$AGENTOPS_GC_DELIVERY_TERMINAL_REF" \
  --rig "$AGENTOPS_GC_DELIVERY_RIG" --repository "$AGENTOPS_GC_DELIVERY_REPOSITORY" --remote "$AGENTOPS_GC_DELIVERY_REMOTE" \
  --epoch "$AGENTOPS_GC_DELIVERY_EPOCH" --mode "$AGENTOPS_GC_DELIVERY_MODE" --deadline "$AGENTOPS_GC_DELIVERY_DEADLINE" \
  --prepared-at "$AGENTOPS_GC_DELIVERY_PREPARED_AT" --committed-at "$AGENTOPS_GC_DELIVERY_COMMITTED_AT" \
  --base-ref "$AGENTOPS_GC_DELIVERY_BASE_REF" --base-oid "$AGENTOPS_GC_DELIVERY_BASE_OID" \
  --fake-terminal-ref "$AGENTOPS_GC_DELIVERY_FAKE_TERMINAL_REF" --fixture-state "$AGENTOPS_GC_DELIVERY_FIXTURE_STATE"

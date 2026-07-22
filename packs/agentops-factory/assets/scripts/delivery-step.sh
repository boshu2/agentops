#!/usr/bin/env bash
set -euo pipefail

# The Order owns only the periodic trigger. The separately pinned reducer owns
# one transition; it never receives an event wake and never runs a loop here.
: "${AGENTOPS_GC_DELIVERY_BIN:?absolute pack-selected reducer binary required}"
: "${GC_BIN:?deployment-pinned Gas City binary required}"
: "${AGENTOPS_GC_DELIVERY_ROOT:?rig-scoped evidence root required}"
: "${AGENTOPS_GC_DELIVERY_CERTIFICATE:?exact certificate path required}"
: "${AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST:?subject-manifest.v1 path required}"
: "${AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST_DIGEST:?canonical subject manifest digest required}"
: "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT:?native delivery context path required}"
: "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST:?exact native context digest required}"
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
: "${AGENTOPS_GC_BEADS_BIN:?deployment-pinned Beads binary required}"
: "${AGENTOPS_GC_GIT_BIN:?pinned Git binary required}"
: "${AGENTOPS_GC_GH_BIN:?pinned GitHub CLI binary required}"
[[ "$AGENTOPS_GC_DELIVERY_BIN" = /* && -x "$AGENTOPS_GC_DELIVERY_BIN" ]] || { echo "AGENTOPS_GC_DELIVERY_BIN must be an absolute executable" >&2; exit 2; }
[[ "$GC_BIN" = /* && -x "$GC_BIN" ]] || { echo "GC_BIN must be an absolute executable" >&2; exit 2; }
[[ "$AGENTOPS_GC_BEADS_BIN" = /* && -x "$AGENTOPS_GC_BEADS_BIN" ]] || { echo "AGENTOPS_GC_BEADS_BIN must be an absolute executable" >&2; exit 2; }
[[ "$AGENTOPS_GC_GIT_BIN" = /* && -x "$AGENTOPS_GC_GIT_BIN" ]] || { echo "AGENTOPS_GC_GIT_BIN must be an absolute executable" >&2; exit 2; }
[[ "$AGENTOPS_GC_GH_BIN" = /* && -x "$AGENTOPS_GC_GH_BIN" ]] || { echo "AGENTOPS_GC_GH_BIN must be an absolute executable" >&2; exit 2; }
[[ "$AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST" = /* && -f "$AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST" && "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT" = /* && -f "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT" ]] || { echo "delivery subject manifest and native context must be absolute files" >&2; exit 2; }
[[ "$AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST_DIGEST" =~ ^[0-9a-f]{64}$ && "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST" =~ ^[0-9a-f]{64}$ ]] || { echo "delivery manifest/context digests must be lowercase sha256" >&2; exit 2; }

exec "$AGENTOPS_GC_DELIVERY_BIN" step \
  --root "$AGENTOPS_GC_DELIVERY_ROOT" --certificate "$AGENTOPS_GC_DELIVERY_CERTIFICATE" \
  --subject-manifest "$AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST" --subject-manifest-digest "$AGENTOPS_GC_DELIVERY_SUBJECT_MANIFEST_DIGEST" \
  --native-context "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT" --native-context-digest "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST" \
  --semantic-bead "$AGENTOPS_GC_DELIVERY_SEMANTIC_BEAD" --terminal-ref "$AGENTOPS_GC_DELIVERY_TERMINAL_REF" \
  --rig "$AGENTOPS_GC_DELIVERY_RIG" --repository "$AGENTOPS_GC_DELIVERY_REPOSITORY" --remote "$AGENTOPS_GC_DELIVERY_REMOTE" \
  --epoch "$AGENTOPS_GC_DELIVERY_EPOCH" --mode "$AGENTOPS_GC_DELIVERY_MODE" --deadline "$AGENTOPS_GC_DELIVERY_DEADLINE" \
  --prepared-at "$AGENTOPS_GC_DELIVERY_PREPARED_AT" --committed-at "$AGENTOPS_GC_DELIVERY_COMMITTED_AT" \
  --base-ref "$AGENTOPS_GC_DELIVERY_BASE_REF" --base-oid "$AGENTOPS_GC_DELIVERY_BASE_OID" \
  --gc-bin "$GC_BIN" --beads-bin "$AGENTOPS_GC_BEADS_BIN" --git-bin "$AGENTOPS_GC_GIT_BIN" --gh-bin "$AGENTOPS_GC_GH_BIN"

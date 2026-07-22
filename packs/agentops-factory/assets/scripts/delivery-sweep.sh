#!/usr/bin/env bash
set -euo pipefail

# Controller-owned rig configuration names the fixed evidence namespace and
# pinned tools.  No delivery bead supplies environment or executable paths.
# Per-delivery --subject-manifest, certificate, terminal, and target facts are
# resolved only from the selected Bead's immutable request reference.
: "${AGENTOPS_GC_DELIVERY_BIN:?absolute pack-selected reducer binary required}"
: "${GC_BIN:?deployment-pinned Gas City binary required}"
: "${AGENTOPS_GC_BEADS_BIN:?deployment-pinned Beads binary required}"
: "${AGENTOPS_GC_GIT_BIN:?pinned Git binary required}"
: "${AGENTOPS_GC_GH_BIN:?pinned GitHub CLI required}"
: "${AGENTOPS_GC_BASH_BIN:?pinned Bash binary required}"
: "${AGENTOPS_GC_DELIVERY_ROOT:?rig-scoped evidence root required}"
: "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT:?native context path required}"
: "${AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST:?native context digest required}"
: "${AGENTOPS_GC_DELIVERY_RIG:?rig required}"
: "${AGENTOPS_GC_DELIVERY_REPOSITORY:?repository required}"
: "${AGENTOPS_GC_DELIVERY_REMOTE:?remote required}"
for binary in "$AGENTOPS_GC_DELIVERY_BIN" "$GC_BIN" "$AGENTOPS_GC_BEADS_BIN" "$AGENTOPS_GC_GIT_BIN" "$AGENTOPS_GC_GH_BIN" "$AGENTOPS_GC_BASH_BIN"; do
  [[ "$binary" = /* && -x "$binary" ]] || { echo "delivery sweep binary must be absolute and executable" >&2; exit 2; }
done
exec "$AGENTOPS_GC_DELIVERY_BIN" sweep \
  --root "$AGENTOPS_GC_DELIVERY_ROOT" \
  --native-context "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT" --native-context-digest "$AGENTOPS_GC_DELIVERY_NATIVE_CONTEXT_DIGEST" \
  --rig "$AGENTOPS_GC_DELIVERY_RIG" --repository "$AGENTOPS_GC_DELIVERY_REPOSITORY" --remote "$AGENTOPS_GC_DELIVERY_REMOTE" \
  --gc-bin "$GC_BIN" --beads-bin "$AGENTOPS_GC_BEADS_BIN" --git-bin "$AGENTOPS_GC_GIT_BIN" --gh-bin "$AGENTOPS_GC_GH_BIN" --bash-bin "$AGENTOPS_GC_BASH_BIN"

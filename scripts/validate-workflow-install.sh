#!/usr/bin/env bash
# validate-workflow-install.sh — the named blocking parent for workflow install
# freshness (ag-wi9w1; gate check ID workflow.install-drift).
#
# Thin parent: run check-workflow-drift.sh, then check-bdd-foundry-markers.sh
# (argless — the repo canonical). Exit non-zero if either fails, propagating
# their output. Clean machines stay green via the drift check's absent=>SKIP.
set -uo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"

status=0
bash "$repo_root/scripts/check-workflow-drift.sh" || status=1
bash "$repo_root/scripts/check-bdd-foundry-markers.sh" || status=1
exit "$status"

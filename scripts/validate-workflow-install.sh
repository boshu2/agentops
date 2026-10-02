#!/usr/bin/env bash
# validate-workflow-install.sh — the named blocking parent for workflow install
# freshness (ag-wi9w1; gate check ID workflow.install-drift).
#
# Thin parent: check installed copies, then ensure retired workflow names fail
# before dispatch. Exit non-zero if either check fails. Clean machines stay
# green via the drift check's absent=>SKIP.
set -uo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"

status=0
bash "$repo_root/scripts/check-workflow-drift.sh" || status=1
bash "$repo_root/scripts/check-workflow-tombstones.sh" || status=1
exit "$status"

#!/usr/bin/env bash
# Compatibility entry point for the retired workflow check. The former
# bdd-foundry implementation markers no longer describe a live workflow.
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
exec bash "$repo_root/scripts/check-workflow-tombstones.sh" "${1:-$repo_root/workflows}"

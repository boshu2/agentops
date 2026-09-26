#!/usr/bin/env bash
# install-policy-dispatch.sh — repo-root entry point for wiring the AgentOps
# policy dispatcher (age-bhsz, epic age-4qw1). The implementation lives INSIDE
# the native guard package (hooks/guards/scripts/install-hooks.sh); this script
# delegates to it for source checkouts, where users expect scripts/install-*.
#
# Claude Code PLUGIN installs need neither: the plugin bundles hooks/hooks.json
# and Claude wires it automatically.
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
exec bash "${repo_root}/hooks/guards/scripts/install-hooks.sh" "$@"

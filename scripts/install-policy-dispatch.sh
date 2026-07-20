#!/usr/bin/env bash
# install-policy-dispatch.sh — repo-root entry point for wiring the AgentOps
# policy dispatcher (age-bhsz, epic age-4qw1). The implementation lives INSIDE
# the cc-hooks skill package (skills/cc-hooks/scripts/install-hooks.sh) so that
# skills-copy installs (npx skills / skills.sh) carry their own wiring; this
# script just delegates for clone/brew users who expect scripts/install-*.
#
# Claude Code PLUGIN installs need neither: the plugin bundles hooks/hooks.json
# and Claude wires it automatically.
set -euo pipefail

# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"
exec bash "${repo_root}/skills/cc-hooks/scripts/install-hooks.sh" "$@"

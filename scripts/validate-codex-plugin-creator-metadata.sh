#!/usr/bin/env bash
# Validate AgentOps Codex plugin metadata against plugin-creator marketplace expectations.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
    cat <<'EOF'
Usage: bash scripts/validate-codex-plugin-creator-metadata.sh [--repo-root <path>]

Checks the AgentOps Codex marketplace and plugin manifest for the stricter
plugin-creator metadata fields used by Codex plugin discovery surfaces.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo-root)
            if [[ $# -lt 2 ]]; then
                echo "error: --repo-root requires a value" >&2
                usage >&2
                exit 2
            fi
            REPO_ROOT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "error: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"
MARKETPLACE_FILE="$REPO_ROOT/plugins/marketplace.json"
PLUGIN_MANIFEST="$REPO_ROOT/.codex-plugin/plugin.json"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

require_file() {
    [[ -f "$1" ]] || fail "missing file: $1"
}

command -v jq >/dev/null 2>&1 || fail "jq is required"
require_file "$MARKETPLACE_FILE"
require_file "$PLUGIN_MANIFEST"

jq -e '
  .name == "agentops-marketplace"
  and .interface.displayName == "AgentOps"
  and (.plugins | type == "array" and length > 0)
  and all(.plugins[];
    (.policy.installation as $installation
      | ["NOT_AVAILABLE", "AVAILABLE", "INSTALLED_BY_DEFAULT"] | index($installation) != null)
    and (.policy.authentication as $authentication
      | ["ON_INSTALL", "ON_USE"] | index($authentication) != null)
    and (.category | type == "string" and length > 0)
  )
  and any(.plugins[];
    .name == "agentops"
    and .source.source == "local"
    and .source.path == "./"
    and .policy.installation == "AVAILABLE"
    and .policy.authentication == "ON_INSTALL"
    and ((.policy | has("products")) | not)
    and .category == "Productivity"
  )
' "$MARKETPLACE_FILE" >/dev/null || fail "plugins/marketplace.json is missing required plugin-creator metadata"

jq -e '
  .name == "agentops"
  and .skills == "./skills-codex"
  and .interface.displayName == "AgentOps"
  and .interface.shortDescription == "Repo-native memory, validation gates, and agent workflows."
  and (.interface.longDescription | type == "string" and length > 0)
  and .interface.developerName == "AgentOps"
  and .interface.category == "Productivity"
  and (.interface.capabilities as $capabilities
    | ($capabilities | type == "array" and length > 0)
    and (["Skills", "Hooks"] | all(. as $capability | ($capabilities | index($capability) != null))))
  and (.interface.defaultPrompt | type == "array" and length > 0 and length <= 3)
  and all(.interface.defaultPrompt[]; type == "string" and length > 0 and length <= 128)
' "$PLUGIN_MANIFEST" >/dev/null || fail ".codex-plugin/plugin.json is missing required plugin-creator interface metadata"

echo "Codex plugin-creator metadata validation OK."

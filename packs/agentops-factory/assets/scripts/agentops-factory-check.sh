#!/usr/bin/env bash
set -euo pipefail
# Gas City v1.3.5 exposes the checked attempt's molecule artifact directory
# through GC_ARTIFACT_DIR. The agent writes the exact role request/result
# there; this wrapper never guesses a bead or scans another workflow.
: "${GC_ARTIFACT_DIR:?Gas City check artifact directory required}"
script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)"
feeder="$script_dir/agentops-factory-feeder"
[[ "$feeder" = /* && -x "$feeder" && ! -L "$feeder" ]] || { echo "projected factory feeder must be an executable regular sibling" >&2; exit 2; }
request="$GC_ARTIFACT_DIR/agentops-factory-check-request.json"
[[ -f "$request" && ! -L "$request" ]] || { echo "factory check request is missing from the checked attempt artifact directory" >&2; exit 2; }
exec "$feeder" check --request "$request"

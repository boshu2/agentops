#!/usr/bin/env bash
# Convenience wrapper for the ordinary deterministic repository checks.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

scope="worktree"
mode="--fast"

usage() {
    cat <<'EOF'
Usage: ./scripts/validate-local.sh [--scope head|staged|worktree|upstream|range:<base>..<head>] [--full]

Runs ao gate check as a deterministic test command. It installs no hook,
serializes no caller, invokes no model runtime, and conveys no semantic verdict.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --scope)
            scope="${2:-}"
            [[ -n "$scope" ]] || { usage >&2; exit 2; }
            shift 2
            ;;
        --full)
            mode="--full"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

ao_bin="${AO_BIN:-}"
[[ -z "$ao_bin" && -x "$REPO_ROOT/cli/bin/ao" ]] && ao_bin="$REPO_ROOT/cli/bin/ao"
[[ -z "$ao_bin" ]] && ao_bin="$(command -v ao 2>/dev/null || true)"
[[ -n "$ao_bin" ]] || { echo "ao is not available; build cli/bin/ao or set AO_BIN" >&2; exit 1; }

cd "$REPO_ROOT"
exec "$ao_bin" gate check "$mode" --scope "$scope"

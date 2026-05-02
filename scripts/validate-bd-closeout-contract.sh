#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

failures=0

fail() {
    echo "BD_CLOSEOUT_CONTRACT: FAIL: $*" >&2
    failures=$((failures + 1))
}

require_file() {
    local path="$1"
    if [[ ! -f "$repo_root/$path" ]]; then
        fail "missing required file: $path"
        return 1
    fi
    return 0
}

require_conditional_push_wording() {
    local path="$1"
    local bare_push_matches

    require_file "$path" || return 0

    if ! grep -Eq 'bd dolt push.*(only if|only when|remote is configured|remote configured)' "$repo_root/$path"; then
        fail "$path must describe bd dolt push as conditional on a configured remote"
    fi

    bare_push_matches="$(
        grep -nE '^[[:space:]]*bd dolt push[[:space:]]*$' "$repo_root/$path" || true
    )"
    if [[ -n "$bare_push_matches" ]]; then
        fail "$path contains a bare bd dolt push command without the conditional remote note: ${bare_push_matches//$'\n'/; }"
    fi
}

require_conditional_push_wording "AGENTS.md"
require_conditional_push_wording "cli/AGENTS.md"

if require_file "docs/runbooks/bd-server-mode-closeout.md"; then
    grep -q 'Server-mode direct-write' "$repo_root/docs/runbooks/bd-server-mode-closeout.md" \
        || fail "runbook must name server-mode direct-write trackers"
    grep -q 'bd dolt remote list' "$repo_root/docs/runbooks/bd-server-mode-closeout.md" \
        || fail "runbook must include bd dolt remote list"
    grep -q 'no remote is configured' "$repo_root/docs/runbooks/bd-server-mode-closeout.md" \
        || fail "runbook must explain the no-remote case"
fi

if require_file "docs/documentation-index.md"; then
    grep -q 'runbooks/bd-server-mode-closeout.md' "$repo_root/docs/documentation-index.md" \
        || fail "documentation index must link bd server-mode closeout runbook"
fi

if (( failures > 0 )); then
    exit 1
fi

echo "BD_CLOSEOUT_CONTRACT: PASS"

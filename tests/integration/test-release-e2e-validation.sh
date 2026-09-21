#!/usr/bin/env bash
# Integration test for successful current ci-local fast release checks.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

verify_release_output() {
    local output_file="$1" marker failed=0
    for marker in "Codex runtime sections" "Codex artifact metadata" \
        "Install surface smoke" "ao init + live-waist smoke"; do
        # The check's success line is required; a section header is not proof.
        if sed -E $'s/\033\\[[0-9;]*m//g' "$output_file" | grep -Fx "  ✓ $marker" >/dev/null; then
            echo "PASS: completed $marker"
        else
            echo "FAIL: missing successful completion of $marker" >&2
            failed=1
        fi
    done
    return "$failed"
}

main() {
    local log_root="${AGENTOPS_TEST_LOG_DIR:-${TMPDIR:-/tmp}}"
    mkdir -p "$log_root"
    local artifacts status
    artifacts="$(mktemp -d "$log_root/release-e2e.XXXXXX")"
    mkdir "$artifacts/agents-hub"
    echo "Release E2E fast gate; retained diagnostics: $artifacts"
    if (cd "$REPO_ROOT" && AGENTS_HUB_OVERRIDE="$artifacts/agents-hub" \
        bash scripts/ci-local-release.sh --fast --jobs 4) > "$artifacts/release.log" 2>&1; then
        status=0
    else
        status=$?
    fi
    printf '%s\n' "$status" > "$artifacts/command.status"
    if [[ "$status" -ne 0 ]]; then
        echo "FAIL: ci-local fast mode exited $status; full log: $artifacts/release.log" >&2
        tail -40 "$artifacts/release.log" >&2
        return "$status"
    fi
    verify_release_output "$artifacts/release.log"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi

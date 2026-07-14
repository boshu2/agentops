#!/usr/bin/env bats

# Regression guard (ag-c2i): AgentOps 3.0 removed hooks and deleted
# scripts/validate-hooks-doc-parity.sh. Operational docs must not instruct a
# reader to run the deleted script. Historical records (docs/CHANGELOG.md and
# docs/releases/*) legitimately describe the removal and are excluded. The
# retired scripts/pre-push-gate.sh and its bats stub keep their references until
# the soc-g2r9 deletion waves remove those files wholesale, so they are out of
# this guard's scope (see docs/contracts/local-pre-push-gate-retirement.md).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "operational docs do not reference deleted validate-hooks-doc-parity.sh" {
    cd "$REPO_ROOT"
    run grep -rn --include='*.md' 'validate-hooks-doc-parity' docs
    filtered="$(printf '%s\n' "$output" | grep -Ev '^docs/(CHANGELOG\.md|releases/)' | grep -v '^$' || true)"
    if [ -n "$filtered" ]; then
        echo "Dangling references to deleted scripts/validate-hooks-doc-parity.sh in operational docs:"
        printf '%s\n' "$filtered"
        return 1
    fi
}

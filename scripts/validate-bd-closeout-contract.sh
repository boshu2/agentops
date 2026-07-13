#!/usr/bin/env bash
# validate-bd-closeout-contract.sh — br closeout doc-contract gate (pre-push check 19b).
#
# Inverted 2026-06-11 (ag-joto6): the tracker migrated bd/Dolt -> br (beads_rust,
# workspace _beads/). This gate now asserts the NEW br closeout contract in
# docs/agent-workflow-reference.md:
#   1. flush discipline is documented:  br sync --flush-only
#   2. ledger staging is documented through the canonical resolver:
#                                      git -C "$(ao beads dir)" add/commit/push
#   3. no live bd/Dolt closeout instructions remain (bd dolt push / bd dolt commit)
#
# Retired with the migration: the bd-era conditional `bd dolt push` wording checks
# (the canonical workflow reference + cli/AGENTS.md) and the bd-server-mode-closeout runbook
# anchors. cli/AGENTS.md still carries bd-era text pending its own doc flip
# (wave 3) and is deliberately NOT asserted here.
#
# Self-test hook: set CLOSEOUT_CONTRACT_WORKFLOW_DOC to a file path to validate
# an alternate document (used by tests to prove the gate fails on bd-era docs
# without reverting the real ones).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

workflow_doc="${CLOSEOUT_CONTRACT_WORKFLOW_DOC:-$repo_root/docs/agent-workflow-reference.md}"

failures=0

fail() {
    echo "BR_CLOSEOUT_CONTRACT: FAIL: $*" >&2
    failures=$((failures + 1))
}

if [[ ! -f "$workflow_doc" ]]; then
    fail "missing required file: $workflow_doc"
else
    if ! grep -Fq 'br sync --flush-only' "$workflow_doc"; then
        fail "$workflow_doc must document the br flush discipline (br sync --flush-only)"
    fi

    if ! grep -Fq 'ao beads dir' "$workflow_doc"; then
        fail "$workflow_doc must document resolving the private ledger with ao beads dir"
    fi

    if ! grep -Fq 'git -C "$(ao beads dir)"' "$workflow_doc"; then
        fail "$workflow_doc must document the private-ledger sync (git -C \"\$(ao beads dir)\" add/commit/push)"
    fi

    if grep -Fq 'git -C _beads' "$workflow_doc"; then
        fail "$workflow_doc hard-codes _beads/ and breaks linked worktrees; use git -C \"\$(ao beads dir)\""
    fi

    if grep -Fq 'git add _beads' "$workflow_doc"; then
        fail "$workflow_doc stages _beads/ into the PUBLIC repo — the ledger is a private nested repo; use git -C \"\$(ao beads dir)\""
    fi

    bd_matches="$(grep -nE 'bd dolt (push|commit)' "$workflow_doc" || true)"
    if [[ -n "$bd_matches" ]]; then
        fail "$workflow_doc still contains live bd/Dolt closeout instructions: ${bd_matches//$'\n'/; }"
    fi
fi

if (( failures > 0 )); then
    exit 1
fi

echo "BR_CLOSEOUT_CONTRACT: PASS"

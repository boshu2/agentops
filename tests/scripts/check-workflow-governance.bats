#!/usr/bin/env bats
#
# Tests for the workflow governance / drift gate (ag-jy8gj):
# scripts/check-workflow-governance.sh asserts a BIDIRECTIONAL identity match
# between .claude/workflows/*.js and the top-level `workflows:` section of
# docs/contracts/skill-dispositions.yaml, and that each ledger row carries the
# DDD identity triple: kind: workflow + a Bounded Context (domain) + a
# hexagonal_role.
#
# The gate runs against a temp git fixture (it derives repo_root from
# `git rev-parse --show-toplevel`), so each case stamps a minimal repo with a
# workflow .js + a ledger and asserts the gate's verdict. Workflows are
# Claude-only (ag-jy8gj comment): the gate checks Claude-runtime presence and
# never requires a Codex twin.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
    GATE="$REPO_ROOT/scripts/check-workflow-governance.sh"
    # Build a throwaway git repo with the gate script + workflow dir + ledger.
    FIX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIX/scripts" "$FIX/.claude/workflows" "$FIX/docs/contracts"
    cp "$GATE" "$FIX/scripts/check-workflow-governance.sh"
    chmod +x "$FIX/scripts/check-workflow-governance.sh"
    git -C "$FIX" init -q
    git -C "$FIX" config user.email t@t.t
    git -C "$FIX" config user.name t
}

# write_workflow_js <name> <meta-name>
write_workflow_js() {
    local file="$FIX/.claude/workflows/$1"
    cat > "$file" <<JS
export const meta = { name: '$2' };
JS
    git -C "$FIX" add "$file"
}

run_gate() { ( cd "$FIX" && bash scripts/check-workflow-governance.sh ); }

@test "a fully-governed workflow (kind+BC+role, js<->ledger match) passes" {
    write_workflow_js "demo.js" "demo"
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
workflows:
  demo:
    kind:           workflow
    domain:         "BC3 Loop"
    hexagonal_role: driving-adapter
    path:           .claude/workflows/demo.js
YAML
    run run_gate
    [ "$status" -eq 0 ]
}

@test "a .js with no ledger row FAILS naming it" {
    write_workflow_js "orphan.js" "orphan"
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
workflows: {}
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"orphan"* ]]
}

@test "a ledger row kind: workflow with no matching .js FAILS as stale" {
    # No .js authored, but the ledger declares one.
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
workflows:
  ghost:
    kind:           workflow
    domain:         "BC3 Loop"
    hexagonal_role: driving-adapter
    path:           .claude/workflows/ghost.js
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"ghost"* ]]
    [[ "$output" == *"stale"* || "$output" == *"no .claude/workflows"* || "$output" == *"no matching"* ]]
}

@test "a ledger row missing its Bounded Context (domain) FAILS" {
    write_workflow_js "nobc.js" "nobc"
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
workflows:
  nobc:
    kind:           workflow
    hexagonal_role: driving-adapter
    path:           .claude/workflows/nobc.js
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"nobc"* ]]
}

@test "a ledger row missing its hexagonal_role FAILS" {
    write_workflow_js "norole.js" "norole"
    cat > "$FIX/docs/contracts/skill-dispositions.yaml" <<'YAML'
workflows:
  norole:
    kind:   workflow
    domain: "BC3 Loop"
    path:   .claude/workflows/norole.js
YAML
    run run_gate
    [ "$status" -ne 0 ]
    [[ "$output" == *"norole"* ]]
}

@test "the real repo passes the governance gate" {
    run bash "$GATE"
    [ "$status" -eq 0 ]
}

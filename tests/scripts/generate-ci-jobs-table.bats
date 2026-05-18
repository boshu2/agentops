#!/usr/bin/env bats
# Tests for scripts/generate-ci-jobs-table.sh (soc-3oij).
#
# Verifies the generator renders the AGENTS '### CI Jobs and What They Check'
# table from docs/contracts/ci-jobs.yaml + .github/workflows/validate.yml in
# workflow.summary.needs order, marks continue-on-error jobs (non-blocking),
# detects manifest/workflow drift, and rewrites the section in --write mode.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    GENERATOR="$ROOT/scripts/generate-ci-jobs-table.sh"

    [ -x "$GENERATOR" ] || skip "generator not executable: $GENERATOR"

    TMP="$(mktemp -d)"
    export TMP
}

teardown() {
    [ -n "${TMP:-}" ] && rm -rf "$TMP"
}

# Fixture helpers — write minimal valid AGENTS / workflow / manifest into TMP.
write_workflow() {
    local needs_list="$1"
    local coe_jobs="${2:-}"

    {
        echo "name: Validate"
        echo "on: { push: { branches: [main] } }"
        echo "jobs:"
        echo "  changes:"
        echo "    runs-on: ubuntu-latest"
        for j in $(echo "$needs_list" | tr ',' ' '); do
            j="$(echo "$j" | xargs)"
            [ -z "$j" ] && continue
            echo "  $j:"
            echo "    runs-on: ubuntu-latest"
            for c in $coe_jobs; do
                if [ "$j" = "$c" ]; then
                    echo "    continue-on-error: true"
                fi
            done
        done
        echo "  summary:"
        echo "    runs-on: ubuntu-latest"
        echo "    needs: [changes, $needs_list]"
        echo "    if: always()"
    } > "$TMP/validate.yml"
}

write_manifest() {
    # Args: list of "name|reason|failure" entries
    {
        echo "jobs:"
        for entry in "$@"; do
            local name="${entry%%|*}"
            local rest="${entry#*|}"
            local reason="${rest%%|*}"
            local failure="${rest#*|}"
            printf '  - name: %s\n    reason: %s\n    failure: %s\n' \
                "$name" "$reason" "$failure"
        done
    } > "$TMP/ci-jobs.yaml"
}

write_agents() {
    # Pre-populate AGENTS.md with a stub section + arbitrary table content
    cat > "$TMP/AGENTS.md" <<'EOF'
# Stub AGENTS

## Some other section

Content.

### CI Jobs and What They Check

(placeholder)

### Next section

Other content.
EOF
}

run_generator() {
    AGENTS_PATH="$TMP/AGENTS.md" \
        WORKFLOW_PATH="$TMP/validate.yml" \
        MANIFEST_PATH="$TMP/ci-jobs.yaml" \
        bash "$GENERATOR" "$@"
}

@test "render: emits workflow.summary.needs order, excludes 'changes'" {
    write_workflow "doc-release-gate, hook-preflight"
    write_manifest \
        "doc-release-gate|docs parity|stale docs" \
        "hook-preflight|hook safety|missing guard"
    write_agents

    run run_generator
    [ "$status" -eq 0 ]
    [[ "$output" == *"| Job | What it validates | Common failure |"* ]]
    [[ "$output" == *"| **doc-release-gate** | docs parity | stale docs |"* ]]
    [[ "$output" == *"| **hook-preflight** | hook safety | missing guard |"* ]]
    # 'changes' is the utility job; never rendered
    [[ "$output" != *"changes"* ]] || [[ "$output" == *"changes"*"validates"* ]]
}

@test "render: continue-on-error jobs get (non-blocking) suffix" {
    write_workflow "doc-release-gate, doctor-check" "doctor-check"
    write_manifest \
        "doc-release-gate|docs parity|stale docs" \
        "doctor-check|doctor smoke|non-blocking smoke"
    write_agents

    run run_generator
    [ "$status" -eq 0 ]
    [[ "$output" == *"**doctor-check** (non-blocking)"* ]]
    [[ "$output" != *"**doc-release-gate** (non-blocking)"* ]]
}

@test "render: orders rows by workflow.summary.needs declaration" {
    write_workflow "zeta-job, alpha-job, mu-job"
    write_manifest \
        "alpha-job|a|af" \
        "mu-job|m|mf" \
        "zeta-job|z|zf"
    write_agents

    run run_generator
    [ "$status" -eq 0 ]
    # First job-row line after the header should be zeta-job
    first_row="$(echo "$output" | grep -E '^\| \*\*' | head -n1)"
    [[ "$first_row" == *"**zeta-job**"* ]]
}

@test "fail: workflow has job missing from manifest" {
    write_workflow "doc-release-gate, mystery-job"
    write_manifest "doc-release-gate|docs|stale"
    write_agents

    run run_generator
    [ "$status" -ne 0 ]
    [[ "$output" == *"missing"* ]] || [[ "$output" == *"mystery-job"* ]]
}

@test "fail: manifest has job not in workflow.summary.needs" {
    write_workflow "doc-release-gate"
    write_manifest \
        "doc-release-gate|docs|stale" \
        "ghost-job|ghost|ghost-fail"
    write_agents

    run run_generator
    [ "$status" -ne 0 ]
    [[ "$output" == *"ghost-job"* ]]
}

@test "fail: missing manifest file exits 2" {
    write_workflow "doc-release-gate"
    write_agents
    # No write_manifest call

    run run_generator
    [ "$status" -eq 2 ]
    [[ "$output" == *"missing required file"* ]]
}

@test "fail: missing workflow file exits 2" {
    write_manifest "doc-release-gate|docs|stale"
    write_agents
    # No write_workflow call

    run run_generator
    [ "$status" -eq 2 ]
}

@test "check: passes when AGENTS section matches generator output" {
    write_workflow "doc-release-gate"
    write_manifest "doc-release-gate|docs parity|stale docs"
    write_agents

    # Bootstrap by writing once, then verify --check is a no-op pass
    run run_generator --write
    [ "$status" -eq 0 ]

    run run_generator --check
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "check: fails when AGENTS table drifts from generator output" {
    write_workflow "doc-release-gate"
    write_manifest "doc-release-gate|docs parity|stale docs"
    # AGENTS has the wrong content under the heading
    cat > "$TMP/AGENTS.md" <<'EOF'
# Stub

### CI Jobs and What They Check

| Job | What it validates | Common failure |
|-----|-------------------|----------------|
| **wrong-job** | wrong reason | wrong failure |

### Next section
EOF

    run run_generator --check
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL"* ]] || [[ "$output" == *"drift"* ]]
}

@test "write: rewrites the section in place; --check then passes" {
    write_workflow "doc-release-gate, hook-preflight" ""
    write_manifest \
        "doc-release-gate|docs|stale" \
        "hook-preflight|hooks|missing"
    write_agents

    # Initial --check should fail (placeholder content doesn't match)
    run run_generator --check
    [ "$status" -ne 0 ]

    # --write should succeed and produce a matching section
    run run_generator --write
    [ "$status" -eq 0 ]
    [[ "$output" == *"wrote"* ]]

    # Subsequent --check should pass
    run run_generator --check
    [ "$status" -eq 0 ]
}

@test "write: preserves surrounding sections (Some other / Next section)" {
    write_workflow "doc-release-gate"
    write_manifest "doc-release-gate|d|df"
    write_agents

    run run_generator --write
    [ "$status" -eq 0 ]

    grep -q "^## Some other section" "$TMP/AGENTS.md"
    grep -q "^### Next section$" "$TMP/AGENTS.md"
    grep -q "^Other content\.$" "$TMP/AGENTS.md"
}

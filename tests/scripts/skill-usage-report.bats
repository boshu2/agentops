#!/usr/bin/env bats
# skill-usage-report.bats — age-skills-audit-fable-l6ic.5
# Fixture fidelity: transcript lines below use the REAL persisted shapes observed
# 2026-07-07 — compact `"skill":"name"` Skill-tool inputs and
# `<command-name>/name</command-name>` slash records — never invented schemas.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/skill-usage-report.sh"
    TDIR="$BATS_TEST_TMPDIR/projects/-Users-x-dev-repo"
    mkdir -p "$TDIR"
    cat > "$TDIR/s1.jsonl" <<'EOF'
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"discovery","args":"x"}}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"discovery"}}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"validate"}}]}}
{"type":"user","message":{"content":"<command-name>/crank</command-name><command-args>epic-1</command-args>"}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"standards"}}]}}
EOF
}

@test "counts Skill-tool calls and slash commands from real persisted shapes" {
    run bash "$SCRIPT" --dir "$BATS_TEST_TMPDIR/projects" --since 9999
    [ "$status" -eq 0 ]
    [[ "$output" == *"2  discovery"* ]]
    [[ "$output" == *"1  validate"* ]]
    [[ "$output" == *"1  crank"* ]]
    [[ "$output" == *"CAVEATS"* ]]
}

@test "library skills are segmented, not mixed into invocable counts" {
    run bash "$SCRIPT" --dir "$BATS_TEST_TMPDIR/projects" --since 9999
    [ "$status" -eq 0 ]
    # standards appears in the library section, after its header
    lib_part="${output#*library / read-not-invoked}"
    [[ "$lib_part" == *"1  standards"* ]]
    inv_part="${output%%library / read-not-invoked*}"
    [[ "$inv_part" != *"standards"* ]]
}

@test "--json emits machine-readable counts with caveats and segmentation" {
    run bash "$SCRIPT" --dir "$BATS_TEST_TMPDIR/projects" --since 9999 --json
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.invocations | map(select(.skill=="discovery"))[0].count == 2'
    echo "$output" | jq -e '.library_hits | map(select(.skill=="standards"))[0].count == 1'
    echo "$output" | jq -e '.caveats | length >= 3'
    echo "$output" | jq -e '.since_days == 9999'
}

@test "missing transcripts dir fails with remedy" {
    run bash "$SCRIPT" --dir "$BATS_TEST_TMPDIR/nope"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
}

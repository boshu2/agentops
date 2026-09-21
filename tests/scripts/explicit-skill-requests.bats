#!/usr/bin/env bats
setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SUITE="$REPO_ROOT/tests/explicit-skill-requests"
    FIXTURE="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$FIXTURE"/{schemas,.claude-plugin,.codex-plugin,plugins,skills/research,skills-codex/research,tests/explicit-skill-requests/prompts}
    cp "$REPO_ROOT/.claude-plugin/plugin.json" "$FIXTURE/.claude-plugin/"
    cp "$REPO_ROOT/.codex-plugin/plugin.json" "$FIXTURE/.codex-plugin/"
    cp "$REPO_ROOT/plugins/marketplace.json" "$FIXTURE/plugins/"
    for schema in plugin-manifest codex-plugin-manifest codex-marketplace; do
        cp "$REPO_ROOT/schemas/$schema.v1.schema.json" "$FIXTURE/schemas/"
    done
    cp "$REPO_ROOT/skills/research/SKILL.md" "$FIXTURE/skills/research/"
    cp "$REPO_ROOT/skills-codex/research/SKILL.md" "$FIXTURE/skills-codex/research/"
    cp "$SUITE/prompts/research.txt" "$FIXTURE/tests/explicit-skill-requests/prompts/"
}

@test "current explicit address resolves both artifact surfaces" {
    run bash "$SUITE/run-all.sh" "$FIXTURE"
    [ "$status" -eq 0 ]
    [[ "$output" == *'1 passed, 0 failed'* ]]
    [[ "$output" == *'Live skill selection and first-tool ordering are not checked.'* ]]
}

@test "missing canonical and projected targets fail" {
    for surface in skills skills-codex; do
        mv "$FIXTURE/$surface/research/SKILL.md" "$FIXTURE/held.md"
        run bash "$SUITE/run-test.sh" research "$FIXTURE"
        [ "$status" -ne 0 ]
        [[ "$output" == *'canonical target missing'* ]]
        mv "$FIXTURE/held.md" "$FIXTURE/$surface/research/SKILL.md"
    done
}

@test "frontmatter name mismatch or missing name fails on either surface" {
    for surface in skills skills-codex; do
        cp "$FIXTURE/$surface/research/SKILL.md" "$FIXTURE/held.md"
        for name in 'name: retired' ''; do
            printf -- '---\n%s\ndescription: fixture\n---\n' "$name" > "$FIXTURE/$surface/research/SKILL.md"
            run bash "$SUITE/run-test.sh" research "$FIXTURE"
            [ "$status" -ne 0 ]
            [[ "$output" == *'differs from requested slug'* ]]
        done
        mv "$FIXTURE/held.md" "$FIXTURE/$surface/research/SKILL.md"
    done
}

@test "invalid manifest cannot pass explicit request suite" {
    printf '{"name":42}\n' > "$FIXTURE/.claude-plugin/plugin.json"
    run bash "$SUITE/run-all.sh" "$FIXTURE"
    [ "$status" -ne 0 ]
    [[ "$output" == *'plugin manifest failed schema validation'* ]]
}

@test "wrong address, bad slug and missing current fixture fail" {
    printf 'Use /agentops:researcher for this task.\n' > "$FIXTURE/tests/explicit-skill-requests/prompts/research.txt"
    run bash "$SUITE/run-test.sh" research "$FIXTURE"
    [ "$status" -ne 0 ]
    run bash "$SUITE/run-test.sh" ../research "$FIXTURE"
    [ "$status" -ne 0 ]
    rm "$FIXTURE/tests/explicit-skill-requests/prompts/research.txt"
    run bash "$SUITE/run-all.sh" "$FIXTURE"
    [ "$status" -ne 0 ]
    [[ "$output" == *'lacks an explicit request fixture'* ]]
}

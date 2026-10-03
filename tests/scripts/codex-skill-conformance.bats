#!/usr/bin/env bats
# Codex loads the canonical skills/ tree directly. These tests pin the facts
# scripts/validate-codex-api-conformance.sh holds about that loader: what makes
# Codex refuse a skill, what it loads that should never ship, and where it
# reads the invocation policy.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  GATE="$REPO_ROOT/scripts/validate-codex-api-conformance.sh"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/skills"
  mkdir -p "$FIXTURE_ROOT/sample-skill"
}

write_skill() {
  printf '%s' "$1" > "$FIXTURE_ROOT/sample-skill/SKILL.md"
}

write_policy() {
  mkdir -p "$FIXTURE_ROOT/sample-skill/agents"
  printf '%s' "$1" > "$FIXTURE_ROOT/sample-skill/agents/openai.yaml"
}

run_gate() {
  run env CODEX_SKILLS_ROOT="$FIXTURE_ROOT" bash "$GATE"
}

@test "the shipped skills tree is loadable by Codex" {
  local expected
  expected="$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["skills"]))' "$REPO_ROOT/skills/catalog.json")"
  [ "$expected" -gt 0 ]
  run bash "$GATE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS [codex] $expected package(s)"* ]]
}

@test "the Codex plugin manifest ships the canonical skills tree" {
  run jq -r '.skills' "$REPO_ROOT/.codex-plugin/plugin.json"
  [ "$status" -eq 0 ]
  [ "$output" = "./skills" ]
  [ -d "$REPO_ROOT/skills" ]
}

@test "a minimal package passes" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n'
  run_gate
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS [codex] 1 package(s)"* ]]
}

@test "host-only frontmatter fields pass because Codex ignores them" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\npractices: [tdd]\nhexagonal_role: domain\nskill_api_version: 1\nuser-invocable: true\nmetadata:\n  tier: execution\n  dependencies: [plan]\n---\n# Sample skill\n'
  run_gate
  [ "$status" -eq 0 ]
}

@test "a repeated frontmatter key is rejected" {
  write_skill $'---\nname: wrong-name\nname: sample-skill\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"found duplicate key"* ]]
}

@test "a missing or empty description is rejected" {
  write_skill $'---\nname: sample-skill\n---\n# Sample skill\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"FAIL [sample-skill] description must be a nonempty string"* ]]

  write_skill $'---\nname: sample-skill\ndescription: \'\'\n---\n# Sample skill\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"FAIL [sample-skill] description must be a nonempty string"* ]]
}

@test "a name over 64 characters is rejected and 64 is accepted" {
  local long
  long="$(printf 'n%.0s' $(seq 1 65))"
  write_skill "$(printf -- '---\nname: %s\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n' "$long")"
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"name must be a string of at most 64 characters"* ]]

  write_skill "$(printf -- '---\nname: %s\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n' "${long%n}")"
  run_gate
  [ "$status" -eq 0 ]
}

@test "a SKILL.md nested below a skill directory is rejected" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n'
  mkdir -p "$FIXTURE_ROOT/_fixtures/planted"
  printf '%s' $'---\nname: Planted\ndescription: A fixture Codex would load as a skill.\n---\n# Planted\n' \
    > "$FIXTURE_ROOT/_fixtures/planted/SKILL.md"
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"nested _fixtures/planted/SKILL.md would be loaded by Codex as its own skill"* ]]
}

@test "the shipped skills tree holds no nested SKILL.md" {
  run find "$REPO_ROOT/skills" -mindepth 3 -name SKILL.md
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "an explicit-only skill without a Codex policy is rejected" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\ndisable-model-invocation: true\n---\n# Sample skill\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"needs agents/openai.yaml with policy.allow_implicit_invocation: false"* ]]

  write_policy $'policy:\n  allow_implicit_invocation: true\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"needs agents/openai.yaml with policy.allow_implicit_invocation: false"* ]]

  write_policy $'interface:\n  display_name: Sample\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"needs agents/openai.yaml with policy.allow_implicit_invocation: false"* ]]
}

@test "an explicit-only skill with the Codex policy passes" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\ndisable-model-invocation: true\n---\n# Sample skill\n'
  write_policy $'interface:\n  display_name: Sample\npolicy:\n  allow_implicit_invocation: false\n'
  run_gate
  [ "$status" -eq 0 ]
}

@test "a caller-authored explicit-only policy passes without the frontmatter flag" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n'
  write_policy $'policy:\n  allow_implicit_invocation: false\n'
  run_gate
  [ "$status" -eq 0 ]
}

@test "a malformed agents/openai.yaml is rejected" {
  write_skill $'---\nname: sample-skill\ndescription: Use when a fixture is needed.\n---\n# Sample skill\n'
  write_policy $'policy: [unclosed\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"agents/openai.yaml is not valid YAML"* ]]

  write_policy $'policy:\n  allow_implicit_invocation: maybe\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"policy.allow_implicit_invocation must be spelled true or false"* ]]

  # Shapes Codex 0.156.1 drops silently, policy included.
  write_policy $'policy:\n  allow_implicit_invocation: no\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"Codex reads 'no' as a string"* ]]

  write_policy $'interface: nope\npolicy:\n  allow_implicit_invocation: false\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"interface must be a mapping"* ]]

  write_policy $'dependencies:\n  tools: nope\npolicy:\n  allow_implicit_invocation: false\n'
  run_gate
  [ "$status" -ne 0 ]
  [[ "$output" == *"dependencies must be a mapping with a tools list"* ]]
}

@test "every explicit-only source skill carries the Codex policy" {
  # The generated copy used to derive this file; it is now hand-maintained in
  # the source skill, so removing it must turn the gate red.
  local sandbox="$BATS_TEST_TMPDIR/tree"
  mkdir -p "$sandbox"
  cp -R "$REPO_ROOT/skills" "$sandbox/skills"
  run env CODEX_SKILLS_ROOT="$sandbox/skills" bash "$GATE"
  [ "$status" -eq 0 ]

  local flagged=0 skill_md skill
  for skill_md in "$sandbox"/skills/*/SKILL.md; do
    grep -q '^disable-model-invocation: true$' "$skill_md" || continue
    flagged=$((flagged + 1))
    skill="$(basename "$(dirname "$skill_md")")"
    mv "$sandbox/skills/$skill/agents/openai.yaml" "$sandbox/held.yaml"
    run env CODEX_SKILLS_ROOT="$sandbox/skills" bash "$GATE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"FAIL [$skill] disable-model-invocation: true needs agents/openai.yaml"* ]]
    mv "$sandbox/held.yaml" "$sandbox/skills/$skill/agents/openai.yaml"
  done
  [ "$flagged" -gt 0 ]
}

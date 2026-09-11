#!/usr/bin/env bats
# Exercise the production generator in an isolated minimal catalog. No writes
# reach installed skills, repository source skills or checked-in projections.

setup() {
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/catalog"
  SOURCE="$FIXTURE_ROOT/skills/routing-probe"
  TWIN="$FIXTURE_ROOT/skills-codex/routing-probe"
  GENERATOR="$FIXTURE_ROOT/scripts/codex-sync.sh"
  mkdir -p "$SOURCE" "$FIXTURE_ROOT/scripts" "$FIXTURE_ROOT/skills-codex" \
    "$FIXTURE_ROOT/skills-codex-overrides"
  cp "$BATS_TEST_DIRNAME/../../scripts/codex-sync.sh" "$GENERATOR"
  printf '%s\n' '{"skills":[],"codex_override_catalog":{"skills":[]}}' \
    > "$FIXTURE_ROOT/skills-codex/.agentops-manifest.json"
  printf '%s\n' '{"skills":[]}' > "$FIXTURE_ROOT/skills-codex-overrides/catalog.json"
}

write_source() {
  cat > "$SOURCE/SKILL.md" <<'EOF'
---
name: routing-probe
description: >-
  Compare a claimed state with evidence. Requires a concrete claim to test.
  Use when checking completion. Do not use for unscoped exploration.
  Triggers: "check the claim", "verify completion".
EOF
  if [ -n "${1:-}" ]; then
    printf 'disable-model-invocation: %s\n' "$1" >> "$SOURCE/SKILL.md"
  fi
  printf '%s\n' '---' '# Routing probe' 'Follow the caller-selected claim.' >> "$SOURCE/SKILL.md"
}

generate() {
  run bash "$GENERATOR" "$@"
  echo "$output" >&2
  [ "$status" -eq 0 ]
}

assert_explicit_only() {
  python3 - "$TWIN" <<'PY'
import pathlib, sys, yaml
twin = pathlib.Path(sys.argv[1])
metadata = yaml.safe_load((twin / "agents/openai.yaml").read_text())
assert metadata["policy"]["allow_implicit_invocation"] is False
frontmatter = yaml.safe_load((twin / "SKILL.md").read_text().split("---", 2)[1])
assert set(frontmatter) == {"name", "description"}
PY
}

@test "complete folded descriptions preserve use cases, required inputs, exclusions and triggers" {
  write_source
  generate
  python3 - "$TWIN/SKILL.md" <<'PY'
import pathlib, sys, yaml
description = yaml.safe_load(pathlib.Path(sys.argv[1]).read_text().split("---", 2)[1])["description"]
assert description == (
    'Compare a claimed state with evidence. Requires a concrete claim to test. '
    'Use when checking completion. Do not use for unscoped exploration. '
    'Triggers: "check the claim", "verify completion".'
), description
PY
  generate --check
}

@test "descriptions without a Triggers clause still retain later use cases and exclusions" {
  cat > "$SOURCE/SKILL.md" <<'EOF'
---
name: routing-probe
description: 'Find skill guidance. Search or load a named skill. Avoid running the selected workflow.'
---
# Routing probe
EOF
  generate
  python3 - "$TWIN/SKILL.md" <<'PY'
import pathlib, sys, yaml
description = yaml.safe_load(pathlib.Path(sys.argv[1]).read_text().split("---", 2)[1])["description"]
assert description == 'Find skill guidance. Search or load a named skill. Avoid running the selected workflow.'
PY
}

@test "absent or false source flag leaves Codex default implicit invocation enabled" {
  for flag in '' false; do
    write_source "$flag"
    generate
    [ ! -e "$TWIN/agents/openai.yaml" ]
    generate --check
  done
}

@test "true flag generates explicit-only policy and check mode detects missing policy" {
  write_source true
  generate
  assert_explicit_only
  [ ! -e "$SOURCE/agents/openai.yaml" ]
  generate --check

  rm "$TWIN/agents/openai.yaml"
  run bash "$GENERATOR" --check
  [ "$status" -ne 0 ]
  [[ "$output" == *"missing agents/openai.yaml"* ]]
  [ ! -e "$TWIN/agents/openai.yaml" ]
  generate
  assert_explicit_only
  generate --check
}

@test "false or removed source flag clears a generated-only explicit invocation policy" {
  for flag in false ''; do
    write_source true
    generate
    assert_explicit_only
    write_source "$flag"
    run bash "$GENERATOR" --check
    [ "$status" -ne 0 ]
    [[ "$output" == *"extra agents/openai.yaml"* ]]
    generate
    [ ! -e "$TWIN/agents/openai.yaml" ]
    generate --check
  done
}

@test "policy projection preserves source UI, dependencies and sibling files across flag removal" {
  write_source true
  mkdir -p "$SOURCE/agents"
  cat > "$SOURCE/agents/openai.yaml" <<'EOF'
# Caller-owned metadata: keep values and restore original bytes on removal.
interface:
  display_name: "Check a claim"
  short_description: "Compare claimed and observed state"
  default_prompt: "Use $routing-probe with the supplied claim."
  brand_color: "#123456"
policy:
  allow_implicit_invocation: true
dependencies:
  tools:
    - type: mcp
      value: evidence
      description: "Evidence source"
      transport: streamable_http
      url: https://example.com/mcp
EOF
  cp "$SOURCE/agents/openai.yaml" "$BATS_TEST_TMPDIR/original.yaml"
  printf '%s\n' 'Keep this sibling resource.' > "$SOURCE/agents/context.md"
  generate
  assert_explicit_only
  cmp "$SOURCE/agents/openai.yaml" "$BATS_TEST_TMPDIR/original.yaml"
  cmp "$SOURCE/agents/context.md" "$TWIN/agents/context.md"
  python3 - "$SOURCE/agents/openai.yaml" "$TWIN/agents/openai.yaml" <<'PY'
import sys, yaml
source, twin = (yaml.safe_load(open(path)) for path in sys.argv[1:])
source["policy"]["allow_implicit_invocation"] = False
assert twin == source, (source, twin)
PY
  generate --check
  generate --force --only routing-probe
  generate --check
  write_source
  generate
  cmp "$SOURCE/agents/openai.yaml" "$TWIN/agents/openai.yaml"
  cmp "$SOURCE/agents/context.md" "$TWIN/agents/context.md"
  generate --check
}

@test "caller-authored source policy stays in effect after the frontmatter flag is removed" {
  write_source true
  mkdir -p "$SOURCE/agents"
  printf '%s\n' 'policy:' '  allow_implicit_invocation: false' > "$SOURCE/agents/openai.yaml"
  generate
  write_source
  generate
  cmp "$SOURCE/agents/openai.yaml" "$TWIN/agents/openai.yaml"
  assert_explicit_only
  generate --check
}

@test "invalid source policy metadata is rejected instead of silently overwritten" {
  write_source true
  mkdir -p "$SOURCE/agents"
  printf '%s\n' 'policy: false' > "$SOURCE/agents/openai.yaml"
  run bash "$GENERATOR"
  [ "$status" -ne 0 ]
  [[ "$output" == *"policy must be a mapping"* ]]
  [ ! -e "$TWIN/agents/openai.yaml" ]
}

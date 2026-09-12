#!/usr/bin/env bats

setup() {
  ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export CODEX_HOME="$BATS_TEST_TMPDIR/codex home"
}

require_codex() {
  command -v codex >/dev/null 2>&1 || skip "Codex runtime required for native config editor"
}

@test "Codex project registrations resolve role templates with required config" {
  python3 - "$ROOT" <<'PY'
import pathlib, sys, tomllib
root = pathlib.Path(sys.argv[1])
config = tomllib.loads((root / '.codex/config.toml').read_text())
for name in ('bulk-reader', 'code-writer'):
    source = root / 'skills/agent-native/agents' / (name + '.toml')
    project = root / '.codex/agents' / (name + '.toml')
    assert project.resolve() == source.resolve()
    assert (root / '.codex' / config['agents'][name]['config_file']).resolve() == source.resolve()
    data = tomllib.loads(project.read_text())
    assert data['name'] == name and data['description'] and data['developer_instructions']
    assert data['model'] == 'gpt-5.6-luna'
    assert data['sandbox_mode'] == ('read-only' if name == 'bulk-reader' else 'workspace-write')
PY
}

@test "personal installation copies generated roles and does not enable hooks" {
  require_codex
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  cmp "$CODEX_HOME/agents/bulk-reader.toml" "$ROOT/skills-codex/agent-native/agents/bulk-reader.toml"
  cmp "$CODEX_HOME/agents/code-writer.toml" "$ROOT/skills-codex/agent-native/agents/code-writer.toml"
  [ ! -e "$CODEX_HOME/hooks.json" ]
  [ -f "$CODEX_HOME/config.toml" ]
  python3 - "$CODEX_HOME/config.toml" <<'PY'
import sys,tomllib
with open(sys.argv[1], "rb") as f: cfg=tomllib.load(f)
assert set(cfg["agents"]) == {"bulk-reader", "code-writer"}
for role in cfg["agents"]:
    assert cfg["agents"][role]["config_file"].endswith("/"+role+".toml")
PY
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  [ "$(find "$CODEX_HOME" -name '*.bak.*' | wc -l | tr -d ' ')" -eq 0 ]
}

@test "changed role backups are retained and symlink source is preserved" {
  require_codex
  mkdir -p "$CODEX_HOME/agents"
  printf 'original\n' > "$BATS_TEST_TMPDIR/original.toml"
  ln -s "$BATS_TEST_TMPDIR/original.toml" "$CODEX_HOME/agents/bulk-reader.toml"
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  [ "$(cat "$BATS_TEST_TMPDIR/original.toml")" = original ]
  [ ! -L "$CODEX_HOME/agents/bulk-reader.toml" ]
  [ "$(cat "$CODEX_HOME"/agents/bulk-reader.toml.bak.*)" = original ]
  printf 'second\n' > "$CODEX_HOME/agents/bulk-reader.toml"
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  [ "$(find "$CODEX_HOME/agents" -name '*.bak.*' | wc -l | tr -d ' ')" -eq 2 ]
}

@test "project installation targets the caller project" {
  require_codex
  mkdir -p "$BATS_TEST_TMPDIR/project"
  cd "$BATS_TEST_TMPDIR/project"
  run bash "$ROOT/scripts/install-codex-context-agents.sh" --project
  [ "$status" -eq 0 ]
  [ -f .codex/agents/code-writer.toml ]
  [ ! -e "$CODEX_HOME/agents" ]
}

@test "native config registration preserves unrelated TOML and is idempotent" {
  require_codex
  mkdir -p "$CODEX_HOME"
  printf 'model = "gpt-6-astra"\n[agents.other]\ndescription = "existing"\n' > "$CODEX_HOME/config.toml"
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  python3 - "$CODEX_HOME/config.toml" <<'PY'
import sys,tomllib
with open(sys.argv[1], "rb") as f: cfg=tomllib.load(f)
assert cfg["model"] == "gpt-6-astra"
assert cfg["agents"]["other"]["description"] == "existing"
assert set(cfg["agents"]) == {"other", "bulk-reader", "code-writer"}
PY
  [ "$(find "$CODEX_HOME" -name 'config.toml.bak.*' | wc -l | tr -d ' ')" -eq 1 ]
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -eq 0 ]
  [ "$(find "$CODEX_HOME" -name 'config.toml.bak.*' | wc -l | tr -d ' ')" -eq 1 ]
}

@test "malformed existing config fails before publishing roles or modifying settings" {
  require_codex
  mkdir -p "$CODEX_HOME/agents"
  printf '[invalid TOML\n' > "$CODEX_HOME/config.toml"
  printf 'existing role\n' > "$CODEX_HOME/agents/bulk-reader.toml"
  cp "$CODEX_HOME/config.toml" "$BATS_TEST_TMPDIR/original-config"
  run bash "$ROOT/scripts/install-codex-context-agents.sh"
  [ "$status" -ne 0 ]
  cmp "$CODEX_HOME/config.toml" "$BATS_TEST_TMPDIR/original-config"
  [ "$(cat "$CODEX_HOME/agents/bulk-reader.toml")" = 'existing role' ]
  [ ! -e "$CODEX_HOME/agents/code-writer.toml" ]
  [ "$(find "$CODEX_HOME" -name '*.bak.*' | wc -l | tr -d ' ')" -eq 0 ]
}

#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_DIR="$REPO_ROOT/.agy-plugin"

core_skills=(
  agent-mail
  beads-br
  beads-bv
  beads-workflow
  ntm
  vibing-with-ntm
  cass
  cass-memory
  dcg
  caam
  casr
  ubs
  ru-multi-repo-workflow
  gh-triage-ru
  rch
  sbh
  process-triage
  system-performance-remediation
  ssh
  gcloud
  gh-cli
  gh-actions
  planning-workflow
  multi-model-triangulation
  research-software
  repeatedly-apply-skill
  cc-hooks
)

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

command -v jq >/dev/null || fail "jq is required"
command -v agy >/dev/null || fail "agy is required for plugin validation"

[[ -f "$PLUGIN_DIR/plugin.json" ]] || fail "missing .agy-plugin/plugin.json"
[[ -f "$PLUGIN_DIR/mcp_config.json" ]] || fail "missing .agy-plugin/mcp_config.json"
[[ -f "$PLUGIN_DIR/hooks.json" ]] || fail "missing .agy-plugin/hooks.json"

jq -e '
  .name == "agentops-antigravity-core"
  and .skills == "./skills"
  and .mcpServers["agent-mail"].command == "am"
  and .mcpServers["agent-mail"].args == ["serve-stdio"]
' "$PLUGIN_DIR/plugin.json" >/dev/null || fail "plugin.json does not expose the expected skills and Agent Mail MCP"

jq -e '
  .mcpServers["agent-mail"].command == "am"
  and .mcpServers["agent-mail"].args == ["serve-stdio"]
' "$PLUGIN_DIR/mcp_config.json" >/dev/null || fail "mcp_config.json does not expose Agent Mail over stdio"

jq -e '
  .hooks.BeforeTool[0].matcher == "run_shell_command"
  and .hooks.BeforeTool[0].hooks[0].name == "agentops-dcg"
  and .hooks.BeforeTool[0].hooks[0].command == "dcg"
  and .hooks.Stop[0].hooks[0].name == "agentops-evidence-surface"
' "$PLUGIN_DIR/hooks.json" >/dev/null || fail "hooks.json does not expose the expected guard/evidence hooks"

actual_count="$(
  find "$PLUGIN_DIR/skills" -mindepth 2 -maxdepth 2 -name SKILL.md -print |
    wc -l |
    tr -d ' '
)"
expected_count="${#core_skills[@]}"
[[ "$actual_count" == "$expected_count" ]] || fail "expected $expected_count bundled skills, found $actual_count"

for skill in "${core_skills[@]}"; do
  source_file="$REPO_ROOT/skills/$skill/SKILL.md"
  bundled_file="$PLUGIN_DIR/skills/$skill/SKILL.md"

  [[ -f "$source_file" ]] || fail "missing canonical source skill: $source_file"
  [[ -f "$bundled_file" ]] || fail "missing bundled skill: $bundled_file"
  cmp -s "$source_file" "$bundled_file" || fail "bundled skill drifted from source: $skill"
done

agy plugin validate "$PLUGIN_DIR"

printf 'PASS: Antigravity plugin bundle is valid (%s skills)\n' "$expected_count"
